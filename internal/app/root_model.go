package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Official-Husko/pkmn-tc-value/internal/bootstrap"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	trackerpricing "github.com/Official-Husko/pkmn-tc-value/internal/pricing/pokemonpricetracker"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/viewmodel"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
	_ "golang.org/x/image/webp"
)

const imageCompatTestURL = "https://tcgplayer-cdn.tcgplayer.com/product/567569_in_800x800.jpg"
const appDisplayName = "Pokémon Trading Card Value & Collection Tracker"

var hotkeyActionSpecs = []hotkeyActionSpec{
	{ID: "quit", Label: "Quit app", Description: "Global hard exit key"},
	{ID: "back", Label: "Back / close", Description: "Go back from current screen"},
	{ID: "confirm", Label: "Confirm", Description: "Select current option"},
	{ID: "filter", Label: "Open filter", Description: "Focus filter input in list screens"},
	{ID: "set_jump_id", Label: "Set jump by ID", Description: "Open set-id jump input in set list"},
	{ID: "move_up", Label: "Move up", Description: "Move selection up"},
	{ID: "move_down", Label: "Move down", Description: "Move selection down"},
	{ID: "page_up", Label: "Page up", Description: "Move one page up"},
	{ID: "page_down", Label: "Page down", Description: "Move one page down"},
	{ID: "go_top", Label: "Go top", Description: "Jump to first item"},
	{ID: "go_bottom", Label: "Go bottom", Description: "Jump to last item"},
	{ID: "card_add", Label: "Card add", Description: "Add selected card to collection"},
	{ID: "card_close", Label: "Card close", Description: "Close card detail"},
	{ID: "card_left", Label: "Card action left", Description: "Move action to left button"},
	{ID: "card_right", Label: "Card action right", Description: "Move action to right button"},
	{ID: "main_browse", Label: "Main: browse sets", Description: "Open browse flow from main menu"},
	{ID: "main_settings", Label: "Main: settings", Description: "Open settings from main menu"},
	{ID: "main_quit", Label: "Main: quit", Description: "Quit directly from main menu"},
}

type uiMode int

const (
	modeStartupSync uiMode = iota
	modeMenu
	modeInput
	modeMessage
	modeSetSync
	modeCardDetail
	modeBusy
)

type menuKind string

const (
	menuMain             menuKind = "main"
	menuLanguage         menuKind = "language"
	menuSet              menuKind = "set"
	menuSettings         menuKind = "settings"
	menuDatabaseActions  menuKind = "database_actions"
	menuBuildFullConfirm menuKind = "build_full_confirm"
	menuHotkeys          menuKind = "hotkeys"
	menuAPIKeys          menuKind = "api_keys"
	menuSettingBool      menuKind = "setting_bool"
	menuImageCompat      menuKind = "image_compat"
	menuImageCompatApply menuKind = "image_compat_apply"
)

type inputKind string

const (
	inputCardLookup inputKind = "card_lookup"
	inputSetJumpID  inputKind = "set_jump_id"
	inputSettingInt inputKind = "setting_int"
	inputSettingStr inputKind = "setting_str"
)

type nextAction int

const (
	nextNone nextAction = iota
	nextQuit
	nextMainMenu
	nextLanguageMenu
	nextSetMenu
	nextCardLookup
	nextSettingsMenu
	nextCurrentInput
)

type menuOption struct {
	Label       string
	Description string
	Value       string
}

type hotkeyActionSpec struct {
	ID          string
	Label       string
	Description string
}

type mainMenuSnapshot struct {
	SetCount          int
	CardCount         int
	LanguageCount     int
	CollectionEntries int
	CollectionCards   int
	LastSync          string
	LastSyncAt        *time.Time
	CatalogProvider   string
	PriceProvider     string
}

type startupProgressMsg struct {
	progress syncer.StartupProgress
}

type startupDoneMsg struct {
	stats syncer.Stats
	err   error
}

type setSyncProgressMsg struct {
	progress syncer.SetSyncProgress
}

type setSyncDoneMsg struct {
	result syncer.SetSyncResult
	err    error
}

type cardRefreshDoneMsg struct {
	card domain.Card
	err  error
}

type imageCompatDoneMsg struct {
	rendered  string
	renderErr string
	err       error
}

type fullBuildProgressMsg struct {
	status string
}

type fullBuildDoneMsg struct {
	summary string
	err     error
}

type uiTickMsg struct{}

type rootModel struct {
	ctx       context.Context
	container *bootstrap.Container

	mode     uiMode
	fatalErr error
	version  string

	width  int
	height int

	spinner spinner.Model

	startupProgress   syncer.StartupProgress
	startupProgressCh chan syncer.StartupProgress
	startupDoneCh     chan startupDoneMsg

	setSyncProgress     syncer.SetSyncProgress
	setSyncProgressCh   chan syncer.SetSyncProgress
	setSyncDoneCh       chan setSyncDoneMsg
	fullBuildProgressCh chan string
	fullBuildDoneCh     chan fullBuildDoneMsg
	startupReveal       int
	startupRevealGoal   int

	messageTitle string
	messageBody  string
	messageNext  nextAction

	menuKind          menuKind
	menuTitle         string
	menuDescription   string
	menuOptions       []menuOption
	menuFiltered      []int
	menuCursor        int
	menuCursorVisual  float64
	menuPrevCursor    int
	menuTrailFrames   int
	menuAnimFrame     int
	menuMaxRows       int
	menuFilterEnabled bool
	menuFilterActive  bool
	menuFilter        textinput.Model
	menuCancel        nextAction
	menuSettingKey    string

	inputKind        inputKind
	inputTitle       string
	inputDescription string
	input            textinput.Model
	inputCancel      nextAction
	inputSettingKey  string
	inputMin         int
	inputMax         int
	inputAllowBlank  bool
	inputError       string

	allSets          []domain.Set
	filteredSets     []domain.Set
	selectedLanguage string
	selectedSet      domain.Set
	card             domain.Card

	cardStatus     string
	cardRefreshing bool
	cardImageReady bool
	cardImageErr   string
	cardSelected   int

	settingsDraft config.Config
	mainSnapshot  mainMenuSnapshot

	busyTitle  string
	busyStatus string

	statusPulseFrame int
	stripeFrame      int
	uiTickScheduled  bool
}

func newRootModel(ctx context.Context, container *bootstrap.Container) *rootModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	if container.Config.ColorsEnabled {
		sp.Style = lipgloss.NewStyle().Foreground(uitheme.Gold)
	}

	filterInput := textinput.New()
	filterInput.Prompt = "Filter: "
	filterInput.CharLimit = 64

	return &rootModel{
		ctx:           ctx,
		container:     container,
		mode:          modeMenu,
		spinner:       sp,
		menuFilter:    filterInput,
		menuMaxRows:   10,
		version:       detectAppVersion(),
		settingsDraft: container.Config,
	}
}

func (m *rootModel) Init() tea.Cmd {
	if err := m.container.ImageCache.Validate(); err != nil {
		m.fatalErr = err
		return tea.Quit
	}

	m.mode = modeStartupSync
	m.startupReveal = 1
	m.startupRevealGoal = 1
	m.uiTickScheduled = true
	return tea.Batch(m.spinner.Tick, m.startStartupSyncCmd(), uiTickCmd())
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.spinnerActive() {
			cmds = append(cmds, cmd)
		}
	case startupProgressMsg:
		m.startupProgress = msg.progress
		m.updateStartupRevealGoal()
		cmds = append(cmds, waitStartupProgress(m.startupProgressCh))
	case startupDoneMsg:
		if msg.err != nil {
			if m.container.Store.HasData() {
				m.openMessage("Startup Sync Warning", msg.err.Error(), nextMainMenu)
			} else {
				m.fatalErr = msg.err
				return m, tea.Quit
			}
			break
		}
		body := fmt.Sprintf("New sets: %d\nUpdated sets: %d", msg.stats.NewSets, msg.stats.UpdatedSets)
		m.openMessage("Startup Sync Complete", body, nextMainMenu)
	case setSyncProgressMsg:
		m.setSyncProgress = msg.progress
		cmds = append(cmds, waitSetSyncProgress(m.setSyncProgressCh))
	case setSyncDoneMsg:
		if msg.err != nil {
			if m.container.Store.HasData() {
				m.openMessage("Set Sync Warning", msg.err.Error(), nextMainMenu)
			} else {
				m.fatalErr = msg.err
				return m, tea.Quit
			}
			break
		}
		if updated, ok, err := m.container.Sets.Get(msg.result.SetID); err == nil && ok {
			m.selectedSet = updated
		}
		body := fmt.Sprintf(
			"%s\nNew cards: %d\nUpdated cards: %d\nTotal cards: %d\nImages saved: %d\nDetails synced: %d\nDetails failed: %d",
			msg.result.SetName,
			msg.result.NewCards,
			msg.result.UpdatedCards,
			msg.result.TotalCards,
			msg.result.ImagesSaved,
			msg.result.DetailsSynced,
			msg.result.DetailsFailed,
		)
		m.openMessage("Set Ready", body, nextCardLookup)
	case cardRefreshDoneMsg:
		m.cardRefreshing = false
		if msg.err != nil {
			reason := strings.TrimSpace(msg.err.Error())
			if reason == "" {
				m.cardStatus = "Refresh failed, showing cached data"
			} else {
				if len(reason) > 120 {
					reason = reason[:117] + "..."
				}
				m.cardStatus = "Refresh failed: " + reason
			}
			break
		}
		m.card = msg.card
		m.loadCardImageWidget()
		m.cardStatus = "Updated just now"
	case imageCompatDoneMsg:
		if msg.err != nil {
			m.openMessage("Image Compatibility", msg.err.Error(), nextSettingsMenu)
			break
		}
		desc := "If you can see the sample image below, choose Visible.\n\n" + msg.rendered
		if msg.renderErr != "" {
			desc += "\n\nRenderer error: " + msg.renderErr
		}
		m.setMenu(
			menuImageCompatApply,
			"Was the image visible?",
			desc,
			[]menuOption{
				{Label: "Visible", Value: "visible"},
				{Label: "Not visible", Value: "not_visible"},
			},
			false,
			8,
			nextSettingsMenu,
		)
	case fullBuildProgressMsg:
		m.busyStatus = msg.status
		cmds = append(cmds, waitFullBuildProgress(m.fullBuildProgressCh))
	case fullBuildDoneMsg:
		if msg.err != nil {
			m.openMessage("Build Full DB Failed", msg.err.Error(), nextSettingsMenu)
			break
		}
		m.openMessage("Build Full DB Complete", msg.summary, nextSettingsMenu)
	case uiTickMsg:
		m.uiTickScheduled = false
		m.stepAnimations()
		m.refreshMainMenuClock()
	case tea.KeyMsg:
		if quit := m.globalQuitKey(msg); quit != nil {
			return m, quit
		}
		var cmd tea.Cmd
		switch m.mode {
		case modeMenu:
			cmd = m.updateMenu(msg)
		case modeInput:
			cmd = m.updateInput(msg)
		case modeMessage:
			cmd = m.updateMessage(msg)
		case modeCardDetail:
			cmd = m.updateCardDetail(msg)
		case modeStartupSync, modeSetSync, modeBusy:
			// no-op
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	m.maybeScheduleUITick(&cmds)

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *rootModel) View() string {
	var content string
	switch m.mode {
	case modeStartupSync:
		content = m.viewStartupSync()
	case modeSetSync:
		content = m.viewSetSync()
	case modeMenu:
		content = m.viewMenu()
	case modeInput:
		content = m.viewInput()
	case modeMessage:
		content = m.viewMessage()
	case modeCardDetail:
		content = m.viewCardDetail()
	case modeBusy:
		content = m.viewBusy()
	default:
		content = "Loading..."
	}
	content = m.fitToViewport(content)
	if m.mode != modeCardDetail && m.container.Renderer != nil {
		return m.container.Renderer.ClearAllString() + content
	}
	return content
}

func (m *rootModel) fitToViewport(content string) string {
	lines := strings.Split(content, "\n")

	if m.width > 0 {
		maxWidth := m.width - 1
		if maxWidth < 1 {
			maxWidth = 1
		}
		for i := range lines {
			if lipgloss.Width(lines[i]) > maxWidth {
				lines[i] = ansi.Truncate(lines[i], maxWidth, "")
			}
		}
	}

	if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m *rootModel) globalQuitKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case m.keyMatch(msg, "quit", "ctrl+c"):
		return tea.Quit
	}
	return nil
}

func (m *rootModel) spinnerActive() bool {
	return m.mode == modeStartupSync || m.mode == modeSetSync || m.mode == modeBusy
}

func uiTickCmd() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg {
		return uiTickMsg{}
	})
}

func (m *rootModel) maybeScheduleUITick(cmds *[]tea.Cmd) {
	if m.uiTickScheduled || !m.animationsActive() {
		return
	}
	m.uiTickScheduled = true
	*cmds = append(*cmds, uiTickCmd())
}

func (m *rootModel) animationsActive() bool {
	// Keep UI micro-animations active across screens so shared header elements
	// (status pulse + animated stripe) stay alive even outside menus.
	return true
}

func (m *rootModel) stepAnimations() {
	m.statusPulseFrame = (m.statusPulseFrame + 1) % 24
	m.stripeFrame++
	if m.stripeFrame > 1000000 {
		m.stripeFrame = 0
	}

	if m.mode == modeMenu {
		m.menuAnimFrame++
		if m.menuAnimFrame > 100000 {
			m.menuAnimFrame = 0
		}
		if m.menuTrailFrames > 0 {
			m.menuTrailFrames--
		}
		target := float64(m.menuCursor)
		delta := target - m.menuCursorVisual
		if math.Abs(delta) < 0.05 {
			m.menuCursorVisual = target
		} else {
			m.menuCursorVisual += delta * 0.35
		}
	}

	if m.mode == modeStartupSync && m.startupReveal < m.startupRevealGoal {
		m.startupReveal++
	}
}

func (m *rootModel) updateStartupRevealGoal() {
	goal := 2 // sets + status
	if m.startupProgress.CardsTotal > 0 {
		goal++
	}
	if strings.TrimSpace(m.startupProgress.CurrentSet) != "" {
		goal++
	}
	if goal < 1 {
		goal = 1
	}
	m.startupRevealGoal = goal
	if m.startupReveal < 1 {
		m.startupReveal = 1
	}
	if m.startupReveal > m.startupRevealGoal {
		m.startupReveal = m.startupRevealGoal
	}
}

func (m *rootModel) styles() uitheme.Styles {
	return uitheme.NewStyles(m.container.Config.ColorsEnabled)
}

func (m *rootModel) startStartupSyncCmd() tea.Cmd {
	m.startupProgress = syncer.StartupProgress{Stage: "sets", Status: "Fetching set list"}
	m.startupReveal = 1
	m.updateStartupRevealGoal()
	m.startupProgressCh = make(chan syncer.StartupProgress, 1024)
	m.startupDoneCh = make(chan startupDoneMsg, 1)
	go func() {
		reportProgress := func(p syncer.StartupProgress) {
			select {
			case m.startupProgressCh <- p:
			default:
			}
		}
		stats, err := m.container.StartupSync.Run(m.ctx, reportProgress)
		if err == nil {
			switch {
			case m.container.Config.DownloadAllImagesOnStartup:
				err = m.prefetchSetDataOnStartup(reportProgress, true)
			case m.container.Config.PrefetchCardMetadataOnStartup:
				err = m.prefetchSetDataOnStartup(reportProgress, false)
			}
		}
		m.startupDoneCh <- startupDoneMsg{stats: stats, err: err}
		close(m.startupProgressCh)
	}()
	return tea.Batch(waitStartupProgress(m.startupProgressCh), waitStartupDone(m.startupDoneCh))
}

