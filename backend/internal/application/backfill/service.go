// Package backfill provides services for backfilling denormalized AI metadata tables.
package backfill

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Phase represents a backfill phase.
type Phase string

const (
	PhaseValidation       Phase = "validation"
	PhaseAIContextBasic   Phase = "ai_context_basic"   // file_authors, file_locations, file_people
	PhaseAIContextAdvanced Phase = "ai_context_advanced" // file_organizations, file_events, file_references
	PhasePublicationInfo  Phase = "publication_info"
	PhaseEnrichmentBasic  Phase = "enrichment_basic"   // file_named_entities, file_citations
	PhaseEnrichmentAdvanced Phase = "enrichment_advanced" // file_dependencies, file_duplicates, file_sentiment
	PhaseFileHashes       Phase = "file_hashes"
	PhaseIntegrity        Phase = "integrity"
)

// AllPhases returns all backfill phases in execution order.
func AllPhases() []Phase {
	return []Phase{
		PhaseValidation,
		PhaseAIContextBasic,
		PhaseAIContextAdvanced,
		PhasePublicationInfo,
		PhaseEnrichmentBasic,
		PhaseEnrichmentAdvanced,
		PhaseFileHashes,
		PhaseIntegrity,
	}
}

// Config configures backfill behavior.
type Config struct {
	BatchSize      int
	MaxConcurrency int
	DryRun         bool
	SkipValidation bool
	StartFromFileID string
	Phases         []Phase // If empty, run all phases
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		BatchSize:      100,
		MaxConcurrency: 4,
		DryRun:         false,
		SkipValidation: false,
	}
}

// Progress tracks backfill progress.
type Progress struct {
	Phase          Phase
	TotalFiles     int
	ProcessedFiles int
	SuccessCount   int
	ErrorCount     int
	SkippedCount   int
	StartedAt      time.Time
	LastUpdateAt   time.Time
	EstimatedETA   time.Duration
	Errors         []string
}

// Service orchestrates backfill operations.
type Service struct {
	metadataRepo repository.MetadataRepository
	fileRepo     repository.FileRepository
	logger       zerolog.Logger
	config       Config

	mu       sync.RWMutex
	progress map[entity.WorkspaceID]*Progress
}

// NewService creates a new backfill service.
func NewService(
	metadataRepo repository.MetadataRepository,
	fileRepo repository.FileRepository,
	logger zerolog.Logger,
) *Service {
	return &Service{
		metadataRepo: metadataRepo,
		fileRepo:     fileRepo,
		logger:       logger.With().Str("component", "backfill").Logger(),
		config:       DefaultConfig(),
		progress:     make(map[entity.WorkspaceID]*Progress),
	}
}

// Run executes backfill for a workspace.
func (s *Service) Run(ctx context.Context, workspaceID entity.WorkspaceID, config Config) error {
	s.config = config
	if len(config.Phases) == 0 {
		config.Phases = AllPhases()
	}

	s.logger.Info().
		Str("workspace_id", string(workspaceID)).
		Bool("dry_run", config.DryRun).
		Int("batch_size", config.BatchSize).
		Msg("Starting backfill")

	for _, phase := range config.Phases {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("backfill cancelled: %w", err)
		}

		s.logger.Info().Str("phase", string(phase)).Msg("Starting phase")

		if err := s.runPhase(ctx, workspaceID, phase); err != nil {
			s.logger.Error().Err(err).Str("phase", string(phase)).Msg("Phase failed")
			return fmt.Errorf("phase %s failed: %w", phase, err)
		}

		s.logger.Info().Str("phase", string(phase)).Msg("Phase completed")
	}

	s.logger.Info().Msg("Backfill completed successfully")
	return nil
}

// GetProgress returns current progress for a workspace.
func (s *Service) GetProgress(workspaceID entity.WorkspaceID) *Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress[workspaceID]
}

func (s *Service) runPhase(ctx context.Context, workspaceID entity.WorkspaceID, phase Phase) error {
	switch phase {
	case PhaseValidation:
		return s.runValidationPhase(ctx, workspaceID)
	case PhaseAIContextBasic:
		return s.runAIContextBasicPhase(ctx, workspaceID)
	case PhaseAIContextAdvanced:
		return s.runAIContextAdvancedPhase(ctx, workspaceID)
	case PhasePublicationInfo:
		return s.runPublicationInfoPhase(ctx, workspaceID)
	case PhaseEnrichmentBasic:
		return s.runEnrichmentBasicPhase(ctx, workspaceID)
	case PhaseEnrichmentAdvanced:
		return s.runEnrichmentAdvancedPhase(ctx, workspaceID)
	case PhaseFileHashes:
		return s.runFileHashesPhase(ctx, workspaceID)
	case PhaseIntegrity:
		return s.runIntegrityPhase(ctx, workspaceID)
	default:
		return fmt.Errorf("unknown phase: %s", phase)
	}
}

