package adapters

import (
	"context"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetadataServiceAdapter implements cortexv1.MetadataServiceServer.
type MetadataServiceAdapter struct {
	cortexv1.UnimplementedMetadataServiceServer
	handler  *handlers.MetadataHandler
	fileRepo repository.FileRepository
}

// NewMetadataServiceAdapter creates a new metadata service adapter.
func NewMetadataServiceAdapter(handler *handlers.MetadataHandler, fileRepo repository.FileRepository) *MetadataServiceAdapter {
	return &MetadataServiceAdapter{handler: handler, fileRepo: fileRepo}
}

func (a *MetadataServiceAdapter) GetMetadata(ctx context.Context, req *cortexv1.GetMetadataRequest) (*cortexv1.FileMetadata, error) {
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

	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, path)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, status.Error(codes.NotFound, "metadata not found")
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) AddTag(ctx context.Context, req *cortexv1.AddTagRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Tag == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and tag are required")
	}
	if err := a.handler.AddTag(ctx, req.WorkspaceId, req.RelativePath, req.Tag); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) RemoveTag(ctx context.Context, req *cortexv1.RemoveTagRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Tag == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and tag are required")
	}
	if err := a.handler.RemoveTag(ctx, req.WorkspaceId, req.RelativePath, req.Tag); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) ListByTag(req *cortexv1.ListByTagRequest, stream cortexv1.MetadataService_ListByTagServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.Tag == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and tag are required")
	}

	opts := handlers.ListFilesOptions{}
	if req.Pagination != nil {
		opts.Offset = int(req.Pagination.Offset)
		opts.Limit = int(req.Pagination.Limit)
	}
	entries, err := a.handler.ListByTag(stream.Context(), req.WorkspaceId, req.Tag, opts)
	if err != nil {
		return err
	}
	for _, meta := range entries {
		entry := a.fileEntryForMetadata(stream.Context(), req.WorkspaceId, meta)
		if err := stream.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func (a *MetadataServiceAdapter) GetAllTags(ctx context.Context, req *cortexv1.GetAllTagsRequest) (*cortexv1.TagList, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	tags, err := a.handler.GetAllTags(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	return &cortexv1.TagList{Tags: tags}, nil
}

func (a *MetadataServiceAdapter) GetTagCounts(ctx context.Context, req *cortexv1.GetTagCountsRequest) (*cortexv1.TagCountList, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	counts, err := a.handler.GetTagCounts(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int32, len(counts))
	for k, v := range counts {
		result[k] = int32(v)
	}
	return &cortexv1.TagCountList{Counts: result}, nil
}

func (a *MetadataServiceAdapter) AddContext(ctx context.Context, req *cortexv1.AddContextRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Context == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and context are required")
	}
	if err := a.handler.AddContext(ctx, req.WorkspaceId, req.RelativePath, req.Context); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) RemoveContext(ctx context.Context, req *cortexv1.RemoveContextRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Context == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and context are required")
	}
	if err := a.handler.RemoveContext(ctx, req.WorkspaceId, req.RelativePath, req.Context); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) ListByContext(req *cortexv1.ListByContextRequest, stream cortexv1.MetadataService_ListByContextServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.Context == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and context are required")
	}

	opts := handlers.ListFilesOptions{}
	if req.Pagination != nil {
		opts.Offset = int(req.Pagination.Offset)
		opts.Limit = int(req.Pagination.Limit)
	}
	entries, err := a.handler.ListByContext(stream.Context(), req.WorkspaceId, req.Context, opts)
	if err != nil {
		return err
	}
	for _, meta := range entries {
		entry := a.fileEntryForMetadata(stream.Context(), req.WorkspaceId, meta)
		if err := stream.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func (a *MetadataServiceAdapter) GetAllContexts(ctx context.Context, req *cortexv1.GetAllContextsRequest) (*cortexv1.ContextList, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	contexts, err := a.handler.GetAllContexts(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	return &cortexv1.ContextList{Contexts: contexts}, nil
}

func (a *MetadataServiceAdapter) ListProcessingTraces(ctx context.Context, req *cortexv1.ListProcessingTracesRequest) (*cortexv1.ProcessingTraceList, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	traces, err := a.handler.ListProcessingTraces(ctx, req.WorkspaceId, req.GetRelativePath(), int(req.Limit))
	if err != nil {
		return nil, err
	}
	resp := &cortexv1.ProcessingTraceList{}
	for _, trace := range traces {
		resp.Traces = append(resp.Traces, processingTraceToProto(trace))
	}
	return resp, nil
}

func processingTraceToProto(trace entity.ProcessingTrace) *cortexv1.ProcessingTrace {
	msg := &cortexv1.ProcessingTrace{
		FileId:        sanitizeUTF8(trace.FileID.String()),
		RelativePath:  sanitizeUTF8(trace.RelativePath),
		Stage:         sanitizeUTF8(trace.Stage),
		Operation:     sanitizeUTF8(trace.Operation),
		PromptPath:    sanitizeUTF8(trace.PromptPath),
		OutputPath:    sanitizeUTF8(trace.OutputPath),
		PromptPreview: sanitizeUTF8(trace.PromptPreview),
		OutputPreview: sanitizeUTF8(trace.OutputPreview),
		Model:         sanitizeUTF8(trace.Model),
		TokensUsed:    int32(trace.TokensUsed),
		DurationMs:    trace.DurationMs,
		CreatedAt:     trace.CreatedAt.Unix(),
	}
	if trace.Error != nil {
		err := sanitizeUTF8(*trace.Error)
		msg.Error = &err
	}
	return msg
}

func sanitizeUTF8(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	// Replace invalid UTF-8 sequences with replacement character
	return strings.ToValidUTF8(text, "?")
}

func (a *MetadataServiceAdapter) AddSuggestedContext(ctx context.Context, req *cortexv1.AddSuggestedContextRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Context == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and context are required")
	}
	if err := a.handler.AddSuggestedContext(ctx, req.WorkspaceId, req.RelativePath, req.Context); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) AcceptSuggestion(ctx context.Context, req *cortexv1.AcceptSuggestionRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Context == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and context are required")
	}
	if err := a.handler.AcceptSuggestion(ctx, req.WorkspaceId, req.RelativePath, req.Context); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) DismissSuggestion(ctx context.Context, req *cortexv1.DismissSuggestionRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" || req.Context == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, relative_path, and context are required")
	}
	if err := a.handler.DismissSuggestion(ctx, req.WorkspaceId, req.RelativePath, req.Context); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) GetSuggestions(ctx context.Context, req *cortexv1.GetSuggestionsRequest) (*cortexv1.SuggestionList, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	entries, err := a.handler.GetSuggestions(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	suggestions := make([]*cortexv1.Suggestion, 0, len(entries))
	for _, meta := range entries {
		suggestions = append(suggestions, &cortexv1.Suggestion{
			FileId:            meta.FileID.String(),
			RelativePath:      meta.RelativePath,
			SuggestedContexts: meta.SuggestedContexts,
		})
	}
	return &cortexv1.SuggestionList{Suggestions: suggestions}, nil
}

func (a *MetadataServiceAdapter) GetSuggestedMetadata(ctx context.Context, req *cortexv1.GetSuggestedMetadataRequest) (*cortexv1.SuggestedMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and relative_path are required")
	}

	suggested, err := a.handler.GetSuggestedMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	if suggested == nil {
		return nil, status.Error(codes.NotFound, "no suggested metadata found")
	}
	return suggestedMetadataToProto(suggested), nil
}

