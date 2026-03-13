package tcgdex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/providerslog"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

const (
	tcgdexBaseURL         = "https://api.tcgdex.net/v2"
	pokewalletCatalogURL  = "https://api.pokewallet.io/sets"
	providerLogTCGDex     = "tcgdex"
	providerLogPokewallet = "pokewallet-catalog"
)

var japaneseSetIDRE = regexp.MustCompile(`\d[a-z]$`)
var likelySetCodeRE = regexp.MustCompile(`^[A-Za-z0-9.\-]+$`)

type setLocator struct {
	LanguageCode string
	TCGDexID     string
	SetCode      string
	Name         string
}

type Provider struct {
	client  *http.Client
	logger  *providerslog.Logger
	apiKeys []string

	mu        sync.RWMutex
	setLookup map[string]setLocator
}

func New(client *http.Client, logger *providerslog.Logger, apiKeys []string) catalog.Provider {
	cleanKeys := make([]string, 0, len(apiKeys))
	for _, key := range apiKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			cleanKeys = append(cleanKeys, trimmed)
		}
	}
	return &Provider{
		client:    client,
		logger:    logger,
		apiKeys:   cleanKeys,
		setLookup: make(map[string]setLocator),
	}
}

func (p *Provider) Name() string {
	return "pokewallet+tcgdex"
}

func (p *Provider) FetchSets(ctx context.Context) ([]domain.RemoteSet, error) {
	walletSets, err := p.fetchPokewalletSets(ctx)
	if err != nil {
		return nil, err
	}
	tcgdexByLang, err := p.fetchTCGDexSetIndexes(ctx)
	if err != nil {
		return nil, err
	}

	sets := make([]domain.RemoteSet, 0, len(walletSets))
	nextLookup := make(map[string]setLocator, len(walletSets))

	for _, walletSet := range walletSets {
		setID := strings.TrimSpace(walletSet.SetID.String())
		if setID == "" {
			continue
		}

		languageCode := normalizeWalletLanguage(walletSet.Language)
		if languageCode == "" {
			languageCode = "en"
		}

		setCode := strings.TrimSpace(walletSet.SetCode)
		cleanName := cleanWalletSetName(walletSet.Name, setCode)
		matched, hasMatched := pickTCGDexSet(tcgdexByLang, languageCode, setCode)

		official := walletSet.CardCount
		total := walletSet.CardCount
		logoURL := ""
		symbolURL := ""
		foreignName := ""
		tcgdexSetID := strings.TrimSpace(setCode)

		if hasMatched {
			if matched.CardCount.Official > 0 {
				official = matched.CardCount.Official
			}
			if matched.CardCount.Total > 0 {
				total = matched.CardCount.Total
			}
			tcgdexSetID = strings.TrimSpace(matched.ID)
			logoURL = ensurePNG(matched.Logo)
			symbolURL = ensurePNG(matched.Symbol)
			if languageCode != "en" {
				foreignName = strings.TrimSpace(matched.Name)
			}
		}

		if cleanName == "" {
			if hasMatched && languageCode == "en" {
				cleanName = strings.TrimSpace(matched.Name)
			}
			if cleanName == "" {
				cleanName = strings.TrimSpace(walletSet.Name)
			}
		}

		nextLookup[setID] = setLocator{
			LanguageCode: languageCode,
			TCGDexID:     tcgdexSetID,
			SetCode:      setCode,
			Name:         cleanName,
		}

		sets = append(sets, domain.RemoteSet{
			ID:                   setID,
			Language:             toLanguageLabel(languageCode),
			Name:                 cleanName,
			ForeignName:          foreignName,
			EnglishName:          cleanName,
			SetCode:              setCode,
			PriceProviderSetID:   setID,
			PriceProviderSetName: cleanName,
			PriceProviderSetCode: setCode,
			Series:               "",
			PrintedTotal:         official,
			Total:                total,
			ReleaseDate:          strings.TrimSpace(walletSet.ReleaseDate),
			SymbolURL:            symbolURL,
			LogoURL:              logoURL,
		})
	}

	p.mu.Lock()
	p.setLookup = nextLookup
	p.mu.Unlock()

	return sets, nil
}

