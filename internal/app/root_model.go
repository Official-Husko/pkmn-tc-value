package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	termimg "github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Official-Husko/pkmn-tc-value/internal/bootstrap"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/syncer"
	uitheme "github.com/Official-Husko/pkmn-tc-value/internal/ui/theme"
	"github.com/Official-Husko/pkmn-tc-value/internal/ui/viewmodel"
	_ "golang.org/x/image/webp"
)

const imageCompatTestURL = "https://pokemoncardimages.pokedata.io/images/Shiny+Treasure+ex/349.webp"

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
	menuSettingBool      menuKind = "setting_bool"
	menuImageCompat      menuKind = "image_compat"
	menuImageCompatApply menuKind = "image_compat_apply"
)

type inputKind string

const (
	inputCardLookup inputKind = "card_lookup"
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

type rootModel struct {
	ctx       context.Context
	container *bootstrap.Container

	mode     uiMode
	fatalErr error

	width  int
	height int

	spinner spinner.Model

	startupProgress   syncer.StartupProgress
	startupProgressCh chan syncer.StartupProgress
	startupDoneCh     chan startupDoneMsg

	setSyncProgress   syncer.SetSyncProgress
	setSyncProgressCh chan syncer.SetSyncProgress
	setSyncDoneCh     chan setSyncDoneMsg

	messageTitle string
	messageBody  string
	messageNext  nextAction

	menuKind          menuKind
	menuTitle         string
	menuDescription   string
	menuOptions       []menuOption
	menuFiltered      []int
	menuCursor        int
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
	cardImage      *termimg.ImageWidget
	cardImageErr   string
	cardSelected   int

	settingsDraft config.Config

	busyTitle  string
	busyStatus string
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
		settingsDraft: container.Config,
	}
}

func (m *rootModel) Init() tea.Cmd {
	if err := m.container.ImageCache.Validate(); err != nil {
		m.fatalErr = err
		return tea.Quit
	}

	if m.container.Config.StartupSyncEnabled {
		m.mode = modeStartupSync
		return tea.Batch(m.spinner.Tick, m.startStartupSyncCmd())
	}

	m.openMainMenu()
	return nil
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
		body := fmt.Sprintf("New sets: %d\nUpdated sets: %d\nCards are synced per set when you open one.", msg.stats.NewSets, msg.stats.UpdatedSets)
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
			m.cardStatus = "Refresh failed, showing cached data"
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
	if m.mode != modeCardDetail && m.container.Renderer != nil {
		return m.container.Renderer.ClearAllString() + content
	}
	return content
}

func (m *rootModel) globalQuitKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	default:
		return nil
	}
}

func (m *rootModel) spinnerActive() bool {
	return m.mode == modeStartupSync || m.mode == modeSetSync || m.mode == modeBusy
}

func (m *rootModel) styles() uitheme.Styles {
	return uitheme.NewStyles(m.container.Config.ColorsEnabled)
}

func (m *rootModel) startStartupSyncCmd() tea.Cmd {
	m.startupProgress = syncer.StartupProgress{Stage: "sets", Status: "Fetching set list"}
	m.startupProgressCh = make(chan syncer.StartupProgress, 1024)
	m.startupDoneCh = make(chan startupDoneMsg, 1)
	go func() {
		stats, err := m.container.StartupSync.Run(m.ctx, func(p syncer.StartupProgress) {
			select {
			case m.startupProgressCh <- p:
			default:
			}
		})
		m.startupDoneCh <- startupDoneMsg{stats: stats, err: err}
		close(m.startupProgressCh)
	}()
	return tea.Batch(waitStartupProgress(m.startupProgressCh), waitStartupDone(m.startupDoneCh))
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
		haystack := strings.ToLower(opt.Label + " " + opt.Description)
		if strings.Contains(haystack, query) {
			m.menuFiltered = append(m.menuFiltered, idx)
		}
	}
	if len(m.menuFiltered) == 0 {
		m.menuCursor = 0
		return
	}
	if m.menuCursor >= len(m.menuFiltered) {
		m.menuCursor = len(m.menuFiltered) - 1
	}
	if m.menuCursor < 0 {
		m.menuCursor = 0
	}
}

