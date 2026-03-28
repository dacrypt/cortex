package metadata

import (
	"context"
	"regexp"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// FormulaExtractor extracts mathematical formulas from documents.
type FormulaExtractor struct {
	logger zerolog.Logger
}

// NewFormulaExtractor creates a new formula extractor.
func NewFormulaExtractor(logger zerolog.Logger) *FormulaExtractor {
	return &FormulaExtractor{
		logger: logger.With().Str("component", "formula_extractor").Logger(),
	}
}

// ExtractFormulas extracts mathematical formulas from content.
func (e *FormulaExtractor) ExtractFormulas(ctx context.Context, content string) ([]entity.Formula, error) {
	var formulas []entity.Formula

	// Pattern 1: LaTeX inline formulas: $...$ or \(...\)
	latexInlinePattern := regexp.MustCompile(`\$([^$]+)\$|\\\(([^)]+)\\\)`)
	matches := latexInlinePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		formula := match[1]
		if formula == "" {
			formula = match[2]
		}
		if formula != "" {
			formulas = append(formulas, entity.Formula{
				Text: formula,
				LaTeX: formula,
				Type: "expression",
			})
		}
	}

	// Pattern 2: LaTeX display formulas: $$...$$ or \[...\]
	latexDisplayPattern := regexp.MustCompile(`\$\$([^$]+)\$\$|\\\[([^\]]+)\\\]`)
	matches = latexDisplayPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		formula := match[1]
		if formula == "" {
			formula = match[2]
		}
		if formula != "" {
			formulas = append(formulas, entity.Formula{
				Text: formula,
				LaTeX: formula,
				Type: "equation",
			})
		}
	}

	// Pattern 3: MathML: <math>...</math>
	mathmlPattern := regexp.MustCompile(`<math[^>]*>([^<]+)</math>`)
	matches = mathmlPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if match[1] != "" {
			formulas = append(formulas, entity.Formula{
				Text: match[1],
				Type: "mathml",
			})
		}
	}

	// Pattern 4: Common mathematical expressions (simple patterns)
	// E.g., "x = y + z", "f(x) = ...", etc.
	simpleFormulaPattern := regexp.MustCompile(`\b([a-zA-Z]\s*[=<>≤≥≠]\s*[^,;.]+)`)
	matches = simpleFormulaPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		formula := strings.TrimSpace(match[1])
		// Filter out common false positives
		if !isFalsePositive(formula) {
			formulas = append(formulas, entity.Formula{
				Text: formula,
				Type: "expression",
			})
		}
	}

	return formulas, nil
}

// isFalsePositive checks if a formula candidate is likely a false positive.
func isFalsePositive(text string) bool {
	// Common false positives
	falsePositives := []string{
		"ISBN", "ISSN", "DOI", "URL", "HTTP", "HTTPS",
		"PDF", "XML", "JSON", "HTML", "CSS",
		"API", "CPU", "GPU", "RAM", "ROM",
	}
	
	upper := strings.ToUpper(text)
	for _, fp := range falsePositives {
		if strings.Contains(upper, fp) {
			return true
		}
	}
	
	return false
}






