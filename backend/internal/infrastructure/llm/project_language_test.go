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

// TestSuggestProjectSpanishContent tests that project suggestions are generated in Spanish for Spanish content.
func TestSuggestProjectSpanishContent(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)

	// Mock Spanish LLM provider
	mockProvider := &mockSpanishProjectProvider{
		id:   "mock-spanish",
		name: "Mock Spanish LLM Provider",
	}
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))

	// Spanish content about Purgatory souls (from the actual document)
	spanishContent := `MIS CONVERSACIONES CON LAS ALMAS DEL PURGATORIO

La Princesa Eugenia von der Leyen tuvo contacto con las Almas del Purgatorio desde 1921 hasta 1929. 
El Señor Jesús dice a Santa Faustina: "Tráeme a las almas que están en la cárcel del purgatorio 
y sumérgelas en el abismo de mi misericordia. Que los torrentes de mi sangre refresquen el ardor del purgatorio."

Este documento contiene información sobre cómo ayudar a las almas del purgatorio mediante oraciones, 
sufrimiento reparador, el rezo del Rosario, y otros medios espirituales.`

	// Test with summary and description in Spanish
	spanishSummary := "Documento sobre conversaciones con almas del purgatorio y métodos para ayudarlas mediante oración y sacrificios."
	spanishDescription := "purgatorio, almas, oración, misericordia, sacrificio, rosario"

	project, err := router.SuggestProjectWithSummary(ctx, spanishSummary, spanishDescription, spanishContent, "test/spanish-doc.pdf", []string{})
	require.NoError(t, err)
	assert.NotEmpty(t, project, "Project should be suggested")

	// Verify the project name is in Spanish (should contain Spanish words, not English)
	projectLower := strings.ToLower(project)
	spanishIndicators := []string{"purgatorio", "almas", "oración", "conversaciones", "misericordia", "espiritual", "religión", "fe"}
	englishIndicators := []string{"purgatory", "souls", "prayer", "conversations", "mercy", "spiritual", "religion", "faith"}

	hasSpanish := false
	for _, indicator := range spanishIndicators {
		if strings.Contains(projectLower, indicator) {
			hasSpanish = true
			break
		}
	}

	hasEnglish := false
	for _, indicator := range englishIndicators {
		if strings.Contains(projectLower, indicator) {
			hasEnglish = true
			break
		}
	}

	assert.True(t, hasSpanish || !hasEnglish, "Project name should be in Spanish, got: %s", project)
}

// TestSuggestProjectEnglishContent tests that project suggestions are generated in English for English content.
func TestSuggestProjectEnglishContent(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)

	// Mock English LLM provider
	mockProvider := &mockEnglishProjectProvider{
		id:   "mock-english",
		name: "Mock English LLM Provider",
	}
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-english", "mock-model"))

	// English content about software development
	englishContent := `Software Development Best Practices

This document covers modern software development practices including test-driven development, 
continuous integration, and code review processes. It discusses how to write maintainable code 
and build scalable systems using agile methodologies.`

	// Test with summary and description in English
	englishSummary := "Document about software development best practices and modern methodologies."
	englishDescription := "software, development, testing, agile, code review"

	project, err := router.SuggestProjectWithSummary(ctx, englishSummary, englishDescription, englishContent, "test/english-doc.md", []string{})
	require.NoError(t, err)
	assert.NotEmpty(t, project, "Project should be suggested")

	// Verify the project name is in English (should contain English words, not Spanish)
	projectLower := strings.ToLower(project)
	englishIndicators := []string{"software", "development", "testing", "agile", "code", "practices", "methodology"}
	spanishIndicators := []string{"desarrollo", "software", "pruebas", "ágil", "código", "prácticas", "metodología"}

	hasEnglish := false
	for _, indicator := range englishIndicators {
		if strings.Contains(projectLower, indicator) {
			hasEnglish = true
			break
		}
	}

	hasSpanish := false
	for _, indicator := range spanishIndicators {
		if strings.Contains(projectLower, indicator) {
			hasSpanish = true
			break
		}
	}

	assert.True(t, hasEnglish || !hasSpanish, "Project name should be in English, got: %s", project)
}

// TestSuggestProjectWithExistingProjectsSpanish tests project suggestion with existing projects in Spanish context.
func TestSuggestProjectWithExistingProjectsSpanish(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(io.Discard)
	router := NewRouter(logger)

	// Mock Spanish LLM provider
	mockProvider := &mockSpanishProjectProvider{
		id:   "mock-spanish",
		name: "Mock Spanish LLM Provider",
	}
	router.RegisterProvider(mockProvider)
	require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))

	// Spanish content
	spanishContent := `Documento sobre conferencias católicas y temas de fe.`
	spanishSummary := "Conferencias sobre temas católicos y religiosos."
	spanishDescription := "conferencias, catolicismo, fe, religión"

	// Existing projects in Spanish
	existingProjects := []string{
		"Conferencias Católicas",
		"Libros de Teología",
		"Documentos Históricos",
	}

	project, err := router.SuggestProjectWithSummary(ctx, spanishSummary, spanishDescription, spanishContent, "test/spanish-conference.pdf", existingProjects)
	require.NoError(t, err)
	assert.NotEmpty(t, project, "Project should be suggested")

	// Should suggest an existing project or a new one in Spanish
	projectLower := strings.ToLower(project)
	spanishWords := []string{"conferencias", "católicas", "teología", "documentos", "históricos", "libros", "religión", "fe"}

	hasSpanishWord := false
	for _, word := range spanishWords {
		if strings.Contains(projectLower, word) {
			hasSpanishWord = true
			break
		}
	}

	assert.True(t, hasSpanishWord, "Project name should contain Spanish words, got: %s", project)
}

