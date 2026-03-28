// Package handlers provides gRPC service implementations.
package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/filesystem"
)

// FileHandler handles file-related gRPC requests.
type FileHandler struct {
	fileRepo      repository.FileRepository
	metaRepo      repository.MetadataRepository
	workspaceRepo repository.WorkspaceRepository
	indexer       service.FileIndexer  // Using interface instead of concrete type
	watcher       service.FileWatcher  // Using interface instead of concrete type
	pipeline      *pipeline.Orchestrator
	publisher     event.Publisher
	logger        zerolog.Logger
	workerCount   int
}

// FileHandlerConfig holds configuration for the file handler.
type FileHandlerConfig struct {
	FileRepo      repository.FileRepository
	MetaRepo      repository.MetadataRepository
	WorkspaceRepo repository.WorkspaceRepository
	Indexer       service.FileIndexer  // Using interface - can be any implementation
	Watcher       service.FileWatcher  // Using interface - can be any implementation
	Pipeline      *pipeline.Orchestrator
	Publisher     event.Publisher
	Logger        zerolog.Logger
	WorkerCount   int
}

// FileHandlerConfigLegacy holds legacy configuration for backward compatibility.
// This allows existing code to pass *filesystem.Scanner and *filesystem.Watcher
// which will be automatically converted to interfaces.
type FileHandlerConfigLegacy struct {
	FileRepo      repository.FileRepository
	MetaRepo      repository.MetadataRepository
	WorkspaceRepo repository.WorkspaceRepository
	Scanner       *filesystem.Scanner  // Legacy: will be converted to service.FileIndexer
	Watcher       *filesystem.Watcher  // Legacy: will be converted to service.FileWatcher
	Pipeline      *pipeline.Orchestrator
	Publisher     event.Publisher
	Logger        zerolog.Logger
	WorkerCount   int
}

// NewFileHandler creates a new file handler.
func NewFileHandler(cfg FileHandlerConfig) *FileHandler {
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 4 // Default to 4 workers
	}
	return &FileHandler{
		fileRepo:      cfg.FileRepo,
		metaRepo:      cfg.MetaRepo,
		workspaceRepo: cfg.WorkspaceRepo,
		indexer:       cfg.Indexer,
		watcher:       cfg.Watcher,
		pipeline:      cfg.Pipeline,
		publisher:     cfg.Publisher,
		logger:        cfg.Logger.With().Str("handler", "file").Logger(),
		workerCount:   workerCount,
	}
}

// NewFileHandlerLegacy creates a new file handler from legacy config.
// This function provides backward compatibility for code that still uses
// concrete types (*filesystem.Scanner, *filesystem.Watcher).
func NewFileHandlerLegacy(cfg FileHandlerConfigLegacy) *FileHandler {
	// Convert concrete types to interfaces
	var indexer service.FileIndexer
	if cfg.Scanner != nil {
		indexer = cfg.Scanner
	}

	var watcher service.FileWatcher
	if cfg.Watcher != nil {
		watcher = cfg.Watcher
	}

	return NewFileHandler(FileHandlerConfig{
		FileRepo:      cfg.FileRepo,
		MetaRepo:      cfg.MetaRepo,
		WorkspaceRepo: cfg.WorkspaceRepo,
		Indexer:       indexer,
		Watcher:       watcher,
		Pipeline:      cfg.Pipeline,
		Publisher:     cfg.Publisher,
		Logger:        cfg.Logger,
		WorkerCount:   cfg.WorkerCount,
	})
}

