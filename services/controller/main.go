package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	pb "idia-astro/go-carta/pkg/grpc"
)

var (
	port = flag.Int("port", 8080, "The server port")
)

// server is used to implement FileServiceServer.
type server struct {
	pb.UnimplementedFileServiceServer
}

// CheckStatus implements FileServiceServer
func (s *server) CheckStatus(_ context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	log.Printf("Received CheckStatus message")
	return &pb.StatusResponse{Status: true, StatusMessage: "OK"}, nil
}

func main() {
	// Parse arguments and start TCP listener
	flag.Parse()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	id := uuid.New()
	log.Printf("Starting controller with instance ID: %s\n", id.String())

	// Set up gRPC server
	s := grpc.NewServer()
	pb.RegisterFileServiceServer(s, &server{})
	log.Printf("server listening at %v", listener.Addr())
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
