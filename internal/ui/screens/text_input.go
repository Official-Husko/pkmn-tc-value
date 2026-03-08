package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type inputDoneMsg struct {
	value    string
	canceled bool
}

type textInputModel struct {
	title       string
	description string
	input       textinput.Model
	styles      uitheme.Styles
	result      string
	canceled    bool
}

func runTextInput(title string, description string, initial string, colors bool) (string, bool, error) {
	in := textinput.New()
	in.SetValue(initial)
	in.Focus()
	in.Prompt = "> "

	model := &textInputModel{
		title:       title,
		description: description,
		input:       in,
		styles:      uitheme.NewStyles(colors),
	}

	out, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", false, err
	}
	final := out.(*textInputModel)
	return final.result, final.canceled, nil
}

func (m *textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case inputDoneMsg:
		m.result = msg.value
		m.canceled = msg.canceled
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, func() tea.Msg {
				return inputDoneMsg{canceled: true}
			}
		case "enter":
			return m, func() tea.Msg {
				return inputDoneMsg{value: m.input.Value()}
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *textInputModel) View() string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.Title.Render(m.title),
		m.styles.Muted.Render(m.description),
		"",
		m.input.View(),
		"",
		m.styles.Muted.Render("Enter: Confirm • Esc: Back"),
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}
