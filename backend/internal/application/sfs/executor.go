// Package sfs provides a Semantic File System service for natural language file organization.
package sfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// CommandExecutor executes parsed commands against the repositories.
type CommandExecutor struct {
	fileRepo    repository.FileRepository
	projectRepo repository.ProjectRepository
	metaRepo    repository.MetadataRepository
	logger      zerolog.Logger
}

// NewCommandExecutor creates a new command executor.
func NewCommandExecutor(
	fileRepo repository.FileRepository,
	projectRepo repository.ProjectRepository,
	metaRepo repository.MetadataRepository,
	logger zerolog.Logger,
) *CommandExecutor {
	return &CommandExecutor{
		fileRepo:    fileRepo,
		projectRepo: projectRepo,
		metaRepo:    metaRepo,
		logger:      logger.With().Str("component", "sfs-executor").Logger(),
	}
}

// Execute executes a parsed command and returns the result.
func (e *CommandExecutor) Execute(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	e.logger.Info().
		Str("operation", string(cmd.Operation)).
		Str("target", cmd.Target).
		Int("context_files", len(cmd.FileIDs)).
		Msg("Executing command")

	switch cmd.Operation {
	case OperationGroup:
		return e.executeGroup(ctx, workspaceID, cmd)
	case OperationFind:
		return e.executeFind(ctx, workspaceID, cmd)
	case OperationTag:
		return e.executeTag(ctx, workspaceID, cmd)
	case OperationUntag:
		return e.executeUntag(ctx, workspaceID, cmd)
	case OperationAssign:
		return e.executeAssign(ctx, workspaceID, cmd)
	case OperationUnassign:
		return e.executeUnassign(ctx, workspaceID, cmd)
	case OperationCreate:
		return e.executeCreate(ctx, workspaceID, cmd)
	case OperationMerge:
		return e.executeMerge(ctx, workspaceID, cmd)
	case OperationRename:
		return e.executeRename(ctx, workspaceID, cmd)
	case OperationSummarize:
		return e.executeSummarize(ctx, workspaceID, cmd)
	case OperationRelate:
		return e.executeRelate(ctx, workspaceID, cmd)
	case OperationQuery:
		return e.executeQuery(ctx, workspaceID, cmd)
	default:
		return &CommandResult{
			Success:      false,
			Operation:    cmd.Operation,
			ErrorMessage: fmt.Sprintf("Unknown operation: %s", cmd.Operation),
		}, nil
	}
}

// Preview generates a preview of what would happen without executing.
func (e *CommandExecutor) Preview(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*PreviewResult, error) {
	e.logger.Debug().
		Str("operation", string(cmd.Operation)).
		Str("target", cmd.Target).
		Msg("Generating preview")

	// Get affected files
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve files: %w", err)
	}

	// Build planned changes
	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		change := FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    cmd.Operation,
			Target:       cmd.Target,
		}
		changes = append(changes, change)
	}

	// Generate explanation
	explanation := e.generateExplanation(cmd, len(files))

	// Generate warnings
	warnings := e.generateWarnings(cmd, files)

	return &PreviewResult{
		Operation:      cmd.Operation,
		PlannedChanges: changes,
		Explanation:    explanation,
		FilesAffected:  len(files),
		Confidence:     cmd.Confidence,
		Warnings:       warnings,
	}, nil
}

