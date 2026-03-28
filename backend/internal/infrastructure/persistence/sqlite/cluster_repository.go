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

// ClusterRepository implements repository.ClusterRepository using SQLite.
type ClusterRepository struct {
	conn *Connection
}

// NewClusterRepository creates a new SQLite cluster repository.
func NewClusterRepository(conn *Connection) *ClusterRepository {
	return &ClusterRepository{conn: conn}
}

// UpsertCluster inserts or updates a cluster record.
func (r *ClusterRepository) UpsertCluster(ctx context.Context, cluster *entity.DocumentCluster) error {
	// Ensure cluster has a name - generate one if missing
	clusterName := strings.TrimSpace(cluster.Name)
	if clusterName == "" {
		// Generate a default name based on cluster ID (shortened for readability)
		// Don't include member count in name - it's displayed separately in the UI
		clusterName = fmt.Sprintf("Cluster %s", cluster.ID.String()[:8])
		cluster.Name = clusterName
	}

	centralNodesJSON, err := json.Marshal(cluster.CentralNodes)
	if err != nil {
		return fmt.Errorf("failed to marshal central_nodes: %w", err)
	}

	topEntitiesJSON, err := json.Marshal(cluster.TopEntities)
	if err != nil {
		return fmt.Errorf("failed to marshal top_entities: %w", err)
	}

	topKeywordsJSON, err := json.Marshal(cluster.TopKeywords)
	if err != nil {
		return fmt.Errorf("failed to marshal top_keywords: %w", err)
	}

	var mergedInto interface{}
	if cluster.MergedInto != nil {
		mergedInto = cluster.MergedInto.String()
	}

	query := `
		INSERT INTO document_clusters (
			id, workspace_id, name, summary, status, confidence, member_count,
			central_nodes, top_entities, top_keywords, merged_into, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, id) DO UPDATE SET
			name = excluded.name,
			summary = excluded.summary,
			status = excluded.status,
			confidence = excluded.confidence,
			member_count = excluded.member_count,
			central_nodes = excluded.central_nodes,
			top_entities = excluded.top_entities,
			top_keywords = excluded.top_keywords,
			merged_into = excluded.merged_into,
			updated_at = excluded.updated_at
	`

	_, err = r.conn.Exec(ctx, query,
		cluster.ID.String(),
		cluster.WorkspaceID.String(),
		cluster.Name,
		cluster.Summary,
		cluster.Status.String(),
		cluster.Confidence,
		cluster.MemberCount,
		string(centralNodesJSON),
		string(topEntitiesJSON),
		string(topKeywordsJSON),
		mergedInto,
		cluster.CreatedAt.UnixMilli(),
		cluster.UpdatedAt.UnixMilli(),
	)
	return err
}

// GetCluster retrieves a cluster by ID.
func (r *ClusterRepository) GetCluster(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID) (*entity.DocumentCluster, error) {
	query := `
		SELECT id, workspace_id, name, summary, status, confidence, member_count,
		       central_nodes, top_entities, top_keywords, merged_into, created_at, updated_at
		FROM document_clusters
		WHERE workspace_id = ? AND id = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), id.String())
	return scanCluster(row)
}

// GetClustersByWorkspace retrieves all clusters for a workspace.
func (r *ClusterRepository) GetClustersByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error) {
	query := `
		SELECT id, workspace_id, name, summary, status, confidence, member_count,
		       central_nodes, top_entities, top_keywords, merged_into, created_at, updated_at
		FROM document_clusters
		WHERE workspace_id = ?
		ORDER BY confidence DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanClusters(rows)
}

// GetActiveClustersByWorkspace retrieves active clusters for a workspace.
func (r *ClusterRepository) GetActiveClustersByWorkspace(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentCluster, error) {
	query := `
		SELECT id, workspace_id, name, summary, status, confidence, member_count,
		       central_nodes, top_entities, top_keywords, merged_into, created_at, updated_at
		FROM document_clusters
		WHERE workspace_id = ? AND status = 'active'
		ORDER BY confidence DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanClusters(rows)
}

// DeleteCluster deletes a cluster.
func (r *ClusterRepository) DeleteCluster(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID) error {
	query := `DELETE FROM document_clusters WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), id.String())
	return err
}

// UpdateClusterStatus updates the status of a cluster.
func (r *ClusterRepository) UpdateClusterStatus(ctx context.Context, workspaceID entity.WorkspaceID, id entity.ClusterID, status entity.ClusterStatus) error {
	query := `UPDATE document_clusters SET status = ?, updated_at = ? WHERE workspace_id = ? AND id = ?`
	_, err := r.conn.Exec(ctx, query, status.String(), time.Now().UnixMilli(), workspaceID.String(), id.String())
	return err
}

