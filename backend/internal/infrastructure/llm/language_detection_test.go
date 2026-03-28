package llm

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectSpanish tests the Spanish language detection function
func TestDetectSpanish(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Spanish content with common words",
			content:  "El libro contiene información sobre la historia de España y los acontecimientos más importantes que ocurrieron durante los últimos años.",
			expected: true,
		},
		{
			name:     "Spanish content with accented characters",
			content:  "Este documento trata sobre la educación y el aprendizaje. Contiene información relevante para estudiantes y profesores.",
			expected: true,
		},
		{
			name:     "Spanish content from 40 Conferencias",
			content:  "ÍNDICE\n\n1. EL LUNIK III SOVIÉTICO, EL ÚLTIMO ARGUMENTO DE LA EXISTENCIA DE DIOS\n\n2. LA CONQUISTA DEL ESPACIO LLEVA A DIOS\n\n(Conferencia pronunciada en el Cine Pax de Zaragoza)\n\n3. LA CIENCIA Y LA FE FRENTE A FRENTE",
			expected: true,
		},
		{
			name:     "Spanish content with religious terms",
			content:  "Este documento contiene una colección de conferencias sobre temas relacionados con la fe católica, la ciencia, la existencia de Dios, y aspectos históricos del cristianismo.",
			expected: true,
		},
		{
			name:     "English content",
			content:  "This document contains information about various topics. It includes details about the subject matter and provides comprehensive coverage.",
			expected: false,
		},
		{
			name:     "Mixed content (more Spanish)",
			content:  "El libro es muy interesante. It contains important information. Los capítulos están bien organizados.",
			expected: true,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "Short Spanish text",
			content:  "El libro es bueno.",
			expected: true, // Short but clearly Spanish with common words
		},
		{
			name:     "Spanish with ñ",
			content:  "El niño estudia español en la escuela. La mañana es muy bonita y el sol brilla con intensidad.",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectSpanish(tt.content)
			assert.Equal(t, tt.expected, result, "Content: %s", tt.content[:min(50, len(tt.content))])
		})
	}
}

// TestGenerateSummarySpanish tests that summaries are generated in Spanish for Spanish content
func TestGenerateSummarySpanish(t *testing.T) {
	// Create a mock LLM provider that returns Spanish summaries
	mockProvider := &mockSpanishLLMProvider{
		id:   "mock-spanish",
		name: "Mock Spanish LLM",
	}

	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))

	spanishContent := `ÍNDICE

1. EL LUNIK III SOVIÉTICO, EL ÚLTIMO ARGUMENTO DE LA EXISTENCIA DE DIOS

2. LA CONQUISTA DEL ESPACIO LLEVA A DIOS

(Conferencia pronunciada en el Cine Pax de Zaragoza)

3. LA CIENCIA Y LA FE FRENTE A FRENTE

(Conferencia pronunciada en el Salón de la Caja de Ahorros del Círculo Católico de Obreros de Burgos)

4. ATEÍSMO Y CIENCIA DE HOY

(Conferencia pronunciada en la Universidad de Deusto. Bilbao)

5. HISTORICIDAD DE LOS EVANGELIOS

(Conferencia pronunciada a matrimonios en Santa Cruz de Tenerife)

6. LA DIVINIDAD DE CRISTO

7. CRISTO EL MÁS GRANDE

8. LA AUTENTICIDAD DE LA SÁBANA SANTA DE TURÍN

Este documento contiene una colección de conferencias sobre temas relacionados con la fe católica, 
la ciencia, la existencia de Dios, y aspectos históricos del cristianismo. Las conferencias fueron 
pronunciadas en diferentes lugares de España y abordan temas como la relación entre ciencia y fe, 
la historicidad de los evangelios, y la divinidad de Cristo.`

	ctx := context.Background()
	summary, err := router.GenerateSummary(ctx, spanishContent, 200)
	require.NoError(t, err)
	require.NotEmpty(t, summary)

	// Verify the summary is in Spanish
	// Check for common Spanish words
	summaryLower := strings.ToLower(summary)
	spanishIndicators := []string{"el", "la", "los", "las", "de", "del", "que", "y", "en", "un", "una", "es", "son", "con", "por", "para", "como", "más", "pero", "sus"}
	
	hasSpanishWords := false
	for _, indicator := range spanishIndicators {
		if strings.Contains(summaryLower, indicator) {
			hasSpanishWords = true
			break
		}
	}

	// Check for Spanish characters
	hasSpanishChars := strings.ContainsAny(summary, "áéíóúñÁÉÍÓÚÑ")

	assert.True(t, hasSpanishWords || hasSpanishChars, 
		"Summary should contain Spanish words or characters. Summary: %s", summary)
	
	// Verify it's not just English
	englishIndicators := []string{"the", "and", "is", "are", "was", "were", "this", "that", "with", "for"}
	hasEnglishWords := false
	for _, indicator := range englishIndicators {
		if strings.Contains(summaryLower, " "+indicator+" ") {
			hasEnglishWords = true
			break
		}
	}

	// If we have Spanish indicators, we should not have primarily English
	if hasSpanishWords || hasSpanishChars {
		assert.False(t, hasEnglishWords && !hasSpanishWords, 
			"Summary should be in Spanish, not English. Summary: %s", summary)
	}
}