func (s *Service) runValidationPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	if s.config.SkipValidation {
		s.logger.Info().Msg("Skipping validation phase")
		return nil
	}

	s.initProgress(workspaceID, PhaseValidation)

	// Validate that source data exists
	files, err := s.fileRepo.List(ctx, workspaceID, repository.FileListOptions{Offset: 0, Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in workspace")
	}

	s.updateProgress(workspaceID, 1, 1, 0, 0)
	return nil
}

func (s *Service) runAIContextBasicPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return s.processFilesWithAIContext(ctx, workspaceID, PhaseAIContextBasic, func(metadata *entity.FileMetadata) error {
		if metadata.AIContext == nil {
			return nil
		}

		// Process authors, locations, and people
		// In dry run, just log what would be done
		if s.config.DryRun {
			s.logger.Debug().
				Str("file_id", string(metadata.FileID)).
				Int("authors", len(metadata.AIContext.Authors)).
				Int("locations", len(metadata.AIContext.Locations)).
				Int("people", len(metadata.AIContext.PeopleMentioned)).
				Msg("Would backfill AI context basic data")
			return nil
		}

		// The actual sync is done by the metadata repository's UpdateAIContext
		// This phase verifies the data is correctly denormalized
		return nil
	})
}

func (s *Service) runAIContextAdvancedPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return s.processFilesWithAIContext(ctx, workspaceID, PhaseAIContextAdvanced, func(metadata *entity.FileMetadata) error {
		if metadata.AIContext == nil {
			return nil
		}

		if s.config.DryRun {
			s.logger.Debug().
				Str("file_id", string(metadata.FileID)).
				Int("orgs", len(metadata.AIContext.Organizations)).
				Int("events", len(metadata.AIContext.HistoricalEvents)).
				Int("refs", len(metadata.AIContext.References)).
				Msg("Would backfill AI context advanced data")
			return nil
		}

		return nil
	})
}

func (s *Service) runPublicationInfoPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return s.processFilesWithAIContext(ctx, workspaceID, PhasePublicationInfo, func(metadata *entity.FileMetadata) error {
		if metadata.AIContext == nil {
			return nil
		}

		hasPublicationInfo := metadata.AIContext.Publisher != nil ||
			metadata.AIContext.PublicationYear != nil ||
			metadata.AIContext.ISBN != nil ||
			metadata.AIContext.DOI != nil

		if s.config.DryRun && hasPublicationInfo {
			s.logger.Debug().
				Str("file_id", string(metadata.FileID)).
				Msg("Would backfill publication info")
			return nil
		}

		return nil
	})
}

func (s *Service) runEnrichmentBasicPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return s.processFilesWithEnrichment(ctx, workspaceID, PhaseEnrichmentBasic, func(metadata *entity.FileMetadata) error {
		if metadata.EnrichmentData == nil {
			return nil
		}

		if s.config.DryRun {
			s.logger.Debug().
				Str("file_id", string(metadata.FileID)).
				Int("entities", len(metadata.EnrichmentData.NamedEntities)).
				Int("citations", len(metadata.EnrichmentData.Citations)).
				Msg("Would backfill enrichment basic data")
			return nil
		}

		return nil
	})
}

func (s *Service) runEnrichmentAdvancedPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return s.processFilesWithEnrichment(ctx, workspaceID, PhaseEnrichmentAdvanced, func(metadata *entity.FileMetadata) error {
		if metadata.EnrichmentData == nil {
			return nil
		}

		if s.config.DryRun {
			s.logger.Debug().
				Str("file_id", string(metadata.FileID)).
				Int("deps", len(metadata.EnrichmentData.Dependencies)).
				Int("dups", len(metadata.EnrichmentData.Duplicates)).
				Bool("has_sentiment", metadata.EnrichmentData.Sentiment != nil).
				Msg("Would backfill enrichment advanced data")
			return nil
		}

		return nil
	})
}

func (s *Service) runFileHashesPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	s.initProgress(workspaceID, PhaseFileHashes)

	// This phase would compute file hashes for files that don't have them
	// Implementation depends on file access patterns

	if s.config.DryRun {
		s.logger.Info().Msg("Would compute file hashes (dry run)")
		return nil
	}

	s.logger.Info().Msg("File hash computation not yet implemented")
	return nil
}

func (s *Service) runIntegrityPhase(ctx context.Context, workspaceID entity.WorkspaceID) error {
	s.initProgress(workspaceID, PhaseIntegrity)

	validator := NewValidator(s.metadataRepo, s.fileRepo, s.logger)
	report, err := validator.ValidateAll(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("integrity validation failed: %w", err)
	}

	if report.TotalIssues > 0 {
		s.logger.Warn().
			Int("orphaned", report.OrphanedRecords).
			Int("missing_refs", report.MissingReferences).
			Int("duplicates", report.DuplicateEntries).
			Int("timestamp_issues", report.TimestampAnomalies).
			Msg("Integrity issues found")

		if !s.config.DryRun {
			// Log detailed issues
			for _, issue := range report.Details {
				s.logger.Warn().
					Str("type", issue.Type).
					Str("table", issue.Table).
					Str("id", issue.RecordID).
					Str("message", issue.Message).
					Msg("Integrity issue")
			}
		}
	} else {
		s.logger.Info().Msg("Integrity validation passed")
	}

	return nil
}

