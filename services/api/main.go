package main

import (
	"fmt"
	"github.com/google/uuid"
)

func main() {
	id := uuid.New()
	fmt.Printf("Started HTTP API with UUID: %s\n", id.String())
}
