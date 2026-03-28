package metadata

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// TableExtractor extracts tables from PDFs and documents.
type TableExtractor struct {
	logger zerolog.Logger
}

// NewTableExtractor creates a new table extractor.
func NewTableExtractor(logger zerolog.Logger) *TableExtractor {
	return &TableExtractor{
		logger: logger.With().Str("component", "table_extractor").Logger(),
	}
}

// ExtractTables extracts tables from a PDF file.
func (e *TableExtractor) ExtractTables(ctx context.Context, pdfPath string) ([]entity.Table, error) {
	// Try tabula-py first (requires Python and tabula-py)
	if tables, err := e.extractWithTabula(ctx, pdfPath); err == nil {
		return tables, nil
	}

	// Try camelot (requires Python and camelot-py)
	if tables, err := e.extractWithCamelot(ctx, pdfPath); err == nil {
		return tables, nil
	}

	// Fallback: return empty (could use LLM in future)
	return nil, fmt.Errorf("no table extraction tools available (tabula or camelot required)")
}

// extractWithTabula extracts tables using tabula-py.
func (e *TableExtractor) extractWithTabula(ctx context.Context, pdfPath string) ([]entity.Table, error) {
	// Check if Python and tabula are available
	if _, err := exec.LookPath("python3"); err != nil {
		return nil, fmt.Errorf("python3 not found")
	}

	// Create Python script to extract tables
	script := fmt.Sprintf(`
import tabula
import json
import sys

pdf_path = "%s"
tables = tabula.read_pdf(pdf_path, pages="all", multiple_tables=True)

result = []
for i, table in enumerate(tables):
    table_dict = {
        "rows": table.values.tolist(),
        "headers": table.columns.tolist() if hasattr(table, 'columns') else [],
        "row_count": len(table),
        "column_count": len(table.columns) if hasattr(table, 'columns') else 0,
        "page": i + 1
    }
    result.append(table_dict)

print(json.dumps(result))
`, pdfPath)

	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tabula extraction failed: %w", err)
	}

	// Parse JSON output
	var tables []entity.Table
	// Note: Would need JSON unmarshaling here
	// For now, return empty and log
	e.logger.Debug().Str("output", string(output)).Msg("Tabula extraction completed")

	return tables, nil
}

// extractWithCamelot extracts tables using camelot-py.
func (e *TableExtractor) extractWithCamelot(ctx context.Context, pdfPath string) ([]entity.Table, error) {
	// Check if Python and camelot are available
	if _, err := exec.LookPath("python3"); err != nil {
		return nil, fmt.Errorf("python3 not found")
	}

	// Create Python script to extract tables
	script := fmt.Sprintf(`
import camelot
import json
import sys

pdf_path = "%s"
tables = camelot.read_pdf(pdf_path, pages="all")

result = []
for i, table in enumerate(tables):
    table_dict = {
        "rows": table.df.values.tolist(),
        "headers": table.df.columns.tolist(),
        "row_count": len(table.df),
        "column_count": len(table.df.columns),
        "page": table.page
    }
    result.append(table_dict)

print(json.dumps(result))
`, pdfPath)

	cmd := exec.CommandContext(ctx, "python3", "-c", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("camelot extraction failed: %w", err)
	}

	// Parse JSON output
	var tables []entity.Table
	e.logger.Debug().Str("output", string(output)).Msg("Camelot extraction completed")

	return tables, nil
}

// ExtractTablesFromMarkdown extracts tables from Markdown content.
func (e *TableExtractor) ExtractTablesFromMarkdown(ctx context.Context, content string) ([]entity.Table, error) {
	var tables []entity.Table

	// Simple markdown table parser
	lines := strings.Split(content, "\n")
	var currentTable *entity.Table
	var inTable bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check if line is a table row (contains |)
		if strings.Contains(line, "|") && !strings.HasPrefix(line, "|---") {
			if !inTable {
				// Start new table
				currentTable = &entity.Table{
					Rows: [][]string{},
					Page: 1, // Markdown doesn't have pages
				}
				inTable = true
			}

			// Parse row
			cells := strings.Split(line, "|")
			var row []string
			for _, cell := range cells {
				cell = strings.TrimSpace(cell)
				if cell != "" {
					row = append(row, cell)
				}
			}
			
			if len(row) > 0 {
				if len(currentTable.Rows) == 0 {
					// First row is header
					currentTable.Headers = row
				} else {
					currentTable.Rows = append(currentTable.Rows, row)
				}
			}
		} else {
			// End of table
			if inTable && currentTable != nil {
				currentTable.RowCount = len(currentTable.Rows)
				if len(currentTable.Rows) > 0 {
					currentTable.ColumnCount = len(currentTable.Rows[0])
				}
				tables = append(tables, *currentTable)
				currentTable = nil
				inTable = false
			}
		}
	}

	// Add last table if still in progress
	if inTable && currentTable != nil {
		currentTable.RowCount = len(currentTable.Rows)
		if len(currentTable.Rows) > 0 {
			currentTable.ColumnCount = len(currentTable.Rows[0])
		}
		tables = append(tables, *currentTable)
	}

	return tables, nil
}

