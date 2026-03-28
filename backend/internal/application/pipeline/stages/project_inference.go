// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/pipeline/contextinfo"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// ProjectInferenceConfig contains configuration for project inference.
type ProjectInferenceConfig struct {
	Enabled               bool
	MinFilesForProject    int
	ConfidenceThreshold   float64 // Minimum confidence to auto-create project
	UseAIForDescription   bool
	MaxProjectsPerFolder  int
}

// InferredProject represents a potential project identified from folder structure.
type InferredProject struct {
	Name              string
	FolderPath        string
	Nature            string
	Confidence        float64
	FileCount         int
	IndicatorFiles    []string // Files that indicate this is a project (README, package.json, etc.)
	DominantLanguage  string
	Description       string
}

// ProjectInferenceStage infers projects from folder structure and file patterns.
type ProjectInferenceStage struct {
	config              ProjectInferenceConfig
	logger              zerolog.Logger
	inferredProjectRepo repository.InferredProjectRepository
	candidates          map[string]*InferredProject // folderPath -> candidate
	filesByFolder       map[string][]*entity.FileEntry
}

// NewProjectInferenceStage creates a new project inference stage.
func NewProjectInferenceStage(config ProjectInferenceConfig, inferredProjectRepo repository.InferredProjectRepository, logger zerolog.Logger) *ProjectInferenceStage {
	return &ProjectInferenceStage{
		config:              config,
		logger:              logger.With().Str("component", "project_inference_stage").Logger(),
		inferredProjectRepo: inferredProjectRepo,
		candidates:          make(map[string]*InferredProject),
		filesByFolder:       make(map[string][]*entity.FileEntry),
	}
}

// Name returns the stage name.
func (s *ProjectInferenceStage) Name() string {
	return "project_inference"
}

// Process analyzes a file and updates project candidates.
func (s *ProjectInferenceStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if entry == nil {
		return nil
	}

	// Get workspace info from context for persistence
	wsInfo, hasWorkspace := contextinfo.GetWorkspaceInfo(ctx)

	// Get folder path
	folderPath := filepath.Dir(entry.RelativePath)
	if folderPath == "." {
		folderPath = ""
	}

	// Track files by folder
	s.filesByFolder[folderPath] = append(s.filesByFolder[folderPath], entry)

	// Check if this file is a project indicator
	if isProjectIndicatorFile(entry.Filename) {
		s.updateInferredProject(folderPath, entry)

		// Persist the updated candidate if repository is available
		if s.inferredProjectRepo != nil && hasWorkspace {
			candidate := s.candidates[folderPath]
			if candidate != nil {
				// Update file count and description before persisting
				candidate.FileCount = len(s.filesByFolder[folderPath])
				candidate.DominantLanguage = s.inferDominantLanguage(s.filesByFolder[folderPath])
				candidate.Description = s.generateDescription(candidate)

				// Convert to repository type and persist
				repoProject := s.toRepoProject(candidate)
				if err := s.inferredProjectRepo.Upsert(ctx, wsInfo.ID, repoProject); err != nil {
					s.logger.Warn().Err(err).
						Str("project", candidate.Name).
						Str("folder", folderPath).
						Msg("Failed to persist inferred project")
				}
			}
		}
	}

	return nil
}

// toRepoProject converts an InferredProject to repository.InferredProject.
func (s *ProjectInferenceStage) toRepoProject(p *InferredProject) *repository.InferredProject {
	// Generate a stable ID from folder path
	hash := sha256.Sum256([]byte("inferred_project:" + p.FolderPath))
	id := hex.EncodeToString(hash[:])

	return &repository.InferredProject{
		ID:               id,
		Name:             p.Name,
		FolderPath:       p.FolderPath,
		Nature:           p.Nature,
		Confidence:       p.Confidence,
		FileCount:        p.FileCount,
		IndicatorFiles:   p.IndicatorFiles,
		DominantLanguage: p.DominantLanguage,
		Description:      p.Description,
		AutoCreated:      false, // Will be set to true when auto-created as a real project
	}
}

