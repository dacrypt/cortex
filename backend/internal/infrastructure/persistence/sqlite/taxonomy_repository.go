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

// TaxonomyRepository implements repository.TaxonomyRepository using SQLite.
type TaxonomyRepository struct {
	conn *Connection
}

// NewTaxonomyRepository creates a new taxonomy repository.
func NewTaxonomyRepository(conn *Connection) *TaxonomyRepository {
	return &TaxonomyRepository{conn: conn}
}

// CreateNode creates a new taxonomy node.
func (r *TaxonomyRepository) CreateNode(ctx context.Context, workspaceID entity.WorkspaceID, node *entity.TaxonomyNode) error {
	keywords, _ := json.Marshal(node.Keywords)
	exampleDocs, _ := json.Marshal(node.ExampleDocs)

	var parentID *string
	if node.ParentID != nil {
		s := node.ParentID.String()
		parentID = &s
	}

	_, err := r.conn.Exec(ctx, `
		INSERT INTO taxonomy_nodes (
			id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		node.ID.String(),
		workspaceID.String(),
		node.Name,
		node.Description,
		parentID,
		node.Path,
		node.Level,
		string(node.Source),
		node.Confidence,
		string(keywords),
		string(exampleDocs),
		node.ChildCount,
		node.DocCount,
		node.CreatedAt.UnixMilli(),
		node.UpdatedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to create taxonomy node: %w", err)
	}

	// Update parent's child count
	if node.ParentID != nil {
		_, err = r.conn.Exec(ctx, `
			UPDATE taxonomy_nodes
			SET child_count = child_count + 1, updated_at = ?
			WHERE workspace_id = ? AND id = ?
		`, time.Now().UnixMilli(), workspaceID.String(), node.ParentID.String())
		if err != nil {
			return fmt.Errorf("failed to update parent child count: %w", err)
		}
	}

	return nil
}

// GetNode retrieves a taxonomy node by ID.
func (r *TaxonomyRepository) GetNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) (*entity.TaxonomyNode, error) {
	row := r.conn.QueryRow(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND id = ?
	`, workspaceID.String(), nodeID.String())

	return r.scanNode(row)
}

