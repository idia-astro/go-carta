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

type Handler func(*websocket.Conn, *spawnerHelpers.SessionContext, uint32, []byte) error

func CheckAndParse[T proto.Message](msg T, ctx *spawnerHelpers.SessionContext, requestId uint32, rawMsg []byte) error {
	if ctx == nil || ctx.WorkerConn == nil {
		return fmt.Errorf("missing context")
	}

	if requestId == 0 {
		return fmt.Errorf("invalid or missing request id")
	}

	err := proto.Unmarshal(rawMsg, msg)

	if err != nil {
		return err
	}

	return nil
}

func SendMessage[T proto.Message](conn *websocket.Conn, msg T, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, byteData)
}

var HandlerMap = map[cartaDefinitions.EventType]Handler{
	cartaDefinitions.EventType_REGISTER_VIEWER:   HandleRegisterViewerMessage,
	cartaDefinitions.EventType_FILE_LIST_REQUEST: HandleFileListRequest,
	cartaDefinitions.EventType_FILE_INFO_REQUEST: HandleNotImplementedMessage,
	cartaDefinitions.EventType_STOP_FILE_LIST:    HandleNotImplementedMessage,
	cartaDefinitions.EventType_EMPTY_EVENT:       HandleStatusMessage,
}

func HandleNotImplementedMessage(_ *websocket.Conn, _ *spawnerHelpers.SessionContext, requestId uint32, _ []byte) error {
	log.Printf("Not implemented message type for request %v", requestId)
	return nil
}

func HandleStatusMessage(_ *websocket.Conn, sessionContext *spawnerHelpers.SessionContext, _ uint32, _ []byte) error {
	if sessionContext == nil || sessionContext.Info.WorkerId == "" {
		return fmt.Errorf("status request received before worker registration")
	}
	status, err := spawnerHelpers.GetWorkerStatus(sessionContext.Info.WorkerId, sessionContext.SpawnerAddress)
	if err != nil {
		return fmt.Errorf("error getting worker status: %v", err)
	} else {
		log.Printf("Worker status: Alive: %v, Reachable: %v", status.Alive, status.IsReachable)
	}
	return nil
}
