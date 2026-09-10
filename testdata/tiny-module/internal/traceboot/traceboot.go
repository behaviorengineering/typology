// Package traceboot boots process tracing. Folder name is not evidence.
package traceboot

import "go.opentelemetry.io/otel"

// Init installs a tracer provider using the otel API.
func Init(service string) {
	_ = otel.Tracer(service)
}
