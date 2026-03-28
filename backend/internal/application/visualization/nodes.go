package visualization

import (
	"context"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// NodeFilter contains filters for node generation.
type NodeFilter struct {
	ProjectID      *entity.ProjectID
	DocumentStates []entity.DocumentState
	MinUsageCount  int
}

// GenerateNodes generates node data based on filters.
func GenerateNodes(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	projectRepo repository.ProjectRepository,
	docRepo repository.DocumentRepository,
	stateRepo repository.DocumentStateRepository,
	usageRepo repository.UsageRepository,
	filter *NodeFilter,
) ([]Node, error) {
	nodes := []Node{}

	// Get projects
	var projects []*entity.Project
	var err error

	if filter != nil && filter.ProjectID != nil {
		proj, err := projectRepo.Get(ctx, workspaceID, *filter.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to get project: %w", err)
		}
		projects = []*entity.Project{proj}
		descendants, _ := projectRepo.GetDescendants(ctx, workspaceID, *filter.ProjectID)
		projects = append(projects, descendants...)
	} else {
		projects, err = projectRepo.List(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}
	}

	// Add project nodes
	for _, proj := range projects {
		nodes = append(nodes, Node{
			ID:    proj.ID.String(),
			Type:  "project",
			Label: proj.Name,
			Metadata: map[string]interface{}{
				"path":        proj.Path,
				"description": proj.Description,
			},
		})
	}

	// Add document nodes based on filters
	var states []entity.DocumentState
	if filter != nil && len(filter.DocumentStates) > 0 {
		states = filter.DocumentStates
	} else {
		// Default: include all states
		states = []entity.DocumentState{
			entity.DocumentStateDraft,
			entity.DocumentStateActive,
			entity.DocumentStateReplaced,
			entity.DocumentStateArchived,
		}
	}

	// Get documents by states
	docIDs, err := stateRepo.GetDocumentsByStates(ctx, workspaceID, states)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents by states: %w", err)
	}

	// Filter by project if specified
	if filter != nil && filter.ProjectID != nil {
		projectDocIDs, _ := projectRepo.GetDocuments(ctx, workspaceID, *filter.ProjectID, true)
		projectDocMap := make(map[entity.DocumentID]bool)
		for _, docID := range projectDocIDs {
			projectDocMap[docID] = true
		}

		filteredDocIDs := []entity.DocumentID{}
		for _, docID := range docIDs {
			if projectDocMap[docID] {
				filteredDocIDs = append(filteredDocIDs, docID)
			}
		}
		docIDs = filteredDocIDs
	}

	// Filter by usage count if specified
	if filter != nil && filter.MinUsageCount > 0 {
		filteredDocIDs := []entity.DocumentID{}
		for _, docID := range docIDs {
		// Use a reasonable since time (e.g., 1 year ago)
		since := time.Now().AddDate(-1, 0, 0)
		stats, err := usageRepo.GetUsageStats(ctx, workspaceID, docID, since)
		if err != nil {
			continue
		}
		if stats.AccessCount >= filter.MinUsageCount {
			filteredDocIDs = append(filteredDocIDs, docID)
		}
		}
		docIDs = filteredDocIDs
	}

	// Add document nodes
	for _, docID := range docIDs {
		doc, err := docRepo.GetDocument(ctx, workspaceID, docID)
		if err != nil {
			continue
		}

		state, _ := stateRepo.GetState(ctx, workspaceID, docID)
		// Use a reasonable since time (e.g., 1 year ago)
		since := time.Now().AddDate(-1, 0, 0)
		stats, _ := usageRepo.GetUsageStats(ctx, workspaceID, docID, since)

		nodes = append(nodes, Node{
			ID:    doc.ID.String(),
			Type:  "document",
			Label: doc.Title,
			Metadata: map[string]interface{}{
				"path":         doc.RelativePath,
				"state":        state.String(),
				"access_count": stats.AccessCount,
			},
		})
	}

	return nodes, nil
}

