// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/textsplitter"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/utils"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DocumentStage parses documents, chunks content, and stores embeddings.
type DocumentStage struct {
	metaRepo    repository.MetadataRepository
	docRepo     repository.DocumentRepository
	vectorStore repository.VectorStore
	embedder    embedding.Embedder
	logger      zerolog.Logger
	maxTokens   int
	minTokens   int
}

// NewDocumentStage creates a new document parsing stage.
// WorkspaceID is obtained from context during processing.
func NewDocumentStage(
	metaRepo repository.MetadataRepository,
	docRepo repository.DocumentRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	logger zerolog.Logger,
) *DocumentStage {
	return &DocumentStage{
		metaRepo:    metaRepo,
		docRepo:     docRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		logger:      logger.With().Str("component", "document_stage").Logger(),
		maxTokens:   800,
		minTokens:   300,
	}
}

// Name returns the stage name.
func (s *DocumentStage) Name() string {
	return "document"
}

// CanProcess returns true for Markdown files or mirrorable content.
func (s *DocumentStage) CanProcess(entry *entity.FileEntry) bool {
	if entry == nil {
		return false
	}
	if strings.EqualFold(entry.Extension, ".md") {
		return true
	}
	return entity.GetMirrorFormat(strings.TrimPrefix(entry.Extension, ".")) != nil
}

