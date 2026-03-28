package llm

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// AnalyzeSentiment analyzes the sentiment of content using LLM.
func (r *Router) AnalyzeSentiment(ctx context.Context, content string, summary string) (*SentimentAnalysis, error) {
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
		prompt = fmt.Sprintf(`Eres un experto en análisis de sentimiento. Analiza el siguiente documento y determina su sentimiento general.

INSTRUCCIONES:
1. Determina el sentimiento general (positivo, negativo, neutral, mixto)
2. Asigna un score de -1.0 (muy negativo) a 1.0 (muy positivo)
3. Identifica emociones presentes (alegría, tristeza, ira, miedo, sorpresa, asco)
4. Proporciona un nivel de confianza

FORMATO DE RESPUESTA (JSON estricto):
{
  "overall_sentiment": "positive|negative|neutral|mixed",
  "score": 0.75,
  "confidence": 0.9,
  "emotions": {
    "joy": 0.8,
    "sadness": 0.1,
    "anger": 0.0,
    "fear": 0.0,
    "surprise": 0.2,
    "disgust": 0.0
  }
}

IMPORTANTE:
- Responde SOLO con JSON válido, sin texto adicional
- El score debe estar entre -1.0 y 1.0
- Las emociones deben estar entre 0.0 y 1.0
- Sé objetivo y preciso%s

Análisis de sentimiento (JSON):`, contentSection)
	} else {
		prompt = fmt.Sprintf(`You are an expert in sentiment analysis. Analyze the following document and determine its overall sentiment.

INSTRUCTIONS:
1. Determine overall sentiment (positive, negative, neutral, mixed)
2. Assign a score from -1.0 (very negative) to 1.0 (very positive)
3. Identify present emotions (joy, sadness, anger, fear, surprise, disgust)
4. Provide a confidence level

RESPONSE FORMAT (strict JSON):
{
  "overall_sentiment": "positive|negative|neutral|mixed",
  "score": 0.75,
  "confidence": 0.9,
  "emotions": {
    "joy": 0.8,
    "sadness": 0.1,
    "anger": 0.0,
    "fear": 0.0,
    "surprise": 0.2,
    "disgust": 0.0
  }
}

IMPORTANT:
- Respond ONLY with valid JSON, no additional text
- Score must be between -1.0 and 1.0
- Emotions must be between 0.0 and 1.0
- Be objective and precise%s

Sentiment analysis (JSON):`, contentSection)
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   500,
		Temperature: 0.2, // Low temperature for consistent analysis
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result SentimentAnalysis

	// Use robust JSON parser with retry capability
	parser := NewJSONParser(r.logger)
	if err := parser.ParseJSON(ctx, resp.Text, &result); err != nil {
		return nil, fmt.Errorf("failed to parse sentiment response: %w", err)
	}

	return &result, nil
}

// SentimentAnalysis is an alias for entity.SentimentAnalysis for backward compatibility.
// Use entity.SentimentAnalysis directly.
type SentimentAnalysis = entity.SentimentAnalysis

