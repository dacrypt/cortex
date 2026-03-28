package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// EnrichmentRepository handles denormalized enrichment data.
type EnrichmentRepository struct {
	conn *Connection
}

// NewEnrichmentRepository creates a new enrichment repository.
func NewEnrichmentRepository(conn *Connection) *EnrichmentRepository {
	return &EnrichmentRepository{conn: conn}
}

// SyncEnrichmentData syncs enrichment data JSON to normalized tables.
func (r *EnrichmentRepository) SyncEnrichmentData(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, enrichment *entity.EnrichmentData) error {
	if enrichment == nil {
		return nil
	}

	// Delete existing entries for this file
	if err := r.DeleteByFile(ctx, workspaceID, fileID); err != nil {
		return fmt.Errorf("failed to delete existing enrichment data: %w", err)
	}

	now := time.Now().UnixMilli()

	// Sync named entities
	for _, entity := range enrichment.NamedEntities {
		if err := r.insertNamedEntity(ctx, workspaceID, fileID, entity, now); err != nil {
			return fmt.Errorf("failed to insert named entity: %w", err)
		}
	}

	// Sync citations
	for _, citation := range enrichment.Citations {
		if err := r.insertCitation(ctx, workspaceID, fileID, citation, now); err != nil {
			return fmt.Errorf("failed to insert citation: %w", err)
		}
	}

	// Sync dependencies
	for _, dep := range enrichment.Dependencies {
		if err := r.insertDependency(ctx, workspaceID, fileID, dep, now); err != nil {
			return fmt.Errorf("failed to insert dependency: %w", err)
		}
	}

	// Sync duplicates
	for _, dup := range enrichment.Duplicates {
		if err := r.insertDuplicate(ctx, workspaceID, fileID, dup, now); err != nil {
			return fmt.Errorf("failed to insert duplicate: %w", err)
		}
	}

	// Sync sentiment
	if enrichment.Sentiment != nil {
		if err := r.insertSentiment(ctx, workspaceID, fileID, enrichment.Sentiment, now); err != nil {
			return fmt.Errorf("failed to insert sentiment: %w", err)
		}
	}

	return nil
}

// DeleteByFile deletes all enrichment entries for a file.
func (r *EnrichmentRepository) DeleteByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	tables := []string{
		"file_named_entities",
		"file_citations",
		"file_dependencies",
		"file_duplicates",
		"file_sentiment",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE workspace_id = ? AND file_id = ?", table)
		if _, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String()); err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
	}

	return nil
}

func (r *EnrichmentRepository) insertNamedEntity(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, entity entity.NamedEntity, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_named_entities (id, workspace_id, file_id, text, type, start_pos, end_pos, confidence, context, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		entity.Text,
		entity.Type,
		entity.StartPos,
		entity.EndPos,
		entity.Confidence,
		entity.Context,
		createdAt,
	)
	return err
}

func (r *EnrichmentRepository) insertCitation(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, citation entity.Citation, createdAt int64) error {
	id := uuid.New().String()
	authorsJSON, _ := json.Marshal(citation.Authors)
	authorsStr := string(authorsJSON)
	
	var yearStr *string
	if citation.Year != nil {
		yearStrVal := fmt.Sprintf("%d", *citation.Year)
		yearStr = &yearStrVal
	}

	query := `
		INSERT INTO file_citations (id, workspace_id, file_id, text, authors, title, year, doi, url, type, confidence, page, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		citation.Text,
		authorsStr,
		citation.Title,
		yearStr,
		citation.DOI,
		citation.URL,
		citation.Type,
		citation.Confidence,
		citation.Page,
		createdAt,
	)
	return err
}

func (r *EnrichmentRepository) insertDependency(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, dep entity.Dependency, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_dependencies (id, workspace_id, file_id, name, version, type, language, path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, name, type) DO UPDATE SET
			version = excluded.version,
			language = excluded.language,
			path = excluded.path
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		dep.Name,
		dep.Version,
		dep.Type,
		dep.Language,
		dep.Path,
		createdAt,
	)
	return err
}

func (r *EnrichmentRepository) insertDuplicate(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, dup entity.DuplicateInfo, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_duplicates (id, workspace_id, file_id, duplicate_file_id, similarity, type, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, duplicate_file_id) DO UPDATE SET
			similarity = excluded.similarity,
			type = excluded.type,
			reason = excluded.reason
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		dup.DocumentID.String(), // Using DocumentID as file_id reference
		dup.Similarity,
		dup.Type,
		dup.Reason,
		createdAt,
	)
	return err
}

func (r *EnrichmentRepository) insertSentiment(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, sentiment *entity.SentimentAnalysis, createdAt int64) error {
	id := uuid.New().String()
	emotionsJSON, _ := json.Marshal(sentiment.Emotions)
	emotionsStr := string(emotionsJSON)

	query := `
		INSERT INTO file_sentiment (id, workspace_id, file_id, overall_sentiment, score, confidence, emotions_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id) DO UPDATE SET
			overall_sentiment = excluded.overall_sentiment,
			score = excluded.score,
			confidence = excluded.confidence,
			emotions_json = excluded.emotions_json
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		sentiment.OverallSentiment,
		sentiment.Score,
		sentiment.Confidence,
		emotionsStr,
		createdAt,
	)
	return err
}

