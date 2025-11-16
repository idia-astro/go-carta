package session

import (
	"fmt"
	"log"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"

	"github.com/gorilla/websocket"
)

func sendHandler(channel chan []byte, conn *websocket.Conn, name string) {
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

func (s *Session) handleNotImplementedMessage(eventType cartaDefinitions.EventType, requestId uint32, _ []byte) error {
	log.Printf("Not implemented message type %v for request %v", eventType, requestId)
	return nil
}

// TODO: This is currently a very generic function to proxy any unhandled messages to the backend
func (s *Session) handleProxiedMessage(eventType cartaDefinitions.EventType, requestId uint32, bytes []byte) error {
	messageBytes := cartaHelpers.PrepareBinaryMessage(bytes, eventType, requestId)

	var workerName string
	if s.fileMap != nil {
		workerName = fmt.Sprintf("worker:%d", requestId)
	} else {
		workerName = "shared-worker"
	}
	log.Printf("Proxying message for event type %v from client to worker %s", eventType, workerName)
	// Currently we just use the shared worker
	s.sharedWorker.sendChan <- messageBytes
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