// GetNodeByPath retrieves a taxonomy node by its full path.
func (r *TaxonomyRepository) GetNodeByPath(ctx context.Context, workspaceID entity.WorkspaceID, path string) (*entity.TaxonomyNode, error) {
	row := r.conn.QueryRow(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND path = ?
	`, workspaceID.String(), path)

	return r.scanNode(row)
}

// GetNodeByName retrieves a taxonomy node by name within a parent.
func (r *TaxonomyRepository) GetNodeByName(ctx context.Context, workspaceID entity.WorkspaceID, name string, parentID *entity.TaxonomyNodeID) (*entity.TaxonomyNode, error) {
	var row *sql.Row
	if parentID == nil {
		row = r.conn.QueryRow(ctx, `
			SELECT id, workspace_id, name, description, parent_id, path, level,
				source, confidence, keywords, example_docs, child_count, doc_count,
				created_at, updated_at
			FROM taxonomy_nodes
			WHERE workspace_id = ? AND name = ? AND parent_id IS NULL
		`, workspaceID.String(), name)
	} else {
		row = r.conn.QueryRow(ctx, `
			SELECT id, workspace_id, name, description, parent_id, path, level,
				source, confidence, keywords, example_docs, child_count, doc_count,
				created_at, updated_at
			FROM taxonomy_nodes
			WHERE workspace_id = ? AND name = ? AND parent_id = ?
		`, workspaceID.String(), name, parentID.String())
	}

	return r.scanNode(row)
}

// UpdateNode updates an existing taxonomy node.
func (r *TaxonomyRepository) UpdateNode(ctx context.Context, workspaceID entity.WorkspaceID, node *entity.TaxonomyNode) error {
	keywords, _ := json.Marshal(node.Keywords)
	exampleDocs, _ := json.Marshal(node.ExampleDocs)

	var parentID *string
	if node.ParentID != nil {
		s := node.ParentID.String()
		parentID = &s
	}

	node.UpdatedAt = time.Now()

	_, err := r.conn.Exec(ctx, `
		UPDATE taxonomy_nodes SET
			name = ?, description = ?, parent_id = ?, path = ?, level = ?,
			source = ?, confidence = ?, keywords = ?, example_docs = ?,
			child_count = ?, doc_count = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`,
		node.Name,
		node.Description,
		parentID,
		node.Path,
		node.Level,
		string(node.Source),
		node.Confidence,
		string(keywords),
		string(exampleDocs),
		node.ChildCount,
		node.DocCount,
		node.UpdatedAt.UnixMilli(),
		workspaceID.String(),
		node.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update taxonomy node: %w", err)
	}

	return nil
}

// DeleteNode deletes a taxonomy node.
func (r *TaxonomyRepository) DeleteNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) error {
	// Get node to find parent
	node, err := r.GetNode(ctx, workspaceID, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}

	// Delete the node
	_, err = r.conn.Exec(ctx, `
		DELETE FROM taxonomy_nodes
		WHERE workspace_id = ? AND id = ?
	`, workspaceID.String(), nodeID.String())
	if err != nil {
		return fmt.Errorf("failed to delete taxonomy node: %w", err)
	}

	// Update parent's child count
	if node.ParentID != nil {
		_, err = r.conn.Exec(ctx, `
			UPDATE taxonomy_nodes
			SET child_count = child_count - 1, updated_at = ?
			WHERE workspace_id = ? AND id = ?
		`, time.Now().UnixMilli(), workspaceID.String(), node.ParentID.String())
		if err != nil {
			return fmt.Errorf("failed to update parent child count: %w", err)
		}
	}

	return nil
}

// GetRootNodes returns all root taxonomy nodes.
func (r *TaxonomyRepository) GetRootNodes(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND parent_id IS NULL
		ORDER BY name
	`, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get root nodes: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// GetChildren returns child nodes of a parent.
func (r *TaxonomyRepository) GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND parent_id = ?
		ORDER BY name
	`, workspaceID.String(), parentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// GetAncestors returns ancestors of a node (from parent to root).
func (r *TaxonomyRepository) GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error) {
	var ancestors []*entity.TaxonomyNode

	currentID := nodeID
	for {
		node, err := r.GetNode(ctx, workspaceID, currentID)
		if err != nil {
			return nil, err
		}
		if node == nil || node.ParentID == nil {
			break
		}

		parent, err := r.GetNode(ctx, workspaceID, *node.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			break
		}

		ancestors = append(ancestors, parent)
		currentID = parent.ID
	}

	return ancestors, nil
}

// GetDescendants returns all descendants of a node.
func (r *TaxonomyRepository) GetDescendants(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error) {
	node, err := r.GetNode(ctx, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}

	// Use path prefix to find all descendants
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND path LIKE ? AND id != ?
		ORDER BY path
	`, workspaceID.String(), node.Path+"/%", nodeID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get descendants: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// ListAll returns all taxonomy nodes in a workspace.
func (r *TaxonomyRepository) ListAll(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ?
		ORDER BY path
	`, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// AddFileMapping adds a file-to-taxonomy mapping.
func (r *TaxonomyRepository) AddFileMapping(ctx context.Context, mapping *entity.FileTaxonomyMapping) error {
	_, err := r.conn.Exec(ctx, `
		INSERT INTO file_taxonomy_mappings (
			file_id, node_id, workspace_id, score, source, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, file_id, node_id) DO UPDATE SET
			score = excluded.score,
			source = excluded.source,
			updated_at = excluded.updated_at
	`,
		mapping.FileID.String(),
		mapping.NodeID.String(),
		mapping.WorkspaceID.String(),
		mapping.Score,
		string(mapping.Source),
		mapping.CreatedAt.UnixMilli(),
		mapping.UpdatedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("failed to add file mapping: %w", err)
	}

	// Update node's doc count
	_, err = r.conn.Exec(ctx, `
		UPDATE taxonomy_nodes
		SET doc_count = (
			SELECT COUNT(DISTINCT file_id)
			FROM file_taxonomy_mappings
			WHERE workspace_id = ? AND node_id = ?
		), updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`, mapping.WorkspaceID.String(), mapping.NodeID.String(), time.Now().UnixMilli(),
		mapping.WorkspaceID.String(), mapping.NodeID.String())
	if err != nil {
		return fmt.Errorf("failed to update node doc count: %w", err)
	}

	return nil
}

// RemoveFileMapping removes a file-to-taxonomy mapping.
func (r *TaxonomyRepository) RemoveFileMapping(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, nodeID entity.TaxonomyNodeID) error {
	_, err := r.conn.Exec(ctx, `
		DELETE FROM file_taxonomy_mappings
		WHERE workspace_id = ? AND file_id = ? AND node_id = ?
	`, workspaceID.String(), fileID.String(), nodeID.String())
	if err != nil {
		return fmt.Errorf("failed to remove file mapping: %w", err)
	}

	// Update node's doc count
	_, err = r.conn.Exec(ctx, `
		UPDATE taxonomy_nodes
		SET doc_count = (
			SELECT COUNT(DISTINCT file_id)
			FROM file_taxonomy_mappings
			WHERE workspace_id = ? AND node_id = ?
		), updated_at = ?
		WHERE workspace_id = ? AND id = ?
	`, workspaceID.String(), nodeID.String(), time.Now().UnixMilli(),
		workspaceID.String(), nodeID.String())
	if err != nil {
		return fmt.Errorf("failed to update node doc count: %w", err)
	}

	return nil
}

// GetFileMappings returns all taxonomy mappings for a file.
func (r *TaxonomyRepository) GetFileMappings(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.FileTaxonomyMapping, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT file_id, node_id, workspace_id, score, source, created_at, updated_at
		FROM file_taxonomy_mappings
		WHERE workspace_id = ? AND file_id = ?
		ORDER BY score DESC
	`, workspaceID.String(), fileID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get file mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*entity.FileTaxonomyMapping
	for rows.Next() {
		var (
			fileIDStr, nodeIDStr, wsIDStr string
			score                         float64
			source                        string
			createdAt, updatedAt          int64
		)

		if err := rows.Scan(&fileIDStr, &nodeIDStr, &wsIDStr, &score, &source, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan mapping: %w", err)
		}

		mappings = append(mappings, &entity.FileTaxonomyMapping{
			FileID:      entity.FileID(fileIDStr),
			NodeID:      entity.TaxonomyNodeID(nodeIDStr),
			WorkspaceID: entity.WorkspaceID(wsIDStr),
			Score:       score,
			Source:      entity.TaxonomyNodeSource(source),
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		})
	}

	return mappings, rows.Err()
}

// GetNodeFiles returns all files mapped to a taxonomy node.
func (r *TaxonomyRepository) GetNodeFiles(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID, includeDescendants bool) ([]entity.FileID, error) {
	var query string
	var args []interface{}

	if includeDescendants {
		// Get the node to find its path
		node, err := r.GetNode(ctx, workspaceID, nodeID)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, nil
		}

		query = `
			SELECT DISTINCT ftm.file_id
			FROM file_taxonomy_mappings ftm
			JOIN taxonomy_nodes tn ON ftm.workspace_id = tn.workspace_id AND ftm.node_id = tn.id
			WHERE ftm.workspace_id = ? AND (tn.id = ? OR tn.path LIKE ?)
			ORDER BY ftm.score DESC
		`
		args = []interface{}{workspaceID.String(), nodeID.String(), node.Path + "/%"}
	} else {
		query = `
			SELECT DISTINCT file_id
			FROM file_taxonomy_mappings
			WHERE workspace_id = ? AND node_id = ?
			ORDER BY score DESC
		`
		args = []interface{}{workspaceID.String(), nodeID.String()}
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get node files: %w", err)
	}
	defer rows.Close()

	var files []entity.FileID
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err != nil {
			return nil, fmt.Errorf("failed to scan file ID: %w", err)
		}
		files = append(files, entity.FileID(fileID))
	}

	return files, rows.Err()
}

