package main

import (
	"context"
	"log"

	pb "idia-astro/go-carta/pkg/grpc"
)

func (s *fileInfoServer) CheckStatus(_ context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	log.Printf("Received CheckStatus message")
	return &pb.StatusResponse{Status: true, StatusMessage: "FileInfoService is running", InstanceId: s.instanceId.String()}, nil
}
