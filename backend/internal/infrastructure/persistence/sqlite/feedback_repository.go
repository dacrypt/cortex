// Package sqlite provides SQLite implementations of repository interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// FeedbackRepository implements FeedbackRepository using SQLite.
type FeedbackRepository struct {
	db *sql.DB
}

// NewFeedbackRepository creates a new SQLite feedback repository.
func NewFeedbackRepository(db *sql.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

// SaveFeedback stores a feedback entry.
func (r *FeedbackRepository) SaveFeedback(ctx context.Context, feedback *entity.UserFeedback) error {
	var suggestionType, suggestionValue, suggestionReasoning, suggestionSource string
	var suggestionConfidence float64
	var suggestionMetadata, correctionValue, correctionReason, correctionMetadata string
	var contextFileID, contextFilePath, contextFileType, contextFolderPath, contextSessionID, contextFull string

	if feedback.Suggestion != nil {
		suggestionType = string(feedback.Suggestion.Type)
		suggestionValue = feedback.Suggestion.Value
		suggestionConfidence = feedback.Suggestion.Confidence
		suggestionReasoning = feedback.Suggestion.Reasoning
		suggestionSource = feedback.Suggestion.Source
		if feedback.Suggestion.Metadata != nil {
			data, _ := json.Marshal(feedback.Suggestion.Metadata)
			suggestionMetadata = string(data)
		}
	}

	if feedback.Correction != nil {
		correctionValue = feedback.Correction.Value
		correctionReason = feedback.Correction.Reason
		if feedback.Correction.Metadata != nil {
			data, _ := json.Marshal(feedback.Correction.Metadata)
			correctionMetadata = string(data)
		}
	}

	if feedback.Context != nil {
		contextFileID = feedback.Context.FileID
		contextFilePath = feedback.Context.FilePath
		contextFileType = feedback.Context.FileType
		contextFolderPath = feedback.Context.FolderPath
		contextSessionID = feedback.Context.SessionID
		fullCtx, _ := feedback.Context.ToJSON()
		contextFull = fullCtx
	}

	query := `
		INSERT INTO user_feedback (
			id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		feedback.ID.String(), feedback.WorkspaceID.String(), string(feedback.ActionType),
		suggestionType, suggestionValue, suggestionConfidence, suggestionReasoning,
		suggestionSource, suggestionMetadata, correctionValue, correctionReason,
		correctionMetadata, contextFileID, contextFilePath, contextFileType,
		contextFolderPath, contextSessionID, contextFull, feedback.ResponseTime,
		feedback.CreatedAt.UnixMilli(),
	)

	return err
}

// GetFeedback retrieves a feedback entry by ID.
func (r *FeedbackRepository) GetFeedback(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FeedbackID) (*entity.UserFeedback, error) {
	query := `
		SELECT id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		FROM user_feedback
		WHERE workspace_id = ? AND id = ?
	`

	row := r.db.QueryRowContext(ctx, query, workspaceID.String(), id.String())
	return r.scanFeedback(row)
}

// DeleteFeedback removes a feedback entry.
func (r *FeedbackRepository) DeleteFeedback(ctx context.Context, workspaceID entity.WorkspaceID, id entity.FeedbackID) error {
	query := `DELETE FROM user_feedback WHERE workspace_id = ? AND id = ?`
	_, err := r.db.ExecContext(ctx, query, workspaceID.String(), id.String())
	return err
}

// ListFeedback returns feedback entries.
func (r *FeedbackRepository) ListFeedback(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.FeedbackListOptions) ([]*entity.UserFeedback, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		FROM user_feedback
		WHERE workspace_id = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizeSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFeedbackRows(rows)
}

// ListFeedbackByType returns feedback for a specific suggestion type.
func (r *FeedbackRepository) ListFeedbackByType(ctx context.Context, workspaceID entity.WorkspaceID, suggestionType entity.SuggestionType, opts repository.FeedbackListOptions) ([]*entity.UserFeedback, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		FROM user_feedback
		WHERE workspace_id = ? AND suggestion_type = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizeSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), string(suggestionType), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFeedbackRows(rows)
}

// ListFeedbackByAction returns feedback for a specific action type.
func (r *FeedbackRepository) ListFeedbackByAction(ctx context.Context, workspaceID entity.WorkspaceID, actionType entity.FeedbackActionType, opts repository.FeedbackListOptions) ([]*entity.UserFeedback, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		FROM user_feedback
		WHERE workspace_id = ? AND action_type = ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizeSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), string(actionType), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFeedbackRows(rows)
}

// ListFeedbackSince returns feedback since a specific time.
func (r *FeedbackRepository) ListFeedbackSince(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, opts repository.FeedbackListOptions) ([]*entity.UserFeedback, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, action_type, suggestion_type, suggestion_value,
			suggestion_confidence, suggestion_reasoning, suggestion_source, suggestion_metadata,
			correction_value, correction_reason, correction_metadata,
			context_file_id, context_file_path, context_file_type, context_folder_path,
			context_session_id, context_full, response_time, created_at
		FROM user_feedback
		WHERE workspace_id = ? AND created_at >= ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizeSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), since.UnixMilli(), opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFeedbackRows(rows)
}

// GetFeedbackStats returns feedback statistics.
func (r *FeedbackRepository) GetFeedbackStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.FeedbackStats, error) {
	stats := &repository.FeedbackStats{
		ByType: make(map[entity.SuggestionType]repository.FeedbackTypeStats),
	}

	// Get overall counts
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN action_type = 'accepted' THEN 1 ELSE 0 END) as accepted,
			SUM(CASE WHEN action_type = 'rejected' THEN 1 ELSE 0 END) as rejected,
			SUM(CASE WHEN action_type = 'corrected' THEN 1 ELSE 0 END) as corrected,
			SUM(CASE WHEN action_type = 'ignored' THEN 1 ELSE 0 END) as ignored,
			AVG(response_time) as avg_response_time
		FROM user_feedback
		WHERE workspace_id = ?
	`

	var avgResponseTime sql.NullFloat64
	err := r.db.QueryRowContext(ctx, query, workspaceID.String()).Scan(
		&stats.TotalFeedback, &stats.AcceptedCount, &stats.RejectedCount,
		&stats.CorrectedCount, &stats.IgnoredCount, &avgResponseTime,
	)
	if err != nil {
		return nil, err
	}

	if avgResponseTime.Valid {
		stats.AverageResponseTime = int64(avgResponseTime.Float64)
	}

	if stats.TotalFeedback > 0 {
		stats.AcceptanceRate = float64(stats.AcceptedCount) / float64(stats.TotalFeedback)
	}

	// Get per-type stats
	typeQuery := `
		SELECT
			suggestion_type,
			COUNT(*) as total,
			SUM(CASE WHEN action_type = 'accepted' THEN 1 ELSE 0 END) as accepted,
			SUM(CASE WHEN action_type = 'rejected' THEN 1 ELSE 0 END) as rejected,
			SUM(CASE WHEN action_type = 'corrected' THEN 1 ELSE 0 END) as corrected
		FROM user_feedback
		WHERE workspace_id = ?
		GROUP BY suggestion_type
	`

	rows, err := r.db.QueryContext(ctx, typeQuery, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var suggType string
		var typeStats repository.FeedbackTypeStats

		err := rows.Scan(&suggType, &typeStats.Total, &typeStats.Accepted, &typeStats.Rejected, &typeStats.Corrected)
		if err != nil {
			return nil, err
		}

		if typeStats.Total > 0 {
			typeStats.AcceptanceRate = float64(typeStats.Accepted) / float64(typeStats.Total)
		}

		stats.ByType[entity.SuggestionType(suggType)] = typeStats
	}

	return stats, rows.Err()
}

// GetAcceptanceRateBySuggestionType returns acceptance rates by suggestion type.
func (r *FeedbackRepository) GetAcceptanceRateBySuggestionType(ctx context.Context, workspaceID entity.WorkspaceID) (map[entity.SuggestionType]float64, error) {
	query := `
		SELECT
			suggestion_type,
			CAST(SUM(CASE WHEN action_type = 'accepted' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as rate
		FROM user_feedback
		WHERE workspace_id = ?
		GROUP BY suggestion_type
	`

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make(map[entity.SuggestionType]float64)
	for rows.Next() {
		var suggType string
		var rate float64
		if err := rows.Scan(&suggType, &rate); err != nil {
			return nil, err
		}
		rates[entity.SuggestionType(suggType)] = rate
	}

	return rates, rows.Err()
}

// SavePreference stores a learned preference.
func (r *FeedbackRepository) SavePreference(ctx context.Context, pref *entity.LearnedPreference) error {
	patternJSON, err := pref.Pattern.ToJSON()
	if err != nil {
		return err
	}

	behaviorJSON, err := pref.Behavior.ToJSON()
	if err != nil {
		return err
	}

	var lastUsed *int64
	if pref.LastUsed != nil {
		ts := pref.LastUsed.UnixMilli()
		lastUsed = &ts
	}

	query := `
		INSERT INTO learned_preferences (
			id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		pref.ID.String(), pref.WorkspaceID.String(), string(pref.Type),
		patternJSON, behaviorJSON, pref.Confidence, pref.Examples,
		lastUsed, pref.CreatedAt.UnixMilli(), pref.UpdatedAt.UnixMilli(),
	)

	return err
}

// GetPreference retrieves a learned preference by ID.
func (r *FeedbackRepository) GetPreference(ctx context.Context, workspaceID entity.WorkspaceID, id entity.PreferenceID) (*entity.LearnedPreference, error) {
	query := `
		SELECT id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		FROM learned_preferences
		WHERE workspace_id = ? AND id = ?
	`

	row := r.db.QueryRowContext(ctx, query, workspaceID.String(), id.String())
	return r.scanPreference(row)
}

// UpdatePreference updates a learned preference.
func (r *FeedbackRepository) UpdatePreference(ctx context.Context, pref *entity.LearnedPreference) error {
	patternJSON, err := pref.Pattern.ToJSON()
	if err != nil {
		return err
	}

	behaviorJSON, err := pref.Behavior.ToJSON()
	if err != nil {
		return err
	}

	var lastUsed *int64
	if pref.LastUsed != nil {
		ts := pref.LastUsed.UnixMilli()
		lastUsed = &ts
	}

	query := `
		UPDATE learned_preferences
		SET pattern = ?, behavior = ?, confidence = ?, examples = ?,
			last_used = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`

	_, err = r.db.ExecContext(ctx, query,
		patternJSON, behaviorJSON, pref.Confidence, pref.Examples,
		lastUsed, pref.UpdatedAt.UnixMilli(),
		pref.WorkspaceID.String(), pref.ID.String(),
	)

	return err
}

// DeletePreference removes a learned preference.
func (r *FeedbackRepository) DeletePreference(ctx context.Context, workspaceID entity.WorkspaceID, id entity.PreferenceID) error {
	query := `DELETE FROM learned_preferences WHERE workspace_id = ? AND id = ?`
	_, err := r.db.ExecContext(ctx, query, workspaceID.String(), id.String())
	return err
}

// ListPreferences returns learned preferences.
func (r *FeedbackRepository) ListPreferences(ctx context.Context, workspaceID entity.WorkspaceID, opts repository.PreferenceListOptions) ([]*entity.LearnedPreference, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		FROM learned_preferences
		WHERE workspace_id = ? AND confidence >= ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizePrefSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), opts.MinConfidence, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPreferenceRows(rows)
}

// ListPreferencesByType returns preferences of a specific type.
func (r *FeedbackRepository) ListPreferencesByType(ctx context.Context, workspaceID entity.WorkspaceID, prefType entity.PreferenceType, opts repository.PreferenceListOptions) ([]*entity.LearnedPreference, error) {
	query := fmt.Sprintf(`
		SELECT id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		FROM learned_preferences
		WHERE workspace_id = ? AND preference_type = ? AND confidence >= ?
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, r.sanitizePrefSortField(opts.SortBy), r.sortDirection(opts.SortDesc))

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), string(prefType), opts.MinConfidence, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPreferenceRows(rows)
}

