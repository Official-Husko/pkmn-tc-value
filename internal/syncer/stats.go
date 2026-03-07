package syncer

type Stats struct {
	NewSets      int
	UpdatedSets  int
	NewCards     int
	UpdatedCards int
}

type StartupProgress struct {
	Stage      string
	Status     string
	SetsDone   int
	SetsTotal  int
	CardsDone  int
	CardsTotal int
	CurrentSet string
}