// Process extracts Markdown structure, chunks, and embeds.
func (s *DocumentStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}
	if s.docRepo == nil {
		err := fmt.Errorf("CRITICAL: DocumentRepository is nil - DocumentStage requires DocumentRepository")
		s.logger.Error().Err(err).Str("file", entry.RelativePath).Msg("CRITICAL: DocumentRepository not available")
		return err
	}
	if s.vectorStore == nil {
		err := fmt.Errorf("CRITICAL: VectorStore is nil - DocumentStage requires VectorStore")
		s.logger.Error().Err(err).Str("file", entry.RelativePath).Msg("CRITICAL: VectorStore not available")
		return err
	}
	if s.embedder == nil {
		err := fmt.Errorf("CRITICAL: Embedder is nil - DocumentStage requires Embedder for RAG")
		s.logger.Error().Err(err).Str("file", entry.RelativePath).Msg("CRITICAL: Embedder not available")
		return err
	}

	// Get workspace ID from context
	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return fmt.Errorf("workspace info not found in context")
	}
	workspaceID := wsInfo.ID

	contentPath := entry.AbsolutePath
	mirrorPath := ""
	if !strings.EqualFold(entry.Extension, ".md") {
		// First, check if MirrorStage has already processed this file (in-memory state)
		if entry.Enhanced != nil && entry.Enhanced.IndexedState.Mirror {
			// MirrorStage has run - construct mirror path using same logic
			format := entity.GetMirrorFormat(strings.TrimPrefix(entry.Extension, "."))
			if format != nil && (*format == entity.MirrorFormatMarkdown || *format == entity.MirrorFormatCSV) {
				mirrorPath = entity.GetMirrorPath(entry.RelativePath, *format)
				// Convert to absolute path
				mirrorPath = filepath.Join(wsInfo.Root, mirrorPath)
				// Verify mirror file exists before using it
				if _, err := os.Stat(mirrorPath); err == nil {
					contentPath = mirrorPath
					s.logger.Debug().
						Str("file", entry.RelativePath).
						Str("mirror_path", mirrorPath).
						Msg("Using mirror file for document parsing (from IndexedState)")
				} else {
					// Mirror file doesn't exist even though IndexedState says it should
					errMsg := fmt.Sprintf("Mirror file not found: %s", mirrorPath)
					details := fmt.Sprintf("IndexedState.Mirror is true but mirror file does not exist at %s. This indicates a data inconsistency.", mirrorPath)
					
					entry.Enhanced.AddIndexingError(
						"document",
						"read_mirror",
						errMsg,
						details,
						"mirror_file",
					)
					
					s.logger.Warn().
						Err(err).
						Str("file", entry.RelativePath).
						Str("mirror_path", mirrorPath).
						Str("extension", entry.Extension).
						Msg("Indexing error: Mirror file required but not available - stopping pipeline")
					
					return fmt.Errorf("indexing error: mirror file required but not available for %s: %w", entry.RelativePath, err)
				}
			}
		} else if s.metaRepo != nil {
			// MirrorStage hasn't run yet or IndexedState not set - try reading from repository
			meta, err := s.metaRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
			if err == nil && meta != nil && meta.Mirror != nil &&
				(meta.Mirror.Format == entity.MirrorFormatMarkdown || meta.Mirror.Format == entity.MirrorFormatCSV) {
				mirrorPath = meta.Mirror.Path
				// Verify mirror file exists before using it
				if _, err := os.Stat(mirrorPath); err == nil {
					contentPath = mirrorPath
					s.logger.Debug().
						Str("file", entry.RelativePath).
						Str("mirror_path", mirrorPath).
						Msg("Using mirror file for document parsing (from repository)")
				} else {
					// Mirror file doesn't exist - mark as indexing error and stop pipeline
					errMsg := fmt.Sprintf("Mirror file not found: %s", mirrorPath)
					details := fmt.Sprintf("Expected mirror file at %s but file does not exist. MirrorStage may not have run yet or extraction failed.", mirrorPath)
					
					// Ensure EnhancedMetadata exists
					if entry.Enhanced == nil {
						entry.Enhanced = &entity.EnhancedMetadata{}
					}
					entry.Enhanced.AddIndexingError(
						"document",
						"read_mirror",
						errMsg,
						details,
						"mirror_file",
					)
					
					s.logger.Warn().
						Err(err).
						Str("file", entry.RelativePath).
						Str("mirror_path", mirrorPath).
						Str("extension", entry.Extension).
						Msg("Indexing error: Mirror file required but not available - stopping pipeline")
					
					// Stop pipeline for this file - return error to indicate failure
					return fmt.Errorf("indexing error: mirror file required but not available for %s: %w", entry.RelativePath, err)
				}
			} else {
				// For non-Markdown files, we need a mirror - if metadata doesn't have it, mark as error
				if strings.EqualFold(entry.Extension, ".pdf") {
					s.logger.Info().
						Str("file", entry.RelativePath).
						Str("extension", entry.Extension).
						Msg("No mirror metadata for PDF; skipping document parsing")
					return nil
				}

				errMsg := "Mirror metadata not available"
				details := fmt.Sprintf("File %s requires a mirror file for document parsing, but no mirror metadata found. MirrorStage may not have run yet or extraction failed.", entry.RelativePath)
				
				// Ensure EnhancedMetadata exists
				if entry.Enhanced == nil {
					entry.Enhanced = &entity.EnhancedMetadata{}
				}
				entry.Enhanced.AddIndexingError(
					"document",
					"check_mirror_metadata",
					errMsg,
					details,
					"mirror_metadata",
				)
				
				s.logger.Warn().
					Str("file", entry.RelativePath).
					Str("extension", entry.Extension).
					Msg("Indexing error: Mirror metadata required but not available - stopping pipeline")
				
				// Stop pipeline for this file
				return fmt.Errorf("indexing error: mirror metadata required but not available for %s", entry.RelativePath)
			}
		} else {
			// No metaRepo available and IndexedState not set - mark as error
			if strings.EqualFold(entry.Extension, ".pdf") {
				s.logger.Info().
					Str("file", entry.RelativePath).
					Str("extension", entry.Extension).
					Msg("No mirror metadata for PDF; skipping document parsing")
				return nil
			}

			errMsg := "Mirror metadata not available"
			details := fmt.Sprintf("File %s requires a mirror file for document parsing, but MetadataRepository is not available and IndexedState.Mirror is not set.", entry.RelativePath)
			
			// Ensure EnhancedMetadata exists
			if entry.Enhanced == nil {
				entry.Enhanced = &entity.EnhancedMetadata{}
			}
			entry.Enhanced.AddIndexingError(
				"document",
				"check_mirror_metadata",
				errMsg,
				details,
				"mirror_metadata",
			)
			
			s.logger.Warn().
				Str("file", entry.RelativePath).
				Str("extension", entry.Extension).
				Msg("Indexing error: Mirror metadata required but not available - stopping pipeline")
			
			// Stop pipeline for this file
			return fmt.Errorf("indexing error: mirror metadata required but not available for %s", entry.RelativePath)
		}
	}

	s.logger.Debug().
		Str("file", entry.RelativePath).
		Str("content_path", contentPath).
		Bool("is_mirror", mirrorPath != "" && contentPath == mirrorPath).
		Msg("Reading content for document parsing")

	content, err := os.ReadFile(contentPath)
	if err != nil {
		s.logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("content_path", contentPath).
			Msg("Failed to read content file")
		return err
	}
	
	// Validate that content is text (not binary)
	// First, try to decode as UTF-8 to verify it's valid text
	// If it's valid UTF-8, it's likely text even if it has many non-ASCII characters
	decodedContent := ""
	if utf8.Valid(content) {
		decodedContent = string(content)
	} else {
		// Not valid UTF-8 - likely binary
		errMsg := "Content is not valid UTF-8 text"
		details := fmt.Sprintf("Content at %s is not valid UTF-8, indicating binary content. Expected text/markdown content.", contentPath)
		
		if entry.Enhanced == nil {
			entry.Enhanced = &entity.EnhancedMetadata{}
		}
		entry.Enhanced.AddIndexingError(
			"document",
			"validate_utf8",
			errMsg,
			details,
			"utf8_text",
		)
		
		s.logger.Warn().
			Str("file", entry.RelativePath).
			Str("content_path", contentPath).
			Int("content_size", len(content)).
			Msg("Indexing error: Content is not valid UTF-8 - stopping pipeline")
		
		return fmt.Errorf("indexing error: content is not valid UTF-8 for %s", entry.RelativePath)
	}
	
	// Additional check: verify it's not mostly control characters
	// Count printable characters (letters, numbers, punctuation, whitespace) by runes
	printableRunes := 0
	totalRunes := 0
	maxSampleRunes := 10000
	for _, r := range decodedContent {
		if totalRunes >= maxSampleRunes {
			break
		}
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printableRunes++
		}
		totalRunes++
	}
	
	if totalRunes == 0 {
		totalRunes = 1
	}
	printableRatio := float64(printableRunes) / float64(totalRunes)
	// If less than 50% is printable, it's likely binary or corrupted
	if printableRatio < 0.5 {
		errMsg := "Content appears to be binary or corrupted"
		details := fmt.Sprintf("Content at %s has only %.2f%% printable characters, indicating binary or corrupted content. Expected text/markdown content. Mirror file may not be available yet or extraction failed.", contentPath, printableRatio*100)
		
		// Ensure EnhancedMetadata exists
		if entry.Enhanced == nil {
			entry.Enhanced = &entity.EnhancedMetadata{}
		}
		entry.Enhanced.AddIndexingError(
			"document",
			"validate_content",
			errMsg,
			details,
			"text_content",
		)
		
		s.logger.Warn().
			Str("file", entry.RelativePath).
			Str("content_path", contentPath).
			Float64("printable_ratio", printableRatio).
			Int("content_size", len(content)).
			Msg("Indexing error: Content appears to be binary or corrupted - stopping pipeline")
		
		// Stop pipeline for this file
		return fmt.Errorf("indexing error: content appears to be binary or corrupted for %s (printable ratio: %.2f%%)", entry.RelativePath, printableRatio*100)
	}
	
	// Use the decoded UTF-8 content for parsing
	content = []byte(decodedContent)

	s.logger.Debug().
		Str("file", entry.RelativePath).
		Int("content_size", len(content)).
		Msg("Parsing document structure")

	frontmatter, body, bodyStartLine := parseFrontmatter(string(content))
	docID := entity.NewDocumentID(entry.RelativePath)
	chunks := buildChunksWithLangchain(docID, body, bodyStartLine, s.minTokens, s.maxTokens, s.logger)

	title := inferTitle(entry, frontmatter, body)
	checksum := hashContent(body)

	// Analyze document structure
	contentStructure := s.analyzeContentStructure(body)
	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}
	entry.Enhanced.ContentStructure = contentStructure

	s.logger.Debug().
		Str("file", entry.RelativePath).
		Str("doc_id", string(docID)).
		Str("title", title).
		Int("chunks", len(chunks)).
		Str("checksum", checksum).
		Int("headings", len(contentStructure.Headings)).
		Int("lists", contentStructure.ListCount).
		Msg("Document parsed, creating document entity")

	doc := &entity.Document{
		ID:           docID,
		FileID:       entry.ID,
		RelativePath: entry.RelativePath,
		Title:        title,
		Frontmatter:  frontmatter,
		Checksum:     checksum,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.docRepo.UpsertDocument(ctx, workspaceID, doc); err != nil {
		s.logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("doc_id", string(docID)).
			Msg("Failed to upsert document")
		return err
	}
	s.logger.Info().
		Str("file", entry.RelativePath).
		Str("doc_id", string(docID)).
		Str("title", title).
		Msg("Document created/updated in database")

	if err := s.docRepo.ReplaceChunks(ctx, workspaceID, doc.ID, chunks); err != nil {
		s.logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("doc_id", string(docID)).
			Int("chunks", len(chunks)).
			Msg("Failed to replace chunks")
		return err
	}
	s.logger.Info().
		Str("file", entry.RelativePath).
		Str("doc_id", string(docID)).
		Int("chunks", len(chunks)).
		Msg("Chunks created/updated in database")

	s.logger.Debug().
		Str("file", entry.RelativePath).
		Str("doc_id", string(docID)).
		Int("chunks", len(chunks)).
		Msg("Generating embeddings for chunks")

	// Retrieve metadata to enrich embeddings
	var fileMeta *entity.FileMetadata
	if s.metaRepo != nil {
		meta, err := s.metaRepo.GetByPath(ctx, workspaceID, entry.RelativePath)
		if err == nil && meta != nil {
			fileMeta = meta
			s.logger.Debug().
				Str("file", entry.RelativePath).
				Int("tags", len(meta.Tags)).
				Int("contexts", len(meta.Contexts)).
				Msg("Retrieved metadata for embedding enrichment")
		}
	}

	embeddings := make([]entity.ChunkEmbedding, 0, len(chunks))
	embeddingErrors := 0
	for i, chunk := range chunks {
		// Enrich chunk text with metadata for better vector search
		enrichedText := s.enrichChunkTextWithMetadata(chunk.Text, fileMeta, entry.RelativePath)
		
		// Truncate enriched text if too long for embedding model
		// nomic-embed-text has a limit around 8192 tokens (~4000-5000 chars)
		chunkText := enrichedText
		originalLength := len(chunkText)
		if len(chunkText) > 4000 {
			// Truncate at word boundary
			truncated := chunkText[:4000]
			lastSpace := strings.LastIndex(truncated, " ")
			if lastSpace > 3600 { // If space is near the end, use it
				chunkText = truncated[:lastSpace] + "..."
			} else {
				chunkText = truncated + "..."
			}
			s.logger.Debug().
				Str("file", entry.RelativePath).
				Str("chunk_id", string(chunk.ID)).
				Int("original_length", originalLength).
				Int("truncated_length", len(chunkText)).
				Int("metadata_enriched", len(enrichedText)-len(chunk.Text)).
				Msg("Truncated enriched chunk text for embedding")
		}
		
		vector, err := s.embedder.Embed(ctx, chunkText)
		if err != nil {
			// CRITICAL: Embeddings are required - fail fast
			embeddingErrors++
			errMsg := fmt.Errorf("CRITICAL: Failed to generate embedding for chunk %d/%d: %w", i+1, len(chunks), err)
			s.logger.Error().Err(errMsg).
				Str("file", entry.RelativePath).
				Str("chunk_id", string(chunk.ID)).
				Int("chunk_index", i+1).
				Int("total_chunks", len(chunks)).
				Msg("CRITICAL: Failed to generate embedding for chunk")
			// Continue with other chunks for partial success, but log as error
			// If too many fail, we'll return error at the end
			continue
		}
		embeddings = append(embeddings, entity.ChunkEmbedding{
			ChunkID:    chunk.ID,
			Vector:     vector,
			Dimensions: len(vector),
			UpdatedAt:  time.Now(),
		})
	}
	
	s.logger.Debug().
		Str("file", entry.RelativePath).
		Str("doc_id", string(docID)).
		Int("total_chunks", len(chunks)).
		Int("embeddings_created", len(embeddings)).
		Int("embedding_errors", embeddingErrors).
		Msg("Embedding generation completed")
	
	// CRITICAL: If no embeddings were created, this is a failure
	// Embeddings are required for RAG functionality
	if len(embeddings) == 0 {
		err := fmt.Errorf("CRITICAL: No embeddings created for document - all %d chunks failed embedding generation", len(chunks))
		s.logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("doc_id", string(docID)).
			Int("total_chunks", len(chunks)).
			Int("embedding_errors", embeddingErrors).
			Msg("CRITICAL: Failed to create any embeddings for document")
		return err
	}
	
	// If too many embeddings failed, this is also a critical error
	if embeddingErrors > len(chunks)/2 {
		err := fmt.Errorf("CRITICAL: More than 50%% of embeddings failed (%d/%d) - document indexing incomplete", embeddingErrors, len(chunks))
		s.logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("doc_id", string(docID)).
			Int("total_chunks", len(chunks)).
			Int("embeddings_created", len(embeddings)).
			Int("embedding_errors", embeddingErrors).
			Msg("CRITICAL: Too many embedding failures")
		return err
	}
	
	// If we have at least some embeddings, store them
	// This allows partial indexing of large documents with some failures
	if len(embeddings) > 0 {
		if err := s.vectorStore.BulkUpsert(ctx, workspaceID, embeddings); err != nil {
			s.logger.Error().Err(err).
				Str("file", entry.RelativePath).
				Str("doc_id", string(docID)).
				Int("embeddings", len(embeddings)).
				Msg("Failed to store embeddings in vector store")
			return err
		}
		
		s.logger.Info().
			Str("file", entry.RelativePath).
			Str("doc_id", string(docID)).
			Int("embeddings", len(embeddings)).
			Int("dimensions", len(embeddings[0].Vector)).
			Msg("Embeddings stored in vector store")
	}

	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}
	entry.Enhanced.IndexedState.Document = true
	
	// Merge document metrics instead of overwriting
	// This preserves metadata extracted by MetadataStage (fonts, images, links, etc.)
	if entry.Enhanced.DocumentMetrics == nil {
		entry.Enhanced.DocumentMetrics = &entity.DocumentMetrics{
			WordCount:      countWords(body),
			CharacterCount: len(body),
			Title:          &title,
		}
	} else {
		// Only update if not already set (preserve metadata from MetadataStage)
		if entry.Enhanced.DocumentMetrics.WordCount == 0 {
			entry.Enhanced.DocumentMetrics.WordCount = countWords(body)
		}
		if entry.Enhanced.DocumentMetrics.CharacterCount == 0 {
			entry.Enhanced.DocumentMetrics.CharacterCount = len(body)
		}
		if entry.Enhanced.DocumentMetrics.Title == nil {
			entry.Enhanced.DocumentMetrics.Title = &title
		}
	}

	return nil
}

