// Package main provides a migration tool to convert contexts to projects.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func main() {
	dbPath := flag.String("db", "", "Path to SQLite database file")
	workspaceIDStr := flag.String("workspace-id", "", "Workspace ID (optional, will use first workspace if not provided)")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without making changes")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -db flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Open database connection
	conn, err := sqlite.NewConnection(*dbPath)
	if err != nil {
		logger.Fatal().Err(err).Str("db", *dbPath).Msg("Failed to open database")
	}
	defer conn.Close()

	ctx := context.Background()

	// Migrate database schema if needed
	if err := conn.Migrate(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to migrate database")
	}

	// Determine workspace ID
	var workspaceID entity.WorkspaceID
	if *workspaceIDStr != "" {
		workspaceID = entity.WorkspaceID(*workspaceIDStr)
	} else {
		// Get first workspace
		workspaceRepo := sqlite.NewWorkspaceRepository(conn)
		workspaces, err := workspaceRepo.List(ctx, repository.WorkspaceListOptions{})
		if err != nil || len(workspaces) == 0 {
			logger.Fatal().Msg("No workspaces found. Please specify -workspace-id")
		}
		workspaceID = workspaces[0].ID
		logger.Info().Str("workspace_id", workspaceID.String()).Msg("Using first workspace")
	}

	if *dryRun {
		logger.Info().Msg("DRY RUN MODE - No changes will be made")
	}

	// Run migration
	if err := migrateContextsToProjects(ctx, conn, workspaceID, *dryRun, logger); err != nil {
		logger.Fatal().Err(err).Msg("Migration failed")
	}

	logger.Info().Msg("Migration completed successfully")
}

func migrateContextsToProjects(
	ctx context.Context,
	conn *sqlite.Connection,
	workspaceID entity.WorkspaceID,
	dryRun bool,
	logger zerolog.Logger,
) error {
	metaRepo := sqlite.NewMetadataRepository(conn)
	projectRepo := sqlite.NewProjectRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)

	// Get all unique contexts
	contexts, err := metaRepo.GetAllContexts(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get contexts: %w", err)
	}

	if len(contexts) == 0 {
		logger.Info().Msg("No contexts found to migrate")
		return nil
	}

	logger.Info().Int("count", len(contexts)).Msg("Found contexts to migrate")

	createdCount := 0
	associatedCount := 0
	skippedCount := 0

	for _, contextName := range contexts {
		logger.Info().Str("context", contextName).Msg("Processing context")

		// Check if project already exists
		proj, err := projectRepo.GetByName(ctx, workspaceID, contextName, nil)
		if err == nil && proj != nil {
			logger.Debug().Str("context", contextName).Str("project_id", proj.ID.String()).Msg("Project already exists")
		} else {
			// Create new project
			if !dryRun {
				proj = entity.NewProject(workspaceID, contextName, nil)
				if err := projectRepo.Create(ctx, workspaceID, proj); err != nil {
					logger.Warn().Err(err).Str("context", contextName).Msg("Failed to create project")
					skippedCount++
					continue
				}
				logger.Info().Str("context", contextName).Str("project_id", proj.ID.String()).Msg("Created new project")
			} else {
				logger.Info().Str("context", contextName).Msg("Would create new project")
				proj = &entity.Project{
					ID:          entity.NewProjectID(),
					WorkspaceID: workspaceID,
					Name:        contextName,
				}
			}
			createdCount++
		}

		// Get all files with this context
		fileMetas, err := metaRepo.ListByContext(ctx, workspaceID, contextName, repository.DefaultFileListOptions())
		if err != nil {
			logger.Warn().Err(err).Str("context", contextName).Msg("Failed to list files for context")
			continue
		}

		logger.Debug().Str("context", contextName).Int("file_count", len(fileMetas)).Msg("Found files for context")

		// Associate each file's document with the project
		for _, fileMeta := range fileMetas {
			// Get document for this file
			doc, err := docRepo.GetDocumentByPath(ctx, workspaceID, fileMeta.RelativePath)
			if err != nil || doc == nil {
				logger.Debug().Str("path", fileMeta.RelativePath).Msg("Document not found, skipping")
				continue
			}

			if !dryRun {
				// Check if already associated
				projects, err := projectRepo.GetProjectsForDocument(ctx, workspaceID, doc.ID)
				if err == nil {
					alreadyAssociated := false
					for _, pID := range projects {
						if pID == proj.ID {
							alreadyAssociated = true
							break
						}
					}
					if alreadyAssociated {
						logger.Debug().Str("path", fileMeta.RelativePath).Str("project", contextName).Msg("Document already associated with project")
						continue
					}
				}

				// Associate document with project
				if err := projectRepo.AddDocument(ctx, workspaceID, proj.ID, doc.ID, entity.ProjectDocumentRolePrimary); err != nil {
					logger.Warn().Err(err).Str("path", fileMeta.RelativePath).Str("project", contextName).Msg("Failed to associate document with project")
				} else {
					logger.Debug().Str("path", fileMeta.RelativePath).Str("project", contextName).Msg("Associated document with project")
					associatedCount++
				}
			} else {
				logger.Info().Str("path", fileMeta.RelativePath).Str("project", contextName).Msg("Would associate document with project")
				associatedCount++
			}
		}
	}

	logger.Info().
		Int("created", createdCount).
		Int("associated", associatedCount).
		Int("skipped", skippedCount).
		Msg("Migration summary")

	return nil
}

