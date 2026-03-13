package syncer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Official-Husko/pkmn-tc-value/internal/catalog"
	"github.com/Official-Husko/pkmn-tc-value/internal/config"
	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
	"github.com/Official-Husko/pkmn-tc-value/internal/images"
	"github.com/Official-Husko/pkmn-tc-value/internal/pricing"
	trackerpricing "github.com/Official-Husko/pkmn-tc-value/internal/pricing/pokemonpricetracker"
	"github.com/Official-Husko/pkmn-tc-value/internal/store"
	"github.com/Official-Husko/pkmn-tc-value/internal/util"
)

type SetSyncResult struct {
	SetName       string
	SetID         string
	NewCards      int
	UpdatedCards  int
	TotalCards    int
	ImagesSaved   int
	DetailsSynced int
	DetailsFailed int
}

type SetSyncOptions struct {
	ImageCaching    bool
	SyncCardDetails bool
	Config          config.Config
}

type SetSyncProgress struct {
	Stage  string
	Status string
	Done   int
	Total  int
}

type SetSyncService struct {
	store       *store.Store
	catalog     catalog.Provider
	pricing     pricing.Provider
	images      *images.Downloader
	priceBridge *trackerpricing.Resolver
}

func NewSetSyncService(s *store.Store, c catalog.Provider, p pricing.Provider, i *images.Downloader, bridge *trackerpricing.Resolver) *SetSyncService {
	return &SetSyncService{store: s, catalog: c, pricing: p, images: i, priceBridge: bridge}
}

