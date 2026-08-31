package httpapi

import "example.com/tiny/internal/billing/store"

// Handle serves billing totals.
func Handle() int {
	return store.Total()
}
