module example.com/tiny

go 1.26.5

require google.golang.org/grpc v0.0.0

replace google.golang.org/grpc => ./third_party/grpcstub