func parseFrontmatter(content string) (map[string]interface{}, string, int) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, content, 1
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" || trimmed == "..." {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, content, 1
	}

	frontmatterText := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")
	var frontmatter map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		frontmatter = nil
	}
	return frontmatter, body, end + 2
}

// buildChunksWithLangchain uses langchaingo's MarkdownTextSplitter to split markdown text into chunks.
// This replaces the manual parsing logic with a robust, community-tested solution.
func buildChunksWithLangchain(docID entity.DocumentID, body string, startLine int, minTokens, maxTokens int, logger zerolog.Logger) []*entity.Chunk {
	// Use langchaingo's MarkdownTextSplitter with our configuration
	// Convert maxTokens (word count) to approximate character count (assuming ~4 chars per word)
	chunkSize := maxTokens * 4
	chunkOverlap := minTokens * 2 // Overlap for better context
	
	splitter := textsplitter.NewMarkdownTextSplitter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
		textsplitter.WithHeadingHierarchy(true), // Maintain heading hierarchy
		textsplitter.WithCodeBlocks(true),       // Include code blocks
		textsplitter.WithJoinTableRows(true),    // Keep table rows together
		textsplitter.WithLenFunc(func(s string) int {
			// Use our token counting function (word count)
			return countTokens(s)
		}),
	)
	
	// Split the markdown text
	splitChunks, err := splitter.SplitText(body)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to split text with langchaingo, falling back to simple split")
		// Fallback: create a single chunk
		return []*entity.Chunk{
			{
				ID:          entity.NewChunkID(docID, 1, "Document"),
				DocumentID:  docID,
				Ordinal:     1,
				Heading:     "Document",
				HeadingPath: "Document",
				Text:        body,
				TokenCount:  countTokens(body),
				StartLine:   startLine,
				EndLine:     startLine + strings.Count(body, "\n"),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		}
	}
	
	// Convert langchaingo chunks to our Chunk entities
	chunks := make([]*entity.Chunk, 0, len(splitChunks))
	ordinal := 0
	
	// Track heading hierarchy for each chunk
	// langchaingo's MarkdownTextSplitter preserves heading hierarchy in the text
	// We need to extract it to set HeadingPath
	currentHeadingPath := "Document"
	currentHeading := "Document"
	
	for _, chunkText := range splitChunks {
		// Extract heading from chunk text (langchaingo prepends headings)
		headingPath, heading := extractHeadingFromChunk(chunkText, currentHeadingPath)
		if headingPath != "" {
			currentHeadingPath = headingPath
			currentHeading = heading
		}
		
		// Skip empty chunks
		trimmed := strings.TrimSpace(chunkText)
		if trimmed == "" {
			continue
		}
		
		// Validate chunk meets minimum token requirement
		tokenCount := countTokens(trimmed)
		if tokenCount < minTokens && len(splitChunks) > 1 {
			// Merge with previous chunk if too small (unless it's the only chunk)
			if len(chunks) > 0 {
				lastChunk := chunks[len(chunks)-1]
				lastChunk.Text += "\n\n" + trimmed
				lastChunk.TokenCount = countTokens(lastChunk.Text)
				lastChunk.EndLine = startLine + strings.Count(body[:strings.Index(body, chunkText)+len(chunkText)], "\n")
				continue
			}
		}
		
		ordinal++
		chunkID := entity.NewChunkID(docID, ordinal, currentHeadingPath)
		
		// Calculate approximate line numbers
		chunkStartIdx := strings.Index(body, chunkText)
		chunkStartLine := startLine
		if chunkStartIdx >= 0 {
			chunkStartLine = startLine + strings.Count(body[:chunkStartIdx], "\n")
		}
		chunkEndLine := chunkStartLine + strings.Count(chunkText, "\n")
		
		chunks = append(chunks, &entity.Chunk{
			ID:          chunkID,
			DocumentID:  docID,
			Ordinal:     ordinal,
			Heading:     currentHeading,
			HeadingPath: currentHeadingPath,
			Text:        chunkText,
			TokenCount:  tokenCount,
			StartLine:   chunkStartLine,
			EndLine:     chunkEndLine,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	}
	
	// Ensure we have at least one chunk
	if len(chunks) == 0 {
		ordinal++
		chunkID := entity.NewChunkID(docID, ordinal, "Document")
		chunks = append(chunks, &entity.Chunk{
			ID:          chunkID,
			DocumentID:  docID,
			Ordinal:     ordinal,
			Heading:     "Document",
			HeadingPath: "Document",
			Text:        body,
			TokenCount:  countTokens(body),
			StartLine:   startLine,
			EndLine:     startLine + strings.Count(body, "\n"),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	}
	
	logger.Debug().
		Int("chunks_created", len(chunks)).
		Int("split_chunks", len(splitChunks)).
		Msg("Chunks created using langchaingo MarkdownTextSplitter")
	
	return chunks
}

// extractHeadingFromChunk extracts the heading hierarchy from a chunk text.
// langchaingo's MarkdownTextSplitter prepends headings to chunks.
func extractHeadingFromChunk(chunkText, fallbackPath string) (headingPath, heading string) {
	lines := strings.Split(chunkText, "\n")
	if len(lines) == 0 {
		return fallbackPath, "Document"
	}
	
	// Look for heading in first few lines
	maxLines := 5
	if len(lines) < maxLines {
		maxLines = len(lines)
	}
	for i := 0; i < maxLines; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		// Check if it's a heading (starts with #)
		if strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				headingText := strings.TrimSpace(parts[1])
				
				// Build heading path from hierarchy
				// For simplicity, use just the heading text
				// In a more sophisticated implementation, we'd track the full hierarchy
				return headingText, headingText
			}
		}
	}
	
	return fallbackPath, "Document"
}