func (s *SetSyncService) IsSetCached(setID string) (bool, error) {
	var count int
	err := s.store.Read(func(db *store.DB) error {
		count = len(db.CardsBySet[setID])
		return nil
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SetSyncService) SyncSet(ctx context.Context, setID string, opts SetSyncOptions, progress func(SetSyncProgress)) (SetSyncResult, error) {
	progress = withDefaultProgress(progress)

	progress(SetSyncProgress{Stage: "cards", Status: "Fetching cards", Done: 0, Total: 0})
	var set domain.Set
	found := false
	if err := s.store.Read(func(db *store.DB) error {
		set, found = db.Sets[setID]
		return nil
	}); err != nil {
		return SetSyncResult{}, err
	}
	if !found {
		return SetSyncResult{}, fmt.Errorf("set %s not found", setID)
	}

	remoteCards, err := s.catalog.FetchCardsForSet(ctx, setID)
	if err != nil {
		return SetSyncResult{}, err
	}
	matchedPriceSetName := strings.TrimSpace(set.PriceProviderSetName)
	matchedPriceSetCode := strings.TrimSpace(set.PriceProviderSetCode)
	matchedPriceSetID := strings.TrimSpace(set.PriceProviderSetID)
	setEnglishName := strings.TrimSpace(set.EnglishName)
	cardPriceIDByRemoteID := make(map[string]string, len(remoteCards))
	cardEnrichmentByRemoteID := make(map[string]trackerpricing.CardEnrichment, len(remoteCards))
	if s.priceBridge != nil {
		if resolvedSet, matches, mapErr := s.priceBridge.MapSetCards(ctx, set, remoteCards, opts.Config); mapErr == nil {
			if strings.TrimSpace(resolvedSet.ID) != "" {
				matchedPriceSetID = strings.TrimSpace(resolvedSet.ID)
			}
			if strings.TrimSpace(resolvedSet.Name) != "" {
				matchedPriceSetName = strings.TrimSpace(resolvedSet.Name)
			}
			if strings.TrimSpace(resolvedSet.Code) != "" {
				matchedPriceSetCode = strings.TrimSpace(resolvedSet.Code)
			}
			if strings.TrimSpace(resolvedSet.EnglishName) != "" {
				setEnglishName = strings.TrimSpace(resolvedSet.EnglishName)
			}
			for remoteID, enrichment := range matches {
				cardEnrichmentByRemoteID[remoteID] = enrichment
				cardPriceIDByRemoteID[remoteID] = strings.TrimSpace(enrichment.PriceProviderCardID)
			}
		}
	}

	result := SetSyncResult{
		SetID:      setID,
		SetName:    set.Name,
		TotalCards: len(remoteCards),
	}
	progress(SetSyncProgress{Stage: "cards", Status: fmt.Sprintf("Fetched %d cards", len(remoteCards)), Done: len(remoteCards), Total: len(remoteCards)})

	var existingCards map[string]domain.Card
	if err := s.store.Read(func(db *store.DB) error {
		existingCards = db.CardsBySet[setID]
		return nil
	}); err != nil {
		return SetSyncResult{}, err
	}
	if existingCards == nil {
		existingCards = make(map[string]domain.Card)
	}

	nextCards := make(map[string]domain.Card, len(remoteCards))
	for _, remoteCard := range remoteCards {
		cardSetCode := strings.TrimSpace(remoteCard.SetCode)
		if cardSetCode == "" {
			cardSetCode = strings.TrimSpace(set.SetCode)
		}
		remoteCanonical := util.NormalizeCardNumber(remoteCard.Number)
		existing, ok := existingCards[remoteCard.ID]
		if !ok {
			for _, candidate := range existingCards {
				if util.NormalizeCardNumber(candidate.Number) == remoteCanonical {
					existing = candidate
					break
				}
			}
		}
		if cardSetCode == "" {
			cardSetCode = strings.TrimSpace(existing.SetCode)
		}
		priceProviderCardID := strings.TrimSpace(cardPriceIDByRemoteID[remoteCard.ID])
		if priceProviderCardID == "" {
			priceProviderCardID = strings.TrimSpace(remoteCard.PriceProviderCardID)
		}
		if priceProviderCardID == "" {
			priceProviderCardID = strings.TrimSpace(existing.PriceProviderCardID)
		}
		priceProviderSetID := strings.TrimSpace(existing.PriceProviderSetID)
		if priceProviderSetID == "" {
			priceProviderSetID = strings.TrimSpace(set.PriceProviderSetID)
		}
		enrichment := cardEnrichmentByRemoteID[remoteCard.ID]
		if strings.TrimSpace(enrichment.PriceProviderSetID) != "" {
			priceProviderSetID = strings.TrimSpace(enrichment.PriceProviderSetID)
		}
		cardEnglishName := strings.TrimSpace(existing.EnglishName)
		if cardEnglishName == "" {
			cardEnglishName = strings.TrimSpace(remoteCard.EnglishName)
		}
		if strings.TrimSpace(enrichment.EnglishName) != "" {
			cardEnglishName = strings.TrimSpace(enrichment.EnglishName)
		}
		setEnglishNameForCard := strings.TrimSpace(existing.SetEnglishName)
		if setEnglishNameForCard == "" {
			setEnglishNameForCard = strings.TrimSpace(set.EnglishName)
		}
		if setEnglishNameForCard == "" {
			setEnglishNameForCard = strings.TrimSpace(remoteCard.SetEnglishName)
		}
		if strings.TrimSpace(enrichment.SetEnglishName) != "" {
			setEnglishNameForCard = strings.TrimSpace(enrichment.SetEnglishName)
		}
		totalSetNumber := strings.TrimSpace(existing.TotalSetNumber)
		if totalSetNumber == "" {
			totalSetNumber = strings.TrimSpace(remoteCard.TotalSetNumber)
		}
		if strings.TrimSpace(enrichment.TotalSetNumber) != "" {
			totalSetNumber = strings.TrimSpace(enrichment.TotalSetNumber)
		}
		cardType := strings.TrimSpace(existing.CardType)
		if cardType == "" {
			cardType = strings.TrimSpace(remoteCard.CardType)
		}
		if strings.TrimSpace(enrichment.CardType) != "" {
			cardType = strings.TrimSpace(enrichment.CardType)
		}
		hp := strings.TrimSpace(existing.HP)
		if hp == "" {
			hp = strings.TrimSpace(remoteCard.HP)
		}
		stage := strings.TrimSpace(existing.Stage)
		if stage == "" {
			stage = strings.TrimSpace(remoteCard.Stage)
		}
		cardText := strings.TrimSpace(existing.CardText)
		if cardText == "" {
			cardText = strings.TrimSpace(remoteCard.CardText)
		}
		attacks := cloneStrings(existing.Attacks)
		if len(attacks) == 0 && len(remoteCard.Attacks) > 0 {
			attacks = cloneStrings(remoteCard.Attacks)
		}
		weakness := strings.TrimSpace(existing.Weakness)
		if weakness == "" {
			weakness = strings.TrimSpace(remoteCard.Weakness)
		}
		resistance := strings.TrimSpace(existing.Resistance)
		if resistance == "" {
			resistance = strings.TrimSpace(remoteCard.Resistance)
		}
		retreatCost := strings.TrimSpace(existing.RetreatCost)
		if retreatCost == "" {
			retreatCost = strings.TrimSpace(remoteCard.RetreatCost)
		}
		artist := strings.TrimSpace(existing.Artist)
		if artist == "" {
			artist = strings.TrimSpace(remoteCard.Artist)
		}
		if strings.TrimSpace(enrichment.Artist) != "" {
			artist = strings.TrimSpace(enrichment.Artist)
		}
		rarity := strings.TrimSpace(remoteCard.Rarity)
		if rarity == "" {
			rarity = strings.TrimSpace(existing.Rarity)
		}
		if strings.TrimSpace(enrichment.Rarity) != "" {
			rarity = strings.TrimSpace(enrichment.Rarity)
		}
		imageBaseURL := strings.TrimSpace(remoteCard.ImageBaseURL)
		if imageBaseURL == "" {
			imageBaseURL = strings.TrimSpace(existing.ImageBaseURL)
		}
		if strings.TrimSpace(enrichment.ImageBaseURL) != "" {
			imageBaseURL = strings.TrimSpace(enrichment.ImageBaseURL)
		}
		imageURL := strings.TrimSpace(remoteCard.ImageURL)
		if imageURL == "" {
			imageURL = strings.TrimSpace(existing.ImageURL)
		}
		if strings.TrimSpace(enrichment.ImageURL) != "" {
			imageURL = strings.TrimSpace(enrichment.ImageURL)
		}
		tcgPlayerID := strings.TrimSpace(existing.TCGPlayerID)
		if tcgPlayerID == "" {
			tcgPlayerID = strings.TrimSpace(remoteCard.TCGPlayerID)
		}
		if strings.TrimSpace(enrichment.TCGPlayerID) != "" {
			tcgPlayerID = strings.TrimSpace(enrichment.TCGPlayerID)
		}
		if tcgPlayerID == "" {
			tcgPlayerID = priceProviderCardID
		}
		if priceProviderCardID == "" {
			priceProviderCardID = tcgPlayerID
		}
		if existing.ID == "" {
			result.NewCards++
		} else if existing.Name != remoteCard.Name ||
			existing.Rarity != rarity ||
			existing.ImageBaseURL != imageBaseURL ||
			existing.ImageURL != imageURL ||
			existing.PriceProviderCardID != priceProviderCardID ||
			existing.EnglishName != cardEnglishName ||
			existing.CardType != cardType ||
			existing.HP != hp ||
			existing.Stage != stage ||
			existing.CardText != cardText ||
			existing.Weakness != weakness ||
			existing.Resistance != resistance ||
			existing.RetreatCost != retreatCost ||
			!equalStrings(existing.Attacks, attacks) {
			result.UpdatedCards++
		}
		nextCards[remoteCard.ID] = domain.Card{
			ID:                  remoteCard.ID,
			SetID:               remoteCard.SetID,
			SetName:             remoteCard.SetName,
			SetEnglishName:      setEnglishNameForCard,
			SetCode:             cardSetCode,
			PriceProviderSetID:  priceProviderSetID,
			PriceProviderCardID: priceProviderCardID,
			Language:            remoteCard.Language,
			Name:                remoteCard.Name,
			EnglishName:         cardEnglishName,
			Number:              util.CardLocalNumber(remoteCard.Number),
			TotalSetNumber:      totalSetNumber,
			ReleaseDate:         remoteCard.ReleaseDate,
			Secret:              remoteCard.Secret,
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
			Artist:              artist,
			ImageBaseURL:        imageBaseURL,
			ImageURL:            imageURL,
			ImagePath:           existing.ImagePath,
			UngradedPrice:       existing.UngradedPrice,
			LowPrice:            existing.LowPrice,
			PSA10Price:          existing.PSA10Price,
			GradeWorth:          existing.GradeWorth,
			UngradedSmartPrice:  existing.UngradedSmartPrice,
			UngradedSmartMeta:   existing.UngradedSmartMeta,
			SalesVelocity:       existing.SalesVelocity,
			TotalSales:          existing.TotalSales,
			TotalSalesValue:     existing.TotalSalesValue,
			RecentSales:         existing.RecentSales,
			Population:          existing.Population,
			PriceSourceURL:      existing.PriceSourceURL,
			PriceCheckedAt:      existing.PriceCheckedAt,
			CatalogUpdatedAt:    remoteCard.CatalogUpdatedAt,
		}
	}

	orderedIDs := make([]string, 0, len(nextCards))
	for cardID := range nextCards {
		orderedIDs = append(orderedIDs, cardID)
	}
	sort.Strings(orderedIDs)

	discoveredSetCode := strings.TrimSpace(set.SetCode)
	if discoveredSetCode == "" {
		for _, cardID := range orderedIDs {
			candidate := strings.TrimSpace(nextCards[cardID].SetCode)
			if candidate != "" {
				discoveredSetCode = candidate
				break
			}
		}
	}

	if opts.ImageCaching && s.images != nil {
		total := len(orderedIDs)
		done := 0
		progress(SetSyncProgress{Stage: "images", Status: "Downloading card images", Done: done, Total: total})
		workers := configuredImageWorkers(opts.Config.ImageDownloadWorkers, total)
		if workers > 0 {
			type imageJob struct {
				cardID string
				card   domain.Card
			}
			type imageResult struct {
				cardID string
				path   string
				err    error
			}

			jobs := make(chan imageJob, total)
			for _, cardID := range orderedIDs {
				jobs <- imageJob{cardID: cardID, card: nextCards[cardID]}
			}
			close(jobs)

			results := make(chan imageResult, total)
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						path, err := s.images.Ensure(ctx, job.card)
						select {
						case results <- imageResult{cardID: job.cardID, path: path, err: err}:
						case <-ctx.Done():
							return
						}
						if ctx.Err() != nil {
							return
						}
					}
				}()
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			for res := range results {
				done++
				if res.err == nil && res.path != "" {
					card := nextCards[res.cardID]
					if card.ImagePath != res.path {
						card.ImagePath = res.path
						nextCards[res.cardID] = card
						result.ImagesSaved++
					}
				}
				progress(SetSyncProgress{
					Stage:  "images",
					Status: fmt.Sprintf("Downloading card images (%d/%d, %d workers)", done, total, workers),
					Done:   done,
					Total:  total,
				})
			}
		}
	}

	if opts.SyncCardDetails && s.pricing != nil {
		total := len(orderedIDs)
		done := 0
		progress(SetSyncProgress{Stage: "details", Status: "Downloading card details", Done: done, Total: total})
		for _, cardID := range orderedIDs {
			done++
			card := nextCards[cardID]
			snapshot, err := s.pricing.RefreshCard(ctx, card, set, opts.Config)
			if err != nil {
				result.DetailsFailed++
			} else {
				card.UngradedPrice = snapshot.Ungraded
				card.LowPrice = snapshot.Low
				card.PSA10Price = snapshot.PSA10
				card.GradeWorth = snapshot.GradeWorth
				card.UngradedSmartPrice = snapshot.UngradedSmartPrice
				card.UngradedSmartMeta = snapshot.UngradedSmartMeta
				card.SalesVelocity = snapshot.SalesVelocity
				card.TotalSales = snapshot.TotalSales
				card.TotalSalesValue = snapshot.TotalSalesValue
				card.RecentSales = snapshot.RecentSales
				card.Population = snapshot.Population
				card.PriceSourceURL = snapshot.SourceURL
				card.PriceCheckedAt = &snapshot.CheckedAt
				if strings.TrimSpace(snapshot.PriceProviderCardID) != "" {
					card.PriceProviderCardID = snapshot.PriceProviderCardID
				}
				if strings.TrimSpace(snapshot.TCGPlayerID) != "" {
					card.TCGPlayerID = snapshot.TCGPlayerID
				}
				if strings.TrimSpace(snapshot.PriceProviderSetID) != "" {
					card.PriceProviderSetID = snapshot.PriceProviderSetID
					matchedPriceSetID = snapshot.PriceProviderSetID
				}
				if strings.TrimSpace(snapshot.PriceProviderSetName) != "" {
					matchedPriceSetName = snapshot.PriceProviderSetName
				}
				if strings.TrimSpace(snapshot.PriceProviderSetCode) != "" {
					matchedPriceSetCode = snapshot.PriceProviderSetCode
				}
				if strings.TrimSpace(snapshot.SetName) != "" {
					card.SetName = snapshot.SetName
				}
				if strings.TrimSpace(snapshot.CardName) != "" {
					card.Name = snapshot.CardName
				}
				if strings.TrimSpace(snapshot.CardNumber) != "" {
					card.Number = util.CardLocalNumber(snapshot.CardNumber)
				}
				if strings.TrimSpace(snapshot.TotalSetNumber) != "" {
					card.TotalSetNumber = snapshot.TotalSetNumber
				}
				if strings.TrimSpace(snapshot.Rarity) != "" {
					card.Rarity = snapshot.Rarity
				}
				if strings.TrimSpace(snapshot.CardType) != "" {
					card.CardType = snapshot.CardType
				}
				if strings.TrimSpace(snapshot.Artist) != "" {
					card.Artist = snapshot.Artist
				}
				if strings.TrimSpace(snapshot.ImageURL) != "" {
					card.ImageURL = snapshot.ImageURL
				}
				if strings.TrimSpace(snapshot.ImageBaseURL) != "" {
					card.ImageBaseURL = snapshot.ImageBaseURL
				}
				nextCards[cardID] = card
				result.DetailsSynced++
			}
			progress(SetSyncProgress{
				Stage:  "details",
				Status: fmt.Sprintf("Downloading card details (%d/%d)", done, total),
				Done:   done,
				Total:  total,
			})
		}
	}

	err = s.store.Update(func(db *store.DB) error {
		db.CardsBySet[setID] = nextCards
		setRecord := db.Sets[setID]
		setRecord.Total = len(nextCards)
		setRecord.Cards.Total = len(nextCards)
		if discoveredSetCode != "" {
			setRecord.SetCode = discoveredSetCode
		}
		if matchedPriceSetName != "" {
			setRecord.PriceProviderSetName = matchedPriceSetName
		}
		if matchedPriceSetID != "" {
			setRecord.PriceProviderSetID = matchedPriceSetID
		}
		if matchedPriceSetCode != "" {
			setRecord.PriceProviderSetCode = matchedPriceSetCode
		}
		if setEnglishName != "" {
			setRecord.EnglishName = setEnglishName
		}
		db.Sets[setID] = setRecord
		return nil
	})
	return result, err
}

func withDefaultProgress(progress func(SetSyncProgress)) func(SetSyncProgress) {
	if progress == nil {
		return func(SetSyncProgress) {}
	}
	return progress
}

func configuredImageWorkers(configured int, total int) int {
	if total <= 0 {
		return 0
	}
	if configured < 1 {
		configured = 1
	}
	if configured > total {
		configured = total
	}
	return configured
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
