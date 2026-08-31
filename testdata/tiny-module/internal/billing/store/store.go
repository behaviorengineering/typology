package store

import "example.com/tiny/internal/ledger"

// Total sums ledger balance for billing.
func Total() int {
	return ledger.Balance()
}
