package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/filesystem"
)

func startWatchers(
	ctx context.Context,
	cfg *config.Config,
	workspaceRepo repository.WorkspaceRepository,
	fileRepo repository.FileRepository,
	orchestrator *pipeline.Orchestrator,
	logger zerolog.Logger,
) ([]service.FileWatcher, error) {
	if len(cfg.WatchPaths) == 0 {
		return nil, nil
	}

	var watchers []service.FileWatcher

	for _, root := range cfg.WatchPaths {
		absPath, err := filepath.Abs(root)
		if err != nil {
			logger.Warn().Err(err).Str("path", root).Msg("Invalid watch path")
			continue
		}

		if err := os.MkdirAll(absPath, 0755); err != nil {
			return watchers, fmt.Errorf("create watch path: %w", err)
		}

		ws, err := ensureWorkspace(ctx, workspaceRepo, absPath, logger)
		if err != nil {
			return watchers, err
		}

		// Create indexer and watcher using factory functions
		// These return service interfaces, making it easy to swap implementations
		indexer := filesystem.NewFileIndexer(absPath, &ws.Config)
		if err := initialScan(ctx, ws, indexer, orchestrator, fileRepo, workspaceRepo, logger); err != nil {
			return watchers, err
		}

		watcher, err := filesystem.NewFileWatcher(absPath, &ws.Config)
		if err != nil {
			return watchers, err
		}
		if err := watcher.Start(); err != nil {
			return watchers, err
		}

		watchers = append(watchers, watcher)
		logger.Info().Str("path", absPath).Msg("Watching workspace path")

		go handleWatchEvents(ctx, watcher, indexer, orchestrator, fileRepo, workspaceRepo, ws, logger)
	}

	return watchers, nil
}

func ensureWorkspace(
	ctx context.Context,
	repo repository.WorkspaceRepository,
	path string,
	logger zerolog.Logger,
) (*entity.Workspace, error) {
	ws, err := repo.GetByPath(ctx, path)
	if err != nil {
		return nil, err
	}
	if ws != nil {
		return ws, nil
	}

	ws = entity.NewWorkspace(path, "")
	if err := repo.Create(ctx, ws); err != nil {
		return nil, err
	}

	logger.Info().
		Str("workspace_id", ws.ID.String()).
		Str("path", ws.Path).
		Msg("Workspace registered from watch_paths")

	return ws, nil
}

func initialScan(
	ctx context.Context,
	ws *entity.Workspace,
	indexer service.FileIndexer, // Using interface instead of concrete type
	processor pipeline.Processor, // Using interface for testability
	fileRepo repository.FileRepository,
	workspaceRepo repository.WorkspaceRepository,
	logger zerolog.Logger,
) error {
	entries, err := indexer.Scan(ctx, nil)
	if err != nil {
		return err
	}

	stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
		ID:     ws.ID,
		Root:   ws.Path,
		Config: ws.Config,
	})
	
	var filesProcessed, filesSkipped int
	for _, entry := range entries {
		// Check if file already exists and hasn't changed
		existing, err := fileRepo.GetByPath(ctx, ws.ID, entry.RelativePath)
		if err == nil && existing != nil {
			// Compare timestamp (with 1 second tolerance for filesystem precision) and size - skip if unchanged
			timeDiff := existing.LastModified.Sub(entry.LastModified)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff < time.Second && existing.FileSize == entry.FileSize {
				filesSkipped++
				logger.Debug().
					Str("path", entry.RelativePath).
					Msg("File unchanged, skipping initial scan")
				continue
			}
			logger.Info().
				Str("path", entry.RelativePath).
				Int64("old_size", existing.FileSize).
				Int64("new_size", entry.FileSize).
				Time("old_modified", existing.LastModified).
				Time("new_modified", entry.LastModified).
				Dur("time_diff", timeDiff).
				Msg("File changed, processing in initial scan")
		} else {
			logger.Debug().
				Str("path", entry.RelativePath).
				Msg("New file, processing in initial scan")
		}
		
		// Process new or changed files
		if err := processor.Process(stageCtx, entry); err != nil {
			// Pipeline failed - check if there are indexing errors to persist
			if entry.Enhanced != nil && entry.Enhanced.HasIndexingErrors() {
				logger.Warn().
					Err(err).
					Str("path", entry.RelativePath).
					Int("error_count", len(entry.Enhanced.IndexingErrors)).
					Msg("Pipeline failed with indexing errors - persisting errors for debugging")
			} else {
				logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed")
			}
		}
		// Always upsert to persist indexing errors even if pipeline failed
		if err := fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
			logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to upsert file")
		} else {
			filesProcessed++
		}
	}

	count, err := fileRepo.Count(ctx, ws.ID)
	if err != nil {
		return err
	}
	if err := workspaceRepo.UpdateFileCount(ctx, ws.ID, count); err != nil {
		return err
	}
	if err := workspaceRepo.UpdateLastIndexed(ctx, ws.ID); err != nil {
		return err
	}

	logger.Info().
		Str("workspace_id", ws.ID.String()).
		Int("files_total", len(entries)).
		Int("files_processed", filesProcessed).
		Int("files_skipped", filesSkipped).
		Int("files_in_index", count).
		Msg("Initial scan complete")

	return nil
}

