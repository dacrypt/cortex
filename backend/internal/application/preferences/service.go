// Package preferences provides preference learning from user feedback.
// Inspired by MemGPT - learns from user corrections to improve future suggestions.
package preferences

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// ServiceConfig configures the preference learning service.
type ServiceConfig struct {
	MinExamplesForHighConfidence int     // Examples needed to reach high confidence
	HighConfidenceThreshold      float64 // Threshold for auto-applying preferences
	StalePreferenceDays          int     // Days without use before preference is stale
	CleanupLowConfidenceThreshold float64 // Threshold for cleanup
}

// DefaultServiceConfig returns the default configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MinExamplesForHighConfidence: 5,
		HighConfidenceThreshold:      0.8,
		StalePreferenceDays:          90,
		CleanupLowConfidenceThreshold: 0.2,
	}
}

// Service provides preference learning and application.
type Service struct {
	config       ServiceConfig
	feedbackRepo repository.FeedbackRepository
	logger       zerolog.Logger
}

// NewService creates a new preference learning service.
func NewService(
	config ServiceConfig,
	feedbackRepo repository.FeedbackRepository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		config:       config,
		feedbackRepo: feedbackRepo,
		logger:       logger.With().Str("component", "preference-learning").Logger(),
	}
}

// RecordFeedback records user feedback and learns from it.
func (s *Service) RecordFeedback(ctx context.Context, feedback *entity.UserFeedback) error {
	s.logger.Info().
		Str("workspace_id", feedback.WorkspaceID.String()).
		Str("action", string(feedback.ActionType)).
		Msg("Recording user feedback")

	// Save the feedback
	if err := s.feedbackRepo.SaveFeedback(ctx, feedback); err != nil {
		return err
	}

	// Learn from the feedback
	return s.learnFromFeedback(ctx, feedback)
}

// learnFromFeedback extracts patterns from feedback to create/update preferences.
func (s *Service) learnFromFeedback(ctx context.Context, feedback *entity.UserFeedback) error {
	if feedback.Suggestion == nil {
		return nil
	}

	// Determine preference type from suggestion type
	prefType := s.suggestionTypeToPreferenceType(feedback.Suggestion.Type)
	if prefType == "" {
		return nil
	}

	// Build pattern from context
	pattern := s.buildPatternFromContext(feedback.Context)
	if pattern == nil {
		return nil
	}

	// Build behavior based on action type
	behavior := s.buildBehaviorFromFeedback(feedback)
	if behavior == nil {
		return nil
	}

	// Find existing matching preference
	existingPrefs, err := s.feedbackRepo.FindMatchingPreferences(ctx, feedback.WorkspaceID, feedback.Context)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to find matching preferences")
	}

	var matchingPref *entity.LearnedPreference
	for _, pref := range existingPrefs {
		if pref.Type == prefType {
			matchingPref = pref
			break
		}
	}

	if matchingPref != nil {
		// Update existing preference
		if feedback.ActionType == entity.FeedbackActionAccepted {
			matchingPref.Reinforce()
		} else if feedback.ActionType == entity.FeedbackActionRejected {
			matchingPref.Weaken()
		}
		return s.feedbackRepo.UpdatePreference(ctx, matchingPref)
	}

	// Create new preference
	newPref := entity.NewLearnedPreference(
		feedback.WorkspaceID,
		prefType,
		pattern,
		behavior,
	)

	// Set initial confidence based on action
	if feedback.ActionType == entity.FeedbackActionAccepted {
		newPref.Confidence = 0.6 // Start higher for accepted
	} else if feedback.ActionType == entity.FeedbackActionCorrected {
		newPref.Confidence = 0.7 // Even higher for corrections (explicit preference)
	}

	return s.feedbackRepo.SavePreference(ctx, newPref)
}

