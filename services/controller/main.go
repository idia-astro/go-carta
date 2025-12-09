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

	helpers "github.com/idia-astro/go-carta/pkg/shared"
	"github.com/idia-astro/go-carta/services/controller/internal/session"
)

var (
	port           = flag.Int("port", 8081, "TCP server port")
	hostname       = flag.String("hostname", "", "Hostname to listen on")
	spawnerAddress = flag.String("spawnerAddress", "http://localhost:8080", "Address of the process spawner")
	baseFolder     = flag.String("baseFolder", "", "Base folder to use")
	frontendDir    = flag.String("frontendDir", "", "Path to built carta_frontend assets (e.g. /path/to/carta_frontend/build)")
)

var upgrader = websocket.Upgrader{
	// Ignore Origin header
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// spaHandler serves static files if they exist, otherwise falls back to index.html
type spaHandler struct {
	fs http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		wsHandler(w, r)
	} else {
		h.fs.ServeHTTP(w, r)
	}

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
	s.HandleConnection()
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

	// Default baseFolder to $HOME if unset
	if len(strings.TrimSpace(*baseFolder)) == 0 {
		dirname, err := os.UserHomeDir()
		if err != nil {
			dirname = "/"
		}
		if err := flag.Set("baseFolder", dirname); err != nil {
			log.Fatalf("Failed to set --baseFolder: %v\n", err)
		}
	}

	// If a frontend directory is provided, serve carta_frontend from there
	if *frontendDir != "" {
		info, err := os.Stat(*frontendDir)
		if err != nil || !info.IsDir() {
			log.Fatalf("Invalid --frontendDir %q: %v\n", *frontendDir, err)
		}

		log.Printf("Serving carta_frontend from %s\n", *frontendDir)
		http.Handle("/", spaHandler{fs: http.FileServer(http.Dir(*frontendDir))})
	} else {
		log.Print("No --frontendDir supplied: controller will *not* serve the frontend (only /carta WebSocket).")
		http.HandleFunc("/carta", wsHandler)
	}

	addr := fmt.Sprintf("%s:%d", *hostname, *port)
	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))

}