// updateInferredProject updates or creates a project candidate for a folder.
func (s *ProjectInferenceStage) updateInferredProject(folderPath string, indicatorFile *entity.FileEntry) {
	candidate, exists := s.candidates[folderPath]
	if !exists {
		folderName := filepath.Base(folderPath)
		if folderPath == "" {
			folderName = "root"
		}

		candidate = &InferredProject{
			Name:           normalizeProjectName(folderName),
			FolderPath:     folderPath,
			Confidence:     0.0,
			IndicatorFiles: []string{},
		}
		s.candidates[folderPath] = candidate
	}

	// Add indicator file
	candidate.IndicatorFiles = append(candidate.IndicatorFiles, indicatorFile.Filename)

	// Update confidence based on indicator
	candidate.Confidence += getProjectIndicatorConfidence(indicatorFile.Filename)
	if candidate.Confidence > 1.0 {
		candidate.Confidence = 1.0
	}

	// Infer project nature from indicator files
	candidate.Nature = inferProjectNatureFromIndicators(candidate.IndicatorFiles)
}

// Project indicator file constants
const (
	indicatorPackageJSON     = "package.json"
	indicatorGoMod           = "go.mod"
	indicatorCargoToml       = "Cargo.toml"
	indicatorPomXML          = "pom.xml"
	indicatorPyprojectToml   = "pyproject.toml"
	indicatorSetupPy         = "setup.py"
	indicatorRequirementsTxt = "requirements.txt"
	indicatorComposerJSON    = "composer.json"
	indicatorGemfile         = "Gemfile"
	indicatorMakefile        = "Makefile"
	indicatorDockerfile      = "Dockerfile"
	indicatorReadmeMD        = "README.md"
	indicatorLicense         = "LICENSE"
)

// isProjectIndicatorFile checks if a file indicates a project root.
func isProjectIndicatorFile(filename string) bool {
	indicators := map[string]bool{
		// Documentation
		indicatorReadmeMD:    true,
		"README.txt":         true,
		"README":             true,
		"readme.md":          true,

		// Package managers
		indicatorPackageJSON: true,
		"package-lock.json":  true,
		"yarn.lock":          true,
		"pnpm-lock.yaml":     true,
		indicatorGoMod:       true,
		"go.sum":             true,
		indicatorCargoToml:   true,
		"Cargo.lock":         true,
		indicatorPomXML:      true,
		"build.gradle":       true,
		"build.gradle.kts":   true,
		indicatorRequirementsTxt: true,
		indicatorSetupPy:         true,
		indicatorPyprojectToml:   true,
		indicatorGemfile:         true,
		"Gemfile.lock":           true,
		indicatorComposerJSON:    true,
		"composer.lock":          true,
		"Podfile":                true,
		"Podfile.lock":           true,

		// Build/Configuration
		indicatorMakefile:    true,
		"CMakeLists.txt":     true,
		indicatorDockerfile:  true,
		"docker-compose.yml": true,
		"docker-compose.yaml": true,
		".gitignore":         true,
		".git":               true,

		// IDE/Editor
		".vscode":            true,
		".idea":              true,
		".project":           true,

		// CI/CD
		".github":            true,
		".gitlab-ci.yml":     true,
		".travis.yml":        true,
		"Jenkinsfile":        true,

		// License
		indicatorLicense:     true,
		"LICENSE.md":         true,
		"LICENSE.txt":        true,
		"COPYING":            true,

		// Other
		"CHANGELOG.md":       true,
		"CONTRIBUTING.md":    true,
		".editorconfig":      true,
	}

	return indicators[filename]
}

// getProjectIndicatorConfidence returns confidence score for an indicator file.
func getProjectIndicatorConfidence(filename string) float64 {
	// High confidence indicators (definite project)
	highConfidence := map[string]bool{
		indicatorPackageJSON:   true,
		indicatorGoMod:         true,
		indicatorCargoToml:     true,
		indicatorPomXML:        true,
		indicatorPyprojectToml: true,
		indicatorSetupPy:       true,
		indicatorGemfile:       true,
		indicatorComposerJSON:  true,
		".git":                 true,
	}

	// Medium confidence
	mediumConfidence := map[string]bool{
		indicatorReadmeMD:        true,
		indicatorMakefile:        true,
		indicatorDockerfile:      true,
		indicatorRequirementsTxt: true,
		indicatorLicense:         true,
	}

	if highConfidence[filename] {
		return 0.4
	}
	if mediumConfidence[filename] {
		return 0.2
	}
	return 0.1
}

