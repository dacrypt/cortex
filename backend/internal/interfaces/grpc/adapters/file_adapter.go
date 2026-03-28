package adapters

import (
	"context"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FileServiceAdapter implements cortexv1.FileServiceServer.
type FileServiceAdapter struct {
	cortexv1.UnimplementedFileServiceServer
	handler *handlers.FileHandler
}

// NewFileServiceAdapter creates a new file service adapter.
func NewFileServiceAdapter(handler *handlers.FileHandler) *FileServiceAdapter {
	return &FileServiceAdapter{handler: handler}
}

func (a *FileServiceAdapter) GetFile(ctx context.Context, req *cortexv1.GetFileRequest) (*cortexv1.FileEntry, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	path := req.GetRelativePath()
	if path == "" {
		return nil, status.Error(codes.InvalidArgument, "relative_path is required")
	}

	entry, err := a.handler.GetFile(ctx, req.WorkspaceId, path)
	if err != nil {
		// Log error but return NotFound instead of propagating internal errors
		// This prevents exposing internal errors and provides consistent API behavior
		return nil, status.Error(codes.NotFound, "file not found")
	}
	if entry == nil {
		return nil, status.Error(codes.NotFound, "file not found")
	}

	return fileEntryToProto(entry), nil
}

func (a *FileServiceAdapter) ProcessFile(ctx context.Context, req *cortexv1.ProcessFileRequest) (*cortexv1.FileEntry, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.RelativePath == "" {
		return nil, status.Error(codes.InvalidArgument, "relative_path is required")
	}

	entry, err := a.handler.ProcessFile(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	return fileEntryToProto(entry), nil
}

func (a *FileServiceAdapter) ListFiles(req *cortexv1.ListFilesRequest, stream cortexv1.FileService_ListFilesServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	offset := 0
	limit := 0
	if req.Pagination != nil {
		offset = int(req.Pagination.Offset)
		limit = int(req.Pagination.Limit)
	}
	if limit <= 0 {
		limit = 1000
	}

	for {
		// Check if stream context is cancelled before processing
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
		}

		entries, _, err := a.handler.ListFiles(stream.Context(), req.WorkspaceId, handlers.ListFilesOptions{
			Offset: offset,
			Limit:  limit,
		})
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			return nil
		}

		for _, entry := range entries {
			// Check context before each send
			select {
			case <-stream.Context().Done():
				return stream.Context().Err()
			default:
			}

			if err := stream.Send(fileEntryToProto(entry)); err != nil {
				return err
			}
		}

		if len(entries) < limit {
			return nil
		}
		offset += limit
	}
}

func (a *FileServiceAdapter) SearchFiles(req *cortexv1.SearchFilesRequest, stream cortexv1.FileService_SearchFilesServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query is required")
	}

	entries, err := a.handler.SearchFiles(stream.Context(), req.WorkspaceId, req.Query)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := stream.Send(fileEntryToProto(entry)); err != nil {
			return err
		}
	}

	return nil
}

func (a *FileServiceAdapter) ListByType(req *cortexv1.ListByTypeRequest, stream cortexv1.FileService_ListByTypeServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Extension == "" {
		return status.Error(codes.InvalidArgument, "extension is required")
	}

	entries, err := a.handler.ListByType(stream.Context(), req.WorkspaceId, req.Extension)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := stream.Send(fileEntryToProto(entry)); err != nil {
			return err
		}
	}
	return nil
}

func (a *FileServiceAdapter) ListByFolder(req *cortexv1.ListByFolderRequest, stream cortexv1.FileService_ListByFolderServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.Folder == "" {
		return status.Error(codes.InvalidArgument, "folder is required")
	}

	entries, err := a.handler.ListByFolder(stream.Context(), req.WorkspaceId, req.Folder)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := stream.Send(fileEntryToProto(entry)); err != nil {
			return err
		}
	}
	return nil
}

func (a *FileServiceAdapter) ListByDate(req *cortexv1.ListByDateRequest, stream cortexv1.FileService_ListByDateServer) error {
	return status.Error(codes.Unimplemented, "ListByDate not implemented")
}

func (a *FileServiceAdapter) ListBySize(req *cortexv1.ListBySizeRequest, stream cortexv1.FileService_ListBySizeServer) error {
	return status.Error(codes.Unimplemented, "ListBySize not implemented")
}

func (a *FileServiceAdapter) ListByContentType(req *cortexv1.ListByContentTypeRequest, stream cortexv1.FileService_ListByContentTypeServer) error {
	return status.Error(codes.Unimplemented, "ListByContentType not implemented")
}

func (a *FileServiceAdapter) GetWorkspaceStats(ctx context.Context, req *cortexv1.GetWorkspaceStatsRequest) (*cortexv1.WorkspaceStats, error) {
	return nil, status.Error(codes.Unimplemented, "GetWorkspaceStats not implemented")
}

func (a *FileServiceAdapter) ScanWorkspace(req *cortexv1.ScanWorkspaceRequest, stream cortexv1.FileService_ScanWorkspaceServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.Path == "" {
		return status.Error(codes.InvalidArgument, "path is required")
	}

	progressCh := make(chan handlers.ScanProgress, 32)
	go func() {
		defer func() {
			// Drain remaining progress messages if channel is closed
			for range progressCh {
			}
		}()
		for progress := range progressCh {
			msg := &cortexv1.ScanProgress{
				FilesScanned: int32(progress.FilesProcessed),
				FilesTotal:   int32(progress.FilesDiscovered),
				CurrentPath:  progress.CurrentFile,
				Phase:        progress.Phase,
			}
			if len(progress.Errors) > 0 {
				msg.Error = &progress.Errors[0]
			}
			// Check if stream context is done before sending
			select {
			case <-stream.Context().Done():
				return
			default:
				if err := stream.Send(msg); err != nil {
					// Stream closed by client - this is normal, don't log as error
					return
				}
			}
		}
	}()

	err := a.handler.ScanWorkspace(stream.Context(), req.WorkspaceId, req.Path, req.ForceFullScan, progressCh)
	close(progressCh)
	if err != nil {
		return err
	}

	return stream.Send(&cortexv1.ScanProgress{
		Phase:      "complete",
		Completed:  true,
		Percentage: 100,
	})
}

func (a *FileServiceAdapter) WatchFiles(req *cortexv1.WatchFilesRequest, stream cortexv1.FileService_WatchFilesServer) error {
	return status.Error(codes.Unimplemented, "WatchFiles not implemented")
}
