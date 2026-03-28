package trace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

const (
	promptSuffix = "prompt"
	outputSuffix = "output"
	tracePreview = 4000
)

// Writer persists LLM traces to disk and database.
type Writer struct {
	repo   repository.TraceRepository
	logger zerolog.Logger
}

// NewWriter creates a new trace writer.
func NewWriter(repo repository.TraceRepository, logger zerolog.Logger) *Writer {
	return &Writer{
		repo:   repo,
		logger: logger.With().Str("component", "trace_writer").Logger(),
	}
}

// WriteLLMTrace writes prompt/output files and stores trace metadata.
func (w *Writer) WriteLLMTrace(ctx context.Context, trace llm.LLMTrace) error {
	if w.repo == nil {
		return fmt.Errorf("trace repository not configured")
	}
	info := trace.Info
	if info.WorkspaceRoot == "" || info.WorkspaceID == "" || info.RelativePath == "" {
		return fmt.Errorf("trace info incomplete")
	}

	op := strings.TrimSpace(info.Operation)
	if op == "" {
		op = "llm"
	}

	traceRoot := filepath.Join(info.WorkspaceRoot, ".cortex", "traces")
	basePath := filepath.Join(traceRoot, filepath.FromSlash(info.RelativePath))
	
	w.logger.Debug().
		Str("workspace_root", info.WorkspaceRoot).
		Str("relative_path", info.RelativePath).
		Str("operation", op).
		Str("stage", info.Stage).
		Str("trace_root", traceRoot).
		Msg("Writing LLM trace files")
	
	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		w.logger.Error().Err(err).
			Str("dir", filepath.Dir(basePath)).
			Msg("Failed to create trace directory")
		return err
	}
	w.logger.Debug().
		Str("dir", filepath.Dir(basePath)).
		Msg("Created trace directory")

	promptPath := basePath + "." + op + "." + promptSuffix + ".md"
	outputPath := basePath + "." + op + "." + outputSuffix + ".md"

	promptText := sanitizeUTF8(trace.Prompt)
	promptSize := len([]byte(promptText))
	if err := os.WriteFile(promptPath, []byte(promptText), 0o600); err != nil {
		w.logger.Error().Err(err).
			Str("path", promptPath).
			Int("size", promptSize).
			Msg("Failed to write trace prompt file")
		return err
	}
	w.logger.Info().
		Str("path", promptPath).
		Int("size", promptSize).
		Str("operation", op).
		Str("file", info.RelativePath).
		Msg("Created trace prompt file")

	outputText := sanitizeUTF8(trace.Output)
	if outputText == "" && trace.Error != nil {
		outputText = fmt.Sprintf("ERROR: %s", sanitizeUTF8(*trace.Error))
	}
	outputSize := len([]byte(outputText))
	if err := os.WriteFile(outputPath, []byte(outputText), 0o600); err != nil {
		w.logger.Error().Err(err).
			Str("path", outputPath).
			Int("size", outputSize).
			Msg("Failed to write trace output file")
		return err
	}
	w.logger.Info().
		Str("path", outputPath).
		Int("size", outputSize).
		Str("operation", op).
		Str("file", info.RelativePath).
		Int("tokens", trace.TokensUsed).
		Int64("duration_ms", trace.DurationMs).
		Msg("Created trace output file")

	promptRel, _ := filepath.Rel(info.WorkspaceRoot, promptPath)
	outputRel, _ := filepath.Rel(info.WorkspaceRoot, outputPath)

	entityTrace := entity.ProcessingTrace{
		WorkspaceID:   entity.WorkspaceID(info.WorkspaceID),
		FileID:        entity.FileID(info.FileID),
		RelativePath:  sanitizeUTF8(info.RelativePath),
		Stage:         sanitizeUTF8(info.Stage),
		Operation:     sanitizeUTF8(op),
		PromptPath:    sanitizeUTF8(promptRel),
		OutputPath:    sanitizeUTF8(outputRel),
		PromptPreview: truncate(promptText, tracePreview),
		OutputPreview: truncate(outputText, tracePreview),
		Model:         sanitizeUTF8(trace.Model),
		TokensUsed:    trace.TokensUsed,
		DurationMs:    trace.DurationMs,
		Error:         sanitizeErr(trace.Error),
		CreatedAt:     trace.GeneratedAt,
	}

	// Use a longer timeout for database operations, and don't block the pipeline
	// Traces are debugging information and should not prevent file processing
	dbCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.repo.AddTrace(dbCtx, entityTrace); err != nil {
		// Log the error but don't return it - traces are non-critical debugging info
		// This prevents trace write failures from blocking the processing pipeline
		w.logger.Warn().Err(err).
			Str("file", info.RelativePath).
			Str("operation", op).
			Msg("Failed to persist trace metadata to database (non-blocking)")
		// Don't return error - traces are optional debugging information
		return nil
	}
	
	w.logger.Debug().
		Str("file", info.RelativePath).
		Str("operation", op).
		Str("model", trace.Model).
		Msg("Persisted trace metadata to database")

	return nil
}

func truncate(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max]
}

func sanitizeUTF8(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	return strings.ToValidUTF8(text, "?")
}

func sanitizeErr(err *string) *string {
	if err == nil {
		return nil
	}
	s := sanitizeUTF8(*err)
	return &s
}
