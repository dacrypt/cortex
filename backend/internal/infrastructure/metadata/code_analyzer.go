package metadata

import (
	"path/filepath"
	"regexp"
	"strings"
)

// CodeAnalyzer extracts import/export relationships from source code.
type CodeAnalyzer struct{}

// NewCodeAnalyzer creates a new code analyzer.
func NewCodeAnalyzer() *CodeAnalyzer {
	return &CodeAnalyzer{}
}

// ImportRelationship represents a code import relationship.
type ImportRelationship struct {
	Path      string // Import path/module name
	Type      string // "import", "require", "include", etc.
	Language  string // Programming language
	Line      int    // Line number where import occurs
	Confidence float64 // Confidence in extraction (0.0-1.0)
}

// ExtractImports extracts import statements from source code.
func (a *CodeAnalyzer) ExtractImports(content string, extension string) []ImportRelationship {
	var imports []ImportRelationship
	language := a.getLanguage(extension)
	
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "--") {
			continue
		}
		
		// Extract based on language
		switch language {
		case "go":
			if imp := a.extractGoImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "javascript", "typescript":
			if imp := a.extractJSImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "python":
			if imp := a.extractPythonImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "rust":
			if imp := a.extractRustImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "java":
			if imp := a.extractJavaImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "c", "cpp":
			if imp := a.extractCImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "ruby":
			if imp := a.extractRubyImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		case "php":
			if imp := a.extractPHPImport(trimmed, lineNum+1); imp != nil {
				imports = append(imports, *imp)
			}
		}
	}
	
	return imports
}

func (a *CodeAnalyzer) getLanguage(extension string) string {
	switch extension {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

// Go: import "path" or import ( "path1" "path2" )
var goImportRegex = regexp.MustCompile(`import\s+(?:"([^"]+)"|\(([^)]+)\))`)
var goImportPathRegex = regexp.MustCompile(`"([^"]+)"`)

func (a *CodeAnalyzer) extractGoImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "import") {
		return nil
	}
	
	matches := goImportPathRegex.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}
	
	// Take first import path
	path := matches[0][1]
	if path == "" {
		return nil
	}
	
	return &ImportRelationship{
		Path:      path,
		Type:      "import",
		Language:  "go",
		Line:      lineNum,
		Confidence: 0.9,
	}
}

// JavaScript/TypeScript: import ... from "path" or require("path")
// Note: Go regexp doesn't support \1 backreferences, so we match either single or double quotes
var jsImportRegex = regexp.MustCompile(`(?:import\s+.*\s+from\s+|require\s*\(\s*)(["'])([^"']+)(["'])`)
var jsRequireRegex = regexp.MustCompile(`require\s*\(\s*(["'])([^"']+)(["'])`)

func (a *CodeAnalyzer) extractJSImport(line string, lineNum int) *ImportRelationship {
	if strings.Contains(line, "import") {
		matches := jsImportRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			return &ImportRelationship{
				Path:      matches[2],
				Type:      "import",
				Language:  "javascript",
				Line:      lineNum,
				Confidence: 0.9,
			}
		}
	}
	
	if strings.Contains(line, "require") {
		matches := jsRequireRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			return &ImportRelationship{
				Path:      matches[2],
				Type:      "require",
				Language:  "javascript",
				Line:      lineNum,
				Confidence: 0.9,
			}
		}
	}
	
	return nil
}

// Python: import module or from module import ...
var pythonImportRegex = regexp.MustCompile(`(?:import\s+|from\s+)([\w.]+)`)

func (a *CodeAnalyzer) extractPythonImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "import") && !strings.HasPrefix(line, "from") {
		return nil
	}
	
	matches := pythonImportRegex.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return &ImportRelationship{
			Path:      matches[1],
			Type:      "import",
			Language:  "python",
			Line:      lineNum,
			Confidence: 0.9,
		}
	}
	
	return nil
}

// Rust: use path::to::module;
var rustUseRegex = regexp.MustCompile(`use\s+([\w:]+)`)

