package query

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// FacetRequest represents a request for faceting.
type FacetRequest struct {
	Field string // Field to facet on (e.g., "extension", "tag", "file_size", "last_modified")
	Type  FacetType
}

// FacetType represents the type of facet.
type FacetType string

const (
	FacetTypeTerms        FacetType = "terms"         // Count distinct values
	FacetTypeNumericRange FacetType = "numeric_range" // Count within numeric ranges
	FacetTypeDateRange    FacetType = "date_range"    // Count within date ranges
)

// FacetResult contains the results of a faceting operation.
type FacetResult struct {
	Field string
	Type  FacetType
	Data  interface{} // TermsFacetData, NumericRangeFacetData, or DateRangeFacetData
}

// TermsFacetData contains term facet results.
type TermsFacetData struct {
	Terms []TermCount
}

// TermCount represents a term and its count.
type TermCount struct {
	Term  string
	Count int
}

// NumericRangeFacetData contains numeric range facet results.
type NumericRangeFacetData struct {
	Ranges []NumericRangeCount
}

// NumericRangeCount represents a numeric range and its count.
type NumericRangeCount = repository.NumericRangeCount

// DateRangeFacetData contains date range facet results.
type DateRangeFacetData struct {
	Ranges []DateRangeCount
}

// DateRangeCount represents a date range and its count.
type DateRangeCount = repository.DateRangeCount

// FacetExecutor executes facet requests.
type FacetExecutor struct {
	fileRepo        repository.FileRepository
	metaRepo        repository.MetadataRepository
	folderRepo      repository.FolderRepository
	projectRepo     repository.ProjectRepository
	entityRepo      repository.EntityRepository
	clusterRepo     repository.ClusterRepository
	registry        *FacetRegistry
	termsHandlers   map[string]termsFacetHandler
	numericHandlers map[string]numericRangeFacetHandler
	dateHandlers    map[string]dateRangeFacetHandler
}

// termsFacetHandler is a function that returns term counts for a facet.
type termsFacetHandler func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)

// numericRangeFacetHandler is a function that returns numeric range counts for a facet.
type numericRangeFacetHandler func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.NumericRangeCount, error)

// dateRangeFacetHandler is a function that returns date range counts for a facet.
type dateRangeFacetHandler func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) ([]repository.DateRangeCount, error)

// FileID is a type alias for entity.FileID to avoid circular dependencies
type FileID = entity.FileID

// NewFacetExecutor creates a new facet executor.
func NewFacetExecutor(
	fileRepo repository.FileRepository,
	metaRepo repository.MetadataRepository,
) *FacetExecutor {
	e := &FacetExecutor{
		fileRepo:        fileRepo,
		metaRepo:        metaRepo,
		registry:        NewFacetRegistry(),
		termsHandlers:   make(map[string]termsFacetHandler),
		numericHandlers: make(map[string]numericRangeFacetHandler),
		dateHandlers:    make(map[string]dateRangeFacetHandler),
	}
	e.initHandlers()
	return e
}

// NewFacetExecutorWithEntities creates a new facet executor with entity support.
func NewFacetExecutorWithEntities(
	fileRepo repository.FileRepository,
	metaRepo repository.MetadataRepository,
	folderRepo repository.FolderRepository,
	projectRepo repository.ProjectRepository,
	entityRepo repository.EntityRepository,
) *FacetExecutor {
	e := &FacetExecutor{
		fileRepo:        fileRepo,
		metaRepo:        metaRepo,
		folderRepo:      folderRepo,
		projectRepo:     projectRepo,
		entityRepo:      entityRepo,
		registry:        NewFacetRegistry(),
		termsHandlers:   make(map[string]termsFacetHandler),
		numericHandlers: make(map[string]numericRangeFacetHandler),
		dateHandlers:    make(map[string]dateRangeFacetHandler),
	}
	e.initHandlers()
	return e
}

// SetClusterRepository sets the cluster repository for cluster faceting.
// This allows cluster faceting to be enabled after executor creation.
func (e *FacetExecutor) SetClusterRepository(clusterRepo repository.ClusterRepository) {
	e.clusterRepo = clusterRepo
	// Register the cluster handler now that repo is available
	if clusterRepo != nil {
		e.termsHandlers["cluster"] = clusterRepo.GetClusterFacet
	}
}

