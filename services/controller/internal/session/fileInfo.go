package session

import (
	"context"
	"log"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	
	cartaProto "idia-astro/go-carta/pkg/grpc"
)


func (s *Session) handleFileInfoRequest(requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileInfoRequest

	log.Printf("Received FileInfoRequest: %s", string(msg))

	err := s.checkAndParse(&payload, requestId, msg)
	if err != nil {
		log.Printf("Error parsing FileInfoRequest: %v", err)
		return err
	}
	client := cartaProto.NewFileInfoServiceClient(s.WorkerConn)
	rpcCtx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	response, err := client.GetFileInfo(rpcCtx, &cartaProto.FileInfoRequest{Path: payload.File})
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		return err
	}

	rpcCtx = rpcCtx
	response = response

	return err
}