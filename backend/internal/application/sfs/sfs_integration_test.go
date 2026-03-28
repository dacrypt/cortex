package sfs

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// MockLLMRouter implements LLMRouter for testing.
type MockLLMRouter struct {
	responses map[string]string
}

func NewMockLLMRouter() *MockLLMRouter {
	return &MockLLMRouter{
		responses: make(map[string]string),
	}
}

func (m *MockLLMRouter) Complete(ctx context.Context, prompt string, maxTokens int) (string, error) {
	// Return canned responses for known prompts
	return "group|test-project|group_by=type|0.85", nil
}

// TestCommandParser_PatternMatching tests regex-based command parsing.
func TestCommandParser_PatternMatching(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	tests := []struct {
		name           string
		command        string
		expectedOp     OperationType
		expectedTarget string
		expectedCrit   map[string]string
	}{
		{
			name:       "group by extension",
			command:    "group files by extension",
			expectedOp: OperationGroup,
			expectedCrit: map[string]string{
				"group_by": "extension",
			},
		},
		{
			name:       "group by type",
			command:    "group by type",
			expectedOp: OperationGroup,
			expectedCrit: map[string]string{
				"group_by": "type",
			},
		},
		{
			name:           "group into project",
			command:        "group these files into MyProject",
			expectedOp:     OperationGroup,
			expectedTarget: "MyProject",
		},
		{
			name:       "find PDF files",
			command:    "find all PDF files",
			expectedOp: OperationFind,
			expectedCrit: map[string]string{
				"extension": "PDF",
			},
		},
		{
			name:       "find modified files",
			command:    "find files modified today",
			expectedOp: OperationFind,
			expectedCrit: map[string]string{
				"modified": "today",
			},
		},
		{
			name:       "find unorganized",
			command:    "show unorganized files",
			expectedOp: OperationFind,
			expectedCrit: map[string]string{
				"unorganized": "true",
			},
		},
		{
			name:       "find large files",
			command:    "find large files",
			expectedOp: OperationFind,
			expectedCrit: map[string]string{
				"size": "large",
			},
		},
		{
			name:       "find duplicates",
			command:    "find duplicate files",
			expectedOp: OperationFind,
			expectedCrit: map[string]string{
				"duplicates": "true",
			},
		},
		{
			name:           "tag files",
			command:        "tag these files as important",
			expectedOp:     OperationTag,
			expectedTarget: "important",
		},
		{
			name:           "add tag",
			command:        "add tag review-needed to files",
			expectedOp:     OperationTag,
			expectedTarget: "review-needed",
		},
		{
			name:           "untag files",
			command:        "untag draft from files",
			expectedOp:     OperationUntag,
			expectedTarget: "draft",
		},
		{
			name:           "assign to project",
			command:        "assign files to project Alpha",
			expectedOp:     OperationAssign,
			expectedTarget: "Alpha",
		},
		{
			name:           "add to project",
			command:        "add these files to project Beta",
			expectedOp:     OperationAssign,
			expectedTarget: "Beta",
		},
		{
			name:           "unassign from project",
			command:        "remove these files from project Gamma",
			expectedOp:     OperationUnassign,
			expectedTarget: "Gamma",
		},
		{
			name:           "create project",
			command:        "create project TestProject",
			expectedOp:     OperationCreate,
			expectedTarget: "TestProject",
		},
		{
			name:           "create named project",
			command:        "create a new project called MyNewProject",
			expectedOp:     OperationCreate,
			expectedTarget: "MyNewProject",
		},
		{
			name:       "create from folder",
			command:    "create project from folder",
			expectedOp: OperationCreate,
			expectedCrit: map[string]string{
				"from": "folder",
			},
		},
		{
			name:           "merge projects",
			command:        "merge project Alpha into Beta",
			expectedOp:     OperationMerge,
			expectedTarget: "Beta",
			expectedCrit: map[string]string{
				"source": "Alpha",
			},
		},
		{
			name:           "rename project",
			command:        "rename project OldName to NewName",
			expectedOp:     OperationRename,
			expectedTarget: "NewName",
			expectedCrit: map[string]string{
				"from": "OldName",
			},
		},
		{
			name:       "summarize files",
			command:    "summarize these files",
			expectedOp: OperationSummarize,
		},
		{
			name:       "relate files",
			command:    "relate these files",
			expectedOp: OperationRelate,
		},
		{
			name:       "query",
			command:    "what are the most important files?",
			expectedOp: OperationQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parser.Parse(ctx, tt.command, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if parsed.Operation != tt.expectedOp {
				t.Errorf("Expected operation %s, got %s", tt.expectedOp, parsed.Operation)
			}

			if tt.expectedTarget != "" && parsed.Target != tt.expectedTarget {
				t.Errorf("Expected target '%s', got '%s'", tt.expectedTarget, parsed.Target)
			}

			for key, expectedVal := range tt.expectedCrit {
				if gotVal, ok := parsed.Criteria[key]; !ok {
					t.Errorf("Expected criteria key '%s' not found", key)
				} else if gotVal != expectedVal {
					t.Errorf("Criteria '%s': expected '%s', got '%s'", key, expectedVal, gotVal)
				}
			}

			if parsed.RawCommand != tt.command {
				t.Errorf("RawCommand mismatch: expected '%s', got '%s'", tt.command, parsed.RawCommand)
			}
		})
	}
}

