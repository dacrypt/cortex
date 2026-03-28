// Package repository defines repository interfaces for domain entities.
package repository

import (
	"context"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// EntityRepository provides unified access to files, folders, and projects
type EntityRepository interface {
	// Get entity by ID
	GetEntity(ctx context.Context, workspaceID entity.WorkspaceID, id entity.EntityID) (*entity.Entity, error)

	// List entities with filters
	ListEntities(ctx context.Context, workspaceID entity.WorkspaceID, filters EntityFilters) ([]*entity.Entity, error)

	// Get entities by facet value
	GetEntitiesByFacet(ctx context.Context, workspaceID entity.WorkspaceID, facet string, value string, entityTypes []entity.EntityType) ([]*entity.Entity, error)

	// Update entity metadata
	UpdateEntityMetadata(ctx context.Context, workspaceID entity.WorkspaceID, id entity.EntityID, metadata EntityMetadata) error

	// Count entities by facet value
	CountEntitiesByFacet(ctx context.Context, workspaceID entity.WorkspaceID, facet string, value string, entityTypes []entity.EntityType) (int, error)
}

// EntityFilters for querying entities
type EntityFilters struct {
	Types           []entity.EntityType // file, folder, project
	Tags            []string
	Projects        []string
	Language        *string
	Category        *string
	Author          *string
	Owner           *string
	Location        *string
	PublicationYear *int
	Status          *string
	Priority        *string
	Visibility      *string
	ComplexityMin   *float64
	ComplexityMax   *float64
	SizeMin         *int64
	SizeMax         *int64
	CreatedAfter    *int64 // Unix timestamp
	CreatedBefore   *int64
	UpdatedAfter    *int64
	UpdatedBefore   *int64
	Limit           int
	Offset          int
}

// EntityMetadata contains semantic metadata that can be updated
type EntityMetadata struct {
	Tags            []string
	Projects        []string
	Language        *string
	Category        *string
	Author          *string
	Owner           *string
	Location        *string
	PublicationYear *int
	Status          *string
	Priority        *string
	Visibility      *string
	AISummary       *string
	AIKeywords      []string
	Description     *string
}


