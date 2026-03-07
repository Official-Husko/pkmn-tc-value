package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type syncDoneMsg struct {
	stats syncer.Stats
	err   error
}

type syncProgressMsg struct {
	progress syncer.StartupProgress
}

type startupSyncModel struct {
	ctx        context.Context
	run        func(context.Context, func(syncer.StartupProgress)) (syncer.Stats, error)
	styles     uitheme.Styles
	progressCh chan syncer.StartupProgress
	doneCh     chan syncDoneMsg
	progress   syncer.StartupProgress
	stats      syncer.Stats
	err        error
	done       bool
}

func RunStartupSync(ctx context.Context, colors bool, run func(context.Context, func(syncer.StartupProgress)) (syncer.Stats, error)) (syncer.Stats, error) {
	model := startupSyncModel{
		ctx:        ctx,
		run:        run,
		styles:     uitheme.NewStyles(colors),
		progressCh: make(chan syncer.StartupProgress, 1024),
		doneCh:     make(chan syncDoneMsg, 1),
	}
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return syncer.Stats{}, err
	}
	result := finalModel.(startupSyncModel)
	return result.stats, result.err
}

func (m startupSyncModel) Init() tea.Cmd {
	go func() {
		stats, err := m.run(m.ctx, func(p syncer.StartupProgress) {
			select {
			case m.progressCh <- p:
			default:
			}
		})
		m.doneCh <- syncDoneMsg{stats: stats, err: err}
	}()
	return tea.Batch(waitForSyncProgress(m.progressCh), waitForSyncDone(m.doneCh))
}

func (m startupSyncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncProgressMsg:
		m.progress = msg.progress
		return m, waitForSyncProgress(m.progressCh)
	case syncDoneMsg:
		m.stats = msg.stats
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m startupSyncModel) View() string {
	title := m.styles.Title.Render("Startup Sync")
	setPercent := 0.0
	if m.progress.SetsTotal > 0 {
		setPercent = float64(m.progress.SetsDone) / float64(m.progress.SetsTotal)
	}
	body := []string{
		title,
		"",
		fmt.Sprintf("Sets  %s %d/%d", renderBar(setPercent, 36), m.progress.SetsDone, m.progress.SetsTotal),
	}
	if m.progress.CardsTotal > 0 {
		cardPercent := float64(m.progress.CardsDone) / float64(m.progress.CardsTotal)
		body = append(body, fmt.Sprintf("Cards %s %d/%d", renderBar(cardPercent, 36), m.progress.CardsDone, m.progress.CardsTotal))
	}
	body = append(body, "", "Status: "+m.progress.Status)
	if m.progress.CurrentSet != "" {
		body = append(body, "Current: "+m.progress.CurrentSet)
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, body...))
}

func waitForSyncProgress(ch <-chan syncer.StartupProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return syncProgressMsg{progress: p}
	}
}

func waitForSyncDone(ch <-chan syncDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func renderBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent * float64(width))
	return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
}
