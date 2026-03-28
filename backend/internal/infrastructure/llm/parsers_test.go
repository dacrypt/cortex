package llm

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONParser_ParseJSON(t *testing.T) {
	logger := zerolog.Nop()
	parser := NewJSONParser(logger)
	ctx := context.Background()

	t.Run("valid JSON", func(t *testing.T) {
		response := `{"name": "test", "value": 42}`
		var result map[string]interface{}
		err := parser.ParseJSON(ctx, response, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(42), result["value"])
	})

	t.Run("JSON with markdown code blocks", func(t *testing.T) {
		response := "```json\n{\"name\": \"test\"}\n```"
		var result map[string]interface{}
		err := parser.ParseJSON(ctx, response, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("JSON wrapped in text", func(t *testing.T) {
		response := "Here is the response: {\"name\": \"test\"} and some more text"
		var result map[string]interface{}
		err := parser.ParseJSON(ctx, response, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("JSON with trailing comma (should be fixed on retry)", func(t *testing.T) {
		response := `{"name": "test", "value": 42,}`
		var result map[string]interface{}
		err := parser.ParseJSON(ctx, response, &result)
		// Should succeed after aggressive cleaning
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("complex nested JSON", func(t *testing.T) {
		response := `{
			"authors": [{"name": "John", "role": "author"}],
			"tags": ["tag1", "tag2"],
			"metadata": {"count": 10}
		}`
		var result map[string]interface{}
		err := parser.ParseJSON(ctx, response, &result)
		require.NoError(t, err)
		assert.NotNil(t, result["authors"])
		assert.NotNil(t, result["tags"])
	})
}

func TestStringParser_ParseString(t *testing.T) {
	logger := zerolog.Nop()
	parser := NewStringParser(logger)

	t.Run("simple string", func(t *testing.T) {
		response := "test project"
		result := parser.ParseString(response)
		assert.Equal(t, "test project", result)
	})

	t.Run("string with markdown", func(t *testing.T) {
		response := "```\ntest project\n```"
		result := parser.ParseString(response)
		assert.Equal(t, "test project", result)
	})

	t.Run("string with quotes", func(t *testing.T) {
		response := `"test project"`
		result := parser.ParseString(response)
		assert.Equal(t, "test project", result)
	})

	t.Run("string with trailing punctuation", func(t *testing.T) {
		response := "test project."
		result := parser.ParseString(response)
		assert.Equal(t, "test project", result)
	})

	t.Run("string with multiple punctuation", func(t *testing.T) {
		response := "test project,;:"
		result := parser.ParseString(response)
		assert.Equal(t, "test project", result)
	})
}

func TestArrayParser_ParseArray(t *testing.T) {
	logger := zerolog.Nop()
	parser := NewArrayParser(logger)
	ctx := context.Background()

	t.Run("JSON array", func(t *testing.T) {
		response := `["tag1", "tag2", "tag3"]`
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, result)
	})

	t.Run("JSON array with markdown", func(t *testing.T) {
		response := "```json\n[\"tag1\", \"tag2\"]\n```"
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2"}, result)
	})

	t.Run("comma-separated list", func(t *testing.T) {
		response := "tag1, tag2, tag3"
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, result)
	})

	t.Run("comma-separated with brackets", func(t *testing.T) {
		response := "[tag1, tag2, tag3]"
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, result)
	})

	t.Run("array with empty elements", func(t *testing.T) {
		response := `["tag1", "", "tag2"]`
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2"}, result)
	})

	t.Run("array with quotes in elements", func(t *testing.T) {
		response := `["tag1", "'tag2'", "\"tag3\""]`
		result, err := parser.ParseArray(ctx, response)
		require.NoError(t, err)
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, result)
	})
}

func TestPromptTemplates(t *testing.T) {
	t.Run("FormatTagSuggestion", func(t *testing.T) {
		prompt := FormatTagSuggestion(5, "Resumen del documento", "Descripción del documento")
		assert.Contains(t, prompt, "5")
		assert.Contains(t, prompt, "Resumen del documento")
		assert.Contains(t, prompt, "Descripción del documento")
	})

	t.Run("FormatProjectSuggestion", func(t *testing.T) {
		prompt := FormatProjectSuggestion("Proyecto1\nProyecto2", "Contenido del archivo")
		assert.Contains(t, prompt, "Proyecto1")
		assert.Contains(t, prompt, "Contenido del archivo")
	})

	t.Run("FormatSummary", func(t *testing.T) {
		content := "Este es un contenido largo que debería ser truncado si es muy largo"
		prompt := FormatSummary(100, content)
		assert.Contains(t, prompt, "20") // maxLength/5
		assert.Contains(t, prompt, content)
	})

	t.Run("FormatCategoryClassification", func(t *testing.T) {
		prompt := FormatCategoryClassification("Resumen", "Descripción")
		assert.Contains(t, prompt, "Resumen")
		assert.Contains(t, prompt, "Descripción")
		assert.Contains(t, prompt, "Religión y Teología")
	})
}

func TestPromptTemplateRegistry(t *testing.T) {
	registry := NewPromptTemplateRegistry()

	t.Run("register and get template", func(t *testing.T) {
		registry.Register("test", "Hello %s")
		template, ok := registry.Get("test")
		require.True(t, ok)
		assert.NotNil(t, template)
	})

	t.Run("format template", func(t *testing.T) {
		registry.Register("greeting", "Hello %s, welcome to %s")
		result, err := registry.Format("greeting", "John", "Cortex")
		require.NoError(t, err)
		assert.Equal(t, "Hello John, welcome to Cortex", result)
	})

	t.Run("template not found", func(t *testing.T) {
		_, err := registry.Format("nonexistent", "value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Test integration with real-world LLM response formats
func TestRealWorldLLMResponses(t *testing.T) {
	logger := zerolog.Nop()
	jsonParser := NewJSONParser(logger)
	ctx := context.Background()

	t.Run("LLM response with explanation before JSON", func(t *testing.T) {
		response := `Based on the content, here is the JSON:
{
  "tags": ["religion", "theology", "catholic"],
  "category": "Religión y Teología"
}`
		var result map[string]interface{}
		err := jsonParser.ParseJSON(ctx, response, &result)
		require.NoError(t, err)
		assert.NotNil(t, result["tags"])
		assert.Equal(t, "Religión y Teología", result["category"])
	})

	t.Run("LLM response with comments (should be removed)", func(t *testing.T) {
		response := `{
  // This is a comment
  "name": "test",
  // Another comment
  "value": 42
}`
		var result map[string]interface{}
		err := jsonParser.ParseJSON(ctx, response, &result)
		// Should succeed after aggressive cleaning removes comments
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("malformed JSON that should fail gracefully", func(t *testing.T) {
		response := `This is not JSON at all`
		var result map[string]interface{}
		err := jsonParser.ParseJSON(ctx, response, &result)
		// Should fail after all retries
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})
}

// Benchmark tests
func BenchmarkJSONParser_ParseJSON(b *testing.B) {
	logger := zerolog.Nop()
	parser := NewJSONParser(logger)
	ctx := context.Background()
	response := `{"name": "test", "value": 42, "tags": ["tag1", "tag2"]}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		_ = parser.ParseJSON(ctx, response, &result)
	}
}

func BenchmarkStringParser_ParseString(b *testing.B) {
	logger := zerolog.Nop()
	parser := NewStringParser(logger)
	response := `"test project with some text."`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.ParseString(response)
	}
}

