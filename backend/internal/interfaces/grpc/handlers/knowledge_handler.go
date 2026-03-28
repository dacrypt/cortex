package handlers

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/application/query"
	"github.com/dacrypt/cortex/backend/internal/application/relationship"
	"github.com/dacrypt/cortex/backend/internal/application/usage"
	"github.com/dacrypt/cortex/backend/internal/application/visualization"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// KnowledgeHandler handles knowledge engine operations.
type KnowledgeHandler struct {
	projectService       *project.Service
	usageTracker         *usage.Tracker
	usageAnalytics       *usage.Analytics
	relationshipDetector *relationship.Detector
	relationshipTraverser *relationship.Traverser
	queryExecutor        *query.Executor
	facetExecutor        *query.FacetExecutor
	projectRepo          repository.ProjectRepository
	docRepo              repository.DocumentRepository
	stateRepo            repository.DocumentStateRepository
	relRepo              repository.RelationshipRepository
	usageRepo            repository.UsageRepository
	fileRepo             repository.FileRepository
	metaRepo             repository.MetadataRepository
	assignmentRepo       repository.ProjectAssignmentRepository
	logger               zerolog.Logger
}

// KnowledgeHandlerConfig holds configuration for the knowledge handler.
type KnowledgeHandlerConfig struct {
	ProjectRepo      repository.ProjectRepository
	DocumentRepo     repository.DocumentRepository
	StateRepo        repository.DocumentStateRepository
	RelationshipRepo repository.RelationshipRepository
	UsageRepo        repository.UsageRepository
	FileRepo         repository.FileRepository
	MetaRepo         repository.MetadataRepository
	AssignmentRepo   repository.ProjectAssignmentRepository
	ClusterRepo      repository.ClusterRepository
	Logger           zerolog.Logger
}

// NewKnowledgeHandler creates a new knowledge handler.
func NewKnowledgeHandler(cfg KnowledgeHandlerConfig) *KnowledgeHandler {
	projectService := project.NewService(cfg.ProjectRepo)
	usageTracker := usage.NewTracker(cfg.UsageRepo)
	usageAnalytics := usage.NewAnalytics(cfg.UsageRepo)
	relationshipDetector := relationship.NewDetector(cfg.RelationshipRepo)
	relationshipTraverser := relationship.NewTraverser(cfg.RelationshipRepo)
	queryExecutor := query.NewExecutor(
		cfg.DocumentRepo,
		cfg.ProjectRepo,
		cfg.StateRepo,
		cfg.RelationshipRepo,
		cfg.UsageRepo,
	)

	facetExecutor := query.NewFacetExecutor(
		cfg.FileRepo,
		cfg.MetaRepo,
	)
	// Enable cluster faceting if cluster repository is provided
	if cfg.ClusterRepo != nil {
		facetExecutor.SetClusterRepository(cfg.ClusterRepo)
	}

	return &KnowledgeHandler{
		projectService:       projectService,
		usageTracker:         usageTracker,
		usageAnalytics:       usageAnalytics,
		relationshipDetector:  relationshipDetector,
		relationshipTraverser: relationshipTraverser,
		queryExecutor:        queryExecutor,
		facetExecutor:        facetExecutor,
		projectRepo:          cfg.ProjectRepo,
		docRepo:              cfg.DocumentRepo,
		stateRepo:            cfg.StateRepo,
		relRepo:              cfg.RelationshipRepo,
		usageRepo:            cfg.UsageRepo,
		fileRepo:             cfg.FileRepo,
		metaRepo:             cfg.MetaRepo,
		assignmentRepo:       cfg.AssignmentRepo,
		logger:               cfg.Logger.With().Str("handler", "knowledge").Logger(),
	}
}

// CreateProject creates a new project.
func (h *KnowledgeHandler) CreateProject(ctx context.Context, workspaceID entity.WorkspaceID, name, description string, parentID *entity.ProjectID) (*entity.Project, error) {
	return h.projectService.CreateProject(ctx, workspaceID, name, description, parentID)
}