// resolveFiles finds the files that match the command criteria.
func (e *CommandExecutor) resolveFiles(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) ([]*entity.FileEntry, error) {
	opts := repository.FileListOptions{
		Limit: 1000, // Safety limit
	}

	// If specific files are provided, use those
	if len(cmd.FileIDs) > 0 {
		files := make([]*entity.FileEntry, 0, len(cmd.FileIDs))
		for _, id := range cmd.FileIDs {
			file, err := e.fileRepo.GetByID(ctx, workspaceID, id)
			if err != nil {
				e.logger.Warn().Err(err).Str("file_id", id.String()).Msg("Failed to get file")
				continue
			}
			files = append(files, file)
		}
		return files, nil
	}

	// Use criteria to find files via specific repository methods
	for key, value := range cmd.Criteria {
		switch key {
		case "extension":
			return e.fileRepo.ListByExtension(ctx, workspaceID, value, opts)
		case "type":
			return e.fileRepo.ListByContentType(ctx, workspaceID, value, opts)
		case "folder":
			return e.fileRepo.ListByFolder(ctx, workspaceID, value, true, opts)
		}
	}

	// Default: list all files
	return e.fileRepo.List(ctx, workspaceID, opts)
}

// executeGroup groups files by the specified criteria.
func (e *CommandExecutor) executeGroup(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	// Group by target criteria (e.g., "extension", "type", "folder")
	groupBy := cmd.Target
	if groupBy == "" {
		groupBy = "extension"
	}

	groups := make(map[string][]*entity.FileEntry)
	for _, f := range files {
		var key string
		switch groupBy {
		case "extension", "type":
			key = f.Extension
		case "folder", "directory":
			parts := strings.Split(f.RelativePath, "/")
			if len(parts) > 1 {
				key = parts[0]
			} else {
				key = "root"
			}
		default:
			key = f.Extension
		}
		groups[key] = append(groups[key], f)
	}

	// Create or assign to projects for each group
	changes := make([]FileChange, 0)
	projectsCreated := 0

	for groupName, groupFiles := range groups {
		if len(groupFiles) == 0 {
			continue
		}

		// Create project for this group
		projectName := fmt.Sprintf("Files: %s", groupName)
		project := entity.NewProject(workspaceID, projectName, nil)
		project.Description = fmt.Sprintf("Auto-grouped by %s", groupBy)

		if err := e.projectRepo.Create(ctx, workspaceID, project); err != nil {
			e.logger.Warn().Err(err).Str("project", projectName).Msg("Failed to create project")
			continue
		}
		projectsCreated++

		// Assign files to project
		for _, f := range groupFiles {
			docID := entity.DocumentID(f.ID.String())
			if err := e.projectRepo.AddDocument(ctx, workspaceID, project.ID, docID, entity.ProjectDocumentRolePrimary); err != nil {
				e.logger.Warn().Err(err).Msg("Failed to assign file")
				continue
			}

			changes = append(changes, FileChange{
				FileID:       f.ID,
				RelativePath: f.RelativePath,
				Operation:    OperationGroup,
				AfterValue:   projectName,
				Target:       groupName,
			})
		}
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationGroup,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Created %d projects and organized %d files by %s", projectsCreated, len(changes), groupBy),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("ungroup files from %s projects", groupBy),
	}, nil
}

// executeFind finds files matching criteria.
func (e *CommandExecutor) executeFind(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		changes = append(changes, FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    OperationFind,
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationFind,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Found %d files matching criteria", len(files)),
		FilesAffected: len(files),
	}, nil
}

// executeTag adds tags to files.
func (e *CommandExecutor) executeTag(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	tag := cmd.Target
	if tag == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationTag,
			ErrorMessage: "No tag specified",
		}, nil
	}

	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		if err := e.metaRepo.AddTag(ctx, workspaceID, f.ID, tag); err != nil {
			e.logger.Warn().Err(err).Str("file", f.RelativePath).Msg("Failed to add tag")
			continue
		}

		changes = append(changes, FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    OperationTag,
			AfterValue:   tag,
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationTag,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Tagged %d files with '%s'", len(changes), tag),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("untag '%s' from these files", tag),
	}, nil
}