// ScanWorkspace scans a workspace for files.
func (h *FileHandler) ScanWorkspace(ctx context.Context, workspaceID, path string, forceFullScan bool, progressCh chan<- ScanProgress) error {
	h.logger.Info().
		Str("workspace_id", workspaceID).
		Str("path", path).
		Bool("force_full_scan", forceFullScan).
		Msg("Starting workspace scan")

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// If forceFullScan is true, clear file data (but preserve documents, chunks, embeddings, clusters)
	// This allows incremental clustering to work correctly while reindexing files
	if forceFullScan {
		h.logger.Info().
			Str("workspace_id", workspaceID).
			Str("path", absPath).
			Msg("Force full scan enabled: clearing file data (preserving documents, embeddings, clusters)")

		// Clear only file-related data, preserving documents, chunks, embeddings, and clusters
		// This allows incremental clustering to work correctly
		if err := h.clearFileData(ctx, entity.WorkspaceID(workspaceID), absPath); err != nil {
			h.logger.Error().
				Err(err).
				Str("workspace_id", workspaceID).
				Msg("Failed to clear file data, continuing with scan anyway")
			// Continue with scan even if cleanup fails
		} else {
			h.logger.Info().
				Str("workspace_id", workspaceID).
				Msg("File data cleared successfully (documents, embeddings, clusters preserved)")
		}
	}

	scanProgressCh := h.setupProgressChannel(progressCh)
	defer func() {
		if scanProgressCh != nil {
			close(scanProgressCh)
		}
	}()

	wsConfig := h.getWorkspaceConfig(ctx, workspaceID)
	entries, err := h.scanFiles(ctx, absPath, wsConfig, scanProgressCh)
	if err != nil {
		return err
	}

	h.sendProcessingStartProgress(progressCh, len(entries))

	stageCtx := h.createStageContext(ctx, workspaceID, absPath, wsConfig, forceFullScan)
	processErrors, filesProcessed := h.processFilesConcurrently(ctx, stageCtx, workspaceID, entries, progressCh)

	// Finalize pipeline stages with a non-cancelable context to avoid premature cancellations.
	finalizeCtx := context.Background()
	if wsInfo, ok := contextinfo.GetWorkspaceInfo(stageCtx); ok {
		finalizeCtx = contextinfo.WithWorkspaceInfo(finalizeCtx, wsInfo)
	}
	if err := h.pipeline.Finalize(finalizeCtx); err != nil {
		h.logger.Warn().Err(err).Msg("Pipeline finalization failed")
	}

	h.logProcessingSummary(processErrors, len(entries), filesProcessed)
	h.publishCompletionEvent(ctx, absPath, len(entries))

	h.logger.Info().
		Str("workspace_id", workspaceID).
		Int("files", len(entries)).
		Msg("Workspace scan completed")

	return nil
}

// setupProgressChannel sets up a progress channel adapter if needed.
func (h *FileHandler) setupProgressChannel(progressCh chan<- ScanProgress) chan service.ScanProgress {
	if progressCh == nil {
		return nil
	}

	scanProgressCh := make(chan service.ScanProgress, 16)
	go func() {
		for progress := range scanProgressCh {
			update := ScanProgress{
				Phase:           progress.Phase,
				FilesDiscovered: progress.FilesTotal,
				FilesProcessed:  progress.FilesScanned,
				CurrentFile:     progress.CurrentPath,
			}
			if progress.Error != nil {
				update.Errors = []string{*progress.Error}
			}
			select {
			case progressCh <- update:
			default:
			}
		}
	}()
	return scanProgressCh
}

// getWorkspaceConfig retrieves workspace configuration.
func (h *FileHandler) getWorkspaceConfig(ctx context.Context, workspaceID string) *entity.WorkspaceConfig {
	if workspaceID == "" || h.workspaceRepo == nil {
		return nil
	}

	ws, err := h.workspaceRepo.Get(ctx, entity.WorkspaceID(workspaceID))
	if err != nil || ws == nil {
		return nil
	}
	return &ws.Config
}

// scanFiles performs the actual file scanning.
func (h *FileHandler) scanFiles(ctx context.Context, absPath string, wsConfig *entity.WorkspaceConfig, scanProgressCh chan service.ScanProgress) ([]*entity.FileEntry, error) {
	// If indexer is not set, create a new one for this scan
	// This maintains backward compatibility
	if h.indexer == nil {
		indexer := filesystem.NewScanner(absPath, wsConfig)
		return indexer.Scan(ctx, scanProgressCh)
	}
	// Use the configured indexer (could be a mock, remote indexer, etc.)
	return h.indexer.Scan(ctx, scanProgressCh)
}

// sendProcessingStartProgress sends initial processing progress.
func (h *FileHandler) sendProcessingStartProgress(progressCh chan<- ScanProgress, totalFiles int) {
	if progressCh == nil {
		return
	}

	select {
	case progressCh <- ScanProgress{
		Phase:           "processing",
		FilesDiscovered: totalFiles,
		FilesProcessed:  0,
	}:
	default:
	}
}

// createStageContext creates a context with workspace info.
func (h *FileHandler) createStageContext(ctx context.Context, workspaceID, absPath string, wsConfig *entity.WorkspaceConfig, forceFullScan bool) context.Context {
	var config entity.WorkspaceConfig
	if wsConfig != nil {
		config = *wsConfig
	}

	return contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
		ID:            entity.WorkspaceID(workspaceID),
		Root:          absPath,
		Config:        config,
		ForceFullScan: forceFullScan,
	})
}