func (m *rootModel) updateMenu(msg tea.KeyMsg) tea.Cmd {
	if m.menuFilterActive {
		switch msg.String() {
		case "esc", "enter":
			m.menuFilterActive = false
			m.menuFilter.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.menuFilter, cmd = m.menuFilter.Update(msg)
		m.applyMenuFilter()
		return cmd
	}

	switch msg.String() {
	case "q", "esc":
		return m.runNextAction(m.menuCancel)
	case "/":
		if m.menuFilterEnabled {
			m.menuFilterActive = true
			m.menuFilter.Focus()
		}
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(m.menuFiltered)-1 {
			m.menuCursor++
		}
	case "pgup":
		m.menuCursor -= m.menuMaxRows
		if m.menuCursor < 0 {
			m.menuCursor = 0
		}
	case "pgdown":
		m.menuCursor += m.menuMaxRows
		if m.menuCursor > len(m.menuFiltered)-1 {
			m.menuCursor = len(m.menuFiltered) - 1
		}
	case "home":
		m.menuCursor = 0
	case "end":
		m.menuCursor = len(m.menuFiltered) - 1
	case "enter":
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
			m.settingsDraft = m.container.Config
			m.openSettingsMenu()
			return nil
		case "quit":
			return tea.Quit
		}
	case menuLanguage:
		m.selectedLanguage = value
		return m.openSetMenuForLanguage(value)
	case menuSet:
		set, ok, err := m.container.Sets.Get(value)
		if err != nil {
			m.fatalErr = err
			return tea.Quit
		}
		if !ok {
			m.openMessage("Set Missing", "The selected set was not found in the local database.", nextLanguageMenu)
			return nil
		}
		m.selectedSet = set
		cached, err := m.container.SetSync.IsSetCached(set.ID)
		if err != nil {
			m.fatalErr = err
			return tea.Quit
		}
		if cached {
			m.openCardLookupInput()
			return nil
		}
		return m.startSetSyncCmd()
	case menuSettings:
		return m.onSettingsMenuSelect(value)
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
		m.openMainMenu()
		return nil
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
	switch msg.String() {
	case "esc":
		return m.runNextAction(m.inputCancel)
	case "enter":
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
		m.openSettingsMenu()
		return nil
	}
	return nil
}

func (m *rootModel) updateMessage(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "esc", "q":
		return m.runNextAction(m.messageNext)
	}
	return nil
}

func (m *rootModel) openMainMenu() {
	m.setMenu(
		menuMain,
		"Pokemon Card Value",
		"Pick what to do next.",
		[]menuOption{
			{Label: "Browse sets", Value: "browse"},
			{Label: "Settings", Value: "settings"},
			{Label: "Quit", Value: "quit"},
		},
		false,
		8,
		nextQuit,
	)
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
			Label:       fmt.Sprintf("%s (%d sets)", item.Display, item.Count),
			Description: "Filter sets by this language",
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
	if m.container.Config.SaveSearchedCardsDefault {
		m.cardSelected = 1
	} else {
		m.cardSelected = 0
	}
	m.loadCardImageWidget()

	needsRefresh := m.container.CardRefresh.NeedsRefresh(card, m.container.Config) || shouldRefreshImage(card, m.container.Config)
	if needsRefresh {
		m.cardRefreshing = true
		m.cardStatus = "Refreshing prices..."
		return m.refreshCardCmd(card)
	}
	return nil
}

func (m *rootModel) updateCardDetail(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "left", "h":
		if m.cardSelected > 0 {
			m.cardSelected--
		}
		return nil
	case "right", "l", "tab":
		if m.cardSelected < 1 {
			m.cardSelected++
		}
		return nil
	case "enter":
		if m.cardSelected == 1 {
			return m.addCardToCollection()
		}
		return m.runNextAction(nextCardLookup)
	case "a":
		return m.addCardToCollection()
	case "c", "esc", "q":
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
	m.cardImage = nil
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

	widget, err := termimg.NewImageWidgetFromFile(m.card.ImagePath)
	if err != nil {
		m.cardImageErr = "Failed to load image widget"
		return
	}
	widget.SetProtocol(m.container.Renderer.Protocol())
	m.cardImage = widget
}

