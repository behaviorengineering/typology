package kitchen

import (
	"example.com/tiny/internal/board"
	"example.com/tiny/internal/config"
)

// Service builds page rows from adapters.
type Service struct{}

// New builds the page-data aggregator.
func New() *Service {
	return &Service{}
}

// Collect assembles board items using shared settings.
func (s *Service) Collect() []board.Item {
	_ = config.Name()
	return []board.Item{{ID: "1", Title: "demo"}}
}
