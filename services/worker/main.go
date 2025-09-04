package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	pb "idia-astro/go-carta/pkg/grpc"
	utils "idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/worker/fitsWrapper"
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

func main() {
	flag.Parse()
	fitsWrapper.TestWrapper("/Users/angus/cubes/m422.fits")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer utils.CloseOrLog(listener)

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