func (a *CodeAnalyzer) extractRustImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "use ") {
		return nil
	}
	
	matches := rustUseRegex.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return &ImportRelationship{
			Path:      matches[1],
			Type:      "use",
			Language:  "rust",
			Line:      lineNum,
			Confidence: 0.9,
		}
	}
	
	return nil
}

// Java: import package.Class;
var javaImportRegex = regexp.MustCompile(`import\s+(?:static\s+)?([\w.]+)`)

func (a *CodeAnalyzer) extractJavaImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "import") {
		return nil
	}
	
	matches := javaImportRegex.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return &ImportRelationship{
			Path:      matches[1],
			Type:      "import",
			Language:  "java",
			Line:      lineNum,
			Confidence: 0.9,
		}
	}
	
	return nil
}

// C/C++: #include "path" or #include <path>
var cIncludeRegex = regexp.MustCompile(`#include\s+(?:<|")([^>"]+)(?:>|")`)

func (a *CodeAnalyzer) extractCImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "#include") {
		return nil
	}
	
	matches := cIncludeRegex.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return &ImportRelationship{
			Path:      matches[1],
			Type:      "include",
			Language:  "c",
			Line:      lineNum,
			Confidence: 0.9,
		}
	}
	
	return nil
}

// Ruby: require "path" or require_relative "path"
var rubyRequireRegex = regexp.MustCompile(`require(?:_relative)?\s+(["'])([^"']+)(["'])`)

func (a *CodeAnalyzer) extractRubyImport(line string, lineNum int) *ImportRelationship {
	if !strings.HasPrefix(line, "require") {
		return nil
	}
	
	matches := rubyRequireRegex.FindStringSubmatch(line)
	// matches[0] = full match, matches[1] = opening quote, matches[2] = path, matches[3] = closing quote
	if len(matches) >= 4 && matches[1] == matches[3] {
		return &ImportRelationship{
			Path:      matches[2],
			Type:      "require",
			Language:  "ruby",
			Line:      lineNum,
			Confidence: 0.9,
		}
	}
	
	return nil
}

// PHP: require "path" or include "path" or use Namespace\Class;
var phpRequireRegex = regexp.MustCompile(`(?:require|include)(?:_once)?\s+(["'])([^"']+)(["'])`)
var phpUseRegex = regexp.MustCompile(`use\s+([\w\\]+)`)

func (a *CodeAnalyzer) extractPHPImport(line string, lineNum int) *ImportRelationship {
	if strings.HasPrefix(line, "require") || strings.HasPrefix(line, "include") {
		matches := phpRequireRegex.FindStringSubmatch(line)
		// matches[0] = full match, matches[1] = opening quote, matches[2] = path, matches[3] = closing quote
		if len(matches) >= 4 && matches[1] == matches[3] {
			return &ImportRelationship{
				Path:      matches[2],
				Type:      "require",
				Language:  "php",
				Line:      lineNum,
				Confidence: 0.9,
			}
		}
	}
	
	if strings.HasPrefix(line, "use ") {
		matches := phpUseRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			return &ImportRelationship{
				Path:      matches[1],
				Type:      "use",
				Language:  "php",
				Line:      lineNum,
				Confidence: 0.9,
			}
		}
	}
	
	return nil
}

// ResolveImportPath attempts to resolve an import path to a file path.
// This is a simplified implementation - full resolution would require
// understanding the project's module system and build configuration.
func (a *CodeAnalyzer) ResolveImportPath(importPath string, currentFile string, workspaceRoot string) string {
	// Remove file extension from import path if present
	importPath = strings.TrimSuffix(importPath, filepath.Ext(importPath))
	
	// Handle relative imports (starting with ./ or ../)
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		currentDir := filepath.Dir(currentFile)
		resolved := filepath.Join(currentDir, importPath)
		// Make relative to workspace root
		rel, err := filepath.Rel(workspaceRoot, resolved)
		if err == nil {
			return rel
		}
		return resolved
	}
	
	// For absolute imports, try common patterns
	// This is language-specific and would need enhancement
	return importPath
}

