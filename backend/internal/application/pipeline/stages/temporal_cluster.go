// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	metadataApp "github.com/dacrypt/cortex/backend/internal/application/metadata"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// TemporalClusterStage processes temporal clustering for automatic project assignment.
// It groups files that were edited together within a time window and propagates
// project assignments from files that have projects to files that don't.
type TemporalClusterStage struct {
	clusterer *metadataApp.TemporalClusterer
	config    metadataApp.TemporalClusterConfig
	logger    zerolog.Logger
}

// NewTemporalClusterStage creates a new temporal clustering stage.
func NewTemporalClusterStage(
	usageRepo repository.UsageRepository,
	metaRepo repository.MetadataRepository,
	config metadataApp.TemporalClusterConfig,
	logger zerolog.Logger,
) *TemporalClusterStage {
	return &TemporalClusterStage{
		clusterer: metadataApp.NewTemporalClusterer(usageRepo, metaRepo, config, logger),
		config:    config,
		logger:    logger.With().Str("component", "temporal_cluster_stage").Logger(),
	}
}

// Name returns the stage name.
func (s *TemporalClusterStage) Name() string {
	return "temporal_cluster"
}

// CanProcess returns true for all files.
// The actual work is done in Finalize() after all files are processed.
func (s *TemporalClusterStage) CanProcess(entry *entity.FileEntry) bool {
	return true
}

// Process is a no-op for individual files.
// Temporal clustering requires analyzing patterns across multiple files,
// so the work is done in Finalize().
func (s *TemporalClusterStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	return nil
}

// Finalize runs temporal clustering after all files are processed.
// It analyzes recent file edit patterns and propagates project assignments.
func (s *TemporalClusterStage) Finalize(ctx context.Context) error {
	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		s.logger.Debug().Msg("No workspace info, skipping temporal clustering")
		return nil
	}

	// Look back at activity from the configured window period
	since := time.Now().Add(-time.Duration(s.config.WindowHours) * time.Hour)

	s.logger.Debug().
		Str("workspace_id", wsInfo.ID.String()).
		Time("since", since).
		Int("window_hours", s.config.WindowHours).
		Msg("Running temporal clustering")

	clusters, err := s.clusterer.FindClusters(ctx, wsInfo.ID, since)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to find temporal clusters")
		return nil // Don't fail the pipeline
	}

	if len(clusters) == 0 {
		s.logger.Debug().Msg("No temporal clusters found")
		return nil
	}

	s.logger.Info().
		Int("clusters", len(clusters)).
		Msg("Found temporal clusters")

	propagated, err := s.clusterer.PropagateProjectsFromClusters(ctx, wsInfo.ID, clusters)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to propagate projects from clusters")
		return nil // Don't fail the pipeline
	}

	if propagated > 0 {
		s.logger.Info().
			Int("propagated", propagated).
			Msg("Propagated projects from temporal clusters")
	}

	return nil
}