func (a *MetadataServiceAdapter) UpdateNotes(ctx context.Context, req *cortexv1.UpdateNotesRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and relative_path are required")
	}
	notes := req.Notes
	if err := a.handler.UpdateNotes(ctx, req.WorkspaceId, req.RelativePath, &notes); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) UpdateAISummary(ctx context.Context, req *cortexv1.UpdateAISummaryRequest) (*cortexv1.FileMetadata, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WorkspaceId == "" || req.RelativePath == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and relative_path are required")
	}
	summary := entity.AISummary{
		Summary:     req.Summary,
		ContentHash: req.ContentHash,
		KeyTerms:    req.KeyTerms,
		GeneratedAt: time.Now(),
	}
	if err := a.handler.UpdateAISummary(ctx, req.WorkspaceId, req.RelativePath, summary); err != nil {
		return nil, err
	}
	meta, err := a.handler.GetMetadata(ctx, req.WorkspaceId, req.RelativePath)
	if err != nil {
		return nil, err
	}
	return fileMetadataToProto(meta), nil
}

func (a *MetadataServiceAdapter) fileEntryForMetadata(ctx context.Context, workspaceID string, meta *entity.FileMetadata) *cortexv1.FileEntry {
	if meta == nil {
		return &cortexv1.FileEntry{}
	}
	if a.fileRepo != nil {
		if entry, err := a.fileRepo.GetByID(ctx, entity.WorkspaceID(workspaceID), meta.FileID); err == nil && entry != nil {
			return fileEntryToProto(entry)
		}
	}

	extension := filepath.Ext(meta.RelativePath)
	return &cortexv1.FileEntry{
		FileId:       meta.FileID.String(),
		RelativePath: meta.RelativePath,
		Filename:     filepath.Base(meta.RelativePath),
		Extension:    extension,
	}
}
