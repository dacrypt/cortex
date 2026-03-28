package adapters

import (
	"context"
	"encoding/json"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EntityServiceAdapter implements cortexv1.EntityServiceServer.
type EntityServiceAdapter struct {
	cortexv1.UnimplementedEntityServiceServer
	handler *handlers.EntityHandler
}

// NewEntityServiceAdapter creates a new entity service adapter.
func NewEntityServiceAdapter(handler *handlers.EntityHandler) *EntityServiceAdapter {
	return &EntityServiceAdapter{
		handler: handler,
	}
}

// GetEntity retrieves a single entity by ID.
func (a *EntityServiceAdapter) GetEntity(ctx context.Context, req *cortexv1.GetEntityRequest) (*cortexv1.Entity, error) {
	if req == nil || req.WorkspaceId == "" || req.Id == nil {
		return nil, status.Error(codes.InvalidArgument, "workspace_id and id are required")
	}

	entityID := entity.NewEntityID(
		protoToEntityType(req.Id.Type),
		req.Id.Id,
	)

	ent, err := a.handler.GetEntity(ctx, entity.WorkspaceID(req.WorkspaceId), entityID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return entityToProto(ent)
}

// ListEntities lists entities with filters.
func (a *EntityServiceAdapter) ListEntities(ctx context.Context, req *cortexv1.ListEntitiesRequest) (*cortexv1.ListEntitiesResponse, error) {
	if req == nil || req.WorkspaceId == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}

	filters := protoToEntityFilters(req)
	entities, err := a.handler.ListEntities(ctx, entity.WorkspaceID(req.WorkspaceId), filters)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoEntities := make([]*cortexv1.Entity, 0, len(entities))
	for _, ent := range entities {
		proto, err := entityToProto(ent)
		if err != nil {
			continue
		}
		protoEntities = append(protoEntities, proto)
	}

	return &cortexv1.ListEntitiesResponse{
		Entities: protoEntities,
		Total:    int32(len(protoEntities)),
		HasMore:  false, // TODO: Implement pagination
	}, nil
}

// GetEntitiesByFacet retrieves entities matching a facet value.
func (a *EntityServiceAdapter) GetEntitiesByFacet(ctx context.Context, req *cortexv1.GetEntitiesByFacetRequest) (*cortexv1.ListEntitiesResponse, error) {
	if req == nil || req.WorkspaceId == "" || req.Facet == "" || req.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, facet, and value are required")
	}

	entityTypes := make([]entity.EntityType, 0, len(req.Types))
	for _, t := range req.Types {
		entityTypes = append(entityTypes, protoToEntityType(t))
	}
	if len(entityTypes) == 0 {
		entityTypes = []entity.EntityType{entity.EntityTypeFile, entity.EntityTypeFolder, entity.EntityTypeProject}
	}

	entities, err := a.handler.GetEntitiesByFacet(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		req.Facet,
		req.Value,
		entityTypes,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Apply pagination
	start := int(req.Offset)
	end := start + int(req.Limit)
	if end > len(entities) {
		end = len(entities)
	}
	if start < len(entities) {
		entities = entities[start:end]
	} else {
		entities = []*entity.Entity{}
	}

	protoEntities := make([]*cortexv1.Entity, 0, len(entities))
	for _, ent := range entities {
		proto, err := entityToProto(ent)
		if err != nil {
			continue
		}
		protoEntities = append(protoEntities, proto)
	}

	return &cortexv1.ListEntitiesResponse{
		Entities: protoEntities,
		Total:    int32(len(protoEntities)),
		HasMore:  end < len(entities),
	}, nil
}

// UpdateEntityMetadata updates semantic metadata for an entity.
func (a *EntityServiceAdapter) UpdateEntityMetadata(ctx context.Context, req *cortexv1.UpdateEntityMetadataRequest) (*cortexv1.Entity, error) {
	if req == nil || req.WorkspaceId == "" || req.Id == nil || req.Metadata == nil {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, id, and metadata are required")
	}

	entityID := entity.NewEntityID(
		protoToEntityType(req.Id.Type),
		req.Id.Id,
	)

	metadata := protoToEntityMetadata(req.Metadata)

	err := a.handler.UpdateEntityMetadata(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		entityID,
		metadata,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Get updated entity
	ent, err := a.handler.GetEntity(ctx, entity.WorkspaceID(req.WorkspaceId), entityID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return entityToProto(ent)
}

// CountEntitiesByFacet counts entities matching a facet value.
func (a *EntityServiceAdapter) CountEntitiesByFacet(ctx context.Context, req *cortexv1.CountEntitiesByFacetRequest) (*cortexv1.CountEntitiesByFacetResponse, error) {
	if req == nil || req.WorkspaceId == "" || req.Facet == "" || req.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id, facet, and value are required")
	}

	entityTypes := make([]entity.EntityType, 0, len(req.Types))
	for _, t := range req.Types {
		entityTypes = append(entityTypes, protoToEntityType(t))
	}
	if len(entityTypes) == 0 {
		entityTypes = []entity.EntityType{entity.EntityTypeFile, entity.EntityTypeFolder, entity.EntityTypeProject}
	}

	count, err := a.handler.CountEntitiesByFacet(
		ctx,
		entity.WorkspaceID(req.WorkspaceId),
		req.Facet,
		req.Value,
		entityTypes,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &cortexv1.CountEntitiesByFacetResponse{
		Count: int32(count),
	}, nil
}

// entityToProto converts an entity to protobuf format.
func entityToProto(ent *entity.Entity) (*cortexv1.Entity, error) {
	if ent == nil {
		return nil, status.Error(codes.Internal, "entity is nil")
	}

	proto := &cortexv1.Entity{
		Id: &cortexv1.EntityID{
			Type: entityTypeToProto(ent.Type),
			Id:   ent.ID.ID,
		},
		Type:        entityTypeToProto(ent.Type),
		WorkspaceId: ent.WorkspaceID.String(),
		Name:        ent.Name,
		Path:        ent.Path,
		CreatedAt:   ent.CreatedAt.UnixMilli(),
		UpdatedAt:   ent.UpdatedAt.UnixMilli(),
	}

	if ent.Description != nil {
		proto.Description = ent.Description
	}
	if ent.ModifiedAt != nil {
		modifiedAt := ent.ModifiedAt.UnixMilli()
		proto.ModifiedAt = &modifiedAt
	}
	if ent.Size != nil {
		proto.Size = ent.Size
	}

	// Semantic metadata
	if len(ent.Tags) > 0 {
		proto.Tags = ent.Tags
	}
	if len(ent.Projects) > 0 {
		proto.Projects = ent.Projects
	}
	if ent.Language != nil {
		proto.Language = ent.Language
	}
	if ent.Category != nil {
		proto.Category = ent.Category
	}
	if ent.Author != nil {
		proto.Author = ent.Author
	}
	if ent.Owner != nil {
		proto.Owner = ent.Owner
	}
	if ent.Location != nil {
		proto.Location = ent.Location
	}
	if ent.PublicationYear != nil {
		year := int32(*ent.PublicationYear)
		proto.PublicationYear = &year
	}
	if ent.Complexity != nil {
		proto.Complexity = ent.Complexity
	}
	if ent.LinesOfCode != nil {
		loc := int32(*ent.LinesOfCode)
		proto.LinesOfCode = &loc
	}
	if ent.QualityScore != nil {
		proto.QualityScore = ent.QualityScore
	}
	if ent.Status != nil {
		proto.Status = ent.Status
	}
	if ent.Priority != nil {
		proto.Priority = ent.Priority
	}
	if ent.Visibility != nil {
		proto.Visibility = ent.Visibility
	}
	if ent.AISummary != nil {
		proto.AiSummary = ent.AISummary
	}
	if len(ent.AIKeywords) > 0 {
		proto.AiKeywords = ent.AIKeywords
	}

	// Type-specific data
	if ent.FileData != nil {
		fileDataJSON, err := json.Marshal(ent.FileData)
		if err == nil {
			fileDataStr := string(fileDataJSON)
			proto.FileData = &fileDataStr
		}
	}
	if ent.FolderData != nil {
		folderDataJSON, err := json.Marshal(ent.FolderData)
		if err == nil {
			folderDataStr := string(folderDataJSON)
			proto.FolderData = &folderDataStr
		}
	}
	if ent.ProjectData != nil {
		projectDataJSON, err := json.Marshal(ent.ProjectData)
		if err == nil {
			projectDataStr := string(projectDataJSON)
			proto.ProjectData = &projectDataStr
		}
	}

	return proto, nil
}

// protoToEntityFilters converts protobuf filters to repository filters.
func protoToEntityFilters(req *cortexv1.ListEntitiesRequest) repository.EntityFilters {
	filters := repository.EntityFilters{}

	if len(req.Types) > 0 {
		filters.Types = make([]entity.EntityType, 0, len(req.Types))
		for _, t := range req.Types {
			filters.Types = append(filters.Types, protoToEntityType(t))
		}
	}

	if len(req.Tags) > 0 {
		filters.Tags = req.Tags
	}
	if len(req.Projects) > 0 {
		filters.Projects = req.Projects
	}
	if req.Language != nil {
		filters.Language = req.Language
	}
	if req.Category != nil {
		filters.Category = req.Category
	}
	if req.Author != nil {
		filters.Author = req.Author
	}
	if req.Owner != nil {
		filters.Owner = req.Owner
	}
	if req.Location != nil {
		filters.Location = req.Location
	}
	if req.PublicationYear != nil {
		year := int(*req.PublicationYear)
		filters.PublicationYear = &year
	}
	if req.Status != nil {
		filters.Status = req.Status
	}
	if req.Priority != nil {
		filters.Priority = req.Priority
	}
	if req.Visibility != nil {
		filters.Visibility = req.Visibility
	}
	if req.ComplexityMin != nil {
		filters.ComplexityMin = req.ComplexityMin
	}
	if req.ComplexityMax != nil {
		filters.ComplexityMax = req.ComplexityMax
	}
	if req.SizeMin != nil {
		filters.SizeMin = req.SizeMin
	}
	if req.SizeMax != nil {
		filters.SizeMax = req.SizeMax
	}
	if req.CreatedAfter != nil {
		after := *req.CreatedAfter
		filters.CreatedAfter = &after
	}
	if req.CreatedBefore != nil {
		before := *req.CreatedBefore
		filters.CreatedBefore = &before
	}
	if req.UpdatedAfter != nil {
		after := *req.UpdatedAfter
		filters.UpdatedAfter = &after
	}
	if req.UpdatedBefore != nil {
		before := *req.UpdatedBefore
		filters.UpdatedBefore = &before
	}

	filters.Limit = int(req.Limit)
	filters.Offset = int(req.Offset)

	return filters
}

// protoToEntityMetadata converts protobuf metadata to repository metadata.
func protoToEntityMetadata(proto *cortexv1.EntityMetadata) repository.EntityMetadata {
	metadata := repository.EntityMetadata{}

	if len(proto.Tags) > 0 {
		metadata.Tags = proto.Tags
	}
	if len(proto.Projects) > 0 {
		metadata.Projects = proto.Projects
	}
	if proto.Language != nil {
		metadata.Language = proto.Language
	}
	if proto.Category != nil {
		metadata.Category = proto.Category
	}
	if proto.Author != nil {
		metadata.Author = proto.Author
	}
	if proto.Owner != nil {
		metadata.Owner = proto.Owner
	}
	if proto.Location != nil {
		metadata.Location = proto.Location
	}
	if proto.PublicationYear != nil {
		year := int(*proto.PublicationYear)
		metadata.PublicationYear = &year
	}
	if proto.Status != nil {
		metadata.Status = proto.Status
	}
	if proto.Priority != nil {
		metadata.Priority = proto.Priority
	}
	if proto.Visibility != nil {
		metadata.Visibility = proto.Visibility
	}
	if proto.AiSummary != nil {
		metadata.AISummary = proto.AiSummary
	}
	if len(proto.AiKeywords) > 0 {
		metadata.AIKeywords = proto.AiKeywords
	}
	if proto.Description != nil {
		metadata.Description = proto.Description
	}

	return metadata
}

// entityTypeToProto converts EntityType to protobuf enum.
func entityTypeToProto(t entity.EntityType) cortexv1.EntityType {
	switch t {
	case entity.EntityTypeFile:
		return cortexv1.EntityType_ENTITY_TYPE_FILE
	case entity.EntityTypeFolder:
		return cortexv1.EntityType_ENTITY_TYPE_FOLDER
	case entity.EntityTypeProject:
		return cortexv1.EntityType_ENTITY_TYPE_PROJECT
	default:
		return cortexv1.EntityType_ENTITY_TYPE_UNSPECIFIED
	}
}

// protoToEntityType converts protobuf enum to EntityType.
func protoToEntityType(t cortexv1.EntityType) entity.EntityType {
	switch t {
	case cortexv1.EntityType_ENTITY_TYPE_FILE:
		return entity.EntityTypeFile
	case cortexv1.EntityType_ENTITY_TYPE_FOLDER:
		return entity.EntityTypeFolder
	case cortexv1.EntityType_ENTITY_TYPE_PROJECT:
		return entity.EntityTypeProject
	default:
		return entity.EntityTypeFile
	}
}


