package screens

import (
	"github.com/charmbracelet/huh"
)

func ShowMessage(title, description string, theme *huh.Theme) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(title).Description(description).Next(true).NextLabel("Continue"),
		),
	).WithTheme(theme).Run()
}
