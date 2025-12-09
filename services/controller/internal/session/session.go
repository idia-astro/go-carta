package session

import (
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/idia-astro/go-carta/pkg/cartaDefinitions"
	"github.com/idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"github.com/idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

type Session struct {
	Info           spawnerHelpers.WorkerInfo
	SpawnerAddress string
	BaseFolder     string
	WebSocket      *websocket.Conn
	Context        context.Context
	clientSendChan chan []byte
	// maps incoming file IDs to the internal IDs of the workers
	fileMap      map[int32]*SessionWorker
	sharedWorker *SessionWorker
}

var handlerMap = map[cartaDefinitions.EventType]func(*Session, cartaDefinitions.EventType, uint32, []byte) error{
	cartaDefinitions.EventType_REGISTER_VIEWER: (*Session).handleRegisterViewerMessage,
	cartaDefinitions.EventType_OPEN_FILE:       (*Session).handleOpenFile,
	// TODO: We need to handle CLOSE_FILE separately as well, because it will require shutting down a worker
	cartaDefinitions.EventType_EMPTY_EVENT: (*Session).handleStatusMessage,
}

func (s *Session) checkAndParse(msg proto.Message, requestId uint32, rawMsg []byte) error {
	// Register viewer messages are allowed without a worker connection
	if s.sharedWorker == nil {
		switch msg.(type) {
		case *cartaDefinitions.RegisterViewer:
			break
		default:
			return fmt.Errorf("missing worker connection")
		}
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

func (s *Session) HandleConnection() {
	s.clientSendChan = make(chan []byte, 100)
	go sendHandler(s.clientSendChan, s.WebSocket, "client")

}

func (s *Session) HandleMessage(msg []byte) error {
	// Message prefix is used for determining message type and matching requests to responses
	prefix, err := cartaHelpers.DecodeMessagePrefix(msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message: %v", err)
	}

	handler, ok := handlerMap[prefix.EventType]
	if !ok {
		// Any messages that don't have a specific handler are simply proxied to the worker
		err = s.handleProxiedMessage(prefix.EventType, prefix.RequestId, msg[8:])
	} else {
		err = handler(s, prefix.EventType, prefix.RequestId, msg[8:])
	}

	if err != nil {
		return fmt.Errorf("error handling message: %v", err)
	}
	return nil
}

func (s *Session) HandleDisconnect() {
	// Close the client channel to signal the sender goroutine to stop
	if s.clientSendChan != nil {
		close(s.clientSendChan)
	}

	if s.Info.WorkerId == "" {
		return
	}

	// Close the worker channel to signal the sender goroutine to stop
	if s.sharedWorker != nil {
		s.sharedWorker.disconnect()
	}

	err := spawnerHelpers.RequestWorkerShutdown(s.Info.WorkerId, s.SpawnerAddress)
	if err != nil {
		log.Printf("Error shutting down worker: %v", err)
	}
	log.Printf("Shut down worker with UUID: %s", s.Info.WorkerId)

}