// AddMembership adds a document to a cluster.
func (r *ClusterRepository) AddMembership(ctx context.Context, membership *entity.ClusterMembership) error {
	query := `
		INSERT INTO cluster_memberships (cluster_id, document_id, workspace_id, score, is_central, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, cluster_id, document_id) DO UPDATE SET
			score = excluded.score,
			is_central = excluded.is_central
	`
	isCentral := 0
	if membership.IsCentral {
		isCentral = 1
	}
	_, err := r.conn.Exec(ctx, query,
		membership.ClusterID.String(),
		membership.DocumentID.String(),
		membership.WorkspaceID.String(),
		membership.Score,
		isCentral,
		membership.JoinedAt.UnixMilli(),
	)
	return err
}

// RemoveMembership removes a document from a cluster.
func (r *ClusterRepository) RemoveMembership(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID) error {
	query := `DELETE FROM cluster_memberships WHERE cluster_id = ? AND document_id = ?`
	_, err := r.conn.Exec(ctx, query, clusterID.String(), documentID.String())
	return err
}

// GetMembershipsByCluster retrieves all memberships for a cluster.
func (r *ClusterRepository) GetMembershipsByCluster(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]*entity.ClusterMembership, error) {
	query := `
		SELECT cluster_id, document_id, workspace_id, score, is_central, joined_at
		FROM cluster_memberships
		WHERE workspace_id = ? AND cluster_id = ?
		ORDER BY score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), clusterID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMemberships(rows)
}

// GetMembershipsByDocument retrieves all cluster memberships for a document.
func (r *ClusterRepository) GetMembershipsByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.ClusterMembership, error) {
	query := `
		SELECT cluster_id, document_id, workspace_id, score, is_central, joined_at
		FROM cluster_memberships
		WHERE workspace_id = ? AND document_id = ?
		ORDER BY score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), documentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMemberships(rows)
}

// GetClusterMembers retrieves all document IDs in a cluster.
func (r *ClusterRepository) GetClusterMembers(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]entity.DocumentID, error) {
	query := `
		SELECT document_id
		FROM cluster_memberships
		WHERE workspace_id = ? AND cluster_id = ?
		ORDER BY score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), clusterID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []entity.DocumentID
	for rows.Next() {
		var docID string
		if err := rows.Scan(&docID); err != nil {
			return nil, err
		}
		members = append(members, entity.DocumentID(docID))
	}
	return members, rows.Err()
}

// GetClusterMembersWithInfo retrieves all cluster members with file information.
func (r *ClusterRepository) GetClusterMembersWithInfo(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) ([]*repository.ClusterMemberInfo, error) {
	query := `
		SELECT
			cm.document_id,
			COALESCE(d.relative_path, '') as relative_path,
			cm.score,
			cm.is_central
		FROM cluster_memberships cm
		LEFT JOIN documents d ON cm.document_id = d.id AND cm.workspace_id = d.workspace_id
		WHERE cm.workspace_id = ? AND cm.cluster_id = ?
		ORDER BY cm.score DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), clusterID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*repository.ClusterMemberInfo
	for rows.Next() {
		var docID, relativePath string
		var score float64
		var isCentral int
		if err := rows.Scan(&docID, &relativePath, &score, &isCentral); err != nil {
			return nil, err
		}

		// Extract filename from relative path
		filename := ""
		if relativePath != "" {
			parts := strings.Split(relativePath, "/")
			filename = parts[len(parts)-1]
		}

		members = append(members, &repository.ClusterMemberInfo{
			DocumentID:      entity.DocumentID(docID),
			RelativePath:    relativePath,
			Filename:        filename,
			MembershipScore: score,
			IsCentral:       isCentral == 1,
		})
	}
	return members, rows.Err()
}

// UpdateMembershipScore updates the membership score.
func (r *ClusterRepository) UpdateMembershipScore(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, score float64) error {
	query := `UPDATE cluster_memberships SET score = ? WHERE cluster_id = ? AND document_id = ?`
	_, err := r.conn.Exec(ctx, query, score, clusterID.String(), documentID.String())
	return err
}

