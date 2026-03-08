package screens

func MainMenu(colors bool) (string, bool, error) {
	return runSelect(
		"Pokemon Card Value",
		"Pick what to do next.",
		[]SelectOption{
			{Label: "Browse sets", Value: "browse"},
			{Label: "Settings", Value: "settings"},
			{Label: "Quit", Value: "quit"},
		},
		colors,
		false,
		8,
	)
}
