module example.com/tiny

go 1.26.5

require gopkg.in/yaml.v3 v3.0.1

require go.opentelemetry.io/otel v0.0.0
require google.golang.org/grpc v0.0.0

replace go.opentelemetry.io/otel => ./third_party/otelstub
replace google.golang.org/grpc => ./third_party/grpcstub
