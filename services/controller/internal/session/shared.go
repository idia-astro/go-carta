package session

import (
	"fmt"
	"log"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func (s *Session) handleNotImplementedMessage(eventType cartaDefinitions.EventType, requestId uint32, _ []byte) error {
	log.Printf("Not implemented message type %v for request %v", eventType, requestId)
	return nil
}

// TODO: This is currently a very generic function to proxy any unhandled messages to the backend
func (s *Session) handleProxiedMessage(eventType cartaDefinitions.EventType, requestId uint32, bytes []byte) error {
	messageBytes := cartaHelpers.PrepareBinaryMessage(bytes, eventType, requestId)
	s.workerSendChan <- messageBytes
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
