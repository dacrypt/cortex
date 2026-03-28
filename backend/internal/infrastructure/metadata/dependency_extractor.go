package metadata

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// DependencyExtractor extracts code dependencies from source files.
type DependencyExtractor struct {
	logger zerolog.Logger
}

// NewDependencyExtractor creates a new dependency extractor.
func NewDependencyExtractor(logger zerolog.Logger) *DependencyExtractor {
	return &DependencyExtractor{
		logger: logger.With().Str("component", "dependency_extractor").Logger(),
	}
}

// ExtractDependencies extracts dependencies from a code file.
func (e *DependencyExtractor) ExtractDependencies(ctx context.Context, filePath string, content string) ([]entity.Dependency, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".go":
		return e.extractGoDependencies(content, filePath)
	case ".py":
		return e.extractPythonDependencies(content, filePath)
	case ".js", ".jsx", ".ts", ".tsx":
		return e.extractJSDependencies(content, filePath)
	case ".java":
		return e.extractJavaDependencies(content, filePath)
	case ".rb":
		return e.extractRubyDependencies(content, filePath)
	case ".rs":
		return e.extractRustDependencies(content, filePath)
	default:
		return nil, nil // Unsupported language
	}
}

// extractGoDependencies extracts Go import statements.
func (e *DependencyExtractor) extractGoDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: import "package" or import ("package1" "package2")
	importPattern := regexp.MustCompile(`import\s+(?:"([^"]+)"|\(([^)]+)\))`)
	matches := importPattern.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if match[1] != "" {
			// Single import
			deps = append(deps, entity.Dependency{
				Name:     match[1],
				Type:     "import",
				Language: "go",
				Path:     filePath,
			})
		} else if match[2] != "" {
			// Multiple imports in parentheses
			imports := strings.Split(match[2], "\n")
			for _, imp := range imports {
				imp = strings.TrimSpace(imp)
				// Extract package name from "package" or alias "package"
				if strings.HasPrefix(imp, `"`) {
					packageName := strings.Trim(imp, `"`)
					if packageName != "" {
						deps = append(deps, entity.Dependency{
							Name:     packageName,
							Type:     "import",
							Language: "go",
							Path:     filePath,
						})
					}
				}
			}
		}
	}
	
	return deps, nil
}

// extractPythonDependencies extracts Python import statements.
func (e *DependencyExtractor) extractPythonDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: import package or from package import module
	importPattern := regexp.MustCompile(`(?:^|\n)\s*(?:import|from)\s+([a-zA-Z0-9_.]+)`)
	matches := importPattern.FindAllStringSubmatch(content, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		packageName := strings.Split(match[1], ".")[0] // Get root package
		if !seen[packageName] {
			seen[packageName] = true
			deps = append(deps, entity.Dependency{
				Name:     packageName,
				Type:     "import",
				Language: "python",
				Path:     filePath,
			})
		}
	}
	
	return deps, nil
}

// extractJSDependencies extracts JavaScript/TypeScript import/require statements.
func (e *DependencyExtractor) extractJSDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: import ... from "package" or require("package")
	importPattern := regexp.MustCompile(`(?:import|require)\s*\(?["']([^"']+)["']\)?`)
	matches := importPattern.FindAllStringSubmatch(content, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		packageName := match[1]
		// Skip relative imports
		if !strings.HasPrefix(packageName, ".") && !strings.HasPrefix(packageName, "/") {
			// Get root package name
			rootPackage := strings.Split(packageName, "/")[0]
			if strings.HasPrefix(rootPackage, "@") {
				// Scoped package: @scope/package
				parts := strings.Split(packageName, "/")
				if len(parts) >= 2 {
					rootPackage = parts[0] + "/" + parts[1]
				}
			}
			
			if !seen[rootPackage] {
				seen[rootPackage] = true
				deps = append(deps, entity.Dependency{
					Name:     rootPackage,
					Type:     "import",
					Language: "javascript",
					Path:     filePath,
				})
			}
		}
	}
	
	return deps, nil
}

// extractJavaDependencies extracts Java import statements.
func (e *DependencyExtractor) extractJavaDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: import package.Class;
	importPattern := regexp.MustCompile(`import\s+([a-zA-Z0-9_.]+);`)
	matches := importPattern.FindAllStringSubmatch(content, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		packageName := match[1]
		// Get root package
		rootPackage := strings.Split(packageName, ".")[0]
		if !seen[rootPackage] {
			seen[rootPackage] = true
			deps = append(deps, entity.Dependency{
				Name:     rootPackage,
				Type:     "import",
				Language: "java",
				Path:     filePath,
			})
		}
	}
	
	return deps, nil
}

// extractRubyDependencies extracts Ruby require/require_relative statements.
func (e *DependencyExtractor) extractRubyDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: require "package" or require_relative "file"
	requirePattern := regexp.MustCompile(`require(?:_relative)?\s+["']([^"']+)["']`)
	matches := requirePattern.FindAllStringSubmatch(content, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		packageName := match[1]
		// Skip relative requires
		if !strings.HasPrefix(packageName, ".") && !strings.HasPrefix(packageName, "/") {
			if !seen[packageName] {
				seen[packageName] = true
				deps = append(deps, entity.Dependency{
					Name:     packageName,
					Type:     "require",
					Language: "ruby",
					Path:     filePath,
				})
			}
		}
	}
	
	return deps, nil
}

// extractRustDependencies extracts Rust use statements.
func (e *DependencyExtractor) extractRustDependencies(content string, filePath string) ([]entity.Dependency, error) {
	var deps []entity.Dependency
	
	// Pattern: use package::module;
	usePattern := regexp.MustCompile(`use\s+([a-zA-Z0-9_::]+);`)
	matches := usePattern.FindAllStringSubmatch(content, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		packageName := match[1]
		// Get root package
		rootPackage := strings.Split(packageName, "::")[0]
		if !seen[rootPackage] {
			seen[rootPackage] = true
			deps = append(deps, entity.Dependency{
				Name:     rootPackage,
				Type:     "use",
				Language: "rust",
				Path:     filePath,
			})
		}
	}
	
	return deps, nil
}

