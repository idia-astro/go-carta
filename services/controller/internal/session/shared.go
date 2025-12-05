package session

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func sendHandler(channel <-chan []byte, conn *websocket.Conn, name string) {
	for byteData := range channel {
		byteLength := len(byteData)
		err := conn.WriteMessage(websocket.BinaryMessage, byteData)
		remaining := len(channel)
		log.Printf("Sent message of length %v bytes to %s, %d buffered messages remaining", byteLength, name, remaining)
		if err != nil {
			log.Printf("Error sending message to %s: %v", name, err)
			// Continue processing other messages even if one fails
		}
	}
}

// handleProxiedMessage proxies unhandled messages to the appropriate worker.
// It extracts the fileId from the message (if present) and routes to the corresponding worker.
func (s *Session) handleProxiedMessage(eventType cartaDefinitions.EventType, requestId uint32, bytes []byte) error {
	messageBytes := cartaHelpers.PrepareBinaryMessage(bytes, eventType, requestId)

	// Try to extract fileId from the message
	fileId, hasFileId := cartaHelpers.ExtractFileIdFromBytes(eventType, bytes)

	// Determine which worker to send the message to
	var targetWorker *SessionWorker
	var workerName string

	if hasFileId && s.fileMap != nil {
		// Check if we have a worker for this fileId
		if worker, exists := s.fileMap[fileId]; exists {
			targetWorker = worker
			workerName = fmt.Sprintf("worker:%d", fileId)
		} else {
			// FileId found but no worker mapped, use shared worker
			targetWorker = s.sharedWorker
			workerName = fmt.Sprintf("shared-worker (fileId:%d not mapped)", fileId)
		}
	} else {
		// No fileId in message or fileMap not initialized, use shared worker
		targetWorker = s.sharedWorker
		workerName = "shared-worker"
	}

	log.Printf("Proxying message for event type %v from client to %s", eventType, workerName)

	if targetWorker == nil {
		return fmt.Errorf("no worker available to handle message")
	}

	targetWorker.sendChan <- messageBytes
	return nil
}

func (s *Session) handleStatusMessage(_ cartaDefinitions.EventType, _ uint32, _ []byte) error {
	if s.Info.WorkerId == "" {
		return fmt.Errorf("status request received before worker registration")
	}
	status, err := spawnerHelpers.GetWorkerStatus(s.Info.WorkerId, s.SpawnerAddress)
	if err != nil {
		return fmt.Errorf("error getting worker status: %v", err)
	} else {
		log.Printf("Worker status: Alive: %v, Reachable: %v", status.Alive, status.IsReachable)
	}
	return nil
}
