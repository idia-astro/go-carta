package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/CARTAvis/go-carta/pkg/grpc"
)

var fitsFileRegex = regexp.MustCompile(`(?i)\.(fits?(\.gz)?|fz)$`)
var hdf5FileRegex = regexp.MustCompile(`(?i)\.(hdf5|hd5)$`)
var crtfFileRegex = regexp.MustCompile(`(?i)\.(crtf)$`)
var _xmlFileRegex = regexp.MustCompile(`(?i)\.(xml)$`) //lint:ignore U1000, we will use this later
var ds9RegionFileRegex = regexp.MustCompile(`(?i)\.(reg)$`)

func (s *fileListServer) CheckStatus(_ context.Context, _ *pb.Empty) (*pb.StatusResponse, error) {
	log.Printf("Received CheckStatus message")
	return &pb.StatusResponse{Status: true, StatusMessage: "FileListService is running", InstanceId: s.instanceId.String()}, nil
}

func (s *fileListServer) GetFileList(_ context.Context, req *pb.FileListRequest) (*pb.FileListResponse, error) {
	entries, err := os.ReadDir(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "file list not found")
		} else if os.IsPermission(err) {
			return nil, status.Errorf(codes.PermissionDenied, "permission denied")
		} else {
			return nil, status.Errorf(codes.Internal, "failed to read directory")
		}
	}

	resp := pb.FileListResponse{}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if req.IgnoreHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fileInfo := &pb.BasicFileInfo{
			Name:         entry.Name(),
			Size:         info.Size(),
			IsDirectory:  entry.IsDir(),
			LastModified: info.ModTime().Unix(),
		}

		switch {
		case fitsFileRegex.MatchString(entry.Name()):
			fileInfo.Type = pb.FileType_FITS
		case hdf5FileRegex.MatchString(entry.Name()):
			fileInfo.Type = pb.FileType_HDF5
		case crtfFileRegex.MatchString(entry.Name()):
			fileInfo.Type = pb.FileType_CRTF
		case ds9RegionFileRegex.MatchString(entry.Name()):
			fileInfo.Type = pb.FileType_DS9_REG
		default:
			fileInfo.Type = pb.FileType_UNKNOWN
		}

		resp.Files = append(resp.Files, fileInfo)
	}
	return &resp, nil
}