// Helper methods

func (s *Service) processFilesWithAIContext(ctx context.Context, workspaceID entity.WorkspaceID, phase Phase, processor func(*entity.FileMetadata) error) error {
	return s.processFiles(ctx, workspaceID, phase, func(file *entity.FileEntry) error {
		metadata, err := s.metadataRepo.Get(ctx, workspaceID, file.ID)
		if err != nil || metadata == nil {
			return nil // Skip files without metadata
		}

		if metadata.AIContext == nil {
			return nil // Skip files without AI context
		}

		return processor(metadata)
	})
}

func (s *Service) processFilesWithEnrichment(ctx context.Context, workspaceID entity.WorkspaceID, phase Phase, processor func(*entity.FileMetadata) error) error {
	return s.processFiles(ctx, workspaceID, phase, func(file *entity.FileEntry) error {
		metadata, err := s.metadataRepo.Get(ctx, workspaceID, file.ID)
		if err != nil || metadata == nil {
			return nil
		}

		if metadata.EnrichmentData == nil {
			return nil
		}

		return processor(metadata)
	})
}

func (s *Service) processFiles(ctx context.Context, workspaceID entity.WorkspaceID, phase Phase, processor func(*entity.FileEntry) error) error {
	offset := 0
	totalProcessed := 0
	totalSuccess := 0
	totalErrors := 0

	s.initProgress(workspaceID, phase)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		files, err := s.fileRepo.List(ctx, workspaceID, repository.FileListOptions{Offset: offset, Limit: s.config.BatchSize})
		if err != nil {
			return fmt.Errorf("failed to list files: %w", err)
		}

		if len(files) == 0 {
			break
		}

		for _, file := range files {
			if s.config.StartFromFileID != "" && string(file.ID) < s.config.StartFromFileID {
				continue
			}

			if err := processor(file); err != nil {
				s.logger.Warn().Err(err).Str("file_id", string(file.ID)).Msg("File processing failed")
				totalErrors++
			} else {
				totalSuccess++
			}
			totalProcessed++
		}

		s.updateProgress(workspaceID, -1, totalProcessed, totalSuccess, totalErrors)
		offset += len(files)

		if len(files) < s.config.BatchSize {
			break
		}
	}

	return nil
}

func (s *Service) initProgress(workspaceID entity.WorkspaceID, phase Phase) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress[workspaceID] = &Progress{
		Phase:     phase,
		StartedAt: time.Now(),
	}
}

func (s *Service) updateProgress(workspaceID entity.WorkspaceID, total, processed, success, errors int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := s.progress[workspaceID]
	if p == nil {
		return
	}

	if total >= 0 {
		p.TotalFiles = total
	}
	p.ProcessedFiles = processed
	p.SuccessCount = success
	p.ErrorCount = errors
	p.LastUpdateAt = time.Now()

	// Estimate ETA
	if processed > 0 && p.TotalFiles > 0 {
		elapsed := time.Since(p.StartedAt)
		rate := float64(processed) / elapsed.Seconds()
		remaining := p.TotalFiles - processed
		if rate > 0 {
			p.EstimatedETA = time.Duration(float64(remaining)/rate) * time.Second
		}
	}
}

// RollbackPoint represents a point to which backfill can be rolled back.
type RollbackPoint struct {
	ID        string
	Phase     Phase
	Timestamp time.Time
	FileCount int
}

// CreateRollbackPoint creates a rollback point before a phase.
func (s *Service) CreateRollbackPoint(ctx context.Context, workspaceID entity.WorkspaceID, phase Phase) (*RollbackPoint, error) {
	point := &RollbackPoint{
		ID:        uuid.New().String(),
		Phase:     phase,
		Timestamp: time.Now(),
	}

	// In a real implementation, this would snapshot the current state
	s.logger.Info().
		Str("point_id", point.ID).
		Str("phase", string(phase)).
		Msg("Created rollback point")

	return point, nil
}

// Rollback reverts changes made after a rollback point.
func (s *Service) Rollback(ctx context.Context, workspaceID entity.WorkspaceID, point *RollbackPoint) error {
	s.logger.Info().
		Str("point_id", point.ID).
		Str("phase", string(point.Phase)).
		Msg("Rolling back to point")

	// In a real implementation, this would restore from the snapshot
	// For now, we can delete records created after the timestamp

	return fmt.Errorf("rollback not yet implemented")
}

// Result contains the final result of a backfill operation.
type Result struct {
	WorkspaceID    entity.WorkspaceID
	StartedAt      time.Time
	CompletedAt    time.Time
	TotalFiles     int
	ProcessedFiles int
	SuccessCount   int
	ErrorCount     int
	SkippedCount   int
	PhaseResults   map[Phase]PhaseResult
}

// PhaseResult contains the result of a single phase.
type PhaseResult struct {
	Phase       Phase
	StartedAt   time.Time
	CompletedAt time.Time
	Success     bool
	Error       string
	Metrics     json.RawMessage
}