// initHandlers initializes all facet handlers.
func (e *FacetExecutor) initHandlers() {
	// Terms facet handlers
	e.termsHandlers["extension"] = e.fileRepo.GetExtensionFacet
	e.termsHandlers["type"] = e.fileRepo.GetTypeFacet
	e.termsHandlers["tag"] = func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
		return e.metaRepo.GetTagCounts(ctx, workspaceID)
	}
	e.termsHandlers["project"] = func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error) {
		return e.metaRepo.GetContextCounts(ctx, workspaceID)
	}
	e.termsHandlers["language"] = e.metaRepo.GetLanguageFacet
	e.termsHandlers["category"] = e.metaRepo.GetAICategoryFacet
	e.termsHandlers["author"] = e.metaRepo.GetAuthorFacet
	e.termsHandlers["publication_year"] = e.metaRepo.GetPublicationYearFacet
	e.termsHandlers["owner"] = e.fileRepo.GetOwnerFacet
	e.termsHandlers["indexing_status"] = e.fileRepo.GetIndexingStatusFacet
	e.termsHandlers["indexing_error"] = e.fileRepo.GetIndexingErrorFacet
	e.termsHandlers["index_error"] = e.fileRepo.GetIndexingErrorFacet
	e.termsHandlers["indexing_errors"] = e.fileRepo.GetIndexingErrorFacet
	e.termsHandlers["sentiment"] = e.metaRepo.GetSentimentFacet
	e.termsHandlers["duplicate_type"] = e.metaRepo.GetDuplicateTypeFacet
	e.termsHandlers["location"] = e.metaRepo.GetLocationFacet
	e.termsHandlers["organization"] = e.metaRepo.GetOrganizationFacet
	e.termsHandlers["document_type"] = e.metaRepo.GetContentTypeFacet // Uses suggested_taxonomy.content_type
	e.termsHandlers["folder"] = e.metaRepo.GetFolderNameFacet // Groups files by folder name
	e.termsHandlers["mime_type"] = e.fileRepo.GetMimeTypeFacet
	e.termsHandlers["mime_category"] = e.fileRepo.GetMimeCategoryFacet
	e.termsHandlers["purpose"] = e.metaRepo.GetPurposeFacet
	e.termsHandlers["audience"] = e.metaRepo.GetAudienceFacet
	e.termsHandlers["domain"] = e.metaRepo.GetDomainFacet
	e.termsHandlers["subdomain"] = e.metaRepo.GetSubdomainFacet
	e.termsHandlers["topic"] = e.metaRepo.GetTopicFacet
	e.termsHandlers["event"] = e.metaRepo.GetEventFacet
	e.termsHandlers["citation_type"] = e.metaRepo.GetCitationTypeFacet
	e.termsHandlers["relationship_type"] = e.metaRepo.GetRelationshipTypeFacet
	e.termsHandlers["temporal_pattern"] = e.fileRepo.GetTemporalPatternFacet
	e.termsHandlers["readability_level"] = e.fileRepo.GetReadabilityLevelFacet
	e.termsHandlers["video_resolution"] = e.fileRepo.GetVideoResolutionFacet
	e.termsHandlers["permission_level"] = e.fileRepo.GetPermissionLevelFacet
	e.termsHandlers["image_format"] = e.fileRepo.GetImageFormatFacet
	e.termsHandlers["image_color_space"] = e.fileRepo.GetImageColorSpaceFacet
	e.termsHandlers["camera_make"] = e.fileRepo.GetCameraMakeFacet
	e.termsHandlers["camera_model"] = e.fileRepo.GetCameraModelFacet
	e.termsHandlers["image_gps_location"] = e.fileRepo.GetImageGPSLocationFacet
	e.termsHandlers["image_orientation"] = e.fileRepo.GetImageOrientationFacet
	e.termsHandlers["image_has_transparency"] = e.fileRepo.GetImageTransparencyFacet
	e.termsHandlers["image_is_animated"] = e.fileRepo.GetImageAnimatedFacet
	e.termsHandlers["audio_codec"] = e.fileRepo.GetAudioCodecFacet
	e.termsHandlers["audio_format"] = e.fileRepo.GetAudioFormatFacet
	e.termsHandlers["audio_genre"] = e.fileRepo.GetAudioGenreFacet
	e.termsHandlers["audio_artist"] = e.fileRepo.GetAudioArtistFacet
	e.termsHandlers["audio_album"] = e.fileRepo.GetAudioAlbumFacet
	e.termsHandlers["audio_year"] = e.fileRepo.GetAudioYearFacet
	e.termsHandlers["audio_channels"] = e.fileRepo.GetAudioChannelsFacet
	e.termsHandlers["audio_has_album_art"] = e.fileRepo.GetAudioHasAlbumArtFacet
	e.termsHandlers["video_codec"] = e.fileRepo.GetVideoCodecFacet
	e.termsHandlers["video_audio_codec"] = e.fileRepo.GetVideoAudioCodecFacet
	e.termsHandlers["video_container"] = e.fileRepo.GetVideoContainerFacet
	e.termsHandlers["video_aspect_ratio"] = e.fileRepo.GetVideoAspectRatioFacet
	e.termsHandlers["video_has_subtitles"] = e.fileRepo.GetVideoHasSubtitlesFacet
	e.termsHandlers["video_subtitle_languages"] = e.fileRepo.GetVideoSubtitleLanguageFacet
	e.termsHandlers["video_has_chapters"] = e.fileRepo.GetVideoHasChaptersFacet
	e.termsHandlers["video_is_3d"] = e.fileRepo.GetVideoIs3DFacet
	e.termsHandlers["video_quality_tier"] = e.fileRepo.GetVideoQualityTierFacet
	e.termsHandlers["content_encoding"] = e.fileRepo.GetContentEncodingFacet
	e.termsHandlers["filesystem_type"] = e.fileRepo.GetFilesystemTypeFacet
	e.termsHandlers["mount_point"] = e.fileRepo.GetMountPointFacet
	e.termsHandlers["security_category"] = e.fileRepo.GetSecurityCategoryFacet
	e.termsHandlers["security_attributes"] = e.fileRepo.GetSecurityAttributesFacet
	e.termsHandlers["has_acls"] = e.fileRepo.GetHasACLsFacet
	e.termsHandlers["acl_complexity"] = e.fileRepo.GetACLComplexityFacet
	e.termsHandlers["owner_type"] = e.fileRepo.GetOwnerTypeFacet
	e.termsHandlers["group_category"] = e.fileRepo.GetGroupCategoryFacet
	e.termsHandlers["access_relation"] = e.fileRepo.GetAccessRelationFacet
	e.termsHandlers["ownership_pattern"] = e.fileRepo.GetOwnershipPatternFacet
	e.termsHandlers["access_frequency"] = e.fileRepo.GetAccessFrequencyFacet
	e.termsHandlers["time_category"] = e.fileRepo.GetTimeCategoryFacet
	e.termsHandlers["system_file_type"] = e.fileRepo.GetSystemFileTypeFacet
	e.termsHandlers["file_system_category"] = e.fileRepo.GetFileSystemCategoryFacet
	e.termsHandlers["system_attributes"] = e.fileRepo.GetSystemAttributesFacet
	e.termsHandlers["system_features"] = e.fileRepo.GetSystemFeaturesFacet

	// Numeric range facet handlers
	e.numericHandlers["size"] = e.fileRepo.GetSizeRangeFacet
	e.numericHandlers["complexity"] = e.fileRepo.GetComplexityRangeFacet
	e.numericHandlers["project_score"] = e.fileRepo.GetProjectScoreRangeFacet
	e.numericHandlers["function_count"] = e.fileRepo.GetFunctionCountRangeFacet
	e.numericHandlers["lines_of_code"] = e.fileRepo.GetLinesOfCodeRangeFacet
	e.numericHandlers["comment_percentage"] = e.fileRepo.GetCommentPercentageRangeFacet
	e.numericHandlers["content_quality"] = e.fileRepo.GetContentQualityRangeFacet
	e.numericHandlers["image_dimensions"] = e.fileRepo.GetImageDimensionsRangeFacet
	e.numericHandlers["image_color_depth"] = e.fileRepo.GetImageColorDepthRangeFacet
	e.numericHandlers["image_iso"] = e.fileRepo.GetImageISORangeFacet
	e.numericHandlers["image_aperture"] = e.fileRepo.GetImageApertureRangeFacet
	e.numericHandlers["image_focal_length"] = e.fileRepo.GetImageFocalLengthRangeFacet
	e.numericHandlers["audio_duration"] = e.fileRepo.GetAudioDurationRangeFacet
	e.numericHandlers["audio_bitrate"] = e.fileRepo.GetAudioBitrateRangeFacet
	e.numericHandlers["audio_sample_rate"] = e.fileRepo.GetAudioSampleRateRangeFacet
	e.numericHandlers["video_duration"] = e.fileRepo.GetVideoDurationRangeFacet
	e.numericHandlers["video_bitrate"] = e.fileRepo.GetVideoBitrateRangeFacet
	e.numericHandlers["video_frame_rate"] = e.fileRepo.GetVideoFrameRateRangeFacet
	e.numericHandlers["language_confidence"] = e.fileRepo.GetLanguageConfidenceRangeFacet

	// Date range facet handlers
	e.dateHandlers["modified"] = e.fileRepo.GetDateRangeFacet
	e.dateHandlers["created"] = e.fileRepo.GetCreatedDateRangeFacet
	e.dateHandlers["accessed"] = e.fileRepo.GetAccessedDateRangeFacet
	e.dateHandlers["changed"] = e.fileRepo.GetChangedDateRangeFacet
}

