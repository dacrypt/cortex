// Package grpc provides gRPC handlers for the Cortex API.
package grpc

import (
	"context"

	"github.com/rs/zerolog"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/preferences"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// PreferencesHandler implements the PreferencesService gRPC interface.
type PreferencesHandler struct {
	cortexv1.UnimplementedPreferencesServiceServer
	preferencesService *preferences.Service
	logger             zerolog.Logger
}

// NewPreferencesHandler creates a new Preferences gRPC handler.
func NewPreferencesHandler(preferencesService *preferences.Service, logger zerolog.Logger) *PreferencesHandler {
	return &PreferencesHandler{
		preferencesService: preferencesService,
		logger:             logger.With().Str("handler", "preferences").Logger(),
	}
}

// RecordFeedback records user feedback on a suggestion.
func (h *PreferencesHandler) RecordFeedback(ctx context.Context, req *cortexv1.RecordFeedbackRequest) (*cortexv1.RecordFeedbackResponse, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Int32("action", int32(req.GetAction())).
		Msg("RecordFeedback request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	// Convert proto to entity
	feedback := entity.NewUserFeedback(
		workspaceID,
		feedbackActionFromProto(req.GetAction()),
		suggestionFromProto(req.GetSuggestion()),
	)

	if req.GetCorrection() != nil {
		feedback.WithCorrection(&entity.Correction{
			Value:  req.GetCorrection().GetCorrectedValue(),
			Reason: req.GetCorrection().GetCorrectionReason(),
		})
	}

	if req.GetContext() != nil {
		feedback.WithContext(feedbackContextFromProto(req.GetContext()))
	}

	feedback.WithResponseTime(req.GetResponseTimeMs())

	if err := h.preferencesService.RecordFeedback(ctx, feedback); err != nil {
		return nil, err
	}

	return &cortexv1.RecordFeedbackResponse{
		Success:    true,
		FeedbackId: feedback.ID.String(),
		Message:    "Feedback recorded successfully",
	}, nil
}

// GetPreferences returns learned preferences for a workspace.
func (h *PreferencesHandler) GetPreferences(req *cortexv1.GetPreferencesRequest, stream cortexv1.PreferencesService_GetPreferencesServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetPreferences request")

	ctx := stream.Context()
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())

	prefType := preferenceTypeFromProto(req.GetTypeFilter())
	prefs, err := h.preferencesService.GetPreferences(ctx, workspaceID, prefType)
	if err != nil {
		return err
	}

	for _, p := range prefs {
		if req.GetMinConfidence() > 0 && p.Confidence < req.GetMinConfidence() {
			continue
		}
		if err := stream.Send(preferenceToProto(p)); err != nil {
			return err
		}
	}

	return nil
}

// GetPreference returns a specific preference.
func (h *PreferencesHandler) GetPreference(ctx context.Context, req *cortexv1.GetPreferenceRequest) (*cortexv1.LearnedPreference, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Str("preference_id", req.GetPreferenceId()).
		Msg("GetPreference request")

	// This would need a direct get by ID method in the service
	// For now, we can search through all preferences
	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	prefs, err := h.preferencesService.GetPreferences(ctx, workspaceID, "")
	if err != nil {
		return nil, err
	}

	for _, p := range prefs {
		if p.ID.String() == req.GetPreferenceId() {
			return preferenceToProto(p), nil
		}
	}

	return nil, nil
}

// ApplyPreferences applies learned preferences to modify a suggestion.
func (h *PreferencesHandler) ApplyPreferences(ctx context.Context, req *cortexv1.ApplyPreferencesRequest) (*cortexv1.ApplyPreferencesResponse, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("ApplyPreferences request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	suggestion := suggestionFromProto(req.GetSuggestion())
	feedbackCtx := feedbackContextFromProto(req.GetContext())

	originalConfidence := suggestion.Confidence
	modified, err := h.preferencesService.ApplyPreferences(ctx, workspaceID, suggestion, feedbackCtx)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ApplyPreferencesResponse{
		ModifiedSuggestion: &cortexv1.AISuggestion{
			Type:       suggestionTypeToProto(modified.Type),
			Value:      modified.Value,
			Confidence: modified.Confidence,
			Reasoning:  modified.Reasoning,
			Source:     modified.Source,
		},
		ConfidenceDelta: modified.Confidence - originalConfidence,
		Explanation:     "Preferences applied based on learned patterns",
	}, nil
}

// GetFeedbackStats returns feedback statistics.
func (h *PreferencesHandler) GetFeedbackStats(ctx context.Context, req *cortexv1.GetFeedbackStatsRequest) (*cortexv1.FeedbackStats, error) {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetFeedbackStats request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	stats, err := h.preferencesService.GetFeedbackStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return feedbackStatsToProto(stats), nil
}

// GetFeedbackHistory returns recent feedback.
func (h *PreferencesHandler) GetFeedbackHistory(req *cortexv1.GetFeedbackHistoryRequest, stream cortexv1.PreferencesService_GetFeedbackHistoryServer) error {
	h.logger.Debug().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("GetFeedbackHistory request")

	// This would need a method in the service to list feedback history
	// For now, return empty as the service doesn't expose this directly
	return nil
}

// ConsolidatePatterns triggers pattern analysis from feedback.
func (h *PreferencesHandler) ConsolidatePatterns(ctx context.Context, req *cortexv1.ConsolidatePatternsRequest) (*cortexv1.ConsolidatePatternsResponse, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("ConsolidatePatterns request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	err := h.preferencesService.ConsolidatePatterns(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.ConsolidatePatternsResponse{
		Success: true,
		Message: "Pattern consolidation completed",
	}, nil
}

// Cleanup removes stale preferences.
func (h *PreferencesHandler) Cleanup(ctx context.Context, req *cortexv1.CleanupRequest) (*cortexv1.CleanupResponse, error) {
	h.logger.Info().
		Str("workspace_id", req.GetWorkspaceId()).
		Msg("Cleanup request")

	workspaceID := entity.WorkspaceID(req.GetWorkspaceId())
	deleted, err := h.preferencesService.Cleanup(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return &cortexv1.CleanupResponse{
		Success:            true,
		PreferencesRemoved: int32(deleted),
		Message:            "Cleanup completed successfully",
	}, nil
}

// Conversion helpers

func feedbackActionFromProto(action cortexv1.FeedbackAction) entity.FeedbackActionType {
	switch action {
	case cortexv1.FeedbackAction_FEEDBACK_ACTION_ACCEPTED:
		return entity.FeedbackActionAccepted
	case cortexv1.FeedbackAction_FEEDBACK_ACTION_REJECTED:
		return entity.FeedbackActionRejected
	case cortexv1.FeedbackAction_FEEDBACK_ACTION_CORRECTED:
		return entity.FeedbackActionCorrected
	case cortexv1.FeedbackAction_FEEDBACK_ACTION_IGNORED:
		return entity.FeedbackActionIgnored
	default:
		return ""
	}
}

func feedbackActionToProto(action entity.FeedbackActionType) cortexv1.FeedbackAction {
	switch action {
	case entity.FeedbackActionAccepted:
		return cortexv1.FeedbackAction_FEEDBACK_ACTION_ACCEPTED
	case entity.FeedbackActionRejected:
		return cortexv1.FeedbackAction_FEEDBACK_ACTION_REJECTED
	case entity.FeedbackActionCorrected:
		return cortexv1.FeedbackAction_FEEDBACK_ACTION_CORRECTED
	case entity.FeedbackActionIgnored:
		return cortexv1.FeedbackAction_FEEDBACK_ACTION_IGNORED
	default:
		return cortexv1.FeedbackAction_FEEDBACK_ACTION_UNKNOWN
	}
}

func suggestionTypeFromProto(st cortexv1.SuggestionType) entity.SuggestionType {
	switch st {
	case cortexv1.SuggestionType_SUGGESTION_TYPE_TAG:
		return entity.SuggestionTypeTag
	case cortexv1.SuggestionType_SUGGESTION_TYPE_PROJECT:
		return entity.SuggestionTypeProject
	case cortexv1.SuggestionType_SUGGESTION_TYPE_CATEGORY:
		return entity.SuggestionTypeCategory
	case cortexv1.SuggestionType_SUGGESTION_TYPE_CLUSTER:
		return entity.SuggestionTypeCluster
	default:
		return ""
	}
}

func suggestionTypeToProto(st entity.SuggestionType) cortexv1.SuggestionType {
	switch st {
	case entity.SuggestionTypeTag:
		return cortexv1.SuggestionType_SUGGESTION_TYPE_TAG
	case entity.SuggestionTypeProject:
		return cortexv1.SuggestionType_SUGGESTION_TYPE_PROJECT
	case entity.SuggestionTypeCategory:
		return cortexv1.SuggestionType_SUGGESTION_TYPE_CATEGORY
	case entity.SuggestionTypeCluster:
		return cortexv1.SuggestionType_SUGGESTION_TYPE_CLUSTER
	default:
		return cortexv1.SuggestionType_SUGGESTION_TYPE_UNKNOWN
	}
}

func preferenceTypeFromProto(pt cortexv1.PreferenceType) entity.PreferenceType {
	switch pt {
	case cortexv1.PreferenceType_PREFERENCE_TYPE_TAGGING:
		return entity.PreferenceTypeTagging
	case cortexv1.PreferenceType_PREFERENCE_TYPE_CATEGORIZATION:
		return entity.PreferenceTypeCategorization
	case cortexv1.PreferenceType_PREFERENCE_TYPE_CLUSTERING:
		return entity.PreferenceTypeClustering
	case cortexv1.PreferenceType_PREFERENCE_TYPE_NAMING:
		return entity.PreferenceTypeProjectNaming
	default:
		return ""
	}
}

func preferenceTypeToProto(pt entity.PreferenceType) cortexv1.PreferenceType {
	switch pt {
	case entity.PreferenceTypeTagging:
		return cortexv1.PreferenceType_PREFERENCE_TYPE_TAGGING
	case entity.PreferenceTypeCategorization:
		return cortexv1.PreferenceType_PREFERENCE_TYPE_CATEGORIZATION
	case entity.PreferenceTypeClustering:
		return cortexv1.PreferenceType_PREFERENCE_TYPE_CLUSTERING
	case entity.PreferenceTypeProjectNaming:
		return cortexv1.PreferenceType_PREFERENCE_TYPE_NAMING
	default:
		return cortexv1.PreferenceType_PREFERENCE_TYPE_UNKNOWN
	}
}

func suggestionFromProto(s *cortexv1.AISuggestion) *entity.Suggestion {
	if s == nil {
		return nil
	}
	return &entity.Suggestion{
		Type:       suggestionTypeFromProto(s.GetType()),
		Value:      s.GetValue(),
		Confidence: s.GetConfidence(),
		Reasoning:  s.GetReasoning(),
		Source:     s.GetSource(),
	}
}

func feedbackContextFromProto(c *cortexv1.FeedbackContext) *entity.FeedbackContext {
	if c == nil {
		return nil
	}
	return &entity.FeedbackContext{
		FileID:       c.GetFileId(),
		FileType:     c.GetFileType(),
		FolderPath:   c.GetFolderPath(),
		FileTags:     c.GetExistingTags(),
		FileProjects: c.GetExistingProjects(),
	}
}

func preferenceToProto(p *entity.LearnedPreference) *cortexv1.LearnedPreference {
	if p == nil {
		return nil
	}

	var lastUsedAt int64
	if p.LastUsed != nil {
		lastUsedAt = p.LastUsed.UnixMilli()
	}

	return &cortexv1.LearnedPreference{
		Id:           p.ID.String(),
		WorkspaceId:  p.WorkspaceID.String(),
		Type:         preferenceTypeToProto(p.Type),
		Pattern:      patternToProto(p.Pattern),
		Behavior:     behaviorToProto(p.Behavior),
		Confidence:   p.Confidence,
		ExampleCount: int32(p.Examples),
		CreatedAt:    p.CreatedAt.UnixMilli(),
		UpdatedAt:    p.UpdatedAt.UnixMilli(),
		LastUsedAt:   lastUsedAt,
	}
}

func patternToProto(p *entity.PreferencePattern) *cortexv1.PreferencePattern {
	if p == nil {
		return nil
	}
	return &cortexv1.PreferencePattern{
		FileExtensions: p.FileExtensions,
		FolderPatterns: p.FolderPatterns,
		Keywords:       p.Keywords,
	}
}

func behaviorToProto(b *entity.PreferenceBehavior) *cortexv1.PreferenceBehavior {
	if b == nil {
		return nil
	}
	return &cortexv1.PreferenceBehavior{
		ConfidenceBoost:   b.ConfidenceBoost,
		PreferredTags:     b.PreferredTags,
		AvoidedTags:       b.AvoidedTags,
		PreferredProjects: b.PreferredProjects,
		AvoidedProjects:   b.AvoidedProjects,
	}
}

func feedbackStatsToProto(s *repository.FeedbackStats) *cortexv1.FeedbackStats {
	if s == nil {
		return nil
	}
	return &cortexv1.FeedbackStats{
		TotalFeedback:      int32(s.TotalFeedback),
		Accepted:           int32(s.AcceptedCount),
		Rejected:           int32(s.RejectedCount),
		Corrected:          int32(s.CorrectedCount),
		Ignored:            int32(s.IgnoredCount),
		AcceptanceRate:     s.AcceptanceRate,
		AvgResponseTimeMs:  float64(s.AverageResponseTime),
	}
}
