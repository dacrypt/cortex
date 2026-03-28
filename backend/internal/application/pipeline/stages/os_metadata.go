// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
)

// OSMetadataStage extracts OS-level metadata from files.
type OSMetadataStage struct {
	extractor  *metadata.OSExtractor
	classifier *metadata.OSContextClassifier
	fileRepo   repository.FileRepository
	logger     zerolog.Logger
}

// NewOSMetadataStage creates a new OS metadata extraction stage.
func NewOSMetadataStage(fileRepo repository.FileRepository, logger zerolog.Logger) *OSMetadataStage {
	return &OSMetadataStage{
		extractor:  metadata.NewOSExtractor(logger),
		classifier: metadata.NewOSContextClassifier(),
		fileRepo:   fileRepo,
		logger:     logger.With().Str("component", "os_metadata_stage").Logger(),
	}
}

// Name returns the stage name.
func (s *OSMetadataStage) Name() string {
	return "os_metadata"
}

// Process extracts OS metadata and classifies it into taxonomic dimensions.
func (s *OSMetadataStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	// Extract OS metadata
	osMeta, err := s.extractor.Extract(ctx, entry.AbsolutePath)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("file", entry.RelativePath).
			Msg("Failed to extract OS metadata")
		// Don't fail the pipeline, just log the warning
		return nil
	}

	// Classify into taxonomic dimensions
	taxonomy := s.classifier.Classify(osMeta, entry)

	// Store in enhanced metadata
	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}

	entry.Enhanced.OSMetadata = osMeta
	entry.Enhanced.OSContextTaxonomy = taxonomy

	// Get workspace ID from context
	wsInfo, ok := contextinfo.GetWorkspaceInfo(ctx)
	if !ok {
		s.logger.Warn().
			Str("file", entry.RelativePath).
			Msg("Workspace info not found in context, skipping ownership persistence")
	} else if s.fileRepo != nil {
		// Save ownership data to database
		if err := s.saveOwnership(ctx, wsInfo.ID, entry, osMeta); err != nil {
			s.logger.Warn().
				Err(err).
				Str("file", entry.RelativePath).
				Msg("Failed to save ownership data")
			// Don't fail the pipeline, just log the warning
		}
	}

	ownerName := getOwnerName(osMeta)
	groupName := getGroupName(osMeta)
	
	s.logger.Debug().
		Str("file", entry.RelativePath).
		Str("owner", ownerName).
		Str("group", groupName).
		Bool("has_owner", osMeta != nil && osMeta.Owner != nil).
		Bool("has_workspace", ok).
		Bool("has_file_repo", s.fileRepo != nil).
		Msg("Extracted OS metadata")

	return nil
}

// saveOwnership saves ownership data to the database.
func (s *OSMetadataStage) saveOwnership(ctx context.Context, workspaceID entity.WorkspaceID, entry *entity.FileEntry, osMeta *entity.OSMetadata) error {
	if osMeta == nil {
		return nil
	}

	now := time.Now()

	// Save owner if present
	if osMeta.Owner != nil {
		systemUser := &entity.SystemUser{
			ID:          fmt.Sprintf("%s:%s:%d", workspaceID.String(), osMeta.Owner.Username, osMeta.Owner.UID),
			WorkspaceID: workspaceID.String(),
			Username:    osMeta.Owner.Username,
			UID:         osMeta.Owner.UID,
			FullName:    osMeta.Owner.FullName,
			HomeDir:     osMeta.Owner.HomeDir,
			Shell:       osMeta.Owner.Shell,
			System:      osMeta.Owner.UID < 1000, // System users typically have UID < 1000
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.fileRepo.UpsertSystemUser(ctx, workspaceID, systemUser); err != nil {
			return fmt.Errorf("failed to upsert system user: %w", err)
		}

		// Save file ownership relationship
		permissions := ""
		if osMeta.Permissions != nil {
			permissions = osMeta.Permissions.String
		}

		ownership := &entity.FileOwnership{
			FileID:        entry.ID.String(),
			WorkspaceID:   workspaceID.String(),
			UserID:        systemUser.ID,
			OwnershipType: "owner",
			Permissions:   permissions,
			DetectedAt:    now,
		}

		if err := s.fileRepo.UpsertFileOwnership(ctx, workspaceID, ownership); err != nil {
			return fmt.Errorf("failed to upsert file ownership: %w", err)
		}

		s.logger.Debug().
			Str("file", entry.RelativePath).
			Str("username", osMeta.Owner.Username).
			Int("uid", osMeta.Owner.UID).
			Str("user_id", systemUser.ID).
			Msg("Saved ownership data to database")
	} else {
		s.logger.Debug().
			Str("file", entry.RelativePath).
			Msg("No owner information found in OS metadata")
	}

	return nil
}

// getOwnerName returns the owner name for logging.
func getOwnerName(meta *entity.OSMetadata) string {
	if meta == nil || meta.Owner == nil {
		return "unknown"
	}
	return meta.Owner.Username
}

// getGroupName returns the group name for logging.
func getGroupName(meta *entity.OSMetadata) string {
	if meta == nil || meta.Group == nil {
		return "unknown"
	}
	return meta.Group.GroupName
}





