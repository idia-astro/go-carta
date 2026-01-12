package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	pb "github.com/idia-astro/go-carta/pkg/grpc"
	utils "github.com/idia-astro/go-carta/pkg/shared"
	"github.com/idia-astro/go-carta/services/worker/fitsWrapper"
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

	logger := utils.NewLogger("worker", "debug")
	slog.SetDefault(logger)

	fitsWrapper.TestWrapper("/Users/angus/cubes/m422.fits")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}
	defer utils.CloseOrLog(listener)

	id := uuid.New()
	slog.Info("Starting worker with instance ID", "instanceId", id.String())

	// Set up gRPC server
	s := grpc.NewServer()
	pb.RegisterFileListServiceServer(s, &fileListServer{instanceId: id})
	pb.RegisterFileInfoServiceServer(s, &fileInfoServer{instanceId: id})
	slog.Info("server listening", "address", listener.Addr())
	if err := s.Serve(listener); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
