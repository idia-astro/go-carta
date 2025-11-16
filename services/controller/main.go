package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/controller/internal/session"
)

var (
	port           = flag.Int("port", 8081, "TCP server port")
	hostname       = flag.String("hostname", "", "Hostname to listen on")
	spawnerAddress = flag.String("spawnerAddress", "http://localhost:8080", "Address of the process spawner")
	baseFolder     = flag.String("baseFolder", "", "Base folder to use")
)

var upgrader = websocket.Upgrader{
	// Ignore Origin header
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Print("Client connected")
	defer helpers.CloseOrLog(c)

	subCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	s := session.Session{
		SpawnerAddress: *spawnerAddress,
		BaseFolder:     *baseFolder,
		WebSocket:      c,
		Context:        subCtx,
	}

	// Close worker on exit if it exists
	defer s.HandleDisconnect()

	// Basic handler based on gorilla/websocket example
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			err := c.WriteMessage(websocket.TextMessage, []byte("PONG"))
			if err != nil {
				log.Printf("Failed to send pong message: %v\n", err)
			}
			continue
		}

		// Ignore all other non-binary messages
		if messageType != websocket.BinaryMessage {
			log.Printf("Ignoring non-binary message: %s\n", message)
			continue
		}

		go func() {
			err := s.HandleMessage(message)
			if err != nil {
				log.Printf("Failed to handle message: %v\n", err)
			}
		}()
	}

	// defer should shut down the worker afterwards
	log.Print("Client disconnected")
}

func main() {
	flag.Parse()

	id := uuid.New()
	log.Printf("Starting controller with UUID: %s\n", id.String())

	// Get
	if len(strings.TrimSpace(*baseFolder)) == 0 {
		dirname, err := os.UserHomeDir()
		if err != nil {
			dirname = "/"
		}
		err = flag.Set("baseFolder", dirname)
		if err != nil {
			log.Fatalf("Failed to set --baseFolder: %v\n", err)
		}
	}

	http.HandleFunc("/carta", wsHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *hostname, *port), nil))
}
