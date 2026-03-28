package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestFeedbackRepository_SaveAndGetFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create feedback
	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeTag,
		Value:      "important",
		Confidence: 0.85,
		Reasoning:  "Contains keywords",
		Source:     "llm",
		Metadata:   map[string]any{"model": "llama3"},
	}

	feedbackCtx := &entity.FeedbackContext{
		FileID:       "file-123",
		FilePath:     "docs/readme.md",
		FileType:     ".md",
		FileSize:     1024,
		FileTags:     []string{"documentation"},
		FileProjects: []string{"cortex"},
		FolderPath:   "docs",
		SessionID:    "session-abc",
		TimeOfDay:    "morning",
		DayOfWeek:    "Monday",
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
	feedback.WithContext(feedbackCtx)
	feedback.WithResponseTime(1500)

	// Save
	if err := repo.SaveFeedback(ctx, feedback); err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	// Get
	retrieved, err := repo.GetFeedback(ctx, workspace.ID, feedback.ID)
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	// Verify
	if retrieved.ID != feedback.ID {
		t.Errorf("ID mismatch: expected %s, got %s", feedback.ID, retrieved.ID)
	}

	if retrieved.ActionType != entity.FeedbackActionAccepted {
		t.Errorf("ActionType mismatch: expected %s, got %s", entity.FeedbackActionAccepted, retrieved.ActionType)
	}

	if retrieved.Suggestion == nil {
		t.Fatal("Expected suggestion to be present")
	}

	if retrieved.Suggestion.Value != "important" {
		t.Errorf("Suggestion value mismatch: expected 'important', got '%s'", retrieved.Suggestion.Value)
	}

	if retrieved.Suggestion.Type != entity.SuggestionTypeTag {
		t.Errorf("Suggestion type mismatch: expected %s, got %s", entity.SuggestionTypeTag, retrieved.Suggestion.Type)
	}

	if retrieved.Context == nil {
		t.Fatal("Expected context to be present")
	}

	if retrieved.Context.FileType != ".md" {
		t.Errorf("Context FileType mismatch: expected '.md', got '%s'", retrieved.Context.FileType)
	}

	if retrieved.ResponseTime != 1500 {
		t.Errorf("ResponseTime mismatch: expected 1500, got %d", retrieved.ResponseTime)
	}
}

func TestFeedbackRepository_SaveFeedbackWithCorrection(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	suggestion := &entity.Suggestion{
		Type:       entity.SuggestionTypeProject,
		Value:      "ProjectA",
		Confidence: 0.7,
	}

	correction := &entity.Correction{
		Value:    "ProjectB",
		Reason:   "Wrong project suggested",
		Metadata: map[string]any{"corrected_by": "user"},
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionCorrected, suggestion)
	feedback.WithCorrection(correction)

	if err := repo.SaveFeedback(ctx, feedback); err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	retrieved, err := repo.GetFeedback(ctx, workspace.ID, feedback.ID)
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	if retrieved.Correction == nil {
		t.Fatal("Expected correction to be present")
	}

	if retrieved.Correction.Value != "ProjectB" {
		t.Errorf("Correction value mismatch: expected 'ProjectB', got '%s'", retrieved.Correction.Value)
	}

	if retrieved.Correction.Reason != "Wrong project suggested" {
		t.Errorf("Correction reason mismatch")
	}
}

func TestFeedbackRepository_DeleteFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	suggestion := &entity.Suggestion{
		Type:  entity.SuggestionTypeTag,
		Value: "test",
	}

	feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)

	if err := repo.SaveFeedback(ctx, feedback); err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	// Delete
	if err := repo.DeleteFeedback(ctx, workspace.ID, feedback.ID); err != nil {
		t.Fatalf("DeleteFeedback failed: %v", err)
	}

	// Verify deletion
	_, err = repo.GetFeedback(ctx, workspace.ID, feedback.ID)
	if err == nil {
		t.Error("Expected error getting deleted feedback")
	}
}

