package session

import (
	"fmt"
	"log/slog"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/idia-astro/go-carta/pkg/cartaDefinitions"
	helpers "github.com/idia-astro/go-carta/pkg/shared"
	"github.com/idia-astro/go-carta/services/controller/internal/cartaHelpers"
)

type SessionWorker struct {
	fileRequest    *cartaDefinitions.OpenFile
	requestId      uint32
	conn           *websocket.Conn
	sendChan       chan []byte
	clientSendChan chan []byte
}

func (sw *SessionWorker) proxyMessageToWorker(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) error {
	byteData, err := cartaHelpers.PrepareMessagePayload(msg, eventType, requestId)
	if err != nil {
		return err
	}

	slog.Debug("Proxying message from session to worker", "eventType", eventType)
	sw.sendChan <- byteData
	return nil
}

func (sw *SessionWorker) workerMessageHandler() {
	for {
		messageType, message, err := sw.conn.ReadMessage()
		if err != nil {
			slog.Error("Error reading message from worker", "error", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			slog.Debug("Received PING from worker, sending PONG")
			err := sw.conn.WriteMessage(websocket.TextMessage, []byte("PONG"))
			if err != nil {
				slog.Error("Failed to send pong message", "error", err)
			}
			continue
		}

		// Ignore all other non-binary messages
		if messageType != websocket.BinaryMessage {
			slog.Warn("Ignoring non-binary message", "messageType", messageType, "message", string(message))
			continue
		}

		go func() {
			prefix, err := cartaHelpers.DecodeMessagePrefix(message)
			if err != nil {
				slog.Error("failed to unmarshal message", "error", err)
				return
			}
			if prefix.IcdVersion != cartaHelpers.IcdVersion {
				slog.Error("invalid ICD version", "version", prefix.IcdVersion)
				return
			}
			slog.Debug("Received message from worker", "eventType", prefix.EventType)

			var workerName string
			if sw.fileRequest != nil {
				workerName = fmt.Sprintf("worker:%d", sw.fileRequest.FileId)
			} else {
				workerName = "shared-worker"
			}

			// Special case for register viewer: send the open file payload once the worker is ready

			slog.Debug("Received message from worker", "eventType", prefix.EventType, "workerName", workerName, "hasFileRequest", sw.fileRequest != nil)

			if sw.fileRequest != nil && prefix.EventType == cartaDefinitions.EventType_REGISTER_VIEWER_ACK {
				slog.Debug("Proxying OPEN_FILE message to worker after REGISTER_VIEWER_ACK", "workerName", workerName)
				err = sw.proxyMessageToWorker(sw.fileRequest, cartaDefinitions.EventType_OPEN_FILE, sw.requestId)
				if err != nil {
					slog.Error("Error proxying open file message to worker", "error", err)
				}
			} else {
				// TODO: We will often need to adjust responses here
				// Pass the incoming message along to the client
				sw.clientSendChan <- message
			}
		}()
	}
}

func (sw *SessionWorker) handleInit() {
	sw.sendChan = make(chan []byte, 100)
	// Start up the message sender and proxy handler
	var workerName string
	if sw.fileRequest != nil {
		workerName = fmt.Sprintf("worker:%d", sw.fileRequest.FileId)
	} else {
		workerName = "shared-worker"
	}

	go sendHandler(sw.sendChan, sw.conn, workerName)
	go sw.workerMessageHandler()
}

func (sw *SessionWorker) disconnect() {
	if sw.conn != nil {
		helpers.CloseOrLog(sw.conn)
	}
	if sw.sendChan != nil {
		close(sw.sendChan)
	}
}
