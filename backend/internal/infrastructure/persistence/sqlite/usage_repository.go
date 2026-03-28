package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// UsageRepository implements repository.UsageRepository using SQLite.
type UsageRepository struct {
	conn *Connection
}

// NewUsageRepository creates a new SQLite usage repository.
func NewUsageRepository(conn *Connection) *UsageRepository {
	return &UsageRepository{conn: conn}
}

// RecordEvent records a usage event.
func (r *UsageRepository) RecordEvent(ctx context.Context, workspaceID entity.WorkspaceID, event *entity.DocumentUsageEvent) error {
	if !event.EventType.IsValid() {
		return fmt.Errorf("invalid usage event type: %s", event.EventType)
	}

	var metadataJSON []byte
	if event.Metadata != nil {
		data, err := json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = data
	}

	query := `
		INSERT INTO document_usage_events (
			id, workspace_id, document_id, event_type, context, metadata, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.conn.Exec(ctx, query,
		event.ID.String(),
		workspaceID.String(),
		event.DocumentID.String(),
		event.EventType.String(),
		event.Context,
		metadataJSON,
		event.Timestamp.UnixMilli(),
	)
	return err
}

// RecordEvents records multiple usage events in a transaction.
func (r *UsageRepository) RecordEvents(ctx context.Context, workspaceID entity.WorkspaceID, events []*entity.DocumentUsageEvent) error {
	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO document_usage_events (
				id, workspace_id, document_id, event_type, context, metadata, timestamp
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, event := range events {
			var metadataJSON []byte
			if event.Metadata != nil {
				data, err := json.Marshal(event.Metadata)
				if err != nil {
					return fmt.Errorf("failed to marshal metadata: %w", err)
				}
				metadataJSON = data
			}

			_, err := stmt.ExecContext(ctx,
				event.ID.String(),
				workspaceID.String(),
				event.DocumentID.String(),
				event.EventType.String(),
				event.Context,
				metadataJSON,
				event.Timestamp.UnixMilli(),
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUsageStats calculates usage statistics for a document.
func (r *UsageRepository) GetUsageStats(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, since time.Time) (*entity.DocumentUsageStats, error) {
	query := `
		SELECT 
			COUNT(*) as access_count,
			MIN(timestamp) as first_accessed,
			MAX(timestamp) as last_accessed
		FROM document_usage_events
		WHERE workspace_id = ? AND document_id = ? AND timestamp >= ?
	`
	var accessCount int
	var firstAccessed, lastAccessed sql.NullInt64

	err := r.conn.QueryRow(ctx, query,
		workspaceID.String(),
		docID.String(),
		since.UnixMilli(),
	).Scan(&accessCount, &firstAccessed, &lastAccessed)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	stats := entity.NewDocumentUsageStats(docID)
	stats.AccessCount = accessCount

	if firstAccessed.Valid {
		stats.FirstAccessed = time.UnixMilli(firstAccessed.Int64)
	}
	if lastAccessed.Valid {
		stats.LastAccessed = time.UnixMilli(lastAccessed.Int64)
	}

	// Calculate frequency (accesses per day)
	if !stats.FirstAccessed.IsZero() && !stats.LastAccessed.IsZero() {
		days := stats.LastAccessed.Sub(stats.FirstAccessed).Hours() / 24
		if days > 0 {
			stats.Frequency = float64(accessCount) / days
		}
	}

	// Get co-occurrences
	coOccurrences, err := r.GetCoOccurrences(ctx, workspaceID, docID, 10, since)
	if err == nil {
		stats.CoOccurrences = coOccurrences
	}

	return stats, nil
}

// GetCoOccurrences finds documents that are used together with the given document.
func (r *UsageRepository) GetCoOccurrences(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, limit int, since time.Time) (map[entity.DocumentID]int, error) {
	// Find events within a time window (e.g., same session = within 1 hour)
	// This query finds documents accessed within 1 hour of the target document
	query := `
		SELECT DISTINCT e2.document_id, COUNT(*) as co_count
		FROM document_usage_events e1
		JOIN document_usage_events e2 ON 
			e1.workspace_id = e2.workspace_id AND
			e1.document_id != e2.document_id AND
			ABS(e1.timestamp - e2.timestamp) <= 3600000 AND
			e1.timestamp >= ? AND e2.timestamp >= ?
		WHERE e1.workspace_id = ? AND e1.document_id = ?
		GROUP BY e2.document_id
		ORDER BY co_count DESC
		LIMIT ?
	`

	rows, err := r.conn.Query(ctx, query,
		since.UnixMilli(),
		since.UnixMilli(),
		workspaceID.String(),
		docID.String(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coOccurrences := make(map[entity.DocumentID]int)
	for rows.Next() {
		var otherDocIDStr string
		var count int
		if err := rows.Scan(&otherDocIDStr, &count); err != nil {
			return nil, err
		}
		coOccurrences[entity.DocumentID(otherDocIDStr)] = count
	}
	return coOccurrences, rows.Err()
}

// GetFrequentlyUsed returns the most frequently accessed documents.
func (r *UsageRepository) GetFrequentlyUsed(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, limit int) ([]entity.DocumentID, error) {
	query := `
		SELECT document_id, COUNT(*) as access_count
		FROM document_usage_events
		WHERE workspace_id = ? AND timestamp >= ?
		GROUP BY document_id
		ORDER BY access_count DESC
		LIMIT ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docIDs []entity.DocumentID
	for rows.Next() {
		var docIDStr string
		if err := rows.Scan(&docIDStr, new(int)); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, entity.DocumentID(docIDStr))
	}
	return docIDs, rows.Err()
}

