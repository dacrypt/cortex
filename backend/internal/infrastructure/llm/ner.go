package llm

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ExtractNamedEntities extracts named entities from content using LLM.
func (r *Router) ExtractNamedEntities(ctx context.Context, content string, summary string) ([]NamedEntity, error) {
	// Use summary if available, otherwise use truncated content
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf("Resumen del documento:\n%s\n", summary)
	} else {
		contentSection = fmt.Sprintf("Contenido (primeras 2000 palabras):\n%s\n", truncateContent(content, 2000))
	}

	// Detect language
	isSpanish := detectRomanceLanguage(content + summary)

	var prompt string
	if isSpanish {
		prompt = fmt.Sprintf(`Eres un experto en reconocimiento de entidades nombradas (NER). Analiza el siguiente documento y extrae TODAS las entidades nombradas en formato JSON.

INSTRUCCIONES:
1. Identifica personas mencionadas (nombres completos)
2. Identifica lugares (ciudades, países, regiones, continentes)
3. Identifica organizaciones (iglesias, universidades, gobiernos, empresas)
4. Identifica fechas (años, fechas específicas, períodos)
5. Identifica cantidades monetarias
6. Identifica porcentajes
7. Identifica otros conceptos importantes

FORMATO DE RESPUESTA (JSON estricto):
{
  "entities": [
    {
      "text": "Nombre completo de la entidad",
      "type": "PERSON|LOCATION|ORGANIZATION|DATE|MONEY|PERCENT|EVENT|WORK_OF_ART|LAW|LANGUAGE|OTHER",
      "start_pos": 0,
      "end_pos": 10,
      "confidence": 0.95,
      "context": "Texto alrededor de la entidad (opcional)"
    }
  ]
}

TIPOS DE ENTIDADES:
- PERSON: Nombres de personas
- LOCATION: Lugares geográficos
- ORGANIZATION: Organizaciones, instituciones
- DATE: Fechas, años, períodos temporales
- MONEY: Cantidades monetarias
- PERCENT: Porcentajes
- EVENT: Eventos históricos
- WORK_OF_ART: Obras de arte, libros, películas
- LAW: Leyes, decretos
- LANGUAGE: Idiomas
- OTHER: Otros conceptos importantes

IMPORTANTE:
- Responde SOLO con JSON válido, sin texto adicional
- Extrae TODAS las entidades, no solo las principales
- Sé preciso con los tipos
- Incluye contexto cuando sea relevante%s

Información contextual (JSON):`, contentSection)
	} else {
		prompt = fmt.Sprintf(`You are an expert in Named Entity Recognition (NER). Analyze the following document and extract ALL named entities in JSON format.

INSTRUCTIONS:
1. Identify people mentioned (full names)
2. Identify locations (cities, countries, regions, continents)
3. Identify organizations (churches, universities, governments, companies)
4. Identify dates (years, specific dates, periods)
5. Identify monetary amounts
6. Identify percentages
7. Identify other important concepts

RESPONSE FORMAT (strict JSON):
{
  "entities": [
    {
      "text": "Full name of entity",
      "type": "PERSON|LOCATION|ORGANIZATION|DATE|MONEY|PERCENT|EVENT|WORK_OF_ART|LAW|LANGUAGE|OTHER",
      "start_pos": 0,
      "end_pos": 10,
      "confidence": 0.95,
      "context": "Surrounding text (optional)"
    }
  ]
}

ENTITY TYPES:
- PERSON: Person names
- LOCATION: Geographic places
- ORGANIZATION: Organizations, institutions
- DATE: Dates, years, time periods
- MONEY: Monetary amounts
- PERCENT: Percentages
- EVENT: Historical events
- WORK_OF_ART: Artworks, books, movies
- LAW: Laws, decrees
- LANGUAGE: Languages
- OTHER: Other important concepts

IMPORTANT:
- Respond ONLY with valid JSON, no additional text
- Extract ALL entities, not just main ones
- Be precise with types
- Include context when relevant%s

Contextual information (JSON):`, contentSection)
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   3000,
		Temperature: 0.2, // Low temperature for structured output
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result struct {
		Entities []NamedEntity `json:"entities"`
	}

	// Use robust JSON parser with retry capability
	parser := NewJSONParser(r.logger)
	if err := parser.ParseJSON(ctx, resp.Text, &result); err != nil {
		return nil, fmt.Errorf("failed to parse NER response: %w", err)
	}

	return result.Entities, nil
}

// NamedEntity is an alias for entity.NamedEntity for backward compatibility.
// Use entity.NamedEntity directly.
type NamedEntity = entity.NamedEntity