func (m *rootModel) openSettingsMenu() {
	m.setMenu(
		menuSettings,
		"Settings",
		"Select one option to edit.",
		[]menuOption{
			{Label: "Startup sync: " + onOff(m.settingsDraft.StartupSyncEnabled), Value: "startup_sync"},
			{Label: "Debug logging: " + onOff(m.settingsDraft.Debug), Value: "debug"},
			{Label: fmt.Sprintf("Card refresh TTL: %d hours", m.settingsDraft.CardRefreshTTLHours), Value: "card_refresh_ttl"},
			{Label: "Image previews: " + onOff(m.settingsDraft.ImagePreviewsEnabled), Value: "image_previews"},
			{Label: "Test image compatibility", Value: "image_compat"},
			{Label: "Image caching: " + onOff(m.settingsDraft.ImageCaching), Value: "image_caching"},
			{Label: "Backup image source: " + onOff(m.settingsDraft.BackupImageSource), Value: "backup_image_source"},
			{Label: "Sync card details: " + onOff(m.settingsDraft.SyncCardDetails), Value: "sync_card_details"},
			{Label: "Colors: " + onOff(m.settingsDraft.ColorsEnabled), Value: "colors"},
			{Label: fmt.Sprintf("Request delay: %d ms", m.settingsDraft.RequestDelayMs), Value: "request_delay"},
			{Label: fmt.Sprintf("Rate-limit cooldown: %d sec", m.settingsDraft.RateLimitCooldownSeconds), Value: "rate_limit_cooldown"},
			{Label: "Save searched cards by default: " + onOff(m.settingsDraft.SaveSearchedCardsDefault), Value: "save_searched"},
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
	case "startup_sync":
		m.openBoolSetting("startup_sync", "Startup Sync", "Fetch the latest set catalog when the app starts.", m.settingsDraft.StartupSyncEnabled)
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
	case "backup_image_source":
		m.openBoolSetting("backup_image_source", "Backup Image Source", "When enabled, PokeData and JSON image URLs are used if Scrydex fails.", m.settingsDraft.BackupImageSource)
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
		m.openBoolSetting("save_searched", "Save Searched Cards by Default", "When true, Enter defaults to Add to collection in card detail.", m.settingsDraft.SaveSearchedCardsDefault)
		return nil
	case "user_agent":
		m.openTextSetting("user_agent", "HTTP User Agent", "User-Agent header sent to remote requests.", m.settingsDraft.UserAgent, false)
		return nil
	case "save_back":
		if err := m.settingsDraft.Validate(); err != nil {
			m.openMessage("Invalid Settings", err.Error(), nextSettingsMenu)
			return nil
		}
		if m.settingsDraft == m.container.Config {
			m.openMainMenu()
			return nil
		}
		if err := config.Save(m.container.Paths.ConfigFile, m.settingsDraft); err != nil {
			m.openMessage("Settings Error", err.Error(), nextSettingsMenu)
			return nil
		}
		m.container = bootstrap.New(m.settingsDraft, m.container.Paths, m.container.Store)
		m.openMessage("Settings Saved", "The tool configuration was updated.", nextMainMenu)
		return nil
	case "back_no_save":
		m.openMainMenu()
		return nil
	}
	return nil
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
	case "backup_image_source":
		m.settingsDraft.BackupImageSource = value
	case "sync_card_details":
		m.settingsDraft.SyncCardDetails = value
	case "colors":
		m.settingsDraft.ColorsEnabled = value
	case "save_searched":
		m.settingsDraft.SaveSearchedCardsDefault = value
	}
}

func (m *rootModel) applyIntSetting(key string, value int) {
	switch key {
	case "card_refresh_ttl":
		m.settingsDraft.CardRefreshTTLHours = value
	case "request_delay":
		m.settingsDraft.RequestDelayMs = value
	case "rate_limit_cooldown":
		m.settingsDraft.RateLimitCooldownSeconds = value
	}
}

func (m *rootModel) applyTextSetting(key string, value string) {
	switch key {
	case "user_agent":
		m.settingsDraft.UserAgent = value
	}
}

