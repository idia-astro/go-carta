package cartaHelpers

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	"idia-astro/go-carta/pkg/cartaDefinitions"
)

const IcdVersion = 30

type MessagePrefix struct {
	EventType  cartaDefinitions.EventType
	IcdVersion uint16
	RequestId  uint32
}

func DecodeMessagePrefix(data []byte) (prefix MessagePrefix, err error) {
	if len(data) < 8 {
		err = fmt.Errorf("message too short")
		return
	}

	prefix = MessagePrefix{
		EventType:  cartaDefinitions.EventType(binary.LittleEndian.Uint16(data[0:2])),
		IcdVersion: binary.LittleEndian.Uint16(data[2:4]),
		RequestId:  binary.LittleEndian.Uint32(data[4:8]),
	}
	if prefix.IcdVersion != IcdVersion {
		err = fmt.Errorf("invalid ICD version")
		return
	}
	return
}

func PrepareMessagePayload(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) ([]byte, error) {
	byteData, err := proto.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling data:", err)
		return nil, err
	}

	// Prepend 8 bytes: first 2 bytes is event Type, next 2 bytes is ICD version, last 4 bytes is request ID
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header[0:2], uint16(eventType))
	binary.LittleEndian.PutUint16(header[2:4], IcdVersion)
	binary.LittleEndian.PutUint32(header[4:8], requestId)

	// Prepend header to byteData
	byteData = append(header, byteData...)
	return byteData, nil
}