// ExecuteFacet executes a facet request and returns results.
func (e *FacetExecutor) ExecuteFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	req FacetRequest,
	fileIDs []entity.FileID, // Optional: if nil/empty, facet on all files
) (*FacetResult, error) {
	// Resolve field name to canonical name using registry
	def, err := e.registry.Resolve(req.Field)
	if err != nil {
		return nil, err
	}

	// Use canonical name and type from definition
	req.Field = def.CanonicalName
	if req.Type == "" {
		req.Type = def.Type
	}

	// Execute based on facet type
	switch req.Type {
	case FacetTypeTerms:
		return e.executeTermsFacet(ctx, workspaceID, req.Field, fileIDs)
	case FacetTypeNumericRange:
		return e.executeNumericRangeFacet(ctx, workspaceID, req.Field, fileIDs)
	case FacetTypeDateRange:
		return e.executeDateRangeFacet(ctx, workspaceID, req.Field, fileIDs)
	default:
		return nil, fmt.Errorf("unknown facet type: %s", req.Type)
	}
}

// GetRegistry returns the facet registry for querying available facets.
func (e *FacetExecutor) GetRegistry() *FacetRegistry {
	return e.registry
}

// ListAvailableFacets returns all available facets, optionally filtered by category or type.
func (e *FacetExecutor) ListAvailableFacets(category *FacetCategory, facetType *FacetType) []*FacetDefinition {
	if category != nil {
		return e.registry.GetByCategory(*category)
	}
	if facetType != nil {
		return e.registry.GetByType(*facetType)
	}
	return e.registry.GetAll()
}

