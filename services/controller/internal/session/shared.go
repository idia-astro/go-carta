package session

import (
	"fmt"
	"log"

	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func (s *Session) handleNotImplementedMessage(requestId uint32, _ []byte) error {
	log.Printf("Not implemented message type for request %v", requestId)
	return nil
}

func (s *Session) handleStatusMessage(_ uint32, _ []byte) error {
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
