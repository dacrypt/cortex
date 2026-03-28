// Package stages provides pipeline processing stages.
package stages

import (
	"bufio"
	"context"
	"os"
	"strings"
	"unicode"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
)

// CodeStage analyzes source code files.
type CodeStage struct {
	codeExtensions map[string]bool
}

// NewCodeStage creates a new code analysis stage.
func NewCodeStage() *CodeStage {
	return &CodeStage{
		codeExtensions: map[string]bool{
			".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
			".py": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
			".h": true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
			".swift": true, ".kt": true, ".scala": true, ".sh": true, ".bash": true,
			".sql": true, ".r": true, ".lua": true, ".pl": true, ".pm": true,
			".ex": true, ".exs": true, ".erl": true, ".hs": true, ".ml": true,
			".fs": true, ".dart": true, ".v": true, ".zig": true, ".nim": true,
			".vue": true, ".svelte": true, ".html": true, ".css": true, ".scss": true,
			".sass": true, ".less": true, ".xml": true, ".json": true, ".yaml": true,
			".yml": true, ".toml": true, ".md": true, ".markdown": true,
		},
	}
}

// Name returns the stage name.
func (s *CodeStage) Name() string {
	return "code"
}

// CanProcess returns true if this stage can process the file.
func (s *CodeStage) CanProcess(entry *entity.FileEntry) bool {
	return s.codeExtensions[entry.Extension]
}

// Process analyzes the source code file.
func (s *CodeStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	if !s.CanProcess(entry) {
		return nil
	}

	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}

	file, err := os.Open(entry.AbsolutePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read file content for import extraction
	contentBytes, err := os.ReadFile(entry.AbsolutePath)
	if err != nil {
		return err
	}
	content := string(contentBytes)

	// Extract import relationships
	codeAnalyzer := metadata.NewCodeAnalyzer()
	imports := codeAnalyzer.ExtractImports(content, entry.Extension)
	
	// Convert to CodeImport entities
	codeImports := make([]entity.CodeImport, 0, len(imports))
	for _, imp := range imports {
		codeImports = append(codeImports, entity.CodeImport{
			Path:      imp.Path,
			Type:      imp.Type,
			Language:  imp.Language,
			Line:      imp.Line,
			Confidence: imp.Confidence,
		})
	}
	entry.Enhanced.CodeImports = codeImports

	stats := &CodeStats{}
	scanner := bufio.NewScanner(strings.NewReader(content))

	// Get comment style for this language
	lineComment, blockStart, blockEnd := s.getCommentStyle(entry.Extension)
	inBlockComment := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		stats.Lines++

		trimmed := strings.TrimSpace(line)

		// Empty line
		if trimmed == "" {
			stats.BlankLines++
			continue
		}

		// Handle block comments
		if blockStart != "" && blockEnd != "" {
			if inBlockComment {
				stats.CommentLines++
				if strings.Contains(trimmed, blockEnd) {
					inBlockComment = false
				}
				continue
			}

			if strings.HasPrefix(trimmed, blockStart) {
				inBlockComment = true
				stats.CommentLines++
				if strings.Contains(trimmed, blockEnd) && !strings.HasSuffix(trimmed, blockStart) {
					inBlockComment = false
				}
				continue
			}
		}

		// Single-line comment
		if lineComment != "" && strings.HasPrefix(trimmed, lineComment) {
			stats.CommentLines++
			// Check for special markers
			s.checkMarkers(trimmed, stats)
			continue
		}

		// Code line
		stats.CodeLines++

		// Check for function/method definitions
		s.checkDefinitions(line, entry.Extension, stats)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Store in enhanced metadata
	commentPercentage := 0.0
	if stats.Lines > 0 {
		commentPercentage = float64(stats.CommentLines) / float64(stats.Lines) * 100
	}

	entry.Enhanced.CodeMetrics = &entity.CodeMetrics{
		LinesOfCode:       stats.CodeLines,
		CommentLines:      stats.CommentLines,
		BlankLines:        stats.BlankLines,
		CommentPercentage: commentPercentage,
		FunctionCount:     stats.Functions,
		ClassCount:        stats.Classes,
	}

	// Mark as code-indexed
	entry.Enhanced.IndexedState.Code = true

	return nil
}

// CodeStats holds code analysis statistics.
type CodeStats struct {
	Lines        int
	CodeLines    int
	CommentLines int
	BlankLines   int
	Functions    int
	Classes      int
	TODOs        int
	FIXMEs       int
}