// ApplyPreferences modifies a suggestion based on learned preferences.
func (s *Service) ApplyPreferences(ctx context.Context, workspaceID entity.WorkspaceID, suggestion *entity.Suggestion, feedbackCtx *entity.FeedbackContext) (*entity.Suggestion, error) {
	// Find matching preferences
	prefs, err := s.feedbackRepo.FindMatchingPreferences(ctx, workspaceID, feedbackCtx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to find preferences")
		return suggestion, nil
	}

	if len(prefs) == 0 {
		return suggestion, nil
	}

	// Apply highest confidence matching preference
	var bestPref *entity.LearnedPreference
	for _, pref := range prefs {
		if bestPref == nil || pref.Confidence > bestPref.Confidence {
			bestPref = pref
		}
	}

	if bestPref == nil || bestPref.Confidence < s.config.HighConfidenceThreshold {
		return suggestion, nil
	}

	// Apply the preference
	modified := s.applyPreferenceBehavior(suggestion, bestPref.Behavior)

	// Update last used
	bestPref.Reinforce()
	s.feedbackRepo.UpdatePreference(ctx, bestPref)

	s.logger.Debug().
		Str("preference_id", bestPref.ID.String()).
		Float64("confidence", bestPref.Confidence).
		Msg("Applied learned preference")

	return modified, nil
}

// GetPreferences returns learned preferences for a workspace.
func (s *Service) GetPreferences(ctx context.Context, workspaceID entity.WorkspaceID, prefType entity.PreferenceType) ([]*entity.LearnedPreference, error) {
	opts := repository.DefaultPreferenceListOptions()

	if prefType != "" {
		return s.feedbackRepo.ListPreferencesByType(ctx, workspaceID, prefType, opts)
	}

	return s.feedbackRepo.ListPreferences(ctx, workspaceID, opts)
}

// GetFeedbackStats returns feedback statistics.
func (s *Service) GetFeedbackStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.FeedbackStats, error) {
	return s.feedbackRepo.GetFeedbackStats(ctx, workspaceID)
}

// ConsolidatePatterns analyzes feedback to identify and strengthen patterns.
func (s *Service) ConsolidatePatterns(ctx context.Context, workspaceID entity.WorkspaceID) error {
	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Msg("Consolidating patterns from feedback")

	// Get recent feedback
	since := time.Now().Add(-30 * 24 * time.Hour) // Last 30 days
	opts := repository.FeedbackListOptions{
		Limit:    500,
		SortBy:   "created_at",
		SortDesc: true,
	}

	feedbacks, err := s.feedbackRepo.ListFeedbackSince(ctx, workspaceID, since, opts)
	if err != nil {
		return err
	}

	// Group feedback by suggestion type and pattern
	patternCounts := make(map[string]int)
	patternActions := make(map[string]map[entity.FeedbackActionType]int)

	for _, fb := range feedbacks {
		if fb.Suggestion == nil || fb.Context == nil {
			continue
		}

		key := s.buildPatternKey(fb.Suggestion.Type, fb.Context)
		patternCounts[key]++

		if patternActions[key] == nil {
			patternActions[key] = make(map[entity.FeedbackActionType]int)
		}
		patternActions[key][fb.ActionType]++
	}

	// Identify strong patterns (consistent user behavior)
	for key, count := range patternCounts {
		if count < s.config.MinExamplesForHighConfidence {
			continue
		}

		actions := patternActions[key]
		total := float64(count)
		acceptRate := float64(actions[entity.FeedbackActionAccepted]) / total

		s.logger.Debug().
			Str("pattern", key).
			Int("count", count).
			Float64("accept_rate", acceptRate).
			Msg("Identified pattern")
	}

	return nil
}

// Cleanup removes stale and low-confidence preferences.
func (s *Service) Cleanup(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	totalDeleted := 0

	// Remove old feedback
	cutoff := time.Now().Add(-180 * 24 * time.Hour) // 6 months
	deleted, err := s.feedbackRepo.DeleteOldFeedback(ctx, workspaceID, cutoff)
	if err != nil {
		return 0, err
	}
	totalDeleted += deleted

	// Remove low confidence preferences
	deleted, err = s.feedbackRepo.DeleteLowConfidencePreferences(ctx, workspaceID, s.config.CleanupLowConfidenceThreshold)
	if err != nil {
		return totalDeleted, err
	}
	totalDeleted += deleted

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Int("deleted", totalDeleted).
		Msg("Cleaned up stale preferences")

	return totalDeleted, nil
}

// Helper methods

