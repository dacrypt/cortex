// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// FolderIndexConfig contains configuration for the folder indexing stage.
type FolderIndexConfig struct {
	Enabled                  bool
	InferProjectsFromFolders bool
	MinFilesForProject       int
}

// FolderIndexStage indexes folders as first-class entities with aggregated metrics.
type FolderIndexStage struct {
	config     FolderIndexConfig
	logger     zerolog.Logger
	folderRepo repository.FolderRepository
	folders    map[string]*entity.FolderEntry // relativePath -> FolderEntry
}

// NewFolderIndexStage creates a new folder indexing stage.
func NewFolderIndexStage(config FolderIndexConfig, folderRepo repository.FolderRepository, logger zerolog.Logger) *FolderIndexStage {
	return &FolderIndexStage{
		config:     config,
		logger:     logger.With().Str("component", "folder_index_stage").Logger(),
		folderRepo: folderRepo,
		folders:    make(map[string]*entity.FolderEntry),
	}
}

// Name returns the stage name.
func (s *FolderIndexStage) Name() string {
	return "folder_index"
}

// Process updates folder metrics based on the file entry.
func (s *FolderIndexStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if entry == nil {
		return nil
	}

	// Get workspace info from context for persistence
	wsInfo, hasWorkspace := contextinfo.GetWorkspaceInfo(ctx)

	// Get or create folder entries for all parent directories
	folderPath := filepath.Dir(entry.RelativePath)
	if folderPath == "." {
		folderPath = ""
	}

	// Collect folders to persist
	var foldersToSave []*entity.FolderEntry

	// Walk up the directory tree and update each folder
	currentPath := folderPath
	for currentPath != "" {
		folder := s.getOrCreateFolder(entry.AbsolutePath, currentPath)
		s.updateFolderMetrics(folder, entry, currentPath == folderPath)
		foldersToSave = append(foldersToSave, folder)

		// Move to parent
		parentPath := filepath.Dir(currentPath)
		if parentPath == "." || parentPath == currentPath {
			break
		}
		currentPath = parentPath
	}

	// Also update root folder if file is at root
	if folderPath == "" {
		folder := s.getOrCreateFolder(entry.AbsolutePath, "")
		s.updateFolderMetrics(folder, entry, true)
		foldersToSave = append(foldersToSave, folder)
	}

	// Persist folders if repository is available
	if s.folderRepo != nil && hasWorkspace {
		for _, folder := range foldersToSave {
			if err := s.folderRepo.Upsert(ctx, wsInfo.ID, folder); err != nil {
				s.logger.Warn().Err(err).
					Str("folder", folder.RelativePath).
					Msg("Failed to persist folder")
			}
		}
	}

	return nil
}

// getOrCreateFolder gets or creates a folder entry.
func (s *FolderIndexStage) getOrCreateFolder(fileAbsPath, relativePath string) *entity.FolderEntry {
	if folder, exists := s.folders[relativePath]; exists {
		return folder
	}

	// Derive workspace root from file path
	workspaceRoot := strings.TrimSuffix(fileAbsPath, relativePath)
	if relativePath != "" {
		workspaceRoot = strings.TrimSuffix(workspaceRoot, string(filepath.Separator))
	}

	folder := entity.NewFolderEntry(workspaceRoot, relativePath)
	folder.Metrics = &entity.FolderMetrics{
		FileTypeCounts: make(map[string]int),
		MimeTypeCounts: make(map[string]int),
	}
	folder.Metadata = &entity.FolderMetadata{}

	s.folders[relativePath] = folder
	return folder
}

// updateFolderMetrics updates the folder metrics with file data.
func (s *FolderIndexStage) updateFolderMetrics(folder *entity.FolderEntry, file *entity.FileEntry, isDirect bool) {
	if folder.Metrics == nil {
		folder.Metrics = &entity.FolderMetrics{
			FileTypeCounts: make(map[string]int),
			MimeTypeCounts: make(map[string]int),
		}
	}

	m := folder.Metrics
	folder.UpdatedAt = time.Now()

	// Update file counts
	m.TotalFiles++
	if isDirect {
		m.DirectFiles++
	} else {
		m.RecursiveFiles++
	}

	// Update size metrics
	m.TotalSize += file.FileSize
	if isDirect {
		m.DirectSize += file.FileSize
	} else {
		m.RecursiveSize += file.FileSize
	}

	// Update type counts
	ext := strings.ToLower(file.Extension)
	if ext != "" {
		m.FileTypeCounts[ext]++
	}

	// Update MIME type counts
	if file.Enhanced != nil && file.Enhanced.MimeType != nil {
		category := file.Enhanced.MimeType.Category
		if category != "" {
			m.MimeTypeCounts[category]++
		}
	}

	// Update temporal metrics
	if m.OldestFile == nil || file.LastModified.Before(*m.OldestFile) {
		m.OldestFile = &file.LastModified
	}
	if m.NewestFile == nil || file.LastModified.After(*m.NewestFile) {
		m.NewestFile = &file.LastModified
	}

	// Update code metrics if available
	if file.Enhanced != nil && file.Enhanced.CodeMetrics != nil {
		cm := file.Enhanced.CodeMetrics
		m.TotalLinesOfCode += cm.LinesOfCode
		m.TotalCommentLines += cm.CommentLines
		m.TotalFunctions += cm.FunctionCount
		m.TotalClasses += cm.ClassCount
		// Running average for complexity
		if cm.Complexity > 0 {
			total := m.AverageComplexity * float64(m.TotalFiles-1)
			m.AverageComplexity = (total + cm.Complexity) / float64(m.TotalFiles)
		}
	}

	// Update document metrics if available
	if file.Enhanced != nil && file.Enhanced.DocumentMetrics != nil {
		dm := file.Enhanced.DocumentMetrics
		m.TotalPages += dm.PageCount
		m.TotalWords += dm.WordCount
		m.TotalDocuments++
	}

	// Infer folder metadata
	s.inferFolderMetadata(folder)
}

