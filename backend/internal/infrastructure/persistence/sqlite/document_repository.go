package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DocumentRepository implements repository.DocumentRepository using SQLite.
type DocumentRepository struct {
	conn *Connection
}

// NewDocumentRepository creates a new SQLite document repository.
func NewDocumentRepository(conn *Connection) *DocumentRepository {
	return &DocumentRepository{conn: conn}
}

// UpsertDocument inserts or updates a document record.
func (r *DocumentRepository) UpsertDocument(ctx context.Context, workspaceID entity.WorkspaceID, doc *entity.Document) error {
	var frontmatterJSON []byte
	if doc.Frontmatter != nil {
		data, err := json.Marshal(doc.Frontmatter)
		if err != nil {
			return fmt.Errorf("failed to marshal frontmatter: %w", err)
		}
		frontmatterJSON = data
	}

	var stateChangedAt interface{}
	if doc.StateChangedAt != nil {
		stateChangedAt = doc.StateChangedAt.UnixMilli()
	} else {
		stateChangedAt = nil
	}

	// Default state to draft if not set
	state := doc.State
	if state == "" {
		state = entity.DocumentStateDraft
	}

	query := `
		INSERT INTO documents (
			id, workspace_id, file_id, relative_path, title, frontmatter,
			checksum, state, state_changed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, id) DO UPDATE SET
			file_id = excluded.file_id,
			relative_path = excluded.relative_path,
			title = excluded.title,
			frontmatter = excluded.frontmatter,
			checksum = excluded.checksum,
			state = excluded.state,
			state_changed_at = excluded.state_changed_at,
			updated_at = excluded.updated_at
	`

	_, err := r.conn.Exec(ctx, query,
		doc.ID.String(),
		workspaceID.String(),
		doc.FileID.String(),
		doc.RelativePath,
		doc.Title,
		frontmatterJSON,
		doc.Checksum,
		state.String(),
		stateChangedAt,
		doc.CreatedAt.UnixMilli(),
		doc.UpdatedAt.UnixMilli(),
	)
	return err
}

// GetDocument retrieves a document by ID.
func (r *DocumentRepository) GetDocument(ctx context.Context, workspaceID entity.WorkspaceID, id entity.DocumentID) (*entity.Document, error) {
	query := `
		SELECT id, file_id, relative_path, title, frontmatter, checksum, state, state_changed_at, created_at, updated_at
		FROM documents
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return scanDocument(row)
}

// GetDocumentByPath retrieves a document by relative path.
func (r *DocumentRepository) GetDocumentByPath(ctx context.Context, workspaceID entity.WorkspaceID, relativePath string) (*entity.Document, error) {
	query := `
		SELECT id, file_id, relative_path, title, frontmatter, checksum, state, state_changed_at, created_at, updated_at
		FROM documents
		WHERE workspace_id = ? AND relative_path = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), relativePath)
	return scanDocument(row)
}

// ReplaceChunks replaces all chunks for a document.
func (r *DocumentRepository) ReplaceChunks(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID, chunks []*entity.Chunk) error {
	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM chunks WHERE workspace_id = ? AND document_id = ?`,
			workspaceID.String(),
			documentID.String(),
		); err != nil {
			return err
		}

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO chunks (
				id, workspace_id, document_id, ordinal, heading, heading_path, text,
				token_count, start_line, end_line, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, chunk := range chunks {
			if _, err := stmt.ExecContext(ctx,
				chunk.ID.String(),
				workspaceID.String(),
				chunk.DocumentID.String(),
				chunk.Ordinal,
				chunk.Heading,
				chunk.HeadingPath,
				chunk.Text,
				chunk.TokenCount,
				chunk.StartLine,
				chunk.EndLine,
				chunk.CreatedAt.UnixMilli(),
				chunk.UpdatedAt.UnixMilli(),
			); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetChunksByDocument retrieves chunks for a document.
func (r *DocumentRepository) GetChunksByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.Chunk, error) {
	query := `
		SELECT id, document_id, ordinal, heading, heading_path, text, token_count,
		       start_line, end_line, created_at, updated_at
		FROM chunks
		WHERE workspace_id = ? AND document_id = ?
		ORDER BY ordinal ASC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), documentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunks(rows)
}

// GetChunksByIDs retrieves chunks by ID.
func (r *DocumentRepository) GetChunksByIDs(ctx context.Context, workspaceID entity.WorkspaceID, ids []entity.ChunkID) ([]*entity.Chunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, workspaceID.String())
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id.String())
	}

	query := fmt.Sprintf(`
		SELECT id, document_id, ordinal, heading, heading_path, text, token_count,
		       start_line, end_line, created_at, updated_at
		FROM chunks
		WHERE workspace_id = ? AND id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChunks(rows)
}

func scanDocument(row *sql.Row) (*entity.Document, error) {
	var (
		id            string
		fileID        string
		relativePath  string
		title         string
		frontmatter   sql.NullString
		checksum      string
		state         string
		stateChangedAt sql.NullInt64
		createdAt     int64
		updatedAt     int64
	)

	if err := row.Scan(&id, &fileID, &relativePath, &title, &frontmatter, &checksum, &state, &stateChangedAt, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var frontmatterMap map[string]interface{}
	if frontmatter.Valid && frontmatter.String != "" {
		_ = json.Unmarshal([]byte(frontmatter.String), &frontmatterMap)
	}

	doc := &entity.Document{
		ID:           entity.DocumentID(id),
		FileID:       entity.FileID(fileID),
		RelativePath: relativePath,
		Title:        title,
		Frontmatter:  frontmatterMap,
		Checksum:     checksum,
		State:        entity.DocumentState(state),
		CreatedAt:    time.UnixMilli(createdAt),
		UpdatedAt:    time.UnixMilli(updatedAt),
	}

	if stateChangedAt.Valid {
		t := time.UnixMilli(stateChangedAt.Int64)
		doc.StateChangedAt = &t
	}

	return doc, nil
}

func scanChunks(rows *sql.Rows) ([]*entity.Chunk, error) {
	var chunks []*entity.Chunk
	for rows.Next() {
		var (
			id          string
			documentID  string
			ordinal     int
			heading     string
			headingPath string
			text        string
			tokenCount  int
			startLine   int
			endLine     int
			createdAt   int64
			updatedAt   int64
		)

		if err := rows.Scan(
			&id,
			&documentID,
			&ordinal,
			&heading,
			&headingPath,
			&text,
			&tokenCount,
			&startLine,
			&endLine,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		chunks = append(chunks, &entity.Chunk{
			ID:          entity.ChunkID(id),
			DocumentID:  entity.DocumentID(documentID),
			Ordinal:     ordinal,
			Heading:     heading,
			HeadingPath: headingPath,
			Text:        text,
			TokenCount:  tokenCount,
			StartLine:   startLine,
			EndLine:     endLine,
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

var _ repository.DocumentRepository = (*DocumentRepository)(nil)