func (m *rootModel) prefetchSetDataOnStartup(report func(syncer.StartupProgress), withImages bool) error {
	sets, err := m.container.Sets.List()
	if err != nil {
		return err
	}
	stageName := "metadata"
	if withImages {
		stageName = "images"
	}
	if len(sets) == 0 {
		report(syncer.StartupProgress{Stage: stageName, Status: "No sets available for startup prefetch"})
		return nil
	}

	totalSets := len(sets)
	estimatedCards := 0
	for _, set := range sets {
		if set.Total > 0 {
			estimatedCards += set.Total
		}
	}
	doneCards := 0

	for i, set := range sets {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		default:
		}

		report(syncer.StartupProgress{
			Stage:      stageName,
			Status:     fmt.Sprintf("Prefetching %s for %s (%d/%d)", stageName, set.Name, i+1, totalSets),
			SetsDone:   i,
			SetsTotal:  totalSets,
			CardsDone:  doneCards,
			CardsTotal: estimatedCards,
			CurrentSet: set.Name,
		})

		var currentSetDone int
		var currentSetTotal int
		_, err := m.container.SetSync.SyncSet(m.ctx, set.ID, syncer.SetSyncOptions{
			ImageCaching:    withImages,
			SyncCardDetails: false,
			Config:          m.container.Config,
		}, func(p syncer.SetSyncProgress) {
			if p.Done < 0 || p.Total < 0 {
				return
			}
			currentSetDone = p.Done
			currentSetTotal = p.Total
			total := estimatedCards
			if total < doneCards+p.Total {
				total = doneCards + p.Total
			}
			report(syncer.StartupProgress{
				Stage:      stageName,
				Status:     fmt.Sprintf("Prefetching %s for %s", stageName, set.Name),
				SetsDone:   i,
				SetsTotal:  totalSets,
				CardsDone:  doneCards + p.Done,
				CardsTotal: total,
				CurrentSet: set.Name,
			})
		})
		if err != nil {
			return err
		}

		if currentSetTotal > 0 {
			doneCards += currentSetDone
		} else if set.Total > 0 {
			doneCards += set.Total
		}

		if estimatedCards < doneCards {
			estimatedCards = doneCards
		}
		report(syncer.StartupProgress{
			Stage:      stageName,
			Status:     fmt.Sprintf("Finished %s prefetch for %s (%d/%d)", stageName, set.Name, i+1, totalSets),
			SetsDone:   i + 1,
			SetsTotal:  totalSets,
			CardsDone:  doneCards,
			CardsTotal: estimatedCards,
			CurrentSet: set.Name,
		})
	}

	report(syncer.StartupProgress{
		Stage:      stageName,
		Status:     fmt.Sprintf("Startup %s prefetch complete", stageName),
		SetsDone:   totalSets,
		SetsTotal:  totalSets,
		CardsDone:  doneCards,
		CardsTotal: estimatedCards,
	})
	return nil
}

func (m *rootModel) startSetSyncCmd() tea.Cmd {
	m.mode = modeSetSync
	m.setSyncProgress = syncer.SetSyncProgress{Stage: "cards", Status: "Starting set sync"}
	m.setSyncProgressCh = make(chan syncer.SetSyncProgress, 256)
	m.setSyncDoneCh = make(chan setSyncDoneMsg, 1)
	setID := m.selectedSet.ID

	go func() {
		result, err := m.container.SetSync.SyncSet(m.ctx, setID, syncer.SetSyncOptions{
			ImageCaching:    m.container.Config.ImageCaching,
			SyncCardDetails: m.container.Config.SyncCardDetails,
			Config:          m.container.Config,
		}, func(progress syncer.SetSyncProgress) {
			select {
			case m.setSyncProgressCh <- progress:
			default:
			}
		})
		m.setSyncDoneCh <- setSyncDoneMsg{result: result, err: err}
		close(m.setSyncProgressCh)
	}()

	return tea.Batch(m.spinner.Tick, waitSetSyncProgress(m.setSyncProgressCh), waitSetSyncDone(m.setSyncDoneCh))
}

func (m *rootModel) refreshCardCmd(card domain.Card) tea.Cmd {
	set := m.selectedSet
	cfg := m.container.Config
	return func() tea.Msg {
		updated, err := m.container.CardRefresh.Refresh(m.ctx, card, set, cfg)
		return cardRefreshDoneMsg{card: updated, err: err}
	}
}

func (m *rootModel) imageCompatCmd() tea.Cmd {
	renderer := m.container.Renderer
	return func() tea.Msg {
		if renderer == nil {
			return imageCompatDoneMsg{err: fmt.Errorf("renderer unavailable")}
		}

		path, err := downloadCompatibilityImage(imageCompatTestURL)
		if err != nil {
			return imageCompatDoneMsg{err: err}
		}
		defer os.Remove(path)

		rendered, renderErr := renderer.Render(path, 32, 12)
		out := imageCompatDoneMsg{rendered: rendered}
		if renderErr != nil {
			out.renderErr = renderErr.Error()
		}
		if strings.TrimSpace(out.rendered) == "" {
			out.rendered = "[image unavailable]"
		}
		return out
	}
}

func waitStartupProgress(ch <-chan syncer.StartupProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return nil
		}
		return startupProgressMsg{progress: progress}
	}
}

func waitStartupDone(ch <-chan startupDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func waitSetSyncProgress(ch <-chan syncer.SetSyncProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return nil
		}
		return setSyncProgressMsg{progress: progress}
	}
}

func waitSetSyncDone(ch <-chan setSyncDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func waitFullBuildProgress(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		status, ok := <-ch
		if !ok {
			return nil
		}
		return fullBuildProgressMsg{status: status}
	}
}

func waitFullBuildDone(ch <-chan fullBuildDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m *rootModel) openMessage(title string, body string, next nextAction) {
	m.mode = modeMessage
	m.messageTitle = title
	m.messageBody = body
	m.messageNext = next
}

func (m *rootModel) setMenu(kind menuKind, title, description string, options []menuOption, filtering bool, maxRows int, cancel nextAction) {
	in := textinput.New()
	in.Prompt = "Filter: "
	in.CharLimit = 64

	m.mode = modeMenu
	m.menuKind = kind
	m.menuTitle = title
	m.menuDescription = description
	m.menuOptions = options
	m.menuFiltered = make([]int, 0, len(options))
	m.menuCursor = 0
	m.menuCursorVisual = 0
	m.menuPrevCursor = 0
	m.menuTrailFrames = 0
	m.menuMaxRows = maxRows
	m.menuFilterEnabled = filtering
	m.menuFilterActive = false
	m.menuFilter = in
	m.menuCancel = cancel
	m.applyMenuFilter()
}

func (m *rootModel) applyMenuFilter() {
	query := strings.ToLower(strings.TrimSpace(m.menuFilter.Value()))
	m.menuFiltered = m.menuFiltered[:0]
	for idx, opt := range m.menuOptions {
		if query == "" {
			m.menuFiltered = append(m.menuFiltered, idx)
			continue
		}
		haystack := strings.ToLower(opt.Label + " " + opt.Description + " " + opt.Value)
		if m.menuKind == menuSet {
			if set, ok := m.findFilteredSetByID(opt.Value); ok {
				haystack += " " + strings.ToLower(strings.TrimSpace(set.ID))
				haystack += " " + strings.ToLower(strings.TrimSpace(set.SetCode))
				haystack += " " + strings.ToLower(strings.TrimSpace(set.Name))
				haystack += " " + strings.ToLower(strings.TrimSpace(set.Series))
			}
		}
		if strings.Contains(haystack, query) {
			m.menuFiltered = append(m.menuFiltered, idx)
		}
	}
	if len(m.menuFiltered) == 0 {
		m.menuCursor = 0
		m.menuCursorVisual = 0
		m.menuTrailFrames = 0
		return
	}
	m.moveMenuCursor(m.menuCursor)
}

func (m *rootModel) moveMenuCursor(next int) {
	if len(m.menuFiltered) == 0 {
		m.menuCursor = 0
		m.menuCursorVisual = 0
		m.menuTrailFrames = 0
		return
	}
	if next < 0 {
		next = 0
	}
	max := len(m.menuFiltered) - 1
	if next > max {
		next = max
	}
	if next == m.menuCursor {
		return
	}
	m.menuPrevCursor = m.menuCursor
	m.menuCursor = next
	m.menuTrailFrames = 4
}

func (m *rootModel) updateMenu(msg tea.KeyMsg) tea.Cmd {
	if m.menuFilterActive {
		switch {
		case m.keyMatch(msg, "back", "esc"), m.keyMatch(msg, "confirm", "enter"):
			m.menuFilterActive = false
			m.menuFilter.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.menuFilter, cmd = m.menuFilter.Update(msg)
		m.applyMenuFilter()
		return cmd
	}

	if m.menuKind == menuMain {
		switch {
		case m.keyMatch(msg, "main_browse", "b"):
			return m.onMenuSelect("browse")
		case m.keyMatch(msg, "main_settings", "s"):
			return m.onMenuSelect("settings")
		case m.keyMatch(msg, "main_quit", "q"):
			return m.onMenuSelect("quit")
		}
	}

	switch {
	case m.keyMatch(msg, "back", "q", "esc"):
		return m.runNextAction(m.menuCancel)
	case m.keyMatch(msg, "filter", "/"):
		if m.menuFilterEnabled {
			m.menuFilterActive = true
			m.menuFilter.Focus()
		}
	case m.keyMatch(msg, "set_jump_id", "i"):
		if m.menuKind == menuSet {
			m.openSetJumpInput()
		}
	case m.keyMatch(msg, "move_up", "up", "k"):
		m.moveMenuCursor(m.menuCursor - 1)
	case m.keyMatch(msg, "move_down", "down", "j"):
		m.moveMenuCursor(m.menuCursor + 1)
	case m.keyMatch(msg, "page_up", "pgup"):
		m.moveMenuCursor(m.menuCursor - m.menuMaxRows)
	case m.keyMatch(msg, "page_down", "pgdown"):
		m.moveMenuCursor(m.menuCursor + m.menuMaxRows)
	case m.keyMatch(msg, "go_top", "home"):
		m.moveMenuCursor(0)
	case m.keyMatch(msg, "go_bottom", "end"):
		m.moveMenuCursor(len(m.menuFiltered) - 1)
	case m.keyMatch(msg, "confirm", "enter"):
		if len(m.menuFiltered) == 0 {
			return nil
		}
		selection := m.menuOptions[m.menuFiltered[m.menuCursor]]
		return m.onMenuSelect(selection.Value)
	}
	return nil
}

func (m *rootModel) onMenuSelect(value string) tea.Cmd {
	switch m.menuKind {
	case menuMain:
		switch value {
		case "browse":
			return m.openLanguageMenu()
		case "settings":
			m.settingsDraft = cloneConfig(m.container.Config)
			m.openSettingsMenu()
			return nil
		case "quit":
			return tea.Quit
		}
	case menuLanguage:
		m.selectedLanguage = value
		return m.openSetMenuForLanguage(value)
	case menuSet:
		return m.openSetByID(value)
	case menuSettings:
		return m.onSettingsMenuSelect(value)
	case menuDatabaseActions:
		switch value {
		case "db_clear_catalog":
			return m.clearCatalogData()
		case "db_clear_collection":
			return m.clearCollectionData()
		case "db_build_full":
			m.openBuildFullConfirmMenu()
			return nil
		case "db_back":
			m.openSettingsMenu()
			return nil
		}
	case menuBuildFullConfirm:
		switch value {
		case "build_full_continue":
			return m.startFullBuildCmd()
		case "build_full_cancel":
			m.openDatabaseActionsMenu()
			return nil
		}
	case menuHotkeys:
		switch {
		case value == "hotkey_back":
			m.openSettingsMenu()
			return nil
		case value == "hotkey_reset":
			m.settingsDraft.Hotkeys = config.DefaultHotkeys()
			m.openHotkeysMenu()
			return nil
		case strings.HasPrefix(value, "hotkey_edit:"):
			action := strings.TrimPrefix(value, "hotkey_edit:")
			spec, ok := findHotkeyActionSpec(action)
			if !ok {
				m.openHotkeysMenu()
				return nil
			}
			current := ""
			if m.settingsDraft.Hotkeys != nil {
				current = m.settingsDraft.Hotkeys[action]
			}
			if strings.TrimSpace(current) == "" {
				current = config.DefaultHotkeys()[action]
			}
			m.openTextSetting(
				"hotkey:"+action,
				"Hotkey: "+spec.Label,
				"Set key token (examples: enter, esc, /, k, up, pgdown, ctrl+c). Type 'default' to reset this action.",
				current,
				false,
			)
			return nil
		}
	case menuAPIKeys:
		switch {
		case value == "api_add":
			m.openTextSetting("api_add_key", "Add API Key", "Paste a Pokewallet API key.", "", false)
			return nil
		case value == "api_back":
			m.openSettingsMenu()
			return nil
		case strings.HasPrefix(value, "api_remove:"):
			indexRaw := strings.TrimPrefix(value, "api_remove:")
			index, err := strconv.Atoi(indexRaw)
			if err != nil {
				m.openMessage("API Keys", "Invalid API key index.", nextSettingsMenu)
				return nil
			}
			if index < 0 || index >= len(m.settingsDraft.APIKeys) {
				m.openMessage("API Keys", "API key index out of range.", nextSettingsMenu)
				return nil
			}
			next := append([]string{}, m.settingsDraft.APIKeys[:index]...)
			next = append(next, m.settingsDraft.APIKeys[index+1:]...)
			m.settingsDraft.APIKeys = next
			m.openAPIKeysMenu()
			return nil
		}
	case menuSettingBool:
		m.applyBoolSetting(m.menuSettingKey, value == "true")
		m.openSettingsMenu()
		return nil
	case menuImageCompatApply:
		visible := value == "visible"
		m.settingsDraft.ImagePreviewsEnabled = visible
		m.settingsDraft.ImageCaching = visible
		m.openSettingsMenu()
		return nil
	}
	return nil
}

func (m *rootModel) runNextAction(action nextAction) tea.Cmd {
	switch action {
	case nextQuit:
		return tea.Quit
	case nextMainMenu:
		return m.openMainMenu()
	case nextLanguageMenu:
		return m.openLanguageMenu()
	case nextSetMenu:
		return m.openSetMenuForLanguage(m.selectedLanguage)
	case nextCardLookup:
		m.openCardLookupInput()
		return nil
	case nextSettingsMenu:
		m.openSettingsMenu()
		return nil
	case nextCurrentInput:
		m.mode = modeInput
		return nil
	default:
		return nil
	}
}

func (m *rootModel) updateInput(msg tea.KeyMsg) tea.Cmd {
	switch {
	case m.keyMatch(msg, "back", "esc"):
		return m.runNextAction(m.inputCancel)
	case m.keyMatch(msg, "confirm", "enter"):
		return m.onInputSubmit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *rootModel) setInput(kind inputKind, title, description, initial string, cancel nextAction) {
	in := textinput.New()
	in.Prompt = "> "
	in.SetValue(initial)
	in.Focus()
	in.CharLimit = 256

	m.mode = modeInput
	m.inputKind = kind
	m.inputTitle = title
	m.inputDescription = description
	m.input = in
	m.inputCancel = cancel
	m.inputError = ""
}

func (m *rootModel) onInputSubmit() tea.Cmd {
	value := strings.TrimSpace(m.input.Value())
	switch m.inputKind {
	case inputCardLookup:
		card, ok, err := m.container.Cards.GetBySetAndNumber(m.selectedSet.ID, value)
		if err != nil {
			m.fatalErr = err
			return tea.Quit
		}
		if !ok {
			m.openMessage("Card Not Found", fmt.Sprintf("No card %q was found in %s.", value, m.selectedSet.Name), nextCardLookup)
			return nil
		}
		return m.openCardDetail(card)
	case inputSetJumpID:
		return m.openSetByID(value)
	case inputSettingInt:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			m.inputError = "Invalid number. Enter digits only."
			return nil
		}
		if parsed < m.inputMin || parsed > m.inputMax {
			m.inputError = fmt.Sprintf("Value must be between %d and %d.", m.inputMin, m.inputMax)
			return nil
		}
		m.applyIntSetting(m.inputSettingKey, parsed)
		m.openSettingsMenu()
		return nil
	case inputSettingStr:
		if value == "" && !m.inputAllowBlank {
			m.inputError = "This value cannot be blank."
			return nil
		}
		m.applyTextSetting(m.inputSettingKey, value)
		if m.inputSettingKey == "api_add_key" || strings.HasPrefix(m.inputSettingKey, "hotkey:") {
			return nil
		}
		m.openSettingsMenu()
		return nil
	}
	return nil
}

func (m *rootModel) updateMessage(msg tea.KeyMsg) tea.Cmd {
	switch {
	case m.keyMatch(msg, "confirm", "enter"), m.keyMatch(msg, "back", "esc", "q"):
		return m.runNextAction(m.messageNext)
	}
	return nil
}

func (m *rootModel) openMainMenu() tea.Cmd {
	m.refreshMainMenuSnapshot()
	m.setMenu(
		menuMain,
		appDisplayName,
		"Quickly browse sets, lookup cards, and track your collection.",
		[]menuOption{
			{Label: "Browse sets", Description: "Open language and set picker", Value: "browse"},
			{Label: "Settings", Description: "Tool and sync preferences", Value: "settings"},
			{Label: "Quit", Description: "Exit the application", Value: "quit"},
		},
		false,
		8,
		nextQuit,
	)
	m.menuAnimFrame = 0
	return nil
}

func (m *rootModel) refreshMainMenuSnapshot() {
	snapshot := mainMenuSnapshot{
		LastSync: "never",
	}

	err := m.container.Store.Read(func(db *store.DB) error {
		snapshot.SetCount = len(db.Sets)
		languages := make(map[string]struct{}, len(db.Sets))
		for _, set := range db.Sets {
			if lang := strings.TrimSpace(set.Language); lang != "" {
				languages[strings.ToLower(lang)] = struct{}{}
			}
		}
		snapshot.LanguageCount = len(languages)

		for _, cards := range db.CardsBySet {
			snapshot.CardCount += len(cards)
		}

		snapshot.CollectionEntries = len(db.Collection)
		for _, entry := range db.Collection {
			snapshot.CollectionCards += entry.Quantity
		}

		if db.SyncState.LastSuccessfulStartupSyncAt != nil {
			lastSyncAt := *db.SyncState.LastSuccessfulStartupSyncAt
			snapshot.LastSyncAt = &lastSyncAt
			snapshot.LastSync = util.HumanizeAge(snapshot.LastSyncAt)
		}
		snapshot.CatalogProvider = strings.TrimSpace(db.SyncState.CatalogProvider)
		snapshot.PriceProvider = strings.TrimSpace(db.SyncState.PriceProvider)
		return nil
	})
	if err != nil {
		snapshot.LastSync = "unknown"
	}
	if snapshot.CatalogProvider == "" {
		snapshot.CatalogProvider = "tcgdex"
	}
	if snapshot.PriceProvider == "" {
		snapshot.PriceProvider = "pokewallet"
	}
	m.mainSnapshot = snapshot
}

func (m *rootModel) refreshMainMenuClock() {
	if m.mode != modeMenu || m.menuKind != menuMain {
		return
	}
	if m.mainSnapshot.LastSyncAt == nil {
		m.mainSnapshot.LastSync = "never"
		return
	}
	m.mainSnapshot.LastSync = util.HumanizeAge(m.mainSnapshot.LastSyncAt)
}

func (m *rootModel) openLanguageMenu() tea.Cmd {
	sets, err := m.container.Sets.List()
	if err != nil {
		m.fatalErr = err
		return tea.Quit
	}
	if len(sets) == 0 {
		m.openMessage("No Data", "No sets are available yet. Run the tool again with startup sync enabled.", nextMainMenu)
		return nil
	}
	m.allSets = sets

	type languageCounter struct {
		Display string
		Count   int
	}
	byLanguage := make(map[string]languageCounter)
	for _, set := range sets {
		display := strings.TrimSpace(set.Language)
		if display == "" {
			display = "Unknown"
		}
		key := strings.ToLower(display)
		item := byLanguage[key]
		if item.Display == "" {
			item.Display = display
		}
		item.Count++
		byLanguage[key] = item
	}

	keys := make([]string, 0, len(byLanguage))
	for key := range byLanguage {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return byLanguage[keys[i]].Display < byLanguage[keys[j]].Display
	})

	options := make([]menuOption, 0, len(keys))
	for _, key := range keys {
		item := byLanguage[key]
		options = append(options, menuOption{
			Label:       item.Display,
			Description: fmt.Sprintf("%d sets", item.Count),
			Value:       item.Display,
		})
	}
	m.setMenu(
		menuLanguage,
		"Choose Card Language",
		"Sets will be filtered to this language only. Use / to filter.",
		options,
		true,
		12,
		nextMainMenu,
	)
	return nil
}