// CreateProjectWithNature creates a new project with a specific nature and attributes.
func (h *KnowledgeHandler) CreateProjectWithNature(ctx context.Context, workspaceID entity.WorkspaceID, name, description string, parentID *entity.ProjectID, nature entity.ProjectNature, attributes *entity.ProjectAttributes) (*entity.Project, error) {
	return h.projectService.CreateProjectWithNature(ctx, workspaceID, name, description, parentID, nature, attributes)
}

// GetProject retrieves a project.
func (h *KnowledgeHandler) GetProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) (*entity.Project, error) {
	return h.projectService.GetProject(ctx, workspaceID, projectID)
}

// UpdateProject updates a project.
func (h *KnowledgeHandler) UpdateProject(ctx context.Context, workspaceID entity.WorkspaceID, proj *entity.Project) error {
	return h.projectService.UpdateProject(ctx, workspaceID, proj)
}

// DeleteProject deletes a project.
func (h *KnowledgeHandler) DeleteProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) error {
	return h.projectService.DeleteProject(ctx, workspaceID, projectID)
}

// ListProjects lists projects.
func (h *KnowledgeHandler) ListProjects(ctx context.Context, workspaceID entity.WorkspaceID, parentID *entity.ProjectID) ([]*entity.Project, error) {
	return h.projectService.ListProjects(ctx, workspaceID, parentID)
}

// GetProjectChildren returns child projects.
func (h *KnowledgeHandler) GetProjectChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.ProjectID) ([]*entity.Project, error) {
	return h.projectService.GetProjectChildren(ctx, workspaceID, parentID)
}

// GetProjectParents returns parent projects.
func (h *KnowledgeHandler) GetProjectParents(ctx context.Context, workspaceID entity.WorkspaceID, childID entity.ProjectID) ([]*entity.Project, error) {
	return h.projectService.GetProjectParents(ctx, workspaceID, childID)
}

// AddDocumentToProject associates a document with a project using default role (Primary).
func (h *KnowledgeHandler) AddDocumentToProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID) error {
	return h.projectRepo.AddDocument(ctx, workspaceID, projectID, docID, entity.ProjectDocumentRolePrimary)
}

// AddDocumentToProjectWithRole associates a document with a project using a specific role.
func (h *KnowledgeHandler) AddDocumentToProjectWithRole(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID, role entity.ProjectDocumentRole) error {
	if role == "" {
		role = entity.ProjectDocumentRolePrimary
	}
	return h.projectRepo.AddDocument(ctx, workspaceID, projectID, docID, role)
}

// RemoveDocumentFromProject removes a document from a project.
func (h *KnowledgeHandler) RemoveDocumentFromProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID) error {
	return h.projectRepo.RemoveDocument(ctx, workspaceID, projectID, docID)
}

// GetProjectsForDocument returns all projects that contain a document.
func (h *KnowledgeHandler) GetProjectsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]entity.ProjectID, error) {
	return h.projectRepo.GetProjectsForDocument(ctx, workspaceID, docID)
}

// DocumentProjectMembership represents a document's membership in a project with role and metadata.
type DocumentProjectMembership struct {
	WorkspaceID entity.WorkspaceID
	ProjectID   entity.ProjectID
	ProjectName string
	DocumentID  entity.DocumentID
	Role        entity.ProjectDocumentRole
	Score       float64
	AddedAt     time.Time
}

// GetAllProjectMembershipsForDocument returns all project memberships for a document with full details.
func (h *KnowledgeHandler) GetAllProjectMembershipsForDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, includeArchived bool) ([]*DocumentProjectMembership, error) {
	projectIDs, err := h.projectRepo.GetProjectsForDocument(ctx, workspaceID, docID)
	if err != nil {
		return nil, err
	}

	memberships := make([]*DocumentProjectMembership, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		proj, err := h.projectRepo.Get(ctx, workspaceID, projectID)
		if err != nil {
			h.logger.Debug().Err(err).Str("project_id", projectID.String()).Msg("Failed to get project for membership")
			continue
		}
		if proj == nil {
			continue
		}

		// Get the role from project documents (we need to query the relationship)
		// For now, assume Primary role as default since repository doesn't expose role
		role := entity.ProjectDocumentRolePrimary

		// Skip archived if not requested
		if !includeArchived && role == entity.ProjectDocumentRoleArchive {
			continue
		}

		membership := &DocumentProjectMembership{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			ProjectName: proj.Name,
			DocumentID:  docID,
			Role:        role,
			Score:       1.0, // Default score
			AddedAt:     proj.CreatedAt,
		}
		memberships = append(memberships, membership)
	}

	return memberships, nil
}