// TestCommandParser_UnknownCommands tests handling of unrecognized commands.
func TestCommandParser_UnknownCommands(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	unknownCommands := []string{
		"do something strange",
		"xyzzy",
		"frobulate the widgets",
	}

	for _, cmd := range unknownCommands {
		t.Run(cmd, func(t *testing.T) {
			parsed, err := parser.Parse(ctx, cmd, nil)
			if err != nil {
				t.Fatalf("Parse should not error: %v", err)
			}

			if parsed.Operation != OperationUnknown {
				t.Errorf("Expected OperationUnknown, got %s", parsed.Operation)
			}

			if parsed.Confidence >= 0.5 {
				t.Errorf("Unknown command should have low confidence, got %f", parsed.Confidence)
			}
		})
	}
}

// TestCommandParser_EmptyCommand tests handling of empty commands.
func TestCommandParser_EmptyCommand(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	_, err := parser.Parse(ctx, "", nil)
	if err == nil {
		t.Error("Expected error for empty command")
	}

	_, err = parser.Parse(ctx, "   ", nil)
	if err == nil {
		t.Error("Expected error for whitespace-only command")
	}
}

// TestCommandParser_WithContextFiles tests command parsing with context files.
func TestCommandParser_WithContextFiles(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	fileIDs := []entity.FileID{
		entity.FileID("file1"),
		entity.FileID("file2"),
		entity.FileID("file3"),
	}

	parsed, err := parser.Parse(ctx, "tag these files as important", fileIDs)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(parsed.FileIDs) != 3 {
		t.Errorf("Expected 3 file IDs, got %d", len(parsed.FileIDs))
	}

	for i, fid := range parsed.FileIDs {
		if fid != fileIDs[i] {
			t.Errorf("FileID mismatch at index %d", i)
		}
	}
}

// TestCommandParser_CaseInsensitivity tests that commands are case insensitive.
func TestCommandParser_CaseInsensitivity(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	variations := []string{
		"GROUP FILES BY EXTENSION",
		"Group Files By Extension",
		"group files by extension",
		"GrOuP fIlEs By ExTeNsIoN",
	}

	for _, cmd := range variations {
		t.Run(cmd, func(t *testing.T) {
			parsed, err := parser.Parse(ctx, cmd, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if parsed.Operation != OperationGroup {
				t.Errorf("Expected OperationGroup, got %s", parsed.Operation)
			}
		})
	}
}

// TestCommandParser_Interpretation tests interpretation generation.
func TestCommandParser_Interpretation(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	parser := NewCommandParser(nil, logger)
	ctx := context.Background()

	tests := []struct {
		command              string
		expectedInterpContains string
	}{
		{"group files by type", "Group files by type"},
		{"find PDF files", "extension"},
		{"tag as important", "important"},
		{"assign to project Alpha", "Alpha"},
		{"create project NewProject", "NewProject"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			parsed, err := parser.Parse(ctx, tt.command, nil)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if parsed.Interpretation == "" {
				t.Error("Expected non-empty interpretation")
			}

			if tt.expectedInterpContains != "" {
				if !containsIgnoreCase(parsed.Interpretation, tt.expectedInterpContains) {
					t.Errorf("Interpretation '%s' should contain '%s'",
						parsed.Interpretation, tt.expectedInterpContains)
				}
			}
		})
	}
}

