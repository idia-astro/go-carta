package types

type CartaMessageType string
type CartaResponseType string

// From protobuf definitions

const (
	RegisterViewer  CartaMessageType = "REGISTER_VIEWER"
	FileListRequest CartaMessageType = "FILE_LIST_REQUEST"
	FileInfoRequest CartaMessageType = "FILE_INFO_REQUEST"
	OpenFile        CartaMessageType = "OPEN_FILE"
	StatusRequest   CartaMessageType = "STATUS_REQUEST" // This isn't actually in carta-protobuf, but it's useful here
)

const (
	RegisterViewerAck CartaResponseType = "REGISTER_VIEWER_ACK"
	FileListResponse  CartaResponseType = "FILE_LIST_RESPONSE"
	FileInfoResponse  CartaResponseType = "FILE_INFO_RESPONSE"
	OpenFileAck       CartaResponseType = "OPEN_FILE_ACK"
	StatusResponse    CartaResponseType = "STATUS_RESPONSE"
)

// TODO: I feel like there's a better way to do this
// validActions contains all valid CartaMessageType values for validation
var validActions = map[CartaMessageType]bool{
	RegisterViewer:  true,
	FileListRequest: true,
	FileInfoRequest: true,
	OpenFile:        true,
	StatusRequest:   true,
}

// IsValidAction checks if the given action is a valid CartaMessageType
func IsValidAction(action CartaMessageType) bool {
	return validActions[action]
}