func (m *rootModel) openSetMenuForLanguage(language string) tea.Cmd {
	want := normalizeLanguage(language)
	filtered := make([]domain.Set, 0, len(m.allSets))
	for _, set := range m.allSets {
		if normalizeLanguage(set.Language) == want {
			filtered = append(filtered, set)
		}
	}
	m.filteredSets = filtered
	if len(filtered) == 0 {
		m.openMessage("No Sets", fmt.Sprintf("No sets are available for language %q.", language), nextLanguageMenu)
		return nil
	}

	if m.container.Config.LastViewedSetOnTop {
		if lastViewedSetID, err := m.container.SyncState.LastViewedSetID(); err == nil && lastViewedSetID != "" {
			for idx, set := range filtered {
				if strings.TrimSpace(set.ID) == lastViewedSetID {
					if idx > 0 {
						filtered = append([]domain.Set{set}, append(filtered[:idx], filtered[idx+1:]...)...)
					}
					break
				}
			}
		}
	}
	m.filteredSets = filtered

	options := make([]menuOption, 0, len(filtered))
	for _, set := range filtered {
		options = append(options, menuOption{
			Label:       viewmodel.SetLabel(set),
			Description: set.ReleaseDate,
			Value:       set.ID,
		})
	}
	m.setMenu(
		menuSet,
		"Choose a Set",
		"Type / to filter the set list.",
		options,
		true,
		16,
		nextLanguageMenu,
	)
	return nil
}

func (m *rootModel) openSetJumpInput() {
	m.setInput(
		inputSetJumpID,
		"Open set by ID or code",
		"Enter exact set ID/code (example: sv4a, swsh3).",
		"",
		nextSetMenu,
	)
}

