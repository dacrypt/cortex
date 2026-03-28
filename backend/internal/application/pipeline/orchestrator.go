// Package pipeline provides the processing pipeline orchestrator.
package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
)

// Processor defines the interface for processing file entries through the pipeline.
// This interface allows for easier testing and dependency injection.
type Processor interface {
	Process(ctx context.Context, entry *entity.FileEntry) error
}

// Stage represents a pipeline processing stage.
type Stage interface {
	Name() string
	Process(ctx context.Context, entry *entity.FileEntry) error
}

// ConditionalStage is a stage that only processes certain files.
type ConditionalStage interface {
	Stage
	CanProcess(entry *entity.FileEntry) bool
}

// FinalizableStage is a stage that can be finalized after all files are processed.
type FinalizableStage interface {
	Stage
	Finalize(ctx context.Context) error
}

// Orchestrator manages the processing pipeline.
type Orchestrator struct {
	stages    []Stage
	publisher event.Publisher
	logger    zerolog.Logger
	mu        sync.RWMutex
}

// NewOrchestrator creates a new pipeline orchestrator with default stages.
func NewOrchestrator(publisher event.Publisher, logger zerolog.Logger) *Orchestrator {
	o := &Orchestrator{
		publisher: publisher,
		logger:    logger.With().Str("component", "pipeline").Logger(),
	}

	// Add default stages in order
	o.stages = []Stage{
		stages.NewBasicStage(),
		stages.NewMimeStage(),
		stages.NewCodeStage(),
	}

	return o
}

// AddStage adds a stage to the pipeline.
func (o *Orchestrator) AddStage(stage Stage) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stages = append(o.stages, stage)
}

// InsertStage inserts a stage at a specific position.
func (o *Orchestrator) InsertStage(index int, stage Stage) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if index < 0 || index > len(o.stages) {
		return fmt.Errorf("invalid stage index: %d", index)
	}

	o.stages = append(o.stages[:index], append([]Stage{stage}, o.stages[index:]...)...)
	return nil
}

// Process runs a file entry through all pipeline stages.
func (o *Orchestrator) Process(ctx context.Context, entry *entity.FileEntry) error {
	o.mu.RLock()
	stagesCopy := make([]Stage, len(o.stages))
	copy(stagesCopy, o.stages)
	o.mu.RUnlock()

	o.logger.Debug().
		Str("path", entry.RelativePath).
		Int("stages", len(stagesCopy)).
		Msg("Processing file through pipeline")

	publisher := o.createEventPublisher(ctx, entry)
	publisher(event.EventPipelineStarted, "start", nil)

	for _, stage := range stagesCopy {
		if err := o.processStage(ctx, entry, stage, publisher); err != nil {
			return err
		}
	}

	publisher(event.EventPipelineCompleted, "complete", nil)

	o.logger.Debug().
		Str("path", entry.RelativePath).
		Msg("Pipeline processing complete")

	return nil
}

// createEventPublisher creates an event publisher function for a file entry.
func (o *Orchestrator) createEventPublisher(ctx context.Context, entry *entity.FileEntry) func(eventType event.EventType, stage string, err error) {
	wsInfo, wsOK := contextinfo.GetWorkspaceInfo(ctx)
	return func(eventType event.EventType, stage string, err error) {
		if o.publisher == nil {
			return
		}
		var errStr *string
		if err != nil {
			msg := err.Error()
			errStr = &msg
		}
		evt := event.NewEvent(eventType, event.PipelineEventData{
			FilePath: entry.RelativePath,
			Stage:    stage,
			Error:    errStr,
		})
		if wsOK {
			evt.WithWorkspace(wsInfo.ID)
		}
		if pubErr := o.publisher.Publish(ctx, evt); pubErr != nil {
			o.logger.Warn().
				Err(pubErr).
				Str("path", entry.RelativePath).
				Str("stage", stage).
				Msg("Failed to publish pipeline event")
		}
	}
}