// executeUntag removes tags from files.
func (e *CommandExecutor) executeUntag(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	tag := cmd.Target
	if tag == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationUntag,
			ErrorMessage: "No tag specified",
		}, nil
	}

	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		if err := e.metaRepo.RemoveTag(ctx, workspaceID, f.ID, tag); err != nil {
			e.logger.Warn().Err(err).Str("file", f.RelativePath).Msg("Failed to remove tag")
			continue
		}

		changes = append(changes, FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    OperationUntag,
			BeforeValue:  tag,
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationUntag,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Removed tag '%s' from %d files", tag, len(changes)),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("tag these files as '%s'", tag),
	}, nil
}

// executeAssign assigns files to a project.
func (e *CommandExecutor) executeAssign(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	projectName := cmd.Target
	if projectName == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationAssign,
			ErrorMessage: "No project specified",
		}, nil
	}

	// Find or create project
	projects, err := e.projectRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var project *entity.Project
	for _, p := range projects {
		if strings.EqualFold(p.Name, projectName) {
			project = p
			break
		}
	}

	if project == nil {
		// Create new project
		project = entity.NewProject(workspaceID, projectName, nil)
		if err := e.projectRepo.Create(ctx, workspaceID, project); err != nil {
			return nil, fmt.Errorf("failed to create project: %w", err)
		}
	}

	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		docID := entity.DocumentID(f.ID.String())
		if err := e.projectRepo.AddDocument(ctx, workspaceID, project.ID, docID, entity.ProjectDocumentRolePrimary); err != nil {
			e.logger.Warn().Err(err).Msg("Failed to assign file")
			continue
		}

		changes = append(changes, FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    OperationAssign,
			AfterValue:   projectName,
			Target:       project.ID.String(),
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationAssign,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Assigned %d files to project '%s'", len(changes), projectName),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("unassign these files from '%s'", projectName),
	}, nil
}

// executeUnassign removes files from a project.
func (e *CommandExecutor) executeUnassign(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	projectName := cmd.Target
	if projectName == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationUnassign,
			ErrorMessage: "No project specified",
		}, nil
	}

	// Find project
	projects, err := e.projectRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var project *entity.Project
	for _, p := range projects {
		if strings.EqualFold(p.Name, projectName) {
			project = p
			break
		}
	}

	if project == nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationUnassign,
			ErrorMessage: fmt.Sprintf("Project '%s' not found", projectName),
		}, nil
	}

	changes := make([]FileChange, 0, len(files))
	for _, f := range files {
		docID := entity.DocumentID(f.ID.String())
		if err := e.projectRepo.RemoveDocument(ctx, workspaceID, project.ID, docID); err != nil {
			e.logger.Warn().Err(err).Msg("Failed to unassign file")
			continue
		}

		changes = append(changes, FileChange{
			FileID:       f.ID,
			RelativePath: f.RelativePath,
			Operation:    OperationUnassign,
			BeforeValue:  projectName,
			Target:       project.ID.String(),
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationUnassign,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Removed %d files from project '%s'", len(changes), projectName),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("assign these files to '%s'", projectName),
	}, nil
}

// executeCreate creates a new project.
func (e *CommandExecutor) executeCreate(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	projectName := cmd.Target
	if projectName == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationCreate,
			ErrorMessage: "No project name specified",
		}, nil
	}

	project := entity.NewProject(workspaceID, projectName, nil)

	// Add description from criteria if provided
	if desc, ok := cmd.Criteria["description"]; ok {
		project.Description = desc
	}

	if err := e.projectRepo.Create(ctx, workspaceID, project); err != nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationCreate,
			ErrorMessage: fmt.Sprintf("Failed to create project: %v", err),
		}, nil
	}

	// Optionally assign context files
	changes := make([]FileChange, 0)
	if len(cmd.FileIDs) > 0 {
		for _, fileID := range cmd.FileIDs {
			docID := entity.DocumentID(fileID.String())
			if err := e.projectRepo.AddDocument(ctx, workspaceID, project.ID, docID, entity.ProjectDocumentRolePrimary); err != nil {
				e.logger.Warn().Err(err).Msg("Failed to assign file to new project")
				continue
			}

			file, _ := e.fileRepo.GetByID(ctx, workspaceID, fileID)
			path := ""
			if file != nil {
				path = file.RelativePath
			}

			changes = append(changes, FileChange{
				FileID:       fileID,
				RelativePath: path,
				Operation:    OperationAssign,
				AfterValue:   projectName,
			})
		}
	}

	extraInfo := ""
	if len(changes) > 0 {
		extraInfo = fmt.Sprintf(" with %d files", len(changes))
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationCreate,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Created project '%s'%s", projectName, extraInfo),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("delete project '%s'", projectName),
	}, nil
}

