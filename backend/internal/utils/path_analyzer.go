package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// PathAnalyzer provides utilities for analyzing file paths.
type PathAnalyzer struct{}

// NewPathAnalyzer creates a new path analyzer.
func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{}
}

// ExtractComponents breaks down a relative path into semantic components.
// Example: "docs/projects/website/README.md" -> ["docs", "projects", "website", "README.md"]
func (p *PathAnalyzer) ExtractComponents(relativePath string) []string {
	if relativePath == "" {
		return []string{}
	}

	// Normalize path separators
	normalized := filepath.ToSlash(relativePath)
	
	// Split by path separator
	components := strings.Split(normalized, "/")
	
	// Filter out empty components
	result := make([]string, 0, len(components))
	for _, comp := range components {
		if comp != "" {
			result = append(result, comp)
		}
	}
	
	return result
}

// ExtractPattern extracts a normalized path pattern for matching.
// Example: "docs/projects/website/README.md" -> "docs/projects/*/README.md"
// This helps identify files in similar directory structures.
func (p *PathAnalyzer) ExtractPattern(relativePath string) string {
	if relativePath == "" {
		return ""
	}

	components := p.ExtractComponents(relativePath)
	if len(components) == 0 {
		return ""
	}

	// For patterns, we can create variations:
	// - Full path pattern
	// - Directory pattern (without filename)
	// - Common patterns like "*/README.md", "docs/*/*.md", etc.
	
	// Simple pattern: replace specific directory names with * for common structures
	// This is a basic implementation - can be enhanced later
	pattern := strings.Join(components, "/")
	
	// Replace common variable parts with wildcards
	// e.g., "docs/projects/project-a/README.md" -> "docs/projects/*/README.md"
	// This is a heuristic - can be made configurable
	re := regexp.MustCompile(`/([^/]+)/([^/]+)/([^/]+)/`)
	pattern = re.ReplaceAllString(pattern, "/*/*/")
	
	return pattern
}

// GetDirectoryPath returns the directory portion of the path.
func (p *PathAnalyzer) GetDirectoryPath(relativePath string) string {
	return filepath.Dir(relativePath)
}

// GetFilename returns just the filename portion.
func (p *PathAnalyzer) GetFilename(relativePath string) string {
	return filepath.Base(relativePath)
}

// GetDepth returns the directory depth of the path.
func (p *PathAnalyzer) GetDepth(relativePath string) int {
	components := p.ExtractComponents(relativePath)
	// Depth is number of components minus 1 (for the filename)
	if len(components) <= 1 {
		return 0
	}
	return len(components) - 1
}

// ExtractSemanticInfo extracts semantic information from path components.
// Returns a map of semantic hints that can be used for project assignment.
func (p *PathAnalyzer) ExtractSemanticInfo(relativePath string) map[string]string {
	info := make(map[string]string)
	components := p.ExtractComponents(relativePath)
	
	if len(components) == 0 {
		return info
	}

	// Extract common semantic patterns
	// Project indicators
	for i, comp := range components {
		lower := strings.ToLower(comp)
		
		// Check for project-related directories
		if strings.Contains(lower, "project") || strings.Contains(lower, "proj") {
			info["has_project_indicator"] = "true"
			if i+1 < len(components) {
				info["project_name_hint"] = components[i+1]
			}
		}
		
		// Check for documentation
		if lower == "docs" || lower == "documentation" || lower == "doc" {
			info["is_documentation"] = "true"
		}
		
		// Check for source code
		if lower == "src" || lower == "source" || lower == "lib" || lower == "libs" {
			info["is_source_code"] = "true"
		}
		
		// Check for tests
		if lower == "test" || lower == "tests" || lower == "spec" || lower == "specs" {
			info["is_test"] = "true"
		}
		
		// Check for configuration
		if lower == "config" || lower == "conf" || lower == "cfg" {
			info["is_config"] = "true"
		}
	}
	
	// Extract filename patterns
	filename := p.GetFilename(relativePath)
	lowerFilename := strings.ToLower(filename)
	
	if strings.HasPrefix(lowerFilename, "readme") {
		info["is_readme"] = "true"
	}
	if strings.HasPrefix(lowerFilename, "license") {
		info["is_license"] = "true"
	}
	if strings.HasPrefix(lowerFilename, "changelog") || strings.HasPrefix(lowerFilename, "changes") {
		info["is_changelog"] = "true"
	}
	
	return info
}

// FormatPathForContext formats a path for inclusion in AI context.
// Returns a human-readable description of the path's semantic meaning.
func (p *PathAnalyzer) FormatPathForContext(relativePath string) string {
	if relativePath == "" {
		return ""
	}

	components := p.ExtractComponents(relativePath)
	if len(components) == 0 {
		return ""
	}

	// Build a natural language description
	var parts []string
	
	// Add directory context
	if len(components) > 1 {
		dirParts := components[:len(components)-1]
		parts = append(parts, "Located in: "+strings.Join(dirParts, "/"))
	}
	
	// Add semantic hints
	semantic := p.ExtractSemanticInfo(relativePath)
	if semantic["is_documentation"] == "true" {
		parts = append(parts, "Documentation file")
	}
	if semantic["is_source_code"] == "true" {
		parts = append(parts, "Source code file")
	}
	if semantic["is_test"] == "true" {
		parts = append(parts, "Test file")
	}
	if semantic["has_project_indicator"] == "true" && semantic["project_name_hint"] != "" {
		parts = append(parts, "Part of project: "+semantic["project_name_hint"])
	}
	
	if len(parts) > 0 {
		return strings.Join(parts, ". ")
	}
	
	// Fallback to just the path
	return "Path: " + relativePath
}



