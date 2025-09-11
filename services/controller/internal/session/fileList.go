package session

import (
	"context"
	"fmt"
	"strings"
	"time"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"idia-astro/go-carta/pkg/cartaDefinitions"
	cartaProto "idia-astro/go-carta/pkg/grpc"
)

var timeoutDuration = time.Second * 5

// isSymlinkToDir reports whether name in baseDir is a symlink pointing to a directory.
func isSymlinkToDir(baseDir, name string) bool {
	full := filepath.Join(baseDir, name)

	fi, err := os.Lstat(full)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false
	}

	target, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false
	}

	tfi, err := os.Stat(target)
	if err != nil {
		return false
	}
	return tfi.IsDir()
}

var multiSlash = regexp.MustCompile(`/+`)

func (s *Session) handleFileListRequest(requestId uint32, msg []byte) error {
	var payload cartaDefinitions.FileListRequest

	log.Printf("Received FileListRequest: %s", string(msg))

	err := s.checkAndParse(&payload, requestId, msg)

	if err != nil {
		log.Printf("Error parsing FileListRequest: %v", err)
		return err
	}

	client := cartaProto.NewFileListServiceClient(s.WorkerConn)
	rpcCtx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	log.Printf("Handling FileListRequest for path: %s", payload.Directory)
	path := strings.Replace(payload.Directory, "$BASE", s.BaseFolder, 1)
	log.Printf("Resolved path: %s", path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	path = strings.ReplaceAll(path, "//", "/")

	log.Printf("Fixed path: %s", path)

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
		log.Printf("RPC file type: %s", file.Type)
		log.Printf("Found file: %s (dir: %v)", file.Name, file.IsDirectory)
		log.Printf("SYMLINK CHECK: is %s a symlink to dir? %v", file.Name, isSymlinkToDir(path, file.Name))
		if file.IsDirectory || isSymlinkToDir(path, file.Name) {
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