func (m *rootModel) openSetByID(raw string) tea.Cmd {
	id := strings.TrimSpace(raw)
	if id == "" {
		m.openMessage("Set Missing", "Please enter a set ID or code.", nextSetMenu)
		return nil
	}

	set, ok, err := m.container.Sets.Get(id)
	if err != nil {
		m.fatalErr = err
		return tea.Quit
	}
	if !ok {
		target := strings.ToLower(id)
		for _, candidate := range m.filteredSets {
			if strings.ToLower(strings.TrimSpace(candidate.ID)) == target || strings.ToLower(strings.TrimSpace(candidate.SetCode)) == target {
				set = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		m.openMessage("Set Missing", fmt.Sprintf("No set found for %q in %s.", id, m.selectedLanguage), nextSetMenu)
		return nil
	}

	m.selectedSet = set
	_ = m.container.SyncState.SetLastViewedSetID(set.ID)
	cached, err := m.container.SetSync.IsSetCached(set.ID)
	if err != nil {
		m.fatalErr = err
		return tea.Quit
	}
	needsMetadataBackfill := strings.TrimSpace(set.SetCode) == "" ||
		strings.TrimSpace(set.PriceProviderSetID) == "" ||
		strings.TrimSpace(set.PriceProviderSetName) == ""
	if cached && !needsMetadataBackfill {
		m.openCardLookupInput()
		return nil
	}
	return m.startSetSyncCmd()
}

func (m *rootModel) openCardLookupInput() {
	m.setInput(
		inputCardLookup,
		"Card number for "+m.selectedSet.Name,
		"Examples: 1, 001, TG01, GG35, SVP 001",
		"",
		nextMainMenu,
	)
}

func (m *rootModel) openCardDetail(card domain.Card) tea.Cmd {
	m.mode = modeCardDetail
	m.card = card
	m.cardStatus = "Loaded from local data"
	m.cardRefreshing = false
	m.cardSelected = 0

	autoSaved := false
	if m.container.Config.SaveSearchedCardsDefault {
		if err := m.container.Collection.Add(m.card.ID); err != nil {
			m.cardStatus = "Auto-save failed"
		} else {
			m.cardStatus = "Saved to collection automatically"
			autoSaved = true
		}
	}
	m.loadCardImageWidget()

	needsRefresh := m.container.CardRefresh.NeedsRefresh(card, m.container.Config) || shouldRefreshImage(card, m.container.Config)
	if needsRefresh {
		m.cardRefreshing = true
		if autoSaved {
			m.cardStatus = "Refreshing prices... (saved to collection)"
		} else {
			m.cardStatus = "Refreshing prices..."
		}
		return m.refreshCardCmd(card)
	}
	return nil
}

func (m *rootModel) updateCardDetail(msg tea.KeyMsg) tea.Cmd {
	if m.container.Config.SaveSearchedCardsDefault {
		switch {
		case m.keyMatch(msg, "confirm", "enter"),
			m.keyMatch(msg, "card_close", "c"),
			m.keyMatch(msg, "back", "esc", "q"),
			m.keyMatch(msg, "card_add", "a"):
			return m.runNextAction(nextCardLookup)
		}
		return nil
	}

	switch {
	case m.keyMatch(msg, "card_left", "left", "h"):
		if m.cardSelected > 0 {
			m.cardSelected--
		}
		return nil
	case m.keyMatch(msg, "card_right", "right", "l", "tab"):
		if m.cardSelected < 1 {
			m.cardSelected++
		}
		return nil
	case m.keyMatch(msg, "confirm", "enter"):
		if m.cardSelected == 1 {
			return m.addCardToCollection()
		}
		return m.runNextAction(nextCardLookup)
	case m.keyMatch(msg, "card_add", "a"):
		return m.addCardToCollection()
	case m.keyMatch(msg, "card_close", "c"), m.keyMatch(msg, "back", "esc", "q"):
		return m.runNextAction(nextCardLookup)
	}
	return nil
}

func (m *rootModel) addCardToCollection() tea.Cmd {
	if err := m.container.Collection.Add(m.card.ID); err != nil {
		m.cardStatus = "Collection save failed"
		return nil
	}
	m.openMessage("Saved", m.card.Name+" was added to your collection.", nextCardLookup)
	return nil
}

func (m *rootModel) loadCardImageWidget() {
	m.cardImageReady = false
	m.cardImageErr = ""

	if !m.container.Config.ImagePreviewsEnabled {
		m.cardImageErr = "Image previews disabled"
		return
	}
	if m.card.ImagePath == "" {
		m.cardImageErr = "No cached image"
		return
	}
	if _, err := os.Stat(m.card.ImagePath); err != nil {
		m.cardImageErr = "Image file missing"
		return
	}
	if m.container.Renderer == nil || !m.container.Renderer.Supported() {
		m.cardImageErr = "No supported terminal image protocol"
		return
	}

	m.cardImageReady = true
}

func (m *rootModel) openSettingsMenu() {
	keySummary := trackerpricing.ValidationSummary{}
	if m.container != nil && m.container.KeyRing != nil {
		keySummary = m.container.KeyRing.Summary()
	}
	m.setMenu(
		menuSettings,
		"Settings",
		"Select one option to edit.",
		[]menuOption{
			{Label: fmt.Sprintf("API keys: %d configured / %d usable", len(m.settingsDraft.APIKeys), keySummary.Usable), Value: "api_keys"},
			{Label: fmt.Sprintf("Hotkeys: %d actions", len(m.settingsDraft.Hotkeys)), Value: "hotkeys"},
			{Label: "Database Actions", Value: "database_actions"},
			{Label: fmt.Sprintf("API key daily limit: %d", m.settingsDraft.APIKeyDailyLimit), Value: "api_key_daily_limit"},
			{Label: "Debug logging: " + onOff(m.settingsDraft.Debug), Value: "debug"},
			{Label: fmt.Sprintf("Card refresh TTL: %d hours", m.settingsDraft.CardRefreshTTLHours), Value: "card_refresh_ttl"},
			{Label: "Image previews: " + onOff(m.settingsDraft.ImagePreviewsEnabled), Value: "image_previews"},
			{Label: "Test image compatibility", Value: "image_compat"},
			{Label: "Image caching: " + onOff(m.settingsDraft.ImageCaching), Value: "image_caching"},
			{Label: "Prefetch card metadata on startup: " + onOff(m.settingsDraft.PrefetchCardMetadataOnStartup), Value: "startup_prefetch_metadata"},
			{Label: "Download all images on startup: " + onOff(m.settingsDraft.DownloadAllImagesOnStartup), Value: "startup_all_images"},
			{Label: fmt.Sprintf("Image download workers: %d", m.settingsDraft.ImageDownloadWorkers), Value: "image_download_workers"},
			{Label: "Backup image source: " + onOff(m.settingsDraft.BackupImageSource), Value: "backup_image_source"},
			{Label: "Sync card details: " + onOff(m.settingsDraft.SyncCardDetails), Value: "sync_card_details"},
			{Label: "Colors: " + onOff(m.settingsDraft.ColorsEnabled), Value: "colors"},
			{Label: fmt.Sprintf("Request delay: %d ms", m.settingsDraft.RequestDelayMs), Value: "request_delay"},
			{Label: fmt.Sprintf("Rate-limit cooldown: %d sec", m.settingsDraft.RateLimitCooldownSeconds), Value: "rate_limit_cooldown"},
			{Label: "Save searched cards by default: " + onOff(m.settingsDraft.SaveSearchedCardsDefault), Value: "save_searched"},
			{Label: "Last viewed set on top: " + onOff(m.settingsDraft.LastViewedSetOnTop), Value: "last_viewed_set_top"},
			{Label: "HTTP user agent: " + m.settingsDraft.UserAgent, Value: "user_agent"},
			{Label: "Save and Back", Value: "save_back"},
			{Label: "Back without saving", Value: "back_no_save"},
		},
		false,
		16,
		nextMainMenu,
	)
}

func (m *rootModel) onSettingsMenuSelect(value string) tea.Cmd {
	switch value {
	case "database_actions":
		m.openDatabaseActionsMenu()
		return nil
	case "hotkeys":
		m.openHotkeysMenu()
		return nil
	case "api_keys":
		m.openAPIKeysMenu()
		return nil
	case "api_key_daily_limit":
		m.openIntSetting("api_key_daily_limit", "API Key Daily Limit", "Allowed range: 1 to 100000 requests per key per UTC day.", m.settingsDraft.APIKeyDailyLimit, 1, 100000)
		return nil
	case "debug":
		m.openBoolSetting("debug", "Debug Logging", "Write detailed diagnostics to debug.log in the project folder.", m.settingsDraft.Debug)
		return nil
	case "card_refresh_ttl":
		m.openIntSetting("card_refresh_ttl", "Card Refresh TTL (hours)", "Allowed range: 1 to 168.", m.settingsDraft.CardRefreshTTLHours, 1, 168)
		return nil
	case "image_previews":
		m.openBoolSetting("image_previews", "Image Previews", "Render card images in detail view.", m.settingsDraft.ImagePreviewsEnabled)
		return nil
	case "image_compat":
		m.mode = modeBusy
		m.busyTitle = "Image Compatibility Test"
		m.busyStatus = "Downloading and rendering sample image..."
		return tea.Batch(m.spinner.Tick, m.imageCompatCmd())
	case "image_caching":
		m.openBoolSetting("image_caching", "Image Caching", "Store converted PNG card images in local cache.", m.settingsDraft.ImageCaching)
		return nil
	case "startup_prefetch_metadata":
		m.openBoolSetting("startup_prefetch_metadata", "Prefetch Card Metadata on Startup", "When enabled, startup sync loads full card metadata for all sets (without downloading images).", m.settingsDraft.PrefetchCardMetadataOnStartup)
		return nil
	case "startup_all_images":
		m.openBoolSetting("startup_all_images", "Download All Images on Startup", "When enabled, startup sync prefetches images for all sets. This can take a while.", m.settingsDraft.DownloadAllImagesOnStartup)
		return nil
	case "image_download_workers":
		m.openIntSetting("image_download_workers", "Image Download Workers", "Parallel workers for set image downloads. Allowed range: 1 to 32.", m.settingsDraft.ImageDownloadWorkers, 1, 32)
		return nil
	case "backup_image_source":
		m.openBoolSetting("backup_image_source", "Backup Image Source", "When enabled, fallback image URLs (Scrydex and provider image URL) are used if the primary source fails.", m.settingsDraft.BackupImageSource)
		return nil
	case "sync_card_details":
		m.openBoolSetting("sync_card_details", "Sync Card Details (prices)", "Fetch per-card detail stats and prices during set sync.", m.settingsDraft.SyncCardDetails)
		return nil
	case "colors":
		m.openBoolSetting("colors", "Colors", "Enable themed colors in the UI.", m.settingsDraft.ColorsEnabled)
		return nil
	case "request_delay":
		m.openIntSetting("request_delay", "Request Delay (ms)", "Allowed range: 250 to 10000.", m.settingsDraft.RequestDelayMs, 250, 10000)
		return nil
	case "rate_limit_cooldown":
		m.openIntSetting("rate_limit_cooldown", "Rate-limit Cooldown (seconds)", "Allowed range: 1 to 300.", m.settingsDraft.RateLimitCooldownSeconds, 1, 300)
		return nil
	case "save_searched":
		m.openBoolSetting("save_searched", "Save Searched Cards by Default", "When true, looked-up cards are auto-saved and the Add button is hidden.", m.settingsDraft.SaveSearchedCardsDefault)
		return nil
	case "last_viewed_set_top":
		m.openBoolSetting("last_viewed_set_top", "Last Viewed Set On Top", "When enabled, the most recently opened set is pinned to the top of set selection.", m.settingsDraft.LastViewedSetOnTop)
		return nil
	case "user_agent":
		m.openTextSetting("user_agent", "HTTP User Agent", "User-Agent header sent to remote requests.", m.settingsDraft.UserAgent, false)
		return nil
	case "save_back":
		if err := m.settingsDraft.Validate(); err != nil {
			m.openMessage("Invalid Settings", err.Error(), nextSettingsMenu)
			return nil
		}
		if reflect.DeepEqual(m.settingsDraft, m.container.Config) {
			return m.openMainMenu()
		}
		if err := config.Save(m.container.Paths.ConfigFile, m.settingsDraft); err != nil {
			m.openMessage("Settings Error", err.Error(), nextSettingsMenu)
			return nil
		}
		m.container = bootstrap.New(m.settingsDraft, m.container.Paths, m.container.Store)
		if summary, err := m.container.ValidateAPIKeys(m.ctx); err != nil {
			m.openMessage("API Key Validation", err.Error(), nextMainMenu)
			return nil
		} else if summary.Usable == 0 {
			m.openMessage("API Key Validation", "No usable API keys are configured. The app will not be able to refresh prices.", nextMainMenu)
			return nil
		}
		m.openMessage("Settings Saved", "The tool configuration was updated.", nextMainMenu)
		return nil
	case "back_no_save":
		return m.openMainMenu()
	}
	return nil
}

func (m *rootModel) openAPIKeysMenu() {
	statusByMasked := make(map[string]string)
	statuses := []trackerpricing.KeyStatus{}
	if m.container != nil && m.container.Tracker != nil {
		statuses = m.container.Tracker.KeyStatuses()
	}
	for _, status := range statuses {
		state := "unusable"
		if status.Usable {
			state = "usable"
		}
		statusByMasked[status.Masked] = state + " · " + status.Reason
	}

	options := make([]menuOption, 0, len(m.settingsDraft.APIKeys)+2)
	options = append(options, menuOption{
		Label:       "Add API key",
		Description: "Add a new Pokewallet API key",
		Value:       "api_add",
	})
	for idx, key := range m.settingsDraft.APIKeys {
		masked := maskAPIKeyDisplay(key)
		description := "remove this key"
		if state, ok := statusByMasked[masked]; ok {
			description = state
		}
		options = append(options, menuOption{
			Label:       fmt.Sprintf("Remove %s", masked),
			Description: description,
			Value:       fmt.Sprintf("api_remove:%d", idx),
		})
	}
	options = append(options, menuOption{
		Label:       "Back",
		Description: "Return to settings",
		Value:       "api_back",
	})

	m.setMenu(
		menuAPIKeys,
		"API Keys",
		"Manage API keys used for Pokewallet requests.",
		options,
		false,
		14,
		nextSettingsMenu,
	)
}

func (m *rootModel) openHotkeysMenu() {
	if m.settingsDraft.Hotkeys == nil {
		m.settingsDraft.Hotkeys = config.DefaultHotkeys()
	}
	defaults := config.DefaultHotkeys()
	options := make([]menuOption, 0, len(hotkeyActionSpecs)+2)
	for _, spec := range hotkeyActionSpecs {
		value := normalizedHotkeyToken(m.settingsDraft.Hotkeys[spec.ID])
		if value == "" {
			value = defaults[spec.ID]
		}
		options = append(options, menuOption{
			Label:       fmt.Sprintf("%s: %s", spec.Label, value),
			Description: spec.Description,
			Value:       "hotkey_edit:" + spec.ID,
		})
	}
	options = append(options,
		menuOption{
			Label:       "Reset all to defaults",
			Description: "Restore original hotkey mapping",
			Value:       "hotkey_reset",
		},
		menuOption{
			Label:       "Back",
			Description: "Return to settings",
			Value:       "hotkey_back",
		},
	)

	m.setMenu(
		menuHotkeys,
		"Hotkeys",
		"Configure key bindings used across the CLI.",
		options,
		false,
		16,
		nextSettingsMenu,
	)
}

func (m *rootModel) openDatabaseActionsMenu() {
	m.setMenu(
		menuDatabaseActions,
		"Database Actions",
		"Danger zone actions for local data.",
		[]menuOption{
			{Label: "Clear database", Description: "Remove all local sets and cards (keeps collection)", Value: "db_clear_catalog"},
			{Label: "Clear collection", Description: "Remove all collection entries", Value: "db_clear_collection"},
			{Label: "Build Full DB", Description: "Sync all sets and cards with details", Value: "db_build_full"},
			{Label: "Back", Description: "Return to settings", Value: "db_back"},
		},
		false,
		10,
		nextSettingsMenu,
	)
}

func (m *rootModel) openBuildFullConfirmMenu() {
	warning := strings.Join([]string{
		"!! Warning !!",
		"",
		"Doing a full Database Build can take a long time depending on internet speed!",
		"We recommend setting at least a few Pokewallet API keys for heavy full builds.",
		"",
		"Are you sure you want to continue?",
	}, "\n")
	m.setMenu(
		menuBuildFullConfirm,
		"Build Full DB",
		warning,
		[]menuOption{
			{Label: "Cancel", Description: "Go back to Database Actions", Value: "build_full_cancel"},
			{Label: "Continue", Description: "Start full database build now", Value: "build_full_continue"},
		},
		false,
		8,
		nextSettingsMenu,
	)
}

func (m *rootModel) clearCatalogData() tea.Cmd {
	if err := m.container.Store.Update(func(db *store.DB) error {
		db.Sets = make(map[string]domain.Set)
		db.CardsBySet = make(map[string]map[string]domain.Card)
		db.SyncState.LastStartupSyncAt = nil
		db.SyncState.LastSuccessfulStartupSyncAt = nil
		db.SyncState.LastViewedSetID = ""
		db.SyncState.CatalogProvider = "tcgdex"
		db.SyncState.PriceProvider = "pokewallet"
		return nil
	}); err != nil {
		m.openMessage("Database Actions", "Failed to clear database: "+err.Error(), nextSettingsMenu)
		return nil
	}
	m.openMessage("Database Actions", "Local set/card database was cleared.", nextSettingsMenu)
	return nil
}

func (m *rootModel) clearCollectionData() tea.Cmd {
	if err := m.container.Store.Update(func(db *store.DB) error {
		db.Collection = make(map[string]domain.CollectionEntry)
		return nil
	}); err != nil {
		m.openMessage("Database Actions", "Failed to clear collection: "+err.Error(), nextSettingsMenu)
		return nil
	}
	m.openMessage("Database Actions", "Collection database was cleared.", nextSettingsMenu)
	return nil
}

func (m *rootModel) startFullBuildCmd() tea.Cmd {
	m.mode = modeBusy
	m.busyTitle = "Build Full DB"
	m.busyStatus = "Preparing full database build..."
	m.fullBuildProgressCh = make(chan string, 256)
	m.fullBuildDoneCh = make(chan fullBuildDoneMsg, 1)

	go func() {
		report := func(status string) {
			select {
			case m.fullBuildProgressCh <- status:
			default:
			}
		}

		report("Refreshing set catalog...")
		stats, err := m.container.StartupSync.Run(m.ctx, func(syncer.StartupProgress) {})
		if err != nil {
			m.fullBuildDoneCh <- fullBuildDoneMsg{err: err}
			close(m.fullBuildProgressCh)
			return
		}

		sets, err := m.container.Sets.List()
		if err != nil {
			m.fullBuildDoneCh <- fullBuildDoneMsg{err: err}
			close(m.fullBuildProgressCh)
			return
		}

		totalSets := len(sets)
		totalCards := 0
		totalUpdated := 0
		totalImages := 0
		totalDetailsSynced := 0
		totalDetailsFailed := 0
		failedSets := 0

		for idx, set := range sets {
			select {
			case <-m.ctx.Done():
				m.fullBuildDoneCh <- fullBuildDoneMsg{err: m.ctx.Err()}
				close(m.fullBuildProgressCh)
				return
			default:
			}
			report(fmt.Sprintf("Syncing set %d/%d: %s", idx+1, totalSets, set.Name))
			result, syncErr := m.container.SetSync.SyncSet(m.ctx, set.ID, syncer.SetSyncOptions{
				ImageCaching:    m.container.Config.ImageCaching,
				SyncCardDetails: true,
				Config:          m.container.Config,
			}, nil)
			if syncErr != nil {
				failedSets++
				continue
			}
			totalCards += result.NewCards
			totalUpdated += result.UpdatedCards
			totalImages += result.ImagesSaved
			totalDetailsSynced += result.DetailsSynced
			totalDetailsFailed += result.DetailsFailed
		}

		summary := strings.Join([]string{
			fmt.Sprintf("Startup new sets: %d", stats.NewSets),
			fmt.Sprintf("Startup updated sets: %d", stats.UpdatedSets),
			fmt.Sprintf("Sets processed: %d", totalSets),
			fmt.Sprintf("Set sync failures: %d", failedSets),
			fmt.Sprintf("New cards: %d", totalCards),
			fmt.Sprintf("Updated cards: %d", totalUpdated),
			fmt.Sprintf("Images saved: %d", totalImages),
			fmt.Sprintf("Details synced: %d", totalDetailsSynced),
			fmt.Sprintf("Details failed: %d", totalDetailsFailed),
		}, "\n")
		m.fullBuildDoneCh <- fullBuildDoneMsg{summary: summary}
		close(m.fullBuildProgressCh)
	}()

	return tea.Batch(m.spinner.Tick, waitFullBuildProgress(m.fullBuildProgressCh), waitFullBuildDone(m.fullBuildDoneCh))
}

func (m *rootModel) openBoolSetting(key, title, description string, current bool) {
	options := []menuOption{
		{Label: "Enabled", Value: "true"},
		{Label: "Disabled", Value: "false"},
	}
	if !current {
		options[0], options[1] = options[1], options[0]
	}
	m.menuSettingKey = key
	m.setMenu(menuSettingBool, title, description, options, false, 8, nextSettingsMenu)
}

func (m *rootModel) openIntSetting(key, title, description string, current, min, max int) {
	m.inputSettingKey = key
	m.inputMin = min
	m.inputMax = max
	m.setInput(inputSettingInt, title, description, strconv.Itoa(current), nextSettingsMenu)
}

func (m *rootModel) openTextSetting(key, title, description, current string, allowBlank bool) {
	m.inputSettingKey = key
	m.inputAllowBlank = allowBlank
	m.setInput(inputSettingStr, title, description, current, nextSettingsMenu)
}

func (m *rootModel) applyBoolSetting(key string, value bool) {
	switch key {
	case "startup_sync":
		m.settingsDraft.StartupSyncEnabled = value
	case "debug":
		m.settingsDraft.Debug = value
	case "image_previews":
		m.settingsDraft.ImagePreviewsEnabled = value
	case "image_caching":
		m.settingsDraft.ImageCaching = value
	case "startup_prefetch_metadata":
		m.settingsDraft.PrefetchCardMetadataOnStartup = value
	case "startup_all_images":
		m.settingsDraft.DownloadAllImagesOnStartup = value
	case "backup_image_source":
		m.settingsDraft.BackupImageSource = value
	case "sync_card_details":
		m.settingsDraft.SyncCardDetails = value
	case "colors":
		m.settingsDraft.ColorsEnabled = value
	case "save_searched":
		m.settingsDraft.SaveSearchedCardsDefault = value
	case "last_viewed_set_top":
		m.settingsDraft.LastViewedSetOnTop = value
	}
}

func (m *rootModel) applyIntSetting(key string, value int) {
	switch key {
	case "api_key_daily_limit":
		m.settingsDraft.APIKeyDailyLimit = value
	case "card_refresh_ttl":
		m.settingsDraft.CardRefreshTTLHours = value
	case "request_delay":
		m.settingsDraft.RequestDelayMs = value
	case "rate_limit_cooldown":
		m.settingsDraft.RateLimitCooldownSeconds = value
	case "image_download_workers":
		m.settingsDraft.ImageDownloadWorkers = value
	}
}

func (m *rootModel) applyTextSetting(key string, value string) {
	switch key {
	case "api_add_key":
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		for _, existing := range m.settingsDraft.APIKeys {
			if strings.TrimSpace(existing) == trimmed {
				return
			}
		}
		m.settingsDraft.APIKeys = append(m.settingsDraft.APIKeys, trimmed)
		m.openAPIKeysMenu()
	case "user_agent":
		m.settingsDraft.UserAgent = value
	default:
		if strings.HasPrefix(key, "hotkey:") {
			action := strings.TrimPrefix(key, "hotkey:")
			normalized := normalizedHotkeyToken(value)
			if normalized == "" {
				return
			}
			if m.settingsDraft.Hotkeys == nil {
				m.settingsDraft.Hotkeys = config.DefaultHotkeys()
			}
			if normalized == "default" || normalized == "reset" {
				if fallback, ok := config.DefaultHotkeys()[action]; ok {
					m.settingsDraft.Hotkeys[action] = fallback
				}
			} else {
				m.settingsDraft.Hotkeys[action] = normalized
			}
			m.openHotkeysMenu()
		}
	}
}

func (m *rootModel) viewMenu() string {
	styles := m.styles()
	switch m.menuKind {
	case menuMain:
		return m.viewMainSelectionScreen(styles)
	case menuLanguage:
		return m.viewLanguageSelectionScreen(styles)
	case menuSet:
		return m.viewSetSelectionScreen(styles)
	case menuSettings:
		return m.viewSettingsSelectionScreen(styles)
	case menuHotkeys:
		return m.viewHotkeysSelectionScreen(styles)
	case menuAPIKeys:
		return m.viewAPIKeysSelectionScreen(styles)
	case menuBuildFullConfirm:
		return m.viewBuildFullConfirmScreen(styles)
	case menuDatabaseActions, menuSettingBool, menuImageCompatApply:
		return m.viewChoiceSelectionScreen(styles)
	default:
		return m.viewGenericMenuScreen(styles)
	}
}

func (m *rootModel) viewGenericMenuScreen(styles uitheme.Styles) string {
	lines := make([]string, 0, 24)
	section := "Menu"
	subtitle := m.menuTitle
	if strings.TrimSpace(m.menuDescription) != "" {
		lines = append(lines, styles.Muted.Render(m.menuDescription), "")
	}

	if m.menuFilterEnabled {
		lines = append(lines, m.renderFilterSummaryLine(styles), "")
	}

	maxVisible, compact := m.menuViewportRows(11, len(lines), m.menuFilterEnabled)
	actionRows, start, end := m.renderMenuOptionCards(styles, maxVisible, compact)
	lines = append(lines, actionRows...)
	if len(m.menuFiltered) > maxVisible {
		lines = append(lines, "", styles.Muted.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.menuFiltered))))
	}

	hints := "Enter: Select • Esc: Back • ↑/↓: Move"
	if m.menuFilterEnabled {
		hints += " • /: Filter"
	}
	lines = append(lines, "", styles.Muted.Render(hints))
	content := strings.Join(lines, "\n")
	return m.renderScreenShell(styles, section, subtitle, content)
}