// SetCentralNode marks or unmarks a document as a central node.
func (r *ClusterRepository) SetCentralNode(ctx context.Context, clusterID entity.ClusterID, documentID entity.DocumentID, isCentral bool) error {
	central := 0
	if isCentral {
		central = 1
	}
	query := `UPDATE cluster_memberships SET is_central = ? WHERE cluster_id = ? AND document_id = ?`
	_, err := r.conn.Exec(ctx, query, central, clusterID.String(), documentID.String())
	return err
}

// UpsertEdge inserts or updates a document edge.
func (r *ClusterRepository) UpsertEdge(ctx context.Context, edge *entity.DocumentEdge) error {
	sourcesJSON, err := json.Marshal(edge.Sources)
	if err != nil {
		return fmt.Errorf("failed to marshal sources: %w", err)
	}

	// Ensure canonical ordering (smaller ID first)
	fromDoc := edge.FromDoc
	toDoc := edge.ToDoc
	if string(fromDoc) > string(toDoc) {
		fromDoc, toDoc = toDoc, fromDoc
	}

	query := `
		INSERT INTO document_edges (from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, from_doc, to_doc) DO UPDATE SET
			weight = excluded.weight,
			sources = excluded.sources,
			updated_at = excluded.updated_at
	`
	_, err = r.conn.Exec(ctx, query,
		fromDoc.String(),
		toDoc.String(),
		edge.WorkspaceID.String(),
		edge.Weight,
		string(sourcesJSON),
		edge.CreatedAt.UnixMilli(),
		edge.UpdatedAt.UnixMilli(),
	)
	return err
}

// GetEdge retrieves an edge between two documents.
func (r *ClusterRepository) GetEdge(ctx context.Context, workspaceID entity.WorkspaceID, fromDoc, toDoc entity.DocumentID) (*entity.DocumentEdge, error) {
	// Ensure canonical ordering
	if string(fromDoc) > string(toDoc) {
		fromDoc, toDoc = toDoc, fromDoc
	}

	query := `
		SELECT from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at
		FROM document_edges
		WHERE workspace_id = ? AND from_doc = ? AND to_doc = ?
	`
	row := r.conn.QueryRow(ctx, query, workspaceID.String(), fromDoc.String(), toDoc.String())
	return scanEdge(row)
}

// GetEdgesByDocument retrieves all edges connected to a document.
func (r *ClusterRepository) GetEdgesByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) ([]*entity.DocumentEdge, error) {
	query := `
		SELECT from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at
		FROM document_edges
		WHERE workspace_id = ? AND (from_doc = ? OR to_doc = ?)
		ORDER BY weight DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), documentID.String(), documentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEdges(rows)
}

// GetAllEdges retrieves all edges for a workspace.
func (r *ClusterRepository) GetAllEdges(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.DocumentEdge, error) {
	query := `
		SELECT from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at
		FROM document_edges
		WHERE workspace_id = ?
		ORDER BY weight DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEdges(rows)
}