// executeTermsFacet executes a terms facet.
func (e *FacetExecutor) executeTermsFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	fileIDs []entity.FileID, // If empty, facet on all files
) (*FacetResult, error) {
	handler, ok := e.termsHandlers[field]
	if !ok {
		return nil, fmt.Errorf("unsupported field for terms facet: %s", field)
	}

	counts, err := handler(ctx, workspaceID, fileIDs)
	if err != nil {
		return nil, err
	}

	return e.buildTermsFacetResult(field, counts), nil
}

// buildTermsFacetResult converts a map of term counts to a FacetResult.
func (e *FacetExecutor) buildTermsFacetResult(field string, counts map[string]int) *FacetResult {
	terms := make([]TermCount, 0, len(counts))
	for term, count := range counts {
		terms = append(terms, TermCount{Term: term, Count: count})
	}
	return &FacetResult{
		Field: field,
		Type:  FacetTypeTerms,
		Data:  TermsFacetData{Terms: terms},
	}
}

// executeNumericRangeFacet executes a numeric range facet.
func (e *FacetExecutor) executeNumericRangeFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	fileIDs []entity.FileID,
) (*FacetResult, error) {
	handler, ok := e.numericHandlers[field]
	if !ok {
		return nil, fmt.Errorf("unsupported field for numeric range facet: %s", field)
	}

	ranges, err := handler(ctx, workspaceID, fileIDs)
	if err != nil {
		return nil, err
	}

	return &FacetResult{
		Field: field,
		Type:  FacetTypeNumericRange,
		Data:  NumericRangeFacetData{Ranges: ranges},
	}, nil
}

// executeDateRangeFacet executes a date range facet.
func (e *FacetExecutor) executeDateRangeFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	fileIDs []entity.FileID,
) (*FacetResult, error) {
	handler, ok := e.dateHandlers[field]
	if !ok {
		return nil, fmt.Errorf("unsupported field for date range facet: %s", field)
	}

	ranges, err := handler(ctx, workspaceID, fileIDs)
	if err != nil {
		return nil, err
	}

	return &FacetResult{
		Field: field,
		Type:  FacetTypeDateRange,
		Data:  DateRangeFacetData{Ranges: ranges},
	}, nil
}

