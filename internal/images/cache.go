package images

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

type Cache struct {
	root string
}

func NewCache(root string) *Cache {
	return &Cache{root: root}
}

func (c *Cache) Path(card domain.Card) string {
	return filepath.Join(c.root, card.SetID, card.ID+".png")
}

func (c *Cache) EnsureDir(card domain.Card) error {
	return os.MkdirAll(filepath.Dir(c.Path(card)), 0o755)
}

func (c *Cache) Exists(card domain.Card) bool {
	_, err := os.Stat(c.Path(card))
	return err == nil
}

func (c *Cache) Validate() error {
	if c.root == "" {
		return fmt.Errorf("image cache root is empty")
	}
	return os.MkdirAll(c.root, 0o755)
}
