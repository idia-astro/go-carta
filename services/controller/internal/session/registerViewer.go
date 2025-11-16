package session

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

// RegisterViewer is a special case as it is the first message we receive and is used to spin up the worker connection and set up the proxy handler
func (s *Session) handleRegisterViewerMessage(_ cartaDefinitions.EventType, requestId uint32, msg []byte) error {
	var payload cartaDefinitions.RegisterViewer
	err := s.checkAndParse(&payload, requestId, msg)
	if err != nil {
		return fmt.Errorf("error parsing message: %v", err)
	}

	info, err := spawnerHelpers.RequestWorkerStartup(s.SpawnerAddress)
	if err != nil {
		return fmt.Errorf("error starting worker: %v", err)
	}
	s.Info = info

	log.Printf("Worker %s started for session %v and is available at %s:%d", info.WorkerId, payload.SessionId, info.Address, info.Port)
	addr := fmt.Sprintf("ws://%s:%d", info.Address, info.Port)
	workerConn, _, err := websocket.DefaultDialer.DialContext(s.Context, addr, nil)
	if err != nil {
		return fmt.Errorf("could not connect to worker at %s: %w", addr, err)
	}
	s.WorkerConn = workerConn

	// Initialize the worker message channel with a buffer
	s.workerSendChan = make(chan []byte, 100)

	// Start up the message sender and proxy handler
	go sendHandler(s.workerSendChan, s.WorkerConn, "worker")
	go s.workerMessageHandler()

	return s.proxyMessageToWorker(&payload, cartaDefinitions.EventType_REGISTER_VIEWER, requestId)
}
