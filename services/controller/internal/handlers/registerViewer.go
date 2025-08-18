package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers/types"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func HandleRegisterViewerMessage(msg cartaHelpers.CartaActionMessage, conn *websocket.Conn, spawnerAddress string) (*spawnerHelpers.WorkerInfo, error) {
	payload := cartaHelpers.RegisterViewerMessage{}
	err := json.Unmarshal(msg.Payload, &payload)

	if err != nil {
		return nil, err
	}

	info, err := spawnerHelpers.RequestWorkerStartup(spawnerAddress)
	if err != nil {
		return nil, fmt.Errorf("error starting worker: %v", err)
	}

	log.Printf("Worker %s started for session %v and is available at %s:%d", info.WorkerId, payload.SessionId, info.Address, info.Port)

	responsePayload := cartaHelpers.RegisterViewerAckMessage{
		SessionId:   payload.SessionId,
		Success:     true,
		SessionType: types.SessionTypeNew,
	}

	// TODO: This seems slightly non-idiomatic, because we're returning an error _and_ the info pointer,
	// but this is needed in order to clean up the worker on exit

	// TODO: We could use some generics here
	byteData, err := json.Marshal(responsePayload)
	if err != nil {
		fmt.Println("Error marshaling data:", err)
		return &info, err
	}

	err = conn.WriteJSON(cartaHelpers.CartaResponseMessage{
		ResponseType: types.RegisterViewerAck,
		Payload:      byteData,
	})

	if err != nil {
		return &info, err
	}

	return &info, nil
}
