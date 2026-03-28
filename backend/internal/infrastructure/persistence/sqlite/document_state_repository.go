package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DocumentStateRepository implements repository.DocumentStateRepository using SQLite.
type DocumentStateRepository struct {
	conn *Connection
}

// NewDocumentStateRepository creates a new SQLite document state repository.
func NewDocumentStateRepository(conn *Connection) *DocumentStateRepository {
	return &DocumentStateRepository{conn: conn}
}

// SetState sets the state of a document and records the transition.
func (r *DocumentStateRepository) SetState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, state entity.DocumentState, reason string) error {
	if !state.IsValid() {
		return fmt.Errorf("invalid document state: %s", state)
	}

	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Get current state
		var currentState sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT state FROM documents WHERE workspace_id = ? AND id = ?`,
			workspaceID.String(), docID.String(),
		).Scan(&currentState)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		var fromState *entity.DocumentState
		if currentState.Valid {
			fs := entity.DocumentState(currentState.String)
			fromState = &fs
		}

		// Update document state
		now := time.Now()
		_, err = tx.ExecContext(ctx, `
			UPDATE documents
			SET state = ?, state_changed_at = ?, updated_at = ?
			WHERE workspace_id = ? AND id = ?
		`, state.String(), now.UnixMilli(), now.UnixMilli(), workspaceID.String(), docID.String())
		if err != nil {
			return err
		}

		// Record state transition
		transition := &entity.DocumentStateTransition{
			ID:         uuid.New().String(),
			DocumentID: docID,
			FromState:  fromState,
			ToState:    state,
			Reason:     reason,
			ChangedAt:  now,
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO document_state_history (
				id, workspace_id, document_id, from_state, to_state, reason, changed_by, changed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			transition.ID,
			workspaceID.String(),
			transition.DocumentID.String(),
			nilIfEmptyState(transition.FromState),
			transition.ToState.String(),
			transition.Reason,
			"", // changed_by can be set later if needed
			transition.ChangedAt.UnixMilli(),
		)
		return err
	})
}

// GetState retrieves the current state of a document.
func (r *DocumentStateRepository) GetState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) (entity.DocumentState, error) {
	var stateStr string
	err := r.conn.QueryRow(ctx, `
		SELECT state FROM documents WHERE workspace_id = ? AND id = ?
	`, workspaceID.String(), docID.String()).Scan(&stateStr)
	if err != nil {
		return "", err
	}
	return entity.DocumentState(stateStr), nil
}

// GetStateHistory retrieves the state transition history for a document.
func (r *DocumentStateRepository) GetStateHistory(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentStateTransition, error) {
	query := `
		SELECT id, document_id, from_state, to_state, reason, changed_by, changed_at
		FROM document_state_history
		WHERE workspace_id = ? AND document_id = ?
		ORDER BY changed_at ASC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), docID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transitions []*entity.DocumentStateTransition
	for rows.Next() {
		var id, docIDStr, toState, reason, changedBy string
		var fromState sql.NullString
		var changedAt int64

		err := rows.Scan(&id, &docIDStr, &fromState, &toState, &reason, &changedBy, &changedAt)
		if err != nil {
			return nil, err
		}

		transition := &entity.DocumentStateTransition{
			ID:         id,
			DocumentID: entity.DocumentID(docIDStr),
			ToState:    entity.DocumentState(toState),
			Reason:     reason,
			ChangedBy:  changedBy,
			ChangedAt:  time.UnixMilli(changedAt),
		}

		if fromState.Valid {
			fs := entity.DocumentState(fromState.String)
			transition.FromState = &fs
		}

		transitions = append(transitions, transition)
	}
	return transitions, rows.Err()
}

// GetDocumentsByState retrieves all documents with a specific state.
func (r *DocumentStateRepository) GetDocumentsByState(ctx context.Context, workspaceID entity.WorkspaceID, state entity.DocumentState) ([]entity.DocumentID, error) {
	return r.GetDocumentsByStates(ctx, workspaceID, []entity.DocumentState{state})
}

// GetDocumentsByStates retrieves all documents with any of the specified states.
func (r *DocumentStateRepository) GetDocumentsByStates(ctx context.Context, workspaceID entity.WorkspaceID, states []entity.DocumentState) ([]entity.DocumentID, error) {
	if len(states) == 0 {
		return []entity.DocumentID{}, nil
	}

	placeholders := ""
	args := make([]interface{}, len(states)+1)
	args[0] = workspaceID.String()
	for i, state := range states {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i+1] = state.String()
	}

	query := fmt.Sprintf(`
		SELECT id FROM documents
		WHERE workspace_id = ? AND state IN (%s)
	`, placeholders)

	rows, err := r.conn.Query(ctx, query, args...)
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

// nilIfEmptyState returns nil if the state pointer is nil.
func nilIfEmptyState(state *entity.DocumentState) interface{} {
	if state == nil {
		return nil
	}
	return state.String()
}

var _ repository.DocumentStateRepository = (*DocumentStateRepository)(nil)

