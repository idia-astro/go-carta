package session

import (


//	"strings"
//	"time"
	"log"

	"idia-astro/go-carta/pkg/cartaDefinitions"
//	cartaProto "idia-astro/go-carta/pkg/grpc"
)


func (s *Session) handleFileInfoRequest(requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileInfoRequest

	log.Printf("Received FileInfoRequest: %s", string(msg))

	err := s.checkAndParse(&payload, requestId, msg)

	if err != nil {
		log.Printf("Error parsing FileInfoRequest: %v", err)
		return err
	}
	return err
}