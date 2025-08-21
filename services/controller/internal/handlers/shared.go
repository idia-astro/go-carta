package handlers

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

type Handler func(*websocket.Conn, *spawnerHelpers.WorkerInfo, uint32, []byte, string) error

var HandlerMap = map[cartaDefinitions.EventType]Handler{
	cartaDefinitions.EventType_REGISTER_VIEWER:   HandleRegisterViewerMessage,
	cartaDefinitions.EventType_FILE_LIST_REQUEST: HandleNotImplementedMessage,
	cartaDefinitions.EventType_FILE_INFO_REQUEST: HandleNotImplementedMessage,
	cartaDefinitions.EventType_EMPTY_EVENT:       HandleStatusMessage,
}

func HandleNotImplementedMessage(_ *websocket.Conn, _ *spawnerHelpers.WorkerInfo, requestId uint32, _ []byte, _ string) error {
	log.Printf("Not implemented message type for request %v", requestId)
	return nil
}

func HandleStatusMessage(_ *websocket.Conn, workerInfo *spawnerHelpers.WorkerInfo, _ uint32, _ []byte, spawnerAddress string) error {
	if workerInfo.WorkerId == "" {
		return fmt.Errorf("status request received before worker registration")
	}
	status, err := spawnerHelpers.GetWorkerStatus(workerInfo.WorkerId, spawnerAddress)
	if err != nil {
		return fmt.Errorf("error getting worker status: %v", err)
	} else {
		log.Printf("Worker status: Alive: %v, Reachable: %v", status.Alive, status.IsReachable)
	}
	return nil
}