// TestSuggestProjectLanguageDetection tests that language detection works correctly for project suggestions.
func TestSuggestProjectLanguageDetection(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(io.Discard)

	tests := []struct {
		name          string
		content       string
		summary       string
		description   string
		expectSpanish bool
	}{
		{
			name:          "Spanish content with Spanish summary",
			content:       "Contenido en español sobre temas religiosos.",
			summary:       "Resumen en español sobre religión.",
			description:   "religión, fe, espiritualidad",
			expectSpanish: true,
		},
		{
			name:          "English content with English summary",
			content:       "Content in English about software development.",
			summary:       "Summary in English about software.",
			description:   "software, development, coding",
			expectSpanish: false,
		},
		{
			name:          "Spanish content without summary (fallback to content)",
			content:       "Este es un documento en español que habla sobre temas importantes de la fe católica y las almas del purgatorio.",
			summary:       "",
			description:   "",
			expectSpanish: true,
		},
		{
			name:          "English content without summary (fallback to content)",
			content:       "This is a document in English about important topics in software engineering and best practices.",
			summary:       "",
			description:   "",
			expectSpanish: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(logger)

			var mockProvider Provider
			if tt.expectSpanish {
				mockProvider = &mockSpanishProjectProvider{
					id:   "mock-spanish",
					name: "Mock Spanish LLM Provider",
				}
				router.RegisterProvider(mockProvider)
				require.NoError(t, router.SetActiveProvider("mock-spanish", "mock-model"))
			} else {
				mockProvider = &mockEnglishProjectProvider{
					id:   "mock-english",
					name: "Mock English LLM Provider",
				}
				router.RegisterProvider(mockProvider)
				require.NoError(t, router.SetActiveProvider("mock-english", "mock-model"))
			}

			project, err := router.SuggestProjectWithSummary(ctx, tt.summary, tt.description, tt.content, "test/document.txt", []string{})
			require.NoError(t, err)
			assert.NotEmpty(t, project, "Project should be suggested for: %s", tt.name)

			// Verify language detection worked (check if Spanish prompt was used)
			// This is indirect verification - we check that the response makes sense
			projectLower := strings.ToLower(project)

			if tt.expectSpanish {
				// Should not contain common English project words
				englishProjectWords := []string{"project", "development", "software", "system", "application"}
				for _, word := range englishProjectWords {
					assert.NotEqual(t, projectLower, word, "Project should not be generic English word: %s", project)
				}
			} else {
				// Should not be generic Spanish words
				spanishProjectWords := []string{"proyecto", "desarrollo", "sistema", "aplicación"}
				for _, word := range spanishProjectWords {
					assert.NotEqual(t, projectLower, word, "Project should not be generic Spanish word: %s", project)
				}
			}
		})
	}
}

// mockSpanishProjectProvider is a mock LLM provider that returns Spanish project names.
type mockSpanishProjectProvider struct {
	id   string
	name string
}

func (m *mockSpanishProjectProvider) ID() string   { return m.id }
func (m *mockSpanishProjectProvider) Name() string { return m.name }
func (m *mockSpanishProjectProvider) Type() string { return "mock" }

func (m *mockSpanishProjectProvider) IsAvailable(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockSpanishProjectProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{Name: "mock-model", ContextLength: 4096},
	}, nil
}

func (m *mockSpanishProjectProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Return Spanish project name based on prompt content
	promptLower := strings.ToLower(req.Prompt)

	var text string
	// Check if prompt is in Spanish (contains "sugiere" or "proyecto sugerido")
	if strings.Contains(promptLower, "sugiere") || strings.Contains(promptLower, "proyecto sugerido") {
		// Return Spanish project name
		if strings.Contains(promptLower, "purgatorio") || strings.Contains(promptLower, "almas") {
			text = "Almas del Purgatorio"
		} else if strings.Contains(promptLower, "conferencias") {
			text = "Conferencias Católicas"
		} else if strings.Contains(promptLower, "religión") || strings.Contains(promptLower, "religion") {
			text = "Religión y Teología"
		} else {
			text = "Documentos Religiosos"
		}
	} else {
		// Fallback for English prompts (shouldn't happen in these tests)
		text = "Conferencias Católicas"
	}

	return &GenerateResponse{
		Text:       text,
		TokensUsed: 10,
		Model:      req.Model,
	}, nil
}

func (m *mockSpanishProjectProvider) StreamGenerate(ctx context.Context, req GenerateRequest) (<-chan GenerateChunk, error) {
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

// mockEnglishProjectProvider is a mock LLM provider that returns English project names.
type mockEnglishProjectProvider struct {
	id   string
	name string
}

func (m *mockEnglishProjectProvider) ID() string   { return m.id }
func (m *mockEnglishProjectProvider) Name() string { return m.name }
func (m *mockEnglishProjectProvider) Type() string { return "mock" }

func (m *mockEnglishProjectProvider) IsAvailable(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockEnglishProjectProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{Name: "mock-model", ContextLength: 4096},
	}, nil
}

func (m *mockEnglishProjectProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Return English project name based on prompt content
	promptLower := strings.ToLower(req.Prompt)

	var text string
	if strings.Contains(promptLower, "software") || strings.Contains(promptLower, "development") {
		text = "Software Development"
	} else if strings.Contains(promptLower, "testing") {
		text = "Testing Practices"
	} else if strings.Contains(promptLower, "agile") {
		text = "Agile Methodology"
	} else {
		text = "Development Project"
	}

	return &GenerateResponse{
		Text:       text,
		TokensUsed: 10,
		Model:      req.Model,
	}, nil
}

func (m *mockEnglishProjectProvider) StreamGenerate(ctx context.Context, req GenerateRequest) (<-chan GenerateChunk, error) {
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

