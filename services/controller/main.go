package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/handlers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

var (
	port           = flag.Int("port", 8081, "TCP server port")
	hostname       = flag.String("hostname", "", "Hostname to listen on")
	spawnerAddress = flag.String("spawnerAddress", "http://localhost:8080", "Address of the process spawner")
)

var upgrader = websocket.Upgrader{
	// Ignore Origin header
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Print("Client connected")
	defer helpers.CloseOrLog(c)

	var workerInfo *spawnerHelpers.WorkerInfo = nil

	// Close worker on exit if it exists
	defer func() {
		if workerInfo == nil {
			return
		}
		err := spawnerHelpers.RequestWorkerShutdown(workerInfo.WorkerId, *spawnerAddress)
		if err != nil {
			log.Printf("Error shutting down worker: %v", err)
		}
		log.Printf("Shut down worker with UUID: %s", workerInfo.WorkerId)
	}()

	// Placeholder echo server based on gorilla/websocket example
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

		// Message prefix is used for determining message type and matching requests to responses
		prefix, err := cartaHelpers.DecodeMessagePrefix(message)
		if err != nil {
			log.Printf("Failed to unmarshal message: %v\n", err)
		}

		switch prefix.EventType {
		case cartaDefinitions.EventType_REGISTER_VIEWER:
			workerInfo, err = handlers.HandleRegisterViewerMessage(message[8:], prefix.RequestId, c, *spawnerAddress)
		case cartaDefinitions.EventType_FILE_LIST_REQUEST, cartaDefinitions.EventType_FILE_INFO_REQUEST, cartaDefinitions.EventType_OPEN_FILE:
			log.Printf("Not implemented yet: %s (request id: %d)", prefix.EventType, prefix.RequestId)
		case cartaDefinitions.EventType_EMPTY_EVENT:
			if workerInfo == nil {
				log.Println("Ignoring status request received before worker registration")
				break
			}
			status, err := spawnerHelpers.GetWorkerStatus(workerInfo.WorkerId, *spawnerAddress)
			if err != nil {
				log.Printf("Error getting worker status: %v", err)
			} else {
				log.Printf("Worker status: Alive: %v, Reachable: %v", status.Alive, status.IsReachable)
			}
		default:
			log.Printf("Ignoring unknown action type: %s (request id: %d)", prefix.EventType, prefix.RequestId)
		}

		if err != nil {
			log.Printf("Error handling message: %v", err)
			break
		}
	}

	// defer should shut down the worker afterwards
	log.Print("Client disconnected")
}

func main() {
	flag.Parse()

	id := uuid.New()
	log.Printf("Starting controller with UUID: %s\n", id.String())

	http.HandleFunc("/carta", echo)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *hostname, *port), nil))
}
