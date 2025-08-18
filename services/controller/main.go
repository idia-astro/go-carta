package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "idia-astro/go-carta/pkg/grpc"
	"idia-astro/go-carta/pkg/shared"
)

var (
	remoteAddress = flag.String("remoteAddress", "localhost:8080", "Address of the worker")
)

func main() {
	flag.Parse()

	id := uuid.New()
	log.Printf("Starting controller with UUID: %s\n", id.String())

	conn, err := grpc.NewClient(*remoteAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Coult not connect to worker: %v", err)
	}
	defer helpers.CloseOrLog(conn)

	fileInfoClient := pb.NewFileInfoServiceClient(conn)
	fileListClient := pb.NewFileListServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := fileInfoClient.CheckStatus(ctx, &pb.Empty{})
	if err != nil {
		log.Fatalf("could not check fileInfoService status: %v", err)
	}
	log.Printf("Response from fileInfoService (instanceID: %s): %s", r.InstanceId, r.StatusMessage)

	r, err = fileListClient.CheckStatus(ctx, &pb.Empty{})
	if err != nil {
		log.Fatalf("could not check fileListService status: %v", err)
	}
	log.Printf("Response from fileListService (instanceID: %s): %s", r.InstanceId, r.StatusMessage)

}