func (p *Provider) FetchCardsForSet(ctx context.Context, setID string) ([]domain.RemoteCard, error) {
	locator, ok := p.lookupSet(setID)
	if !ok {
		if _, err := p.FetchSets(ctx); err != nil {
			return nil, err
		}
		locator, ok = p.lookupSet(setID)
		if !ok {
			return nil, fmt.Errorf("set id %s not found in catalog", setID)
		}
	}

	language := strings.TrimSpace(locator.LanguageCode)
	if language == "" {
		if looksJapaneseSetID(locator.SetCode) {
			language = "ja"
		} else {
			language = "en"
		}
	}

	setData, cards, err := p.fetchPokewalletCardsForSet(ctx, setID, locator, language)
	if err != nil {
		return nil, err
	}

	setCode := strings.TrimSpace(locator.SetCode)
	if setCode == "" {
		setCode = strings.TrimSpace(setData.SetCode)
	}
	if setCode == "" {
		setCode = strings.TrimSpace(setData.SetID.String())
	}
	setName := strings.TrimSpace(locator.Name)
	if setName == "" {
		setName = cleanWalletSetName(setData.Name, setCode)
	}
	if setName == "" {
		setName = strings.TrimSpace(setData.Name)
	}
	setEnglishName := setName
	tcgdexImagesByNumber := p.fetchTCGDexCardImageMap(ctx, locator, language)

	out := make([]domain.RemoteCard, 0, len(cards))
	languageLabel := toLanguageLabel(language)
	for _, card := range cards {
		cardID := strings.TrimSpace(card.ID)
		if cardID == "" {
			continue
		}
		name := strings.TrimSpace(card.CardInfo.Name)
		if name == "" {
			name = strings.TrimSpace(card.CardInfo.CleanName)
		}
		number := strings.TrimSpace(card.CardInfo.CardNumber)
		totalSetNumber := strings.TrimSpace(card.CardInfo.TotalSetNumber)
		if totalSetNumber == "" {
			if slash := strings.Index(number, "/"); slash > 0 && slash+1 < len(number) {
				totalSetNumber = strings.TrimSpace(number[slash+1:])
			}
		}
		tcgPlayerID := ""
		if card.TCGPlayer != nil {
			tcgPlayerID = extractTCGPlayerID(card.TCGPlayer.URL)
		}
		storedCardID := storedCardID(cardID, tcgPlayerID)
		storedProviderCardID := storedProviderCardID(cardID)
		rarity := strings.TrimSpace(card.CardInfo.Rarity)
		cardType := strings.TrimSpace(card.CardInfo.CardType)
		hp := strings.TrimSpace(card.CardInfo.HP.String())
		stage := strings.TrimSpace(card.CardInfo.Stage)
		cardText := strings.TrimSpace(card.CardInfo.CardText)
		weakness := strings.TrimSpace(card.CardInfo.Weakness)
		resistance := strings.TrimSpace(card.CardInfo.Resistance)
		retreatCost := strings.TrimSpace(card.CardInfo.RetreatCost.String())
		attacks := compactStrings(card.CardInfo.Attacks)
		imageBase := ""
		for _, key := range numberLookupKeys(number) {
			if mapped := strings.TrimSpace(tcgdexImagesByNumber[key]); mapped != "" {
				imageBase = mapped
				break
			}
		}

		out = append(out, domain.RemoteCard{
			ID:                  storedCardID,
			SetID:               strings.TrimSpace(setID),
			SetName:             setName,
			SetEnglishName:      setEnglishName,
			SetCode:             setCode,
			PriceProviderSetID:  strings.TrimSpace(setID),
			Language:            languageLabel,
			Name:                name,
			EnglishName:         englishNameForLanguage(language, name),
			Number:              util.CardLocalNumber(number),
			TotalSetNumber:      totalSetNumber,
			ReleaseDate:         strings.TrimSpace(setData.ReleaseDate),
			Secret:              false,
			PriceProviderCardID: storedProviderCardID,
			TCGPlayerID:         tcgPlayerID,
			Rarity:              rarity,
			CardType:            cardType,
			HP:                  hp,
			Stage:               stage,
			CardText:            cardText,
			Attacks:             attacks,
			Weakness:            weakness,
			Resistance:          resistance,
			RetreatCost:         retreatCost,
			Artist:              "",
			ImageBaseURL:        imageBase,
			ImageURL:            cardImagePNG(imageBase),
		})
	}
	return out, nil
}

