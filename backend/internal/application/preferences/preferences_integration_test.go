package preferences

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestPreferenceService_RecordFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Create workspace
	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, feedbackRepo, logger)

	// Create feedback with a tag suggestion
	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeTag,
		Value:      "important",
		Confidence: 0.85,
		Reasoning:  "File contains keywords",
		Source:     "llm",
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
	feedback.WithContext(&entity.FeedbackContext{
		FileID:     "file-123",
		FilePath:   "docs/readme.md",
		FileType:   ".md",
		FolderPath: "docs",
	})
	feedback.WithResponseTime(1500)

	err = service.RecordFeedback(ctx, feedback)
	if err != nil {
		t.Fatalf("RecordFeedback failed: %v", err)
	}

	// Verify feedback was saved
	savedFeedback, err := feedbackRepo.GetFeedback(ctx, workspace.ID, feedback.ID)
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	if savedFeedback.ActionType != entity.FeedbackActionAccepted {
		t.Errorf("Expected action type %s, got %s", entity.FeedbackActionAccepted, savedFeedback.ActionType)
	}

	if savedFeedback.Suggestion.Value != "important" {
		t.Errorf("Expected suggestion value 'important', got '%s'", savedFeedback.Suggestion.Value)
	}

	if savedFeedback.ResponseTime != 1500 {
		t.Errorf("Expected response time 1500, got %d", savedFeedback.ResponseTime)
	}
}

func TestPreferenceService_RecordAndLearnFromFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, feedbackRepo, logger)

	// Record multiple feedbacks to build preference
	feedbackContext := &entity.FeedbackContext{
		FileType:   ".pdf",
		FolderPath: "invoices",
	}

	for i := 0; i < 3; i++ {
		suggestion := &entity.Suggestion{
			Type:       entity.SuggestionTypeTag,
			Value:      "invoice",
			Confidence: 0.8,
			Source:     "llm",
		}

		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		feedback.WithContext(feedbackContext)

		if err := service.RecordFeedback(ctx, feedback); err != nil {
			t.Fatalf("RecordFeedback %d failed: %v", i, err)
		}
	}

	// Check that preferences were created/updated
	prefs, err := service.GetPreferences(ctx, workspace.ID, entity.PreferenceTypeTagging)
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	if len(prefs) == 0 {
		t.Error("Expected at least one preference to be learned")
	}

	// Verify confidence increased
	if len(prefs) > 0 {
		pref := prefs[0]
		if pref.Confidence < 0.6 {
			t.Errorf("Expected higher confidence after multiple acceptances, got %f", pref.Confidence)
		}
	}
}

func TestPreferenceService_ApplyPreferences(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()
	config.HighConfidenceThreshold = 0.7

	service := NewService(config, feedbackRepo, logger)

	// Create a high-confidence preference manually
	pattern := &entity.PreferencePattern{
		FileExtensions: []string{".pdf"},
	}
	behavior := &entity.PreferenceBehavior{
		PreferredTags:   []string{"document"},
		ConfidenceBoost: 0.2,
	}
	pref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pattern, behavior)
	pref.Confidence = 0.9 // High confidence

	if err := feedbackRepo.SavePreference(ctx, pref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	// Create a suggestion to apply preferences to
	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeTag,
		Value:      "document",
		Confidence: 0.5,
	}

	feedbackContext := &entity.FeedbackContext{
		FileType: ".pdf",
	}

	// Apply preferences
	modified, err := service.ApplyPreferences(ctx, workspace.ID, suggestion, feedbackContext)
	if err != nil {
		t.Fatalf("ApplyPreferences failed: %v", err)
	}

	// Confidence should be boosted
	if modified.Confidence <= suggestion.Confidence {
		t.Errorf("Expected boosted confidence > %f, got %f", suggestion.Confidence, modified.Confidence)
	}
}

func TestPreferenceService_RejectedFeedbackWeakensPreference(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, feedbackRepo, logger)

	// Create initial preference
	pattern := &entity.PreferencePattern{
		FileExtensions: []string{".txt"},
	}
	behavior := &entity.PreferenceBehavior{
		PreferredTags: []string{"notes"},
	}
	pref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pattern, behavior)
	pref.Confidence = 0.8

	if err := feedbackRepo.SavePreference(ctx, pref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	initialConfidence := pref.Confidence

	// Record rejected feedback
	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeTag,
		Value:      "notes",
		Confidence: 0.7,
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionRejected, suggestion)
	feedback.WithContext(&entity.FeedbackContext{
		FileType: ".txt",
	})

	if err := service.RecordFeedback(ctx, feedback); err != nil {
		t.Fatalf("RecordFeedback failed: %v", err)
	}

	// Verify confidence decreased
	updatedPref, err := feedbackRepo.GetPreference(ctx, workspace.ID, pref.ID)
	if err != nil {
		t.Fatalf("GetPreference failed: %v", err)
	}

	if updatedPref.Confidence >= initialConfidence {
		t.Errorf("Expected confidence to decrease from %f, got %f", initialConfidence, updatedPref.Confidence)
	}
}