func TestFeedbackRepository_ListFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create multiple feedbacks
	for i := 0; i < 5; i++ {
		suggestion := &entity.Suggestion{
			Type:  entity.SuggestionTypeTag,
			Value: "tag" + string(rune('A'+i)),
		}
		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		if err := repo.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %d failed: %v", i, err)
		}
	}

	// List all
	opts := repository.DefaultFeedbackListOptions()
	feedbacks, err := repo.ListFeedback(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(feedbacks) != 5 {
		t.Errorf("Expected 5 feedbacks, got %d", len(feedbacks))
	}

	// Test limit
	opts.Limit = 3
	feedbacks, err = repo.ListFeedback(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListFeedback with limit failed: %v", err)
	}

	if len(feedbacks) != 3 {
		t.Errorf("Expected 3 feedbacks with limit, got %d", len(feedbacks))
	}

	// Test offset
	opts.Offset = 2
	opts.Limit = 10
	feedbacks, err = repo.ListFeedback(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListFeedback with offset failed: %v", err)
	}

	if len(feedbacks) != 3 {
		t.Errorf("Expected 3 feedbacks with offset 2, got %d", len(feedbacks))
	}
}

func TestFeedbackRepository_ListFeedbackByType(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create feedbacks with different suggestion types
	types := []entity.SuggestionType{
		entity.SuggestionTypeTag,
		entity.SuggestionTypeTag,
		entity.SuggestionTypeProject,
		entity.SuggestionTypeCategory,
	}

	for i, st := range types {
		suggestion := &entity.Suggestion{
			Type:  st,
			Value: "value" + string(rune('A'+i)),
		}
		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		if err := repo.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %d failed: %v", i, err)
		}
	}

	opts := repository.DefaultFeedbackListOptions()

	// Filter by tag type
	tagFeedbacks, err := repo.ListFeedbackByType(ctx, workspace.ID, entity.SuggestionTypeTag, opts)
	if err != nil {
		t.Fatalf("ListFeedbackByType failed: %v", err)
	}

	if len(tagFeedbacks) != 2 {
		t.Errorf("Expected 2 tag feedbacks, got %d", len(tagFeedbacks))
	}

	// Filter by project type
	projectFeedbacks, err := repo.ListFeedbackByType(ctx, workspace.ID, entity.SuggestionTypeProject, opts)
	if err != nil {
		t.Fatalf("ListFeedbackByType failed: %v", err)
	}

	if len(projectFeedbacks) != 1 {
		t.Errorf("Expected 1 project feedback, got %d", len(projectFeedbacks))
	}
}

func TestFeedbackRepository_ListFeedbackByAction(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create feedbacks with different actions
	actions := []entity.FeedbackActionType{
		entity.FeedbackActionAccepted,
		entity.FeedbackActionAccepted,
		entity.FeedbackActionRejected,
		entity.FeedbackActionCorrected,
		entity.FeedbackActionIgnored,
	}

	for i, action := range actions {
		suggestion := &entity.Suggestion{
			Type:  entity.SuggestionTypeTag,
			Value: "value" + string(rune('A'+i)),
		}
		feedback := entity.NewUserFeedback(workspace.ID, action, suggestion)
		if err := repo.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %d failed: %v", i, err)
		}
	}

	opts := repository.DefaultFeedbackListOptions()

	// Filter by accepted
	accepted, err := repo.ListFeedbackByAction(ctx, workspace.ID, entity.FeedbackActionAccepted, opts)
	if err != nil {
		t.Fatalf("ListFeedbackByAction failed: %v", err)
	}

	if len(accepted) != 2 {
		t.Errorf("Expected 2 accepted feedbacks, got %d", len(accepted))
	}

	// Filter by rejected
	rejected, err := repo.ListFeedbackByAction(ctx, workspace.ID, entity.FeedbackActionRejected, opts)
	if err != nil {
		t.Fatalf("ListFeedbackByAction failed: %v", err)
	}

	if len(rejected) != 1 {
		t.Errorf("Expected 1 rejected feedback, got %d", len(rejected))
	}
}

