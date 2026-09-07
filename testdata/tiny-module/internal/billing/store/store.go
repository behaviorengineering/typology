package store

import (
	"example.com/tiny/internal/config"
	"example.com/tiny/internal/ledger"
)

// Total sums ledger balance for billing.
func Total() int {
	_ = config.Name()
	return ledger.Balance()
}
