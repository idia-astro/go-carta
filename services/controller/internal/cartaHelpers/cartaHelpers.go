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

func PrepareBinaryMessage(byteData []byte, eventType cartaDefinitions.EventType, requestId uint32) []byte {
	// Prepend 8 bytes: first 2 bytes is event Type, next 2 bytes is ICD version, last 4 bytes is request ID
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header[0:2], uint16(eventType))
	binary.LittleEndian.PutUint16(header[2:4], IcdVersion)
	binary.LittleEndian.PutUint32(header[4:8], requestId)

	// Prepend header to byteData
	byteData = append(header, byteData...)
	return byteData
}

func PrepareMessagePayload(msg proto.Message, eventType cartaDefinitions.EventType, requestId uint32) ([]byte, error) {
	byteData, err := proto.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling data:", err)
		return nil, err
	}
	return PrepareBinaryMessage(byteData, eventType, requestId), nil
}

// ExtractFileId This function attempts to extract the fileId from a protobuf message by determining whether the generic protobuf message
// has a GetFileId method. There's an inconsistency in terms of how we store file IDs. Sometimes they're uint32, other times int32, this
// function attempts to handle both cases.
func ExtractFileId(msg proto.Message) (int32, bool) {
	type fileIdGetter interface {
		GetFileId() int32
	}
	type fileIdGetterUint interface {
		GetFileId() uint32
	}

	if getter, ok := msg.(fileIdGetter); ok {
		return getter.GetFileId(), true
	}
	if getter, ok := msg.(fileIdGetterUint); ok {
		return int32(getter.GetFileId()), true
	}
	return -1, false
}

// UnmarshalMessage Generic unmarshal function that takes an EventType and raw message bytes and attempts to unmarshal the message into the appropriate protobuf message type
// TODO: I feel like there's a better way to do this using a map, but I can't think of it right now
func UnmarshalMessage(eventType cartaDefinitions.EventType, rawMsg []byte) (proto.Message, error) {
	// Map of EventTypes to their corresponding message constructors
	var msg proto.Message

	switch eventType {
	case cartaDefinitions.EventType_OPEN_FILE:
		msg = &cartaDefinitions.OpenFile{}
	case cartaDefinitions.EventType_CLOSE_FILE:
		msg = &cartaDefinitions.CloseFile{}
	case cartaDefinitions.EventType_SET_IMAGE_CHANNELS:
		msg = &cartaDefinitions.SetImageChannels{}
	case cartaDefinitions.EventType_SET_CURSOR:
		msg = &cartaDefinitions.SetCursor{}
	case cartaDefinitions.EventType_SET_SPATIAL_REQUIREMENTS:
		msg = &cartaDefinitions.SetSpatialRequirements{}
	case cartaDefinitions.EventType_SET_HISTOGRAM_REQUIREMENTS:
		msg = &cartaDefinitions.SetHistogramRequirements{}
	case cartaDefinitions.EventType_SET_STATS_REQUIREMENTS:
		msg = &cartaDefinitions.SetStatsRequirements{}
	case cartaDefinitions.EventType_SET_SPECTRAL_REQUIREMENTS:
		msg = &cartaDefinitions.SetSpectralRequirements{}
	case cartaDefinitions.EventType_SET_REGION:
		msg = &cartaDefinitions.SetRegion{}
	case cartaDefinitions.EventType_REMOVE_REGION:
		msg = &cartaDefinitions.RemoveRegion{}
	case cartaDefinitions.EventType_IMPORT_REGION:
		msg = &cartaDefinitions.ImportRegion{}
	case cartaDefinitions.EventType_EXPORT_REGION:
		msg = &cartaDefinitions.ExportRegion{}
	case cartaDefinitions.EventType_SET_CONTOUR_PARAMETERS:
		msg = &cartaDefinitions.SetContourParameters{}
	case cartaDefinitions.EventType_CONCAT_STOKES_FILES:
		msg = &cartaDefinitions.ConcatStokesFiles{}
	case cartaDefinitions.EventType_MOMENT_REQUEST:
		msg = &cartaDefinitions.MomentRequest{}
	case cartaDefinitions.EventType_STOP_MOMENT_CALC:
		msg = &cartaDefinitions.StopMomentCalc{}
	case cartaDefinitions.EventType_SAVE_FILE:
		msg = &cartaDefinitions.SaveFile{}
	case cartaDefinitions.EventType_PV_REQUEST:
		msg = &cartaDefinitions.PvRequest{}
	case cartaDefinitions.EventType_STOP_PV_CALC:
		msg = &cartaDefinitions.StopPvCalc{}
	case cartaDefinitions.EventType_FITTING_REQUEST:
		msg = &cartaDefinitions.FittingRequest{}
	case cartaDefinitions.EventType_STOP_FITTING:
		msg = &cartaDefinitions.StopFitting{}
	case cartaDefinitions.EventType_REMOTE_FILE_REQUEST:
		msg = &cartaDefinitions.RemoteFileRequest{}
	case cartaDefinitions.EventType_ADD_REQUIRED_TILES:
		msg = &cartaDefinitions.AddRequiredTiles{}
	case cartaDefinitions.EventType_REMOVE_REQUIRED_TILES:
		msg = &cartaDefinitions.RemoveRequiredTiles{}
	case cartaDefinitions.EventType_SET_VECTOR_OVERLAY_PARAMETERS:
		msg = &cartaDefinitions.SetVectorOverlayParameters{}
	case cartaDefinitions.EventType_START_ANIMATION:
		msg = &cartaDefinitions.StartAnimation{}
	case cartaDefinitions.EventType_STOP_ANIMATION:
		msg = &cartaDefinitions.StopAnimation{}
	case cartaDefinitions.EventType_ANIMATION_FLOW_CONTROL:
		msg = &cartaDefinitions.AnimationFlowControl{}
	case cartaDefinitions.EventType_OPEN_CATALOG_FILE:
		msg = &cartaDefinitions.OpenCatalogFile{}
	case cartaDefinitions.EventType_CLOSE_CATALOG_FILE:
		msg = &cartaDefinitions.CloseCatalogFile{}
	case cartaDefinitions.EventType_CATALOG_FILTER_REQUEST:
		msg = &cartaDefinitions.CatalogFilterRequest{}
	default:
		return nil, fmt.Errorf("unknown event type: %v", eventType)
	}

	// Unmarshal the message
	err := proto.Unmarshal(rawMsg, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ExtractFileIdFromBytes attempts to extract the fileId from raw message bytes.
// It unmarshals the message based on the EventType and then extracts the fileId.
// Returns the fileId and true if found, or 0 and false if not found or if the message type doesn't have a fileId.
func ExtractFileIdFromBytes(eventType cartaDefinitions.EventType, rawMsg []byte) (int32, bool) {
	msg, err := UnmarshalMessage(eventType, rawMsg)
	if err != nil {
		return -1, false
	}
	// Extract fileId from the unmarshaled message
	return ExtractFileId(msg)
}
