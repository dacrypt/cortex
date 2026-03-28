package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/query"
	"github.com/dacrypt/cortex/backend/internal/application/visualization"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// KnowledgeServiceAdapter implements cortexv1.KnowledgeServiceServer.
type KnowledgeServiceAdapter struct {
	cortexv1.UnimplementedKnowledgeServiceServer
	handler *handlers.KnowledgeHandler
	logger  zerolog.Logger
}

// NewKnowledgeServiceAdapter creates a new knowledge service adapter.
func NewKnowledgeServiceAdapter(handler *handlers.KnowledgeHandler, logger zerolog.Logger) *KnowledgeServiceAdapter {
	return &KnowledgeServiceAdapter{
		handler: handler,
		logger:  logger.With().Str("component", "knowledge_adapter").Logger(),
	}
}

// CreateProject creates a new project.
func (a *KnowledgeServiceAdapter) CreateProject(ctx context.Context, req *cortexv1.CreateProjectRequest) (*cortexv1.Project, error) {
	if req == nil || req.WorkspaceId == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and name are required")
	}

	var parentID *entity.ProjectID
	if req.ParentId != nil && *req.ParentId != "" {
		id := entity.ProjectID(*req.ParentId)
		parentID = &id
	}

	// Determine nature
	nature := entity.NatureGeneric
	if req.Nature != nil && *req.Nature != "" {
		nature = entity.ProjectNature(*req.Nature)
		if !nature.IsValid() {
			nature = entity.NatureGeneric
		}
	}

	// Parse attributes if provided
	var attributes *entity.ProjectAttributes
	if req.Attributes != nil && *req.Attributes != "" {
		attributes = &entity.ProjectAttributes{}
		if err := attributes.FromJSON(*req.Attributes); err != nil {
			attributes = nil // Use default if parsing fails
		}
	}

	project, err := a.handler.CreateProjectWithNature(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		req.Name,
		req.GetDescription(),
		parentID,
		nature,
		attributes,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return projectToProto(project), nil
}

// GetProject retrieves a project.
func (a *KnowledgeServiceAdapter) GetProject(ctx context.Context, req *cortexv1.GetProjectRequest) (*cortexv1.Project, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	project, err := a.handler.GetProject(ctx, entity.WorkspaceID(req.WorkspaceId), entity.ProjectID(req.ProjectId))
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return projectToProto(project), nil
}

// UpdateProject updates a project.
func (a *KnowledgeServiceAdapter) UpdateProject(ctx context.Context, req *cortexv1.UpdateProjectRequest) (*cortexv1.Project, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	// Get existing project
	project, err := a.handler.GetProject(ctx, entity.WorkspaceID(req.WorkspaceId), entity.ProjectID(req.ProjectId))
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// Update fields
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.ParentId != nil {
		if *req.ParentId == "" {
			project.ParentID = nil
		} else {
			id := entity.ProjectID(*req.ParentId)
			project.ParentID = &id
		}
	}
	if req.Nature != nil && *req.Nature != "" {
		nature := entity.ProjectNature(*req.Nature)
		if nature.IsValid() {
			project.Nature = nature
		}
	}
	if req.Attributes != nil {
		if *req.Attributes == "" {
			project.Attributes = &entity.ProjectAttributes{}
		} else {
			attributes := &entity.ProjectAttributes{}
			if err := attributes.FromJSON(*req.Attributes); err == nil {
				project.Attributes = attributes
			}
		}
	}

	if err := a.handler.UpdateProject(ctx, entity.WorkspaceID(req.WorkspaceId), project); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return projectToProto(project), nil
}

// DeleteProject deletes a project.
func (a *KnowledgeServiceAdapter) DeleteProject(ctx context.Context, req *cortexv1.DeleteProjectRequest) (*cortexv1.DeleteProjectResult, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	if err := a.handler.DeleteProject(ctx, entity.WorkspaceID(req.WorkspaceId), entity.ProjectID(req.ProjectId)); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.DeleteProjectResult{Success: true}, nil
}

// ListProjects lists projects.
func (a *KnowledgeServiceAdapter) ListProjects(req *cortexv1.ListProjectsRequest, stream cortexv1.KnowledgeService_ListProjectsServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	var parentID *entity.ProjectID
	if req.ParentId != nil && *req.ParentId != "" {
		id := entity.ProjectID(*req.ParentId)
		parentID = &id
	}

	// Log the request for debugging
	a.logger.Info().
		Str("workspace_id", req.WorkspaceId).
		Interface("parent_id", parentID).
		Msg("ListProjects called")

	projects, err := a.handler.ListProjects(stream.Context(), entity.WorkspaceID(req.WorkspaceId), parentID)
	if err != nil {
		a.logger.Error().
			Err(err).
			Str("workspace_id", req.WorkspaceId).
			Msg("ListProjects handler error")
		return status.Error(codes.Internal, err.Error())
	}

	a.logger.Info().
		Str("workspace_id", req.WorkspaceId).
		Int("project_count", len(projects)).
		Msg("ListProjects returning projects")

	for i, project := range projects {
		a.logger.Debug().
			Str("workspace_id", req.WorkspaceId).
			Int("index", i).
			Str("project_id", project.ID.String()).
			Str("project_name", project.Name).
			Msg("Sending project to client")
		if err := stream.Send(projectToProto(project)); err != nil {
			a.logger.Error().
				Err(err).
				Str("workspace_id", req.WorkspaceId).
				Int("index", i).
				Msg("Failed to send project to client")
			return err
		}
	}

	a.logger.Info().
		Str("workspace_id", req.WorkspaceId).
		Int("total_sent", len(projects)).
		Msg("ListProjects completed successfully")

	return nil
}