// executeMerge merges two projects.
func (e *CommandExecutor) executeMerge(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	// Target is destination project, criteria["source"] is source project
	destName := cmd.Target
	sourceName := cmd.Criteria["source"]

	if destName == "" || sourceName == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationMerge,
			ErrorMessage: "Both source and destination projects must be specified",
		}, nil
	}

	projects, err := e.projectRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var sourceProject, destProject *entity.Project
	for _, p := range projects {
		if strings.EqualFold(p.Name, sourceName) {
			sourceProject = p
		}
		if strings.EqualFold(p.Name, destName) {
			destProject = p
		}
	}

	if sourceProject == nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationMerge,
			ErrorMessage: fmt.Sprintf("Source project '%s' not found", sourceName),
		}, nil
	}

	if destProject == nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationMerge,
			ErrorMessage: fmt.Sprintf("Destination project '%s' not found", destName),
		}, nil
	}

	// Get documents from source project
	sourceDocIDs, err := e.projectRepo.GetDocuments(ctx, workspaceID, sourceProject.ID, false)
	if err != nil {
		return nil, err
	}

	changes := make([]FileChange, 0, len(sourceDocIDs))
	for _, docID := range sourceDocIDs {
		// Assign to destination
		if err := e.projectRepo.AddDocument(ctx, workspaceID, destProject.ID, docID, entity.ProjectDocumentRolePrimary); err != nil {
			e.logger.Warn().Err(err).Msg("Failed to assign file during merge")
			continue
		}

		// Unassign from source
		if err := e.projectRepo.RemoveDocument(ctx, workspaceID, sourceProject.ID, docID); err != nil {
			e.logger.Warn().Err(err).Msg("Failed to unassign file during merge")
		}

		fileID := entity.FileID(docID.String())
		file, _ := e.fileRepo.GetByID(ctx, workspaceID, fileID)
		path := ""
		if file != nil {
			path = file.RelativePath
		}

		changes = append(changes, FileChange{
			FileID:       fileID,
			RelativePath: path,
			Operation:    OperationMerge,
			BeforeValue:  sourceName,
			AfterValue:   destName,
		})
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationMerge,
		Changes:       changes,
		Explanation:   fmt.Sprintf("Merged %d files from '%s' into '%s'", len(changes), sourceName, destName),
		FilesAffected: len(changes),
		UndoCommand:   fmt.Sprintf("move files from '%s' to '%s'", destName, sourceName),
	}, nil
}

// executeRename renames a project.
func (e *CommandExecutor) executeRename(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	oldName := cmd.Criteria["from"]
	newName := cmd.Target

	if oldName == "" || newName == "" {
		return &CommandResult{
			Success:      false,
			Operation:    OperationRename,
			ErrorMessage: "Both old and new project names must be specified",
		}, nil
	}

	projects, err := e.projectRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var project *entity.Project
	for _, p := range projects {
		if strings.EqualFold(p.Name, oldName) {
			project = p
			break
		}
	}

	if project == nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationRename,
			ErrorMessage: fmt.Sprintf("Project '%s' not found", oldName),
		}, nil
	}

	project.Name = newName

	if err := e.projectRepo.Update(ctx, workspaceID, project); err != nil {
		return &CommandResult{
			Success:      false,
			Operation:    OperationRename,
			ErrorMessage: fmt.Sprintf("Failed to rename project: %v", err),
		}, nil
	}

	return &CommandResult{
		Success:       true,
		Operation:     OperationRename,
		Explanation:   fmt.Sprintf("Renamed project '%s' to '%s'", oldName, newName),
		FilesAffected: 0,
		UndoCommand:   fmt.Sprintf("rename project '%s' to '%s'", newName, oldName),
	}, nil
}