func TestFeedbackRepository_ListFeedbackSince(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create feedbacks
	for i := 0; i < 3; i++ {
		suggestion := &entity.Suggestion{
			Type:  entity.SuggestionTypeTag,
			Value: "value" + string(rune('A'+i)),
		}
		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		if err := repo.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %d failed: %v", i, err)
		}
	}

	opts := repository.DefaultFeedbackListOptions()

	// Query with time before creation
	since := time.Now().Add(-1 * time.Hour)
	feedbacks, err := repo.ListFeedbackSince(ctx, workspace.ID, since, opts)
	if err != nil {
		t.Fatalf("ListFeedbackSince failed: %v", err)
	}

	if len(feedbacks) != 3 {
		t.Errorf("Expected 3 feedbacks since an hour ago, got %d", len(feedbacks))
	}

	// Query with time after creation
	since = time.Now().Add(1 * time.Hour)
	feedbacks, err = repo.ListFeedbackSince(ctx, workspace.ID, since, opts)
	if err != nil {
		t.Fatalf("ListFeedbackSince failed: %v", err)
	}

	if len(feedbacks) != 0 {
		t.Errorf("Expected 0 feedbacks since an hour from now, got %d", len(feedbacks))
	}
}

func TestFeedbackRepository_GetFeedbackStats(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create feedbacks with various actions
	testCases := []struct {
		action entity.FeedbackActionType
		count  int
	}{
		{entity.FeedbackActionAccepted, 5},
		{entity.FeedbackActionRejected, 2},
		{entity.FeedbackActionCorrected, 1},
		{entity.FeedbackActionIgnored, 2},
	}

	for _, tc := range testCases {
		for i := 0; i < tc.count; i++ {
			suggestion := &entity.Suggestion{
				Type:  entity.SuggestionTypeTag,
				Value: "value",
			}
			feedback := entity.NewUserFeedback(workspace.ID, tc.action, suggestion)
			feedback.WithResponseTime(int64(100 * (i + 1)))
			if err := repo.SaveFeedback(ctx, feedback); err != nil {
				t.Fatalf("SaveFeedback failed: %v", err)
			}
		}
	}

	stats, err := repo.GetFeedbackStats(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("GetFeedbackStats failed: %v", err)
	}

	if stats.TotalFeedback != 10 {
		t.Errorf("Expected 10 total feedbacks, got %d", stats.TotalFeedback)
	}

	if stats.AcceptedCount != 5 {
		t.Errorf("Expected 5 accepted, got %d", stats.AcceptedCount)
	}

	if stats.RejectedCount != 2 {
		t.Errorf("Expected 2 rejected, got %d", stats.RejectedCount)
	}

	if stats.CorrectedCount != 1 {
		t.Errorf("Expected 1 corrected, got %d", stats.CorrectedCount)
	}

	if stats.IgnoredCount != 2 {
		t.Errorf("Expected 2 ignored, got %d", stats.IgnoredCount)
	}

	// Acceptance rate should be 5/10 = 0.5
	expectedAcceptRate := 0.5
	if stats.AcceptanceRate < expectedAcceptRate-0.01 || stats.AcceptanceRate > expectedAcceptRate+0.01 {
		t.Errorf("Expected acceptance rate ~%f, got %f", expectedAcceptRate, stats.AcceptanceRate)
	}
}