// enrichChunkTextWithMetadata enriches chunk text with metadata to improve vector search.
// This allows documents with similar metadata (tags, categories, projects) to be found
// even if their textual content differs.
// relativePath is included to provide semantic context about file location.
func (s *DocumentStage) enrichChunkTextWithMetadata(chunkText string, fileMeta *entity.FileMetadata, relativePath string) string {
	if fileMeta == nil && relativePath == "" {
		return chunkText
	}
	
	var metadataParts []string
	
	// Add path information first - this provides important semantic context
	if relativePath != "" {
		pathAnalyzer := utils.NewPathAnalyzer()
		pathContext := pathAnalyzer.FormatPathForContext(relativePath)
		if pathContext != "" {
			metadataParts = append(metadataParts, "Ubicación: "+pathContext)
		}
		// Also add path components for semantic matching
		components := pathAnalyzer.ExtractComponents(relativePath)
		if len(components) > 0 {
			// Include directory components (excluding filename) for better semantic matching
			if len(components) > 1 {
				dirComponents := components[:len(components)-1]
				metadataParts = append(metadataParts, "Directorio: "+strings.Join(dirComponents, "/"))
			}
		}
	}
	
	if fileMeta == nil {
		// If we only have path info, still enrich with it
		if len(metadataParts) == 0 {
			return chunkText
		}
		metadataSection := strings.Join(metadataParts, ". ")
		enrichedText := metadataSection + "\n\n---\n\n" + chunkText
		return enrichedText
	}
	
	// Add tags
	if len(fileMeta.Tags) > 0 {
		metadataParts = append(metadataParts, "Tags: "+strings.Join(fileMeta.Tags, ", "))
	}
	
	// Add category
	if fileMeta.AICategory != nil && fileMeta.AICategory.Category != "" {
		metadataParts = append(metadataParts, "Categoría: "+fileMeta.AICategory.Category)
	}
	
	// Add project/context
	if len(fileMeta.Contexts) > 0 {
		metadataParts = append(metadataParts, "Proyecto: "+strings.Join(fileMeta.Contexts, ", "))
	}
	
	// Add key terms from AI summary
	if fileMeta.AISummary != nil && len(fileMeta.AISummary.KeyTerms) > 0 {
		// Limit to top 5 key terms to avoid overwhelming the embedding
		keyTerms := fileMeta.AISummary.KeyTerms
		if len(keyTerms) > 5 {
			keyTerms = keyTerms[:5]
		}
		metadataParts = append(metadataParts, "Términos clave: "+strings.Join(keyTerms, ", "))
	}
	
	// Add summary (truncated) if available
	if fileMeta.AISummary != nil && fileMeta.AISummary.Summary != "" {
		summary := fileMeta.AISummary.Summary
		// Limit summary to 200 chars to keep embedding focused on chunk content
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		metadataParts = append(metadataParts, "Resumen: "+summary)
	}
	
	// If no metadata to add, return original text
	if len(metadataParts) == 0 {
		return chunkText
	}
	
	// Prepend metadata to chunk text
	// Format: metadata first, then separator, then original content
	// This ensures metadata is always included even if text is truncated
	metadataSection := strings.Join(metadataParts, ". ")
	enrichedText := metadataSection + "\n\n---\n\n" + chunkText
	
	s.logger.Debug().
		Int("metadata_parts", len(metadataParts)).
		Int("original_length", len(chunkText)).
		Int("enriched_length", len(enrichedText)).
		Msg("Enriched chunk text with metadata")
	
	return enrichedText
}