// processFilesConcurrently processes files using a worker pool.
func (h *FileHandler) processFilesConcurrently(
	ctx context.Context,
	stageCtx context.Context,
	workspaceID string,
	entries []*entity.FileEntry,
	progressCh chan<- ScanProgress,
) ([]error, int64) {
	workers := h.calculateWorkerCount(len(entries))
	workspaceIDValue := entity.WorkspaceID(workspaceID)
	var filesProcessed int64

	h.logger.Info().
		Int("total_files", len(entries)).
		Int("workers", workers).
		Msg("Starting concurrent file processing with worker pool")

	processEntry := h.createProcessEntryFunc(stageCtx, workspaceIDValue, &filesProcessed, len(entries), progressCh)
	processErrors := h.runWorkerPool(ctx, entries, workers, processEntry)

	return processErrors, atomic.LoadInt64(&filesProcessed)
}

// calculateWorkerCount calculates the optimal number of workers.
func (h *FileHandler) calculateWorkerCount(totalFiles int) int {
	workers := h.workerCount
	if workers <= 0 {
		workers = 4
	}
	if workers > totalFiles {
		workers = totalFiles
	}
	return workers
}

// createProcessEntryFunc creates a function to process a single file entry.
func (h *FileHandler) createProcessEntryFunc(
	stageCtx context.Context,
	workspaceID entity.WorkspaceID,
	filesProcessed *int64,
	totalFiles int,
	progressCh chan<- ScanProgress,
) func(*entity.FileEntry) error {
	return func(entry *entity.FileEntry) error {
		// Create an independent context for processing that won't be canceled
		// when the parent gRPC context is canceled. This allows long-running
		// operations like embedding generation to complete.
		processCtx := context.Background()
		
		// Copy workspace info from stageCtx to the new context
		if wsInfo, ok := contextinfo.GetWorkspaceInfo(stageCtx); ok {
			processCtx = contextinfo.WithWorkspaceInfo(processCtx, wsInfo)
		}

		// TDD Fix: Create metadata BEFORE processing to prevent NotFound warnings
		// This ensures that any gRPC calls to GetMetadata during processing will succeed
		if h.metaRepo != nil {
			_, err := h.metaRepo.GetOrCreate(processCtx, workspaceID, entry.RelativePath, entry.Extension)
			if err != nil {
				h.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to create metadata before processing")
				// Continue anyway - metadata creation is best effort, but this should rarely fail
			} else {
				h.logger.Debug().Str("path", entry.RelativePath).Msg("Created metadata before pipeline processing")
			}
		}

		// Process file through pipeline with independent context
		if err := h.pipeline.Process(processCtx, entry); err != nil {
			h.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to process file")
			return err
		}

		// Ensure metadata still exists after processing (in case it was deleted or needs updates)
		if h.metaRepo != nil {
			_, err := h.metaRepo.GetOrCreate(context.Background(), workspaceID, entry.RelativePath, entry.Extension)
			if err != nil {
				h.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to ensure metadata exists after processing")
				// Continue anyway - metadata creation is best effort
			}
		}

		if err := h.fileRepo.Upsert(context.Background(), workspaceID, entry); err != nil {
			h.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to store file")
			return err
		}

		processed := atomic.AddInt64(filesProcessed, 1)
		h.sendFileProgress(progressCh, totalFiles, int(processed), entry.RelativePath)

		return nil
	}
}

// sendFileProgress sends progress update for a single file.
func (h *FileHandler) sendFileProgress(progressCh chan<- ScanProgress, totalFiles, processed int, currentFile string) {
	if progressCh == nil {
		return
	}

	select {
	case progressCh <- ScanProgress{
		Phase:           "processing",
		FilesDiscovered: totalFiles,
		FilesProcessed:  processed,
		CurrentFile:     currentFile,
	}:
	default:
	}
}

// runWorkerPool runs the worker pool to process files concurrently.
func (h *FileHandler) runWorkerPool(
	ctx context.Context,
	entries []*entity.FileEntry,
	workers int,
	processEntry func(*entity.FileEntry) error,
) []error {
	entryChan := make(chan *entity.FileEntry, len(entries))
	resultChan := make(chan error, len(entries))

	h.startWorkers(ctx, workers, entryChan, resultChan, processEntry)
	h.sendEntriesToWorkers(ctx, entries, entryChan)
	return h.collectResults(ctx, resultChan)
}

