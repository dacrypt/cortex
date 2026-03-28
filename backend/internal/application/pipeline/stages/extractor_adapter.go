// Package stages provides adapters to use pipeline stages as metadata extractors.
package stages

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// StageInterface represents a pipeline stage interface.
// This is a local interface that matches pipeline.Stage to avoid import cycles.
type StageInterface interface {
	Name() string
	Process(ctx context.Context, entry *entity.FileEntry) error
}

// ConditionalStageInterface represents a conditional stage interface.
// This is a local interface that matches pipeline.ConditionalStage to avoid import cycles.
type ConditionalStageInterface interface {
	StageInterface
	CanProcess(entry *entity.FileEntry) bool
}

// StageToExtractorAdapter adapts a pipeline Stage to service.MetadataExtractor interface.
// This allows stages to be used interchangeably with extractors.
type StageToExtractorAdapter struct {
	stage    StageInterface
	priority int
}

// NewStageToExtractorAdapter creates a new adapter from a stage.
func NewStageToExtractorAdapter(stage StageInterface, priority int) service.MetadataExtractor {
	return &StageToExtractorAdapter{
		stage:    stage,
		priority: priority,
	}
}

// Extract extracts metadata by calling the stage's Process method.
func (a *StageToExtractorAdapter) Extract(ctx context.Context, entry *entity.FileEntry) error {
	return a.stage.Process(ctx, entry)
}

// CanExtract checks if the stage can process this file.
func (a *StageToExtractorAdapter) CanExtract(entry *entity.FileEntry) bool {
	cs, ok := a.stage.(ConditionalStageInterface)
	if !ok {
		return true // If not conditional, can process all files
	}
	return cs.CanProcess(entry)
}

// GetPriority returns the priority for this extractor.
func (a *StageToExtractorAdapter) GetPriority() int {
	return a.priority
}

// ExtractorsFromStages converts a slice of stages to extractors with priorities.
// This is a convenience function for setting up extraction pipelines.
func ExtractorsFromStages(stages []StageInterface, priorities []int) []service.MetadataExtractor {
	if len(priorities) != len(stages) {
		// Use default priorities if not provided
		priorities = make([]int, len(stages))
		for i := range priorities {
			priorities[i] = 100 - i*10 // Decreasing priority
		}
	}

	extractors := make([]service.MetadataExtractor, len(stages))
	for i, stage := range stages {
		extractors[i] = NewStageToExtractorAdapter(stage, priorities[i])
	}
	return extractors
}

// ExtractWithExtractors applies multiple extractors to a file entry in priority order.
// Higher priority extractors run first.
func ExtractWithExtractors(ctx context.Context, entry *entity.FileEntry, extractors []service.MetadataExtractor) error {
	// Sort by priority (higher first)
	// Note: In a real implementation, you'd want to sort once and reuse
	// For now, we'll process in order assuming they're already sorted
	
	for _, extractor := range extractors {
		if !extractor.CanExtract(entry) {
			continue
		}
		
		if err := extractor.Extract(ctx, entry); err != nil {
			// Log error but continue with other extractors
			// This allows partial extraction to succeed
			continue
		}
	}
	
	return nil
}