func (s *Service) suggestionTypeToPreferenceType(suggType entity.SuggestionType) entity.PreferenceType {
	switch suggType {
	case entity.SuggestionTypeTag:
		return entity.PreferenceTypeTagging
	case entity.SuggestionTypeProject:
		return entity.PreferenceTypeCategorization
	case entity.SuggestionTypeCategory:
		return entity.PreferenceTypeCategorization
	case entity.SuggestionTypeCluster:
		return entity.PreferenceTypeClustering
	case entity.SuggestionTypeTaxonomy:
		return entity.PreferenceTypeOrganization
	default:
		return ""
	}
}

func (s *Service) buildPatternFromContext(ctx *entity.FeedbackContext) *entity.PreferencePattern {
	if ctx == nil {
		return nil
	}

	pattern := &entity.PreferencePattern{}

	// Extract file type pattern
	if ctx.FileType != "" {
		pattern.FileExtensions = []string{ctx.FileType}
	}

	// Extract folder pattern
	if ctx.FolderPath != "" {
		pattern.FolderPatterns = []string{ctx.FolderPath}
	}

	return pattern
}

func (s *Service) buildBehaviorFromFeedback(feedback *entity.UserFeedback) *entity.PreferenceBehavior {
	behavior := &entity.PreferenceBehavior{}

	switch feedback.ActionType {
	case entity.FeedbackActionAccepted:
		// User likes this type of suggestion
		behavior.ConfidenceBoost = 0.1
		if feedback.Suggestion != nil {
			switch feedback.Suggestion.Type {
			case entity.SuggestionTypeTag:
				behavior.PreferredTags = []string{feedback.Suggestion.Value}
			case entity.SuggestionTypeProject:
				behavior.PreferredProjects = []string{feedback.Suggestion.Value}
			}
		}

	case entity.FeedbackActionRejected:
		// User dislikes this type of suggestion
		behavior.ConfidenceBoost = -0.2
		if feedback.Suggestion != nil {
			switch feedback.Suggestion.Type {
			case entity.SuggestionTypeTag:
				behavior.AvoidedTags = []string{feedback.Suggestion.Value}
			case entity.SuggestionTypeProject:
				behavior.AvoidedProjects = []string{feedback.Suggestion.Value}
			}
		}

	case entity.FeedbackActionCorrected:
		// User prefers something else
		if feedback.Correction != nil && feedback.Suggestion != nil {
			switch feedback.Suggestion.Type {
			case entity.SuggestionTypeTag:
				behavior.AvoidedTags = []string{feedback.Suggestion.Value}
				behavior.PreferredTags = []string{feedback.Correction.Value}
			case entity.SuggestionTypeProject:
				behavior.AvoidedProjects = []string{feedback.Suggestion.Value}
				behavior.PreferredProjects = []string{feedback.Correction.Value}
			}
		}
	}

	return behavior
}

func (s *Service) buildPatternKey(suggType entity.SuggestionType, ctx *entity.FeedbackContext) string {
	parts := []string{string(suggType)}

	if ctx != nil {
		if ctx.FileType != "" {
			parts = append(parts, "ext:"+ctx.FileType)
		}
		if ctx.FolderPath != "" {
			parts = append(parts, "folder:"+ctx.FolderPath)
		}
	}

	return strings.Join(parts, "|")
}

func (s *Service) applyPreferenceBehavior(suggestion *entity.Suggestion, behavior *entity.PreferenceBehavior) *entity.Suggestion {
	if behavior == nil {
		return suggestion
	}

	modified := *suggestion // Copy

	// Apply confidence boost
	modified.Confidence += behavior.ConfidenceBoost
	if modified.Confidence > 1.0 {
		modified.Confidence = 1.0
	} else if modified.Confidence < 0 {
		modified.Confidence = 0
	}

	// Check if suggestion should be avoided
	switch suggestion.Type {
	case entity.SuggestionTypeTag:
		for _, avoided := range behavior.AvoidedTags {
			if strings.EqualFold(suggestion.Value, avoided) {
				modified.Confidence = 0 // Effectively reject
			}
		}
	case entity.SuggestionTypeProject:
		for _, avoided := range behavior.AvoidedProjects {
			if strings.EqualFold(suggestion.Value, avoided) {
				modified.Confidence = 0
			}
		}
	}

	return &modified
}
