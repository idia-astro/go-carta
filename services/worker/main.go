package main

import (
	"context"
	"flag"
	"fmt"
	pb "idia-astro/go-carta/pkg/grpc"
	"log"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 8080, "gRPC server port")
)

type fileInfoServer struct {
	pb.UnimplementedFileInfoServiceServer
	instanceId uuid.UUID
}
type fileListServer struct {
	pb.UnimplementedFileListServiceServer
	instanceId uuid.UUID
}

func (s *fileInfoServer) CheckStatus(_ context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	log.Printf("Received CheckStatus message")
	return &pb.StatusResponse{Status: true, StatusMessage: "FileInfoService is running", InstanceId: s.instanceId.String()}, nil
}

func (s *fileListServer) CheckStatus(_ context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	log.Printf("Received CheckStatus message")
	return &pb.StatusResponse{Status: true, StatusMessage: "FileListService is running", InstanceId: s.instanceId.String()}, nil
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
	log.Printf("Starting worker with instance ID: %s\n", id.String())

	// Set up gRPC server
	s := grpc.NewServer()
	pb.RegisterFileListServiceServer(s, &fileListServer{instanceId: id})
	pb.RegisterFileInfoServiceServer(s, &fileInfoServer{instanceId: id})
	log.Printf("server listening at %v", listener.Addr())
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
