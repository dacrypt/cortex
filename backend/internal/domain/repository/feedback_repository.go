// Package repository defines repository interfaces for domain entities.
package repository

import (
	"context"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// FeedbackRepository defines storage for user feedback and learned preferences.
type FeedbackRepository interface {
	// Feedback CRUD
	SaveFeedback(ctx context.Context, feedback *entity.UserFeedback) error
	GetFeedback(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FeedbackID) (*entity.UserFeedback, error)
	DeleteFeedback(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FeedbackID) error

	// Feedback queries
	ListFeedback(ctx context.Context, workspaceID entity.WorkspaceID, opts FeedbackListOptions) ([]*entity.UserFeedback, error)
	ListFeedbackByType(ctx context.Context, workspaceID entity.WorkspaceID, suggestionType entity.SuggestionType, opts FeedbackListOptions) ([]*entity.UserFeedback, error)
	ListFeedbackByAction(ctx context.Context, workspaceID entity.WorkspaceID, actionType entity.FeedbackActionType, opts FeedbackListOptions) ([]*entity.UserFeedback, error)
	ListFeedbackSince(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, opts FeedbackListOptions) ([]*entity.UserFeedback, error)

	// Feedback statistics
	GetFeedbackStats(ctx context.Context, workspaceID entity.WorkspaceID) (*FeedbackStats, error)
	GetAcceptanceRateBySuggestionType(ctx context.Context, workspaceID entity.WorkspaceID) (map[entity.SuggestionType]float64, error)

	// Learned Preferences CRUD
	SavePreference(ctx context.Context, pref *entity.LearnedPreference) error
	GetPreference(ctx context.Context, workspaceID entity.WorkspaceID, id entity.PreferenceID) (*entity.LearnedPreference, error)
	UpdatePreference(ctx context.Context, pref *entity.LearnedPreference) error
	DeletePreference(ctx context.Context, workspaceID entity.WorkspaceID, id entity.PreferenceID) error

	// Preference queries
	ListPreferences(ctx context.Context, workspaceID entity.WorkspaceID, opts PreferenceListOptions) ([]*entity.LearnedPreference, error)
	ListPreferencesByType(ctx context.Context, workspaceID entity.WorkspaceID, prefType entity.PreferenceType, opts PreferenceListOptions) ([]*entity.LearnedPreference, error)
	FindMatchingPreferences(ctx context.Context, workspaceID entity.WorkspaceID, feedbackCtx *entity.FeedbackContext) ([]*entity.LearnedPreference, error)

	// Preference maintenance
	GetStalePreferences(ctx context.Context, workspaceID entity.WorkspaceID, unusedSince time.Time) ([]*entity.LearnedPreference, error)
	GetLowConfidencePreferences(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.LearnedPreference, error)

	// Bulk operations
	DeleteOldFeedback(ctx context.Context, workspaceID entity.WorkspaceID, before time.Time) (int, error)
	DeleteLowConfidencePreferences(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) (int, error)
}

// FeedbackListOptions contains options for listing feedback.
type FeedbackListOptions struct {
	Offset   int
	Limit    int
	SortBy   string
	SortDesc bool
}

// DefaultFeedbackListOptions returns default list options.
func DefaultFeedbackListOptions() FeedbackListOptions {
	return FeedbackListOptions{
		Offset:   0,
		Limit:    100,
		SortBy:   "created_at",
		SortDesc: true,
	}
}

// PreferenceListOptions contains options for listing preferences.
type PreferenceListOptions struct {
	Offset         int
	Limit          int
	MinConfidence  float64
	SortBy         string
	SortDesc       bool
}

// DefaultPreferenceListOptions returns default list options.
func DefaultPreferenceListOptions() PreferenceListOptions {
	return PreferenceListOptions{
		Offset:        0,
		Limit:         100,
		MinConfidence: 0.0,
		SortBy:        "confidence",
		SortDesc:      true,
	}
}

// FeedbackStats contains feedback statistics.
type FeedbackStats struct {
	TotalFeedback    int
	AcceptedCount    int
	RejectedCount    int
	CorrectedCount   int
	IgnoredCount     int
	AcceptanceRate   float64
	AverageResponseTime int64 // milliseconds

	// Breakdown by suggestion type
	ByType map[entity.SuggestionType]FeedbackTypeStats
}

// FeedbackTypeStats contains stats for a specific suggestion type.
type FeedbackTypeStats struct {
	Total          int
	Accepted       int
	Rejected       int
	Corrected      int
	AcceptanceRate float64
}
