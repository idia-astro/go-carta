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

func HandleRegisterViewerMessage(msg []byte, requestId uint32, conn *websocket.Conn, spawnerAddress string) (*spawnerHelpers.WorkerInfo, error) {
	// unmarshal the message payload to a RegisterViewerMessage
	var payload cartaDefinitions.RegisterViewer
	err := proto.Unmarshal(msg, &payload)

	if err != nil {
		return nil, err
	}

	info, err := spawnerHelpers.RequestWorkerStartup(spawnerAddress)
	if err != nil {
		return nil, fmt.Errorf("error starting worker: %v", err)
	}

	log.Printf("Worker %s started for session %v and is available at %s:%d", info.WorkerId, payload.SessionId, info.Address, info.Port)

	ackResponse := cartaDefinitions.RegisterViewerAck{
		SessionId:   payload.SessionId,
		Success:     true,
		SessionType: cartaDefinitions.SessionType_NEW,
	}
	byteData, err := cartaHelpers.PrepareMessagePayload(&ackResponse, cartaDefinitions.EventType_REGISTER_VIEWER_ACK, requestId)

	// TODO: This seems slightly non-idiomatic, because we're returning an error _and_ the info pointer,
	// but this is needed in order to clean up the worker on exit
	if err != nil {
		return &info, err
	}

	err = conn.WriteMessage(websocket.BinaryMessage, byteData)
	if err != nil {
		return &info, err
	}

	return &info, nil
}
