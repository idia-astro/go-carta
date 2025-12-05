package session

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	"github.com/idia-astro/go-carta/pkg/cartaDefinitions"
	"github.com/idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

// OpenFile needs to spin up a new worker and proxy the message to it
func (s *Session) handleOpenFile(_ cartaDefinitions.EventType, requestId uint32, msg []byte) error {
	var payload cartaDefinitions.OpenFile
	err := s.checkAndParse(&payload, requestId, msg)
	if err != nil {
		return fmt.Errorf("error parsing message: %v", err)
	}

	info, err := spawnerHelpers.RequestWorkerStartup(s.SpawnerAddress, s.BaseFolder)
	if err != nil {
		return fmt.Errorf("error starting worker: %v", err)
	}

	log.Printf("Worker %s started for fileId %v and is available at %s:%d", info.WorkerId, payload.FileId, info.Address, info.Port)
	addr := fmt.Sprintf("ws://%s:%d", info.Address, info.Port)
	workerConn, _, err := websocket.DefaultDialer.DialContext(s.Context, addr, nil)
	if err != nil {
		return fmt.Errorf("could not connect to worker at %s: %w", addr, err)
	}

	fileWorker := &SessionWorker{
		requestId:      requestId,
		fileRequest:    &payload,
		conn:           workerConn,
		clientSendChan: s.clientSendChan,
	}
	fileWorker.handleInit()

	if s.fileMap == nil {
		s.fileMap = make(map[int32]*SessionWorker)
	}

	s.fileMap[payload.FileId] = fileWorker

	// We  need to first pass through a register viewer message, and then wait for the ack before sending through the open file message
	// File opening is handled by workerMessageHandler
	return fileWorker.proxyMessageToWorker(&payload, cartaDefinitions.EventType_REGISTER_VIEWER, requestId)
}
