package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	termimg "github.com/blacktop/go-termimg"
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
	status     string
	imageErr   string
	image      *termimg.ImageWidget
	width      int
	height     int
	result     DetailResult
	refreshing bool
}

func RunCardDetail(ctx context.Context, card domain.Card, cfg config.Config, renderer images.Renderer, colors bool, needsRefresh bool, refresh RefreshFunc) (DetailResult, error) {
	model := &detailModel{
		ctx:      ctx,
		card:     card,
		cfg:      cfg,
		renderer: renderer,
		refresh:  refresh,
		styles:   theme.NewStyles(colors),
		status:   "Loaded from local data",
	}
	model.loadImageWidget()
	if needsRefresh {
		model.refreshing = true
		model.status = "Refreshing prices..."
	}
	p, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return DetailResult{}, err
	}
	finalModel := p.(*detailModel)
	return finalModel.result, nil
}

func (m *detailModel) Init() tea.Cmd {
	if !m.refreshing {
		return nil
	}
	return func() tea.Msg {
		card, err := m.refresh(m.ctx, m.card)
		return detailRefreshDoneMsg{card: card, err: err}
	}
}

func (m *detailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case detailRefreshDoneMsg:
		m.refreshing = false
		if msg.err != nil {
			m.status = "Refresh failed, showing cached data"
			return m, nil
		}
		m.card = msg.card
		m.loadImageWidget()
		m.status = "Updated just now"
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.cfg.SaveSearchedCardsDefault {
				m.result = DetailResult{Action: "add", Card: m.card}
			} else {
				m.result = DetailResult{Action: "close", Card: m.card}
			}
			return m, tea.Quit
		case "a":
			m.result = DetailResult{Action: "add", Card: m.card}
			return m, tea.Quit
		case "c":
			m.result = DetailResult{Action: "close", Card: m.card}
			return m, tea.Quit
		case "esc", "q":
			m.result = DetailResult{Action: "close", Card: m.card}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *detailModel) View() string {
	leftWidth, rightWidth, panelHeight := detailLayout(m.width, m.height)
	lines := append(viewmodel.DetailLines(m.card), "", "Status: "+m.status, "", renderActionRow(m.styles, m.cfg.SaveSearchedCardsDefault))
	details := m.styles.Card.Copy().Width(rightWidth).Height(panelHeight).Render(strings.Join(lines, "\n"))
	title := m.styles.Title.Render("Card Detail")
	imagePane := m.renderImagePane(leftWidth, panelHeight)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, imagePane, " ", details)
	view := lipgloss.NewStyle().Padding(1, 2).Render(title + "\n\n" + layout)
	return view + m.renderImageOverlay(leftWidth, panelHeight)
}

func renderActionRow(styles theme.Styles, addDefault bool) string {
	closeLabel := "C: Close"
	addLabel := "A: Add to collection"
	if addDefault {
		addLabel += " (Enter)"
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.Action.Render(closeLabel),
			" ",
			styles.Active.Render(addLabel),
		)
	} else {
		closeLabel += " (Enter)"
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.Active.Render(closeLabel),
			" ",
			styles.Action.Render(addLabel),
		)
	}
}

func detailLayout(width int, height int) (left int, right int, panelHeight int) {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	contentWidth := width - 5
	if contentWidth < 70 {
		contentWidth = 70
	}

	left = contentWidth / 2
	right = contentWidth - left - 1
	if left < 30 {
		left = 30
		right = contentWidth - left - 1
	}
	if right < 36 {
		right = 36
		left = contentWidth - right - 1
	}

	panelHeight = height - 8
	if panelHeight < 16 {
		panelHeight = 16
	}
	if panelHeight > 36 {
		panelHeight = 36
	}
	return left, right, panelHeight
}

func (m *detailModel) renderImagePane(width int, height int) string {
	content := ""
	switch {
	case m.image != nil:
		content = ""
	case m.imageErr != "":
		content = m.styles.Muted.Render(m.imageErr)
	default:
		content = m.styles.Muted.Render("Image unavailable")
	}
	return m.styles.Card.Copy().Padding(0).Width(width).Height(height).Render(content)
}

func (m *detailModel) renderImageOverlay(panelWidth int, panelHeight int) string {
	if m.image == nil {
		return ""
	}

	imageWidth := panelWidth - 2
	imageHeight := panelHeight - 2
	if imageWidth < 4 || imageHeight < 4 {
		return ""
	}

	m.image.SetSizeWithCorrection(imageWidth, imageHeight)
	rendered, err := m.image.Render()
	if err != nil {
		return ""
	}

	// Root view has padding(1,2), then title + blank line, then left panel border.
	const imageTop = 5
	const imageLeft = 4

	var b strings.Builder
	b.WriteString(m.renderer.ClearAllString())
	b.WriteString("\033[s")
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageTop, imageLeft))
	b.WriteString(rendered)
	b.WriteString("\033[u")
	return b.String()
}

func (m *detailModel) loadImageWidget() {
	m.image = nil
	m.imageErr = ""

	if !m.cfg.ImagePreviewsEnabled {
		m.imageErr = "Image previews disabled"
		return
	}
	if m.card.ImagePath == "" {
		m.imageErr = "No cached image"
		return
	}
	if _, err := os.Stat(m.card.ImagePath); err != nil {
		m.imageErr = "Image file missing"
		return
	}
	if m.renderer == nil || !m.renderer.Supported() {
		m.imageErr = "No supported terminal image protocol"
		return
	}

	widget, err := termimg.NewImageWidgetFromFile(m.card.ImagePath)
	if err != nil {
		m.imageErr = "Failed to load image widget"
		return
	}
	widget.SetProtocol(m.renderer.Protocol())
	m.image = widget
}