func TestPreferenceService_CorrectedFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, feedbackRepo, logger)

	// Record corrected feedback
	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeTag,
		Value:      "draft",
		Confidence: 0.7,
	}

	correction := &entity.Correction{
		Value:  "final",
		Reason: "User prefers 'final' over 'draft'",
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionCorrected, suggestion)
	feedback.WithCorrection(correction)
	feedback.WithContext(&entity.FeedbackContext{
		FileType:   ".docx",
		FolderPath: "documents",
	})

	if err := service.RecordFeedback(ctx, feedback); err != nil {
		t.Fatalf("RecordFeedback failed: %v", err)
	}

	// Verify correction was saved
	savedFeedback, err := feedbackRepo.GetFeedback(ctx, workspace.ID, feedback.ID)
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	if savedFeedback.Correction == nil {
		t.Error("Expected correction to be saved")
	} else if savedFeedback.Correction.Value != "final" {
		t.Errorf("Expected correction value 'final', got '%s'", savedFeedback.Correction.Value)
	}
}

func TestPreferenceService_GetFeedbackStats(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, feedbackRepo, logger)

	// Record various feedbacks
	actions := []entity.FeedbackActionType{
		entity.FeedbackActionAccepted,
		entity.FeedbackActionAccepted,
		entity.FeedbackActionRejected,
		entity.FeedbackActionCorrected,
	}

	for i, action := range actions {
		suggestion := &entity.Suggestion{
			Type:       entity.SuggestionTypeTag,
			Value:      "test",
			Confidence: 0.7,
		}

		feedback := entity.NewUserFeedback(workspace.ID, action, suggestion)
		feedback.WithContext(&entity.FeedbackContext{
			FileID: string(rune('a' + i)),
		})

		if err := service.RecordFeedback(ctx, feedback); err != nil {
			t.Fatalf("RecordFeedback %d failed: %v", i, err)
		}
	}

	// Get stats
	stats, err := service.GetFeedbackStats(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("GetFeedbackStats failed: %v", err)
	}

	if stats.TotalFeedback != 4 {
		t.Errorf("Expected 4 total feedbacks, got %d", stats.TotalFeedback)
	}

	if stats.AcceptedCount != 2 {
		t.Errorf("Expected 2 accepted, got %d", stats.AcceptedCount)
	}

	if stats.RejectedCount != 1 {
		t.Errorf("Expected 1 rejected, got %d", stats.RejectedCount)
	}

	if stats.CorrectedCount != 1 {
		t.Errorf("Expected 1 corrected, got %d", stats.CorrectedCount)
	}
}

func TestPreferenceService_ConsolidatePatterns(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()
	config.MinExamplesForHighConfidence = 3

	service := NewService(config, feedbackRepo, logger)

	// Record multiple similar feedbacks
	for i := 0; i < 5; i++ {
		suggestion := &entity.Suggestion{
			Type:       entity.SuggestionTypeTag,
			Value:      "report",
			Confidence: 0.75,
		}

		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		feedback.WithContext(&entity.FeedbackContext{
			FileType:   ".pdf",
			FolderPath: "reports",
		})

		if err := service.RecordFeedback(ctx, feedback); err != nil {
			t.Fatalf("RecordFeedback %d failed: %v", i, err)
		}
	}

	// Consolidate patterns
	err = service.ConsolidatePatterns(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("ConsolidatePatterns failed: %v", err)
	}

	// This is primarily a smoke test - consolidation logs patterns
	// Real verification would check logs or pattern storage
}