// inferProjectNatureFromIndicators infers project nature from indicator files.
func inferProjectNatureFromIndicators(indicators []string) string {
	for _, ind := range indicators {
		switch ind {
		// JavaScript/TypeScript
		case indicatorPackageJSON:
			return "javascript"
		// Go
		case indicatorGoMod:
			return "go"
		// Rust
		case indicatorCargoToml:
			return "rust"
		// Java
		case indicatorPomXML, "build.gradle", "build.gradle.kts":
			return "java"
		// Python
		case indicatorPyprojectToml, indicatorSetupPy, indicatorRequirementsTxt:
			return "python"
		// Ruby
		case indicatorGemfile:
			return "ruby"
		// PHP
		case indicatorComposerJSON:
			return "php"
		// iOS/macOS
		case "Podfile", "Package.swift":
			return "swift"
		// Docker
		case indicatorDockerfile, "docker-compose.yml":
			return "docker"
		// Documentation
		case "mkdocs.yml", "docusaurus.config.js":
			return "documentation"
		}
	}
	return "unknown"
}

// normalizeProjectName normalizes a folder name to a project name.
func normalizeProjectName(name string) string {
	// Remove common prefixes/suffixes
	name = strings.TrimPrefix(name, ".")
	name = strings.TrimSuffix(name, "-src")
	name = strings.TrimSuffix(name, "_src")
	name = strings.TrimSuffix(name, "-main")
	name = strings.TrimSuffix(name, "_main")

	// Replace separators with spaces for display
	replacer := regexp.MustCompile(`[-_]`)
	name = replacer.ReplaceAllString(name, " ")

	// Title case
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// Finalize processes all collected data and returns project candidates.
func (s *ProjectInferenceStage) Finalize() []*InferredProject {
	results := make([]*InferredProject, 0)

	for folderPath, candidate := range s.candidates {
		// Count files in folder
		files := s.filesByFolder[folderPath]
		candidate.FileCount = len(files)

		// Check minimum files
		if candidate.FileCount < s.config.MinFilesForProject {
			continue
		}

		// Check confidence threshold
		if candidate.Confidence < s.config.ConfidenceThreshold {
			continue
		}

		// Infer dominant language from files
		candidate.DominantLanguage = s.inferDominantLanguage(files)

		// Generate description
		candidate.Description = s.generateDescription(candidate)

		results = append(results, candidate)

		s.logger.Info().
			Str("project", candidate.Name).
			Str("folder", folderPath).
			Float64("confidence", candidate.Confidence).
			Int("files", candidate.FileCount).
			Str("nature", candidate.Nature).
			Msg("Identified project candidate")
	}

	return results
}

// inferDominantLanguage infers the dominant programming language from files.
func (s *ProjectInferenceStage) inferDominantLanguage(files []*entity.FileEntry) string {
	langCounts := make(map[string]int)

	langMap := map[string]string{
		".go":    "Go",
		".ts":    "TypeScript",
		".tsx":   "TypeScript",
		".js":    "JavaScript",
		".jsx":   "JavaScript",
		".py":    "Python",
		".rs":    "Rust",
		".java":  "Java",
		".kt":    "Kotlin",
		".swift": "Swift",
		".rb":    "Ruby",
		".php":   "PHP",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C",
		".hpp":   "C++",
		".cs":    "C#",
		".md":    "Markdown",
	}

	for _, file := range files {
		if lang, ok := langMap[file.Extension]; ok {
			langCounts[lang]++
		}
	}

	var dominant string
	maxCount := 0
	for lang, count := range langCounts {
		if count > maxCount {
			maxCount = count
			dominant = lang
		}
	}

	return dominant
}

// generateDescription generates a description for the project.
func (s *ProjectInferenceStage) generateDescription(candidate *InferredProject) string {
	parts := []string{}

	if candidate.DominantLanguage != "" {
		parts = append(parts, candidate.DominantLanguage+" project")
	} else if candidate.Nature != "unknown" {
		parts = append(parts, candidate.Nature+" project")
	} else {
		parts = append(parts, "Project")
	}

	parts = append(parts, "with", fmt.Sprintf("%d files", candidate.FileCount))

	if len(candidate.IndicatorFiles) > 0 {
		maxIndicators := 3
		if len(candidate.IndicatorFiles) < maxIndicators {
			maxIndicators = len(candidate.IndicatorFiles)
		}
		parts = append(parts, "containing",
			strings.Join(candidate.IndicatorFiles[:maxIndicators], ", "))
	}

	return strings.Join(parts, " ")
}

// GetInferredProjects returns all project candidates.
func (s *ProjectInferenceStage) GetInferredProjects() map[string]*InferredProject {
	return s.candidates
}

// Reset clears the stage state (for new scans).
func (s *ProjectInferenceStage) Reset() {
	s.candidates = make(map[string]*InferredProject)
	s.filesByFolder = make(map[string][]*entity.FileEntry)
}