// FindMatchingPreferences finds preferences that match the given context.
func (r *FeedbackRepository) FindMatchingPreferences(ctx context.Context, workspaceID entity.WorkspaceID, feedbackCtx *entity.FeedbackContext) ([]*entity.LearnedPreference, error) {
	// Get all preferences and filter in memory (pattern matching is complex)
	allPrefs, err := r.ListPreferences(ctx, workspaceID, repository.PreferenceListOptions{
		Limit:         1000,
		MinConfidence: 0.3,
		SortBy:        "confidence",
		SortDesc:      true,
	})
	if err != nil {
		return nil, err
	}

	matching := make([]*entity.LearnedPreference, 0)
	for _, pref := range allPrefs {
		if pref.MatchesContext(feedbackCtx) {
			matching = append(matching, pref)
		}
	}

	return matching, nil
}

// GetStalePreferences returns preferences not used since the given time.
func (r *FeedbackRepository) GetStalePreferences(ctx context.Context, workspaceID entity.WorkspaceID, unusedSince time.Time) ([]*entity.LearnedPreference, error) {
	query := `
		SELECT id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		FROM learned_preferences
		WHERE workspace_id = ? AND (last_used IS NULL OR last_used < ?)
		ORDER BY last_used ASC
	`

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), unusedSince.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPreferenceRows(rows)
}

