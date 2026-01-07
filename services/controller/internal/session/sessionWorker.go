package session

import (
	"fmt"
	"log"

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

	log.Printf("Proxying message for event type %v from session to worker", eventType)
	sw.sendChan <- byteData
	return nil
}

func (sw *SessionWorker) workerMessageHandler() {
	for {
		messageType, message, err := sw.conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message from worker:", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			err := sw.conn.WriteMessage(websocket.TextMessage, []byte("PONG"))
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

			var workerName string
			if sw.fileRequest != nil {
				workerName = fmt.Sprintf("worker:%d", sw.fileRequest.FileId)
			} else {
				workerName = "shared-worker"
			}

			// Special case for register viewer: send the open file payload once the worker is ready

			log.Printf("Received message for event type %v from worker %s fileRequest == %v", prefix.EventType, workerName, sw.fileRequest)

			if sw.fileRequest != nil && prefix.EventType == cartaDefinitions.EventType_REGISTER_VIEWER_ACK {
				log.Printf("Proxying OPEN_FILE message to worker %s after REGISTER_VIEWER_ACK", workerName)
				err = sw.proxyMessageToWorker(sw.fileRequest, cartaDefinitions.EventType_OPEN_FILE, sw.requestId)
				if err != nil {
					log.Printf("Error proxying open file message to worker: %v", err)
				}
			} else {
				// TODO: We will often need to adjust responses here
				// Pass the incoming message along to the client
				log.Printf("Proxying message for event type %v from worker %s to client", prefix.EventType, workerName)
				//	log.Printf("DELAY")
				//	time.Sleep(1000 * time.Millisecond) // slight delay to avoid message clumping

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
