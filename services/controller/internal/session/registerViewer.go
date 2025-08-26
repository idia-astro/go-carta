package session

import (
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func (s *Session) handleRegisterViewerMessage(requestId uint32, msg []byte) error {
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
	addr := fmt.Sprintf("%s:%d", info.Address, info.Port)
	workerConn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("could not connect to worker at %s: %w", addr, err)
	}
	s.WorkerConn = workerConn

	ackResponse := cartaDefinitions.RegisterViewerAck{
		SessionId:   payload.SessionId,
		Success:     true,
		SessionType: cartaDefinitions.SessionType_NEW,
	}
	return s.sendMessage(&ackResponse, cartaDefinitions.EventType_REGISTER_VIEWER_ACK, requestId)
}