// GetLowConfidencePreferences returns preferences below the threshold.
func (r *FeedbackRepository) GetLowConfidencePreferences(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.LearnedPreference, error) {
	query := `
		SELECT id, workspace_id, preference_type, pattern, behavior,
			confidence, examples, last_used, created_at, updated_at
		FROM learned_preferences
		WHERE workspace_id = ? AND confidence < ?
		ORDER BY confidence ASC
	`

	rows, err := r.db.QueryContext(ctx, query, workspaceID.String(), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPreferenceRows(rows)
}

// DeleteOldFeedback removes feedback older than the specified time.
func (r *FeedbackRepository) DeleteOldFeedback(ctx context.Context, workspaceID entity.WorkspaceID, before time.Time) (int, error) {
	query := `DELETE FROM user_feedback WHERE workspace_id = ? AND created_at < ?`
	result, err := r.db.ExecContext(ctx, query, workspaceID.String(), before.UnixMilli())
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// DeleteLowConfidencePreferences removes preferences below the threshold.
func (r *FeedbackRepository) DeleteLowConfidencePreferences(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) (int, error) {
	query := `DELETE FROM learned_preferences WHERE workspace_id = ? AND confidence < ?`
	result, err := r.db.ExecContext(ctx, query, workspaceID.String(), threshold)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// Helper methods

func (r *FeedbackRepository) scanFeedback(row *sql.Row) (*entity.UserFeedback, error) {
	var id, workspaceID, actionType string
	var suggType, suggValue, suggReasoning, suggSource, suggMetadata sql.NullString
	var suggConfidence sql.NullFloat64
	var corrValue, corrReason, corrMetadata sql.NullString
	var ctxFileID, ctxFilePath, ctxFileType, ctxFolderPath, ctxSessionID, ctxFull sql.NullString
	var responseTime sql.NullInt64
	var createdAt int64

	err := row.Scan(
		&id, &workspaceID, &actionType, &suggType, &suggValue,
		&suggConfidence, &suggReasoning, &suggSource, &suggMetadata,
		&corrValue, &corrReason, &corrMetadata,
		&ctxFileID, &ctxFilePath, &ctxFileType, &ctxFolderPath,
		&ctxSessionID, &ctxFull, &responseTime, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	feedback := &entity.UserFeedback{
		ID:          entity.FeedbackID(id),
		WorkspaceID: entity.WorkspaceID(workspaceID),
		ActionType:  entity.FeedbackActionType(actionType),
		CreatedAt:   time.UnixMilli(createdAt),
	}

	if responseTime.Valid {
		feedback.ResponseTime = responseTime.Int64
	}

	if suggType.Valid {
		feedback.Suggestion = &entity.Suggestion{
			Type:       entity.SuggestionType(suggType.String),
			Value:      suggValue.String,
			Confidence: suggConfidence.Float64,
			Reasoning:  suggReasoning.String,
			Source:     suggSource.String,
		}
		if suggMetadata.Valid && suggMetadata.String != "" {
			json.Unmarshal([]byte(suggMetadata.String), &feedback.Suggestion.Metadata)
		}
	}

	if corrValue.Valid {
		feedback.Correction = &entity.Correction{
			Value:  corrValue.String,
			Reason: corrReason.String,
		}
		if corrMetadata.Valid && corrMetadata.String != "" {
			json.Unmarshal([]byte(corrMetadata.String), &feedback.Correction.Metadata)
		}
	}

	if ctxFull.Valid && ctxFull.String != "" {
		feedback.Context = &entity.FeedbackContext{}
		feedback.Context.FromJSON(ctxFull.String)
	}

	return feedback, nil
}

func (r *FeedbackRepository) scanFeedbackRows(rows *sql.Rows) ([]*entity.UserFeedback, error) {
	results := make([]*entity.UserFeedback, 0)

	for rows.Next() {
		var id, workspaceID, actionType string
		var suggType, suggValue, suggReasoning, suggSource, suggMetadata sql.NullString
		var suggConfidence sql.NullFloat64
		var corrValue, corrReason, corrMetadata sql.NullString
		var ctxFileID, ctxFilePath, ctxFileType, ctxFolderPath, ctxSessionID, ctxFull sql.NullString
		var responseTime sql.NullInt64
		var createdAt int64

		err := rows.Scan(
			&id, &workspaceID, &actionType, &suggType, &suggValue,
			&suggConfidence, &suggReasoning, &suggSource, &suggMetadata,
			&corrValue, &corrReason, &corrMetadata,
			&ctxFileID, &ctxFilePath, &ctxFileType, &ctxFolderPath,
			&ctxSessionID, &ctxFull, &responseTime, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		feedback := &entity.UserFeedback{
			ID:          entity.FeedbackID(id),
			WorkspaceID: entity.WorkspaceID(workspaceID),
			ActionType:  entity.FeedbackActionType(actionType),
			CreatedAt:   time.UnixMilli(createdAt),
		}

		if responseTime.Valid {
			feedback.ResponseTime = responseTime.Int64
		}

		if suggType.Valid {
			feedback.Suggestion = &entity.Suggestion{
				Type:       entity.SuggestionType(suggType.String),
				Value:      suggValue.String,
				Confidence: suggConfidence.Float64,
				Reasoning:  suggReasoning.String,
				Source:     suggSource.String,
			}
		}

		if corrValue.Valid {
			feedback.Correction = &entity.Correction{
				Value:  corrValue.String,
				Reason: corrReason.String,
			}
		}

		if ctxFull.Valid && ctxFull.String != "" {
			feedback.Context = &entity.FeedbackContext{}
			feedback.Context.FromJSON(ctxFull.String)
		}

		results = append(results, feedback)
	}

	return results, rows.Err()
}

func (r *FeedbackRepository) scanPreference(row *sql.Row) (*entity.LearnedPreference, error) {
	var id, workspaceID, prefType, patternJSON, behaviorJSON string
	var confidence float64
	var examples int
	var lastUsed sql.NullInt64
	var createdAt, updatedAt int64

	err := row.Scan(&id, &workspaceID, &prefType, &patternJSON, &behaviorJSON,
		&confidence, &examples, &lastUsed, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	pref := &entity.LearnedPreference{
		ID:          entity.PreferenceID(id),
		WorkspaceID: entity.WorkspaceID(workspaceID),
		Type:        entity.PreferenceType(prefType),
		Confidence:  confidence,
		Examples:    examples,
		CreatedAt:   time.UnixMilli(createdAt),
		UpdatedAt:   time.UnixMilli(updatedAt),
	}

	if lastUsed.Valid {
		t := time.UnixMilli(lastUsed.Int64)
		pref.LastUsed = &t
	}

	pref.Pattern = &entity.PreferencePattern{}
	pref.Pattern.FromJSON(patternJSON)

	pref.Behavior = &entity.PreferenceBehavior{}
	pref.Behavior.FromJSON(behaviorJSON)

	return pref, nil
}

func (r *FeedbackRepository) scanPreferenceRows(rows *sql.Rows) ([]*entity.LearnedPreference, error) {
	results := make([]*entity.LearnedPreference, 0)

	for rows.Next() {
		var id, workspaceID, prefType, patternJSON, behaviorJSON string
		var confidence float64
		var examples int
		var lastUsed sql.NullInt64
		var createdAt, updatedAt int64

		err := rows.Scan(&id, &workspaceID, &prefType, &patternJSON, &behaviorJSON,
			&confidence, &examples, &lastUsed, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		pref := &entity.LearnedPreference{
			ID:          entity.PreferenceID(id),
			WorkspaceID: entity.WorkspaceID(workspaceID),
			Type:        entity.PreferenceType(prefType),
			Confidence:  confidence,
			Examples:    examples,
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		}

		if lastUsed.Valid {
			t := time.UnixMilli(lastUsed.Int64)
			pref.LastUsed = &t
		}

		pref.Pattern = &entity.PreferencePattern{}
		pref.Pattern.FromJSON(patternJSON)

		pref.Behavior = &entity.PreferenceBehavior{}
		pref.Behavior.FromJSON(behaviorJSON)

		results = append(results, pref)
	}

	return results, rows.Err()
}

func (r *FeedbackRepository) sanitizeSortField(field string) string {
	allowed := map[string]bool{
		"created_at":    true,
		"action_type":   true,
		"response_time": true,
	}
	if allowed[field] {
		return field
	}
	return "created_at"
}

func (r *FeedbackRepository) sanitizePrefSortField(field string) string {
	allowed := map[string]bool{
		"confidence": true,
		"examples":   true,
		"created_at": true,
		"updated_at": true,
		"last_used":  true,
	}
	if allowed[field] {
		return field
	}
	return "confidence"
}

func (r *FeedbackRepository) sortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}
