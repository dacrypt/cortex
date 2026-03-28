// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// imageExtensions lists file extensions processed by the vision stage.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".tiff": true, ".tif": true, ".bmp": true,
	".heic": true, ".heif": true,
}

// VisionStage uses LLM vision models to describe image content,
// then stores and embeds the description for semantic search.
type VisionStage struct {
	llmRouter   *llm.Router
	docRepo     repository.DocumentRepository
	vectorStore repository.VectorStore
	embedder    embedding.Embedder
	logger      zerolog.Logger
	model       string
	maxSizeMB   int64
	prompt      string
}

// NewVisionStage creates a new vision processing stage.
func NewVisionStage(
	llmRouter *llm.Router,
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	model string,
	maxSizeMB int64,
	prompt string,
	logger zerolog.Logger,
) *VisionStage {
	if model == "" {
		model = "llama3.2-vision"
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	if prompt == "" {
		prompt = "Describe this image in detail. Include: the main subject, any text visible, colors, layout, and what type of document or content this appears to be."
	}
	return &VisionStage{
		llmRouter:   llmRouter,
		docRepo:     docRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		logger:      logger.With().Str("stage", "vision").Logger(),
		model:       model,
		maxSizeMB:   maxSizeMB,
		prompt:      prompt,
	}
}

// Name returns the stage name.
func (s *VisionStage) Name() string {
	return "vision"
}

// CanProcess returns true for image file extensions.
func (s *VisionStage) CanProcess(entry *entity.FileEntry) bool {
	if entry == nil {
		return false
	}
	ext := strings.ToLower(entry.Extension)
	return imageExtensions[ext]
}

// Process reads the image, sends it to a vision model, and stores the description.
func (s *VisionStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) || s.llmRouter == nil {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return nil
	}

	// Check file size
	maxBytes := s.maxSizeMB * 1024 * 1024
	if entry.FileSize > maxBytes {
		s.logger.Debug().
			Str("file", entry.RelativePath).
			Int64("size", entry.FileSize).
			Int64("max", maxBytes).
			Msg("Skipping image: exceeds max file size")
		return nil
	}

	// Read image bytes
	imgData, err := os.ReadFile(entry.AbsolutePath)
	if err != nil {
		s.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to read image")
		return nil // Don't fail the pipeline
	}

	// Send to vision model
	genCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := s.llmRouter.Generate(genCtx, llm.GenerateRequest{
		Prompt:    s.prompt,
		Model:     s.model,
		MaxTokens: 1024,
		Images:    [][]byte{imgData},
	})
	if err != nil {
		s.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Vision model failed, skipping")
		return nil // Graceful degradation
	}

	description := strings.TrimSpace(resp.Text)
	if description == "" {
		return nil
	}

	s.logger.Info().
		Str("file", entry.RelativePath).
		Int("description_len", len(description)).
		Msg("Generated image description")

	// Store as document for RAG indexing
	if s.docRepo != nil {
		docID := entity.DocumentID(fmt.Sprintf("%x", sha256.Sum256([]byte(entry.RelativePath))))
		chunkID := entity.ChunkID(fmt.Sprintf("%x", sha256.Sum256([]byte(entry.RelativePath+":vision"))))

		doc := &entity.Document{
			ID:           docID,
			RelativePath: entry.RelativePath,
			Title:        entry.Filename,
			Checksum:     fmt.Sprintf("%x", sha256.Sum256(imgData)),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := s.docRepo.UpsertDocument(ctx, wsInfo.ID, doc); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to store vision document")
		}

		chunk := &entity.Chunk{
			ID:          chunkID,
			DocumentID:  docID,
			Ordinal:     0,
			Heading:     "Image Description",
			HeadingPath: "Image Description",
			Text:        description,
			TokenCount:  len(strings.Fields(description)),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := s.docRepo.ReplaceChunks(ctx, wsInfo.ID, docID, []*entity.Chunk{chunk}); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to store vision chunk")
		}

		// Embed the description for vector search
		if s.embedder != nil && s.vectorStore != nil {
			vector, err := s.embedder.Embed(ctx, description)
			if err == nil && len(vector) > 0 {
				emb := entity.ChunkEmbedding{
					ChunkID:    chunkID,
					Vector:     vector,
					Dimensions: len(vector),
					UpdatedAt:  time.Now(),
				}
				if err := s.vectorStore.Upsert(ctx, wsInfo.ID, emb); err != nil {
					s.logger.Warn().Err(err).Msg("Failed to store vision embedding")
				}
			}
		}
	}

	return nil
}