// TestGenerateSummaryWithContextSpanish tests Spanish summary generation with context
func TestGenerateSummaryWithContextSpanish(t *testing.T) {
	mockProvider := &mockSpanishLLMProvider{
		id:   "mock-spanish",
		name: "Mock Spanish LLM",
	}

	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))

	spanishContent := `Este libro trata sobre la historia de la Iglesia Católica y sus enseñanzas. 
Contiene información detallada sobre los dogmas de fe, los sacramentos, y la vida cristiana.
Los capítulos están organizados de manera temática, cubriendo desde los fundamentos de la fe 
hasta aspectos más avanzados de la teología.`

	contextSnippets := []string{
		"La fe católica se basa en las enseñanzas de Jesucristo.",
		"Los sacramentos son signos visibles de la gracia divina.",
	}

	ctx := context.Background()
	summary, err := router.GenerateSummaryWithContext(ctx, spanishContent, 200, contextSnippets)
	require.NoError(t, err)
	require.NotEmpty(t, summary)

	// Verify Spanish
	summaryLower := strings.ToLower(summary)
	hasSpanish := strings.ContainsAny(summary, "áéíóúñÁÉÍÓÚÑ") || 
		strings.Contains(summaryLower, " el ") || 
		strings.Contains(summaryLower, " la ") ||
		strings.Contains(summaryLower, " de ") ||
		strings.Contains(summaryLower, " y ")

	assert.True(t, hasSpanish, "Summary with context should be in Spanish. Summary: %s", summary)
}

// TestMultipleSpanishBooks tests language detection and summary generation for multiple Spanish books
func TestMultipleSpanishBooks(t *testing.T) {
	books := []struct {
		name    string
		content string
	}{
		{
			name: "40 Conferencias",
			content: `ÍNDICE

1. EL LUNIK III SOVIÉTICO, EL ÚLTIMO ARGUMENTO DE LA EXISTENCIA DE DIOS

2. LA CONQUISTA DEL ESPACIO LLEVA A DIOS

(Conferencia pronunciada en el Cine Pax de Zaragoza)

3. LA CIENCIA Y LA FE FRENTE A FRENTE

(Conferencia pronunciada en el Salón de la Caja de Ahorros del Círculo Católico de Obreros de Burgos)

4. ATEÍSMO Y CIENCIA DE HOY

(Conferencia pronunciada en la Universidad de Deusto. Bilbao)

5. HISTORICIDAD DE LOS EVANGELIOS

(Conferencia pronunciada a matrimonios en Santa Cruz de Tenerife)

6. LA DIVINIDAD DE CRISTO

7. CRISTO EL MÁS GRANDE

8. LA AUTENTICIDAD DE LA SÁBANA SANTA DE TURÍN

Este documento contiene una colección de conferencias sobre temas relacionados con la fe católica, 
la ciencia, la existencia de Dios, y aspectos históricos del cristianismo.`,
		},
		{
			name: "400 Respuestas",
			content: `Este libro contiene cuatrocientas respuestas a preguntas frecuentes sobre la fe católica.
Las respuestas están organizadas por temas, cubriendo aspectos doctrinales, morales, y prácticos
de la vida cristiana. Cada respuesta está fundamentada en las enseñanzas de la Iglesia Católica
y las Sagradas Escrituras.`,
		},
		{
			name: "Para Salvarte",
			content: `Para Salvarte es un libro de catequesis que explica los fundamentos de la fe católica
de manera clara y accesible. Está dirigido a personas que desean conocer mejor su fe o
prepararse para recibir los sacramentos. El libro cubre temas como la creación, la redención,
la Iglesia, los sacramentos, y la vida eterna.`,
		},
	}

	mockProvider := &mockSpanishLLMProvider{
		id:   "mock-spanish",
		name: "Mock Spanish LLM",
	}

	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))

	ctx := context.Background()

	for _, book := range books {
		t.Run(book.name, func(t *testing.T) {
			// Test language detection
			isSpanish := detectSpanish(book.content)
			assert.True(t, isSpanish, "Book '%s' should be detected as Spanish", book.name)

			// Test summary generation
			summary, err := router.GenerateSummary(ctx, book.content, 200)
			require.NoError(t, err, "Should generate summary for '%s'", book.name)
			require.NotEmpty(t, summary, "Summary should not be empty for '%s'", book.name)

			// Verify summary is in Spanish
			summaryLower := strings.ToLower(summary)
			hasSpanishWords := strings.Contains(summaryLower, " el ") ||
				strings.Contains(summaryLower, " la ") ||
				strings.Contains(summaryLower, " de ") ||
				strings.Contains(summaryLower, " y ") ||
				strings.Contains(summaryLower, " en ") ||
				strings.Contains(summaryLower, " un ") ||
				strings.Contains(summaryLower, " una ")

			hasSpanishChars := strings.ContainsAny(summary, "áéíóúñÁÉÍÓÚÑ")

			assert.True(t, hasSpanishWords || hasSpanishChars,
				"Summary for '%s' should be in Spanish. Summary: %s", book.name, summary)
		})
	}
}

