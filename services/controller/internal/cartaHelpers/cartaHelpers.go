package cartaHelpers

import (
	"encoding/json"
	"fmt"

	"idia-astro/go-carta/services/controller/internal/cartaHelpers/types"
)

type CartaActionMessage struct {
	Action  types.CartaMessageType
	Payload json.RawMessage
}

type CartaResponseMessage struct {
	ResponseType types.CartaResponseType
	Payload      json.RawMessage
}

// Some basic message definitions. The action definitions will come from proto files
// For now, this is useful for debugging and testing using JSON payloads

type RegisterViewerMessage struct {
	SessionId          int    `json:"session_id"`
	ApiKey             string `json:"api_key"`
	ClientFeatureFlags int    `json:"client_feature_flags"`
}

type RegisterViewerAckMessage struct {
	Success            bool
	Message            string
	SessionId          int               `json:"session_id"`
	SessionType        types.SessionType `json:"session_type"`
	ServerFeatureFlags int               `json:"server_feature_flags"`
	// TODO: user prefs, layouts, platform strings
}

func GetActionMessage(data []byte) (CartaActionMessage, error) {
	var msg CartaActionMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return msg, err
	}

	// Check that the action is valid
	if !types.IsValidAction(msg.Action) {
		return msg, fmt.Errorf("invalid action: %s", msg.Action)
	}

	return msg, nil
}