func (m *rootModel) viewLanguageSelectionScreen(styles uitheme.Styles) string {
	selectedLanguage := ""
	setsInSelectedLanguage := 0
	if option, ok := m.currentMenuOption(); ok {
		selectedLanguage = option.Value
		want := normalizeLanguage(selectedLanguage)
		for _, set := range m.allSets {
			if normalizeLanguage(set.Language) == want {
				setsInSelectedLanguage++
			}
		}
	}

	countStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)
	selectedLanguageStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)
	switch normalizeLanguage(selectedLanguage) {
	case "japanese", "ja":
		selectedLanguageStyle = styles.Value.Copy().Bold(true).Foreground(lipgloss.Color("#A855F7"))
	case "english", "en":
		selectedLanguageStyle = styles.Value.Copy().Bold(true).Foreground(uitheme.Green)
	}

	summaryLines := []string{
		styles.Label.Render("Language Catalog"),
		fmt.Sprintf("Available languages: %s", countStyle.Render(strconv.Itoa(len(m.menuOptions)))),
		fmt.Sprintf("Total sets in local DB: %s", countStyle.Render(strconv.Itoa(len(m.allSets)))),
	}
	if selectedLanguage != "" {
		summaryLines = append(summaryLines,
			fmt.Sprintf("Selected language: %s", selectedLanguageStyle.Render(selectedLanguage)),
			fmt.Sprintf("Sets in selection: %s", countStyle.Render(strconv.Itoa(setsInSelectedLanguage))),
		)
	}

	hints := fmt.Sprintf(
		"%s: Select  •  %s/%s: Move  •  %s: Filter  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("filter", "/")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Language", "Choose Card Language", summaryLines, "Languages", hints)
}

func (m *rootModel) viewSetSelectionScreen(styles uitheme.Styles) string {
	selectedSet, selectedSetFound := m.currentSelectedSet()
	summaryWidth := m.clampedContentWidth(18, 80, 20)
	langStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)
	switch normalizeLanguage(m.selectedLanguage) {
	case "japanese", "ja":
		langStyle = styles.Value.Copy().Bold(true).Foreground(lipgloss.Color("#A855F7"))
	case "english", "en":
		langStyle = styles.Value.Copy().Bold(true).Foreground(uitheme.Green)
	}
	countStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)
	summaryLines := []string{
		styles.Label.Render("Set Browser"),
		fmt.Sprintf("Language: %s", langStyle.Render(m.selectedLanguage)),
		fmt.Sprintf("Matching sets: %s", countStyle.Render(strconv.Itoa(len(m.filteredSets)))),
	}
	if selectedSetFound {
		nameWidth := summaryWidth - lipgloss.Width("Name: ") - 2
		if nameWidth < 8 {
			nameWidth = 8
		}
		nameValue := truncateToWidth(selectedSet.Name, nameWidth)
		summaryLines = append(summaryLines,
			"",
			styles.Label.Render("Selected Set"),
			fmt.Sprintf("Name: %s", styles.Value.Copy().Bold(true).Foreground(uitheme.Cream).Render(nameValue)),
			fmt.Sprintf("Release: %s", styles.Muted.Copy().Foreground(uitheme.Slate).Render(selectedSet.ReleaseDate)),
			fmt.Sprintf("Cards cached: %s", countStyle.Render(strconv.Itoa(selectedSet.Total))),
		)
	}

	hints := fmt.Sprintf(
		"%s: Open Set  •  %s: Jump by ID  •  %s/%s: Move  •  %s: Filter  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("set_jump_id", "i")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("filter", "/")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Set", "Choose a Set", summaryLines, "Sets", hints)
}

func (m *rootModel) viewSettingsSelectionScreen(styles uitheme.Styles) string {
	changed := settingsDiffCount(m.settingsDraft, m.container.Config)
	statusValue := "Clean"
	statusStyle := styles.Success
	if changed > 0 {
		statusValue = fmt.Sprintf("%d unsaved change(s)", changed)
		statusStyle = styles.Warn
	}

	summaryLines := []string{
		styles.Label.Render("Settings Draft"),
		"Status: " + statusStyle.Render(statusValue),
		fmt.Sprintf("Hotkeys: %s", styles.Value.Copy().Bold(true).Foreground(uitheme.Gold).Render(strconv.Itoa(len(m.settingsDraft.Hotkeys)))),
		fmt.Sprintf("Image previews: %s", styles.Value.Render(onOff(m.settingsDraft.ImagePreviewsEnabled))),
		fmt.Sprintf("Image caching: %s", styles.Value.Render(onOff(m.settingsDraft.ImageCaching))),
		fmt.Sprintf("Startup metadata prefetch: %s", styles.Value.Render(onOff(m.settingsDraft.PrefetchCardMetadataOnStartup))),
		fmt.Sprintf("Startup image prefetch: %s", styles.Value.Render(onOff(m.settingsDraft.DownloadAllImagesOnStartup))),
		fmt.Sprintf("Image workers: %s", styles.Value.Render(strconv.Itoa(m.settingsDraft.ImageDownloadWorkers))),
		fmt.Sprintf("Request delay: %s", styles.Value.Render(fmt.Sprintf("%d ms", m.settingsDraft.RequestDelayMs))),
		fmt.Sprintf("Last viewed set on top: %s", styles.Value.Render(onOff(m.settingsDraft.LastViewedSetOnTop))),
	}

	hints := fmt.Sprintf(
		"%s: Edit/Apply  •  %s/%s: Move  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Settings", "Tool Configuration", summaryLines, "Settings Actions", hints)
}

func (m *rootModel) viewAPIKeysSelectionScreen(styles uitheme.Styles) string {
	statuses := []trackerpricing.KeyStatus{}
	if m.container != nil && m.container.Tracker != nil {
		statuses = m.container.Tracker.KeyStatuses()
	}
	usable := 0
	for _, status := range statuses {
		if status.Usable {
			usable++
		}
	}

	summaryLines := []string{
		styles.Label.Render("API Key Manager"),
		fmt.Sprintf("Configured keys: %s", styles.Value.Render(strconv.Itoa(len(m.settingsDraft.APIKeys)))),
		fmt.Sprintf("Usable keys: %s", styles.Value.Render(strconv.Itoa(usable))),
	}
	if len(statuses) > 0 {
		limit := statuses[0].DailyLimit
		summaryLines = append(summaryLines, fmt.Sprintf("Daily cap per key: %s", styles.Value.Render(strconv.Itoa(limit))))
	}

	hints := fmt.Sprintf(
		"%s: Select  •  %s/%s: Move  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Settings", "API Keys", summaryLines, "Key Actions", hints)
}

func (m *rootModel) viewHotkeysSelectionScreen(styles uitheme.Styles) string {
	configured := 0
	for _, spec := range hotkeyActionSpecs {
		if normalizedHotkeyToken(m.settingsDraft.Hotkeys[spec.ID]) != "" {
			configured++
		}
	}
	summaryLines := []string{
		styles.Label.Render("Hotkey Manager"),
		fmt.Sprintf("Configurable actions: %s", styles.Value.Copy().Bold(true).Foreground(uitheme.Gold).Render(strconv.Itoa(len(hotkeyActionSpecs)))),
		fmt.Sprintf("Configured actions: %s", styles.Value.Copy().Bold(true).Foreground(uitheme.Green).Render(strconv.Itoa(configured))),
		styles.Muted.Render("Tip: set value to 'default' to reset one action."),
	}
	hints := fmt.Sprintf(
		"%s: Edit  •  %s/%s: Move  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Settings", "Hotkeys", summaryLines, "Hotkey Actions", hints)
}

func (m *rootModel) viewChoiceSelectionScreen(styles uitheme.Styles) string {
	summaryLines := []string{
		styles.Label.Render(m.menuTitle),
		styles.Muted.Render(m.menuDescription),
	}
	hints := fmt.Sprintf(
		"%s: Select  •  %s/%s: Move  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Settings", m.menuTitle, summaryLines, "Choices", hints)
}

func (m *rootModel) viewBuildFullConfirmScreen(styles uitheme.Styles) string {
	slowFlash := (m.statusPulseFrame / 12) % 2
	warnColor := lipgloss.Color("#DC2626")
	if slowFlash == 1 {
		warnColor = lipgloss.Color("#F87171")
	}
	warnStyle := styles.Warn.Copy().Foreground(warnColor).Bold(true)

	summaryLines := []string{
		warnStyle.Render("!! Warning !!"),
		"",
		warnStyle.Render("Doing a full Database Build can take a long time depending on internet speed!"),
		warnStyle.Render("We recommend setting at least a few Pokewallet API keys for heavy full builds."),
		"",
		warnStyle.Render("Are you sure you want to continue?"),
	}
	hints := fmt.Sprintf(
		"%s: Select  •  %s/%s: Move  •  %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)
	return m.renderSelectionMenuScreen(styles, "Settings", "Build Full DB", summaryLines, "Confirm Action", hints)
}

func (m *rootModel) renderSelectionMenuScreen(styles uitheme.Styles, section string, subtitle string, summaryLines []string, actionTitle string, hints string) string {
	lines := make([]string, 0, len(summaryLines)+18)
	lines = append(lines, summaryLines...)
	if m.menuFilterEnabled {
		lines = append(lines, "", styles.Label.Render("Filter:")+" "+m.renderFilterSummaryLine(styles))
	}

	maxVisible, compact := m.menuViewportRows(10, len(lines), m.menuFilterEnabled)
	actionRows, start, end := m.renderMenuOptionCards(styles, maxVisible, compact)
	lines = append(lines, "", styles.Label.Render(actionTitle)+" "+m.statusPulse(styles, "info"))
	if len(m.menuFiltered) > 0 {
		lines = append(lines, styles.Muted.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.menuFiltered))))
	}
	lines = append(lines, m.renderRowsContainer(actionRows, maxVisible))
	if m.width > 0 && lipgloss.Width(hints) > m.width-20 {
		hints = "Enter: Select  •  Esc/Q: Back"
	}
	lines = append(lines, "", styles.Muted.Render(hints))

	lines = m.clampLines(lines, 14)
	return m.renderScreenShell(styles, section, subtitle, strings.Join(lines, "\n"))
}

func (m *rootModel) renderFilterSummaryLine(styles uitheme.Styles) string {
	filterView := m.menuFilter.View()
	if m.menuFilterActive {
		filterView = styles.Label.Render(filterView)
	} else {
		filterView = styles.Muted.Render(filterView)
	}
	return filterView + " " + styles.Muted.Render(fmt.Sprintf("(%d)", len(m.menuFiltered)))
}

func (m *rootModel) renderMenuFilterPanel(styles uitheme.Styles, width int) string {
	filterLines := []string{
		styles.Label.Render("Filter"),
		m.renderFilterSummaryLine(styles),
	}
	return styles.Card.Copy().
		Width(width).
		Padding(0, 1).
		BorderForeground(uitheme.Slate).
		Render(strings.Join(filterLines, "\n"))
}

