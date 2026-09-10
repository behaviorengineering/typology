package grpcserver

import "google.golang.org/grpc"

// Server is a gRPC delivery surface.
type Server struct{}

// RegisterDemoServer is the mechanical signal for a gRPC delivery package.
func RegisterDemoServer(s *grpc.Server, srv *Server) {
	_, _ = s, srv
}
