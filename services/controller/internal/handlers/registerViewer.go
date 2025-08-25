package handlers

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func HandleRegisterViewerMessage(conn *websocket.Conn, sessionContext *spawnerHelpers.SessionContext, requestId uint32, msg []byte) error {
	var payload cartaDefinitions.RegisterViewer
	err := CheckAndParse(&payload, sessionContext, requestId, msg)
	if err != nil {
		return fmt.Errorf("error parsing message: %v", err)
	}

	info, err := spawnerHelpers.RequestWorkerStartup(sessionContext.SpawnerAddress)
	if err != nil {
		return fmt.Errorf("error starting worker: %v", err)
	}
	sessionContext.Info = info

	log.Printf("Worker %s started for session %v and is available at %s:%d", info.WorkerId, payload.SessionId, info.Address, info.Port)
	addr := fmt.Sprintf("%s:%d", info.Address, info.Port)
	workerConn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return fmt.Errorf("could not connect to worker at %s: %w", addr, err)
	}
	sessionContext.WorkerConn = workerConn

	ackResponse := cartaDefinitions.RegisterViewerAck{
		SessionId:   payload.SessionId,
		Success:     true,
		SessionType: cartaDefinitions.SessionType_NEW,
	}
	return SendMessage(conn, &ackResponse, cartaDefinitions.EventType_REGISTER_VIEWER_ACK, requestId)
}
