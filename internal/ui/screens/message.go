package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type messageModel struct {
	title       string
	description string
	styles      uitheme.Styles
}

func ShowMessage(title, description string, colors bool) error {
	_, err := tea.NewProgram(&messageModel{
		title:       title,
		description: description,
		styles:      uitheme.NewStyles(colors),
	}).Run()
	return err
}

func (m *messageModel) Init() tea.Cmd {
	return nil
}

func (m *messageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *messageModel) View() string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.Title.Render(m.title),
		"",
		m.description,
		"",
		m.styles.Muted.Render("Enter: Continue • Esc: Close"),
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}