func (p *Provider) fetchTCGDexCardImageMap(ctx context.Context, locator setLocator, language string) map[string]string {
	imagesByNumber := make(map[string]string)
	tcgdexID := strings.TrimSpace(locator.TCGDexID)
	if tcgdexID == "" {
		tcgdexID = strings.TrimSpace(locator.SetCode)
	}
	if tcgdexID == "" {
		return imagesByNumber
	}

	var detail setDetailDTO
	if err := p.getJSON(ctx, setDetailEndpoint(language, tcgdexID), &detail); err != nil {
		// Optional enrichment: keep card sync working even if this endpoint fails.
		return imagesByNumber
	}
	for _, card := range detail.Cards {
		imageBase := strings.TrimSpace(card.Image)
		if imageBase == "" {
			continue
		}
		for _, key := range numberLookupKeys(card.LocalID) {
			if key == "" {
				continue
			}
			if _, exists := imagesByNumber[key]; !exists {
				imagesByNumber[key] = imageBase
			}
		}
	}
	return imagesByNumber
}

func (p *Provider) fetchPokewalletCardsForSet(ctx context.Context, setID string, locator setLocator, language string) (walletSetCardsSetDTO, []walletSetCardDTO, error) {
	identifierCandidates := uniqueNonEmpty(strings.TrimSpace(setID), strings.TrimSpace(locator.SetCode))
	if len(identifierCandidates) == 0 {
		return walletSetCardsSetDTO{}, nil, fmt.Errorf("set %s has no pokewallet identifier", setID)
	}

	langFilter := walletLanguageFilter(language)
	var lastErr error
	for _, identifier := range identifierCandidates {
		page := 1
		allCards := make([]walletSetCardDTO, 0, 200)
		var setMeta walletSetCardsSetDTO
		for {
			endpoint := pokewalletSetCardsEndpoint(identifier, langFilter, page, 200)
			var env walletSetCardsEnvelope
			if err := p.getPokewalletJSON(ctx, endpoint, &env); err != nil {
				lastErr = err
				break
			}
			if strings.TrimSpace(setMeta.SetID.String()) == "" {
				setMeta = env.Set
			}
			allCards = append(allCards, env.Cards...)

			totalPages := env.Pagination.TotalPages
			if totalPages <= 0 {
				if len(env.Cards) < 200 {
					break
				}
				totalPages = page + 1
			}
			if page >= totalPages {
				break
			}
			page++
		}

		if len(allCards) > 0 || strings.TrimSpace(setMeta.SetID.String()) != "" {
			return setMeta, allCards, nil
		}
	}

	if lastErr != nil {
		return walletSetCardsSetDTO{}, nil, lastErr
	}
	return walletSetCardsSetDTO{}, nil, fmt.Errorf("set %s returned no cards", setID)
}

func (p *Provider) lookupSet(setID string) (setLocator, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	loc, ok := p.setLookup[setID]
	return loc, ok
}

func (p *Provider) fetchTCGDexSetIndexes(ctx context.Context) (map[string]map[string]setBriefDTO, error) {
	byLang := map[string]map[string]setBriefDTO{
		"en": {},
		"ja": {},
	}
	for _, language := range []string{"en", "ja"} {
		var briefs []setBriefDTO
		if err := p.getJSON(ctx, fmt.Sprintf("%s/%s/sets", tcgdexBaseURL, language), &briefs); err != nil {
			return nil, err
		}
		index := make(map[string]setBriefDTO, len(briefs))
		for _, brief := range briefs {
			key := normalizeSetCode(brief.ID)
			if key == "" {
				continue
			}
			index[key] = brief
		}
		byLang[language] = index
	}
	return byLang, nil
}

