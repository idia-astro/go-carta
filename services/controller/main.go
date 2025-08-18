package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers/types"
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
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)

		cartaMessage, err := cartaHelpers.GetActionMessage(message)
		if err != nil {
			log.Printf("Failed to unmarshal message: %v\n", err)
		}

		switch cartaMessage.Action {
		case types.RegisterViewer:
			workerInfo, err = handlers.HandleRegisterViewerMessage(cartaMessage, c, *spawnerAddress)
		case types.FileListRequest:
			//
		case types.FileInfoRequest:
			//
		case types.OpenFile:
			//
		case types.StatusRequest:
			if workerInfo == nil {
				// TODO: send error
				break
			}
			status, err := spawnerHelpers.GetWorkerStatus(workerInfo.WorkerId, *spawnerAddress)
			if err != nil {
				log.Printf("Error getting worker status: %v", err)
			} else {
				log.Printf("Worker status: Alive: %v, Reachable: %v", status.Alive, status.IsReachable)
			}
		default:
			log.Printf("Ignoring unknown action: %s", cartaMessage.Action)
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