// GetRecentlyUsed returns the most recently accessed documents.
func (r *UsageRepository) GetRecentlyUsed(ctx context.Context, workspaceID entity.WorkspaceID, limit int) ([]entity.DocumentID, error) {
	query := `
		SELECT DISTINCT document_id
		FROM document_usage_events
		WHERE workspace_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docIDs []entity.DocumentID
	for rows.Next() {
		var docIDStr string
		if err := rows.Scan(&docIDStr); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, entity.DocumentID(docIDStr))
	}
	return docIDs, rows.Err()
}

// GetEventsByType returns usage events of a specific type.
func (r *UsageRepository) GetEventsByType(ctx context.Context, workspaceID entity.WorkspaceID, eventType entity.UsageEventType, since time.Time, limit int) ([]*entity.DocumentUsageEvent, error) {
	query := `
		SELECT id, document_id, event_type, context, metadata, timestamp
		FROM document_usage_events
		WHERE workspace_id = ? AND event_type = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.conn.Query(ctx, query,
		workspaceID.String(),
		eventType.String(),
		since.UnixMilli(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvents(rows, workspaceID)
}

// GetEventsForDocument returns usage events for a specific document.
func (r *UsageRepository) GetEventsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, since time.Time, limit int) ([]*entity.DocumentUsageEvent, error) {
	query := `
		SELECT id, document_id, event_type, context, metadata, timestamp
		FROM document_usage_events
		WHERE workspace_id = ? AND document_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.conn.Query(ctx, query,
		workspaceID.String(),
		docID.String(),
		since.UnixMilli(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanEvents(rows, workspaceID)
}

// scanEvents scans usage events from database rows.
func (r *UsageRepository) scanEvents(rows *sql.Rows, workspaceID entity.WorkspaceID) ([]*entity.DocumentUsageEvent, error) {
	var events []*entity.DocumentUsageEvent
	for rows.Next() {
		var id, docIDStr, eventTypeStr, context string
		var metadataJSON sql.NullString
		var timestamp int64

		err := rows.Scan(&id, &docIDStr, &eventTypeStr, &context, &metadataJSON, &timestamp)
		if err != nil {
			return nil, err
		}

		event := &entity.DocumentUsageEvent{
			ID:          entity.UsageEventID(id),
			WorkspaceID: workspaceID,
			DocumentID:  entity.DocumentID(docIDStr),
			EventType:   entity.UsageEventType(eventTypeStr),
			Context:     context,
			Timestamp:   time.UnixMilli(timestamp),
			Metadata:    make(map[string]interface{}),
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &event.Metadata); err != nil {
				// Log error but don't fail - metadata is optional
				event.Metadata = make(map[string]interface{})
			}
		}

		events = append(events, event)
	}
	return events, rows.Err()
}

var _ repository.UsageRepository = (*UsageRepository)(nil)

