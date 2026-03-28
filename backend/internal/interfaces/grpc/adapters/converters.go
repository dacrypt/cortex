package adapters

import (
	"strings"
	"time"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// workspaceToProto converts an entity.Workspace to a proto Workspace.
func workspaceToProto(ws *entity.Workspace) *cortexv1.Workspace {
	if ws == nil {
		return nil
	}

	var lastIndexed int64
	if ws.LastIndexed != nil {
		lastIndexed = ws.LastIndexed.Unix()
	}

	return &cortexv1.Workspace{
		Id:          ws.ID.String(),
		Path:        ws.Path,
		Name:        ws.Name,
		Active:      ws.Active,
		LastIndexed: lastIndexed,
		FileCount:   int32(ws.FileCount),
		Config:      workspaceConfigToProto(&ws.Config),
	}
}

// workspaceConfigToProto converts an entity.WorkspaceConfig to a proto WorkspaceConfig.
func workspaceConfigToProto(cfg *entity.WorkspaceConfig) *cortexv1.WorkspaceConfig {
	if cfg == nil {
		return nil
	}

	return &cortexv1.WorkspaceConfig{
		ExcludedPaths:      cfg.ExcludedPaths,
		ExcludedExtensions: cfg.ExcludedExtensions,
		AutoIndex:          cfg.AutoIndex,
		LlmEnabled:         cfg.LLMEnabled,
		CustomSettings:     cfg.CustomSettings,
	}
}

// fileEntryToProto converts an entity.FileEntry to a proto FileEntry.
func fileEntryToProto(file *entity.FileEntry) *cortexv1.FileEntry {
	if file == nil {
		return nil
	}

	return &cortexv1.FileEntry{
		FileId:       file.ID.String(),
		AbsolutePath: file.AbsolutePath,
		RelativePath: file.RelativePath,
		Filename:     file.Filename,
		Extension:    file.Extension,
		FileSize:     file.FileSize,
		LastModified: file.LastModified.Unix(),
		CreatedAt:    file.CreatedAt.Unix(),
		Enhanced:     enhancedMetadataToProto(file.Enhanced),
	}
}

func enhancedMetadataToProto(meta *entity.EnhancedMetadata) *cortexv1.EnhancedMetadata {
	if meta == nil {
		return nil
	}

	return &cortexv1.EnhancedMetadata{
		Stats:           fileStatsToProto(meta.Stats),
		Folder:          meta.Folder,
		Depth:           int32(meta.Depth),
		Language:        meta.Language,
		LanguageConfidence: meta.LanguageConfidence,
		MimeType:        mimeTypeToProto(meta.MimeType),
		CodeMetrics:     codeMetricsToProto(meta.CodeMetrics),
		DocumentMetrics: documentMetricsToProto(meta.DocumentMetrics),
		ImageMetadata:   imageMetadataToProto(meta.ImageMetadata),
		AudioMetadata:   audioMetadataToProto(meta.AudioMetadata),
		VideoMetadata:   videoMetadataToProto(meta.VideoMetadata),
		OsMetadata:      osMetadataToProto(meta.OSMetadata),
		OsTaxonomy:      osContextTaxonomyToProto(meta.OSContextTaxonomy),
		ContentQuality:  contentQualityToProto(meta.ContentQuality),
		Indexed:         indexingStateToProto(meta.IndexedState),
		IndexingErrors:  indexingErrorsToProto(meta.IndexingErrors),
	}
}

func fileStatsToProto(stats *entity.FileStats) *cortexv1.FileStats {
	if stats == nil {
		return nil
	}
	return &cortexv1.FileStats{
		Size:       stats.Size,
		Created:    stats.Created.Unix(),
		Modified:   stats.Modified.Unix(),
		Accessed:   stats.Accessed.Unix(),
		IsReadOnly: stats.IsReadOnly,
		IsHidden:   stats.IsHidden,
	}
}

func mimeTypeToProto(info *entity.MimeTypeInfo) *cortexv1.MimeTypeInfo {
	if info == nil {
		return nil
	}
	return &cortexv1.MimeTypeInfo{
		MimeType: info.MimeType,
		Category: info.Category,
		Encoding: info.Encoding,
	}
}

func codeMetricsToProto(metrics *entity.CodeMetrics) *cortexv1.CodeMetrics {
	if metrics == nil {
		return nil
	}
	return &cortexv1.CodeMetrics{
		LinesOfCode:       int32(metrics.LinesOfCode),
		CommentLines:      int32(metrics.CommentLines),
		BlankLines:        int32(metrics.BlankLines),
		CommentPercentage: metrics.CommentPercentage,
		FunctionCount:     int32(metrics.FunctionCount),
		ClassCount:        int32(metrics.ClassCount),
		Complexity:        metrics.Complexity,
	}
}

func documentMetricsToProto(metrics *entity.DocumentMetrics) *cortexv1.DocumentMetrics {
	if metrics == nil {
		return nil
	}
	return &cortexv1.DocumentMetrics{
		PageCount:      int32(metrics.PageCount),
		WordCount:      int32(metrics.WordCount),
		CharacterCount: int32(metrics.CharacterCount),
		Author:         metrics.Author,
		Title:          metrics.Title,
	}
}

func indexingStateToProto(state entity.IndexedState) *cortexv1.IndexingState {
	return &cortexv1.IndexingState{
		Basic:       state.Basic,
		Mime:        state.Mime,
		Code:        state.Code,
		Document:    state.Document,
		Mirror:      state.Mirror,
		OsMetadata:  false, // Not tracked in IndexedState currently
		Enrichment:  false, // Not tracked in IndexedState currently
	}
}

func indexingErrorsToProto(errors []entity.IndexingError) []*cortexv1.IndexingError {
	if len(errors) == 0 {
		return nil
	}
	result := make([]*cortexv1.IndexingError, 0, len(errors))
	for _, err := range errors {
		result = append(result, &cortexv1.IndexingError{
			Stage:       err.Stage,
			Operation:   err.Operation,
			Error:       err.Error,
			Details:     err.Details,
			Requirement: err.Requirement,
			Timestamp:   err.Timestamp.Unix(),
		})
	}
	return result
}

func contentQualityToProto(quality *entity.ContentQuality) *cortexv1.ContentQuality {
	if quality == nil {
		return nil
	}
	return &cortexv1.ContentQuality{
		ReadabilityScore: quality.ReadabilityScore,
		ComplexityScore:  quality.ComplexityScore,
		QualityScore:     quality.QualityScore,
		ReadabilityLevel: stringPtr(quality.ReadabilityLevel),
	}
}

func imageMetadataToProto(meta *entity.ImageMetadata) *cortexv1.ImageMetadata {
	if meta == nil {
		return nil
	}
	return &cortexv1.ImageMetadata{
		Width:          int32(meta.Width),
		Height:         int32(meta.Height),
		ColorDepth:     int32(meta.ColorDepth),
		ColorSpace:     meta.ColorSpace,
		Format:         meta.Format,
		Orientation:    int32Ptr(meta.Orientation),
		CameraMake:     meta.EXIFCameraMake,
		CameraModel:    meta.EXIFCameraModel,
		Software:       meta.EXIFSoftware,
		DateTaken:      timeToUnixPtr(meta.EXIFDateTimeOriginal),
		Artist:         meta.EXIFArtist,
		Copyright:      meta.EXIFCopyright,
		FNumber:        meta.EXIFFNumber,
		ExposureTime:   meta.EXIFExposureTime,
		Iso:            int32Ptr(meta.EXIFISO),
		FocalLength:    meta.EXIFFocalLength,
		FocalLength_35Mm: int32Ptr(meta.EXIFFocalLength35mm),
		Flash:          meta.EXIFFlash,
		GpsLatitude:    meta.GPSLatitude,
		GpsLongitude:   meta.GPSLongitude,
		GpsAltitude:    meta.GPSAltitude,
		GpsLocation:    meta.GPSLocation,
		IptcCaption:    meta.IPTCCaption,
		IptcKeywords:   meta.IPTCKeywords,
		IptcByline:     meta.IPTCByline,
		IptcCopyright:  meta.IPTCCopyrightNotice,
		DominantColors: meta.DominantColors,
		HasTransparency: meta.HasTransparency,
		IsAnimated:     meta.IsAnimated,
		FrameCount:     int32Ptr(meta.FrameCount),
	}
}

func audioMetadataToProto(meta *entity.AudioMetadata) *cortexv1.AudioMetadata {
	if meta == nil {
		return nil
	}
	return &cortexv1.AudioMetadata{
		Duration:     meta.Duration,
		Bitrate:      int32Ptr(meta.Bitrate),
		SampleRate:   int32Ptr(meta.SampleRate),
		Channels:     int32Ptr(meta.Channels),
		BitDepth:     int32Ptr(meta.BitDepth),
		Codec:        meta.Codec,
		Format:       meta.Format,
		Title:        meta.ID3Title,
		Artist:       meta.ID3Artist,
		Album:        meta.ID3Album,
		Year:         int32Ptr(meta.ID3Year),
		Genre:        meta.ID3Genre,
		Track:        int32Ptr(meta.ID3Track),
		Disc:         int32Ptr(meta.ID3Disc),
		Composer:     meta.ID3Composer,
		Publisher:    meta.ID3Publisher,
		Comment:      meta.ID3Comment,
		Bpm:          int32Ptr(meta.ID3BPM),
		Isrc:         meta.ID3ISRC,
		Copyright:    meta.ID3Copyright,
		AlbumArtist:  meta.ID3AlbumArtist,
		HasAlbumArt:  meta.HasAlbumArt,
		ReplayGain:   meta.ReplayGain,
		IsLossless:   meta.Lossless,
	}
}

func videoMetadataToProto(meta *entity.VideoMetadata) *cortexv1.VideoMetadata {
	if meta == nil {
		return nil
	}
	return &cortexv1.VideoMetadata{
		Duration:        meta.Duration,
		Width:           int32(meta.Width),
		Height:          int32(meta.Height),
		FrameRate:       meta.FrameRate,
		Bitrate:         int32Ptr(meta.Bitrate),
		Codec:           meta.Codec,
		Container:       meta.Container,
		VideoCodec:      meta.VideoCodec,
		VideoBitrate:     int32Ptr(meta.VideoBitrate),
		PixelFormat:     meta.VideoPixelFormat,
		AspectRatio:     meta.VideoAspectRatio,
		AudioCodec:      meta.AudioCodec,
		AudioBitrate:    int32Ptr(meta.AudioBitrate),
		AudioSampleRate: int32Ptr(meta.AudioSampleRate),
		AudioChannels:   int32Ptr(meta.AudioChannels),
		AudioLanguage:   meta.AudioLanguage,
		Title:           meta.Title,
		Artist:          meta.Artist,
		Album:           meta.Album,
		Genre:           meta.Genre,
		Year:            int32Ptr(meta.Year),
		Director:        meta.Director,
		Description:     meta.Description,
		Copyright:       meta.Copyright,
		HasSubtitles:    meta.HasSubtitles,
		SubtitleLanguages: meta.SubtitleTracks,
		HasChapters:     meta.HasChapters,
		ChapterCount:    int32Ptr(meta.ChapterCount),
		IsHd:            meta.IsHD,
		Is_4K:           meta.Is4K,
	}
}

func osMetadataToProto(meta *entity.OSMetadata) *cortexv1.OSMetadata {
	if meta == nil {
		return nil
	}
	return &cortexv1.OSMetadata{
		Permissions:   permissionsInfoToProto(meta.Permissions),
		Owner:         userInfoToProto(meta.Owner),
		Group:         groupInfoToProto(meta.Group),
		Attributes:    fileAttributesToProto(meta.FileAttributes),
		ExtendedAttrs: meta.ExtendedAttrs,
		Acls:          aclEntriesToProto(meta.ACLs),
		Timestamps:    osTimestampsToProto(meta.Timestamps),
		FileSystem:    fileSystemInfoToProto(meta.FileSystem),
	}
}

func osContextTaxonomyToProto(tax *entity.OSContextTaxonomy) *cortexv1.OSContextTaxonomy {
	if tax == nil {
		return nil
	}
	return &cortexv1.OSContextTaxonomy{
		Security:  securityTaxonomyToProto(tax.Security),
		Ownership: ownershipTaxonomyToProto(tax.Ownership),
		Temporal:  temporalTaxonomyToProto(tax.Temporal),
		System:    systemTaxonomyToProto(tax.System),
	}
}

func securityTaxonomyToProto(tax *entity.SecurityTaxonomy) *cortexv1.SecurityTaxonomy {
	if tax == nil {
		return nil
	}
	flags := append([]string{}, tax.SecurityCategory...)
	flags = append(flags, tax.SecurityAttributes...)
	return &cortexv1.SecurityTaxonomy{
		PermissionLevel: tax.PermissionLevel,
		SecurityFlags:   flags,
	}
}

func ownershipTaxonomyToProto(tax *entity.OwnershipTaxonomy) *cortexv1.OwnershipTaxonomy {
	if tax == nil {
		return nil
	}
	return &cortexv1.OwnershipTaxonomy{
		OwnerType:     tax.OwnerType,
		GroupCategory: tax.GroupCategory,
	}
}

func temporalTaxonomyToProto(tax *entity.TemporalTaxonomy) *cortexv1.TemporalTaxonomy {
	if tax == nil {
		return nil
	}
	return &cortexv1.TemporalTaxonomy{
		AccessFrequency:  tax.AccessFrequency,
		AgeCategory:      "",
		StalenessCategory: "",
	}
}

func systemTaxonomyToProto(tax *entity.SystemTaxonomy) *cortexv1.SystemTaxonomy {
	if tax == nil {
		return nil
	}
	return &cortexv1.SystemTaxonomy{
		FileTypeCategory: tax.SystemFileType,
		FsCategory:       tax.FileSystemCategory,
		SystemAttributes: tax.SystemAttributes,
	}
}

func permissionsInfoToProto(info *entity.PermissionsInfo) *cortexv1.PermissionsInfo {
	if info == nil {
		return nil
	}
	return &cortexv1.PermissionsInfo{
		Octal:        info.Octal,
		StringRepr:   info.String,
		OwnerRead:    info.OwnerRead,
		OwnerWrite:   info.OwnerWrite,
		OwnerExecute: info.OwnerExecute,
		GroupRead:    info.GroupRead,
		GroupWrite:   info.GroupWrite,
		GroupExecute: info.GroupExecute,
		OtherRead:    info.OtherRead,
		OtherWrite:   info.OtherWrite,
		OtherExecute: info.OtherExecute,
		Setuid:       info.SetUID,
		Setgid:       info.SetGID,
		StickyBit:    info.StickyBit,
	}
}

func userInfoToProto(info *entity.UserInfo) *cortexv1.UserInfo {
	if info == nil {
		return nil
	}
	return &cortexv1.UserInfo{
		Uid:      int32(info.UID),
		Username: info.Username,
		FullName: stringPtr(info.FullName),
		HomeDir:  stringPtr(info.HomeDir),
	}
}

func groupInfoToProto(info *entity.GroupInfo) *cortexv1.GroupInfo {
	if info == nil {
		return nil
	}
	return &cortexv1.GroupInfo{
		Gid:       int32(info.GID),
		GroupName: info.GroupName,
		Members:   info.Members,
	}
}

func fileAttributesToProto(attr *entity.FileAttributes) *cortexv1.FileAttributes {
	if attr == nil {
		return nil
	}
	return &cortexv1.FileAttributes{
		IsReadOnly:   attr.IsReadOnly,
		IsHidden:     attr.IsHidden,
		IsSystem:     attr.IsSystem,
		IsArchive:    attr.IsArchive,
		IsCompressed: attr.IsCompressed,
		IsEncrypted:  attr.IsEncrypted,
	}
}

func aclEntriesToProto(entries []entity.ACLEntry) []*cortexv1.ACLEntry {
	if len(entries) == 0 {
		return nil
	}
	result := make([]*cortexv1.ACLEntry, 0, len(entries))
	for _, entry := range entries {
		permissions := []string{}
		if entry.Permissions != "" {
			if strings.Contains(entry.Permissions, ",") {
				for _, part := range strings.Split(entry.Permissions, ",") {
					if trimmed := strings.TrimSpace(part); trimmed != "" {
						permissions = append(permissions, trimmed)
					}
				}
			} else if strings.Contains(entry.Permissions, " ") {
				for _, part := range strings.Fields(entry.Permissions) {
					if part != "" {
						permissions = append(permissions, part)
					}
				}
			} else {
				permissions = []string{entry.Permissions}
			}
		}
		isInherited := false
		if entry.Flags != "" {
			lowerFlags := strings.ToLower(entry.Flags)
			isInherited = strings.Contains(lowerFlags, "inherit")
		}
		result = append(result, &cortexv1.ACLEntry{
			Principal:   entry.Identity,
			Type:        entry.Type,
			Permissions: permissions,
			IsInherited: isInherited,
		})
	}
	return result
}

func osTimestampsToProto(ts *entity.OSTimestamps) *cortexv1.OSTimestamps {
	if ts == nil {
		return nil
	}
	return &cortexv1.OSTimestamps{
		Created:  unixSeconds(ts.Created),
		Modified: unixSeconds(&ts.Modified),
		Accessed: unixSeconds(&ts.Accessed),
		Changed:  timeToUnixPtr(&ts.Changed),
		Backup:   timeToUnixPtr(ts.Backup),
	}
}

func fileSystemInfoToProto(info *entity.FileSystemInfo) *cortexv1.FileSystemInfo {
	if info == nil {
		return nil
	}
	return &cortexv1.FileSystemInfo{
		FsType:    stringPtr(info.FileSystemType),
		MountPoint: stringPtr(info.MountPoint),
		Device:    stringPtr(info.DeviceID),
		BlockSize: int64Ptr(info.BlockSize),
		Blocks:    int64Ptr(info.Blocks),
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func int64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func int32Ptr(value *int) *int32 {
	if value == nil {
		return nil
	}
	v := int32(*value)
	return &v
}

func timeToUnixPtr(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	v := value.Unix()
	return &v
}

func unixSeconds(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.Unix()
}

// fileMetadataToProto converts an entity.FileMetadata to a proto FileMetadata.
func fileMetadataToProto(meta *entity.FileMetadata) *cortexv1.FileMetadata {
	if meta == nil {
		return nil
	}

	return &cortexv1.FileMetadata{
		FileId:            meta.FileID.String(),
		RelativePath:      meta.RelativePath,
		Tags:              meta.Tags,
		Contexts:          meta.Contexts,
		SuggestedContexts: meta.SuggestedContexts,
		Type:              meta.Type,
		Notes:             meta.Notes,
		AiSummary:         aiSummaryToProto(meta.AISummary),
		Mirror:            mirrorMetadataToProto(meta.Mirror),
		AiCategory:        aiCategoryToProto(meta.AICategory),
		AiRelated:         aiRelatedToProto(meta.AIRelated),
		CreatedAt:         meta.CreatedAt.Unix(),
		UpdatedAt:         meta.UpdatedAt.Unix(),
	}
}

func aiSummaryToProto(summary *entity.AISummary) *cortexv1.AISummary {
	if summary == nil {
		return nil
	}
	generatedAt := int64(0)
	if !summary.GeneratedAt.IsZero() {
		generatedAt = summary.GeneratedAt.Unix()
	}
	return &cortexv1.AISummary{
		Summary:     summary.Summary,
		ContentHash: summary.ContentHash,
		KeyTerms:    summary.KeyTerms,
		GeneratedAt: generatedAt,
	}
}

func mirrorMetadataToProto(mirror *entity.MirrorMetadata) *cortexv1.MirrorMetadata {
	if mirror == nil {
		return nil
	}
	return &cortexv1.MirrorMetadata{
		Format:      string(mirror.Format),
		Path:        mirror.Path,
		SourceMtime: mirror.SourceMtime.Unix(),
		UpdatedAt:   mirror.UpdatedAt.Unix(),
	}
}

func aiCategoryToProto(category *entity.AICategory) *cortexv1.AICategory {
	if category == nil {
		return nil
	}
	return &cortexv1.AICategory{
		Category:   category.Category,
		Confidence: category.Confidence,
		UpdatedAt:  category.UpdatedAt.Unix(),
	}
}

func aiRelatedToProto(related []entity.RelatedFile) []*cortexv1.RelatedFile {
	if len(related) == 0 {
		return nil
	}
	out := make([]*cortexv1.RelatedFile, 0, len(related))
	for _, item := range related {
		out = append(out, &cortexv1.RelatedFile{
			RelativePath: item.RelativePath,
			Similarity:   item.Similarity,
			Reason:       item.Reason,
		})
	}
	return out
}

// suggestedMetadataToProto converts an entity.SuggestedMetadata to a proto SuggestedMetadata.
func suggestedMetadataToProto(sm *entity.SuggestedMetadata) *cortexv1.SuggestedMetadata {
	if sm == nil {
		return nil
	}

	tags := make([]*cortexv1.SuggestedTag, 0, len(sm.SuggestedTags))
	for _, tag := range sm.SuggestedTags {
		tags = append(tags, &cortexv1.SuggestedTag{
			Tag:        tag.Tag,
			Confidence: tag.Confidence,
			Reason:     tag.Reason,
			Source:     tag.Source,
			Category:   tag.Category,
		})
	}

	projects := make([]*cortexv1.SuggestedProject, 0, len(sm.SuggestedProjects))
	for _, project := range sm.SuggestedProjects {
		var projectID *string
		if project.ProjectID != nil {
			id := project.ProjectID.String()
			projectID = &id
		}
		projects = append(projects, &cortexv1.SuggestedProject{
			ProjectId:   projectID,
			ProjectName: project.ProjectName,
			Confidence:  project.Confidence,
			Reason:      project.Reason,
			Source:      project.Source,
			IsNew:       project.IsNew,
		})
	}

	fields := make([]*cortexv1.SuggestedField, 0, len(sm.SuggestedFields))
	for _, field := range sm.SuggestedFields {
		// Convert value to JSON string
		valueJSON := ""
		if field.Value != nil {
			// Simple conversion - in production, use proper JSON encoding
			if str, ok := field.Value.(string); ok {
				valueJSON = str
			} else {
				// For other types, we'd need proper JSON encoding
				valueJSON = ""
			}
		}
		fields = append(fields, &cortexv1.SuggestedField{
			FieldName:  field.FieldName,
			Value:      valueJSON,
			Confidence: field.Confidence,
			Reason:     field.Reason,
			Source:     field.Source,
			FieldType:  field.FieldType,
		})
	}

	var taxonomy *cortexv1.SuggestedTaxonomy
	if sm.SuggestedTaxonomy != nil {
		taxonomy = &cortexv1.SuggestedTaxonomy{
			Category:              sm.SuggestedTaxonomy.Category,
			Subcategory:           sm.SuggestedTaxonomy.Subcategory,
			Domain:                sm.SuggestedTaxonomy.Domain,
			Subdomain:             sm.SuggestedTaxonomy.Subdomain,
			ContentType:           sm.SuggestedTaxonomy.ContentType,
			Purpose:               sm.SuggestedTaxonomy.Purpose,
			Topic:                 sm.SuggestedTaxonomy.Topic,
			Audience:              sm.SuggestedTaxonomy.Audience,
			Language:              sm.SuggestedTaxonomy.Language,
			CategoryConfidence:    sm.SuggestedTaxonomy.CategoryConfidence,
			DomainConfidence:      sm.SuggestedTaxonomy.DomainConfidence,
			ContentTypeConfidence: sm.SuggestedTaxonomy.ContentTypeConfidence,
			Reasoning:             sm.SuggestedTaxonomy.Reasoning,
			Source:                sm.SuggestedTaxonomy.Source,
		}
	}

	generatedAt := int64(0)
	if !sm.GeneratedAt.IsZero() {
		generatedAt = sm.GeneratedAt.Unix()
	}
	updatedAt := int64(0)
	if !sm.UpdatedAt.IsZero() {
		updatedAt = sm.UpdatedAt.Unix()
	}

	return &cortexv1.SuggestedMetadata{
		FileId:            sm.FileID.String(),
		RelativePath:      sm.RelativePath,
		SuggestedTags:     tags,
		SuggestedProjects: projects,
		SuggestedTaxonomy: taxonomy,
		SuggestedFields:   fields,
		Confidence:        sm.Confidence,
		Source:            sm.Source,
		GeneratedAt:       generatedAt,
		UpdatedAt:         updatedAt,
	}
}

func projectAssignmentToProto(assignment *entity.ProjectAssignment) *cortexv1.ProjectAssignment {
	if assignment == nil {
		return nil
	}

	var projectID *string
	if assignment.ProjectID != "" {
		id := assignment.ProjectID.String()
		projectID = &id
	}

	status := cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_UNSPECIFIED
	switch assignment.Status {
	case entity.ProjectAssignmentAuto:
		status = cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_AUTO
	case entity.ProjectAssignmentSuggested:
		status = cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_SUGGESTED
	case entity.ProjectAssignmentRejected:
		status = cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_REJECTED
	case entity.ProjectAssignmentManual:
		status = cortexv1.ProjectAssignmentStatus_PROJECT_ASSIGNMENT_STATUS_MANUAL
	}

	createdAt := int64(0)
	if !assignment.CreatedAt.IsZero() {
		createdAt = assignment.CreatedAt.UnixMilli()
	}
	updatedAt := int64(0)
	if !assignment.UpdatedAt.IsZero() {
		updatedAt = assignment.UpdatedAt.UnixMilli()
	}

	return &cortexv1.ProjectAssignment{
		WorkspaceId: assignment.WorkspaceID.String(),
		FileId:      assignment.FileID.String(),
		ProjectId:   projectID,
		ProjectName: assignment.ProjectName,
		Score:       assignment.Score,
		Sources:     assignment.Sources,
		Status:      status,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