func (m *rootModel) viewMenu() string {
	styles := m.styles()
	lines := []string{
		styles.Title.Render(m.menuTitle),
		styles.Muted.Render(m.menuDescription),
		"",
	}
	if m.menuFilterEnabled {
		filterView := m.menuFilter.View()
		if m.menuFilterActive {
			filterView = styles.Label.Render(filterView)
		} else {
			filterView = styles.Muted.Render(filterView)
		}
		lines = append(lines, filterView, "")
	}
	if len(m.menuFiltered) == 0 {
		lines = append(lines, styles.Muted.Render("No matching options."))
	} else {
		start := 0
		if m.menuCursor >= m.menuMaxRows {
			start = m.menuCursor - m.menuMaxRows + 1
		}
		end := start + m.menuMaxRows
		if end > len(m.menuFiltered) {
			end = len(m.menuFiltered)
		}
		for i := start; i < end; i++ {
			option := m.menuOptions[m.menuFiltered[i]]
			label := option.Label
			if strings.TrimSpace(option.Description) != "" {
				label += " " + styles.Muted.Render("· "+option.Description)
			}
			if i == m.menuCursor {
				lines = append(lines, styles.Active.Render(label))
			} else {
				lines = append(lines, styles.Action.Render(label))
			}
		}
	}
	hints := "Enter: Select • Esc: Back • ↑/↓: Move"
	if m.menuFilterEnabled {
		hints += " • /: Filter"
	}
	lines = append(lines, "", styles.Muted.Render(hints))
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (m *rootModel) viewInput() string {
	styles := m.styles()
	lines := []string{
		styles.Title.Render(m.inputTitle),
		styles.Muted.Render(m.inputDescription),
		"",
		m.input.View(),
	}
	if m.inputError != "" {
		lines = append(lines, "", styles.Warn.Render(m.inputError))
	}
	lines = append(lines, "", styles.Muted.Render("Enter: Confirm • Esc: Back"))
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func (m *rootModel) viewMessage() string {
	styles := m.styles()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.Title.Render(m.messageTitle),
		"",
		m.messageBody,
		"",
		styles.Muted.Render("Enter: Continue • Esc: Close"),
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m *rootModel) viewBusy() string {
	styles := m.styles()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.Title.Render(m.busyTitle),
		"",
		fmt.Sprintf("%s %s", m.spinner.View(), m.busyStatus),
		styles.Muted.Render("Please wait..."),
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m *rootModel) viewStartupSync() string {
	styles := m.styles()
	setPercent := 0.0
	if m.startupProgress.SetsTotal > 0 {
		setPercent = float64(m.startupProgress.SetsDone) / float64(m.startupProgress.SetsTotal)
	}
	body := []string{
		styles.Title.Render("Startup Sync"),
		"",
		fmt.Sprintf("%s Sets  %s %d/%d", m.spinner.View(), renderBar(setPercent, 36), m.startupProgress.SetsDone, m.startupProgress.SetsTotal),
	}
	if m.startupProgress.CardsTotal > 0 {
		cardPercent := float64(m.startupProgress.CardsDone) / float64(m.startupProgress.CardsTotal)
		body = append(body, fmt.Sprintf("Cards %s %d/%d", renderBar(cardPercent, 36), m.startupProgress.CardsDone, m.startupProgress.CardsTotal))
	}
	body = append(body, "", "Status: "+m.startupProgress.Status)
	if m.startupProgress.CurrentSet != "" {
		body = append(body, "Current: "+m.startupProgress.CurrentSet)
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(body, "\n"))
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
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.Title.Render("Downloading Database"),
		"",
		fmt.Sprintf("%s Syncing %s", m.spinner.View(), m.selectedSet.Name),
		fmt.Sprintf("Stage: %s", m.setSyncProgress.Stage),
		"Status: "+status+count,
		styles.Muted.Render("Please wait..."),
	)
	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *rootModel) viewCardDetail() string {
	styles := m.styles()
	leftWidth, rightWidth, panelHeight := detailLayout(m.width, m.height)
	lines := append(viewmodel.DetailLines(m.card), "", "Status: "+m.cardStatus, "", renderActionRow(styles, m.cardSelected))
	details := styles.Card.Copy().Width(rightWidth).Height(panelHeight).Render(strings.Join(lines, "\n"))
	title := styles.Title.Render("Card Detail")
	imagePane := m.renderCardImagePane(leftWidth, panelHeight)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, imagePane, " ", details)
	view := lipgloss.NewStyle().Padding(1, 2).Render(title + "\n\n" + layout)
	return view + m.renderCardImageOverlay(leftWidth, panelHeight)
}

func (m *rootModel) renderCardImagePane(width int, height int) string {
	styles := m.styles()
	content := ""
	switch {
	case m.cardImage != nil:
		content = ""
	case m.cardImageErr != "":
		content = styles.Muted.Render(m.cardImageErr)
	default:
		content = styles.Muted.Render("Image unavailable")
	}
	return styles.Card.Copy().Padding(0).Width(width).Height(height).Render(content)
}

func (m *rootModel) renderCardImageOverlay(panelWidth int, panelHeight int) string {
	if m.cardImage == nil || m.container.Renderer == nil {
		return ""
	}
	imageWidth := panelWidth - 2
	imageHeight := panelHeight - 2
	if imageWidth < 4 || imageHeight < 4 {
		return ""
	}
	m.cardImage.SetSizeWithCorrection(imageWidth, imageHeight)
	rendered, err := m.cardImage.Render()
	if err != nil {
		return ""
	}

	const imageTop = 5
	const imageLeft = 4

	var b strings.Builder
	b.WriteString(m.container.Renderer.ClearAllString())
	b.WriteString("\033[s")
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageTop, imageLeft))
	b.WriteString(rendered)
	b.WriteString("\033[u")
	return b.String()
}

func renderActionRow(styles uitheme.Styles, selected int) string {
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
		closeStyle.Render("C: "+closeLabel),
		" ",
		addStyle.Render("A: "+addLabel),
	)
}

func detailLayout(width int, height int) (left int, right int, panelHeight int) {
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}
	contentWidth := width - 5
	if contentWidth < 70 {
		contentWidth = 70
	}
	left = contentWidth / 2
	right = contentWidth - left - 1
	if left < 30 {
		left = 30
		right = contentWidth - left - 1
	}
	if right < 36 {
		right = 36
		left = contentWidth - right - 1
	}
	panelHeight = height - 8
	if panelHeight < 16 {
		panelHeight = 16
	}
	if panelHeight > 36 {
		panelHeight = 36
	}
	return left, right, panelHeight
}

func renderBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent * float64(width))
	return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
}

func onOff(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

func normalizeLanguage(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return strings.ToLower(trimmed)
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