// executeSummarize generates summaries for files (placeholder - needs LLM).
func (e *CommandExecutor) executeSummarize(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	files, err := e.resolveFiles(ctx, workspaceID, cmd)
	if err != nil {
		return nil, err
	}

	// Note: Actual summarization would require LLM integration
	return &CommandResult{
		Success:       true,
		Operation:     OperationSummarize,
		Explanation:   fmt.Sprintf("Would summarize %d files (LLM integration required)", len(files)),
		FilesAffected: len(files),
	}, nil
}

// executeRelate creates relationships between files.
func (e *CommandExecutor) executeRelate(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	// Placeholder - would need relationship repository
	return &CommandResult{
		Success:       true,
		Operation:     OperationRelate,
		Explanation:   "Relationship creation not yet implemented",
		FilesAffected: 0,
	}, nil
}

// executeQuery answers questions about files.
func (e *CommandExecutor) executeQuery(ctx context.Context, workspaceID entity.WorkspaceID, cmd *ParsedCommand) (*CommandResult, error) {
	// Placeholder - would need RAG/LLM integration
	return &CommandResult{
		Success:       true,
		Operation:     OperationQuery,
		Explanation:   fmt.Sprintf("Query: %s (RAG integration required)", cmd.Target),
		FilesAffected: 0,
	}, nil
}

// generateExplanation creates a human-readable explanation of the planned operation.
func (e *CommandExecutor) generateExplanation(cmd *ParsedCommand, fileCount int) string {
	switch cmd.Operation {
	case OperationGroup:
		return fmt.Sprintf("Will group %d files by %s", fileCount, cmd.Target)
	case OperationFind:
		return fmt.Sprintf("Will find %d files matching criteria", fileCount)
	case OperationTag:
		return fmt.Sprintf("Will add tag '%s' to %d files", cmd.Target, fileCount)
	case OperationUntag:
		return fmt.Sprintf("Will remove tag '%s' from %d files", cmd.Target, fileCount)
	case OperationAssign:
		return fmt.Sprintf("Will assign %d files to project '%s'", fileCount, cmd.Target)
	case OperationUnassign:
		return fmt.Sprintf("Will remove %d files from project '%s'", fileCount, cmd.Target)
	case OperationCreate:
		return fmt.Sprintf("Will create project '%s' with %d files", cmd.Target, fileCount)
	case OperationMerge:
		return fmt.Sprintf("Will merge %d files into '%s'", fileCount, cmd.Target)
	case OperationRename:
		return fmt.Sprintf("Will rename project to '%s'", cmd.Target)
	case OperationSummarize:
		return fmt.Sprintf("Will generate summaries for %d files", fileCount)
	case OperationRelate:
		return fmt.Sprintf("Will create relationships for %d files", fileCount)
	case OperationQuery:
		return fmt.Sprintf("Will answer query about %d files", fileCount)
	default:
		return fmt.Sprintf("Will perform %s on %d files", cmd.Operation, fileCount)
	}
}

// generateWarnings creates warnings for potentially destructive operations.
func (e *CommandExecutor) generateWarnings(cmd *ParsedCommand, files []*entity.FileEntry) []string {
	warnings := []string{}

	if len(files) > 100 {
		warnings = append(warnings, fmt.Sprintf("This operation will affect %d files", len(files)))
	}

	if cmd.Operation == OperationMerge {
		warnings = append(warnings, "Merge operations cannot be undone automatically")
	}

	if cmd.Confidence < 0.7 {
		warnings = append(warnings, "Low confidence in command interpretation")
	}

	return warnings
}