// TestService_SuggestCommands tests command suggestions.
func TestService_SuggestCommands(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	svc := &Service{
		config:  config,
		history: make([]CommandHistoryEntry, 0),
		logger:  logger,
	}
	svc.parser = NewCommandParser(nil, logger)

	ctx := context.Background()
	workspaceID := entity.NewWorkspaceID()

	// Test contextual suggestions
	fileIDs := []entity.FileID{
		entity.FileID("file1"),
		entity.FileID("file2"),
	}
	suggestions, err := svc.SuggestCommands(ctx, workspaceID, "", fileIDs, 5)
	if err != nil {
		t.Fatalf("SuggestCommands failed: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("Expected some suggestions with context files")
	}

	// Test partial command suggestions
	suggestions, err = svc.SuggestCommands(ctx, workspaceID, "group", nil, 10)
	if err != nil {
		t.Fatalf("SuggestCommands failed: %v", err)
	}

	hasGroupCompletion := false
	for _, s := range suggestions {
		if s.Operation == OperationGroup {
			hasGroupCompletion = true
			break
		}
	}
	if !hasGroupCompletion {
		t.Error("Expected group command completions for 'group' partial")
	}

	// Test limit
	suggestions, err = svc.SuggestCommands(ctx, workspaceID, "", nil, 3)
	if err != nil {
		t.Fatalf("SuggestCommands failed: %v", err)
	}

	if len(suggestions) > 3 {
		t.Errorf("Expected at most 3 suggestions, got %d", len(suggestions))
	}
}

// TestService_CommandHistory tests command history tracking.
func TestService_CommandHistory(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	svc := &Service{
		config:  config,
		history: make([]CommandHistoryEntry, 0),
		logger:  logger,
	}

	workspaceID := entity.NewWorkspaceID()
	otherWorkspaceID := entity.NewWorkspaceID()

	// Record some history
	result1 := &CommandResult{
		Success:       true,
		Operation:     OperationTag,
		FilesAffected: 5,
		Explanation:   "Tagged 5 files",
	}
	svc.recordHistory(workspaceID, "tag as important", result1)

	result2 := &CommandResult{
		Success:       true,
		Operation:     OperationFind,
		FilesAffected: 10,
		Explanation:   "Found 10 files",
	}
	svc.recordHistory(workspaceID, "find PDF files", result2)

	result3 := &CommandResult{
		Success:       false,
		Operation:     OperationAssign,
		ErrorMessage:  "Project not found",
	}
	svc.recordHistory(otherWorkspaceID, "assign to unknown", result3)

	// Get history for workspace
	history := svc.GetCommandHistory(workspaceID, 10, zeroTime())
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}

	// Verify entries
	if history[0].Command != "tag as important" {
		t.Errorf("Unexpected first command: %s", history[0].Command)
	}
	if !history[0].Success {
		t.Error("First command should be successful")
	}

	// Test limit
	history = svc.GetCommandHistory(workspaceID, 1, zeroTime())
	if len(history) != 1 {
		t.Errorf("Expected 1 history entry with limit, got %d", len(history))
	}

	// Test other workspace
	history = svc.GetCommandHistory(otherWorkspaceID, 10, zeroTime())
	if len(history) != 1 {
		t.Errorf("Expected 1 history entry for other workspace, got %d", len(history))
	}
	if history[0].Success {
		t.Error("Other workspace command should be failed")
	}
}

// TestParseOperationType tests operation type parsing.
func TestParseOperationType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected OperationType
	}{
		{"group", OperationGroup},
		{"GROUP", OperationGroup},
		{"Group", OperationGroup},
		{"find", OperationFind},
		{"FIND", OperationFind},
		{"tag", OperationTag},
		{"untag", OperationUntag},
		{"assign", OperationAssign},
		{"unassign", OperationUnassign},
		{"create", OperationCreate},
		{"merge", OperationMerge},
		{"rename", OperationRename},
		{"summarize", OperationSummarize},
		{"relate", OperationRelate},
		{"query", OperationQuery},
		{"unknown", OperationUnknown},
		{"", OperationUnknown},
		{"invalid", OperationUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseOperationType(tt.input)
			if result != tt.expected {
				t.Errorf("parseOperationType(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseFloat tests float parsing.
func TestParseFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"0.5", 0.5, false},
		{"0.85", 0.85, false},
		{"1.0", 1.0, false},
		{" 0.9 ", 0.9, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseFloat(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s'", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("parseFloat(%s) = %f, expected %f", tt.input, result, tt.expected)
				}
			}
		})
	}
}

// Helper functions

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsIgnoreCase(s[1:], substr) ||
		len(s) >= len(substr) && equalIgnoreCase(s[:len(substr)], substr))
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func zeroTime() (t time.Time) {
	return
}
