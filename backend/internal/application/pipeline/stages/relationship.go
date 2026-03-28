package stages

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/embedding"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/application/relationship"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// RelationshipStage detects and creates relationships between documents.
type RelationshipStage struct {
	docRepo     repository.DocumentRepository
	relRepo     repository.RelationshipRepository
	metaRepo    repository.MetadataRepository
	projectRepo repository.ProjectRepository
	vectorStore repository.VectorStore
	embedder    embedding.Embedder
	detector    *relationship.Detector
	logger      zerolog.Logger
	useRAG      bool
}

// NewRelationshipStage creates a new relationship detection stage.
func NewRelationshipStage(
	docRepo repository.DocumentRepository,
	relRepo repository.RelationshipRepository,
	logger zerolog.Logger,
) *RelationshipStage {
	detector := relationship.NewDetector(relRepo)
	return &RelationshipStage{
		docRepo:  docRepo,
		relRepo:  relRepo,
		detector: detector,
		logger:   logger.With().Str("stage", "relationship").Logger(),
		useRAG:   false,
	}
}

// NewRelationshipStageWithRAG creates a new relationship detection stage with RAG support.
func NewRelationshipStageWithRAG(
	docRepo repository.DocumentRepository,
	relRepo repository.RelationshipRepository,
	metaRepo repository.MetadataRepository,
	projectRepo repository.ProjectRepository,
	vectorStore repository.VectorStore,
	embedder embedding.Embedder,
	logger zerolog.Logger,
) *RelationshipStage {
	detector := relationship.NewDetector(relRepo)
	return &RelationshipStage{
		docRepo:     docRepo,
		relRepo:     relRepo,
		metaRepo:    metaRepo,
		projectRepo: projectRepo,
		vectorStore: vectorStore,
		embedder:    embedder,
		detector:    detector,
		logger:      logger.With().Str("stage", "relationship").Logger(),
		useRAG:      true,
	}
}

// Name returns the stage name.
func (s *RelationshipStage) Name() string {
	return "relationship"
}

// CanProcess returns true if this stage can process the file entry.
func (s *RelationshipStage) CanProcess(entry *entity.FileEntry) bool {
	// Only process Markdown files and documents with mirrors
	if entry.Extension == ".md" {
		return true
	}
	// Check if document has a mirror (extracted content)
	return entry.Enhanced != nil && entry.Enhanced.IndexedState.Mirror
}

// Process detects and creates relationships for a document.
func (s *RelationshipStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}

	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		return fmt.Errorf("workspace info not found in context")
	}
	workspaceID := wsInfo.ID

	// Get document
	doc, err := s.docRepo.GetDocumentByPath(ctx, workspaceID, entry.RelativePath)
	if err != nil || doc == nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// Read document content
	contentPath := entry.AbsolutePath
	if entry.Extension != ".md" {
		// Try to use mirror if available
		// This would require metadata lookup - simplified for now
		contentPath = entry.AbsolutePath
	}

	content, err := os.ReadFile(contentPath)
	if err != nil {
		return fmt.Errorf("failed to read document: %w", err)
	}

	// Detect relationships from frontmatter
	var relationships []*entity.DocumentRelationship
	if doc.Frontmatter != nil {
		relationships = append(relationships, s.detector.DetectFromFrontmatter(doc, doc.Frontmatter)...)
	}

	// Detect relationships from content (Markdown links)
	contentRels := s.detector.DetectFromContent(doc, string(content))
	relationships = append(relationships, contentRels...)

	// Resolve relationships (convert paths to DocumentIDs)
	resolvePath := func(relativePath string) (entity.DocumentID, error) {
		targetDoc, err := s.docRepo.GetDocumentByPath(ctx, workspaceID, relativePath)
		if err != nil || targetDoc == nil {
			return "", fmt.Errorf("document not found: %s", relativePath)
		}
		return targetDoc.ID, nil
	}

	resolved, err := s.detector.ResolveRelationships(relationships, entry.RelativePath, resolvePath)
	if err != nil {
		s.logger.Warn().Err(err).Str("path", entry.RelativePath).Msg("Failed to resolve some relationships")
	}

	// If RAG is enabled, detect additional relationships based on semantic similarity
	if s.useRAG && s.embedder != nil && s.vectorStore != nil {
		ragRels := s.detectRelationshipsWithRAG(ctx, workspaceID, doc, entry)
		resolved = append(resolved, ragRels...)
	}

	// Detect relationships based on shared projects
	if s.useRAG && s.projectRepo != nil {
		projectRels := s.detectRelationshipsFromProjects(ctx, workspaceID, doc.ID)
		resolved = append(resolved, projectRels...)
	}

	// Detect relationships based on shared tags
	if s.useRAG && s.metaRepo != nil {
		tagRels := s.detectRelationshipsFromTags(ctx, workspaceID, entry.RelativePath, doc.ID)
		resolved = append(resolved, tagRels...)
	}

	// Create relationships in database
	for _, rel := range resolved {
		rel.WorkspaceID = workspaceID
		if err := s.relRepo.Create(ctx, workspaceID, rel); err != nil {
			// Log but don't fail - relationship might already exist
			s.logger.Debug().Err(err).
				Str("from", rel.FromDocument.String()).
				Str("to", rel.ToDocument.String()).
				Str("type", rel.Type.String()).
				Msg("Failed to create relationship (may already exist)")
		}
	}

	s.logger.Info().
		Str("path", entry.RelativePath).
		Int("relationships", len(resolved)).
		Msg("Relationship detection complete")

	return nil
}

