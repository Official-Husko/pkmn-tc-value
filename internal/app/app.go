package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Official-Husko/pkmn-tc-value/internal/bootstrap"
)

type App struct {
	container *bootstrap.Container
}

func New(container *bootstrap.Container) *App {
	return &App{
		container: container,
	}
}

func (a *App) Run(ctx context.Context) error {
	model := newRootModel(ctx, a.container)
	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	fm := final.(*rootModel)
	a.container = fm.container
	return fm.fatalErr
}
