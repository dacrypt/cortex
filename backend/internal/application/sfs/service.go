// Package sfs provides a Semantic File System service for natural language file organization.
// Inspired by the LSFS paper - allows users to organize files using natural language commands.
package sfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// OperationType defines the type of SFS operation.
type OperationType string

const (
	OperationUnknown   OperationType = "unknown"
	OperationGroup     OperationType = "group"
	OperationFind      OperationType = "find"
	OperationTag       OperationType = "tag"
	OperationUntag     OperationType = "untag"
	OperationAssign    OperationType = "assign"
	OperationUnassign  OperationType = "unassign"
	OperationCreate    OperationType = "create"
	OperationMerge     OperationType = "merge"
	OperationRename    OperationType = "rename"
	OperationSummarize OperationType = "summarize"
	OperationRelate    OperationType = "relate"
	OperationQuery     OperationType = "query"
)

// FileChange represents a change to apply to a file.
type FileChange struct {
	FileID       entity.FileID
	RelativePath string
	Operation    OperationType
	BeforeValue  string
	AfterValue   string
	Target       string
}

// CommandResult represents the result of executing a command.
type CommandResult struct {
	Success       bool
	Operation     OperationType
	Changes       []FileChange
	Explanation   string
	ErrorMessage  string
	FilesAffected int
	UndoCommand   string
}

// PreviewResult shows what would happen without executing.
type PreviewResult struct {
	Operation                  OperationType
	PlannedChanges             []FileChange
	Explanation                string
	FilesAffected              int
	Confidence                 float64
	Warnings                   []string
	AlternativeInterpretations []string
}

// CommandSuggestion represents a suggested command.
type CommandSuggestion struct {
	Command     string
	Description string
	Operation   OperationType
	Relevance   float64
	Category    string
}

// CommandHistoryEntry represents a past command execution.
type CommandHistoryEntry struct {
	ID            string
	WorkspaceID   entity.WorkspaceID
	Command       string
	Operation     OperationType
	Success       bool
	FilesAffected int
	ExecutedAt    time.Time
	ResultSummary string
}

// ServiceConfig configures the SFS service.
type ServiceConfig struct {
	MaxFilesPerOperation int     // Maximum files to affect in one operation
	MinConfidence        float64 // Minimum confidence to execute without confirmation
	EnableUndo           bool    // Track changes for undo support
}

// DefaultServiceConfig returns the default configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MaxFilesPerOperation: 1000,
		MinConfidence:        0.7,
		EnableUndo:           true,
	}
}

// LLMRouter provides LLM completion capabilities.
type LLMRouter interface {
	Complete(ctx context.Context, prompt string, maxTokens int) (string, error)
}

// Service provides semantic file system operations.
type Service struct {
	config      ServiceConfig
	fileRepo    repository.FileRepository
	projectRepo repository.ProjectRepository
	metaRepo    repository.MetadataRepository
	llmRouter   LLMRouter
	parser      *CommandParser
	executor    *CommandExecutor
	history     []CommandHistoryEntry
	logger      zerolog.Logger
}

// NewService creates a new SFS service.
func NewService(
	config ServiceConfig,
	fileRepo repository.FileRepository,
	projectRepo repository.ProjectRepository,
	metaRepo repository.MetadataRepository,
	llmRouter LLMRouter,
	logger zerolog.Logger,
) *Service {
	s := &Service{
		config:      config,
		fileRepo:    fileRepo,
		projectRepo: projectRepo,
		metaRepo:    metaRepo,
		llmRouter:   llmRouter,
		history:     make([]CommandHistoryEntry, 0),
		logger:      logger.With().Str("component", "sfs-service").Logger(),
	}

	s.parser = NewCommandParser(llmRouter, logger)
	s.executor = NewCommandExecutor(fileRepo, projectRepo, metaRepo, logger)

	return s
}

// ExecuteCommand parses and executes a natural language command.
func (s *Service) ExecuteCommand(ctx context.Context, workspaceID entity.WorkspaceID, command string, contextFileIDs []entity.FileID) (*CommandResult, error) {
	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("command", command).
		Int("context_files", len(contextFileIDs)).
		Msg("Executing SFS command")

	// Parse the command
	parsed, err := s.parser.Parse(ctx, command, contextFileIDs)
	if err != nil {
		return &CommandResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse command: %v", err),
		}, nil
	}

	// Execute the command
	result, err := s.executor.Execute(ctx, workspaceID, parsed)
	if err != nil {
		return &CommandResult{
			Success:      false,
			Operation:    parsed.Operation,
			ErrorMessage: fmt.Sprintf("Failed to execute command: %v", err),
		}, nil
	}

	// Record history
	s.recordHistory(workspaceID, command, result)

	return result, nil
}