// getCommentStyle returns comment syntax for a language.
func (s *CodeStage) getCommentStyle(ext string) (lineComment, blockStart, blockEnd string) {
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".swift", ".kt", ".scala", ".rs", ".dart", ".v", ".zig":
		return "//", "/*", "*/"
	case ".py", ".rb", ".sh", ".bash", ".zsh", ".pl", ".pm", ".r", ".yaml", ".yml":
		return "#", "", ""
	case ".lua":
		return "--", "--[[", "]]"
	case ".html", ".xml", ".vue", ".svelte":
		return "", "<!--", "-->"
	case ".css", ".scss", ".sass", ".less":
		return "", "/*", "*/"
	case ".sql":
		return "--", "/*", "*/"
	case ".hs":
		return "--", "{-", "-}"
	case ".ml", ".mli":
		return "", "(*", "*)"
	case ".ex", ".exs":
		return "#", "", ""
	case ".erl":
		return "%", "", ""
	case ".clj", ".cljs":
		return ";", "", ""
	case ".nim":
		return "#", "#[", "]#"
	default:
		return "//", "/*", "*/"
	}
}

// checkMarkers looks for TODO, FIXME, etc in comments.
func (s *CodeStage) checkMarkers(line string, stats *CodeStats) {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "TODO") {
		stats.TODOs++
	}
	if strings.Contains(upper, "FIXME") || strings.Contains(upper, "FIX ME") {
		stats.FIXMEs++
	}
}

// checkDefinitions looks for function/class definitions.
func (s *CodeStage) checkDefinitions(line, ext string, stats *CodeStats) {
	trimmed := strings.TrimSpace(line)

	switch ext {
	case ".go":
		if strings.HasPrefix(trimmed, "func ") {
			stats.Functions++
		}
	case ".ts", ".tsx", ".js", ".jsx":
		if s.isJSFunction(trimmed) {
			stats.Functions++
		}
		if strings.HasPrefix(trimmed, "class ") {
			stats.Classes++
		}
	case ".py":
		if strings.HasPrefix(trimmed, "def ") {
			stats.Functions++
		}
		if strings.HasPrefix(trimmed, "class ") {
			stats.Classes++
		}
	case ".java", ".kt", ".scala", ".cs":
		if s.hasMethodSignature(trimmed) {
			stats.Functions++
		}
		if strings.Contains(trimmed, "class ") {
			stats.Classes++
		}
	case ".rs":
		if strings.HasPrefix(trimmed, "fn ") || strings.HasPrefix(trimmed, "pub fn ") {
			stats.Functions++
		}
		if strings.Contains(trimmed, "struct ") || strings.Contains(trimmed, "impl ") {
			stats.Classes++
		}
	case ".rb":
		if strings.HasPrefix(trimmed, "def ") {
			stats.Functions++
		}
		if strings.HasPrefix(trimmed, "class ") {
			stats.Classes++
		}
	case ".swift":
		if strings.HasPrefix(trimmed, "func ") {
			stats.Functions++
		}
		if strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "struct ") {
			stats.Classes++
		}
	}
}

// isJSFunction checks if line defines a JS/TS function.
func (s *CodeStage) isJSFunction(line string) bool {
	// function keyword
	if strings.HasPrefix(line, "function ") {
		return true
	}

	// Arrow function or method (simplified check)
	if strings.Contains(line, "=>") && strings.Contains(line, "(") {
		return true
	}

	// Method definition
	if s.hasMethodSignature(line) && !strings.HasPrefix(line, "if") &&
		!strings.HasPrefix(line, "for") && !strings.HasPrefix(line, "while") {
		return true
	}

	return false
}

// hasMethodSignature checks for method-like patterns.
func (s *CodeStage) hasMethodSignature(line string) bool {
	// Look for pattern: word(args) {
	parenIdx := strings.Index(line, "(")
	if parenIdx <= 0 {
		return false
	}

	// Check that characters before ( are a valid identifier
	before := strings.TrimSpace(line[:parenIdx])
	words := strings.Fields(before)
	if len(words) == 0 {
		return false
	}

	lastWord := words[len(words)-1]
	if lastWord == "" || !isValidIdentifier(lastWord) {
		return false
	}

	return true
}

// isValidIdentifier checks if s is a valid identifier.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}

	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return true
}
