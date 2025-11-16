package session

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	helpers "idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

type Session struct {
	Info            spawnerHelpers.WorkerInfo
	SpawnerAddress  string
	BaseFolder      string
	WebSocket       *websocket.Conn
	WorkerConn      *websocket.Conn
	Context         context.Context
	clientSendMutex sync.Mutex
	workerSendMutex sync.Mutex
}

var handlerMap = map[cartaDefinitions.EventType]func(*Session, cartaDefinitions.EventType, uint32, []byte) error{
	cartaDefinitions.EventType_REGISTER_VIEWER:   (*Session).handleRegisterViewerMessage,
	cartaDefinitions.EventType_FILE_LIST_REQUEST: (*Session).handleProxiedMessage,
	cartaDefinitions.EventType_FILE_INFO_REQUEST: (*Session).handleProxiedMessage,
	cartaDefinitions.EventType_STOP_FILE_LIST:    (*Session).handleProxiedMessage,
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

func (s *Session) proxyMessageToWorker(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}
	s.workerSendMutex.Lock()
	defer s.workerSendMutex.Unlock()
	return s.WorkerConn.WriteMessage(websocket.BinaryMessage, byteData)
}

func (s *Session) sendBinaryPayload(byteData []byte) error {
	s.clientSendMutex.Lock()
	defer s.clientSendMutex.Unlock()
	return s.WebSocket.WriteMessage(websocket.BinaryMessage, byteData)
}

func (s *Session) sendMessage(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}
	return s.sendBinaryPayload(byteData)
}

func (s *Session) workerMessageHandler() {
	for {
		messageType, message, err := s.WorkerConn.ReadMessage()
		if err != nil {
			log.Println("Error reading message from worker:", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			err := s.WorkerConn.WriteMessage(websocket.TextMessage, []byte("PONG"))
			if err != nil {
				log.Printf("Failed to send pong message: %v\n", err)
			}
			continue
		}

		// Ignore all other non-binary messages
		if messageType != websocket.BinaryMessage {
			log.Printf("Ignoring non-binary message: %s\n", message)
			continue
		}

		go func() {
			prefix, err := cartaHelpers.DecodeMessagePrefix(message)
			if err != nil {
				log.Printf("failed to unmarshal message: %v", err)
				return
			}
			if prefix.IcdVersion != cartaHelpers.IcdVersion {
				log.Printf("invalid ICD version: %d", prefix.IcdVersion)
				return
			}

			// TODO: We will often need to adjust responses here
			// Pass the incoming message along to the client
			err = s.sendBinaryPayload(message)
			if err != nil {
				log.Printf("failed to send message to client: %v", err)
			}

		}()
	}
}

func (s *Session) HandleMessage(msg []byte) error {
	// Message prefix is used for determining message type and matching requests to responses
	prefix, err := cartaHelpers.DecodeMessagePrefix(msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message: %v", err)
	}

	handler, ok := handlerMap[prefix.EventType]
	if !ok {
		log.Printf("unsupported message type: %s (request id: %d), proxying to backend", prefix.EventType, prefix.RequestId)
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
