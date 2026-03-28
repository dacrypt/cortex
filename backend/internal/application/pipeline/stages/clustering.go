// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/clustering"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ClusteringStageConfig contains configuration for the clustering stage.
type ClusteringStageConfig struct {
	Enabled            bool
	MinDocumentsToRun  int           // Minimum documents before running clustering
	ClusteringInterval time.Duration // Minimum time between clustering runs
	RunOnFinalize      bool          // Run clustering at pipeline finalization
}

// DefaultClusteringStageConfig returns the default configuration.
func DefaultClusteringStageConfig() ClusteringStageConfig {
	return ClusteringStageConfig{
		Enabled:            true,
		MinDocumentsToRun:  10,
		ClusteringInterval: 5 * time.Minute,
		RunOnFinalize:      true,
	}
}

// ClusteringStage runs document clustering after indexing.
// This stage is designed to run during Finalize() rather than per-file.
type ClusteringStage struct {
	clusteringService *clustering.Service
	config            ClusteringStageConfig
	logger            zerolog.Logger

	// Track processed documents for clustering
	mu            sync.Mutex
	processedDocs map[entity.WorkspaceID]int
	lastRun       map[entity.WorkspaceID]time.Time
}

// NewClusteringStage creates a new clustering stage.
func NewClusteringStage(
	clusteringService *clustering.Service,
	config ClusteringStageConfig,
	logger zerolog.Logger,
) *ClusteringStage {
	return &ClusteringStage{
		clusteringService: clusteringService,
		config:            config,
		logger:            logger.With().Str("component", "clustering_stage").Logger(),
		processedDocs:     make(map[entity.WorkspaceID]int),
		lastRun:           make(map[entity.WorkspaceID]time.Time),
	}
}

// Name returns the stage name.
func (s *ClusteringStage) Name() string {
	return "clustering"
}

// CanProcess returns true for all documents (we track them for clustering).
func (s *ClusteringStage) CanProcess(entry *entity.FileEntry) bool {
	if !s.config.Enabled || s.clusteringService == nil {
		return false
	}
	// Only count documents that have been indexed
	return entry.Enhanced != nil && entry.Enhanced.IndexedState.Document
}

// Process tracks the document for later clustering.
// The actual clustering happens in Finalize().
func (s *ClusteringStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return nil
	}

	s.mu.Lock()
	s.processedDocs[wsInfo.ID]++
	s.mu.Unlock()

	return nil
}

// Finalize runs clustering after all documents are processed.
func (s *ClusteringStage) Finalize(ctx context.Context) error {
	if !s.config.Enabled || !s.config.RunOnFinalize {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		s.logger.Debug().Msg("No workspace info, skipping clustering")
		return nil
	}

	s.mu.Lock()
	docCount := s.processedDocs[wsInfo.ID]
	lastRun := s.lastRun[wsInfo.ID]
	s.mu.Unlock()

	// Check if we have enough documents
	if docCount < s.config.MinDocumentsToRun {
		s.logger.Debug().
			Int("documents", docCount).
			Int("minimum", s.config.MinDocumentsToRun).
			Msg("Not enough documents for clustering")
		return nil
	}

	// Check if enough time has passed since last run
	if time.Since(lastRun) < s.config.ClusteringInterval {
		s.logger.Debug().
			Dur("since_last", time.Since(lastRun)).
			Dur("interval", s.config.ClusteringInterval).
			Msg("Clustering interval not reached")
		return nil
	}

	s.logger.Info().
		Str("workspace_id", wsInfo.ID.String()).
		Int("documents", docCount).
		Msg("Running document clustering")

	// Run clustering
		result, err := s.clusteringService.RunClustering(ctx, wsInfo.ID, false)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Clustering failed")
		return nil // Don't fail the pipeline
	}

	// Update last run time
	s.mu.Lock()
	s.lastRun[wsInfo.ID] = time.Now()
	s.processedDocs[wsInfo.ID] = 0 // Reset counter
	s.mu.Unlock()

	s.logger.Info().
		Int("clusters", len(result.Clusters)).
		Int("projects_created", result.ProjectsCreated).
		Int("assignments", result.AssignmentsMade).
		Dur("duration", result.Duration).
		Msg("Clustering completed")

	return nil
}

// Reset clears the stage state (for new scans).
func (s *ClusteringStage) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processedDocs = make(map[entity.WorkspaceID]int)
}

// ForceRunClustering forces a clustering run regardless of thresholds.
func (s *ClusteringStage) ForceRunClustering(ctx context.Context, workspaceID entity.WorkspaceID) (*clustering.ClusteringResult, error) {
	if s.clusteringService == nil {
		return nil, nil
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Msg("Force running document clustering")

	result, err := s.clusteringService.RunClustering(ctx, workspaceID, true)
	if err != nil {
		return nil, err
	}

	// Update last run time
	s.mu.Lock()
	s.lastRun[workspaceID] = time.Now()
	s.mu.Unlock()

	return result, nil
}

// GetClusteringStats returns clustering statistics for a workspace.
func (s *ClusteringStage) GetClusteringStats(ctx context.Context, workspaceID entity.WorkspaceID) (*ClusteringStats, error) {
	if s.clusteringService == nil {
		return nil, nil
	}

	stats, err := s.clusteringService.GetClusterStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	lastRun := s.lastRun[workspaceID]
	pendingDocs := s.processedDocs[workspaceID]
	s.mu.Unlock()

	return &ClusteringStats{
		TotalClusters:     stats.TotalClusters,
		ActiveClusters:    stats.ActiveClusters,
		TotalMemberships:  stats.TotalMemberships,
		TotalEdges:        stats.TotalEdges,
		AvgClusterSize:    stats.AvgClusterSize,
		IsolatedDocuments: stats.IsolatedDocuments,
		LastRunAt:         lastRun,
		PendingDocuments:  pendingDocs,
	}, nil
}

// ClusteringStats contains clustering statistics.
type ClusteringStats struct {
	TotalClusters     int
	ActiveClusters    int
	TotalMemberships  int
	TotalEdges        int
	AvgClusterSize    float64
	IsolatedDocuments int
	LastRunAt         time.Time
	PendingDocuments  int
}