// inferFolderMetadata infers metadata from folder contents.
func (s *FolderIndexStage) inferFolderMetadata(folder *entity.FolderEntry) {
	if folder.Metrics == nil || folder.Metadata == nil {
		return
	}

	m := folder.Metrics
	meta := folder.Metadata

	// Find dominant file type
	var dominantType string
	maxCount := 0
	for ext, count := range m.FileTypeCounts {
		if count > maxCount {
			maxCount = count
			dominantType = ext
		}
	}
	if dominantType != "" {
		meta.DominantFileType = &dominantType
	}

	// Infer folder nature from file types and folder name
	meta.ProjectNature = inferFolderNaturePtr(folder.Name, m.FileTypeCounts, m.MimeTypeCounts)

	// Infer project name from folder name if enough files
	if s.config.InferProjectsFromFolders && m.DirectFiles >= s.config.MinFilesForProject {
		if meta.InferredProject == nil {
			projectName := folder.Name
			meta.InferredProject = &projectName
			meta.ProjectConfidence = 0.5 // Base confidence from folder name
		}
	}
}

// inferFolderNaturePtr infers the nature of a folder from its contents.
func inferFolderNaturePtr(name string, fileTypes, mimeTypes map[string]int) *string {
	nature := inferFolderNature(name, fileTypes, mimeTypes)
	natureStr := string(nature)
	return &natureStr
}

// inferFolderNature infers the nature of a folder from its contents.
func inferFolderNature(name string, fileTypes, mimeTypes map[string]int) entity.FolderNature {
	nameLower := strings.ToLower(name)

	// Check folder name patterns
	switch {
	case strings.Contains(nameLower, "test") || strings.Contains(nameLower, "spec"):
		return entity.FolderNatureTests
	case strings.Contains(nameLower, "doc") || strings.Contains(nameLower, "docs"):
		return entity.FolderNatureDocumentation
	case strings.Contains(nameLower, "vendor") || strings.Contains(nameLower, "node_modules"):
		return entity.FolderNatureVendor
	case strings.Contains(nameLower, "build") || strings.Contains(nameLower, "dist") || strings.Contains(nameLower, "out"):
		return entity.FolderNatureBuild
	case strings.Contains(nameLower, "config") || strings.Contains(nameLower, "conf"):
		return entity.FolderNatureConfiguration
	case strings.Contains(nameLower, "data") || strings.Contains(nameLower, "dataset"):
		return entity.FolderNatureData
	case strings.Contains(nameLower, "asset") || strings.Contains(nameLower, "resource") || strings.Contains(nameLower, "static"):
		return entity.FolderNatureResources
	case strings.Contains(nameLower, "media") || strings.Contains(nameLower, "image") || strings.Contains(nameLower, "video"):
		return entity.FolderNatureMedia
	case strings.Contains(nameLower, "archive") || strings.Contains(nameLower, "backup"):
		return entity.FolderNatureArchive
	}

	// Infer from file types
	codeExtensions := map[string]bool{
		".go": true, ".ts": true, ".js": true, ".py": true, ".java": true,
		".rs": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rb": true, ".php": true, ".swift": true, ".kt": true,
	}
	docExtensions := map[string]bool{
		".md": true, ".txt": true, ".pdf": true, ".doc": true, ".docx": true,
		".rst": true, ".adoc": true,
	}
	mediaExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true,
		".mp3": true, ".wav": true, ".mp4": true, ".mov": true, ".avi": true,
	}
	dataExtensions := map[string]bool{
		".json": true, ".csv": true, ".xml": true, ".yaml": true, ".yml": true,
		".sql": true, ".db": true, ".sqlite": true,
	}
	configExtensions := map[string]bool{
		".env": true, ".ini": true, ".toml": true, ".conf": true,
	}

	var codeCount, docCount, mediaCount, dataCount, configCount int
	for ext, count := range fileTypes {
		if codeExtensions[ext] {
			codeCount += count
		}
		if docExtensions[ext] {
			docCount += count
		}
		if mediaExtensions[ext] {
			mediaCount += count
		}
		if dataExtensions[ext] {
			dataCount += count
		}
		if configExtensions[ext] {
			configCount += count
		}
	}

	total := codeCount + docCount + mediaCount + dataCount + configCount
	if total == 0 {
		return entity.FolderNatureUnknown
	}

	// Determine dominant nature (>50% of files)
	threshold := total / 2
	switch {
	case codeCount > threshold:
		return entity.FolderNatureDevelopment
	case docCount > threshold:
		return entity.FolderNatureDocumentation
	case mediaCount > threshold:
		return entity.FolderNatureMedia
	case dataCount > threshold:
		return entity.FolderNatureData
	case configCount > threshold:
		return entity.FolderNatureConfiguration
	default:
		return entity.FolderNatureMixed
	}
}

// GetFolders returns all indexed folders.
func (s *FolderIndexStage) GetFolders() map[string]*entity.FolderEntry {
	return s.folders
}

// Reset clears the folder cache (for new scans).
func (s *FolderIndexStage) Reset() {
	s.folders = make(map[string]*entity.FolderEntry)
}
