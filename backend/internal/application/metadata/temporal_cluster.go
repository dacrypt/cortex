// Package metadata provides metadata services.
package metadata

import (
	"context"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// TemporalClusterConfig configures temporal clustering behavior.
type TemporalClusterConfig struct {
	// WindowHours is the time window for clustering edits together (default: 6h)
	WindowHours int
	// MinClusterSize is the minimum files in a cluster to be considered (default: 2)
	MinClusterSize int
	// DominanceThreshold is the threshold for auto-assigning projects (default: 0.6)
	DominanceThreshold float64
	// SuggestionThreshold is the threshold for suggesting projects (default: 0.3)
	SuggestionThreshold float64
}

// DefaultTemporalClusterConfig returns sensible defaults.
func DefaultTemporalClusterConfig() TemporalClusterConfig {
	return TemporalClusterConfig{
		WindowHours:         6,
		MinClusterSize:      2,
		DominanceThreshold:  0.6,
		SuggestionThreshold: 0.3,
	}
}

// TemporalCluster represents a group of files edited together within a time window.
type TemporalCluster struct {
	FileIDs         []entity.FileID
	RelativePaths   []string
	StartTime       time.Time
	EndTime         time.Time
	DominantProject *string
	ProjectCounts   map[string]int
	Confidence      float64
}

// TemporalClusterer groups files by edit time proximity.
type TemporalClusterer struct {
	usageRepo  repository.UsageRepository
	metaRepo   repository.MetadataRepository
	config     TemporalClusterConfig
	logger     zerolog.Logger
}

// NewTemporalClusterer creates a new temporal clusterer.
func NewTemporalClusterer(
	usageRepo repository.UsageRepository,
	metaRepo repository.MetadataRepository,
	config TemporalClusterConfig,
	logger zerolog.Logger,
) *TemporalClusterer {
	return &TemporalClusterer{
		usageRepo: usageRepo,
		metaRepo:  metaRepo,
		config:    config,
		logger:    logger.With().Str("component", "temporal_clusterer").Logger(),
	}
}

// FindClusters identifies temporal clusters of file edits within the given time window.
func (c *TemporalClusterer) FindClusters(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	since time.Time,
) ([]*TemporalCluster, error) {
	// Get recent edit events
	events, err := c.usageRepo.GetEventsByType(
		ctx, workspaceID, entity.UsageEventEdited, since, 1000)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Cluster using sliding window
	windowDuration := time.Duration(c.config.WindowHours) * time.Hour
	var clusters []*TemporalCluster
	var currentCluster *TemporalCluster

	for _, event := range events {
		if currentCluster == nil {
			currentCluster = &TemporalCluster{
				FileIDs:       []entity.FileID{entity.FileID(event.DocumentID.String())},
				StartTime:     event.Timestamp,
				EndTime:       event.Timestamp,
				ProjectCounts: make(map[string]int),
			}
			continue
		}

		// Check if this event extends the current cluster
		if event.Timestamp.Sub(currentCluster.EndTime) <= windowDuration {
			// Add to current cluster
			currentCluster.FileIDs = append(currentCluster.FileIDs, entity.FileID(event.DocumentID.String()))
			currentCluster.EndTime = event.Timestamp
		} else {
			// Finalize current cluster if it meets minimum size
			if len(currentCluster.FileIDs) >= c.config.MinClusterSize {
				clusters = append(clusters, currentCluster)
			}
			// Start new cluster
			currentCluster = &TemporalCluster{
				FileIDs:       []entity.FileID{entity.FileID(event.DocumentID.String())},
				StartTime:     event.Timestamp,
				EndTime:       event.Timestamp,
				ProjectCounts: make(map[string]int),
			}
		}
	}

	// Don't forget the last cluster
	if currentCluster != nil && len(currentCluster.FileIDs) >= c.config.MinClusterSize {
		clusters = append(clusters, currentCluster)
	}

	// Enrich clusters with project information
	for _, cluster := range clusters {
		if err := c.enrichClusterWithProjects(ctx, workspaceID, cluster); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to enrich cluster with projects")
		}
	}

	return clusters, nil
}

// enrichClusterWithProjects adds project information to a cluster.
func (c *TemporalClusterer) enrichClusterWithProjects(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	cluster *TemporalCluster,
) error {
	for _, fileID := range cluster.FileIDs {
		meta, err := c.metaRepo.Get(ctx, workspaceID, fileID)
		if err != nil || meta == nil {
			continue
		}

		cluster.RelativePaths = append(cluster.RelativePaths, meta.RelativePath)

		for _, project := range meta.Contexts {
			cluster.ProjectCounts[project]++
		}
	}

	// Find dominant project
	if len(cluster.ProjectCounts) > 0 {
		var maxProject string
		maxCount := 0

		for project, count := range cluster.ProjectCounts {
			if count > maxCount {
				maxCount = count
				maxProject = project
			}
		}

		if maxProject != "" {
			cluster.DominantProject = &maxProject
			cluster.Confidence = float64(maxCount) / float64(len(cluster.FileIDs))
		}
	}

	return nil
}

// PropagateProjectsFromClusters propagates projects from dominant cluster members to others.
func (c *TemporalClusterer) PropagateProjectsFromClusters(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	clusters []*TemporalCluster,
) (int, error) {
	propagatedCount := 0

	for _, cluster := range clusters {
		if cluster.DominantProject == nil {
			continue
		}

		// Only propagate if confidence meets threshold
		if cluster.Confidence < c.config.SuggestionThreshold {
			continue
		}

		autoApply := cluster.Confidence >= c.config.DominanceThreshold

		for _, fileID := range cluster.FileIDs {
			meta, err := c.metaRepo.Get(ctx, workspaceID, fileID)
			if err != nil || meta == nil {
				continue
			}

			// Skip if already has this project
			hasProject := false
			for _, ctx := range meta.Contexts {
				if ctx == *cluster.DominantProject {
					hasProject = true
					break
				}
			}
			if hasProject {
				continue
			}

			if autoApply {
				// Auto-assign project (high confidence)
				if err := c.metaRepo.AddContext(ctx, workspaceID, fileID, *cluster.DominantProject); err != nil {
					c.logger.Warn().Err(err).
						Str("file", meta.RelativePath).
						Str("project", *cluster.DominantProject).
						Msg("Failed to auto-assign project from cluster")
					continue
				}
				c.logger.Info().
					Str("file", meta.RelativePath).
					Str("project", *cluster.DominantProject).
					Float64("confidence", cluster.Confidence).
					Msg("Auto-assigned project from temporal cluster")
			} else {
				// Add as suggestion (lower confidence)
				if err := c.metaRepo.AddSuggestedContext(ctx, workspaceID, fileID, *cluster.DominantProject); err != nil {
					c.logger.Warn().Err(err).
						Str("file", meta.RelativePath).
						Str("project", *cluster.DominantProject).
						Msg("Failed to suggest project from cluster")
					continue
				}
				c.logger.Debug().
					Str("file", meta.RelativePath).
					Str("project", *cluster.DominantProject).
					Float64("confidence", cluster.Confidence).
					Msg("Suggested project from temporal cluster")
			}
			propagatedCount++
		}
	}

	return propagatedCount, nil
}
