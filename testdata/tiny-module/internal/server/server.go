package server

import (
	"embed"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// Mux is the HTTP delivery surface.
type Mux struct {
	h http.Handler
}

// NewMux builds the browser/API mux.
func NewMux() *Mux {
	return &Mux{h: http.DefaultServeMux}
}

// ServeHTTP implements http.Handler.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.h.ServeHTTP(w, r)
}