func (m *rootModel) renderMenuOptionCards(styles uitheme.Styles, maxRows int, compact bool) ([]string, int, int) {
	if len(m.menuFiltered) == 0 {
		return []string{styles.Muted.Render("No matching options.")}, 0, 0
	}
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if m.menuCursor >= maxRows {
		start = m.menuCursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.menuFiltered) {
		end = len(m.menuFiltered)
	}

	rows := make([]string, 0, end-start)
	normalRowStyle := styles.Value
	selectedRowStyle := styles.Value.Copy().Bold(true)
	if m.container.Config.ColorsEnabled {
		normalRowStyle = normalRowStyle.Foreground(uitheme.Cream)
		selectedRowStyle = selectedRowStyle.Foreground(uitheme.Ink).Background(uitheme.Gold)
	}
	setReleaseStyle := styles.Muted.Copy().Foreground(lipgloss.Color("#4B5563"))
	setIDStyle := styles.Label.Copy().Foreground(uitheme.Blue)
	setNameStyle := styles.Value.Copy().Bold(true)
	setCardsStyle := styles.Success.Copy().Foreground(uitheme.Gold)
	languageJapaneseStyle := styles.Value.Copy().Bold(true).Foreground(lipgloss.Color("#A855F7"))
	languageEnglishStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Green)
	languageOtherStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)
	languageCountStyle := styles.Muted.Copy().Foreground(uitheme.Slate)
	setRowWidth := m.selectionRowWidth()
	for i := start; i < end; i++ {
		option := m.menuOptions[m.menuFiltered[i]]
		selected := i == m.menuCursor

		prefix := "  "
		if selected {
			frames := []string{"▸", "▹", "▸", "▹"}
			prefix = frames[m.menuAnimFrame%len(frames)] + " "
		}
		titleLine := normalRowStyle.Render(prefix + option.Label)
		descLine := styles.Muted.Render(option.Description)
		if selected {
			titleLine = selectedRowStyle.Render(prefix + option.Label)
			descLine = styles.Value.Render(option.Description)
		}

		if m.menuKind == menuSet && strings.TrimSpace(option.Description) != "" {
			setItem, ok := m.findFilteredSetByID(option.Value)
			if !ok {
				setItem = domain.Set{Name: option.Label, ReleaseDate: option.Description}
			}
			setID := strings.TrimSpace(setItem.SetCode)
			if setID == "" {
				setID = strings.TrimSpace(setItem.ID)
			}
			if setID == "" {
				setID = "?"
			}
			setName := strings.TrimSpace(setItem.Name)
			if setName == "" {
				setName = option.Label
			}
			cardsText := fmt.Sprintf("%d cards", setItem.Total)
			if setItem.Total <= 0 {
				cardsText = "cards ?"
			}
			datePlain := strings.TrimSpace(option.Description)
			includeDate := true
			includeCards := true
			minNameWidth := 6

			computeFixed := func(withCards bool, withDate bool) int {
				parts := lipgloss.Width(prefix) + lipgloss.Width("[]") + lipgloss.Width(setID)
				separators := 0
				if withCards || withDate {
					separators++
				}
				if withCards && withDate {
					separators++
				}
				parts += separators * lipgloss.Width(" · ")
				if withCards {
					parts += lipgloss.Width(cardsText)
				}
				if withDate {
					parts += lipgloss.Width(datePlain)
				}
				return parts
			}

			fixedWidth := computeFixed(includeCards, includeDate)
			nameMaxWidth := setRowWidth - fixedWidth
			if nameMaxWidth < minNameWidth && includeDate {
				includeDate = false
				fixedWidth = computeFixed(includeCards, includeDate)
				nameMaxWidth = setRowWidth - fixedWidth
			}
			if nameMaxWidth < minNameWidth && includeCards {
				includeCards = false
				fixedWidth = computeFixed(includeCards, includeDate)
				nameMaxWidth = setRowWidth - fixedWidth
			}
			if nameMaxWidth < 1 {
				nameMaxWidth = 1
			}
			namePlain := truncateToWidth(setName, nameMaxWidth)

			idPlain := "[" + setID + "]"
			idStyled := setIDStyle.Render(idPlain)
			nameStyled := setNameStyle.Render(namePlain)

			parts := []string{idStyled, nameStyled}
			partsPlain := []string{idPlain, namePlain}
			if includeCards {
				parts = append(parts, setCardsStyle.Render(cardsText))
				partsPlain = append(partsPlain, cardsText)
			}
			if includeDate {
				parts = append(parts, setReleaseStyle.Render(datePlain))
				partsPlain = append(partsPlain, datePlain)
			}
			rowText := prefix + strings.Join(parts, " · ")
			rowPlain := prefix + strings.Join(partsPlain, " · ")
			if lipgloss.Width(rowPlain) > setRowWidth {
				overflow := lipgloss.Width(rowPlain) - setRowWidth
				if overflow > 0 {
					namePlain = truncateToWidth(namePlain, lipgloss.Width(namePlain)-overflow)
					if namePlain == "" {
						namePlain = "…"
					}
					parts[1] = setNameStyle.Render(namePlain)
					rowText = prefix + strings.Join(parts, " · ")
				}
			}
			if lipgloss.Width(rowText) > setRowWidth {
				rowText = ansi.Truncate(rowText, setRowWidth, "")
			}
			if selected {
				rows = append(rows, lipgloss.NewStyle().Bold(true).Render(rowText))
			} else {
				rows = append(rows, rowText)
			}
			continue
		}

		if m.menuKind == menuLanguage {
			langPlain := strings.TrimSpace(option.Value)
			if langPlain == "" {
				langPlain = strings.TrimSpace(option.Label)
			}
			langNorm := normalizeLanguage(langPlain)
			langStyle := languageOtherStyle
			switch langNorm {
			case "japanese", "ja":
				langStyle = languageJapaneseStyle
			case "english", "en":
				langStyle = languageEnglishStyle
			}

			countPlain := strings.TrimSpace(option.Description)
			if countPlain == "" {
				countPlain = "0 sets"
			}
			rowText := prefix + langStyle.Render(langPlain) + styles.Muted.Render(" • ") + languageCountStyle.Render(countPlain)
			if lipgloss.Width(rowText) > setRowWidth {
				rowText = ansi.Truncate(rowText, setRowWidth, "")
			}
			if selected {
				rowText = lipgloss.NewStyle().Bold(true).Render(rowText)
			}
			rows = append(rows, rowText)
			continue
		}

		if m.menuKind == menuMain {
			actionStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Cream)
			switch strings.ToLower(strings.TrimSpace(option.Value)) {
			case "browse":
				actionStyle = styles.Success.Copy().Foreground(uitheme.Green)
			case "settings":
				actionStyle = styles.Value.Copy().Bold(true).Foreground(lipgloss.Color("#A855F7"))
			case "quit":
				actionStyle = styles.Warn.Copy().Foreground(uitheme.Red)
			}
			rowText := prefix + actionStyle.Render(option.Label)
			if lipgloss.Width(rowText) > setRowWidth {
				rowText = ansi.Truncate(rowText, setRowWidth, "")
			}
			if selected {
				rowText = selectedRowStyle.Render(rowText)
			}
			rows = append(rows, rowText)
			continue
		}

		if m.menuKind == menuBuildFullConfirm {
			labelStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Cream)
			switch strings.ToLower(strings.TrimSpace(option.Value)) {
			case "build_full_continue":
				labelStyle = styles.Warn.Copy().Foreground(uitheme.Red)
			case "build_full_cancel":
				labelStyle = styles.Success.Copy().Foreground(uitheme.Green)
			}
			labelPlain := strings.TrimSpace(option.Label)
			descPlain := strings.TrimSpace(option.Description)
			maxTextWidth := setRowWidth - lipgloss.Width(prefix)
			if maxTextWidth < 1 {
				maxTextWidth = 1
			}
			labelTruncated := truncateToWidth(labelPlain, maxTextWidth)
			rendered := labelStyle.Render(labelTruncated)

			if descPlain != "" && !compact {
				remaining := maxTextWidth - lipgloss.Width(labelTruncated)
				if remaining > lipgloss.Width(" · ") {
					descWidth := remaining - lipgloss.Width(" · ")
					descTruncated := truncateToWidth(descPlain, descWidth)
					if strings.TrimSpace(descTruncated) != "" {
						rendered += styles.Muted.Render(" · " + descTruncated)
					}
				}
			}
			if selected {
				rendered = lipgloss.NewStyle().Bold(true).Render(rendered)
			}
			rowText := prefix + rendered
			rows = append(rows, rowText)
			continue
		}

		row := titleLine
		if compact {
			rows = append(rows, row)
			continue
		}
		rows = append(rows, row)
		if strings.TrimSpace(option.Description) != "" {
			rows = append(rows, "   "+descLine)
		}
	}
	return rows, start, end
}

func (m *rootModel) renderRowsContainer(rows []string, maxVisible int) string {
	if len(rows) == 0 {
		return ""
	}
	if maxVisible < 1 {
		maxVisible = 1
	}
	body := strings.Join(rows, "\n")
	width := m.selectionRowWidth()
	return lipgloss.NewStyle().
		MaxWidth(width).
		Width(width).
		Height(maxVisible).
		MaxHeight(maxVisible).
		Render(body)
}

func (m *rootModel) currentMenuOption() (menuOption, bool) {
	if len(m.menuFiltered) == 0 || m.menuCursor < 0 || m.menuCursor >= len(m.menuFiltered) {
		return menuOption{}, false
	}
	return m.menuOptions[m.menuFiltered[m.menuCursor]], true
}

func (m *rootModel) currentSelectedSet() (domain.Set, bool) {
	selected, ok := m.currentMenuOption()
	if !ok {
		return domain.Set{}, false
	}
	return m.findFilteredSetByID(selected.Value)
}

func (m *rootModel) findFilteredSetByID(id string) (domain.Set, bool) {
	selectedID := strings.TrimSpace(id)
	for _, set := range m.filteredSets {
		if strings.TrimSpace(set.ID) == selectedID {
			return set, true
		}
	}
	return domain.Set{}, false
}

func (m *rootModel) viewMainSelectionScreen(styles uitheme.Styles) string {
	snap := m.mainSnapshot
	setCountStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)
	cardCountStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Green)
	languageCountStyle := styles.Value.Copy().Bold(true).Foreground(lipgloss.Color("#A855F7"))
	collectionCountStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)
	syncAgeStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Green)
	catalogProviderStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)
	priceProviderStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)

	summaryLines := []string{
		styles.Label.Render("Catalog:") + " " +
			setCountStyle.Render(strconv.Itoa(snap.SetCount)) + styles.Muted.Render(" sets • ") +
			cardCountStyle.Render(strconv.Itoa(snap.CardCount)) + styles.Muted.Render(" cards • ") +
			languageCountStyle.Render(strconv.Itoa(snap.LanguageCount)) + styles.Muted.Render(" languages"),
		styles.Label.Render("Collection:") + " " +
			collectionCountStyle.Render(strconv.Itoa(snap.CollectionEntries)) + styles.Muted.Render(" entries • ") +
			collectionCountStyle.Render(strconv.Itoa(snap.CollectionCards)) + styles.Muted.Render(" cards"),
		styles.Label.Render("Sync:") + " " +
			syncAgeStyle.Render(snap.LastSync) + styles.Muted.Render("  (") +
			catalogProviderStyle.Render(snap.CatalogProvider) + styles.Muted.Render("/") +
			priceProviderStyle.Render(snap.PriceProvider) + styles.Muted.Render(")"),
	}

	lines := make([]string, 0, 24)
	lines = append(lines, summaryLines...)
	maxVisible, _ := m.menuViewportRows(10, len(summaryLines), false)
	actionRows, _, _ := m.renderMenuOptionCards(styles, maxVisible, true)
	lines = append(lines, "", styles.Label.Render("Actions")+" "+m.statusPulse(styles, "info"))
	lines = append(lines, actionRows...)
	lines = append(lines, "", styles.Muted.Render(fmt.Sprintf(
		"%s: Select  •  %s/%s: Move  •  %s: Back  •  %s: Quit",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("move_up", "k")),
		strings.ToUpper(m.displayHotkey("move_down", "j")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
		strings.ToUpper(m.displayHotkey("quit", "ctrl+c")),
	)))

	lines = m.clampLines(lines, 14)
	return m.renderScreenShell(styles, "Main Menu", "", strings.Join(lines, "\n"))
}

func (m *rootModel) mainMenuStripe(width int) string {
	if width < 8 {
		width = 8
	}
	runes := make([]rune, width)
	for i := range runes {
		runes[i] = '-'
	}
	head := m.stripeFrame % width
	runes[head] = 'o'
	if head+1 < width {
		runes[head+1] = '-'
	}
	if head > 0 {
		runes[head-1] = '.'
	}
	return string(runes)
}

func (m *rootModel) viewInput() string {
	styles := m.styles()
	lines := []string{
		styles.Muted.Render(m.inputDescription),
		m.input.View(),
	}
	if m.inputError != "" {
		lines = append(lines, "", styles.Warn.Render(m.inputError))
	}
	lines = append(lines, "", styles.Muted.Render(fmt.Sprintf(
		"%s: Confirm • %s: Back",
		strings.ToUpper(m.displayHotkey("confirm", "enter")),
		strings.ToUpper(m.displayHotkey("back", "esc")),
	)))
	return m.renderScreenShell(styles, "Input", m.inputTitle, strings.Join(lines, "\n"))
}

func (m *rootModel) viewMessage() string {
	styles := m.styles()
	body := m.renderMessageBody(styles)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		body,
		"",
		styles.Muted.Render(fmt.Sprintf(
			"%s: Continue • %s: Close",
			strings.ToUpper(m.displayHotkey("confirm", "enter")),
			strings.ToUpper(m.displayHotkey("back", "esc")),
		)),
	)
	return m.renderScreenShell(styles, "Message", m.messageTitle, content)
}

func (m *rootModel) renderMessageBody(styles uitheme.Styles) string {
	switch strings.ToLower(strings.TrimSpace(m.messageTitle)) {
	case "set ready":
		return renderSetReadyMessageBody(styles, m.messageBody)
	case "startup sync complete":
		return renderStartupSyncCompleteMessageBody(styles, m.messageBody)
	case "build full db complete":
		return renderBuildFullDBCompleteMessageBody(styles, m.messageBody)
	default:
		return m.messageBody
	}
}

func renderSetReadyMessageBody(styles uitheme.Styles, body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	setNameStyle := styles.Value.Copy().Bold(true).Foreground(uitheme.Blue)

	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		if idx == 0 && !strings.Contains(trimmed, ":") {
			out = append(out, setNameStyle.Render(trimmed))
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			out = append(out, styles.Value.Render(trimmed))
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		out = append(out, renderSetReadyMetricLine(styles, key, val))
	}
	return strings.Join(out, "\n")
}

func renderStartupSyncCompleteMessageBody(styles uitheme.Styles, body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			out = append(out, styles.Value.Render(trimmed))
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		out = append(out, renderSetReadyMetricLine(styles, key, val))
	}
	return strings.Join(out, "\n")
}

func renderBuildFullDBCompleteMessageBody(styles uitheme.Styles, body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			out = append(out, styles.Value.Render(trimmed))
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		out = append(out, renderBuildFullDBMetricLine(styles, key, val))
	}
	return strings.Join(out, "\n")
}

func renderSetReadyMetricLine(styles uitheme.Styles, key string, value string) string {
	keyStyle := styles.Label
	valueStyle := styles.Value
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	count, hasCount := parseLeadingInt(value)

	switch {
	case strings.Contains(lowerKey, "failed"):
		keyStyle = styles.Warn
		if hasCount && count > 0 {
			valueStyle = styles.Warn
		} else {
			valueStyle = styles.Muted
		}
	case strings.Contains(lowerKey, "new"),
		strings.Contains(lowerKey, "updated"),
		strings.Contains(lowerKey, "saved"),
		strings.Contains(lowerKey, "synced"):
		keyStyle = styles.Success
		if hasCount && count > 0 {
			valueStyle = styles.Success
		} else {
			valueStyle = styles.Muted
		}
	case strings.Contains(lowerKey, "total"):
		keyStyle = styles.Label.Copy().Foreground(uitheme.Blue)
		valueStyle = styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)
	}

	return keyStyle.Render(key+":") + " " + valueStyle.Render(value)
}

