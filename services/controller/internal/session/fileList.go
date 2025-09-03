package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	cartaProto "idia-astro/go-carta/pkg/grpc"
)

var timeoutDuration = time.Second * 5

func (s *Session) handleFileListRequest(requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileListRequest
	err := s.checkAndParse(&payload, requestId, msg)

	if err != nil {
		return err
	}

	client := cartaProto.NewFileListServiceClient(s.WorkerConn)
	rpcCtx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	path := strings.Replace(payload.Directory, "$BASE", s.BaseFolder, 1)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	rpcResp, err := client.GetFileList(rpcCtx, &cartaProto.FileListRequest{Path: path, IgnoreHidden: true})

	resp := cartaDefinitions.FileListResponse{
		Directory: path,
		Parent:    "",
		Files:     nil,
	}

	if err != nil {
		fmt.Printf("Error getting file list from worker: %v", err)
		resp.Message = "Error getting file list from worker"
		return s.sendMessage(&resp, cartaDefinitions.EventType_FILE_LIST_RESPONSE, requestId)
	}

	for _, file := range rpcResp.Files {
		if file.IsDirectory {
			subDir := cartaDefinitions.DirectoryInfo{
				Name:      file.Name,
				ItemCount: 0,
				Date:      file.LastModified,
			}
			resp.Subdirectories = append(resp.Subdirectories, &subDir)
		} else {
			fileInfo := cartaDefinitions.FileInfo{
				Name:    file.Name,
				Size:    file.Size,
				Date:    file.LastModified,
				HDUList: make([]string, 1),
				// TODO: This is a risk if we ever drift from the carta protobuf definitions. We should consider rather just using the carta-protobuf enum types
				Type: cartaDefinitions.FileType(file.Type),
			}
			resp.Files = append(resp.Files, &fileInfo)
		}
	}
	resp.Success = true

	return s.sendMessage(&resp, cartaDefinitions.EventType_FILE_LIST_RESPONSE, requestId)
}