// detectRelationshipsWithRAG detects relationships based on semantic similarity using RAG.
func (s *RelationshipStage) detectRelationshipsWithRAG(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	doc *entity.Document,
	entry *entity.FileEntry,
) []*entity.DocumentRelationship {
	if s.embedder == nil || s.vectorStore == nil {
		return nil
	}

	// Get document chunks to create embedding
	chunks, err := s.docRepo.GetChunksByDocument(ctx, workspaceID, doc.ID)
	if err != nil || len(chunks) == 0 {
		return nil
	}

	// Use first chunk as representative (or combine multiple chunks)
	text := chunks[0].Text
	if len(text) > 4000 {
		text = text[:4000] // Limit text length for embedding
	}

	// Create embedding for this document
	vector, err := s.embedder.Embed(ctx, text)
	if err != nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("Failed to create embedding for relationship detection")
		return nil
	}

	// Search for similar documents (excluding self)
	matches, err := s.vectorStore.Search(ctx, workspaceID, vector, 10)
	if err != nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("Failed to search vector store for relationships")
		return nil
	}

	var relationships []*entity.DocumentRelationship
	for _, match := range matches {
		// Get chunk to find document ID
		chunks, err := s.docRepo.GetChunksByIDs(ctx, workspaceID, []entity.ChunkID{match.ChunkID})
		if err != nil || len(chunks) == 0 {
			continue
		}
		otherDocID := chunks[0].DocumentID

		// Skip self
		if otherDocID == doc.ID {
			continue
		}

		// Only create relationships for high similarity (threshold: 0.6)
		if match.Similarity < 0.6 {
			continue
		}

		rel := &entity.DocumentRelationship{
			FromDocument: doc.ID,
			ToDocument:   otherDocID,
			Type:         entity.RelationshipReferences,
			Strength:     float64(match.Similarity),
		}
		relationships = append(relationships, rel)
	}

	if len(relationships) > 0 {
		s.logger.Info().
			Str("path", entry.RelativePath).
			Int("rag_relationships", len(relationships)).
			Msg("Detected relationships using RAG")
	}

	return relationships
}