// UpdateDocumentProjectRole updates the role of a document within a project.
func (h *KnowledgeHandler) UpdateDocumentProjectRole(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, docID entity.DocumentID, role entity.ProjectDocumentRole) (*DocumentProjectMembership, error) {
	// Remove and re-add with new role (since UpdateDocumentRole may not exist in repo)
	if err := h.projectRepo.RemoveDocument(ctx, workspaceID, projectID, docID); err != nil {
		return nil, err
	}

	if err := h.projectRepo.AddDocument(ctx, workspaceID, projectID, docID, role); err != nil {
		return nil, err
	}

	proj, err := h.projectRepo.Get(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}

	projectName := ""
	if proj != nil {
		projectName = proj.Name
	}

	return &DocumentProjectMembership{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		ProjectName: projectName,
		DocumentID:  docID,
		Role:        role,
		Score:       1.0,
		AddedAt:     time.Now(),
	}, nil
}

// ListProjectAssignmentsByFile lists assignments for a file.
func (h *KnowledgeHandler) ListProjectAssignmentsByFile(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.ProjectAssignment, error) {
	if h.assignmentRepo == nil {
		return nil, nil
	}
	return h.assignmentRepo.ListByFile(ctx, workspaceID, fileID)
}

// ListProjectAssignmentsByProject lists assignments for a project.
func (h *KnowledgeHandler) ListProjectAssignmentsByProject(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID) ([]*entity.ProjectAssignment, error) {
	if h.assignmentRepo == nil {
		return nil, nil
	}
	return h.assignmentRepo.ListByProject(ctx, workspaceID, projectID)
}

// UpdateProjectAssignmentStatus updates assignment status for a file/project.
func (h *KnowledgeHandler) UpdateProjectAssignmentStatus(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, projectName string, status entity.ProjectAssignmentStatus) (*entity.ProjectAssignment, error) {
	if h.assignmentRepo == nil {
		return nil, nil
	}
	assignments, err := h.assignmentRepo.ListByFile(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	for _, assignment := range assignments {
		if assignment.ProjectName == projectName {
			assignment.Status = status
			assignment.UpdatedAt = time.Now()
			if err := h.assignmentRepo.Upsert(ctx, assignment); err != nil {
				return nil, err
			}
			return assignment, nil
		}
	}
	return nil, nil
}

// SetDocumentState sets a document's state.
func (h *KnowledgeHandler) SetDocumentState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, state entity.DocumentState, reason, changedBy string) error {
	return h.stateRepo.SetState(ctx, workspaceID, docID, state, reason)
}

// GetDocumentState gets a document's current state.
func (h *KnowledgeHandler) GetDocumentState(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) (entity.DocumentState, error) {
	return h.stateRepo.GetState(ctx, workspaceID, docID)
}

// GetDocumentStateHistory gets a document's state history.
func (h *KnowledgeHandler) GetDocumentStateHistory(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) ([]*entity.DocumentStateTransition, error) {
	return h.stateRepo.GetStateHistory(ctx, workspaceID, docID)
}

// ListDocumentsByState lists documents by state.
func (h *KnowledgeHandler) ListDocumentsByState(ctx context.Context, workspaceID entity.WorkspaceID, state entity.DocumentState) ([]entity.DocumentID, error) {
	return h.stateRepo.GetDocumentsByState(ctx, workspaceID, state)
}

