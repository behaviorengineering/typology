// Package otel is a local stub so tiny-module can import go.opentelemetry.io/otel
// without pulling the real SDK. Typology classifies from the import path string.
package otel

// Tracer returns a no-op handle.
func Tracer(name string) struct{} {
	_ = name
	return struct{}{}
}
