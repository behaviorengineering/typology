package ledger

import "example.com/tiny/internal/config"

// Balance returns a stub balance.
func Balance() int {
	_ = config.Name()
	return 0
}
