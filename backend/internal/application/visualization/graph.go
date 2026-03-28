package visualization

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Node represents a node in a graph visualization.
type Node struct {
	ID       string
	Type     string // "document", "project"
	Label    string
	Metadata map[string]interface{}
}

// Edge represents an edge in a graph visualization.
type Edge struct {
	From     string
	To       string
	Type     string // "relationship", "belongs_to", "parent_of"
	Weight   float64
	Metadata map[string]interface{}
}

// GraphData contains the complete graph structure for visualization.
type GraphData struct {
	Nodes    []Node
	Edges    []Edge
	Metadata map[string]interface{}
}

// GenerateGraph generates graph data for visualization.
// If projectID is nil, includes all projects.
func GenerateGraph(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	projectRepo repository.ProjectRepository,
	docRepo repository.DocumentRepository,
	relRepo repository.RelationshipRepository,
	projectID *entity.ProjectID,
	includeDocuments bool,
	includeRelationships bool,
) (*GraphData, error) {
	data := &GraphData{
		Nodes:    []Node{},
		Edges:    []Edge{},
		Metadata: make(map[string]interface{}),
	}

	// Get projects to include
	var projects []*entity.Project
	var err error

	if projectID != nil {
		// Get specific project and its descendants
		proj, err := projectRepo.Get(ctx, workspaceID, *projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to get project: %w", err)
		}
		projects = []*entity.Project{proj}
		descendants, _ := projectRepo.GetDescendants(ctx, workspaceID, *projectID)
		projects = append(projects, descendants...)
	} else {
		// Get all root projects
		projects, err = projectRepo.GetRootProjects(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get root projects: %w", err)
		}
		// Also get all projects
		allProjects, _ := projectRepo.List(ctx, workspaceID)
		projects = allProjects
	}

	// Add project nodes
	projectMap := make(map[entity.ProjectID]*entity.Project)
	for _, proj := range projects {
		projectMap[proj.ID] = proj
		data.Nodes = append(data.Nodes, Node{
			ID:    proj.ID.String(),
			Type:  "project",
			Label: proj.Name,
			Metadata: map[string]interface{}{
				"path":        proj.Path,
				"description": proj.Description,
			},
		})

		// Add parent_of edges for hierarchy
		if proj.ParentID != nil {
			if _, exists := projectMap[*proj.ParentID]; exists {
				data.Edges = append(data.Edges, Edge{
					From:   proj.ParentID.String(),
					To:     proj.ID.String(),
					Type:   "parent_of",
					Weight: 1.0,
					Metadata: map[string]interface{}{
						"hierarchical": true,
					},
				})
			}
		}
	}

	// Add document nodes and belongs_to edges if requested
	if includeDocuments {
		for _, proj := range projects {
			docIDs, err := projectRepo.GetDocuments(ctx, workspaceID, proj.ID, false)
			if err != nil {
				continue
			}

			for _, docID := range docIDs {
				doc, err := docRepo.GetDocument(ctx, workspaceID, docID)
				if err != nil {
					continue
				}

				// Add document node
				data.Nodes = append(data.Nodes, Node{
					ID:    doc.ID.String(),
					Type:  "document",
					Label: doc.Title,
					Metadata: map[string]interface{}{
						"path": doc.RelativePath,
					},
				})

				// Add belongs_to edge
				data.Edges = append(data.Edges, Edge{
					From:   doc.ID.String(),
					To:     proj.ID.String(),
					Type:   "belongs_to",
					Weight: 1.0,
					Metadata: map[string]interface{}{
						"role": "primary",
					},
				})
			}
		}
	}

	// Add relationship edges if requested
	if includeRelationships {
		// Get all document relationships for documents in the graph
		docNodeMap := make(map[entity.DocumentID]bool)
		for _, node := range data.Nodes {
			if node.Type == "document" {
				docID := entity.DocumentID(node.ID)
				docNodeMap[docID] = true
			}
		}

		// Get relationships
		for docID := range docNodeMap {
			// Outgoing relationships
			outgoing, _ := relRepo.GetOutgoing(ctx, workspaceID, docID, entity.RelationshipType(""))
			for _, rel := range outgoing {
				if docNodeMap[rel.ToDocument] {
					weight := 1.0
					if rel.Strength > 0 {
						weight = rel.Strength
					}
					data.Edges = append(data.Edges, Edge{
						From:   rel.FromDocument.String(),
						To:     rel.ToDocument.String(),
						Type:   rel.Type.String(),
						Weight: weight,
						Metadata: map[string]interface{}{
							"relationship_type": rel.Type.String(),
						},
					})
				}
			}
		}
	}

	data.Metadata["node_count"] = len(data.Nodes)
	data.Metadata["edge_count"] = len(data.Edges)
	data.Metadata["project_count"] = len(projects)

	return data, nil
}