// RecordUsage records a usage event.
func (h *KnowledgeHandler) RecordUsage(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, eventType entity.UsageEventType, context string) error {
	switch eventType {
	case entity.UsageEventOpened:
		return h.usageTracker.RecordOpen(ctx, workspaceID, docID, context)
	case entity.UsageEventEdited:
		return h.usageTracker.RecordEdit(ctx, workspaceID, docID, context)
	case entity.UsageEventSearched:
		return h.usageTracker.RecordSearch(ctx, workspaceID, docID, context)
	case entity.UsageEventReferenced:
		// Would need fromDocID - simplified for now
		return h.usageTracker.RecordReference(ctx, workspaceID, docID, entity.DocumentID(""))
	case entity.UsageEventIndexed:
		return h.usageTracker.RecordIndexed(ctx, workspaceID, docID)
	default:
		return nil
	}
}

// GetUsageStats gets usage statistics.
func (h *KnowledgeHandler) GetUsageStats(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, since time.Time) (*entity.DocumentUsageStats, error) {
	return h.usageAnalytics.GetUsageStats(ctx, workspaceID, docID, since)
}

// GetCoOccurringDocuments gets documents used together.
func (h *KnowledgeHandler) GetCoOccurringDocuments(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, limit int, since time.Time) ([]entity.DocumentID, error) {
	return h.usageAnalytics.GetCoOccurringDocuments(ctx, workspaceID, docID, limit, since)
}

// GetFrequentlyUsedDocuments gets frequently used documents.
func (h *KnowledgeHandler) GetFrequentlyUsedDocuments(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, limit int) ([]entity.DocumentID, error) {
	return h.usageAnalytics.GetFrequentlyUsedDocuments(ctx, workspaceID, since, limit)
}

// GetRecentlyUsedDocuments gets recently used documents.
func (h *KnowledgeHandler) GetRecentlyUsedDocuments(ctx context.Context, workspaceID entity.WorkspaceID, limit int) ([]entity.DocumentID, error) {
	return h.usageAnalytics.GetRecentlyUsedDocuments(ctx, workspaceID, limit)
}

// GetKnowledgeClusters gets knowledge clusters.
func (h *KnowledgeHandler) GetKnowledgeClusters(ctx context.Context, workspaceID entity.WorkspaceID, minClusterSize int, since time.Time) (map[entity.DocumentID][]entity.DocumentID, error) {
	return h.usageAnalytics.GetKnowledgeClusters(ctx, workspaceID, minClusterSize, since)
}

// QueryDocuments executes a document query.
func (h *KnowledgeHandler) QueryDocuments(ctx context.Context, workspaceID entity.WorkspaceID, qb *query.QueryBuilder) (*query.QueryResult, error) {
	return qb.Execute(ctx, h.queryExecutor)
}

// GetDocument retrieves a document by ID (helper for adapters).
func (h *KnowledgeHandler) GetDocument(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) (*entity.Document, error) {
	return h.docRepo.GetDocument(ctx, workspaceID, docID)
}

