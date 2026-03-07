package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/Official-Husko/pkmn-tc-value/internal/bootstrap"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/screens"
	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
)

type App struct {
	container *bootstrap.Container
	theme     *huh.Theme
}

func New(container *bootstrap.Container) *App {
	return &App{
		container: container,
		theme:     uitheme.NewHuhTheme(container.Config.ColorsEnabled),
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.container.ImageCache.Validate(); err != nil {
		return err
	}
	if err := a.runStartupSync(ctx); err != nil {
		return err
	}

	for {
		choice, err := screens.MainMenu(a.theme)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		switch choice {
		case "browse":
			if err := a.runBrowse(ctx); err != nil {
				return err
			}
		case "settings":
			if err := a.runSettings(); err != nil {
				return err
			}
		case "quit":
			return nil
		default:
			return nil
		}
	}
}

func (a *App) runStartupSync(ctx context.Context) error {
	if !a.container.Config.StartupSyncEnabled {
		return nil
	}
	stats, err := screens.RunStartupSync(ctx, a.container.Config.ColorsEnabled, a.container.StartupSync.Run)
	if err != nil {
		if a.container.Store.HasData() {
			return screens.ShowMessage("Startup Sync Warning", err.Error(), a.theme)
		}
		return err
	}
	return screens.ShowStartupSummary(stats, a.theme)
}

func (a *App) runBrowse(ctx context.Context) error {
	sets, err := a.container.Sets.List()
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return screens.ShowMessage("No Data", "No sets are available yet. Run the tool again with startup sync enabled.", a.theme)
	}
	language, err := screens.PickLanguage(sets, a.theme)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	filteredSets := filterSetsByLanguage(sets, language)
	if len(filteredSets) == 0 {
		return screens.ShowMessage("No Sets", fmt.Sprintf("No sets are available for language %q.", language), a.theme)
	}

	setID, err := screens.PickSet(filteredSets, a.theme)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	set, ok, err := a.container.Sets.Get(setID)
	if err != nil {
		return err
	}
	if !ok {
		return screens.ShowMessage("Set Missing", "The selected set was not found in the local database.", a.theme)
	}
	if err := a.syncSetOnDemand(ctx, set); err != nil {
		if a.container.Store.HasData() {
			if msgErr := screens.ShowMessage("Set Sync Warning", err.Error(), a.theme); msgErr != nil {
				return msgErr
			}
			return nil
		} else {
			return err
		}
	}
	updatedSet, found, err := a.container.Sets.Get(setID)
	if err != nil {
		return err
	}
	if found {
		set = updatedSet
	}

	for {
		number, err := screens.LookupCardNumber(set, a.theme)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		card, ok, err := a.container.Cards.GetBySetAndNumber(set.ID, number)
		if err != nil {
			return err
		}
		if !ok {
			if err := screens.ShowMessage("Card Not Found", fmt.Sprintf("No card %q was found in %s.", number, set.Name), a.theme); err != nil {
				return err
			}
			continue
		}
		if err := a.showCardDetail(ctx, set, card); err != nil {
			return err
		}
	}
}

func (a *App) syncSetOnDemand(ctx context.Context, set domain.Set) error {
	cached, err := a.container.SetSync.IsSetCached(set.ID)
	if err != nil {
		return err
	}
	if cached {
		return nil
	}

	result, err := screens.RunSetDownload(ctx, set, a.container.Config.ColorsEnabled, func(ctx context.Context, report func(screens.SetDownloadProgress)) (screens.SetDownloadResult, error) {
		out, err := a.container.SetSync.SyncSet(ctx, set.ID, syncer.SetSyncOptions{
			SaveCardImages:  a.container.Config.SaveCardImages,
			SyncCardDetails: a.container.Config.SyncCardDetails,
			Config:          a.container.Config,
		}, func(progress syncer.SetSyncProgress) {
			report(screens.SetDownloadProgress{
				Stage:  progress.Stage,
				Status: progress.Status,
				Done:   progress.Done,
				Total:  progress.Total,
			})
		})
		if err != nil {
			return screens.SetDownloadResult{}, err
		}
		return screens.SetDownloadResult{
			SetName:       out.SetName,
			NewCards:      out.NewCards,
			UpdatedCards:  out.UpdatedCards,
			TotalCards:    out.TotalCards,
			ImagesSaved:   out.ImagesSaved,
			DetailsSynced: out.DetailsSynced,
			DetailsFailed: out.DetailsFailed,
		}, nil
	})
	if err != nil {
		return err
	}
	body := fmt.Sprintf(
		"%s\nNew cards: %d\nUpdated cards: %d\nTotal cards: %d\nImages saved: %d\nDetails synced: %d\nDetails failed: %d",
		result.SetName,
		result.NewCards,
		result.UpdatedCards,
		result.TotalCards,
		result.ImagesSaved,
		result.DetailsSynced,
		result.DetailsFailed,
	)
	return screens.ShowMessage("Set Ready", body, a.theme)
}

func (a *App) showCardDetail(ctx context.Context, set domain.Set, card domain.Card) error {
	needsRefresh := a.container.CardRefresh.NeedsRefresh(card, a.container.Config) ||
		(a.container.Config.ImagePreviewsEnabled && card.ImageURL != "" && card.ImagePath == "")
	result, err := screens.RunCardDetail(
		ctx,
		card,
		a.container.Config,
		a.container.Renderer,
		a.container.Config.ColorsEnabled,
		needsRefresh,
		func(ctx context.Context, card domain.Card) (domain.Card, error) {
			return a.container.CardRefresh.Refresh(ctx, card, set, a.container.Config)
		},
	)
	if err != nil {
		return err
	}
	if result.Action == "add" {
		if err := a.container.Collection.Add(result.Card.ID); err != nil {
			return err
		}
		return screens.ShowMessage("Saved", result.Card.Name+" was added to your collection.", a.theme)
	}
	return nil
}

func (a *App) runSettings() error {
	current := a.container.Config
	next, err := screens.EditSettings(a.container.Config, a.theme)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	if next == current {
		return nil
	}
	if err := config.Save(a.container.Paths.ConfigFile, next); err != nil {
		return err
	}
	a.container = bootstrap.New(next, a.container.Paths, a.container.Store)
	a.theme = uitheme.NewHuhTheme(next.ColorsEnabled)
	return screens.ShowMessage("Settings Saved", "The tool configuration was updated.", a.theme)
}

func filterSetsByLanguage(sets []domain.Set, language string) []domain.Set {
	want := normalizeLanguage(language)
	filtered := make([]domain.Set, 0, len(sets))
	for _, set := range sets {
		if normalizeLanguage(set.Language) == want {
			filtered = append(filtered, set)
		}
	}
	return filtered
}

func normalizeLanguage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return strings.ToLower(trimmed)
}