func renderBuildFullDBMetricLine(styles uitheme.Styles, key string, value string) string {
	keyStyle := styles.Label
	valueStyle := styles.Value
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	count, hasCount := parseLeadingInt(value)

	switch {
	case strings.Contains(lowerKey, "failed"), strings.Contains(lowerKey, "failure"):
		keyStyle = styles.Warn
		if hasCount && count > 0 {
			valueStyle = styles.Warn
		} else {
			valueStyle = styles.Muted
		}
	case strings.Contains(lowerKey, "new"),
		strings.Contains(lowerKey, "updated"),
		strings.Contains(lowerKey, "saved"),
		strings.Contains(lowerKey, "synced"):
		keyStyle = styles.Success
		if hasCount && count > 0 {
			valueStyle = styles.Success
		} else {
			valueStyle = styles.Muted
		}
	case strings.Contains(lowerKey, "processed"),
		strings.Contains(lowerKey, "total"):
		keyStyle = styles.Label.Copy().Foreground(uitheme.Blue)
		valueStyle = styles.Value.Copy().Bold(true).Foreground(uitheme.Gold)
	}

	return keyStyle.Render(key+":") + " " + valueStyle.Render(value)
}

func parseLeadingInt(value string) (int, bool) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return 0, false
	}
	token := strings.Trim(fields[0], ",")
	n, err := strconv.Atoi(token)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (m *rootModel) viewBusy() string {
	styles := m.styles()
	lines := []string{
		fmt.Sprintf("%s %s", m.spinner.View(), m.busyStatus),
		fmt.Sprintf("%s %s", m.statusPulse(styles, "info"), styles.Muted.Render("Working...")),
		styles.Muted.Render("Please wait..."),
	}
	return m.renderScreenShell(styles, "Working", m.busyTitle, strings.Join(lines, "\n"))
}

func (m *rootModel) viewStartupSync() string {
	styles := m.styles()
	setPercent := 0.0
	if m.startupProgress.SetsTotal > 0 {
		setPercent = float64(m.startupProgress.SetsDone) / float64(m.startupProgress.SetsTotal)
	}
	rows := []string{
		fmt.Sprintf(
			"%s Sets  %s %d/%d  %s",
			m.spinner.View(),
			renderBar(setPercent, 32),
			m.startupProgress.SetsDone,
			m.startupProgress.SetsTotal,
			formatPercent(setPercent),
		),
	}
	if m.startupProgress.CardsTotal > 0 {
		cardPercent := float64(m.startupProgress.CardsDone) / float64(m.startupProgress.CardsTotal)
		rows = append(rows, fmt.Sprintf(
			"Cards %s %d/%d  %s",
			renderBar(cardPercent, 32),
			m.startupProgress.CardsDone,
			m.startupProgress.CardsTotal,
			formatPercent(cardPercent),
		))
	}
	rows = append(rows, styles.Label.Render("Status:")+" "+m.statusPulse(styles, "info")+" "+m.startupProgress.Status)
	if m.startupProgress.CurrentSet != "" {
		rows = append(rows, styles.Label.Render("Current:")+" "+m.startupProgress.CurrentSet)
	}

	visible := clampInt(m.startupReveal, 1, len(rows))
	body := make([]string, 0, len(rows)+2)
	body = append(body, rows[:visible]...)
	if visible < len(rows) {
		body = append(body, styles.Muted.Render("…"))
	}
	return m.renderScreenShell(styles, "Startup", "Catalog and optional prefetch tasks", strings.Join(body, "\n"))
}

func (m *rootModel) viewSetSync() string {
	styles := m.styles()
	status := m.setSyncProgress.Status
	if status == "" {
		status = "Working..."
	}
	count := ""
	if m.setSyncProgress.Total > 0 {
		count = fmt.Sprintf(" (%d/%d)", m.setSyncProgress.Done, m.setSyncProgress.Total)
	}
	percent := 0.0
	if m.setSyncProgress.Total > 0 {
		percent = float64(m.setSyncProgress.Done) / float64(m.setSyncProgress.Total)
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("%s Syncing %s", m.spinner.View(), m.selectedSet.Name),
		fmt.Sprintf("Stage: %s", strings.ToUpper(strings.TrimSpace(m.setSyncProgress.Stage))),
		fmt.Sprintf("Progress: %s %s", renderBar(percent, 30), formatPercent(percent)),
		"Status: "+m.statusPulse(styles, "info")+" "+status+count,
		styles.Muted.Render("Please wait..."),
	)
	return m.renderScreenShell(styles, "Set Sync", m.selectedSet.Name, body)
}

func (m *rootModel) viewCardDetail() string {
	styles := m.styles()
	imageWidth, detailsWidth, topHeight, bottomHeight := detailLayout(m.width, m.height)
	statusText := m.cardStatusText()
	statusLine := styles.Muted.Render(statusText)
	switch {
	case strings.Contains(strings.ToLower(statusText), "failed"):
		statusLine = styles.Warn.Render(statusText)
	case strings.Contains(strings.ToLower(statusText), "updated"), strings.Contains(strings.ToLower(statusText), "saved"):
		statusLine = styles.Success.Render(statusText)
	}
	statusKind := "info"
	if strings.Contains(strings.ToLower(statusText), "failed") {
		statusKind = "warn"
	}
	if strings.Contains(strings.ToLower(statusText), "updated") || strings.Contains(strings.ToLower(statusText), "saved") {
		statusKind = "ok"
	}
	closeHotkey := strings.ToUpper(m.displayHotkey("card_close", "c"))
	addHotkey := strings.ToUpper(m.displayHotkey("card_add", "a"))
	backHotkey := strings.ToUpper(m.displayHotkey("back", "esc"))
	hints := fmt.Sprintf("%s/%s: Close", backHotkey, closeHotkey)
	if !m.container.Config.SaveSearchedCardsDefault {
		hints = fmt.Sprintf("%s/%s: Close  •  %s: Add", backHotkey, closeHotkey, addHotkey)
	}
	lines := make([]string, 0, 24)
	for _, line := range viewmodel.DetailLines(m.card) {
		lines = append(lines, colorizeCardDetailLine(styles, line))
	}
	lines = append(
		lines,
		"",
		"Status: "+m.statusPulse(styles, statusKind)+" "+statusLine,
		"",
		renderActionRow(styles, m.cardSelected, m.container.Config.SaveSearchedCardsDefault, closeHotkey, addHotkey),
		styles.Muted.Render(hints),
	)
	details := styles.Card.Copy().Width(detailsWidth).Height(topHeight).Render(strings.Join(lines, "\n"))
	imagePane := m.renderCardImagePane(imageWidth, topHeight)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, imagePane, " ", details)
	topRowWidth := lipgloss.Width(topRow)
	minTopWidth := imageWidth + 1 + detailsWidth
	if topRowWidth < minTopWidth {
		topRowWidth = minTopWidth
	}
	if m.width > 0 {
		maxPanelWidth := m.width - 8
		if maxPanelWidth < 24 {
			maxPanelWidth = 24
		}
		if topRowWidth > maxPanelWidth {
			topRowWidth = maxPanelWidth
		}
	}
	recentSales := m.renderRecentSalesPane(topRowWidth, bottomHeight)
	title := m.renderCardDetailTitleLine()
	layout := lipgloss.JoinVertical(lipgloss.Left, topRow, " ", recentSales)
	view := lipgloss.NewStyle().Padding(1, 2).Render(title + "\n\n" + layout)
	return view + m.renderCardImageOverlay(imageWidth, topHeight)
}

func (m *rootModel) renderScreenShell(styles uitheme.Styles, section string, subtitle string, body string) string {
	stripeWidth := m.selectionRowWidth()
	if stripeWidth < 8 {
		stripeWidth = 8
	}
	content := []string{
		styles.Title.Render(appDisplayName),
		styles.Muted.Render("v" + m.version),
		styles.Muted.Render(m.mainMenuStripe(stripeWidth)),
		"",
	}
	if strings.TrimSpace(subtitle) != "" && section != subtitle {
		content = append(content, styles.Muted.Render(subtitle))
	}
	content = append(content, "", body)

	panelStyle := styles.Card.Copy().BorderForeground(uitheme.Blue).Padding(1, 2)
	if m.width > 0 {
		panelWidth := m.width - 8
		if panelWidth > 24 {
			panelStyle = panelStyle.Width(panelWidth)
		}
	}
	panel := panelStyle.Render(strings.Join(content, "\n"))
	return m.fitToViewport(lipgloss.NewStyle().Padding(1, 2).Render(panel))
}

func (m *rootModel) renderCardImagePane(width int, height int) string {
	styles := m.styles()
	content := ""
	switch {
	case m.cardImageReady:
		content = ""
	case m.cardImageErr != "":
		content = styles.Muted.Render(m.cardImageErr)
	default:
		content = styles.Muted.Render("Image unavailable")
	}
	return styles.Card.Copy().Padding(1).Width(width).Height(height).Render(content)
}

func (m *rootModel) renderCardImageOverlay(panelWidth int, panelHeight int) string {
	if !m.cardImageReady || m.container.Renderer == nil {
		return ""
	}
	// Border (2) + image padding (2) from renderCardImagePane.
	imageWidth := panelWidth - 4
	imageHeight := panelHeight - 4
	if imageWidth < 4 || imageHeight < 4 {
		return ""
	}
	renderWidth, renderHeight := fitImageCells(m.card.ImagePath, imageWidth, imageHeight)
	rendered, err := m.container.Renderer.Render(m.card.ImagePath, renderWidth, renderHeight)
	if err != nil || strings.TrimSpace(rendered) == "" || rendered == "[image unavailable]" {
		return ""
	}
	horizontalPad := 0
	if imageWidth > renderWidth {
		horizontalPad = (imageWidth - renderWidth) / 2
	}
	verticalPad := 0
	if imageHeight > renderHeight {
		verticalPad = (imageHeight - renderHeight) / 2
	}

	// Base cursor offsets for the image panel plus inner padding.
	imageTop := 6 + verticalPad
	imageLeft := 5 + horizontalPad

	var b strings.Builder
	b.WriteString(m.container.Renderer.ClearAllString())
	b.WriteString("\033[s")
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageTop, imageLeft))
	b.WriteString(rendered)
	b.WriteString("\033[u")
	return b.String()
}

func (m *rootModel) renderRecentSalesPane(width int, height int) string {
	styles := m.styles()
	if m.width > 0 {
		maxPanelWidth := m.width - 8
		if maxPanelWidth < 24 {
			maxPanelWidth = 24
		}
		if width > maxPanelWidth {
			width = maxPanelWidth
		}
	}
	if width < 12 {
		width = 12
	}
	innerWidth := width - 4
	if innerWidth < 8 {
		innerWidth = 8
	}
	maxRows := height - 5
	if maxRows < 1 {
		maxRows = 1
	}

	lines := []string{m.renderRecentSalesHeaderLine(innerWidth)}
	lines = append(lines, styles.Muted.Render(strings.Repeat("─", innerWidth)))
	recentSales := prioritizedRecentSales(m.card.RecentSales, maxRows)
	if len(recentSales) == 0 {
		lines = append(lines, styles.Muted.Render("No sold listings yet."))
		return styles.Card.Copy().Width(width).Height(height).Render(strings.Join(lines, "\n"))
	}

	for i := 0; i < len(recentSales); i++ {
		sale := recentSales[i]
		grade := saleGradeLabel(sale)
		gradeStyle := styles.Label
		switch saleGradeBucket(sale.Grade) {
		case salesBucketPSA10:
			gradeStyle = styles.Success
		case salesBucketPSA9:
			gradeStyle = styles.Value.Copy().Bold(true)
		}
		price := util.FormatMoney(sale.Price)
		dateText := "--"
		if sale.SoldAt != nil {
			dateText = sale.SoldAt.Local().Format("2006-01-02")
		}
		title := strings.TrimSpace(sale.Title)
		if title == "" {
			title = "Listing"
		}

		row := lipgloss.JoinHorizontal(
			lipgloss.Left,
			gradeStyle.Render(grade),
			"  ",
			styles.Success.Render(price),
			"  ",
			styles.Muted.Render(dateText),
			"  ",
			styles.Value.Render(title),
		)
		lines = append(lines, ansi.Truncate(row, innerWidth, ""))
	}

	return styles.Card.Copy().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m *rootModel) cardStatusText() string {
	status := strings.TrimSpace(m.cardStatus)
	statusLower := strings.ToLower(status)
	if strings.Contains(statusLower, "refreshing") || strings.Contains(statusLower, "failed") {
		return status
	}
	if m.card.PriceCheckedAt != nil {
		return "Updated " + util.HumanizeAge(m.card.PriceCheckedAt)
	}
	if status == "" {
		return "Status unavailable"
	}
	return status
}

func (m *rootModel) renderCardDetailTitleLine() string {
	styles := m.styles()
	return styles.Title.Render("Card Detail")
}

func (m *rootModel) renderRecentSalesHeaderLine(innerWidth int) string {
	styles := m.styles()
	left := styles.Label.Render("Recent Sales")
	quickSales := m.renderCardQuickSales()
	if strings.TrimSpace(quickSales) == "" {
		return ansi.Truncate(left, innerWidth, "")
	}

	leftWidth := lipgloss.Width(left)
	quickWidth := lipgloss.Width(quickSales)
	if quickWidth > innerWidth-2 {
		quickSales = ansi.Truncate(quickSales, innerWidth-2, "")
		quickWidth = lipgloss.Width(quickSales)
	}

	start := (innerWidth - quickWidth) / 2
	minStart := leftWidth + 2
	if start < minStart {
		start = minStart
	}
	if start+quickWidth > innerWidth {
		start = innerWidth - quickWidth
	}
	if start < leftWidth {
		start = leftWidth
	}

	gap := start - leftWidth
	if gap < 1 {
		gap = 1
	}
	row := left + strings.Repeat(" ", gap) + quickSales
	return ansi.Truncate(row, innerWidth, "")
}

func (m *rootModel) renderCardQuickSales() string {
	styles := m.styles()
	highest := highestSalesByBucket(m.card.RecentSales)
	if len(highest) == 0 {
		return ""
	}

	parts := make([]string, 0, 3)
	appendPart := func(label string, bucket string, labelStyle lipgloss.Style) {
		priceText := "n/a"
		priceStyle := styles.Muted
		if price, ok := highest[bucket]; ok && price != nil {
			priceText = util.FormatMoney(price)
			priceStyle = styles.Success
		}
		parts = append(parts, labelStyle.Render(label+":")+" "+priceStyle.Render(priceText))
	}

	appendPart("Ungraded", salesBucketUngraded, styles.Label)
	appendPart("PSA 10", salesBucketPSA10, styles.Success)
	appendPart("PSA 9", salesBucketPSA9, styles.Value.Copy().Bold(true))
	return strings.Join(parts, "   ")
}

func renderActionRow(styles uitheme.Styles, selected int, autoSave bool, closeHotkey string, addHotkey string) string {
	if autoSave {
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.Active.Render(closeHotkey+": Close (Enter)"),
			" ",
			styles.Success.Render("Auto-saved"),
		)
	}

	closeLabel := "Close"
	addLabel := "Add to collection"
	closeStyle := styles.Action
	addStyle := styles.Action

	if selected == 0 {
		closeLabel += " (Enter)"
		closeStyle = styles.Active
	} else {
		addLabel += " (Enter)"
		addStyle = styles.Active
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		closeStyle.Render(closeHotkey+": "+closeLabel),
		" ",
		addStyle.Render(addHotkey+": "+addLabel),
	)
}