// UpdateFileMapping updates an existing file-to-taxonomy mapping.
func (r *TaxonomyRepository) UpdateFileMapping(ctx context.Context, mapping *entity.FileTaxonomyMapping) error {
	mapping.UpdatedAt = time.Now()

	_, err := r.conn.Exec(ctx, `
		UPDATE file_taxonomy_mappings SET
			score = ?, source = ?, updated_at = ?
		WHERE workspace_id = ? AND file_id = ? AND node_id = ?
	`,
		mapping.Score,
		string(mapping.Source),
		mapping.UpdatedAt.UnixMilli(),
		mapping.WorkspaceID.String(),
		mapping.FileID.String(),
		mapping.NodeID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update file mapping: %w", err)
	}

	return nil
}

// BulkCreateNodes creates multiple nodes in a single transaction.
func (r *TaxonomyRepository) BulkCreateNodes(ctx context.Context, workspaceID entity.WorkspaceID, nodes []*entity.TaxonomyNode) error {
	if len(nodes) == 0 {
		return nil
	}

	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO taxonomy_nodes (
				id, workspace_id, name, description, parent_id, path, level,
				source, confidence, keywords, example_docs, child_count, doc_count,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, node := range nodes {
			keywords, _ := json.Marshal(node.Keywords)
			exampleDocs, _ := json.Marshal(node.ExampleDocs)

			var parentID *string
			if node.ParentID != nil {
				s := node.ParentID.String()
				parentID = &s
			}

			_, err := stmt.ExecContext(ctx,
				node.ID.String(),
				workspaceID.String(),
				node.Name,
				node.Description,
				parentID,
				node.Path,
				node.Level,
				string(node.Source),
				node.Confidence,
				string(keywords),
				string(exampleDocs),
				node.ChildCount,
				node.DocCount,
				node.CreatedAt.UnixMilli(),
				node.UpdatedAt.UnixMilli(),
			)
			if err != nil {
				return fmt.Errorf("failed to insert node %s: %w", node.Name, err)
			}
		}

		return nil
	})
}