func handleWatchEvents(
	ctx context.Context,
	watcher service.FileWatcher,  // Using interface instead of concrete type
	indexer service.FileIndexer,  // Using interface instead of concrete type
	processor pipeline.Processor, // Using interface for testability
	fileRepo repository.FileRepository,
	workspaceRepo repository.WorkspaceRepository,
	ws *entity.Workspace,
	logger zerolog.Logger,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			logger.Warn().Err(err).Str("path", ws.Path).Msg("Watcher error")
		case evt, ok := <-watcher.Events():
			if !ok {
				return
			}

			logger.Info().
				Str("workspace_id", ws.ID.String()).
				Str("path", evt.RelativePath).
				Str("event", evt.Type.String()).
				Msg("Watch event received")

			if !ws.Config.AutoIndex {
				continue
			}

			switch evt.Type {
			case entity.FileEventCreated:
				// Always index new files
				entry, err := indexer.ScanFile(ctx, evt.RelativePath)
				if err != nil {
					logger.Warn().Err(err).Str("path", evt.RelativePath).Msg("Scan file failed")
					continue
				}
				stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
					ID:     ws.ID,
					Root:   ws.Path,
					Config: ws.Config,
				})
				logger.Info().
					Str("path", entry.RelativePath).
					Int64("size", entry.FileSize).
					Msg("File scanned")
				if err := processor.Process(stageCtx, entry); err != nil {
					// Pipeline failed - check if there are indexing errors to persist
					if entry.Enhanced != nil && entry.Enhanced.HasIndexingErrors() {
						logger.Warn().
							Err(err).
							Str("path", entry.RelativePath).
							Int("error_count", len(entry.Enhanced.IndexingErrors)).
							Msg("Pipeline failed with indexing errors - persisting errors for debugging")
					} else {
						logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed")
					}
				}
				// Always upsert to persist indexing errors even if pipeline failed
				if err := fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
					logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Upsert failed")
				} else {
					logger.Info().Str("path", entry.RelativePath).Msg("File indexed")
				}
			case entity.FileEventModified:
				// Check if file actually changed before reindexing
				existing, err := fileRepo.GetByPath(ctx, ws.ID, evt.RelativePath)
				if err == nil && existing != nil {
					// Scan file to get current stats
					entry, err := indexer.ScanFile(ctx, evt.RelativePath)
					if err != nil {
						logger.Warn().Err(err).Str("path", evt.RelativePath).Msg("Scan file failed")
						continue
					}
					
					// Compare timestamp (with 1 second tolerance for filesystem precision) and size - only reindex if changed
					timeDiff := existing.LastModified.Sub(entry.LastModified)
					if timeDiff < 0 {
						timeDiff = -timeDiff
					}
					if timeDiff < time.Second && existing.FileSize == entry.FileSize {
						logger.Debug().
							Str("path", evt.RelativePath).
							Msg("File unchanged, skipping reindex")
						continue
					}
					
					logger.Info().
						Str("path", entry.RelativePath).
						Int64("old_size", existing.FileSize).
						Int64("new_size", entry.FileSize).
						Time("old_modified", existing.LastModified).
						Time("new_modified", entry.LastModified).
						Msg("File changed, reindexing")
					
					// Reindex the file
					stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
						ID:     ws.ID,
						Root:   ws.Path,
						Config: ws.Config,
					})
					if err := processor.Process(stageCtx, entry); err != nil {
						logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed")
					}
					if err := fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
						logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Upsert failed")
					}
					logger.Info().Str("path", entry.RelativePath).Msg("File indexed")
				} else {
					// File not in index yet, treat as new
					entry, err := indexer.ScanFile(ctx, evt.RelativePath)
					if err != nil {
						logger.Warn().Err(err).Str("path", evt.RelativePath).Msg("Scan file failed")
						continue
					}
					logger.Info().
						Str("path", entry.RelativePath).
						Msg("File not in index, indexing as new")
					stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
						ID:     ws.ID,
						Root:   ws.Path,
						Config: ws.Config,
					})
					if err := processor.Process(stageCtx, entry); err != nil {
						logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed")
					}
					if err := fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
						logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Upsert failed")
					}
					logger.Info().Str("path", entry.RelativePath).Msg("File indexed")
				}
			case entity.FileEventDeleted:
				if err := fileRepo.Delete(ctx, ws.ID, entity.NewFileID(evt.RelativePath)); err != nil {
					logger.Warn().Err(err).Str("path", evt.RelativePath).Msg("Delete failed")
				} else {
					logger.Info().Str("path", evt.RelativePath).Msg("File deleted from index")
				}
			case entity.FileEventRenamed:
				if evt.OldPath != nil {
					_ = fileRepo.Delete(ctx, ws.ID, entity.NewFileID(*evt.OldPath))
					logger.Info().Str("path", *evt.OldPath).Msg("Old file removed after rename")
				}
				entry, err := indexer.ScanFile(ctx, evt.RelativePath)
				if err != nil {
					logger.Warn().Err(err).Str("path", evt.RelativePath).Msg("Scan file failed after rename")
					continue
				}
				stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
					ID:     ws.ID,
					Root:   ws.Path,
					Config: ws.Config,
				})
				logger.Info().
					Str("path", entry.RelativePath).
					Str("old_path", valueOrEmpty(evt.OldPath)).
					Msg("File renamed")
				if err := processor.Process(stageCtx, entry); err != nil {
					logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed after rename")
				}
				if err := fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
					logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Upsert failed after rename")
				}
				logger.Info().Str("path", entry.RelativePath).Msg("File reindexed after rename")
			}

			count, err := fileRepo.Count(ctx, ws.ID)
			if err == nil {
				_ = workspaceRepo.UpdateFileCount(ctx, ws.ID, count)
			}
		}
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
