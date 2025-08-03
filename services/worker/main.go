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
)

var (
	remoteAddress = flag.String("remoteAddress", "localhost:8080", "Address of the controller server")
)

func main() {
	id := uuid.New()
	log.Printf("Starting worker with UUID: %s\n", id.String())

	flag.Parse()
	conn, err := grpc.NewClient(*remoteAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Coult not connect to controller: %v", err)
	}
	defer conn.Close()

	c := pb.NewFileServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.CheckStatus(ctx, &pb.Empty{})
	if err != nil {
		log.Fatalf("could not check controller status: %v", err)
	}
	log.Printf("Response from controller: %s", r.StatusMessage)
}