// processStage processes a single stage for a file entry.
func (o *Orchestrator) processStage(ctx context.Context, entry *entity.FileEntry, stage Stage, publish func(eventType event.EventType, stage string, err error)) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if stage can process this file
	if !o.canStageProcess(entry, stage) {
		return nil
	}

	stageName := stage.Name()
	o.logger.Trace().
		Str("path", entry.RelativePath).
		Str("stage", stageName).
		Msg("Running stage")
	publish(event.EventPipelineProgress, stageName, nil)

	if err := stage.Process(ctx, entry); err != nil {
		o.logger.Warn().
			Err(err).
			Str("path", entry.RelativePath).
			Str("stage", stageName).
			Msg("Stage failed")
		
		// Check if this is an indexing error (fatal - requires stopping pipeline)
		// Indexing errors are marked in entry.Enhanced.IndexingErrors
		if entry.Enhanced != nil && entry.Enhanced.HasIndexingErrors() {
			publish(event.EventPipelineFailed, stageName, err)
			o.logger.Warn().
				Err(err).
				Str("path", entry.RelativePath).
				Str("stage", stageName).
				Int("error_count", len(entry.Enhanced.IndexingErrors)).
				Msg("Indexing error detected - stopping pipeline for this file")
			// Return error to stop pipeline - this is a fatal error
			return err
		}
		
		// For non-indexing errors, check if we should continue or stop
		// By default, continue to next stage for non-fatal errors
		// But log that we're continuing despite the error
		o.logger.Debug().
			Err(err).
			Str("path", entry.RelativePath).
			Str("stage", stageName).
			Msg("Non-fatal error - continuing to next stage")
	}

	return nil
}

// canStageProcess checks if a stage can process the given entry.
func (o *Orchestrator) canStageProcess(entry *entity.FileEntry, stage Stage) bool {
	cs, ok := stage.(ConditionalStage)
	if !ok {
		return true
	}
	if !cs.CanProcess(entry) {
		o.logger.Trace().
			Str("path", entry.RelativePath).
			Str("stage", stage.Name()).
			Msg("Stage skipped (cannot process)")
		return false
	}
	return true
}

// ProcessBatch processes multiple files through the pipeline.
func (o *Orchestrator) ProcessBatch(ctx context.Context, entries []*entity.FileEntry) error {
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := o.Process(ctx, entry); err != nil {
			// Log and continue for batch processing
			o.logger.Error().
				Err(err).
				Str("path", entry.RelativePath).
				Msg("Failed to process file in batch")
		}
	}
	return nil
}

// ProcessConcurrent processes files concurrently with limited parallelism.
func (o *Orchestrator) ProcessConcurrent(ctx context.Context, entries []*entity.FileEntry, workers int) error {
	if workers <= 0 {
		workers = 1
	}

	type result struct {
		entry *entity.FileEntry
		err   error
	}

	entryChan := make(chan *entity.FileEntry, len(entries))
	resultChan := make(chan result, len(entries))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range entryChan {
				select {
				case <-ctx.Done():
					return
				default:
					err := o.Process(ctx, entry)
					resultChan <- result{entry: entry, err: err}
				}
			}
		}()
	}

	// Send entries to workers
	for _, entry := range entries {
		entryChan <- entry
	}
	close(entryChan)

	// Wait for all workers
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var errors []error
	for res := range resultChan {
		if res.err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", res.entry.RelativePath, res.err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("pipeline had %d errors", len(errors))
	}

	return nil
}

// Finalize calls Finalize() on all stages that support it.
// This should be called after all files have been processed.
func (o *Orchestrator) Finalize(ctx context.Context) error {
	o.mu.RLock()
	stagesCopy := make([]Stage, len(o.stages))
	copy(stagesCopy, o.stages)
	o.mu.RUnlock()

	o.logger.Debug().Msg("Finalizing pipeline stages")

	for _, stage := range stagesCopy {
		if finalizable, ok := stage.(FinalizableStage); ok {
			if err := finalizable.Finalize(ctx); err != nil {
				o.logger.Warn().
					Err(err).
					Str("stage", stage.Name()).
					Msg("Stage finalization failed")
				// Continue with other stages even if one fails
			}
		}
	}

	o.logger.Debug().Msg("Pipeline finalization complete")
	return nil
}

// GetStages returns the current pipeline stages.
func (o *Orchestrator) GetStages() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	names := make([]string, len(o.stages))
	for i, s := range o.stages {
		names[i] = s.Name()
	}
	return names
}