func TestFeedbackRepository_PreferenceCRUD(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create preference
	pattern := &entity.PreferencePattern{
		FileExtensions: []string{".pdf", ".docx"},
		FolderPatterns: []string{"documents/*"},
		Keywords:       []string{"invoice", "receipt"},
	}

	behavior := &entity.PreferenceBehavior{
		PreferredTags:     []string{"financial", "important"},
		AvoidedTags:       []string{"draft"},
		PreferredProjects: []string{"Finance"},
		ConfidenceBoost:   0.15,
		AutoAccept:        true,
	}

	pref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pattern, behavior)
	pref.Confidence = 0.85

	// Save
	if err := repo.SavePreference(ctx, pref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	// Get
	retrieved, err := repo.GetPreference(ctx, workspace.ID, pref.ID)
	if err != nil {
		t.Fatalf("GetPreference failed: %v", err)
	}

	if retrieved.ID != pref.ID {
		t.Errorf("ID mismatch")
	}

	if retrieved.Type != entity.PreferenceTypeTagging {
		t.Errorf("Type mismatch")
	}

	if retrieved.Confidence != 0.85 {
		t.Errorf("Confidence mismatch: expected 0.85, got %f", retrieved.Confidence)
	}

	if retrieved.Pattern == nil {
		t.Fatal("Expected pattern to be present")
	}

	if len(retrieved.Pattern.FileExtensions) != 2 {
		t.Errorf("Expected 2 file extensions, got %d", len(retrieved.Pattern.FileExtensions))
	}

	if retrieved.Behavior == nil {
		t.Fatal("Expected behavior to be present")
	}

	if len(retrieved.Behavior.PreferredTags) != 2 {
		t.Errorf("Expected 2 preferred tags, got %d", len(retrieved.Behavior.PreferredTags))
	}

	// Update
	retrieved.Confidence = 0.95
	retrieved.Examples = 10
	if err := repo.UpdatePreference(ctx, retrieved); err != nil {
		t.Fatalf("UpdatePreference failed: %v", err)
	}

	updated, err := repo.GetPreference(ctx, workspace.ID, pref.ID)
	if err != nil {
		t.Fatalf("GetPreference after update failed: %v", err)
	}

	if updated.Confidence != 0.95 {
		t.Errorf("Updated confidence mismatch: expected 0.95, got %f", updated.Confidence)
	}

	if updated.Examples != 10 {
		t.Errorf("Updated examples mismatch: expected 10, got %d", updated.Examples)
	}

	// Delete
	if err := repo.DeletePreference(ctx, workspace.ID, pref.ID); err != nil {
		t.Fatalf("DeletePreference failed: %v", err)
	}

	_, err = repo.GetPreference(ctx, workspace.ID, pref.ID)
	if err == nil {
		t.Error("Expected error getting deleted preference")
	}
}

func TestFeedbackRepository_ListPreferences(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create preferences with different types
	types := []entity.PreferenceType{
		entity.PreferenceTypeTagging,
		entity.PreferenceTypeTagging,
		entity.PreferenceTypeCategorization,
		entity.PreferenceTypeClustering,
	}

	for i, pt := range types {
		pref := entity.NewLearnedPreference(workspace.ID, pt, nil, nil)
		pref.Confidence = 0.5 + float64(i)*0.1
		if err := repo.SavePreference(ctx, pref); err != nil {
			t.Fatalf("SavePreference %d failed: %v", i, err)
		}
	}

	opts := repository.DefaultPreferenceListOptions()

	// List all
	prefs, err := repo.ListPreferences(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListPreferences failed: %v", err)
	}

	if len(prefs) != 4 {
		t.Errorf("Expected 4 preferences, got %d", len(prefs))
	}

	// List by type
	taggingPrefs, err := repo.ListPreferencesByType(ctx, workspace.ID, entity.PreferenceTypeTagging, opts)
	if err != nil {
		t.Fatalf("ListPreferencesByType failed: %v", err)
	}

	if len(taggingPrefs) != 2 {
		t.Errorf("Expected 2 tagging preferences, got %d", len(taggingPrefs))
	}

	// Test min confidence filter
	opts.MinConfidence = 0.65
	highConfPrefs, err := repo.ListPreferences(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListPreferences with min confidence failed: %v", err)
	}

	for _, p := range highConfPrefs {
		if p.Confidence < 0.65 {
			t.Errorf("Found preference with confidence %f below threshold 0.65", p.Confidence)
		}
	}
}

