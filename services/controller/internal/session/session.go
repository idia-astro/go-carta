package session

import (
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	helpers "idia-astro/go-carta/pkg/shared"
	"idia-astro/go-carta/services/controller/internal/cartaHelpers"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

type Session struct {
	Info           spawnerHelpers.WorkerInfo
	SpawnerAddress string
	BaseFolder     string
	WebSocket      *websocket.Conn
	WorkerConn     *websocket.Conn
	Context        context.Context
	clientSendChan chan []byte
	workerSendChan chan []byte
}

var handlerMap = map[cartaDefinitions.EventType]func(*Session, cartaDefinitions.EventType, uint32, []byte) error{
	cartaDefinitions.EventType_REGISTER_VIEWER: (*Session).handleRegisterViewerMessage,
	cartaDefinitions.EventType_EMPTY_EVENT:     (*Session).handleStatusMessage,
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

	s.workerSendChan <- byteData
	return nil
}

func (s *Session) sendBinaryPayload(byteData []byte) {
	s.clientSendChan <- byteData
}

func (s *Session) sendMessage(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}
	s.sendBinaryPayload(byteData)
	return nil
}

func (s *Session) HandleConnection() {
	s.clientSendChan = make(chan []byte, 100)
	go sendHandler(s.clientSendChan, s.WebSocket, "client")

}

func sendHandler(channel chan []byte, conn *websocket.Conn, name string) {
	for byteData := range channel {
		byteLength := len(byteData)
		err := conn.WriteMessage(websocket.BinaryMessage, byteData)
		remaining := len(channel)
		if remaining > 1 {
			log.Printf("Sent message of length %v bytes to %s, %d buffered messages remaining", byteLength, name, remaining)
		}
		if err != nil {
			log.Printf("Error sending message to %s: %v", name, err)
			// Continue processing other messages even if one fails
		}
	}
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
			s.sendBinaryPayload(message)

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
	if s.workerSendChan != nil {
		close(s.workerSendChan)
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