// splitParagraphs and splitByTokens are no longer needed as langchaingo handles this.
// Keeping countTokens as it's still used for validation and metadata.

func countTokens(text string) int {
	return len(strings.Fields(text))
}

// analyzeContentStructure analyzes the structure of a document (headings, lists, etc.).
func (s *DocumentStage) analyzeContentStructure(content string) *entity.ContentStructure {
	structure := &entity.ContentStructure{
		Headings:     []entity.HeadingInfo{},
		Sections:      []entity.SectionInfo{},
		TOCEntries:    []string{},
	}

	lines := strings.Split(content, "\n")
	currentSection := entity.SectionInfo{}
	headingPath := []string{}
	
	for lineNum, line := range lines {
		lineNum++ // 1-based line numbers
		trimmed := strings.TrimSpace(line)
		
		// Detect headings (Markdown style: # Heading)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, r := range trimmed {
				if r == '#' {
					level++
				} else if r == ' ' {
					break
				} else {
					level = 0
					break
				}
			}
			
			if level > 0 && level <= 6 {
				headingText := strings.TrimSpace(trimmed[level:])
				if headingText != "" {
					// Update heading path
					if level <= len(headingPath) {
						headingPath = headingPath[:level-1]
					}
					headingPath = append(headingPath, headingText)
					
					heading := entity.HeadingInfo{
						Level: level,
						Text:  headingText,
						Line:  lineNum,
						Path:  strings.Join(headingPath, "/"),
					}
					structure.Headings = append(structure.Headings, heading)
					
					if level > structure.HeadingDepth {
						structure.HeadingDepth = level
					}
					
					// Check if this looks like a TOC entry
					if strings.Contains(headingText, "...") || strings.Contains(headingText, "—") {
						structure.HasTOC = true
						structure.TOCEntries = append(structure.TOCEntries, headingText)
					}
					
					// Start new section
					if currentSection.Title != "" {
						currentSection.EndLine = lineNum - 1
						structure.Sections = append(structure.Sections, currentSection)
					}
					currentSection = entity.SectionInfo{
						Title:       headingText,
						Level:       level,
						StartLine:   lineNum,
						HeadingPath: heading.Path,
					}
				}
			}
		}
		
		// Detect lists (Markdown style: - item, * item, 1. item)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || 
		   strings.HasPrefix(trimmed, "+ ") || (len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' && trimmed[2] == ' ') {
			structure.HasLists = true
			structure.ListCount++
			
			// Calculate list depth (indentation)
			indent := 0
			for _, r := range line {
				if r == ' ' || r == '\t' {
					indent++
				} else {
					break
				}
			}
			depth := indent/2 + 1 // Approximate depth based on indentation
			if depth > structure.MaxListDepth {
				structure.MaxListDepth = depth
			}
		}
		
		// Detect cross-references (Markdown links: [text](url))
		if strings.Contains(trimmed, "](") || strings.Contains(trimmed, "][") {
			structure.HasCrossRefs = true
			structure.CrossRefCount++
		}
		
		// Detect footnotes (Markdown style: [^1] or [^note])
		if strings.Contains(trimmed, "[^") {
			structure.HasFootnotes = true
			structure.FootnoteCount++
		}
		
		// Detect endnotes (usually at end of document)
		if strings.Contains(strings.ToLower(trimmed), "endnote") || strings.Contains(strings.ToLower(trimmed), "end note") {
			structure.HasEndnotes = true
			structure.EndnoteCount++
		}
	}
	
	// Close last section
	if currentSection.Title != "" {
		currentSection.EndLine = len(lines)
		structure.Sections = append(structure.Sections, currentSection)
	}
	
	structure.SectionCount = len(structure.Sections)
	
	return structure
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func inferTitle(entry *entity.FileEntry, frontmatter map[string]interface{}, body string) string {
	if frontmatter != nil {
		if raw, ok := frontmatter["title"]; ok {
			if title, ok := raw.(string); ok && strings.TrimSpace(title) != "" {
				return strings.TrimSpace(title)
			}
		}
	}
	// Extract first heading from body
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, " ", 2)
			if len(parts) == 2 {
				title := strings.TrimSpace(parts[1])
				if title != "" {
					return title
				}
			}
		}
	}
	return strings.TrimSuffix(filepath.Base(entry.RelativePath), filepath.Ext(entry.RelativePath))
}

func hashContent(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
