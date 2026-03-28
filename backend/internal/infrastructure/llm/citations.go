package llm

import (
	"context"
	"fmt"
	"regexp"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ExtractCitations extracts bibliographic citations from content using LLM.
func (r *Router) ExtractCitations(ctx context.Context, content string, summary string) ([]Citation, error) {
	// Use summary if available, otherwise use truncated content
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf("Resumen del documento:\n%s\n", summary)
	} else {
		contentSection = fmt.Sprintf("Contenido (primeras 3000 palabras):\n%s\n", truncateContent(content, 3000))
	}

	// Detect language
	isSpanish := detectRomanceLanguage(content + summary)

	var prompt string
	if isSpanish {
		prompt = fmt.Sprintf(`Eres un experto bibliotecario. Analiza el siguiente documento y extrae TODAS las citas bibliográficas en formato JSON.

INSTRUCCIONES:
1. Identifica todas las citas bibliográficas (libros, artículos, sitios web, etc.)
2. Extrae autores, títulos, años, DOI, URLs cuando estén disponibles
3. Identifica el tipo de referencia (libro, artículo, sitio web, conferencia, etc.)
4. Incluye el contexto donde aparece la cita

FORMATO DE RESPUESTA (JSON estricto):
{
  "citations": [
    {
      "text": "Texto completo de la cita",
      "authors": ["Autor 1", "Autor 2"],
      "title": "Título de la obra",
      "year": 2024,
      "doi": "10.1234/example",
      "url": "https://example.com",
      "type": "book|article|website|conference|thesis|other",
      "context": "Texto alrededor de la cita",
      "confidence": 0.95
    }
  ]
}

TIPOS DE REFERENCIAS:
- book: Libros
- article: Artículos científicos
- website: Sitios web
- conference: Conferencias, congresos
- thesis: Tesis, disertaciones
- report: Informes técnicos
- other: Otros tipos

IMPORTANTE:
- Responde SOLO con JSON válido, sin texto adicional
- Extrae TODAS las citas, incluso las incompletas
- Si falta información, usa null o array vacío
- Sé preciso con años y autores%s

Citas bibliográficas (JSON):`, contentSection)
	} else {
		prompt = fmt.Sprintf(`You are an expert librarian. Analyze the following document and extract ALL bibliographic citations in JSON format.

INSTRUCTIONS:
1. Identify all bibliographic citations (books, articles, websites, etc.)
2. Extract authors, titles, years, DOI, URLs when available
3. Identify the reference type (book, article, website, conference, etc.)
4. Include context where citation appears

RESPONSE FORMAT (strict JSON):
{
  "citations": [
    {
      "text": "Full citation text",
      "authors": ["Author 1", "Author 2"],
      "title": "Work title",
      "year": 2024,
      "doi": "10.1234/example",
      "url": "https://example.com",
      "type": "book|article|website|conference|thesis|other",
      "context": "Surrounding text",
      "confidence": 0.95
    }
  ]
}

REFERENCE TYPES:
- book: Books
- article: Scientific articles
- website: Websites
- conference: Conferences, congresses
- thesis: Theses, dissertations
- report: Technical reports
- other: Other types

IMPORTANT:
- Respond ONLY with valid JSON, no additional text
- Extract ALL citations, even incomplete ones
- If information is missing, use null or empty array
- Be precise with years and authors%s

Bibliographic citations (JSON):`, contentSection)
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   4000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response using robust parser
	var result struct {
		Citations []Citation `json:"citations"`
	}

	parser := NewJSONParser(r.logger)
	if err := parser.ParseJSON(ctx, resp.Text, &result); err != nil {
		// Try fallback parsing
		return parseCitationsFallback(resp.Text), nil
	}

	return result.Citations, nil
}

// Citation is an alias for entity.Citation for backward compatibility.
// Use entity.Citation directly.
type Citation = entity.Citation

// parseCitationsFallback attempts to extract citations from text if JSON parsing fails.
func parseCitationsFallback(text string) []Citation {
	var citations []Citation
	
	// Look for common citation patterns
	// Pattern: Author (Year) or Author, Year
	authorYearPattern := regexp.MustCompile(`([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)\s*\((\d{4})\)`)
	matches := authorYearPattern.FindAllStringSubmatch(text, -1)
	
	for _, match := range matches {
		if len(match) >= 3 {
			year := parseInt(match[2])
			citations = append(citations, Citation{
				Text:       match[0],
				Authors:    []string{match[1]},
				Year:       &year,
				Type:       "other",
				Confidence: 0.6,
			})
		}
	}
	
	return citations
}

func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