// GetProjectChildren returns child projects.
func (a *KnowledgeServiceAdapter) GetProjectChildren(req *cortexv1.GetProjectChildrenRequest, stream cortexv1.KnowledgeService_GetProjectChildrenServer) error {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	projects, err := a.handler.GetProjectChildren(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, project := range projects {
		if err := stream.Send(projectToProto(project)); err != nil {
			return err
		}
	}

	return nil
}

// GetProjectParents returns parent projects.
func (a *KnowledgeServiceAdapter) GetProjectParents(req *cortexv1.GetProjectParentsRequest, stream cortexv1.KnowledgeService_GetProjectParentsServer) error {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	projects, err := a.handler.GetProjectParents(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, project := range projects {
		if err := stream.Send(projectToProto(project)); err != nil {
			return err
		}
	}

	return nil
}

// SetDocumentState sets a document's state.
func (a *KnowledgeServiceAdapter) SetDocumentState(ctx context.Context, req *cortexv1.SetDocumentStateRequest) (*cortexv1.DocumentStateEntry, error) {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	state := protoToDocumentState(req.State)
	if err := a.handler.SetDocumentState(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		state,
		req.GetReason(),
		req.GetChangedBy(),
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get the updated state
	currentState, err := a.handler.GetDocumentState(ctx, entity.WorkspaceID(req.WorkspaceId), entity.DocumentID(req.DocumentId))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	entry := &cortexv1.DocumentStateEntry{
		DocumentId: req.DocumentId,
		State:      documentStateToProto(currentState),
		ChangedAt:  time.Now().Unix(),
	}
	if req.Reason != nil {
		entry.Reason = req.Reason
	}
	if req.ChangedBy != nil {
		entry.ChangedBy = req.ChangedBy
	}

	return entry, nil
}

// GetDocumentState gets a document's current state.
func (a *KnowledgeServiceAdapter) GetDocumentState(ctx context.Context, req *cortexv1.GetDocumentStateRequest) (*cortexv1.DocumentStateEntry, error) {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	state, err := a.handler.GetDocumentState(ctx, entity.WorkspaceID(req.WorkspaceId), entity.DocumentID(req.DocumentId))
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// Create a simple state entry from the current state
	entry := &cortexv1.DocumentStateEntry{
		DocumentId: req.DocumentId,
		State:      documentStateToProto(state),
		ChangedAt:  time.Now().Unix(), // Would need to get actual timestamp from history
	}

	return entry, nil
}

// GetDocumentStateHistory gets a document's state history.
func (a *KnowledgeServiceAdapter) GetDocumentStateHistory(req *cortexv1.GetDocumentStateHistoryRequest, stream cortexv1.KnowledgeService_GetDocumentStateHistoryServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	transitions, err := a.handler.GetDocumentStateHistory(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, transition := range transitions {
		entry := &cortexv1.DocumentStateEntry{
			DocumentId: transition.DocumentID.String(),
			State:      documentStateToProto(transition.ToState),
			ChangedAt:  transition.ChangedAt.Unix(),
		}
		if transition.Reason != "" {
			entry.Reason = &transition.Reason
		}
		if transition.ChangedBy != "" {
			entry.ChangedBy = &transition.ChangedBy
		}
		if transition.FromState != nil {
			fromStateStr := transition.FromState.String()
			entry.ReplacesDocId = &fromStateStr // Using this field to indicate previous state
		}
		if err := stream.Send(entry); err != nil {
			return err
		}
	}

	return nil
}

// ListDocumentsByState lists documents by state.
func (a *KnowledgeServiceAdapter) ListDocumentsByState(req *cortexv1.ListDocumentsByStateRequest, stream cortexv1.KnowledgeService_ListDocumentsByStateServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	state := protoToDocumentState(req.State)
	docIDs, err := a.handler.ListDocumentsByState(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		state,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// RecordUsage records a usage event.
func (a *KnowledgeServiceAdapter) RecordUsage(ctx context.Context, req *cortexv1.RecordUsageRequest) (*cortexv1.RecordUsageResult, error) {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	eventType := protoToUsageEventType(req.EventType)
	if err := a.handler.RecordUsage(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		eventType,
		req.GetContext(),
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.RecordUsageResult{Success: true}, nil
}

// GetUsageStats gets usage statistics.
func (a *KnowledgeServiceAdapter) GetUsageStats(ctx context.Context, req *cortexv1.GetUsageStatsRequest) (*cortexv1.UsageStats, error) {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	var since time.Time
	if req.SinceUnix != nil {
		since = time.Unix(*req.SinceUnix, 0)
	} else {
		since = time.Now().AddDate(0, -1, 0) // Default to last month
	}

	stats, err := a.handler.GetUsageStats(ctx, entity.WorkspaceID(req.WorkspaceId), entity.DocumentID(req.DocumentId), since)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return usageStatsToProto(stats), nil
}

// GetCoOccurringDocuments gets documents used together.
func (a *KnowledgeServiceAdapter) GetCoOccurringDocuments(req *cortexv1.GetCoOccurringDocumentsRequest, stream cortexv1.KnowledgeService_GetCoOccurringDocumentsServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	var since time.Time
	if req.SinceUnix != nil {
		since = time.Unix(*req.SinceUnix, 0)
	} else {
		since = time.Now().AddDate(0, -1, 0)
	}

	docIDs, err := a.handler.GetCoOccurringDocuments(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		limit,
		since,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// GetFrequentlyUsedDocuments gets frequently used documents.
func (a *KnowledgeServiceAdapter) GetFrequentlyUsedDocuments(req *cortexv1.GetFrequentlyUsedDocumentsRequest, stream cortexv1.KnowledgeService_GetFrequentlyUsedDocumentsServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	var since time.Time
	if req.SinceUnix != nil {
		since = time.Unix(*req.SinceUnix, 0)
	} else {
		since = time.Now().AddDate(0, -1, 0)
	}

	docIDs, err := a.handler.GetFrequentlyUsedDocuments(stream.Context(), entity.WorkspaceID(req.WorkspaceId), since, limit)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// GetRecentlyUsedDocuments gets recently used documents.
func (a *KnowledgeServiceAdapter) GetRecentlyUsedDocuments(req *cortexv1.GetRecentlyUsedDocumentsRequest, stream cortexv1.KnowledgeService_GetRecentlyUsedDocumentsServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	docIDs, err := a.handler.GetRecentlyUsedDocuments(stream.Context(), entity.WorkspaceID(req.WorkspaceId), limit)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// GetKnowledgeClusters gets knowledge clusters.
func (a *KnowledgeServiceAdapter) GetKnowledgeClusters(ctx context.Context, req *cortexv1.GetKnowledgeClustersRequest) (*cortexv1.KnowledgeClusters, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	minClusterSize := int(req.MinClusterSize)
	if minClusterSize <= 0 {
		minClusterSize = 2
	}

	var since time.Time
	if req.SinceUnix != nil {
		since = time.Unix(*req.SinceUnix, 0)
	} else {
		since = time.Now().AddDate(0, -1, 0)
	}

	clusters, err := a.handler.GetKnowledgeClusters(ctx, entity.WorkspaceID(req.WorkspaceId), minClusterSize, since)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	result := &cortexv1.KnowledgeClusters{
		Clusters: make(map[string]*cortexv1.TemporalDocumentCluster),
	}

	for rootDocID, docIDs := range clusters {
		protoDocIDs := make([]string, len(docIDs))
		for i, docID := range docIDs {
			protoDocIDs[i] = docID.String()
		}
		result.Clusters[rootDocID.String()] = &cortexv1.TemporalDocumentCluster{
			RootDocumentId: rootDocID.String(),
			DocumentIds:    protoDocIDs,
			Size:           int32(len(docIDs)),
		}
	}

	return result, nil
}

// QueryDocuments executes a document query.
func (a *KnowledgeServiceAdapter) QueryDocuments(req *cortexv1.QueryDocumentsRequest, stream cortexv1.KnowledgeService_QueryDocumentsServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	workspaceID := entity.WorkspaceID(req.WorkspaceId)

	// Build query from request
	qb := query.Query(workspaceID)

	// Convert proto filters to query filters
	for _, protoFilter := range req.Filters {
		filter, err := a.protoToQueryFilter(protoFilter)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		qb.Filter(filter)
	}

	// Apply ordering
	if req.Ordering != nil {
		descending := req.Ordering.Descending
		field := req.Ordering.Field
		if field == "" {
			field = "updated_at"
		}
		qb.OrderBy(field, descending)
	}

	// Apply pagination
	if req.Limit != nil && *req.Limit > 0 {
		qb.Limit(int(*req.Limit))
	}
	if req.Offset != nil && *req.Offset >= 0 {
		qb.Offset(int(*req.Offset))
	}

	// Execute query
	result, err := a.handler.QueryDocuments(stream.Context(), workspaceID, qb)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	// Stream results with document info (ID + relativePath)
	for _, docID := range result.DocumentIDs {
		// Get document to retrieve relativePath
		doc, err := a.handler.GetDocument(stream.Context(), workspaceID, docID)
		if err != nil {
			// If document not found, skip it
			continue
		}

		docInfo := &cortexv1.DocumentInfo{
			Id:    docID.String(),
			Path:  doc.RelativePath,
			Title: doc.Title,
		}

		if err := stream.Send(docInfo); err != nil {
			return err
		}
	}

	return nil
}

// GetFacets executes facet requests and returns results.
func (a *KnowledgeServiceAdapter) GetFacets(ctx context.Context, req *cortexv1.GetFacetsRequest) (*cortexv1.FacetResults, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	if len(req.Facets) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one facet request is required")
	}

	workspaceID := entity.WorkspaceID(req.WorkspaceId)

	// Convert proto facet requests to query facet requests
	facetReqs := make([]query.FacetRequest, 0, len(req.Facets))
	for _, protoFacet := range req.Facets {
		facetType := protoToFacetType(protoFacet.Type)
		facetReqs = append(facetReqs, query.FacetRequest{
			Field: protoFacet.Field,
			Type:  facetType,
		})
	}

	// For now, we'll facet on all files
	// TODO: Apply filters to get filtered file set if filters are provided
	var fileIDs []entity.FileID

	// Execute facets
	results, err := a.handler.GetFacets(ctx, workspaceID, facetReqs, fileIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert results to proto
	protoResults := make([]*cortexv1.FacetResult, 0, len(results))
	for _, result := range results {
		protoResult := &cortexv1.FacetResult{
			Field: result.Field,
			Type:  facetTypeToProto(result.Type),
		}

		switch data := result.Data.(type) {
		case query.TermsFacetData:
			terms := make([]*cortexv1.TermCount, 0, len(data.Terms))
			for _, term := range data.Terms {
				terms = append(terms, &cortexv1.TermCount{
					Term:  term.Term,
					Count: int32(term.Count),
				})
			}
			protoResult.Data = &cortexv1.FacetResult_Terms{
				Terms: &cortexv1.TermsFacetData{Terms: terms},
			}

		case query.NumericRangeFacetData:
			ranges := make([]*cortexv1.NumericRangeCount, 0, len(data.Ranges))
			for _, rng := range data.Ranges {
				ranges = append(ranges, &cortexv1.NumericRangeCount{
					Label: rng.Label,
					Min:   rng.Min,
					Max:   rng.Max,
					Count: int32(rng.Count),
				})
			}
			protoResult.Data = &cortexv1.FacetResult_NumericRange{
				NumericRange: &cortexv1.NumericRangeFacetData{Ranges: ranges},
			}

		case query.DateRangeFacetData:
			ranges := make([]*cortexv1.DateRangeCount, 0, len(data.Ranges))
			for _, rng := range data.Ranges {
				var endUnix int64
				if !rng.End.IsZero() {
					endUnix = rng.End.UnixMilli()
				}
				ranges = append(ranges, &cortexv1.DateRangeCount{
					Label:     rng.Label,
					StartUnix: rng.Start.UnixMilli(),
					EndUnix:   endUnix,
					Count:     int32(rng.Count),
				})
			}
			protoResult.Data = &cortexv1.FacetResult_DateRange{
				DateRange: &cortexv1.DateRangeFacetData{Ranges: ranges},
			}
		}

		protoResults = append(protoResults, protoResult)
	}

	return &cortexv1.FacetResults{Results: protoResults}, nil
}

// protoToFacetType converts a proto facet type to a query facet type.
func protoToFacetType(protoType cortexv1.FacetType) query.FacetType {
	switch protoType {
	case cortexv1.FacetType_FACET_TYPE_TERMS:
		return query.FacetTypeTerms
	case cortexv1.FacetType_FACET_TYPE_NUMERIC_RANGE:
		return query.FacetTypeNumericRange
	case cortexv1.FacetType_FACET_TYPE_DATE_RANGE:
		return query.FacetTypeDateRange
	default:
		return query.FacetTypeTerms
	}
}

// facetTypeToProto converts a query facet type to a proto facet type.
func facetTypeToProto(facetType query.FacetType) cortexv1.FacetType {
	switch facetType {
	case query.FacetTypeTerms:
		return cortexv1.FacetType_FACET_TYPE_TERMS
	case query.FacetTypeNumericRange:
		return cortexv1.FacetType_FACET_TYPE_NUMERIC_RANGE
	case query.FacetTypeDateRange:
		return cortexv1.FacetType_FACET_TYPE_DATE_RANGE
	default:
		return cortexv1.FacetType_FACET_TYPE_UNSPECIFIED
	}
}

// protoToQueryFilter converts a proto filter to a query filter.
func (a *KnowledgeServiceAdapter) protoToQueryFilter(protoFilter *cortexv1.QueryFilter) (query.Filter, error) {
	if protoFilter == nil {
		return nil, status.Error(codes.InvalidArgument, "filter is required")
	}

	switch f := protoFilter.Filter.(type) {
	case *cortexv1.QueryFilter_Project:
		if f.Project == nil || f.Project.ProjectId == "" {
			return nil, status.Error(codes.InvalidArgument, "project filter requires project_id")
		}
		return &query.ProjectFilter{
			ProjectID:          entity.ProjectID(f.Project.ProjectId),
			IncludeSubprojects: f.Project.IncludeSubprojects,
		}, nil
	case *cortexv1.QueryFilter_State:
		if f.State == nil || len(f.State.States) == 0 {
			return nil, status.Error(codes.InvalidArgument, "state filter requires at least one state")
		}
		states := make([]entity.DocumentState, len(f.State.States))
		for i, s := range f.State.States {
			states[i] = protoToDocumentState(s)
		}
		return &query.StateFilter{States: states}, nil
	case *cortexv1.QueryFilter_Relationship:
		if f.Relationship == nil || f.Relationship.FromDocumentId == "" {
			return nil, status.Error(codes.InvalidArgument, "relationship filter requires from_document_id")
		}
		maxDepth := 1
		if f.Relationship.MaxDepth > 0 {
			maxDepth = int(f.Relationship.MaxDepth)
		}
		return &query.RelationshipFilter{
			FromDocument: entity.DocumentID(f.Relationship.FromDocumentId),
			RelType:      protoToRelationshipType(f.Relationship.Type),
			MaxDepth:     maxDepth,
		}, nil
	case *cortexv1.QueryFilter_Temporal:
		if f.Temporal == nil {
			return nil, status.Error(codes.InvalidArgument, "temporal filter is required")
		}
		var since time.Time
		if f.Temporal.SinceUnix > 0 {
			since = time.Unix(f.Temporal.SinceUnix, 0)
		}
		limit := 100
		if f.Temporal.Limit > 0 {
			limit = int(f.Temporal.Limit)
		}
		return &query.TemporalFilter{
			Since:     since,
			EventType: protoToUsageEventType(f.Temporal.EventType),
			Limit:     limit,
		}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown filter type")
	}
}

// QueryProjects executes a project query.
func (a *KnowledgeServiceAdapter) QueryProjects(req *cortexv1.QueryProjectsRequest, stream cortexv1.KnowledgeService_QueryProjectsServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	var parentID *entity.ProjectID
	if req.ParentId != nil && *req.ParentId != "" {
		id := entity.ProjectID(*req.ParentId)
		parentID = &id
	}

	projects, err := a.handler.ListProjects(stream.Context(), entity.WorkspaceID(req.WorkspaceId), parentID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, project := range projects {
		if err := stream.Send(projectToProto(project)); err != nil {
			return err
		}
	}

	return nil
}

// AddDocumentToProject associates a document with a project.
func (a *KnowledgeServiceAdapter) AddDocumentToProject(ctx context.Context, req *cortexv1.AddDocumentToProjectRequest) (*cortexv1.AddDocumentToProjectResult, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, project_id, and document_id are required")
	}

	role := protoToDocumentProjectRole(req.Role)
	err := a.handler.AddDocumentToProjectWithRole(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
		entity.DocumentID(req.DocumentId),
		role,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.AddDocumentToProjectResult{Success: true}, nil
}

// RemoveDocumentFromProject removes a document from a project.
func (a *KnowledgeServiceAdapter) RemoveDocumentFromProject(ctx context.Context, req *cortexv1.RemoveDocumentFromProjectRequest) (*cortexv1.RemoveDocumentFromProjectResult, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, project_id, and document_id are required")
	}

	err := a.handler.RemoveDocumentFromProject(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
		entity.DocumentID(req.DocumentId),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.RemoveDocumentFromProjectResult{Success: true}, nil
}

// GetProjectsForDocument returns all projects that contain a document.
func (a *KnowledgeServiceAdapter) GetProjectsForDocument(req *cortexv1.GetProjectsForDocumentRequest, stream cortexv1.KnowledgeService_GetProjectsForDocumentServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	projectIDs, err := a.handler.GetProjectsForDocument(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, projectID := range projectIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: projectID.String()}); err != nil {
			return err
		}
	}

	return nil
}

func (a *KnowledgeServiceAdapter) ListProjectAssignmentsByFile(req *cortexv1.ListProjectAssignmentsByFileRequest, stream cortexv1.KnowledgeService_ListProjectAssignmentsByFileServer) error {
	if req == nil || req.WorkspaceId == "" || req.FileId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and file_id are required")
	}

	assignments, err := a.handler.ListProjectAssignmentsByFile(stream.Context(), entity.WorkspaceID(req.WorkspaceId), entity.FileID(req.FileId))
	if err != nil {
		return status.Errorf(codes.Internal, "list project assignments by file: %v", err)
	}

	for _, assignment := range assignments {
		if err := stream.Send(projectAssignmentToProto(assignment)); err != nil {
			return err
		}
	}

	return nil
}

func (a *KnowledgeServiceAdapter) ListProjectAssignmentsByProject(req *cortexv1.ListProjectAssignmentsByProjectRequest, stream cortexv1.KnowledgeService_ListProjectAssignmentsByProjectServer) error {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	assignments, err := a.handler.ListProjectAssignmentsByProject(stream.Context(), entity.WorkspaceID(req.WorkspaceId), entity.ProjectID(req.ProjectId))
	if err != nil {
		return status.Errorf(codes.Internal, "list project assignments by project: %v", err)
	}

	for _, assignment := range assignments {
		if err := stream.Send(projectAssignmentToProto(assignment)); err != nil {
			return err
		}
	}

	return nil
}

func (a *KnowledgeServiceAdapter) UpdateProjectAssignmentStatus(ctx context.Context, req *cortexv1.UpdateProjectAssignmentStatusRequest) (*cortexv1.ProjectAssignment, error) {
	if req == nil || req.WorkspaceId == "" || req.FileId == "" || req.ProjectName == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, file_id, and project_name are required")
	}

	statusValue := protoToProjectAssignmentStatus(req.Status)
	if statusValue == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid project assignment status")
	}

	assignment, err := a.handler.UpdateProjectAssignmentStatus(ctx, entity.WorkspaceID(req.WorkspaceId), entity.FileID(req.FileId), req.ProjectName, statusValue)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update project assignment status: %v", err)
	}
	if assignment == nil {
		return nil, status.Error(codes.NotFound, "project assignment not found")
	}

	return projectAssignmentToProto(assignment), nil
}

// AddDocumentRelationship adds a relationship between documents.
func (a *KnowledgeServiceAdapter) AddDocumentRelationship(ctx context.Context, req *cortexv1.AddDocumentRelationshipRequest) (*cortexv1.DocumentRelationship, error) {
	if req == nil || req.WorkspaceId == "" || req.FromDocumentId == "" || req.ToDocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, from_document_id, and to_document_id are required")
	}

	relType := protoToRelationshipType(req.Type)
	rel := &entity.DocumentRelationship{
		ID:           entity.NewRelationshipID(),
		FromDocument: entity.DocumentID(req.FromDocumentId),
		ToDocument:   entity.DocumentID(req.ToDocumentId),
		Type:         relType,
		Strength:     1.0,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
	}

	if req.Strength != nil {
		rel.Strength = *req.Strength
	}
	if req.Description != nil && *req.Description != "" {
		rel.Metadata["description"] = *req.Description
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			rel.Metadata[k] = v
		}
	}

	if err := a.handler.AddDocumentRelationship(ctx, entity.WorkspaceID(req.WorkspaceId), rel); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return documentRelationshipToProto(rel), nil
}

// RemoveDocumentRelationship removes a relationship.
func (a *KnowledgeServiceAdapter) RemoveDocumentRelationship(ctx context.Context, req *cortexv1.RemoveDocumentRelationshipRequest) (*cortexv1.RemoveRelationshipResult, error) {
	if req == nil || req.WorkspaceId == "" || req.FromDocumentId == "" || req.ToDocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, from_document_id, and to_document_id are required")
	}

	relType := protoToRelationshipType(req.Type)
	if err := a.handler.RemoveDocumentRelationship(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.FromDocumentId),
		entity.DocumentID(req.ToDocumentId),
		relType,
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.RemoveRelationshipResult{Success: true}, nil
}

// GetDocumentRelationships gets relationships for a document.
func (a *KnowledgeServiceAdapter) GetDocumentRelationships(req *cortexv1.GetDocumentRelationshipsRequest, stream cortexv1.KnowledgeService_GetDocumentRelationshipsServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	var relType *entity.RelationshipType
	if req.Type != nil {
		t := protoToRelationshipType(*req.Type)
		relType = &t
	}

	rels, err := a.handler.GetDocumentRelationships(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		relType,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, rel := range rels {
		if err := stream.Send(documentRelationshipToProto(rel)); err != nil {
			return err
		}
	}

	return nil
}

// GetRelatedDocuments gets related documents.
func (a *KnowledgeServiceAdapter) GetRelatedDocuments(req *cortexv1.GetRelatedDocumentsRequest, stream cortexv1.KnowledgeService_GetRelatedDocumentsServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	var relType *entity.RelationshipType
	if req.Type != nil {
		t := protoToRelationshipType(*req.Type)
		relType = &t
	}

	docIDs, err := a.handler.GetRelatedDocuments(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		relType,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// TraverseRelationships traverses relationships.
func (a *KnowledgeServiceAdapter) TraverseRelationships(req *cortexv1.TraverseRelationshipsRequest, stream cortexv1.KnowledgeService_TraverseRelationshipsServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	maxDepth := int(req.MaxDepth)
	if maxDepth <= 0 {
		maxDepth = 1
	}

	relType := protoToRelationshipType(req.Type)
	docIDs, err := a.handler.TraverseRelationships(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		relType,
		maxDepth,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, docID := range docIDs {
		if err := stream.Send(&cortexv1.DocumentID{Id: docID.String()}); err != nil {
			return err
		}
	}

	return nil
}

// FindPath finds a path between documents.
func (a *KnowledgeServiceAdapter) FindPath(ctx context.Context, req *cortexv1.FindPathRequest) (*cortexv1.PathResult, error) {
	if req == nil || req.WorkspaceId == "" || req.FromDocumentId == "" || req.ToDocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, from_document_id, and to_document_id are required")
	}

	maxDepth := int(req.MaxDepth)
	if maxDepth <= 0 {
		maxDepth = 5
	}

	path, err := a.handler.FindPath(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.FromDocumentId),
		entity.DocumentID(req.ToDocumentId),
		maxDepth,
	)
	if err != nil {
		return &cortexv1.PathResult{Found: false}, nil
	}

	protoDocIDs := make([]string, len(path))
	for i, docID := range path {
		protoDocIDs[i] = docID.String()
	}

	return &cortexv1.PathResult{
		DocumentIds: protoDocIDs,
		Found:       true,
	}, nil
}

// AddProjectRelationship adds a relationship between projects.
func (a *KnowledgeServiceAdapter) AddProjectRelationship(ctx context.Context, req *cortexv1.AddProjectRelationshipRequest) (*cortexv1.ProjectRelationship, error) {
	if req == nil || req.WorkspaceId == "" || req.FromProjectId == "" || req.ToProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, from_project_id, and to_project_id are required")
	}

	relType := protoToRelationshipType(req.Type)
	rel := &entity.ProjectRelationship{
		FromProjectID: entity.ProjectID(req.FromProjectId),
		ToProjectID:   entity.ProjectID(req.ToProjectId),
		Type:          relType,
		Description:   req.GetDescription(),
		CreatedAt:     time.Now(),
	}

	if err := a.handler.AddProjectRelationship(ctx, entity.WorkspaceID(req.WorkspaceId), rel); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return projectRelationshipToProto(rel), nil
}

// RemoveProjectRelationship removes a project relationship.
func (a *KnowledgeServiceAdapter) RemoveProjectRelationship(ctx context.Context, req *cortexv1.RemoveProjectRelationshipRequest) (*cortexv1.RemoveRelationshipResult, error) {
	if req == nil || req.WorkspaceId == "" || req.FromProjectId == "" || req.ToProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, from_project_id, and to_project_id are required")
	}

	relType := protoToRelationshipType(req.Type)
	if err := a.handler.RemoveProjectRelationship(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.FromProjectId),
		entity.ProjectID(req.ToProjectId),
		relType,
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.RemoveRelationshipResult{Success: true}, nil
}

// GetProjectRelationships gets relationships for a project.
func (a *KnowledgeServiceAdapter) GetProjectRelationships(req *cortexv1.GetProjectRelationshipsRequest, stream cortexv1.KnowledgeService_GetProjectRelationshipsServer) error {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and project_id are required")
	}

	var relType *entity.RelationshipType
	if req.Type != nil {
		t := protoToRelationshipType(*req.Type)
		relType = &t
	}

	rels, err := a.handler.GetProjectRelationships(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
		relType,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, rel := range rels {
		if err := stream.Send(projectRelationshipToProto(rel)); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions for proto conversion

func projectToProto(p *entity.Project) *cortexv1.Project {
	proto := &cortexv1.Project{
		Id:          p.ID.String(),
		WorkspaceId: p.WorkspaceID.String(),
		Name:        p.Name,
		Nature:      p.Nature.String(),
		CreatedAt:   p.CreatedAt.Unix(),
		UpdatedAt:   p.UpdatedAt.Unix(),
	}

	if p.Description != "" {
		proto.Description = &p.Description
	}
	if p.ParentID != nil {
		parentIDStr := p.ParentID.String()
		proto.ParentId = &parentIDStr
	}
	if p.Attributes != nil {
		attrsJSON, err := p.Attributes.ToJSON()
		if err == nil && attrsJSON != "{}" {
			proto.Attributes = &attrsJSON
		}
	}

	return proto
}

// documentStateEntryToProto is no longer needed - we create entries directly from transitions

func documentStateToProto(s entity.DocumentState) cortexv1.DocumentState {
	switch s {
	case entity.DocumentStateDraft:
		return cortexv1.DocumentState_DOCUMENT_STATE_DRAFT
	case entity.DocumentStateActive:
		return cortexv1.DocumentState_DOCUMENT_STATE_ACTIVE
	case entity.DocumentStateReplaced:
		return cortexv1.DocumentState_DOCUMENT_STATE_REPLACED
	case entity.DocumentStateArchived:
		return cortexv1.DocumentState_DOCUMENT_STATE_ARCHIVED
	default:
		return cortexv1.DocumentState_DOCUMENT_STATE_UNSPECIFIED
	}
}

func protoToDocumentState(s cortexv1.DocumentState) entity.DocumentState {
	switch s {
	case cortexv1.DocumentState_DOCUMENT_STATE_DRAFT:
		return entity.DocumentStateDraft
	case cortexv1.DocumentState_DOCUMENT_STATE_ACTIVE:
		return entity.DocumentStateActive
	case cortexv1.DocumentState_DOCUMENT_STATE_REPLACED:
		return entity.DocumentStateReplaced
	case cortexv1.DocumentState_DOCUMENT_STATE_ARCHIVED:
		return entity.DocumentStateArchived
	default:
		return entity.DocumentStateDraft
	}
}

func usageStatsToProto(s *entity.DocumentUsageStats) *cortexv1.UsageStats {
	proto := &cortexv1.UsageStats{
		DocumentId:      s.DocumentID.String(),
		TotalEvents:     int32(s.AccessCount),
		FrequencyPerDay: s.Frequency,
	}

	if !s.LastAccessed.IsZero() {
		proto.LastUsedAt = s.LastAccessed.Unix()
	}
	if !s.FirstAccessed.IsZero() {
		proto.FirstUsedAt = s.FirstAccessed.Unix()
	}

	// Convert co-occurrences to events_by_type (simplified mapping)
	proto.EventsByType = make(map[string]int32)
	// Note: CoOccurrences maps DocumentID to count, not event type to count
	// For now, we'll use AccessCount as a single event type count
	proto.EventsByType["opened"] = int32(s.AccessCount)
	if len(s.CoOccurrences) > 0 {
		proto.EventsByType["co_occurred"] = int32(len(s.CoOccurrences))
	}

	return proto
}

func protoToUsageEventType(et cortexv1.UsageEventType) entity.UsageEventType {
	switch et {
	case cortexv1.UsageEventType_USAGE_EVENT_TYPE_OPENED:
		return entity.UsageEventOpened
	case cortexv1.UsageEventType_USAGE_EVENT_TYPE_EDITED:
		return entity.UsageEventEdited
	case cortexv1.UsageEventType_USAGE_EVENT_TYPE_SEARCHED:
		return entity.UsageEventSearched
	case cortexv1.UsageEventType_USAGE_EVENT_TYPE_REFERENCED:
		return entity.UsageEventReferenced
	case cortexv1.UsageEventType_USAGE_EVENT_TYPE_INDEXED:
		return entity.UsageEventIndexed
	default:
		return entity.UsageEventOpened
	}
}

func documentRelationshipToProto(r *entity.DocumentRelationship) *cortexv1.DocumentRelationship {
	proto := &cortexv1.DocumentRelationship{
		FromDocumentId: r.FromDocument.String(),
		ToDocumentId:   r.ToDocument.String(),
		Type:           relationshipTypeToProto(r.Type),
		Strength:       r.Strength,
		CreatedAt:      r.CreatedAt.Unix(),
	}

	// Extract description from metadata if present
	if r.Metadata != nil {
		proto.Metadata = make(map[string]string)
		for k, v := range r.Metadata {
			if str, ok := v.(string); ok {
				proto.Metadata[k] = str
				if k == "description" {
					proto.Description = &str
				}
			}
		}
	}

	return proto
}

func projectRelationshipToProto(r *entity.ProjectRelationship) *cortexv1.ProjectRelationship {
	proto := &cortexv1.ProjectRelationship{
		FromProjectId: r.FromProjectID.String(),
		ToProjectId:   r.ToProjectID.String(),
		Type:          relationshipTypeToProto(r.Type),
		CreatedAt:     r.CreatedAt.Unix(),
	}

	if r.Description != "" {
		proto.Description = &r.Description
	}

	return proto
}

func relationshipTypeToProto(t entity.RelationshipType) cortexv1.RelationshipType {
	switch t {
	case entity.RelationshipReplaces:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_REPLACES
	case entity.RelationshipDependsOn:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_DEPENDS_ON
	case entity.RelationshipBelongsTo:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_BELONGS_TO
	case entity.RelationshipReferences:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_REFERENCES
	case entity.RelationshipParentOf:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_PARENT_OF
	default:
		return cortexv1.RelationshipType_RELATIONSHIP_TYPE_UNSPECIFIED
	}
}

func protoToRelationshipType(t cortexv1.RelationshipType) entity.RelationshipType {
	switch t {
	case cortexv1.RelationshipType_RELATIONSHIP_TYPE_REPLACES:
		return entity.RelationshipReplaces
	case cortexv1.RelationshipType_RELATIONSHIP_TYPE_DEPENDS_ON:
		return entity.RelationshipDependsOn
	case cortexv1.RelationshipType_RELATIONSHIP_TYPE_BELONGS_TO:
		return entity.RelationshipBelongsTo
	case cortexv1.RelationshipType_RELATIONSHIP_TYPE_REFERENCES:
		return entity.RelationshipReferences
	case cortexv1.RelationshipType_RELATIONSHIP_TYPE_PARENT_OF:
		return entity.RelationshipParentOf
	default:
		return entity.RelationshipReferences
	}
}

func protoToProjectAssignmentStatus(s cortexv1.ProjectAssignmentStatus) entity.ProjectAssignmentStatus {
	switch s {
	case cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_AUTO:
		return entity.ProjectAssignmentAuto
	case cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_SUGGESTED:
		return entity.ProjectAssignmentSuggested
	case cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_REJECTED:
		return entity.ProjectAssignmentRejected
	case cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_MANUAL:
		return entity.ProjectAssignmentManual
	default:
		return ""
	}
}

// GenerateGraph generates graph data for visualization.
func (a *KnowledgeServiceAdapter) GenerateGraph(ctx context.Context, req *cortexv1.GenerateGraphRequest) (*cortexv1.GraphData, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	var projectID *entity.ProjectID
	if req.ProjectId != nil && *req.ProjectId != "" {
		id := entity.ProjectID(*req.ProjectId)
		projectID = &id
	}

	graphData, err := a.handler.GenerateGraph(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		projectID,
		req.IncludeDocuments,
		req.IncludeRelationships,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return graphDataToProto(graphData), nil
}

// GenerateHeatmap generates heatmap data for visualization.
func (a *KnowledgeServiceAdapter) GenerateHeatmap(ctx context.Context, req *cortexv1.GenerateHeatmapRequest) (*cortexv1.HeatmapData, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	since := time.UnixMilli(req.SinceTimestamp)
	var eventType *entity.UsageEventType
	if req.EventType != nil && *req.EventType != "" {
		// Convert string to UsageEventType enum value
		etStr := *req.EventType
		var et entity.UsageEventType
		switch etStr {
		case "opened", "USAGE_EVENT_TYPE_OPENED":
			et = entity.UsageEventOpened
		case "edited", "USAGE_EVENT_TYPE_EDITED":
			et = entity.UsageEventEdited
		case "searched", "USAGE_EVENT_TYPE_SEARCHED":
			et = entity.UsageEventSearched
		case "referenced", "USAGE_EVENT_TYPE_REFERENCED":
			et = entity.UsageEventReferenced
		case "indexed", "USAGE_EVENT_TYPE_INDEXED":
			et = entity.UsageEventIndexed
		default:
			// Unknown type, skip
			eventType = nil
		}
		if eventType == nil {
			eventType = &et
		}
	}

	heatmapData, err := a.handler.GenerateHeatmap(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		since,
		eventType,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return heatmapDataToProto(heatmapData), nil
}

// GenerateNodes generates node data for visualization.
func (a *KnowledgeServiceAdapter) GenerateNodes(req *cortexv1.GenerateNodesRequest, stream cortexv1.KnowledgeService_GenerateNodesServer) error {
	if req == nil || req.WorkspaceId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	filter := &visualization.NodeFilter{}
	if req.ProjectId != nil && *req.ProjectId != "" {
		id := entity.ProjectID(*req.ProjectId)
		filter.ProjectID = &id
	}
	if len(req.DocumentStates) > 0 {
		filter.DocumentStates = make([]entity.DocumentState, len(req.DocumentStates))
		for i, state := range req.DocumentStates {
			filter.DocumentStates[i] = protoToDocumentState(state)
		}
	}
	if req.MinUsageCount != nil {
		filter.MinUsageCount = int(*req.MinUsageCount)
	}

	nodes, err := a.handler.GenerateNodes(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		filter,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, node := range nodes {
		if err := stream.Send(nodeToProto(node)); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions for proto conversion

func graphDataToProto(data *visualization.GraphData) *cortexv1.GraphData {
	if data == nil {
		return nil
	}

	nodes := make([]*cortexv1.GraphNode, len(data.Nodes))
	for i, node := range data.Nodes {
		nodes[i] = nodeToProto(node)
	}

	edges := make([]*cortexv1.GraphEdge, len(data.Edges))
	for i, edge := range data.Edges {
		edges[i] = &cortexv1.GraphEdge{
			From:     edge.From,
			To:       edge.To,
			Type:     edge.Type,
			Weight:   edge.Weight,
			Metadata: stringMapToStringMap(edge.Metadata),
		}
	}

	metadata := make(map[string]string)
	for k, v := range data.Metadata {
		if str, ok := v.(string); ok {
			metadata[k] = str
		} else {
			metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	return &cortexv1.GraphData{
		Nodes:    nodes,
		Edges:    edges,
		Metadata: metadata,
	}
}

func nodeToProto(node visualization.Node) *cortexv1.GraphNode {
	return &cortexv1.GraphNode{
		Id:       node.ID,
		Type:     node.Type,
		Label:    node.Label,
		Metadata: stringMapToStringMap(node.Metadata),
	}
}

func heatmapDataToProto(data *visualization.HeatmapData) *cortexv1.HeatmapData {
	if data == nil {
		return nil
	}

	docInfos := make([]*cortexv1.DocumentInfo, len(data.Documents))
	for i, doc := range data.Documents {
		docInfos[i] = &cortexv1.DocumentInfo{
			Id:    doc.ID.String(),
			Path:  doc.Path,
			Title: doc.Title,
		}
	}

	rows := make([]*cortexv1.HeatmapRow, len(data.Matrix))
	for i, row := range data.Matrix {
		rows[i] = &cortexv1.HeatmapRow{
			Values: row,
		}
	}

	return &cortexv1.HeatmapData{
		Documents:      docInfos,
		Matrix:         rows,
		StartTimestamp: data.TimeRange.Start.UnixMilli(),
		EndTimestamp:   data.TimeRange.End.UnixMilli(),
	}
}

func stringMapToStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// GetAllProjectMembershipsForDocument returns all project memberships for a document.
func (a *KnowledgeServiceAdapter) GetAllProjectMembershipsForDocument(req *cortexv1.GetAllProjectMembershipsRequest, stream cortexv1.KnowledgeService_GetAllProjectMembershipsForDocumentServer) error {
	if req == nil || req.WorkspaceId == "" || req.DocumentId == "" {
		return status.Error(codes.InvalidArgument, "workspace_id and document_id are required")
	}

	memberships, err := a.handler.GetAllProjectMembershipsForDocument(
		stream.Context(),
		entity.WorkspaceID(req.WorkspaceId),
		entity.DocumentID(req.DocumentId),
		req.IncludeArchived,
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	for _, membership := range memberships {
		if err := stream.Send(documentProjectMembershipToProto(membership)); err != nil {
			return err
		}
	}

	return nil
}

// UpdateDocumentProjectRole updates the role of a document within a project.
func (a *KnowledgeServiceAdapter) UpdateDocumentProjectRole(ctx context.Context, req *cortexv1.UpdateDocumentProjectRoleRequest) (*cortexv1.DocumentProjectMembership, error) {
	if req == nil || req.WorkspaceId == "" || req.ProjectId == "" || req.DocumentId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, project_id, and document_id are required")
	}

	role := protoToDocumentProjectRole(req.Role)
	membership, err := a.handler.UpdateDocumentProjectRole(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entity.ProjectID(req.ProjectId),
		entity.DocumentID(req.DocumentId),
		role,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return documentProjectMembershipToProto(membership), nil
}

// Helper functions for poly-hierarchy

func protoToDocumentProjectRole(role cortexv1.DocumentProjectRole) entity.ProjectDocumentRole {
	switch role {
	case cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_PRIMARY:
		return entity.ProjectDocumentRolePrimary
	case cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_RELATED:
		return entity.ProjectDocumentRoleReference
	case cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_ARCHIVE:
		return entity.ProjectDocumentRoleArchive
	default:
		return entity.ProjectDocumentRolePrimary
	}
}

func documentProjectRoleToProto(role entity.ProjectDocumentRole) cortexv1.DocumentProjectRole {
	switch role {
	case entity.ProjectDocumentRolePrimary:
		return cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_PRIMARY
	case entity.ProjectDocumentRoleReference:
		return cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_RELATED
	case entity.ProjectDocumentRoleArchive:
		return cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_ARCHIVE
	default:
		return cortexv1.DocumentProjectRole_DOCUMENT_PROJECT_ROLE_UNSPECIFIED
	}
}

func documentProjectMembershipToProto(m *handlers.DocumentProjectMembership) *cortexv1.DocumentProjectMembership {
	if m == nil {
		return nil
	}
	return &cortexv1.DocumentProjectMembership{
		WorkspaceId: m.WorkspaceID.String(),
		ProjectId:   m.ProjectID.String(),
		ProjectName: m.ProjectName,
		DocumentId:  m.DocumentID.String(),
		Role:        documentProjectRoleToProto(m.Role),
		Score:       m.Score,
		AddedAt:     m.AddedAt.Unix(),
	}
}
