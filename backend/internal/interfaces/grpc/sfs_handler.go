// Package grpc provides gRPC handlers for the Cortex API.
package grpc

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/sfs"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// SFSHandler implements the SemanticFileSystemService gRPC interface.
type SFSHandler struct {
	cortexv1.UnimplementedSemanticFileSystemServiceServer
	sfsService *sfs.Service
	logger     zerolog.Logger
}

// NewSFSHandler creates a new SFS gRPC handler.
func NewSFSHandler(sfsService *sfs.Service, logger zerolog.Logger) *SFSHandler {
	return &SFSHandler{
		sfsService: sfsService,
		logger:     logger.With().Str("handler", "sfs").Logger(),
	}
}

// ExecuteCommand parses and executes a natural language command.
func (h *SFSHandler) ExecuteCommand(ctx context.Context, req *cortexv1.SFSCommandRequest) (*cortexv1.SFSCommandResult, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("command", req.GetCommand()).
		Bool("dry_run", req.GetDryRun()).
		Msg("ExecuteCommand request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Convert context file IDs
	contextFileIDs := make([]entity.FileID, 0, len(req.GetContextFileIds()))
	for _, id := range req.GetContextFileIds() {
		contextFileIDs = append(contextFileIDs, entity.FileID(id))
	}

	// If dry_run, use preview instead
	if req.GetDryRun() {
		preview, err := h.sfsService.PreviewCommand(ctx, workspaceID, req.GetCommand(), contextFileIDs)
		if err != nil {
			return nil, err
		}
		return &cortexv1.SFSCommandResult{
			Success:       true,
			Operation:     operationToProto(preview.Operation),
			Changes:       changesToProto(preview.PlannedChanges),
			Explanation:   preview.Explanation,
			FilesAffected: int32(preview.FilesAffected),
		}, nil
	}

	// Execute command
	result, err := h.sfsService.ExecuteCommand(ctx, workspaceID, req.GetCommand(), contextFileIDs)
	if err != nil {
		return nil, err
	}

	return &cortexv1.SFSCommandResult{
		Success:       result.Success,
		Operation:     operationToProto(result.Operation),
		Changes:       changesToProto(result.Changes),
		Explanation:   result.Explanation,
		ErrorMessage:  result.ErrorMessage,
		FilesAffected: int32(result.FilesAffected),
		UndoCommand:   result.UndoCommand,
	}, nil
}

// PreviewCommand shows what would happen without executing.
func (h *SFSHandler) PreviewCommand(ctx context.Context, req *cortexv1.SFSCommandRequest) (*cortexv1.SFSPreviewResult, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("command", req.GetCommand()).
		Msg("PreviewCommand request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Convert context file IDs
	contextFileIDs := make([]entity.FileID, 0, len(req.GetContextFileIds()))
	for _, id := range req.GetContextFileIds() {
		contextFileIDs = append(contextFileIDs, entity.FileID(id))
	}

	preview, err := h.sfsService.PreviewCommand(ctx, workspaceID, req.GetCommand(), contextFileIDs)
	if err != nil {
		return nil, err
	}

	return &cortexv1.SFSPreviewResult{
		Operation:                  operationToProto(preview.Operation),
		PlannedChanges:             changesToProto(preview.PlannedChanges),
		Explanation:                preview.Explanation,
		FilesAffected:              int32(preview.FilesAffected),
		Confidence:                 preview.Confidence,
		Warnings:                   preview.Warnings,
		AlternativeInterpretations: preview.AlternativeInterpretations,
	}, nil
}

// SuggestCommands suggests possible commands based on context.
func (h *SFSHandler) SuggestCommands(req *cortexv1.SuggestCommandsRequest, stream cortexv1.SemanticFileSystemService_SuggestCommandsServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("partial", req.GetPartialCommand()).
		Msg("SuggestCommands request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Convert context file IDs
	contextFileIDs := make([]entity.FileID, 0, len(req.GetContextFileIds()))
	for _, id := range req.GetContextFileIds() {
		contextFileIDs = append(contextFileIDs, entity.FileID(id))
	}

	suggestions, err := h.sfsService.SuggestCommands(ctx, workspaceID, req.GetPartialCommand(), contextFileIDs, int(req.GetLimit()))
	if err != nil {
		return err
	}

	for _, s := range suggestions {
		if err := stream.Send(&cortexv1.CommandSuggestion{
			Command:     s.Command,
			Description: s.Description,
			Operation:   operationToProto(s.Operation),
			Relevance:   s.Relevance,
			Category:    s.Category,
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetCommandHistory returns recent commands for the workspace.
func (h *SFSHandler) GetCommandHistory(req *cortexv1.GetCommandHistoryRequest, stream cortexv1.SemanticFileSystemService_GetCommandHistoryServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Int32("limit", req.GetLimit()).
		Msg("GetCommandHistory request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Convert timestamp
	var since time.Time
	if req.GetSinceTimestamp() > 0 {
		since = time.UnixMilli(req.GetSinceTimestamp())
	}

	history := h.sfsService.GetCommandHistory(workspaceID, int(req.GetLimit()), since)

	for _, entry := range history {
		if err := stream.Send(&cortexv1.SFSCommandHistoryEntry{
			Id:            entry.ID,
			WorkspaceId:   entry.WorkspaceID.String(),
			Command:       entry.Command,
			Operation:     operationToProto(entry.Operation),
			Success:       entry.Success,
			FilesAffected: int32(entry.FilesAffected),
			ExecutedAt:    entry.ExecutedAt.UnixMilli(),
			ResultSummary: entry.ResultSummary,
		}); err != nil {
			return err
		}
	}

	return nil
}

// operationToProto converts domain operation type to proto enum.
func operationToProto(op sfs.OperationType) cortexv1.SFSOperationType {
	switch op {
	case sfs.OperationGroup:
		return cortexv1.SFSOperationType_SFS_OPERATION_GROUP
	case sfs.OperationFind:
		return cortexv1.SFSOperationType_SFS_OPERATION_FIND
	case sfs.OperationTag:
		return cortexv1.SFSOperationType_SFS_OPERATION_TAG
	case sfs.OperationUntag:
		return cortexv1.SFSOperationType_SFS_OPERATION_UNTAG
	case sfs.OperationAssign:
		return cortexv1.SFSOperationType_SFS_OPERATION_ASSIGN
	case sfs.OperationUnassign:
		return cortexv1.SFSOperationType_SFS_OPERATION_UNASSIGN
	case sfs.OperationCreate:
		return cortexv1.SFSOperationType_SFS_OPERATION_CREATE
	case sfs.OperationMerge:
		return cortexv1.SFSOperationType_SFS_OPERATION_MERGE
	case sfs.OperationRename:
		return cortexv1.SFSOperationType_SFS_OPERATION_RENAME
	case sfs.OperationSummarize:
		return cortexv1.SFSOperationType_SFS_OPERATION_SUMMARIZE
	case sfs.OperationRelate:
		return cortexv1.SFSOperationType_SFS_OPERATION_RELATE
	case sfs.OperationQuery:
		return cortexv1.SFSOperationType_SFS_OPERATION_QUERY
	default:
		return cortexv1.SFSOperationType_SFS_OPERATION_UNKNOWN
	}
}

// changesToProto converts domain file changes to proto messages.
func changesToProto(changes []sfs.FileChange) []*cortexv1.FileChange {
	result := make([]*cortexv1.FileChange, 0, len(changes))
	for _, c := range changes {
		result = append(result, &cortexv1.FileChange{
			FileId:       c.FileID.String(),
			RelativePath: c.RelativePath,
			Operation:    operationToProto(c.Operation),
			BeforeValue:  c.BeforeValue,
			AfterValue:   c.AfterValue,
			Target:       c.Target,
		})
	}
	return result
}