func TestFeedbackRepository_FindMatchingPreferences(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create preferences with different patterns
	pdfPattern := &entity.PreferencePattern{
		FileExtensions: []string{".pdf"},
	}
	pdfPref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, pdfPattern, nil)
	if err := repo.SavePreference(ctx, pdfPref); err != nil {
		t.Fatalf("SavePreference pdf failed: %v", err)
	}

	docPattern := &entity.PreferencePattern{
		FileExtensions: []string{".docx", ".doc"},
	}
	docPref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, docPattern, nil)
	if err := repo.SavePreference(ctx, docPref); err != nil {
		t.Fatalf("SavePreference doc failed: %v", err)
	}

	invoicePattern := &entity.PreferencePattern{
		FolderPatterns: []string{"invoices"},
	}
	invoicePref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeCategorization, invoicePattern, nil)
	if err := repo.SavePreference(ctx, invoicePref); err != nil {
		t.Fatalf("SavePreference invoice failed: %v", err)
	}

	// Find matching for PDF file
	pdfContext := &entity.FeedbackContext{
		FileType: ".pdf",
	}
	matches, err := repo.FindMatchingPreferences(ctx, workspace.ID, pdfContext)
	if err != nil {
		t.Fatalf("FindMatchingPreferences failed: %v", err)
	}

	if len(matches) == 0 {
		t.Error("Expected at least one matching preference for PDF")
	}

	// Find matching for invoice folder
	invoiceContext := &entity.FeedbackContext{
		FolderPath: "invoices",
	}
	matches, err = repo.FindMatchingPreferences(ctx, workspace.ID, invoiceContext)
	if err != nil {
		t.Fatalf("FindMatchingPreferences failed: %v", err)
	}

	if len(matches) == 0 {
		t.Error("Expected at least one matching preference for invoices folder")
	}
}

func TestFeedbackRepository_DeleteOldFeedback(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create some feedbacks
	for i := 0; i < 5; i++ {
		suggestion := &entity.Suggestion{
			Type:  entity.SuggestionTypeTag,
			Value: "test",
		}
		feedback := entity.NewUserFeedback(workspace.ID, entity.FeedbackActionAccepted, suggestion)
		if err := repo.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %d failed: %v", i, err)
		}
	}

	// Delete feedback older than future time (should delete all)
	deleted, err := repo.DeleteOldFeedback(ctx, workspace.ID, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOldFeedback failed: %v", err)
	}

	if deleted != 5 {
		t.Errorf("Expected 5 deletions, got %d", deleted)
	}

	// Verify all deleted
	opts := repository.DefaultFeedbackListOptions()
	remaining, err := repo.ListFeedback(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(remaining) != 0 {
		t.Errorf("Expected 0 remaining feedbacks, got %d", len(remaining))
	}
}

func TestFeedbackRepository_DeleteLowConfidencePreferences(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create preferences with varying confidence
	confidences := []float64{0.1, 0.2, 0.5, 0.8, 0.95}
	for i, conf := range confidences {
		pref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, nil, nil)
		pref.Confidence = conf
		if err := repo.SavePreference(ctx, pref); err != nil {
			t.Fatalf("SavePreference %d failed: %v", i, err)
		}
	}

	// Delete preferences with confidence < 0.3
	deleted, err := repo.DeleteLowConfidencePreferences(ctx, workspace.ID, 0.3)
	if err != nil {
		t.Fatalf("DeleteLowConfidencePreferences failed: %v", err)
	}

	if deleted != 2 {
		t.Errorf("Expected 2 deletions (0.1 and 0.2), got %d", deleted)
	}

	// Verify remaining
	opts := repository.DefaultPreferenceListOptions()
	remaining, err := repo.ListPreferences(ctx, workspace.ID, opts)
	if err != nil {
		t.Fatalf("ListPreferences failed: %v", err)
	}

	if len(remaining) != 3 {
		t.Errorf("Expected 3 remaining preferences, got %d", len(remaining))
	}

	for _, p := range remaining {
		if p.Confidence < 0.3 {
			t.Errorf("Found preference with confidence %f below threshold 0.3", p.Confidence)
		}
	}
}

func TestFeedbackRepository_GetStalePreferences(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFeedbackRepository(conn.DB())

	// Create preference (will have LastUsed = nil initially)
	pref := entity.NewLearnedPreference(workspace.ID, entity.PreferenceTypeTagging, nil, nil)
	if err := repo.SavePreference(ctx, pref); err != nil {
		t.Fatalf("SavePreference failed: %v", err)
	}

	// Get stale preferences (unused since future time should include all with nil LastUsed)
	stale, err := repo.GetStalePreferences(ctx, workspace.ID, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("GetStalePreferences failed: %v", err)
	}

	// Should find the preference since it was never used
	if len(stale) == 0 {
		t.Error("Expected to find stale preference")
	}
}
