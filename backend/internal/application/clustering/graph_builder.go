// Package clustering provides document clustering and community detection services.
package clustering

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// GraphBuilderConfig contains configuration for the graph builder.
type GraphBuilderConfig struct {
	// Weight factors for different edge sources (must sum to 1.0)
	SemanticWeight   float64 // Weight for semantic similarity (default: 0.4)
	EntityWeight     float64 // Weight for shared entities (default: 0.3)
	TemporalWeight   float64 // Weight for temporal co-occurrence (default: 0.2)
	StructuralWeight float64 // Weight for folder proximity (default: 0.1)

	// Thresholds
	MinSemanticSimilarity float64 // Minimum cosine similarity to create edge (default: 0.5)
	MinEdgeWeight         float64 // Minimum combined weight to keep edge (default: 0.2)
	MaxEdgesPerDocument   int     // Maximum edges per document (default: 50)

	// Temporal settings
	TemporalWindowHours int // Time window for co-occurrence (default: 24)
}

// DefaultGraphBuilderConfig returns the default configuration.
func DefaultGraphBuilderConfig() GraphBuilderConfig {
	return GraphBuilderConfig{
		SemanticWeight:        0.4,
		EntityWeight:          0.3,
		TemporalWeight:        0.2,
		StructuralWeight:      0.1,
		MinSemanticSimilarity: 0.5,
		MinEdgeWeight:         0.2,
		MaxEdgesPerDocument:   50,
		TemporalWindowHours:   24,
	}
}

// GraphBuilder builds document graphs from various similarity sources.
type GraphBuilder struct {
	docRepo     repository.DocumentRepository
	fileRepo    repository.FileRepository
	entityRepo  repository.EntityRepository
	usageRepo   repository.UsageRepository
	clusterRepo repository.ClusterRepository
	conn        EmbeddingConnection
	config      GraphBuilderConfig
	logger      zerolog.Logger
}

// EmbeddingConnection provides access to raw embeddings for pairwise comparison.
type EmbeddingConnection interface {
	GetAllEmbeddings(ctx context.Context, workspaceID entity.WorkspaceID) ([]DocumentEmbedding, error)
}

// DocumentEmbedding represents a document's embedding vector.
type DocumentEmbedding struct {
	DocumentID entity.DocumentID
	ChunkID    entity.ChunkID
	Vector     []float32
}

// NewGraphBuilder creates a new graph builder.
func NewGraphBuilder(
	docRepo repository.DocumentRepository,
	fileRepo repository.FileRepository,
	entityRepo repository.EntityRepository,
	usageRepo repository.UsageRepository,
	clusterRepo repository.ClusterRepository,
	conn EmbeddingConnection,
	config GraphBuilderConfig,
	logger zerolog.Logger,
) *GraphBuilder {
	return &GraphBuilder{
		docRepo:     docRepo,
		fileRepo:    fileRepo,
		entityRepo:  entityRepo,
		usageRepo:   usageRepo,
		clusterRepo: clusterRepo,
		conn:        conn,
		config:      config,
		logger:      logger.With().Str("component", "graph_builder").Logger(),
	}
}

// BuildGraph builds a document graph for a workspace.
func (b *GraphBuilder) BuildGraph(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.DocumentGraph, error) {
	b.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Msg("Starting graph build")

	graph := entity.NewDocumentGraph(workspaceID)

	// Step 1: Build semantic edges from embeddings
	semanticEdges, err := b.buildSemanticEdges(ctx, workspaceID)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to build semantic edges")
	} else {
		for _, edge := range semanticEdges {
			graph.AddEdge(edge)
		}
		b.logger.Info().Int("count", len(semanticEdges)).Msg("Added semantic edges")
	}

	// Step 2: Add entity-based edges
	entityEdges, err := b.buildEntityEdges(ctx, workspaceID, graph)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to build entity edges")
	} else {
		for _, edge := range entityEdges {
			graph.AddEdge(edge)
		}
		b.logger.Info().Int("count", len(entityEdges)).Msg("Added entity edges")
	}

	// Step 3: Add temporal edges
	temporalEdges, err := b.buildTemporalEdges(ctx, workspaceID, graph)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to build temporal edges")
	} else {
		for _, edge := range temporalEdges {
			graph.AddEdge(edge)
		}
		b.logger.Info().Int("count", len(temporalEdges)).Msg("Added temporal edges")
	}

	// Step 4: Add structural edges
	structuralEdges, err := b.buildStructuralEdges(ctx, workspaceID, graph)
	if err != nil {
		b.logger.Warn().Err(err).Msg("Failed to build structural edges")
	} else {
		for _, edge := range structuralEdges {
			graph.AddEdge(edge)
		}
		b.logger.Info().Int("count", len(structuralEdges)).Msg("Added structural edges")
	}

	// Step 5: Prune weak edges
	b.pruneWeakEdges(graph)

	b.logger.Info().
		Int("nodes", graph.NodeCount()).
		Int("edges", graph.EdgeCount()).
		Msg("Graph build complete")

	return graph, nil
}