// startWorkers starts worker goroutines.
func (h *FileHandler) startWorkers(
	ctx context.Context,
	workers int,
	entryChan <-chan *entity.FileEntry,
	resultChan chan<- error,
	processEntry func(*entity.FileEntry) error,
) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range entryChan {
				// Don't check ctx.Err() here - allow processing to continue
				// even if the parent context is canceled. The processEntry
				// function uses an independent context for actual processing.
				resultChan <- processEntry(entry)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()
}

// sendEntriesToWorkers sends entries to the worker channel.
func (h *FileHandler) sendEntriesToWorkers(ctx context.Context, entries []*entity.FileEntry, entryChan chan<- *entity.FileEntry) {
	go func() {
		defer close(entryChan)
		for _, entry := range entries {
			// Send all entries even if context is canceled
			// Workers will process them with independent contexts
			select {
			case entryChan <- entry:
			case <-ctx.Done():
				// If context is canceled, still try to send remaining entries
				// but don't block indefinitely
				return
			}
		}
	}()
}

// collectResults collects results from the result channel.
func (h *FileHandler) collectResults(ctx context.Context, resultChan <-chan error) []error {
	var processErrors []error
	var processErrorsMu sync.Mutex

	// Collect all results, even if parent context is canceled
	// This ensures we get complete error reporting
	for err := range resultChan {
		if err != nil {
			processErrorsMu.Lock()
			processErrors = append(processErrors, err)
			processErrorsMu.Unlock()
		}
	}

	return processErrors
}

// logProcessingSummary logs the summary of file processing.
func (h *FileHandler) logProcessingSummary(processErrors []error, totalFiles int, filesProcessed int64) {
	if len(processErrors) > 0 {
		h.logger.Warn().
			Int("total_errors", len(processErrors)).
			Int("total_files", totalFiles).
			Int64("files_processed", filesProcessed).
			Msg("Some files failed to process during scan")
	} else {
		h.logger.Info().
			Int("total_files", totalFiles).
			Int64("files_processed", filesProcessed).
			Msg("Concurrent file processing completed successfully")
	}
}

// publishCompletionEvent publishes the scan completion event.
func (h *FileHandler) publishCompletionEvent(ctx context.Context, absPath string, totalFiles int) {
	if h.publisher == nil {
		return
	}

	_ = h.publisher.Publish(ctx, &event.Event{
		Type: event.EventScanCompleted,
		Data: event.ScanEventData{
			WorkspacePath: absPath,
			FilesScanned:  totalFiles,
			FilesTotal:    totalFiles,
			Percentage:    100,
		},
	})
}

// GetFile retrieves a file by path.
// Returns nil, nil if file is not found (not an error condition).
// This allows callers to distinguish between "file not found" and actual errors.
func (h *FileHandler) GetFile(ctx context.Context, workspaceID, path string) (*entity.FileEntry, error) {
	// Log the request to help diagnose path encoding issues
	h.logger.Debug().
		Str("workspace_id", workspaceID).
		Str("path", path).
		Int("path_length", len(path)).
		Msg("GetFile: searching for file")
	
	entry, err := h.fileRepo.GetByPath(ctx, entity.WorkspaceID(workspaceID), path)
	if err != nil {
		// Log the error for debugging - this helps identify path normalization issues
		h.logger.Debug().
			Err(err).
			Str("workspace_id", workspaceID).
			Str("path", path).
			Msg("GetFile: repository returned error (treating as not found)")
		// If it's a "not found" type error, return nil, nil instead of error
		// This is common when RAG finds documents that aren't in the file index
		// (e.g., documents exist in document/chunk DB but file was deleted/moved)
		return nil, nil
	}
	if entry == nil {
		// Log when file is not found - this helps identify cases where documents
		// exist but files don't (data inconsistency or path encoding mismatch)
		h.logger.Debug().
			Str("workspace_id", workspaceID).
			Str("path", path).
			Msg("GetFile: file not found in index (document may exist but file not indexed, or path encoding mismatch)")
	} else {
		h.logger.Debug().
			Str("workspace_id", workspaceID).
			Str("requested_path", path).
			Str("found_path", entry.RelativePath).
			Msg("GetFile: file found successfully")
	}
	return entry, nil
}

