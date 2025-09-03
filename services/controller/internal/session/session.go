package session

import (
	"fmt"
	helpers "idia-astro/go-carta/pkg/shared"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

type Session struct {
	Info           spawnerHelpers.WorkerInfo
	SpawnerAddress string
	BaseFolder     string
	WorkerConn     *grpc.ClientConn
	WebSocket      *websocket.Conn
	sendMutex      sync.Mutex
}

var handlerMap = map[cartaDefinitions.EventType]func(*Session, uint32, []byte) error{
	cartaDefinitions.EventType_REGISTER_VIEWER:   (*Session).handleRegisterViewerMessage,
	cartaDefinitions.EventType_FILE_LIST_REQUEST: (*Session).handleFileListRequest,
	cartaDefinitions.EventType_FILE_INFO_REQUEST: (*Session).handleNotImplementedMessage,
	cartaDefinitions.EventType_STOP_FILE_LIST:    (*Session).handleNotImplementedMessage,
	cartaDefinitions.EventType_EMPTY_EVENT:       (*Session).handleStatusMessage,
}

func (s *Session) checkAndParse(msg proto.Message, requestId uint32, rawMsg []byte) error {
	// Register viewer messages are allowed without a worker connection
	if s.WorkerConn == nil {
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

func (s *Session) sendMessage(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}
	s.sendMutex.Lock()
	defer s.sendMutex.Unlock()
	return s.WebSocket.WriteMessage(websocket.BinaryMessage, byteData)
}

func (s *Session) HandleMessage(msg []byte) error {
	// Message prefix is used for determining message type and matching requests to responses
	prefix, err := cartaHelpers.DecodeMessagePrefix(msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message: %v", err)
	}

	handler, ok := handlerMap[prefix.EventType]
	if !ok {
		return fmt.Errorf("unsupported message type: %s (request id: %d)", prefix.EventType, prefix.RequestId)
	} else {
		err = handler(s, prefix.RequestId, msg[8:])
	}

	if err != nil {
		return fmt.Errorf("error handling message: %v", err)
	}
	return nil
}

func (s *Session) HandleDisconnect() {
	if s.Info.WorkerId == "" {
		return
	}
	if s.WorkerConn != nil {
		helpers.CloseOrLog(s.WorkerConn)
	}

	err := spawnerHelpers.RequestWorkerShutdown(s.Info.WorkerId, s.SpawnerAddress)
	if err != nil {
		log.Printf("Error shutting down worker: %v", err)
	}
	log.Printf("Shut down worker with UUID: %s", s.Info.WorkerId)

}
