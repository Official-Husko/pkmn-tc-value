package util

import (
	"fmt"

	"github.com/Official-Husko/pkmn-tc-value/internal/domain"
)

func FormatMoney(m *domain.Money) string {
	if m == nil {
		return "N/A"
	}
	currency := m.Currency
	if currency == "" || currency == "USD" {
		return fmt.Sprintf("$%.2f", m.Amount)
	}
	return fmt.Sprintf("%s %.2f", currency, m.Amount)
}