// ExecuteEntityFacet executes a facet request for entities (files, folders, projects).
// It aggregates results from all entity types specified.
func (e *FacetExecutor) ExecuteEntityFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	req FacetRequest,
	entityTypes []entity.EntityType, // Which entity types to include (file, folder, project)
) (*FacetResult, error) {
	if e.entityRepo == nil {
		// Fallback to file-only if entity repo not available
		return e.ExecuteFacet(ctx, workspaceID, req, nil)
	}

	// Resolve field name to canonical name using registry
	def, err := e.registry.Resolve(req.Field)
	if err != nil {
		return nil, err
	}

	// Use canonical name and type from definition
	req.Field = def.CanonicalName
	if req.Type == "" {
		req.Type = def.Type
	}

	// Execute based on facet type
	switch req.Type {
	case FacetTypeTerms:
		return e.executeEntityTermsFacet(ctx, workspaceID, req.Field, entityTypes)
	case FacetTypeNumericRange:
		return e.executeEntityNumericRangeFacet(ctx, workspaceID, req.Field, entityTypes)
	case FacetTypeDateRange:
		return e.executeEntityDateRangeFacet(ctx, workspaceID, req.Field, entityTypes)
	default:
		return nil, fmt.Errorf("unknown facet type: %s", req.Type)
	}
}

// executeEntityTermsFacet executes a terms facet for entities.
func (e *FacetExecutor) executeEntityTermsFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	entityTypes []entity.EntityType,
) (*FacetResult, error) {
	// Aggregate counts from all entity types
	allCounts := make(map[string]int)

	// Get counts for each entity type
	for _, entityType := range entityTypes {
		entities, err := e.entityRepo.GetEntitiesByFacet(ctx, workspaceID, field, "", []entity.EntityType{entityType})
		if err != nil {
			// Continue with other types if one fails
			continue
		}

		// Count by value
		for _, ent := range entities {
			value := e.extractFieldValue(ent, field)
			if value != "" {
				allCounts[value]++
			}
		}
	}

	return e.buildTermsFacetResult(field, allCounts), nil
}

// executeEntityNumericRangeFacet executes a numeric range facet for entities.
func (e *FacetExecutor) executeEntityNumericRangeFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	entityTypes []entity.EntityType,
) (*FacetResult, error) {
	// For now, delegate to file handler if field is file-specific
	// TODO: Implement proper entity-based numeric range faceting
	if e.numericHandlers[field] != nil {
		return e.executeNumericRangeFacet(ctx, workspaceID, field, nil)
	}
	return nil, fmt.Errorf("numeric range facet not yet implemented for entities: %s", field)
}

// executeEntityDateRangeFacet executes a date range facet for entities.
func (e *FacetExecutor) executeEntityDateRangeFacet(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	field string,
	entityTypes []entity.EntityType,
) (*FacetResult, error) {
	// For now, delegate to file handler if field is file-specific
	// TODO: Implement proper entity-based date range faceting
	if e.dateHandlers[field] != nil {
		return e.executeDateRangeFacet(ctx, workspaceID, field, nil)
	}
	return nil, fmt.Errorf("date range facet not yet implemented for entities: %s", field)
}

// extractFieldValue extracts the value of a field from an entity.
func (e *FacetExecutor) extractFieldValue(ent *entity.Entity, field string) string {
	switch field {
	case "tag":
		if len(ent.Tags) > 0 {
			return ent.Tags[0] // Return first tag for counting
		}
	case "project":
		if len(ent.Projects) > 0 {
			return ent.Projects[0]
		}
	case "language":
		if ent.Language != nil {
			return *ent.Language
		}
	case "category":
		if ent.Category != nil {
			return *ent.Category
		}
	case "author":
		if ent.Author != nil {
			return *ent.Author
		}
	case "owner":
		if ent.Owner != nil {
			return *ent.Owner
		}
	case "location":
		if ent.Location != nil {
			return *ent.Location
		}
	case "status":
		if ent.Status != nil {
			return *ent.Status
		}
	case "priority":
		if ent.Priority != nil {
			return *ent.Priority
		}
	case "visibility":
		if ent.Visibility != nil {
			return *ent.Visibility
		}
	case "extension":
		if ent.Type == entity.EntityTypeFile && ent.FileData != nil {
			return ent.FileData.Extension
		}
	case "type":
		if ent.Type == entity.EntityTypeFile && ent.FileData != nil && ent.FileData.ContentType != nil {
			return *ent.FileData.ContentType
		}
	}
	return ""
}
