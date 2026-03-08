package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type SelectOption struct {
	Label       string
	Description string
	Value       string
}

type selectDoneMsg struct {
	value    string
	canceled bool
}

type selectModel struct {
	title         string
	description   string
	styles        uitheme.Styles
	options       []SelectOption
	filtered      []int
	cursor        int
	maxRows       int
	filtering     bool
	filterActive  bool
	filterInput   textinput.Model
	viewportWidth int
	result        string
	canceled      bool
}

func runSelect(title string, description string, options []SelectOption, colors bool, filtering bool, height int) (string, bool, error) {
	rows := height
	if rows <= 0 {
		rows = 10
	}
	filterInput := textinput.New()
	filterInput.Prompt = "Filter: "
	filterInput.CharLimit = 64

	model := &selectModel{
		title:         title,
		description:   description,
		styles:        uitheme.NewStyles(colors),
		options:       options,
		filtered:      make([]int, 0, len(options)),
		maxRows:       rows,
		filtering:     filtering,
		filterInput:   filterInput,
		viewportWidth: 100,
	}
	model.applyFilter()

	out, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", false, err
	}
	final := out.(*selectModel)
	return final.result, final.canceled, nil
}

func (m *selectModel) Init() tea.Cmd {
	return nil
}

func (m *selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportWidth = msg.Width - 6
		if m.viewportWidth < 40 {
			m.viewportWidth = 40
		}
	case selectDoneMsg:
		m.result = msg.value
		m.canceled = msg.canceled
		return m, tea.Quit
	case tea.KeyMsg:
		if m.filterActive {
			switch msg.String() {
			case "enter":
				m.filterActive = false
				m.filterInput.Blur()
				return m, nil
			case "esc":
				m.filterActive = false
				m.filterInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, func() tea.Msg {
				return selectDoneMsg{canceled: true}
			}
		case "/":
			if m.filtering {
				m.filterActive = true
				m.filterInput.Focus()
				return m, nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "pgup":
			m.cursor -= m.maxRows
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += m.maxRows
			if m.cursor > len(m.filtered)-1 {
				m.cursor = len(m.filtered) - 1
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(m.filtered) - 1
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			choice := m.options[m.filtered[m.cursor]]
			return m, func() tea.Msg {
				return selectDoneMsg{value: choice.Value}
			}
		}
	}
	return m, nil
}

func (m *selectModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.filtered = m.filtered[:0]
	for idx, opt := range m.options {
		if query == "" {
			m.filtered = append(m.filtered, idx)
			continue
		}
		haystack := strings.ToLower(opt.Label + " " + opt.Description)
		if strings.Contains(haystack, query) {
			m.filtered = append(m.filtered, idx)
		}
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *selectModel) View() string {
	lines := make([]string, 0, m.maxRows+8)
	lines = append(lines, m.styles.Title.Render(m.title))
	lines = append(lines, m.styles.Muted.Render(m.description))
	lines = append(lines, "")

	if m.filtering {
		filterLabel := m.styles.Muted.Render(m.filterInput.View())
		if m.filterActive {
			filterLabel = m.styles.Label.Render(m.filterInput.View())
		}
		lines = append(lines, filterLabel)
		lines = append(lines, "")
	}

	if len(m.filtered) == 0 {
		lines = append(lines, m.styles.Muted.Render("No matching options."))
	} else {
		start := 0
		if m.cursor >= m.maxRows {
			start = m.cursor - m.maxRows + 1
		}
		end := start + m.maxRows
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for row := start; row < end; row++ {
			opt := m.options[m.filtered[row]]
			text := opt.Label
			if strings.TrimSpace(opt.Description) != "" {
				text += " " + m.styles.Muted.Render("· "+opt.Description)
			}
			if row == m.cursor {
				lines = append(lines, m.styles.Active.Render(text))
			} else {
				lines = append(lines, m.styles.Action.Render(text))
			}
		}
	}

	hints := "Enter: Select • Esc: Back • ↑/↓: Move"
	if m.filtering {
		hints += " • /: Filter"
	}
	lines = append(lines, "")
	lines = append(lines, m.styles.Muted.Render(hints))
	return lipgloss.NewStyle().Padding(1, 2).Width(m.viewportWidth).Render(strings.Join(lines, "\n"))
}