// PreviewCommand shows what would happen without executing.
func (s *Service) PreviewCommand(ctx context.Context, workspaceID entity.WorkspaceID, command string, contextFileIDs []entity.FileID) (*PreviewResult, error) {
	s.logger.Debug().
		Str("workspace_id", workspaceID.String()).
		Str("command", command).
		Msg("Previewing SFS command")

	// Parse the command
	parsed, err := s.parser.Parse(ctx, command, contextFileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	// Generate preview
	preview, err := s.executor.Preview(ctx, workspaceID, parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate preview: %w", err)
	}

	return preview, nil
}

// SuggestCommands suggests possible commands based on context.
func (s *Service) SuggestCommands(ctx context.Context, workspaceID entity.WorkspaceID, partialCommand string, contextFileIDs []entity.FileID, limit int) ([]CommandSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}

	suggestions := make([]CommandSuggestion, 0)

	// Add context-based suggestions
	if len(contextFileIDs) > 0 {
		suggestions = append(suggestions, s.getContextualSuggestions(ctx, workspaceID, contextFileIDs)...)
	}

	// Add partial command completions
	if partialCommand != "" {
		suggestions = append(suggestions, s.getCompletionSuggestions(partialCommand)...)
	}

	// Add general suggestions
	suggestions = append(suggestions, s.getGeneralSuggestions()...)

	// Limit results
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// GetCommandHistory returns recent commands for the workspace.
func (s *Service) GetCommandHistory(workspaceID entity.WorkspaceID, limit int, since time.Time) []CommandHistoryEntry {
	var filtered []CommandHistoryEntry

	for _, entry := range s.history {
		if entry.WorkspaceID == workspaceID && entry.ExecutedAt.After(since) {
			filtered = append(filtered, entry)
		}
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

// recordHistory records a command execution in history.
func (s *Service) recordHistory(workspaceID entity.WorkspaceID, command string, result *CommandResult) {
	entry := CommandHistoryEntry{
		ID:            uuid.New().String(),
		WorkspaceID:   workspaceID,
		Command:       command,
		Operation:     result.Operation,
		Success:       result.Success,
		FilesAffected: result.FilesAffected,
		ExecutedAt:    time.Now(),
		ResultSummary: result.Explanation,
	}

	s.history = append(s.history, entry)

	// Keep history limited
	if len(s.history) > 100 {
		s.history = s.history[1:]
	}
}

// getContextualSuggestions returns suggestions based on selected files.
func (s *Service) getContextualSuggestions(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) []CommandSuggestion {
	suggestions := []CommandSuggestion{
		{
			Command:     "group these files by type",
			Description: "Organize selected files into projects by file type",
			Operation:   OperationGroup,
			Relevance:   0.9,
			Category:    "organization",
		},
		{
			Command:     "tag these files as important",
			Description: "Add 'important' tag to selected files",
			Operation:   OperationTag,
			Relevance:   0.85,
			Category:    "tagging",
		},
		{
			Command:     "summarize these files",
			Description: "Generate AI summary for selected files",
			Operation:   OperationSummarize,
			Relevance:   0.8,
			Category:    "analysis",
		},
		{
			Command:     "find related files",
			Description: "Find files related to selected ones",
			Operation:   OperationFind,
			Relevance:   0.75,
			Category:    "search",
		},
	}

	return suggestions
}

// getCompletionSuggestions returns suggestions for partial commands.
func (s *Service) getCompletionSuggestions(partial string) []CommandSuggestion {
	partial = strings.ToLower(partial)
	suggestions := []CommandSuggestion{}

	completions := map[string][]CommandSuggestion{
		"group": {
			{Command: "group files by extension", Description: "Organize by file type", Operation: OperationGroup, Relevance: 0.95},
			{Command: "group files by folder", Description: "Organize by directory", Operation: OperationGroup, Relevance: 0.9},
			{Command: "group files by date", Description: "Organize by modification date", Operation: OperationGroup, Relevance: 0.85},
		},
		"find": {
			{Command: "find all PDFs", Description: "Find PDF documents", Operation: OperationFind, Relevance: 0.95},
			{Command: "find files modified today", Description: "Recent files", Operation: OperationFind, Relevance: 0.9},
			{Command: "find large files", Description: "Files over 10MB", Operation: OperationFind, Relevance: 0.85},
		},
		"tag": {
			{Command: "tag with review-needed", Description: "Mark for review", Operation: OperationTag, Relevance: 0.95},
			{Command: "tag with archive", Description: "Mark as archived", Operation: OperationTag, Relevance: 0.9},
		},
		"create": {
			{Command: "create project from folder", Description: "New project from current folder", Operation: OperationCreate, Relevance: 0.95},
			{Command: "create project for these files", Description: "New project with selected files", Operation: OperationCreate, Relevance: 0.9},
		},
	}

	for prefix, comps := range completions {
		if strings.HasPrefix(prefix, partial) || strings.Contains(prefix, partial) {
			for _, c := range comps {
				c.Category = "completion"
				suggestions = append(suggestions, c)
			}
		}
	}

	return suggestions
}

// getGeneralSuggestions returns common command suggestions.
func (s *Service) getGeneralSuggestions() []CommandSuggestion {
	return []CommandSuggestion{
		{
			Command:     "show unorganized files",
			Description: "Find files not in any project",
			Operation:   OperationFind,
			Relevance:   0.7,
			Category:    "discovery",
		},
		{
			Command:     "find duplicate files",
			Description: "Locate potential duplicates",
			Operation:   OperationFind,
			Relevance:   0.65,
			Category:    "discovery",
		},
		{
			Command:     "organize by AI suggestion",
			Description: "Apply AI-suggested organization",
			Operation:   OperationGroup,
			Relevance:   0.6,
			Category:    "automation",
		},
	}
}