func (p *Provider) fetchPokewalletSets(ctx context.Context) ([]walletSetDTO, error) {
	var env walletSetsEnvelope
	if err := p.getPokewalletJSON(ctx, pokewalletCatalogURL, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func (p *Provider) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s: %w", endpoint, err)
	}
	if p.logger != nil {
		p.logger.LogHTTP(providerLogTCGDex, endpoint, resp.StatusCode, resp.Status, body)
		p.logger.LogJSON(providerLogTCGDex, endpoint, body)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func (p *Provider) getPokewalletJSON(ctx context.Context, endpoint string, out any) error {
	body, err := p.doPokewalletRequest(ctx, endpoint)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", endpoint, err)
	}
	return nil
}

func (p *Provider) doPokewalletRequest(ctx context.Context, endpoint string) ([]byte, error) {
	if len(p.apiKeys) == 0 {
		return nil, fmt.Errorf("no Pokewallet API keys configured")
	}

	var lastErr error
	for _, apiKey := range p.apiKeys {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request %s failed: %w", endpoint, err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read %s: %w", endpoint, readErr)
			continue
		}

		if p.logger != nil {
			p.logger.LogHTTP(providerLogPokewallet, endpoint, resp.StatusCode, resp.Status, body)
			p.logger.LogJSON(providerLogPokewallet, endpoint, body)
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
			continue
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("request %s failed: %s (%s)", endpoint, resp.Status, strings.TrimSpace(string(body)))
		}
		return body, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no usable Pokewallet API keys")
}

func pickTCGDexSet(byLang map[string]map[string]setBriefDTO, language string, setCode string) (setBriefDTO, bool) {
	key := normalizeSetCode(setCode)
	if key == "" {
		return setBriefDTO{}, false
	}
	if item, ok := byLang[language][key]; ok {
		return item, true
	}
	if item, ok := byLang["en"][key]; ok {
		return item, true
	}
	if item, ok := byLang["ja"][key]; ok {
		return item, true
	}
	return setBriefDTO{}, false
}

func normalizeWalletLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jap", "ja", "japanese":
		return "ja"
	case "eng", "en", "english":
		return "en"
	default:
		return ""
	}
}

func toLanguageLabel(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "en":
		return "English"
	case "ja":
		return "Japanese"
	default:
		return strings.TrimSpace(language)
	}
}

func normalizeSetCode(code string) string {
	value := strings.TrimSpace(strings.ToLower(code))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func cleanWalletSetName(raw string, setCode string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	idx := strings.Index(name, ":")
	if idx <= 0 || idx+1 >= len(name) {
		return name
	}
	prefix := strings.TrimSpace(name[:idx])
	suffix := strings.TrimSpace(name[idx+1:])
	if suffix == "" {
		return name
	}
	if normalizeSetCode(prefix) == normalizeSetCode(setCode) {
		return suffix
	}
	if likelySetCodeRE.MatchString(prefix) {
		return suffix
	}
	return name
}

func ensurePNG(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	base := path.Base(value)
	if strings.Contains(base, ".") {
		return value
	}
	return value + ".png"
}

func cardImagePNG(baseURL string) string {
	value := strings.TrimSpace(baseURL)
	if value == "" {
		return ""
	}
	return strings.TrimRight(value, "/") + "/high.png"
}

func englishNameForLanguage(language string, name string) string {
	if strings.EqualFold(strings.TrimSpace(language), "en") {
		return strings.TrimSpace(name)
	}
	return ""
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func walletLanguageFilter(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "japanese", "jap":
		return "jap"
	case "en", "english", "eng":
		return "eng"
	default:
		return ""
	}
}

func pokewalletSetCardsEndpoint(identifier string, language string, page int, limit int) string {
	values := url.Values{}
	values.Set("page", strconv.Itoa(page))
	values.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(language) != "" {
		values.Set("language", strings.TrimSpace(language))
	}
	return fmt.Sprintf("%s/%s?%s", pokewalletCatalogURL, url.PathEscape(strings.TrimSpace(identifier)), values.Encode())
}

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func numberLookupKeys(raw string) []string {
	local := util.CardLocalNumber(raw)
	canonical := util.NormalizeCardNumber(local)
	if canonical == "" {
		return nil
	}
	keys := []string{canonical}
	if isDigitsOnly(canonical) {
		if normalized, err := strconv.Atoi(canonical); err == nil {
			plain := strconv.Itoa(normalized)
			if plain != canonical {
				keys = append(keys, plain)
			}
		}
	}
	return keys
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func extractTCGPlayerID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := range parts {
		if parts[i] != "product" {
			continue
		}
		if i+1 < len(parts) {
			id := strings.TrimSpace(parts[i+1])
			if _, err := strconv.Atoi(id); err == nil {
				return id
			}
		}
	}
	return ""
}

func storedProviderCardID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "pk_") {
		return strings.TrimPrefix(trimmed, "pk_")
	}
	return trimmed
}