func detailLayout(width int, height int) (imageWidth int, detailsWidth int, topHeight int, bottomHeight int) {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}
	contentWidth := width - 10
	if contentWidth < 40 {
		contentWidth = 40
	}
	totalHeight := height - 10
	if totalHeight < 12 {
		totalHeight = 12
	}
	if totalHeight > 40 {
		totalHeight = 40
	}

	topHeight = (totalHeight * 42) / 100
	if topHeight < 10 {
		topHeight = 10
	}
	if topHeight > 20 {
		topHeight = 20
	}
	bottomHeight = totalHeight - topHeight - 1
	if bottomHeight < 6 {
		bottomHeight = 6
		topHeight = totalHeight - bottomHeight - 1
		if topHeight < 8 {
			topHeight = 8
		}
	}

	// Size image pane from available height so portrait cards fill vertically
	// and avoid large empty side gutters.
	usableImageHeight := max(6, topHeight-4) // border + pane padding consume 4 rows.
	imageWidth = int(math.Round(float64(usableImageHeight)*1.44)) + 4
	if imageWidth < 14 {
		imageWidth = 14
	}
	maxImageWidth := (contentWidth * 40) / 100
	if maxImageWidth < 18 {
		maxImageWidth = 18
	}
	if imageWidth > maxImageWidth {
		imageWidth = maxImageWidth
	}
	detailsWidth = contentWidth - imageWidth - 1
	if detailsWidth < 20 {
		detailsWidth = 20
		imageWidth = contentWidth - detailsWidth - 1
		if imageWidth < 12 {
			imageWidth = 12
			detailsWidth = contentWidth - imageWidth - 1
		}
	}

	totalWidth := imageWidth + detailsWidth + 1
	overflow := totalWidth - contentWidth
	if overflow > 0 {
		reduce := func(v *int, min int) int {
			if overflow <= 0 {
				return 0
			}
			canReduce := *v - min
			if canReduce < 0 {
				canReduce = 0
			}
			cut := canReduce
			if cut > overflow {
				cut = overflow
			}
			*v -= cut
			overflow -= cut
			return cut
		}
		reduce(&detailsWidth, 20)
		reduce(&imageWidth, 12)
	}
	return imageWidth, detailsWidth, topHeight, bottomHeight
}

func fitImageCells(imagePath string, maxWidth int, maxHeight int) (int, int) {
	if maxWidth <= 0 || maxHeight <= 0 {
		return maxWidth, maxHeight
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return maxWidth, maxHeight
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return maxWidth, maxHeight
	}

	cellWidth := float64(termimg.DefaultFontWidth)
	cellHeight := float64(termimg.DefaultFontHeight)
	imageAspect := float64(cfg.Width) / float64(cfg.Height)
	widgetAspect := (float64(maxWidth) * cellWidth) / (float64(maxHeight) * cellHeight)

	renderWidth := maxWidth
	renderHeight := maxHeight
	if imageAspect > widgetAspect {
		renderHeight = int((float64(maxWidth) * cellWidth) / imageAspect / cellHeight)
	} else {
		renderWidth = int((float64(maxHeight) * cellHeight) * imageAspect / cellWidth)
	}
	if renderWidth < 1 {
		renderWidth = 1
	}
	if renderHeight < 1 {
		renderHeight = 1
	}
	return renderWidth, renderHeight
}

const (
	salesBucketUngraded = "ungraded"
	salesBucketPSA10    = "psa10"
	salesBucketPSA9     = "psa9"
)

func sortedRecentSales(sales []domain.SoldListing) []domain.SoldListing {
	if len(sales) == 0 {
		return nil
	}
	out := make([]domain.SoldListing, len(sales))
	copy(out, sales)
	sort.SliceStable(out, func(i, j int) bool {
		left := time.Time{}
		right := time.Time{}
		if out[i].SoldAt != nil {
			left = out[i].SoldAt.UTC()
		}
		if out[j].SoldAt != nil {
			right = out[j].SoldAt.UTC()
		}
		if !left.Equal(right) {
			return left.After(right)
		}

		leftAmount := 0.0
		rightAmount := 0.0
		if out[i].Price != nil {
			leftAmount = out[i].Price.Amount
		}
		if out[j].Price != nil {
			rightAmount = out[j].Price.Amount
		}
		return leftAmount > rightAmount
	})
	return out
}

func prioritizedRecentSales(sales []domain.SoldListing, limit int) []domain.SoldListing {
	ordered := sortedRecentSales(sales)
	if len(ordered) == 0 || limit <= 0 {
		return nil
	}

	grouped := map[string][]domain.SoldListing{
		salesBucketUngraded: {},
		salesBucketPSA10:    {},
		salesBucketPSA9:     {},
	}
	other := make([]domain.SoldListing, 0, len(ordered))
	for _, sale := range ordered {
		bucket := saleGradeBucket(sale.Grade)
		if bucket == "" {
			other = append(other, sale)
			continue
		}
		grouped[bucket] = append(grouped[bucket], sale)
	}

	if len(grouped[salesBucketUngraded]) == 0 && len(grouped[salesBucketPSA10]) == 0 && len(grouped[salesBucketPSA9]) == 0 && len(other) == 0 {
		return nil
	}

	out := make([]domain.SoldListing, 0, limit)
	appendFromGroup := func(bucket string, count int) int {
		if count <= 0 || len(out) >= limit {
			return 0
		}
		items := grouped[bucket]
		maxTake := count
		if maxTake > len(items) {
			maxTake = len(items)
		}
		remaining := limit - len(out)
		if maxTake > remaining {
			maxTake = remaining
		}
		if maxTake <= 0 {
			return 0
		}
		out = append(out, items[:maxTake]...)
		grouped[bucket] = items[maxTake:]
		return maxTake
	}

	minimum := 3
	missingToUngraded := 0
	for _, bucket := range []string{salesBucketUngraded, salesBucketPSA10, salesBucketPSA9} {
		taken := appendFromGroup(bucket, minimum)
		if taken < minimum {
			missingToUngraded += minimum - taken
		}
	}
	appendFromGroup(salesBucketUngraded, missingToUngraded)
	appendFromGroup(salesBucketUngraded, limit)
	appendFromGroup(salesBucketPSA10, limit)
	appendFromGroup(salesBucketPSA9, limit)

	if len(out) < limit && len(other) > 0 {
		remaining := limit - len(out)
		if remaining > len(other) {
			remaining = len(other)
		}
		out = append(out, other[:remaining]...)
	}
	return out
}

func saleGradeBucket(grade string) string {
	normalized := strings.ToUpper(strings.TrimSpace(grade))
	if normalized == "" {
		return salesBucketUngraded
	}
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.Join(strings.Fields(normalized), "")
	switch normalized {
	case "UNGRADED", "RAW":
		return salesBucketUngraded
	case "PSA10":
		return salesBucketPSA10
	case "PSA9":
		return salesBucketPSA9
	default:
		return ""
	}
}

func saleGradeLabel(sale domain.SoldListing) string {
	switch saleGradeBucket(sale.Grade) {
	case salesBucketUngraded:
		return "Ungraded"
	case salesBucketPSA10:
		return "PSA 10"
	case salesBucketPSA9:
		return "PSA 9"
	default:
		grade := strings.TrimSpace(sale.Grade)
		if grade == "" {
			return "Sale"
		}
		return grade
	}
}

func highestSalesByBucket(sales []domain.SoldListing) map[string]*domain.Money {
	out := make(map[string]*domain.Money, 3)
	for _, sale := range sales {
		if sale.Price == nil {
			continue
		}
		bucket := saleGradeBucket(sale.Grade)
		if bucket == "" {
			continue
		}
		current := out[bucket]
		if current == nil || sale.Price.Amount > current.Amount {
			price := *sale.Price
			out[bucket] = &price
		}
	}
	return out
}

func colorizeCardDetailLine(styles uitheme.Styles, line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return styles.Value.Render(trimmed)
	}

	label := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	labelText := styles.Label.Render(label + ":")

	valueStyle := styles.Value
	switch label {
	case "Name":
		valueStyle = styles.Title.Copy().Bold(true)
	case "English", "Set EN":
		valueStyle = styles.Muted
	case "Rarity":
		valueStyle = styles.Label
	case "Market", "PSA 10", "Ungraded Smart":
		valueStyle = styles.Success
	case "Low":
		valueStyle = styles.Warn
	case "Sales", "Population":
		valueStyle = styles.Value.Copy().Bold(true)
	}
	return labelText + " " + valueStyle.Render(value)
}

func (m *rootModel) statusPulse(styles uitheme.Styles, kind string) string {
	frames := []string{"·", "•", "◦", "•"}
	glyph := frames[m.statusPulseFrame%len(frames)]
	switch kind {
	case "ok":
		return styles.Success.Render(glyph)
	case "warn":
		return styles.Warn.Render(glyph)
	default:
		return styles.Label.Render(glyph)
	}
}

func (m *rootModel) configuredHotkey(action string) string {
	actionKey := strings.ToLower(strings.TrimSpace(action))
	if actionKey == "" || m.container == nil {
		return ""
	}
	hotkeys := m.container.Config.Hotkeys
	if hotkeys == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(hotkeys[actionKey]))
}

func (m *rootModel) keyMatch(msg tea.KeyMsg, action string, fallbacks ...string) bool {
	key := strings.ToLower(strings.TrimSpace(msg.String()))
	if key == "" {
		return false
	}
	if configured := m.configuredHotkey(action); configured != "" && key == configured {
		return true
	}
	for _, fallback := range fallbacks {
		if key == strings.ToLower(strings.TrimSpace(fallback)) {
			return true
		}
	}
	return false
}

func normalizedHotkeyToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (m *rootModel) displayHotkey(action string, fallback string) string {
	if configured := m.configuredHotkey(action); configured != "" {
		return configured
	}
	return normalizedHotkeyToken(fallback)
}

func (m *rootModel) clampedContentWidth(outerPadding int, fallback int, minSafe int) int {
	if m.width <= 0 {
		return fallback
	}
	width := m.width - outerPadding
	if width < minSafe {
		if width < 20 {
			return 20
		}
		return width
	}
	return width
}

func (m *rootModel) selectionRowWidth() int {
	if m.width <= 0 {
		return 72
	}
	panelWidth := m.width - 8
	if panelWidth < 24 {
		panelWidth = 24
	}
	innerWidth := panelWidth - 6
	if innerWidth < 16 {
		innerWidth = 16
	}
	return innerWidth
}

func (m *rootModel) menuViewportRows(baseReserve int, summaryLineCount int, hasFilter bool) (int, bool) {
	if m.height <= 0 {
		return m.menuMaxRows, false
	}
	reserve := baseReserve + summaryLineCount
	if hasFilter {
		reserve += 3
	}
	available := m.height - reserve
	if available < 4 {
		available = 4
	}

	compact := available < 20
	linesPerRow := 2
	if compact {
		linesPerRow = 1
	}
	rows := available / linesPerRow
	if rows < 1 {
		rows = 1
	}
	if rows > m.menuMaxRows {
		rows = m.menuMaxRows
	}
	return rows, compact
}

func (m *rootModel) clampLines(lines []string, reserve int) []string {
	if m.height <= 0 {
		return lines
	}
	maxLines := m.height - reserve
	if maxLines < 3 {
		maxLines = 3
	}
	if len(lines) <= maxLines {
		return lines
	}
	clamped := append([]string{}, lines[:maxLines-1]...)
	clamped = append(clamped, "…")
	return clamped
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func truncateToWidth(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= maxWidth {
		return value
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}

func renderBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent * float64(width))
	return "▕" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "▏"
}

func formatPercent(value float64) string {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return fmt.Sprintf("%3.0f%%", value*100)
}

func onOff(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

func settingsDiffCount(a config.Config, b config.Config) int {
	changed := 0
	if a.StartupSyncEnabled != b.StartupSyncEnabled {
		changed++
	}
	if !reflect.DeepEqual(a.APIKeys, b.APIKeys) {
		changed++
	}
	if !reflect.DeepEqual(a.Hotkeys, b.Hotkeys) {
		changed++
	}
	if a.APIKeyDailyLimit != b.APIKeyDailyLimit {
		changed++
	}
	if a.Debug != b.Debug {
		changed++
	}
	if a.CardRefreshTTLHours != b.CardRefreshTTLHours {
		changed++
	}
	if a.ImagePreviewsEnabled != b.ImagePreviewsEnabled {
		changed++
	}
	if a.ImageCaching != b.ImageCaching {
		changed++
	}
	if a.PrefetchCardMetadataOnStartup != b.PrefetchCardMetadataOnStartup {
		changed++
	}
	if a.DownloadAllImagesOnStartup != b.DownloadAllImagesOnStartup {
		changed++
	}
	if a.ImageDownloadWorkers != b.ImageDownloadWorkers {
		changed++
	}
	if a.BackupImageSource != b.BackupImageSource {
		changed++
	}
	if a.SyncCardDetails != b.SyncCardDetails {
		changed++
	}
	if a.ColorsEnabled != b.ColorsEnabled {
		changed++
	}
	if a.RequestDelayMs != b.RequestDelayMs {
		changed++
	}
	if a.RateLimitCooldownSeconds != b.RateLimitCooldownSeconds {
		changed++
	}
	if a.SaveSearchedCardsDefault != b.SaveSearchedCardsDefault {
		changed++
	}
	if a.LastViewedSetOnTop != b.LastViewedSetOnTop {
		changed++
	}
	if a.UserAgent != b.UserAgent {
		changed++
	}
	return changed
}

func findHotkeyActionSpec(actionID string) (hotkeyActionSpec, bool) {
	want := strings.ToLower(strings.TrimSpace(actionID))
	for _, spec := range hotkeyActionSpecs {
		if spec.ID == want {
			return spec, true
		}
	}
	return hotkeyActionSpec{}, false
}

func maskAPIKeyDisplay(key string) string {
	trimmed := strings.TrimSpace(key)
	if len(trimmed) <= 8 {
		return "****"
	}
	return trimmed[:4] + "…" + trimmed[len(trimmed)-4:]
}

func normalizeLanguage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return strings.ToLower(trimmed)
}

func cloneConfig(cfg config.Config) config.Config {
	out := cfg
	if cfg.APIKeys != nil {
		out.APIKeys = append([]string{}, cfg.APIKeys...)
	}
	if cfg.Hotkeys != nil {
		out.Hotkeys = make(map[string]string, len(cfg.Hotkeys))
		for action, key := range cfg.Hotkeys {
			out.Hotkeys[action] = key
		}
	}
	return out
}

func detectAppVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := strings.TrimSpace(info.Main.Version); v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			rev := strings.TrimSpace(setting.Value)
			if len(rev) >= 7 {
				return "dev-" + rev[:7]
			}
			if rev != "" {
				return "dev-" + rev
			}
		}
	}
	return "dev"
}

func shouldRefreshImage(card domain.Card, cfg config.Config) bool {
	if !cfg.ImagePreviewsEnabled || strings.TrimSpace(card.Number) == "" {
		return false
	}
	if card.ImagePath == "" {
		return true
	}
	if strings.ToLower(filepath.Ext(card.ImagePath)) != ".png" {
		return true
	}
	_, err := os.Stat(card.ImagePath)
	return err != nil
}

func downloadCompatibilityImage(url string) (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download test image returned %s", resp.Status)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp("", "pkmn-termimg-test-*.png")
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return "", err
	}
	return file.Name(), nil
}