// buildSemanticEdges creates edges based on embedding similarity.
func (b *GraphBuilder) buildSemanticEdges(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentEdge, error) {
	if b.conn == nil {
		return nil, nil
	}

	embeddings, err := b.conn.GetAllEmbeddings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if len(embeddings) < 2 {
		return nil, nil
	}

	// Group embeddings by document (average chunks)
	docEmbeddings := b.aggregateDocumentEmbeddings(embeddings)

	b.logger.Debug().
		Int("documents", len(docEmbeddings)).
		Msg("Computing pairwise similarities")

	var edges []*entity.DocumentEdge
	docIDs := make([]entity.DocumentID, 0, len(docEmbeddings))
	for docID := range docEmbeddings {
		docIDs = append(docIDs, docID)
	}
	sort.Slice(docIDs, func(i, j int) bool {
		return string(docIDs[i]) < string(docIDs[j])
	})

	// Compute pairwise similarities
	for i := 0; i < len(docIDs); i++ {
		for j := i + 1; j < len(docIDs); j++ {
			docA := docIDs[i]
			docB := docIDs[j]

			sim := cosineSimilarity(docEmbeddings[docA], docEmbeddings[docB])
			if sim >= float32(b.config.MinSemanticSimilarity) {
				edge := entity.NewDocumentEdge(docA, docB, workspaceID)
				edge.AddSource(entity.EdgeSource{
					Type:   entity.EdgeSourceSemantic,
					Weight: float64(sim) * b.config.SemanticWeight,
					Detail: "",
				})
				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

// aggregateDocumentEmbeddings averages chunk embeddings per document.
func (b *GraphBuilder) aggregateDocumentEmbeddings(embeddings []DocumentEmbedding) map[entity.DocumentID][]float32 {
	// Group by document
	docChunks := make(map[entity.DocumentID][][]float32)
	for _, emb := range embeddings {
		docChunks[emb.DocumentID] = append(docChunks[emb.DocumentID], emb.Vector)
	}

	// Average vectors per document
	result := make(map[entity.DocumentID][]float32)
	for docID, chunks := range docChunks {
		if len(chunks) == 0 {
			continue
		}

		dims := len(chunks[0])
		avg := make([]float32, dims)
		for _, chunk := range chunks {
			for i := 0; i < dims && i < len(chunk); i++ {
				avg[i] += chunk[i]
			}
		}
		for i := range avg {
			avg[i] /= float32(len(chunks))
		}
		result[docID] = avg
	}

	return result
}

// buildEntityEdges creates edges based on shared named entities.
// Note: This is a simplified implementation. A full implementation would
// query a dedicated named entity repository.
func (b *GraphBuilder) buildEntityEdges(ctx context.Context, workspaceID entity.WorkspaceID, graph *entity.DocumentGraph) ([]*entity.DocumentEdge, error) {
	// Entity-based edges are optional and require a dedicated NER extraction stage.
	// For now, we skip this and rely on semantic + temporal + structural edges.
	// TODO: Implement when entity extraction is available in the pipeline.
	return nil, nil
}

// buildTemporalEdges creates edges based on temporal co-occurrence.
// It iterates over recently used documents and builds edges from their co-occurrence patterns.
func (b *GraphBuilder) buildTemporalEdges(ctx context.Context, workspaceID entity.WorkspaceID, graph *entity.DocumentGraph) ([]*entity.DocumentEdge, error) {
	if b.usageRepo == nil {
		return nil, nil
	}

	since := time.Now().Add(-time.Duration(b.config.TemporalWindowHours) * time.Hour)

	// Get recently used documents to seed the co-occurrence query
	recentDocs, err := b.usageRepo.GetRecentlyUsed(ctx, workspaceID, 100)
	if err != nil {
		return nil, err
	}

	if len(recentDocs) < 2 {
		return nil, nil
	}

	var edges []*entity.DocumentEdge
	seen := make(map[string]bool)

	// For each recently used document, get its co-occurrences
	for _, docID := range recentDocs {
		coOccurrences, err := b.usageRepo.GetCoOccurrences(ctx, workspaceID, docID, b.config.MaxEdgesPerDocument, since)
		if err != nil {
			b.logger.Debug().Err(err).Str("doc_id", docID.String()).Msg("Failed to get co-occurrences")
			continue
		}

		for otherDocID, count := range coOccurrences {
			if count < 2 {
				continue
			}

			// Avoid duplicate edges
			key := edgeKey(docID, otherDocID)
			if seen[key] {
				continue
			}
			seen[key] = true

			// Normalize co-occurrence count to [0,1]
			normalizedWeight := math.Min(float64(count)/10.0, 1.0) * b.config.TemporalWeight

			edge := entity.NewDocumentEdge(docID, otherDocID, workspaceID)
			edge.AddSource(entity.EdgeSource{
				Type:   entity.EdgeSourceTemporal,
				Weight: normalizedWeight,
				Detail: "", // No specific time window info from this interface
			})
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

// buildStructuralEdges creates edges based on folder proximity.
func (b *GraphBuilder) buildStructuralEdges(ctx context.Context, workspaceID entity.WorkspaceID, graph *entity.DocumentGraph) ([]*entity.DocumentEdge, error) {
	if b.docRepo == nil {
		return nil, nil
	}

	// Get all documents with their paths
	docs, err := b.getAllDocuments(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Group documents by folder
	folderDocs := make(map[string][]entity.DocumentID)
	for _, doc := range docs {
		folder := filepath.Dir(doc.RelativePath)
		folderDocs[folder] = append(folderDocs[folder], doc.ID)
	}

	var edges []*entity.DocumentEdge
	seen := make(map[string]bool)

	for folder, docIDs := range folderDocs {
		if len(docIDs) < 2 {
			continue
		}

		// Create edges between documents in the same folder
		for i := 0; i < len(docIDs); i++ {
			for j := i + 1; j < len(docIDs); j++ {
				edgeKey := edgeKey(docIDs[i], docIDs[j])
				if seen[edgeKey] {
					continue
				}
				seen[edgeKey] = true

				edge := entity.NewDocumentEdge(docIDs[i], docIDs[j], workspaceID)
				edge.AddSource(entity.EdgeSource{
					Type:   entity.EdgeSourceStructural,
					Weight: b.config.StructuralWeight,
					Detail: folder,
				})
				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

// getAllDocuments retrieves all documents for a workspace.
func (b *GraphBuilder) getAllDocuments(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.Document, error) {
	if b.fileRepo == nil || b.docRepo == nil {
		return nil, nil
	}

	// Get all files from the workspace
	files, err := b.fileRepo.List(ctx, workspaceID, repository.DefaultFileListOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Get documents by path
	var documents []*entity.Document
	for _, file := range files {
		doc, err := b.docRepo.GetDocumentByPath(ctx, workspaceID, file.RelativePath)
		if err != nil {
			// Document might not exist yet, skip
			continue
		}
		if doc != nil {
			documents = append(documents, doc)
		}
	}

	b.logger.Debug().
		Int("files", len(files)).
		Int("documents", len(documents)).
		Msg("Retrieved documents for graph building")

	return documents, nil
}

// pruneWeakEdges removes edges below the minimum weight threshold.
func (b *GraphBuilder) pruneWeakEdges(graph *entity.DocumentGraph) {
	toDelete := []string{}
	for key, edge := range graph.Edges {
		if edge.Weight < b.config.MinEdgeWeight {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(graph.Edges, key)
	}

	b.logger.Debug().
		Int("pruned", len(toDelete)).
		Msg("Pruned weak edges")
}

// PersistGraph saves the graph to the repository.
func (b *GraphBuilder) PersistGraph(ctx context.Context, graph *entity.DocumentGraph) error {
	if b.clusterRepo == nil {
		return nil
	}

	edges := make([]*entity.DocumentEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges = append(edges, edge)
	}

	return b.clusterRepo.UpsertEdgesBatch(ctx, edges)
}

// Helper functions

func edgeKey(a, b entity.DocumentID) string {
	if string(a) < string(b) {
		return string(a) + "|" + string(b)
	}
	return string(b) + "|" + string(a)
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(normA*normB)))
}

// encodeVector converts a float32 slice to bytes.
func encodeVector(vector []float32) []byte {
	buf := make([]byte, 4*len(vector))
	for i, v := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// decodeVector is now in embedding_connection.go to avoid duplication

// Entity represents a named entity extracted from a document.
type Entity struct {
	DocumentID entity.DocumentID
	Type       string
	Text       string
	Confidence float64
}

// CoOccurrence represents temporal co-occurrence between documents.
type CoOccurrence struct {
	DocA       entity.DocumentID
	DocB       entity.DocumentID
	Count      int
	TimeWindow string
}