// BulkAddFileMappings adds multiple file mappings in a single transaction.
func (r *TaxonomyRepository) BulkAddFileMappings(ctx context.Context, mappings []*entity.FileTaxonomyMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO file_taxonomy_mappings (
				file_id, node_id, workspace_id, score, source, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, file_id, node_id) DO UPDATE SET
				score = excluded.score,
				source = excluded.source,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, mapping := range mappings {
			_, err := stmt.ExecContext(ctx,
				mapping.FileID.String(),
				mapping.NodeID.String(),
				mapping.WorkspaceID.String(),
				mapping.Score,
				string(mapping.Source),
				mapping.CreatedAt.UnixMilli(),
				mapping.UpdatedAt.UnixMilli(),
			)
			if err != nil {
				return fmt.Errorf("failed to insert mapping: %w", err)
			}
		}

		return nil
	})
}

// GetNodeStats retrieves statistics for a taxonomy node.
func (r *TaxonomyRepository) GetNodeStats(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) (*repository.TaxonomyNodeStats, error) {
	node, err := r.GetNode(ctx, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}

	stats := &repository.TaxonomyNodeStats{
		NodeID:      nodeID,
		ChildCount:  node.ChildCount,
	}

	// Get direct file count
	row := r.conn.QueryRow(ctx, `
		SELECT COUNT(DISTINCT file_id), AVG(score)
		FROM file_taxonomy_mappings
		WHERE workspace_id = ? AND node_id = ?
	`, workspaceID.String(), nodeID.String())

	var avgScore sql.NullFloat64
	if err := row.Scan(&stats.DirectFileCount, &avgScore); err != nil {
		return nil, fmt.Errorf("failed to get direct file count: %w", err)
	}
	if avgScore.Valid {
		stats.AverageScore = avgScore.Float64
	}

	// Get total file count (including descendants)
	row = r.conn.QueryRow(ctx, `
		SELECT COUNT(DISTINCT ftm.file_id)
		FROM file_taxonomy_mappings ftm
		JOIN taxonomy_nodes tn ON ftm.workspace_id = tn.workspace_id AND ftm.node_id = tn.id
		WHERE ftm.workspace_id = ? AND (tn.id = ? OR tn.path LIKE ?)
	`, workspaceID.String(), nodeID.String(), node.Path+"/%")

	if err := row.Scan(&stats.TotalFileCount); err != nil {
		return nil, fmt.Errorf("failed to get total file count: %w", err)
	}

	// Get descendant count
	row = r.conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND path LIKE ? AND id != ?
	`, workspaceID.String(), node.Path+"/%", nodeID.String())

	if err := row.Scan(&stats.DescendantCount); err != nil {
		return nil, fmt.Errorf("failed to get descendant count: %w", err)
	}

	// Get last mapping time
	row = r.conn.QueryRow(ctx, `
		SELECT MAX(updated_at)
		FROM file_taxonomy_mappings
		WHERE workspace_id = ? AND node_id = ?
	`, workspaceID.String(), nodeID.String())

	var lastMapping sql.NullInt64
	if err := row.Scan(&lastMapping); err != nil {
		return nil, fmt.Errorf("failed to get last mapping time: %w", err)
	}
	if lastMapping.Valid {
		stats.LastMappingAt = &lastMapping.Int64
	}

	return stats, nil
}

// GetTaxonomyStats retrieves overall taxonomy statistics.
func (r *TaxonomyRepository) GetTaxonomyStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.TaxonomyStats, error) {
	stats := &repository.TaxonomyStats{
		NodesBySource: make(map[entity.TaxonomyNodeSource]int),
	}

	// Get basic counts
	row := r.conn.QueryRow(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN parent_id IS NULL THEN 1 END) as roots,
			MAX(level) as max_depth,
			AVG(confidence) as avg_conf
		FROM taxonomy_nodes
		WHERE workspace_id = ?
	`, workspaceID.String())

	var avgConf sql.NullFloat64
	if err := row.Scan(&stats.TotalNodes, &stats.RootNodes, &stats.MaxDepth, &avgConf); err != nil {
		return nil, fmt.Errorf("failed to get basic stats: %w", err)
	}
	if avgConf.Valid {
		stats.AverageConfidence = avgConf.Float64
	}

	// Get total mappings
	row = r.conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM file_taxonomy_mappings
		WHERE workspace_id = ?
	`, workspaceID.String())
	if err := row.Scan(&stats.TotalMappings); err != nil {
		return nil, fmt.Errorf("failed to get mapping count: %w", err)
	}

	// Get nodes by source
	rows, err := r.conn.Query(ctx, `
		SELECT source, COUNT(*)
		FROM taxonomy_nodes
		WHERE workspace_id = ?
		GROUP BY source
	`, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by source: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, fmt.Errorf("failed to scan source count: %w", err)
		}
		stats.NodesBySource[entity.TaxonomyNodeSource(source)] = count
	}

	// Get orphaned nodes (no files and no children)
	row = r.conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM taxonomy_nodes tn
		WHERE tn.workspace_id = ?
		AND tn.child_count = 0
		AND tn.doc_count = 0
	`, workspaceID.String())
	if err := row.Scan(&stats.OrphanedNodes); err != nil {
		return nil, fmt.Errorf("failed to get orphaned count: %w", err)
	}

	return stats, nil
}