// detectRelationshipsFromProjects detects relationships based on shared projects.
func (s *RelationshipStage) detectRelationshipsFromProjects(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) []*entity.DocumentRelationship {
	// Get projects for this document
	projectIDs, err := s.projectRepo.GetProjectsForDocument(ctx, workspaceID, docID)
	if err != nil || len(projectIDs) == 0 {
		return nil
	}

	var relationships []*entity.DocumentRelationship

	// For each project, find other documents
	for _, projectID := range projectIDs {
		docIDs, err := s.projectRepo.GetDocuments(ctx, workspaceID, projectID, false)
		if err != nil {
			continue
		}

		for _, otherDocID := range docIDs {
			// Skip self
			if otherDocID == docID {
				continue
			}

			// Check if relationship already exists
			existing, _ := s.relRepo.GetOutgoing(ctx, workspaceID, docID, entity.RelationshipReferences)
			alreadyExists := false
			for _, rel := range existing {
				if rel.ToDocument == otherDocID {
					alreadyExists = true
					break
				}
			}
			if alreadyExists {
				continue
			}

			rel := &entity.DocumentRelationship{
				FromDocument: docID,
				ToDocument:   otherDocID,
				Type:         entity.RelationshipReferences,
				Strength:     0.7, // Medium-high strength for shared projects
			}
			relationships = append(relationships, rel)
		}
	}

	if len(relationships) > 0 {
		s.logger.Info().
			Str("doc_id", docID.String()).
			Int("project_relationships", len(relationships)).
			Msg("Detected relationships from shared projects")
	}

	return relationships
}

// detectRelationshipsFromTags detects relationships based on shared tags.
func (s *RelationshipStage) detectRelationshipsFromTags(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	relativePath string,
	docID entity.DocumentID,
) []*entity.DocumentRelationship {
	// Get metadata for this document
	meta, err := s.metaRepo.GetByPath(ctx, workspaceID, relativePath)
	if err != nil || meta == nil || len(meta.Tags) == 0 {
		return nil
	}

	// Find other documents with shared tags
	// For performance, we'll use a simpler approach: get documents from vector store matches
	// or iterate through known documents. For now, we'll use a limited approach.
	// Get chunks to find similar documents via vector store
	chunks, err := s.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
	if err != nil || len(chunks) == 0 {
		return nil
	}

	// Use first chunk to find similar documents
	text := chunks[0].Text
	if len(text) > 4000 {
		text = text[:4000]
	}

	vector, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return nil
	}

	// Search for similar documents
	matches, err := s.vectorStore.Search(ctx, workspaceID, vector, 20) // Get more matches for tag comparison
	if err != nil {
		return nil
	}

	var relationships []*entity.DocumentRelationship
	tagMap := make(map[string]bool)
	for _, tag := range meta.Tags {
		tagMap[strings.ToLower(tag)] = true
	}

		// Check each match for shared tags
		for _, match := range matches {
			// Get chunk to find document ID
			chunks, err := s.docRepo.GetChunksByIDs(ctx, workspaceID, []entity.ChunkID{match.ChunkID})
			if err != nil || len(chunks) == 0 {
				continue
			}
			otherDocID := chunks[0].DocumentID

			// Skip self
			if otherDocID == docID {
				continue
			}

			// Get document to get path
			otherDoc, err := s.docRepo.GetDocument(ctx, workspaceID, otherDocID)
			if err != nil || otherDoc == nil {
				continue
			}

			// Get metadata for other document
			otherMeta, err := s.metaRepo.GetByPath(ctx, workspaceID, otherDoc.RelativePath)
			if err != nil || otherMeta == nil || len(otherMeta.Tags) == 0 {
				continue
			}

			// Count shared tags
			sharedCount := 0
			for _, otherTag := range otherMeta.Tags {
				if tagMap[strings.ToLower(otherTag)] {
					sharedCount++
				}
			}

		// Create relationship if at least 1 tag is shared (lower threshold for better detection)
		if sharedCount >= 1 {
			// Check if relationship already exists
			existing, _ := s.relRepo.GetOutgoing(ctx, workspaceID, docID, entity.RelationshipReferences)
			alreadyExists := false
			for _, rel := range existing {
				if rel.ToDocument == otherDocID {
					alreadyExists = true
					break
				}
			}
			if alreadyExists {
				continue
			}

			// Strength based on number of shared tags (max 0.6 for tags alone)
			strength := 0.4 + float64(sharedCount)*0.1
			if strength > 0.6 {
				strength = 0.6
			}

			rel := &entity.DocumentRelationship{
				FromDocument: docID,
				ToDocument:   otherDocID,
				Type:         entity.RelationshipReferences,
				Strength:     strength,
			}
				relationships = append(relationships, rel)
			}
		}

	if len(relationships) > 0 {
		s.logger.Info().
			Str("path", relativePath).
			Int("tag_relationships", len(relationships)).
			Msg("Detected relationships from shared tags")
	}

	return relationships
}