// ProcessFile reindexes a single file through the pipeline.
func (h *FileHandler) ProcessFile(ctx context.Context, workspaceID, relativePath string) (*entity.FileEntry, error) {
	if h.workspaceRepo == nil || h.fileRepo == nil || h.pipeline == nil {
		return nil, nil
	}

	ws, err := h.workspaceRepo.Get(ctx, entity.WorkspaceID(workspaceID))
	if err != nil || ws == nil {
		return nil, err
	}

	// Use configured indexer if available, otherwise create a new one
	var indexer service.FileIndexer = h.indexer
	if indexer == nil {
		indexer = filesystem.NewScanner(ws.Path, &ws.Config)
	}

	entry, err := indexer.ScanFile(ctx, relativePath)
	if err != nil {
		return nil, err
	}

	stageCtx := contextinfo.WithWorkspaceInfo(ctx, contextinfo.WorkspaceInfo{
		ID:     ws.ID,
		Root:   ws.Path,
		Config: ws.Config,
	})
	if err := h.pipeline.Process(stageCtx, entry); err != nil {
		h.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Pipeline failed")
	}
	if err := h.fileRepo.Upsert(ctx, ws.ID, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// ListFiles lists files in a workspace.
func (h *FileHandler) ListFiles(ctx context.Context, workspaceID string, opts ListFilesOptions) ([]*entity.FileEntry, int, error) {
	repoOpts := repository.DefaultFileListOptions()
	repoOpts.Offset = opts.Offset
	if opts.Limit > 0 {
		repoOpts.Limit = opts.Limit
	}

	workspaceIDValue := entity.WorkspaceID(workspaceID)
	entries, err := h.fileRepo.List(ctx, workspaceIDValue, repoOpts)
	if err != nil {
		return nil, 0, err
	}

	count, err := h.fileRepo.Count(ctx, workspaceIDValue)
	if err != nil {
		return nil, 0, err
	}

	return entries, count, nil
}

// SearchFiles searches for files.
func (h *FileHandler) SearchFiles(ctx context.Context, workspaceID, query string) ([]*entity.FileEntry, error) {
	repoOpts := repository.DefaultFileListOptions()
	return h.fileRepo.Search(ctx, entity.WorkspaceID(workspaceID), query, repoOpts)
}

// ListByType lists files by extension/type.
func (h *FileHandler) ListByType(ctx context.Context, workspaceID, fileType string) ([]*entity.FileEntry, error) {
	repoOpts := repository.DefaultFileListOptions()
	return h.fileRepo.ListByExtension(ctx, entity.WorkspaceID(workspaceID), fileType, repoOpts)
}

// ListByFolder lists files in a folder.
func (h *FileHandler) ListByFolder(ctx context.Context, workspaceID, folder string) ([]*entity.FileEntry, error) {
	repoOpts := repository.DefaultFileListOptions()
	return h.fileRepo.ListByFolder(ctx, entity.WorkspaceID(workspaceID), folder, true, repoOpts)
}

// StartWatching starts watching a workspace for changes.
func (h *FileHandler) StartWatching(ctx context.Context, workspaceID, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	wsConfig := (*entity.WorkspaceConfig)(nil)
	if workspaceID != "" && h.workspaceRepo != nil {
		if ws, err := h.workspaceRepo.Get(ctx, entity.WorkspaceID(workspaceID)); err == nil && ws != nil {
			wsConfig = &ws.Config
		}
	}

	// If watcher is not configured, create a new one
	if h.watcher == nil {
		watcher, err := filesystem.NewWatcher(absPath, wsConfig)
		if err != nil {
			return err
		}
		h.watcher = watcher
	}

	return h.watcher.Start()
}

// StopWatching stops watching a workspace.
func (h *FileHandler) StopWatching() error {
	if h.watcher == nil {
		return nil
	}
	return h.watcher.Stop()
}

// ScanProgress represents scan progress.
type ScanProgress struct {
	Phase           string
	FilesDiscovered int
	FilesProcessed  int
	CurrentFile     string
	Errors          []string
}

// ListFilesOptions contains options for listing files.
type ListFilesOptions struct {
	Offset int
	Limit  int
}

// clearFileData clears only file-related data, preserving documents, chunks, embeddings, and clusters.
func (h *FileHandler) clearFileData(ctx context.Context, workspaceID entity.WorkspaceID, workspaceRoot string) error {
	if h.workspaceRepo == nil {
		return fmt.Errorf("workspace repository not available")
	}

	return h.workspaceRepo.ClearFileData(ctx, workspaceID, workspaceRoot)
}

// clearWorkspaceData clears all data for a workspace (database and .cortex files).
func (h *FileHandler) clearWorkspaceData(ctx context.Context, workspaceID entity.WorkspaceID, workspaceRoot string) error {
	if h.workspaceRepo == nil {
		return fmt.Errorf("workspace repository not available")
	}

	return h.workspaceRepo.ClearWorkspaceData(ctx, workspaceID, workspaceRoot)
}
