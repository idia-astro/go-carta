package session

import (
	"idia-astro/go-carta/pkg/cartaDefinitions"
)

func (s *Session) handleFileListRequest(requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileListRequest
	err := s.checkAndParse(&payload, requestId, msg)

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

	return s.sendMessage(&resp, cartaDefinitions.EventType_FILE_LIST_RESPONSE, requestId)
}
