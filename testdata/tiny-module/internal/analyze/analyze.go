package analyze

import (
	"example.com/tiny/internal/config"
	"example.com/tiny/internal/ledger"
)

// Request is analysis input. JSON tags alone do not make this a dto package
// because the package also exports analysis logic.
type Request struct {
	Log string `json:"log"`
}

// Response is analysis output.
type Response struct {
	Summary string `json:"summary"`
}

// Run analyzes a request using shared settings and ledger context.
func Run(req Request) Response {
	_ = config.Name()
	_ = ledger.Balance()
	return Response{Summary: req.Log}
}
