package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// AIContextRepository handles denormalized AI context data.
type AIContextRepository struct {
	conn *Connection
}

// NewAIContextRepository creates a new AI context repository.
func NewAIContextRepository(conn *Connection) *AIContextRepository {
	return &AIContextRepository{conn: conn}
}

// SyncAIContext syncs AI context JSON to normalized tables.
func (r *AIContextRepository) SyncAIContext(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, aiContext *entity.AIContext) error {
	if aiContext == nil {
		return nil
	}

	// Delete existing entries for this file
	if err := r.DeleteByFile(ctx, workspaceID, fileID); err != nil {
		return fmt.Errorf("failed to delete existing AI context: %w", err)
	}

	now := time.Now().UnixMilli()

	// Sync authors
	for _, author := range aiContext.Authors {
		if err := r.insertAuthor(ctx, workspaceID, fileID, author, now); err != nil {
			return fmt.Errorf("failed to insert author: %w", err)
		}
	}

	// Sync locations
	for _, location := range aiContext.Locations {
		if err := r.insertLocation(ctx, workspaceID, fileID, location, now); err != nil {
			return fmt.Errorf("failed to insert location: %w", err)
		}
	}

	// Sync people
	for _, person := range aiContext.PeopleMentioned {
		if err := r.insertPerson(ctx, workspaceID, fileID, person, now); err != nil {
			return fmt.Errorf("failed to insert person: %w", err)
		}
	}

	// Sync organizations
	for _, org := range aiContext.Organizations {
		if err := r.insertOrganization(ctx, workspaceID, fileID, org, now); err != nil {
			return fmt.Errorf("failed to insert organization: %w", err)
		}
	}

	// Sync events
	for _, event := range aiContext.HistoricalEvents {
		if err := r.insertEvent(ctx, workspaceID, fileID, event, now); err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	// Sync references
	for _, ref := range aiContext.References {
		if err := r.insertReference(ctx, workspaceID, fileID, ref, now); err != nil {
			return fmt.Errorf("failed to insert reference: %w", err)
		}
	}

	// Sync publication info
	if aiContext.Publisher != nil || aiContext.PublicationYear != nil || aiContext.ISBN != nil {
		if err := r.insertPublicationInfo(ctx, workspaceID, fileID, aiContext, now); err != nil {
			return fmt.Errorf("failed to insert publication info: %w", err)
		}
	}

	return nil
}

// DeleteByFile deletes all AI context entries for a file.
func (r *AIContextRepository) DeleteByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) error {
	tables := []string{
		"file_authors",
		"file_locations",
		"file_people",
		"file_organizations",
		"file_events",
		"file_references",
		"file_publication_info",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE workspace_id = ? AND file_id = ?", table)
		if _, err := r.conn.Exec(ctx, query, workspaceID.String(), fileID.String()); err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
	}

	return nil
}

func (r *AIContextRepository) insertAuthor(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, author entity.AuthorInfo, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_authors (id, workspace_id, file_id, name, role, affiliation, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, name) DO UPDATE SET
			role = excluded.role,
			affiliation = excluded.affiliation,
			confidence = excluded.confidence
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		author.Name,
		author.Role,
		author.Affiliation,
		author.Confidence,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertLocation(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, location entity.LocationInfo, createdAt int64) error {
	id := uuid.New().String()
	var coordinatesJSON []byte
	if location.Coordinates != nil {
		coordinatesJSON, _ = json.Marshal(location.Coordinates)
	}
	coordinatesStr := string(coordinatesJSON)

	query := `
		INSERT INTO file_locations (id, workspace_id, file_id, name, type, coordinates, context, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, name) DO UPDATE SET
			type = excluded.type,
			coordinates = excluded.coordinates,
			context = excluded.context
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		location.Name,
		location.Type,
		coordinatesStr,
		location.Context,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertPerson(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, person entity.PersonInfo, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_people (id, workspace_id, file_id, name, role, context, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, name) DO UPDATE SET
			role = excluded.role,
			context = excluded.context,
			confidence = excluded.confidence
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		person.Name,
		person.Role,
		person.Context,
		person.Confidence,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertOrganization(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, org entity.OrgInfo, createdAt int64) error {
	id := uuid.New().String()
	query := `
		INSERT INTO file_organizations (id, workspace_id, file_id, name, type, context, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, name) DO UPDATE SET
			type = excluded.type,
			context = excluded.context,
			confidence = excluded.confidence
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		org.Name,
		org.Type,
		org.Context,
		org.Confidence,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertEvent(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, event entity.EventInfo, createdAt int64) error {
	id := uuid.New().String()
	var dateStr *string
	if event.Date != nil {
		dateStrVal := event.Date.Format("2006-01-02")
		dateStr = &dateStrVal
	}

	query := `
		INSERT INTO file_events (id, workspace_id, file_id, name, date, location, context, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		event.Name,
		dateStr,
		event.Location,
		event.Context,
		event.Confidence,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertReference(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, ref entity.ReferenceInfo, createdAt int64) error {
	id := uuid.New().String()
	var yearStr *string
	if ref.Year != nil {
		yearStrVal := fmt.Sprintf("%d", *ref.Year)
		yearStr = &yearStrVal
	}

	query := `
		INSERT INTO file_references (id, workspace_id, file_id, title, author, year, type, doi, url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		ref.Title,
		ref.Author,
		yearStr,
		ref.Type,
		nil, // DOI not in ReferenceInfo
		ref.URL,
		createdAt,
	)
	return err
}

func (r *AIContextRepository) insertPublicationInfo(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, aiContext *entity.AIContext, createdAt int64) error {
	id := uuid.New().String()
	var pubYearStr *string
	if aiContext.PublicationYear != nil {
		pubYearStrVal := fmt.Sprintf("%d", *aiContext.PublicationYear)
		pubYearStr = &pubYearStrVal
	}

	query := `
		INSERT INTO file_publication_info (id, workspace_id, file_id, publisher, publication_year, publication_place, isbn, issn, doi, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id) DO UPDATE SET
			publisher = excluded.publisher,
			publication_year = excluded.publication_year,
			publication_place = excluded.publication_place,
			isbn = excluded.isbn,
			issn = excluded.issn,
			doi = excluded.doi
	`
	_, err := r.conn.Exec(ctx, query,
		id,
		workspaceID.String(),
		fileID.String(),
		aiContext.Publisher,
		pubYearStr,
		aiContext.PublicationPlace,
		aiContext.ISBN,
		aiContext.ISSN,
		aiContext.DOI,
		createdAt,
	)
	return err
}