// mockSpanishLLMProvider is a mock LLM provider that always returns Spanish summaries
type mockSpanishLLMProvider struct {
	id   string
	name string
}

func (m *mockSpanishLLMProvider) ID() string   { return m.id }
func (m *mockSpanishLLMProvider) Name() string  { return m.name }
func (m *mockSpanishLLMProvider) Type() string  { return "mock" }

func (m *mockSpanishLLMProvider) IsAvailable(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockSpanishLLMProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{Name: "mock-model", ContextLength: 4096},
	}, nil
}

func (m *mockSpanishLLMProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Return Spanish summary based on prompt
	text := ""
	promptLower := strings.ToLower(req.Prompt)

	switch {
	case strings.Contains(promptLower, "resume") || strings.Contains(promptLower, "resumen"):
		text = "Este documento contiene una colección de conferencias sobre temas relacionados con la fe católica, la ciencia, la existencia de Dios, y aspectos históricos del cristianismo. Las conferencias fueron pronunciadas en diferentes lugares de España y abordan temas como la relación entre ciencia y fe, la historicidad de los evangelios, y la divinidad de Cristo."
	case strings.Contains(promptLower, "términos clave") || strings.Contains(promptLower, "key terms"):
		text = "conferencias, catolicismo, fe, ciencia, dios, cristo, evangelios, teología, religión, iglesia"
	case strings.Contains(promptLower, "categoría") || strings.Contains(promptLower, "categoria"):
		text = "Religión y Teología"
	case strings.Contains(promptLower, "tags") || strings.Contains(promptLower, "etiquetas"):
		text = `["conferencias", "catolicismo", "fe", "ciencia"]`
	case strings.Contains(promptLower, "proyecto") || strings.Contains(promptLower, "project"):
		// Check if prompt is in Spanish (contains "sugiere" or "proyecto sugerido")
		if strings.Contains(promptLower, "sugiere") || strings.Contains(promptLower, "proyecto sugerido") {
			// Return Spanish project name
			if strings.Contains(promptLower, "purgatorio") || strings.Contains(promptLower, "almas") {
				text = "Almas del Purgatorio"
			} else if strings.Contains(promptLower, "conferencias") {
				text = "Conferencias Católicas"
			} else {
				text = "Religión y Teología"
			}
		} else {
			text = "Conferencias Católicas"
		}
	default:
		text = "Resumen en español del contenido proporcionado."
	}

	return &GenerateResponse{
		Text:       text,
		TokensUsed: len(text) / 4,
		Model:      req.Model,
	}, nil
}

func (m *mockSpanishLLMProvider) StreamGenerate(ctx context.Context, req GenerateRequest) (<-chan GenerateChunk, error) {
	ch := make(chan GenerateChunk, 1)
	resp, _ := m.Generate(ctx, req)
	ch <- GenerateChunk{
		Text:  resp.Text,
		Done:  true,
		Error: nil,
	}
	close(ch)
	return ch, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