// GetEdgesAboveThreshold retrieves edges with weight above a threshold.
func (r *ClusterRepository) GetEdgesAboveThreshold(ctx context.Context, workspaceID entity.WorkspaceID, threshold float64) ([]*entity.DocumentEdge, error) {
	query := `
		SELECT from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at
		FROM document_edges
		WHERE workspace_id = ? AND weight >= ?
		ORDER BY weight DESC
	`
	rows, err := r.conn.Query(ctx, query, workspaceID.String(), threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEdges(rows)
}

// DeleteEdge deletes an edge between two documents.
func (r *ClusterRepository) DeleteEdge(ctx context.Context, workspaceID entity.WorkspaceID, fromDoc, toDoc entity.DocumentID) error {
	// Ensure canonical ordering
	if string(fromDoc) > string(toDoc) {
		fromDoc, toDoc = toDoc, fromDoc
	}
	query := `DELETE FROM document_edges WHERE workspace_id = ? AND from_doc = ? AND to_doc = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), fromDoc.String(), toDoc.String())
	return err
}

// DeleteEdgesByDocument deletes all edges connected to a document.
func (r *ClusterRepository) DeleteEdgesByDocument(ctx context.Context, workspaceID entity.WorkspaceID, documentID entity.DocumentID) error {
	query := `DELETE FROM document_edges WHERE workspace_id = ? AND (from_doc = ? OR to_doc = ?)`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), documentID.String(), documentID.String())
	return err
}

// LoadGraph loads the document graph from the database.
func (r *ClusterRepository) LoadGraph(ctx context.Context, workspaceID entity.WorkspaceID, minEdgeWeight float64) (*entity.DocumentGraph, error) {
	graph := entity.NewDocumentGraph(workspaceID)

	edges, err := r.GetEdgesAboveThreshold(ctx, workspaceID, minEdgeWeight)
	if err != nil {
		return nil, err
	}

	for _, edge := range edges {
		graph.AddEdge(edge)
	}

	return graph, nil
}

// UpsertEdgesBatch inserts or updates multiple edges in a batch.
func (r *ClusterRepository) UpsertEdgesBatch(ctx context.Context, edges []*entity.DocumentEdge) error {
	if len(edges) == 0 {
		return nil
	}

	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO document_edges (from_doc, to_doc, workspace_id, weight, sources, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workspace_id, from_doc, to_doc) DO UPDATE SET
				weight = excluded.weight,
				sources = excluded.sources,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, edge := range edges {
			sourcesJSON, err := json.Marshal(edge.Sources)
			if err != nil {
				return fmt.Errorf("failed to marshal sources: %w", err)
			}

			// Ensure canonical ordering
			fromDoc := edge.FromDoc
			toDoc := edge.ToDoc
			if string(fromDoc) > string(toDoc) {
				fromDoc, toDoc = toDoc, fromDoc
			}

			if _, err := stmt.ExecContext(ctx,
				fromDoc.String(),
				toDoc.String(),
				edge.WorkspaceID.String(),
				edge.Weight,
				string(sourcesJSON),
				edge.CreatedAt.UnixMilli(),
				edge.UpdatedAt.UnixMilli(),
			); err != nil {
				return err
			}
		}

		return nil
	})
}

// ClearClusterMemberships removes all memberships for a cluster.
func (r *ClusterRepository) ClearClusterMemberships(ctx context.Context, workspaceID entity.WorkspaceID, clusterID entity.ClusterID) error {
	query := `DELETE FROM cluster_memberships WHERE workspace_id = ? AND cluster_id = ?`
	_, err := r.conn.Exec(ctx, query, workspaceID.String(), clusterID.String())
	return err
}

// ClearAllClusters removes all clusters and memberships for a workspace.
func (r *ClusterRepository) ClearAllClusters(ctx context.Context, workspaceID entity.WorkspaceID) error {
	return r.conn.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM cluster_memberships WHERE workspace_id = ?`, workspaceID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM document_clusters WHERE workspace_id = ?`, workspaceID.String()); err != nil {
			return err
		}
		return nil
	})
}

// GetClusterStats returns aggregate statistics about clusters.
func (r *ClusterRepository) GetClusterStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.ClusterStats, error) {
	stats := &repository.ClusterStats{}

	// Count clusters
	row := r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM document_clusters WHERE workspace_id = ?`, workspaceID.String())
	if err := row.Scan(&stats.TotalClusters); err != nil {
		return nil, err
	}

	// Count active clusters
	row = r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM document_clusters WHERE workspace_id = ? AND status = 'active'`, workspaceID.String())
	if err := row.Scan(&stats.ActiveClusters); err != nil {
		return nil, err
	}

	// Count memberships
	row = r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM cluster_memberships WHERE workspace_id = ?`, workspaceID.String())
	if err := row.Scan(&stats.TotalMemberships); err != nil {
		return nil, err
	}

	// Count edges
	row = r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM document_edges WHERE workspace_id = ?`, workspaceID.String())
	if err := row.Scan(&stats.TotalEdges); err != nil {
		return nil, err
	}

	// Average edge weight
	row = r.conn.QueryRow(ctx, `SELECT COALESCE(AVG(weight), 0) FROM document_edges WHERE workspace_id = ?`, workspaceID.String())
	if err := row.Scan(&stats.AvgEdgeWeight); err != nil {
		return nil, err
	}

	// Cluster size stats
	row = r.conn.QueryRow(ctx, `
		SELECT COALESCE(AVG(member_count), 0), COALESCE(MAX(member_count), 0), COALESCE(MIN(member_count), 0)
		FROM document_clusters WHERE workspace_id = ? AND status = 'active'
	`, workspaceID.String())
	if err := row.Scan(&stats.AvgClusterSize, &stats.MaxClusterSize, &stats.MinClusterSize); err != nil {
		return nil, err
	}

	// Count isolated documents (documents with no edges)
	row = r.conn.QueryRow(ctx, `
		SELECT COUNT(DISTINCT d.id)
		FROM documents d
		LEFT JOIN document_edges e ON (d.id = e.from_doc OR d.id = e.to_doc) AND e.workspace_id = d.workspace_id
		WHERE d.workspace_id = ? AND e.from_doc IS NULL
	`, workspaceID.String())
	if err := row.Scan(&stats.IsolatedDocuments); err != nil {
		return nil, err
	}

	return stats, nil
}

// Helper functions for scanning

func scanCluster(row *sql.Row) (*entity.DocumentCluster, error) {
	var (
		id           string
		workspaceID  string
		name         string
		summary      sql.NullString
		status       string
		confidence   float64
		memberCount  int
		centralNodes sql.NullString
		topEntities  sql.NullString
		topKeywords  sql.NullString
		mergedInto   sql.NullString
		createdAt    int64
		updatedAt    int64
	)

	if err := row.Scan(&id, &workspaceID, &name, &summary, &status, &confidence, &memberCount,
		&centralNodes, &topEntities, &topKeywords, &mergedInto, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	cluster := &entity.DocumentCluster{
		ID:           entity.ClusterID(id),
		WorkspaceID:  entity.WorkspaceID(workspaceID),
		Name:         name,
		Status:       entity.ClusterStatus(status),
		Confidence:   confidence,
		MemberCount:  memberCount,
		CentralNodes: []entity.DocumentID{},
		TopEntities:  []string{},
		TopKeywords:  []string{},
		CreatedAt:    time.UnixMilli(createdAt),
		UpdatedAt:    time.UnixMilli(updatedAt),
	}

	if summary.Valid {
		cluster.Summary = summary.String
	}

	if centralNodes.Valid && centralNodes.String != "" {
		var nodes []string
		if err := json.Unmarshal([]byte(centralNodes.String), &nodes); err == nil {
			for _, n := range nodes {
				cluster.CentralNodes = append(cluster.CentralNodes, entity.DocumentID(n))
			}
		}
	}

	if topEntities.Valid && topEntities.String != "" {
		_ = json.Unmarshal([]byte(topEntities.String), &cluster.TopEntities)
	}

	if topKeywords.Valid && topKeywords.String != "" {
		_ = json.Unmarshal([]byte(topKeywords.String), &cluster.TopKeywords)
	}

	if mergedInto.Valid {
		merged := entity.ClusterID(mergedInto.String)
		cluster.MergedInto = &merged
	}

	return cluster, nil
}

func scanClusters(rows *sql.Rows) ([]*entity.DocumentCluster, error) {
	var clusters []*entity.DocumentCluster
	for rows.Next() {
		var (
			id           string
			workspaceID  string
			name         string
			summary      sql.NullString
			status       string
			confidence   float64
			memberCount  int
			centralNodes sql.NullString
			topEntities  sql.NullString
			topKeywords  sql.NullString
			mergedInto   sql.NullString
			createdAt    int64
			updatedAt    int64
		)

		if err := rows.Scan(&id, &workspaceID, &name, &summary, &status, &confidence, &memberCount,
			&centralNodes, &topEntities, &topKeywords, &mergedInto, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		cluster := &entity.DocumentCluster{
			ID:           entity.ClusterID(id),
			WorkspaceID:  entity.WorkspaceID(workspaceID),
			Name:         name,
			Status:       entity.ClusterStatus(status),
			Confidence:   confidence,
			MemberCount:  memberCount,
			CentralNodes: []entity.DocumentID{},
			TopEntities:  []string{},
			TopKeywords:  []string{},
			CreatedAt:    time.UnixMilli(createdAt),
			UpdatedAt:    time.UnixMilli(updatedAt),
		}

		if summary.Valid {
			cluster.Summary = summary.String
		}

		if centralNodes.Valid && centralNodes.String != "" {
			var nodes []string
			if err := json.Unmarshal([]byte(centralNodes.String), &nodes); err == nil {
				for _, n := range nodes {
					cluster.CentralNodes = append(cluster.CentralNodes, entity.DocumentID(n))
				}
			}
		}

		if topEntities.Valid && topEntities.String != "" {
			_ = json.Unmarshal([]byte(topEntities.String), &cluster.TopEntities)
		}

		if topKeywords.Valid && topKeywords.String != "" {
			_ = json.Unmarshal([]byte(topKeywords.String), &cluster.TopKeywords)
		}

		if mergedInto.Valid {
			merged := entity.ClusterID(mergedInto.String)
			cluster.MergedInto = &merged
		}

		clusters = append(clusters, cluster)
	}

	return clusters, rows.Err()
}

func scanMemberships(rows *sql.Rows) ([]*entity.ClusterMembership, error) {
	var memberships []*entity.ClusterMembership
	for rows.Next() {
		var (
			clusterID   string
			documentID  string
			workspaceID string
			score       float64
			isCentral   int
			joinedAt    int64
		)

		if err := rows.Scan(&clusterID, &documentID, &workspaceID, &score, &isCentral, &joinedAt); err != nil {
			return nil, err
		}

		memberships = append(memberships, &entity.ClusterMembership{
			ClusterID:   entity.ClusterID(clusterID),
			DocumentID:  entity.DocumentID(documentID),
			WorkspaceID: entity.WorkspaceID(workspaceID),
			Score:       score,
			IsCentral:   isCentral == 1,
			JoinedAt:    time.UnixMilli(joinedAt),
		})
	}

	return memberships, rows.Err()
}

func scanEdge(row *sql.Row) (*entity.DocumentEdge, error) {
	var (
		fromDoc     string
		toDoc       string
		workspaceID string
		weight      float64
		sourcesJSON sql.NullString
		createdAt   int64
		updatedAt   int64
	)

	if err := row.Scan(&fromDoc, &toDoc, &workspaceID, &weight, &sourcesJSON, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	edge := &entity.DocumentEdge{
		FromDoc:     entity.DocumentID(fromDoc),
		ToDoc:       entity.DocumentID(toDoc),
		WorkspaceID: entity.WorkspaceID(workspaceID),
		Weight:      weight,
		Sources:     []entity.EdgeSource{},
		CreatedAt:   time.UnixMilli(createdAt),
		UpdatedAt:   time.UnixMilli(updatedAt),
	}

	if sourcesJSON.Valid && sourcesJSON.String != "" {
		_ = json.Unmarshal([]byte(sourcesJSON.String), &edge.Sources)
	}

	return edge, nil
}

func scanEdges(rows *sql.Rows) ([]*entity.DocumentEdge, error) {
	var edges []*entity.DocumentEdge
	for rows.Next() {
		var (
			fromDoc     string
			toDoc       string
			workspaceID string
			weight      float64
			sourcesJSON sql.NullString
			createdAt   int64
			updatedAt   int64
		)

		if err := rows.Scan(&fromDoc, &toDoc, &workspaceID, &weight, &sourcesJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		edge := &entity.DocumentEdge{
			FromDoc:     entity.DocumentID(fromDoc),
			ToDoc:       entity.DocumentID(toDoc),
			WorkspaceID: entity.WorkspaceID(workspaceID),
			Weight:      weight,
			Sources:     []entity.EdgeSource{},
			CreatedAt:   time.UnixMilli(createdAt),
			UpdatedAt:   time.UnixMilli(updatedAt),
		}

		if sourcesJSON.Valid && sourcesJSON.String != "" {
			_ = json.Unmarshal([]byte(sourcesJSON.String), &edge.Sources)
		}

		edges = append(edges, edge)
	}

	return edges, rows.Err()
}

// GetClusterFacet returns cluster names with their member counts for faceting.
func (r *ClusterRepository) GetClusterFacet(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
	result := make(map[string]int)

	// Get active clusters with their member counts
	// If fileIDs provided, filter to only those files/documents
	var query string
	var args []interface{}

	if len(fileIDs) > 0 {
		// Build placeholders for IN clause
		placeholders := make([]string, len(fileIDs))
		args = make([]interface{}, 0, len(fileIDs)+1)
		args = append(args, workspaceID.String())
		for i, fid := range fileIDs {
			placeholders[i] = "?"
			args = append(args, string(fid))
		}

		query = `
			SELECT c.name, COUNT(DISTINCT cm.document_id) as count
			FROM document_clusters c
			INNER JOIN cluster_memberships cm ON c.id = cm.cluster_id AND c.workspace_id = cm.workspace_id
			WHERE c.workspace_id = ? AND c.status = 'active'
			AND cm.document_id IN (` + strings.Join(placeholders, ",") + `)
			GROUP BY c.id, c.name
			HAVING count > 0
			ORDER BY count DESC
		`
	} else {
		query = `
			SELECT c.name, c.member_count as count
			FROM document_clusters c
			WHERE c.workspace_id = ? AND c.status = 'active' AND c.member_count > 0
			ORDER BY c.member_count DESC
		`
		args = []interface{}{workspaceID.String()}
	}

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster facet: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan cluster facet row: %w", err)
		}
		result[name] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cluster facet rows: %w", err)
	}

	return result, nil
}

var _ repository.ClusterRepository = (*ClusterRepository)(nil)