// SearchNodes searches for taxonomy nodes by name or keywords.
func (r *TaxonomyRepository) SearchNodes(ctx context.Context, workspaceID entity.WorkspaceID, query string, limit int) ([]*entity.TaxonomyNode, error) {
	searchPattern := "%" + strings.ToLower(query) + "%"

	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ?
		AND (LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(keywords) LIKE ?)
		ORDER BY confidence DESC, doc_count DESC
		LIMIT ?
	`, workspaceID.String(), searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// GetNodesBySource returns nodes created by a specific source.
func (r *TaxonomyRepository) GetNodesBySource(ctx context.Context, workspaceID entity.WorkspaceID, source entity.TaxonomyNodeSource) ([]*entity.TaxonomyNode, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND source = ?
		ORDER BY path
	`, workspaceID.String(), string(source))
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by source: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// GetLowConfidenceNodes returns nodes with confidence below threshold.
func (r *TaxonomyRepository) GetLowConfidenceNodes(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.TaxonomyNode, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, workspace_id, name, description, parent_id, path, level,
			source, confidence, keywords, example_docs, child_count, doc_count,
			created_at, updated_at
		FROM taxonomy_nodes
		WHERE workspace_id = ? AND confidence < ?
		ORDER BY confidence ASC
	`, workspaceID.String(), threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get low confidence nodes: %w", err)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// MergeNodes merges source node into target node.
func (r *TaxonomyRepository) MergeNodes(ctx context.Context, workspaceID entity.WorkspaceID, sourceID, targetID entity.TaxonomyNodeID) error {
	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Move all file mappings from source to target
		_, err := tx.ExecContext(ctx, `
			UPDATE file_taxonomy_mappings
			SET node_id = ?, updated_at = ?
			WHERE workspace_id = ? AND node_id = ?
		`, targetID.String(), time.Now().UnixMilli(), workspaceID.String(), sourceID.String())
		if err != nil {
			return fmt.Errorf("failed to move file mappings: %w", err)
		}

		// Move all children from source to target
		_, err = tx.ExecContext(ctx, `
			UPDATE taxonomy_nodes
			SET parent_id = ?, updated_at = ?
			WHERE workspace_id = ? AND parent_id = ?
		`, targetID.String(), time.Now().UnixMilli(), workspaceID.String(), sourceID.String())
		if err != nil {
			return fmt.Errorf("failed to move children: %w", err)
		}

		// Delete the source node
		_, err = tx.ExecContext(ctx, `
			DELETE FROM taxonomy_nodes
			WHERE workspace_id = ? AND id = ?
		`, workspaceID.String(), sourceID.String())
		if err != nil {
			return fmt.Errorf("failed to delete source node: %w", err)
		}

		// Update target node counts
		_, err = tx.ExecContext(ctx, `
			UPDATE taxonomy_nodes
			SET
				doc_count = (
					SELECT COUNT(DISTINCT file_id)
					FROM file_taxonomy_mappings
					WHERE workspace_id = ? AND node_id = ?
				),
				child_count = (
					SELECT COUNT(*)
					FROM taxonomy_nodes
					WHERE workspace_id = ? AND parent_id = ?
				),
				source = 'merged',
				updated_at = ?
			WHERE workspace_id = ? AND id = ?
		`, workspaceID.String(), targetID.String(),
			workspaceID.String(), targetID.String(),
			time.Now().UnixMilli(),
			workspaceID.String(), targetID.String())
		if err != nil {
			return fmt.Errorf("failed to update target node: %w", err)
		}

		return nil
	})
}

// Helper methods

func (r *TaxonomyRepository) scanNode(row *sql.Row) (*entity.TaxonomyNode, error) {
	var (
		id, wsID, name, path string
		description          sql.NullString
		parentID             sql.NullString
		level                int
		source               string
		confidence           float64
		keywords, exampleDocs string
		childCount, docCount int
		createdAt, updatedAt int64
	)

	err := row.Scan(
		&id, &wsID, &name, &description, &parentID, &path, &level,
		&source, &confidence, &keywords, &exampleDocs, &childCount, &docCount,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan node: %w", err)
	}

	node := &entity.TaxonomyNode{
		ID:          entity.TaxonomyNodeID(id),
		WorkspaceID: entity.WorkspaceID(wsID),
		Name:        name,
		Path:        path,
		Level:       level,
		Source:      entity.TaxonomyNodeSource(source),
		Confidence:  confidence,
		ChildCount:  childCount,
		DocCount:    docCount,
		CreatedAt:   time.UnixMilli(createdAt),
		UpdatedAt:   time.UnixMilli(updatedAt),
	}

	if description.Valid {
		node.Description = description.String
	}
	if parentID.Valid {
		pid := entity.TaxonomyNodeID(parentID.String)
		node.ParentID = &pid
	}

	json.Unmarshal([]byte(keywords), &node.Keywords)
	json.Unmarshal([]byte(exampleDocs), &node.ExampleDocs)

	return node, nil
}

func (r *TaxonomyRepository) scanNodes(rows *sql.Rows) ([]*entity.TaxonomyNode, error) {
	var nodes []*entity.TaxonomyNode

	for rows.Next() {
		var (
			id, wsID, name, path string
			description          sql.NullString
			parentID             sql.NullString
			level                int
			source               string
			confidence           float64
			keywords, exampleDocs string
			childCount, docCount int
			createdAt, updatedAt int64
		)

		err := rows.Scan(
			&id, &wsID, &name, &description, &parentID, &path, &level,
			&source, &confidence, &keywords, &exampleDocs, &childCount, &docCount,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		node := &entity.TaxonomyNode{
			ID:          entity.TaxonomyNodeID(id),
			WorkspaceID: entity.WorkspaceID(wsID),
			Name:        name,
			Path:        path,
			Level:       level,
			Source:      entity.TaxonomyNodeSource(source),
			Confidence:  confidence,
			ChildCount:  childCount,
			DocCount:    docCount,
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		}

		if description.Valid {
			node.Description = description.String
		}
		if parentID.Valid {
			pid := entity.TaxonomyNodeID(parentID.String)
			node.ParentID = &pid
		}

		json.Unmarshal([]byte(keywords), &node.Keywords)
		json.Unmarshal([]byte(exampleDocs), &node.ExampleDocs)

		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// Ensure TaxonomyRepository implements the interface
var _ repository.TaxonomyRepository = (*TaxonomyRepository)(nil)