func storedCardID(rawProviderCardID string, tcgPlayerID string) string {
	if strings.TrimSpace(tcgPlayerID) != "" {
		return strings.TrimSpace(tcgPlayerID)
	}
	return storedProviderCardID(rawProviderCardID)
}

func setDetailEndpoint(language string, setID string) string {
	id := strings.TrimSpace(setID)
	escaped := url.PathEscape(id)
	escaped = strings.ReplaceAll(escaped, "+", "%2B")
	return fmt.Sprintf("%s/%s/sets/%s", tcgdexBaseURL, language, escaped)
}

func looksJapaneseSetID(setID string) bool {
	id := strings.ToLower(strings.TrimSpace(setID))
	if id == "" {
		return false
	}
	return japaneseSetIDRE.MatchString(id)
}

type walletSetsEnvelope struct {
	Success bool           `json:"success"`
	Data    []walletSetDTO `json:"data"`
}

type walletSetDTO struct {
	Name        string      `json:"name"`
	SetCode     string      `json:"set_code"`
	SetID       stringValue `json:"set_id"`
	CardCount   int         `json:"card_count"`
	Language    string      `json:"language"`
	ReleaseDate string      `json:"release_date"`
}

type walletSetCardsEnvelope struct {
	Success        bool                 `json:"success"`
	Set            walletSetCardsSetDTO `json:"set"`
	Cards          []walletSetCardDTO   `json:"cards"`
	Pagination     walletPaginationDTO  `json:"pagination"`
	Disambiguation bool                 `json:"disambiguation"`
	Matches        []walletSetDTO       `json:"matches"`
}

type walletPaginationDTO struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type walletSetCardsSetDTO struct {
	Name        string      `json:"name"`
	SetCode     string      `json:"set_code"`
	SetID       stringValue `json:"set_id"`
	TotalCards  int         `json:"total_cards"`
	Language    string      `json:"language"`
	ReleaseDate string      `json:"release_date"`
}

type walletSetCardDTO struct {
	ID        string                 `json:"id"`
	CardInfo  walletSetCardInfoDTO   `json:"card_info"`
	TCGPlayer *walletSetTCGPlayerDTO `json:"tcgplayer"`
}

type walletSetCardInfoDTO struct {
	Name           string      `json:"name"`
	CleanName      string      `json:"clean_name"`
	SetName        string      `json:"set_name"`
	SetCode        string      `json:"set_code"`
	SetID          stringValue `json:"set_id"`
	CardNumber     string      `json:"card_number"`
	TotalSetNumber string      `json:"total_set_number"`
	Rarity         string      `json:"rarity"`
	CardType       string      `json:"card_type"`
	HP             stringValue `json:"hp"`
	Stage          string      `json:"stage"`
	CardText       string      `json:"card_text"`
	Attacks        []string    `json:"attacks"`
	Weakness       string      `json:"weakness"`
	Resistance     string      `json:"resistance"`
	RetreatCost    stringValue `json:"retreat_cost"`
}

type walletSetTCGPlayerDTO struct {
	URL string `json:"url"`
}

type setBriefDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Logo      string `json:"logo"`
	Symbol    string `json:"symbol"`
	CardCount struct {
		Total    int `json:"total"`
		Official int `json:"official"`
	} `json:"cardCount"`
}

type setDetailDTO struct {
	Cards []setDetailCardDTO `json:"cards"`
}

type setDetailCardDTO struct {
	LocalID string `json:"localId"`
	Image   string `json:"image"`
}

type stringValue string

func (s *stringValue) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*s = ""
		return nil
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = stringValue(strings.TrimSpace(value))
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*s = stringValue(strings.TrimSpace(number.String()))
		return nil
	}
	return nil
}

func (s stringValue) String() string {
	return strings.TrimSpace(string(s))
}