func TestPreferenceService_Cleanup(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	feedbackRepo := sqlite.NewFeedbackRepository(conn.DB())
	logger := zerolog.Nop()
	config := DefaultServiceConfig()
	config.CleanupLowConfidenceThreshold = 0.3

	service := NewService(config, feedbackRepo, logger)

	// Create low-confidence preference
	pattern := &entity.PreferencePattern{
		FileExtensions: []string{".tmp"},
	}
	behavior := &entity.PreferenceBehavior{
		PreferredTags: []string{"temporary"},
	}
	lowConfPref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pattern, behavior)
	lowConfPref.Confidence = 0.1 // Very low

	if err := feedbackRepo.SavePreference(ctx, lowConfPref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	// Create high-confidence preference
	highConfPref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pattern, behavior)
	highConfPref.Confidence = 0.9

	if err := feedbackRepo.SavePreference(ctx, highConfPref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	// Run cleanup
	deleted, err := service.Cleanup(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Low confidence preference should be deleted
	if deleted < 1 {
		t.Errorf("Expected at least 1 deletion, got %d", deleted)
	}

	// High confidence preference should remain
	prefs, err := service.GetPreferences(ctx, workspace.ID, "")
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	for _, p := range prefs {
		if p.Confidence < config.CleanupLowConfidenceThreshold {
			t.Errorf("Found preference with confidence %f below threshold %f",
				p.Confidence, config.CleanupLowConfidenceThreshold)
		}
	}
}

func TestLearnedPreference_MatchesContext(t *testing.T) {
	t.Parallel()

	workspaceID := entity.NewWorkspaceID()

	tests := []struct {
		name        string
		pattern     *entity.PreferencePattern
		context     *entity.FeedbackContext
		shouldMatch bool
	}{
		{
			name: "matches file extension",
			pattern: &entity.PreferencePattern{
				FileExtensions: []string{".pdf", ".docx"},
			},
			context: &entity.FeedbackContext{
				FileType: ".pdf",
			},
			shouldMatch: true,
		},
		{
			name: "does not match file extension",
			pattern: &entity.PreferencePattern{
				FileExtensions: []string{".pdf", ".docx"},
			},
			context: &entity.FeedbackContext{
				FileType: ".txt",
			},
			shouldMatch: false,
		},
		{
			name: "matches folder pattern",
			pattern: &entity.PreferencePattern{
				FolderPatterns: []string{"invoices"},
			},
			context: &entity.FeedbackContext{
				FolderPath: "invoices",
			},
			shouldMatch: true,
		},
		{
			name: "does not match folder pattern",
			pattern: &entity.PreferencePattern{
				FolderPatterns: []string{"invoices"},
			},
			context: &entity.FeedbackContext{
				FolderPath: "receipts",
			},
			shouldMatch: false,
		},
		{
			name:    "nil pattern returns false",
			pattern: nil,
			context: &entity.FeedbackContext{
				FileType: ".pdf",
			},
			shouldMatch: false,
		},
		{
			name: "nil context returns false",
			pattern: &entity.PreferencePattern{
				FileExtensions: []string{".pdf"},
			},
			context:     nil,
			shouldMatch: false,
		},
		{
			name:        "empty pattern matches any context",
			pattern:     &entity.PreferencePattern{},
			context:     &entity.FeedbackContext{FileType: ".pdf"},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pref := entity.NewLearnedPreference(workspaceID, entity.PreferenceTypeTagging, tt.pattern, nil)
			result := pref.MatchesContext(tt.context)

			if result != tt.shouldMatch {
				t.Errorf("MatchesContext() = %v, expected %v", result, tt.shouldMatch)
			}
		})
	}
}

func TestLearnedPreference_ReinforceAndWeaken(t *testing.T) {
	t.Parallel()

	workspaceID := entity.NewWorkspaceID()
	pref := entity.NewLearnedPreference(workspaceID, entity.PreferenceTypeTagging, nil, nil)

	initialConfidence := pref.Confidence
	initialExamples := pref.Examples

	// Reinforce
	pref.Reinforce()

	if pref.Confidence <= initialConfidence {
		t.Errorf("Reinforce should increase confidence: %f -> %f", initialConfidence, pref.Confidence)
	}

	if pref.Examples != initialExamples+1 {
		t.Errorf("Reinforce should increment examples: %d -> %d", initialExamples, pref.Examples)
	}

	if pref.LastUsed == nil {
		t.Error("Reinforce should set LastUsed")
	}

	// Weaken
	beforeWeaken := pref.Confidence
	pref.Weaken()

	if pref.Confidence >= beforeWeaken {
		t.Errorf("Weaken should decrease confidence: %f -> %f", beforeWeaken, pref.Confidence)
	}

	// Weaken multiple times - should not go below 0.1
	for i := 0; i < 100; i++ {
		pref.Weaken()
	}

	if pref.Confidence < 0.1 {
		t.Errorf("Confidence should not go below 0.1, got %f", pref.Confidence)
	}
}

func TestServiceConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := DefaultServiceConfig()

	if config.MinExamplesForHighConfidence != 5 {
		t.Errorf("Expected MinExamplesForHighConfidence 5, got %d", config.MinExamplesForHighConfidence)
	}

	if config.HighConfidenceThreshold != 0.8 {
		t.Errorf("Expected HighConfidenceThreshold 0.8, got %f", config.HighConfidenceThreshold)
	}

	if config.StalePreferenceDays != 90 {
		t.Errorf("Expected StalePreferenceDays 90, got %d", config.StalePreferenceDays)
	}

	if config.CleanupLowConfidenceThreshold != 0.2 {
		t.Errorf("Expected CleanupLowConfidenceThreshold 0.2, got %f", config.CleanupLowConfidenceThreshold)
	}
}
