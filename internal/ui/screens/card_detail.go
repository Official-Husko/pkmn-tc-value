package screens

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/viewmodel"
)

type RefreshFunc func(context.Context, domain.Card) (domain.Card, error)

type detailRefreshDoneMsg struct {
	card domain.Card
	err  error
}

type DetailResult struct {
	Action string
	Card   domain.Card
}

type detailModel struct {
	ctx        context.Context
	card       domain.Card
	cfg        config.Config
	renderer   images.Renderer
	refresh    RefreshFunc
	styles     theme.Styles
	selected   int
	status     string
	image      string
	done       bool
	result     DetailResult
	refreshing bool
}

func RunCardDetail(ctx context.Context, card domain.Card, cfg config.Config, renderer images.Renderer, colors bool, needsRefresh bool, refresh RefreshFunc) (DetailResult, error) {
	model := detailModel{
		ctx:      ctx,
		card:     card,
		cfg:      cfg,
		renderer: renderer,
		refresh:  refresh,
		styles:   theme.NewStyles(colors),
		status:   "Loaded from local data",
		image:    renderImage(renderer, card.ImagePath),
	}
	if cfg.SaveSearchedCardsDefault {
		model.selected = 1
	}
	if needsRefresh {
		model.refreshing = true
		model.status = "Refreshing prices..."
	}
	p, err := tea.NewProgram(model).Run()
	if err != nil {
		return DetailResult{}, err
	}
	finalModel := p.(detailModel)
	return finalModel.result, nil
}

func (m detailModel) Init() tea.Cmd {
	if !m.refreshing {
		return nil
	}
	return func() tea.Msg {
		card, err := m.refresh(m.ctx, m.card)
		return detailRefreshDoneMsg{card: card, err: err}
	}
}

func (m detailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detailRefreshDoneMsg:
		m.refreshing = false
		if msg.err != nil {
			m.status = "Refresh failed, showing cached data"
			return m, nil
		}
		m.card = msg.card
		m.image = renderImage(m.renderer, m.card.ImagePath)
		m.status = "Updated just now"
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.selected > 0 {
				m.selected--
			}
		case "right", "l", "tab":
			if m.selected < 1 {
				m.selected++
			}
		case "enter":
			if m.selected == 0 {
				m.result = DetailResult{Action: "close", Card: m.card}
			} else {
				m.result = DetailResult{Action: "add", Card: m.card}
			}
			return m, tea.Quit
		case "esc", "q":
			m.result = DetailResult{Action: "close", Card: m.card}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m detailModel) View() string {
	left := m.styles.Card.Width(34).Render(m.image)
	lines := append(viewmodel.DetailLines(m.card), "", "Status: "+m.status, "", renderActionRow(m.styles, m.selected))
	right := m.styles.Card.Width(44).Render(strings.Join(lines, "\n"))
	title := m.styles.Title.Render("Card Detail")
	return lipgloss.NewStyle().Padding(1, 2).Render(title + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
}

func renderActionRow(styles theme.Styles, selected int) string {
	closeStyle := styles.Action
	addStyle := styles.Action
	if selected == 0 {
		closeStyle = styles.Active
	} else {
		addStyle = styles.Active
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, closeStyle.Render("Close"), " ", addStyle.Render("Add to collection"))
}

func renderImage(renderer images.Renderer, path string) string {
	out, err := renderer.Render(path, 32, 20)
	if err != nil {
		return "[image unavailable]"
	}
	return out
}
