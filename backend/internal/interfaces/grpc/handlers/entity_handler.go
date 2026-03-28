package handlers

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// EntityHandler handles entity-related gRPC requests.
type EntityHandler struct {
	entityRepo repository.EntityRepository
	logger     zerolog.Logger
}

// EntityHandlerConfig holds configuration for the entity handler.
type EntityHandlerConfig struct {
	EntityRepo repository.EntityRepository
	Logger     zerolog.Logger
}

// NewEntityHandler creates a new entity handler.
func NewEntityHandler(cfg EntityHandlerConfig) *EntityHandler {
	return &EntityHandler{
		entityRepo: cfg.EntityRepo,
		logger:     cfg.Logger.With().Str("handler", "entity").Logger(),
	}
}

// GetEntity retrieves a single entity by ID.
func (h *EntityHandler) GetEntity(ctx context.Context, workspaceID entity.WorkspaceID, id entity.EntityID) (*entity.Entity, error) {
	return h.entityRepo.GetEntity(ctx, workspaceID, id)
}

// ListEntities lists entities with filters.
func (h *EntityHandler) ListEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters repository.EntityFilters) ([]*entity.Entity, error) {
	return h.entityRepo.ListEntities(ctx, workspaceID, filters)
}

// GetEntitiesByFacet retrieves entities matching a facet value.
func (h *EntityHandler) GetEntitiesByFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	facet string,
	value string,
	entityTypes []entity.EntityType,
) ([]*entity.Entity, error) {
	return h.entityRepo.GetEntitiesByFacet(ctx, workspaceID, facet, value, entityTypes)
}

// UpdateEntityMetadata updates semantic metadata for an entity.
func (h *EntityHandler) UpdateEntityMetadata(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	id entity.EntityID,
	metadata repository.EntityMetadata,
) error {
	return h.entityRepo.UpdateEntityMetadata(ctx, workspaceID, id, metadata)
}

// CountEntitiesByFacet counts entities matching a facet value.
func (h *EntityHandler) CountEntitiesByFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	facet string,
	value string,
	entityTypes []entity.EntityType,
) (int, error) {
	return h.entityRepo.CountEntitiesByFacet(ctx, workspaceID, facet, value, entityTypes)
}

// EntityToProto converts an entity to protobuf format.
func EntityToProto(ent *entity.Entity) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"id": map[string]interface{}{
			"type": string(ent.Type),
			"id":   ent.ID.ID,
		},
		"type":         string(ent.Type),
		"workspace_id": ent.WorkspaceID.String(),
		"name":         ent.Name,
		"path":         ent.Path,
		"created_at":   ent.CreatedAt.UnixMilli(),
		"updated_at":   ent.UpdatedAt.UnixMilli(),
	}

	if ent.Description != nil {
		result["description"] = *ent.Description
	}
	if ent.ModifiedAt != nil {
		result["modified_at"] = ent.ModifiedAt.UnixMilli()
	}
	if ent.Size != nil {
		result["size"] = *ent.Size
	}

	// Semantic metadata
	if len(ent.Tags) > 0 {
		result["tags"] = ent.Tags
	}
	if len(ent.Projects) > 0 {
		result["projects"] = ent.Projects
	}
	if ent.Language != nil {
		result["language"] = *ent.Language
	}
	if ent.Category != nil {
		result["category"] = *ent.Category
	}
	if ent.Author != nil {
		result["author"] = *ent.Author
	}
	if ent.Owner != nil {
		result["owner"] = *ent.Owner
	}
	if ent.Location != nil {
		result["location"] = *ent.Location
	}
	if ent.PublicationYear != nil {
		result["publication_year"] = *ent.PublicationYear
	}
	if ent.Complexity != nil {
		result["complexity"] = *ent.Complexity
	}
	if ent.LinesOfCode != nil {
		result["lines_of_code"] = *ent.LinesOfCode
	}
	if ent.QualityScore != nil {
		result["quality_score"] = *ent.QualityScore
	}
	if ent.Status != nil {
		result["status"] = *ent.Status
	}
	if ent.Priority != nil {
		result["priority"] = *ent.Priority
	}
	if ent.Visibility != nil {
		result["visibility"] = *ent.Visibility
	}
	if ent.AISummary != nil {
		result["ai_summary"] = *ent.AISummary
	}
	if len(ent.AIKeywords) > 0 {
		result["ai_keywords"] = ent.AIKeywords
	}

	// Type-specific data
	if ent.FileData != nil {
		fileDataJSON, err := json.Marshal(ent.FileData)
		if err == nil {
			result["file_data"] = string(fileDataJSON)
		}
	}
	if ent.FolderData != nil {
		folderDataJSON, err := json.Marshal(ent.FolderData)
		if err == nil {
			result["folder_data"] = string(folderDataJSON)
		}
	}
	if ent.ProjectData != nil {
		projectDataJSON, err := json.Marshal(ent.ProjectData)
		if err == nil {
			result["project_data"] = string(projectDataJSON)
		}
	}

	return result, nil
}

// ProtoToEntityFilters converts protobuf filters to repository filters.
func ProtoToEntityFilters(protoFilters map[string]interface{}) repository.EntityFilters {
	filters := repository.EntityFilters{}

	if types, ok := protoFilters["types"].([]interface{}); ok {
		entityTypes := make([]entity.EntityType, 0, len(types))
		for _, t := range types {
			if typeStr, ok := t.(string); ok {
				entityTypes = append(entityTypes, entity.EntityType(typeStr))
			}
		}
		filters.Types = entityTypes
	}

	if tags, ok := protoFilters["tags"].([]interface{}); ok {
		filters.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				filters.Tags = append(filters.Tags, tagStr)
			}
		}
	}

	if projects, ok := protoFilters["projects"].([]interface{}); ok {
		filters.Projects = make([]string, 0, len(projects))
		for _, project := range projects {
			if projectStr, ok := project.(string); ok {
				filters.Projects = append(filters.Projects, projectStr)
			}
		}
	}

	if lang, ok := protoFilters["language"].(string); ok {
		filters.Language = &lang
	}
	if cat, ok := protoFilters["category"].(string); ok {
		filters.Category = &cat
	}
	if author, ok := protoFilters["author"].(string); ok {
		filters.Author = &author
	}
	if owner, ok := protoFilters["owner"].(string); ok {
		filters.Owner = &owner
	}
	if location, ok := protoFilters["location"].(string); ok {
		filters.Location = &location
	}
	if year, ok := protoFilters["publication_year"].(int32); ok {
		yearInt := int(year)
		filters.PublicationYear = &yearInt
	}
	if status, ok := protoFilters["status"].(string); ok {
		filters.Status = &status
	}
	if priority, ok := protoFilters["priority"].(string); ok {
		filters.Priority = &priority
	}
	if visibility, ok := protoFilters["visibility"].(string); ok {
		filters.Visibility = &visibility
	}

	if limit, ok := protoFilters["limit"].(int32); ok {
		filters.Limit = int(limit)
	}
	if offset, ok := protoFilters["offset"].(int32); ok {
		filters.Offset = int(offset)
	}

	return filters
}


