package theme

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	Title   lipgloss.Style
	Card    lipgloss.Style
	Label   lipgloss.Style
	Value   lipgloss.Style
	Muted   lipgloss.Style
	Success lipgloss.Style
	Warn    lipgloss.Style
	Action  lipgloss.Style
	Active  lipgloss.Style
}

func NewStyles(colors bool) Styles {
	if !colors {
		plain := lipgloss.NewStyle()
		return Styles{
			Title:   plain.Bold(true),
			Card:    plain.Border(lipgloss.RoundedBorder()).Padding(1),
			Label:   plain.Bold(true),
			Value:   plain,
			Muted:   plain,
			Success: plain,
			Warn:    plain,
			Action:  plain.Border(lipgloss.RoundedBorder()).Padding(0, 1),
			Active:  plain.Border(lipgloss.RoundedBorder()).Bold(true).Padding(0, 1),
		}
	}
	return Styles{
		Title:   lipgloss.NewStyle().Foreground(Gold).Bold(true),
		Card:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Blue).Padding(1),
		Label:   lipgloss.NewStyle().Foreground(Blue).Bold(true),
		Value:   lipgloss.NewStyle().Foreground(Cream),
		Muted:   lipgloss.NewStyle().Foreground(Slate),
		Success: lipgloss.NewStyle().Foreground(Green).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(Red).Bold(true),
		Action:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Slate).Padding(0, 1),
		Active:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Gold).Foreground(Ink).Background(Gold).Bold(true).Padding(0, 1),
	}
}

func NewHuhTheme(colors bool) *huh.Theme {
	if !colors {
		return huh.ThemeBase()
	}
	t := huh.ThemeBase()
	t.Focused.Title = t.Focused.Title.Foreground(Gold).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(Gold).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(Slate)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(Red)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(Red)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(Green)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(Green)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(Ink).Background(Gold).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(Cream).Background(Blue)
	t.Focused.Next = t.Focused.FocusedButton
	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}
