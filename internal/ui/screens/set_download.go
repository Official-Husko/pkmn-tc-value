package screens

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type SetDownloadResult struct {
	SetName       string
	NewCards      int
	UpdatedCards  int
	TotalCards    int
	ImagesSaved   int
	DetailsSynced int
	DetailsFailed int
}

type SetDownloadProgress struct {
	Stage  string
	Status string
	Done   int
	Total  int
}

type setDownloadDoneMsg struct {
	result SetDownloadResult
	err    error
}

type setDownloadProgressMsg struct {
	progress SetDownloadProgress
}

type setDownloadModel struct {
	ctx        context.Context
	set        domain.Set
	run        func(context.Context, func(SetDownloadProgress)) (SetDownloadResult, error)
	styles     uitheme.Styles
	spinner    spinner.Model
	progressCh chan SetDownloadProgress
	doneCh     chan setDownloadDoneMsg
	progress   SetDownloadProgress
	result     SetDownloadResult
	err        error
	finished   bool
}

func RunSetDownload(ctx context.Context, set domain.Set, colors bool, run func(context.Context, func(SetDownloadProgress)) (SetDownloadResult, error)) (SetDownloadResult, error) {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	if colors {
		spin.Style = lipgloss.NewStyle().Foreground(uitheme.Gold)
	}
	model := setDownloadModel{
		ctx:        ctx,
		set:        set,
		run:        run,
		styles:     uitheme.NewStyles(colors),
		spinner:    spin,
		progressCh: make(chan SetDownloadProgress, 64),
		doneCh:     make(chan setDownloadDoneMsg, 1),
		progress: SetDownloadProgress{
			Stage:  "cards",
			Status: "Starting set sync",
		},
	}
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return SetDownloadResult{}, err
	}
	finalModel := result.(setDownloadModel)
	return finalModel.result, finalModel.err
}

func (m setDownloadModel) Init() tea.Cmd {
	go func() {
		result, err := m.run(m.ctx, func(progress SetDownloadProgress) {
			select {
			case m.progressCh <- progress:
			default:
			}
		})
		m.doneCh <- setDownloadDoneMsg{result: result, err: err}
	}()
	return tea.Batch(m.spinner.Tick, waitForSetDownloadProgress(m.progressCh), waitForSetDownloadDone(m.doneCh))
}

func (m setDownloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setDownloadProgressMsg:
		m.progress = msg.progress
		return m, waitForSetDownloadProgress(m.progressCh)
	case setDownloadDoneMsg:
		m.result = msg.result
		m.err = msg.err
		m.finished = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m setDownloadModel) View() string {
	title := m.styles.Title.Render("Downloading Database")
	status := m.progress.Status
	if status == "" {
		status = "Working..."
	}
	count := ""
	if m.progress.Total > 0 {
		count = fmt.Sprintf(" (%d/%d)", m.progress.Done, m.progress.Total)
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("%s Syncing %s", m.spinner.View(), m.set.Name),
		fmt.Sprintf("Stage: %s", m.progress.Stage),
		"Status: "+status+count,
		"Please wait...",
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(title + "\n\n" + body)
}

func waitForSetDownloadProgress(ch <-chan SetDownloadProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return nil
		}
		return setDownloadProgressMsg{progress: progress}
	}
}

func waitForSetDownloadDone(ch <-chan setDownloadDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
