package handlers

import (
	"github.com/gorilla/websocket"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	"idia-astro/go-carta/services/controller/internal/spawnerHelpers"
)

func HandleFileListRequest(conn *websocket.Conn, sessionContext *spawnerHelpers.SessionContext, requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileListRequest
	err := CheckAndParse(&payload, sessionContext, requestId, msg)

	if err != nil {
		return err
	}

	subDir := cartaDefinitions.DirectoryInfo{
		Name:      "test",
		ItemCount: 5,
		Date:      0,
	}

	resp := cartaDefinitions.FileListResponse{
		Success:        true,
		Directory:      payload.Directory,
		Parent:         "",
		Files:          nil,
		Subdirectories: []*cartaDefinitions.DirectoryInfo{&subDir},
	}

	return SendMessage(conn, &resp, cartaDefinitions.EventType_FILE_LIST_RESPONSE, requestId)
}