// GetFacets executes facet requests and returns results.
func (h *KnowledgeHandler) GetFacets(ctx context.Context, workspaceID entity.WorkspaceID, facetReqs []query.FacetRequest, fileIDs []entity.FileID) ([]*query.FacetResult, error) {
	results := make([]*query.FacetResult, 0, len(facetReqs))
	for _, req := range facetReqs {
		// Check for cancellation before processing each facet
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		result, err := h.facetExecutor.ExecuteFacet(ctx, workspaceID, req, fileIDs)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// AddDocumentRelationship adds a relationship between documents.
func (h *KnowledgeHandler) AddDocumentRelationship(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.DocumentRelationship) error {
	return h.relRepo.Create(ctx, workspaceID, rel)
}

// RemoveDocumentRelationship removes a relationship.
func (h *KnowledgeHandler) RemoveDocumentRelationship(ctx context.Context, workspaceID entity.WorkspaceID, fromDocID, toDocID entity.DocumentID, relType entity.RelationshipType) error {
	return h.relRepo.DeleteByDocuments(ctx, workspaceID, fromDocID, toDocID, relType)
}

// GetDocumentRelationships gets relationships for a document.
func (h *KnowledgeHandler) GetDocumentRelationships(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType *entity.RelationshipType) ([]*entity.DocumentRelationship, error) {
	if relType != nil {
		// Get outgoing relationships of specific type
		return h.relRepo.GetOutgoing(ctx, workspaceID, docID, *relType)
	}
	// Get all outgoing relationships
	return h.relRepo.GetAllOutgoing(ctx, workspaceID, docID)
}

// GetRelatedDocuments gets related documents.
func (h *KnowledgeHandler) GetRelatedDocuments(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType *entity.RelationshipType) ([]entity.DocumentID, error) {
	if relType != nil {
		return h.relRepo.GetRelated(ctx, workspaceID, docID, *relType)
	}
	// If no type specified, get all related (would need a method for this)
	// For now, use references as default
	return h.relRepo.GetRelated(ctx, workspaceID, docID, entity.RelationshipReferences)
}

// TraverseRelationships traverses relationships.
func (h *KnowledgeHandler) TraverseRelationships(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID, relType entity.RelationshipType, maxDepth int) ([]entity.DocumentID, error) {
	return h.relationshipTraverser.Traverse(ctx, workspaceID, docID, relType, maxDepth)
}

// FindPath finds a path between documents.
func (h *KnowledgeHandler) FindPath(ctx context.Context, workspaceID entity.WorkspaceID, fromDocID, toDocID entity.DocumentID, maxDepth int) ([]entity.DocumentID, error) {
	return h.relationshipTraverser.FindPath(ctx, workspaceID, fromDocID, toDocID, maxDepth)
}

// AddProjectRelationship adds a relationship between projects.
func (h *KnowledgeHandler) AddProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, rel *entity.ProjectRelationship) error {
	return h.relRepo.AddProjectRelationship(ctx, workspaceID, rel)
}

// RemoveProjectRelationship removes a project relationship.
func (h *KnowledgeHandler) RemoveProjectRelationship(ctx context.Context, workspaceID entity.WorkspaceID, fromProjectID, toProjectID entity.ProjectID, relType entity.RelationshipType) error {
	return h.relRepo.RemoveProjectRelationship(ctx, workspaceID, fromProjectID, toProjectID, relType)
}

// GetProjectRelationships gets relationships for a project.
func (h *KnowledgeHandler) GetProjectRelationships(ctx context.Context, workspaceID entity.WorkspaceID, projectID entity.ProjectID, relType *entity.RelationshipType) ([]*entity.ProjectRelationship, error) {
	return h.relRepo.GetProjectRelationships(ctx, workspaceID, projectID, relType)
}

// GenerateGraph generates graph data for visualization.
func (h *KnowledgeHandler) GenerateGraph(ctx context.Context, workspaceID entity.WorkspaceID, projectID *entity.ProjectID, includeDocuments, includeRelationships bool) (*visualization.GraphData, error) {
	return visualization.GenerateGraph(
		ctx,
		workspaceID,
		h.projectRepo,
		h.docRepo,
		h.relRepo,
		projectID,
		includeDocuments,
		includeRelationships,
	)
}

// GenerateHeatmap generates heatmap data for visualization.
func (h *KnowledgeHandler) GenerateHeatmap(ctx context.Context, workspaceID entity.WorkspaceID, since time.Time, eventType *entity.UsageEventType) (*visualization.HeatmapData, error) {
	return visualization.GenerateHeatmap(
		ctx,
		workspaceID,
		h.docRepo,
		h.usageRepo,
		since,
		eventType,
	)
}

// GenerateNodes generates node data for visualization.
func (h *KnowledgeHandler) GenerateNodes(ctx context.Context, workspaceID entity.WorkspaceID, filter *visualization.NodeFilter) ([]visualization.Node, error) {
	return visualization.GenerateNodes(
		ctx,
		workspaceID,
		h.projectRepo,
		h.docRepo,
		h.stateRepo,
		h.usageRepo,
		filter,
	)
}
