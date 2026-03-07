package screens

import "github.com/charmbracelet/huh"

func MainMenu(theme *huh.Theme) (string, error) {
	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pokemon Card Value").
				Description("Pick what to do next. Use / to filter once the list is focused.").
				Options(
					huh.NewOption("Browse sets", "browse"),
					huh.NewOption("Settings", "settings"),
					huh.NewOption("Quit", "quit"),
				).
				Value(&choice),
		),
	).WithTheme(theme)
	return choice, form.Run()
}
