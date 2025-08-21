package handlers

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func HandleRegisterViewerMessage(conn *websocket.Conn, workerInfo *spawnerHelpers.WorkerInfo, requestId uint32, msg []byte, spawnerAddress string) error {
	// Cant re-register viewer again
	if workerInfo.WorkerId != "" {
		return fmt.Errorf("viewer already registered")
	}

	// unmarshal the message payload to a RegisterViewerMessage
	var payload cartaDefinitions.RegisterViewer
	err := proto.Unmarshal(msg, &payload)

	if err != nil {
		return err
	}

	info, err := spawnerHelpers.RequestWorkerStartup(spawnerAddress)
	if err != nil {
		return fmt.Errorf("error starting worker: %v", err)
	}
	workerInfo.WorkerId = info.WorkerId
	workerInfo.Address = info.Address
	workerInfo.Port = info.Port

	log.Printf("Worker %s started for session %v and is available at %s:%d", info.WorkerId, payload.SessionId, info.Address, info.Port)

	ackResponse := cartaDefinitions.RegisterViewerAck{
		SessionId:   payload.SessionId,
		Success:     true,
		SessionType: cartaDefinitions.SessionType_NEW,
	}
	byteData, err := cartaHelpers.PrepareMessagePayload(&ackResponse, cartaDefinitions.EventType_REGISTER_VIEWER_ACK, requestId)

	// but this is needed in order to clean up the worker on exit
	if err != nil {
		return err
	}

	err = conn.WriteMessage(websocket.BinaryMessage, byteData)
	if err != nil {
		return err
	}

	return nil
}
