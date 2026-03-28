// Package llm provides LLM router and provider implementations.
package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/utils"
)

const (
	// Error message when no content is available for processing
	errNoContentProvided = "no content, summary, or description provided"

	// Format string for description section in Spanish prompts
	descSectionFormat = "\nDescripción:\n%s\n"

	// Format string for document summary section in Spanish prompts
	summarySectionFormat = "Resumen del documento:\n%s\n"

	llmRetryMaxAttempts   = 100                // Effectively unlimited retries
	llmRetryBaseTimeout   = 30 * time.Minute   // Very long base timeout for analysis
	llmRetryMaxTimeout    = 60 * time.Minute   // Effectively unlimited
	llmRetryBaseBackoff   = 5 * time.Second
	llmRetryMaxBackoff    = 60 * time.Second
)

// Router routes LLM requests to providers.
type Router struct {
	providers        map[string]Provider
	activeProvider   string
	activeModel      string
	logger           zerolog.Logger
	traceWriter      TraceWriter
	prompts          PromptsConfig
	templateRegistry *PromptTemplateRegistry // LangChain-inspired template registry
	mustSucceed      bool
	mu               sync.RWMutex
}

// PromptsConfig holds prompt templates (can be nil to use defaults).
type PromptsConfig interface {
	GetSuggestTags() string
	GetSuggestProject() string
	GetGenerateSummary() string
	GetExtractKeyTerms() string
	GetRAGAnswer() string
	GetClassifyCategory() string
}

// NewRouter creates a new LLM router.
func NewRouter(logger zerolog.Logger) *Router {
	return &Router{
		providers:        make(map[string]Provider),
		logger:           logger.With().Str("component", "llm_router").Logger(),
		prompts:          nil,                         // Will use defaults
		templateRegistry: NewPromptTemplateRegistry(), // Initialize template registry
		mustSucceed:      false,
	}
}

// SetPrompts sets the prompt templates to use.
func (r *Router) SetPrompts(prompts PromptsConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts = prompts
}

// SetMustSucceed forces LLM operations to retry until success (or context cancellation).
func (r *Router) SetMustSucceed(mustSucceed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mustSucceed = mustSucceed
}

// GetTemplateRegistry returns the template registry for advanced template management.
func (r *Router) GetTemplateRegistry() *PromptTemplateRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templateRegistry
}

// GetExtractKeyTermsPrompt returns the key terms extraction prompt with content filled in.
func (r *Router) GetExtractKeyTermsPrompt(summary string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var template string
	if r.prompts != nil && r.prompts.GetExtractKeyTerms() != "" {
		template = r.prompts.GetExtractKeyTerms()
	} else {
		// Default template
		isSpanish := detectRomanceLanguage(summary)
		if isSpanish {
			template = `Extrae los términos clave más importantes del siguiente resumen.
Responde SOLO con una lista de términos separados por comas, sin explicaciones.
Máximo 12 términos. Los términos DEBEN estar en español.
Evita palabras comunes como: que, los, del, por, una, con, las, pero, para, etc.

Resumen:
%s

Términos clave:`
		} else {
			template = `Extract the most important key terms from the following summary.
Respond ONLY with a comma-separated list of terms, no explanations.
Maximum 12 terms. Terms MUST be in the same language as the summary.
Avoid common words like: the, and, for, with, this, that, etc.

Summary:
%s

Key terms:`
		}
	}
	return fmt.Sprintf(template, summary)
}

// GetRAGAnswerPrompt returns the RAG answer prompt with context and query filled in.
func (r *Router) GetRAGAnswerPrompt(context, query string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var template string
	if r.prompts != nil && r.prompts.GetRAGAnswer() != "" {
		template = r.prompts.GetRAGAnswer()
	} else {
		// Default template
		template = `Eres un asistente experto de Cortex. Tu tarea es responder la pregunta del usuario utilizando UNICAMENTE el contexto proporcionado.
Si la información no está en el contexto, indica amablemente que no tienes esa información en tus documentos.
Utiliza un tono profesional y directo. Cita siempre las fuentes usando corchetes como [1], [2], etc., al final de la frase o párrafo que utiliza esa información.

Contexto:
%s

Pregunta: %s

Respuesta del asistente:`
	}
	return fmt.Sprintf(template, context, strings.TrimSpace(query))
}

// SetTraceWriter sets the optional trace writer for LLM prompts/outputs.
func (r *Router) SetTraceWriter(writer TraceWriter) {
	r.traceWriter = writer
}

// RegisterProvider registers a provider.
func (r *Router) RegisterProvider(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[provider.ID()] = provider
	r.logger.Info().
		Str("provider_id", provider.ID()).
		Str("provider_name", provider.Name()).
		Msg("Registered LLM provider")
}

// SetActiveProvider sets the active provider and model.
func (r *Router) SetActiveProvider(providerID, model string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, ok := r.providers[providerID]
	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	r.activeProvider = providerID
	r.activeModel = model

	r.logger.Info().
		Str("provider", providerID).
		Str("model", model).
		Msg("Set active LLM provider")

	// Check if available
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if available, _ := provider.IsAvailable(ctx); !available {
		r.logger.Warn().
			Str("provider", providerID).
			Msg("Active provider is not available")
	}

	return nil
}

// GetActiveProvider returns the active provider.
func (r *Router) GetActiveProvider() (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.activeProvider == "" {
		return nil, "", fmt.Errorf("no active provider set")
	}

	provider, ok := r.providers[r.activeProvider]
	if !ok {
		return nil, "", fmt.Errorf("active provider not found: %s", r.activeProvider)
	}

	return provider, r.activeModel, nil
}

// Generate generates a completion using the active provider.
func (r *Router) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	provider, model, err := r.GetActiveProvider()
	if err != nil {
		return nil, err
	}

	if req.Model == "" {
		req.Model = model
	}

	r.logger.Debug().
		Str("provider", provider.ID()).
		Str("model", req.Model).
		Int("prompt_chars", len(req.Prompt)).
		Int("max_tokens", req.MaxTokens).
		Float64("temperature", req.Temperature).
		Int("timeout_ms", req.TimeoutMs).
		Msg("Dispatching LLM generate request")

	start := time.Now()
	var resp *GenerateResponse
	var lastErr error
	baseTimeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if baseTimeout <= 0 {
		baseTimeout = llmRetryBaseTimeout
	}
	r.mu.RLock()
	mustSucceed := r.mustSucceed
	r.mu.RUnlock()

	for attempt := 1; ; attempt++ {
		if !mustSucceed && attempt > llmRetryMaxAttempts {
			break
		}
		attemptTimeout := baseTimeout * time.Duration(1<<(attempt-1))
		if attemptTimeout > llmRetryMaxTimeout {
			attemptTimeout = llmRetryMaxTimeout
		}

		attemptReq := req
		attemptReq.TimeoutMs = int(attemptTimeout.Milliseconds())

		attemptCtx := ctx
		cancel := func() {}
		if attempt > 1 || mustSucceed {
			// Preserve trace info but avoid inheriting a too-short deadline.
			baseCtx := context.Background()
			if info, ok := TraceInfoFromContext(ctx); ok {
				baseCtx = WithTraceInfo(baseCtx, info)
			}
			attemptCtx, cancel = context.WithTimeout(baseCtx, attemptTimeout)
		}

		resp, lastErr = provider.Generate(attemptCtx, attemptReq)
		cancel()
		if lastErr == nil {
			break
		}

		if !isTimeoutError(lastErr) && !mustSucceed {
			break
		}

		backoff := llmRetryBaseBackoff * time.Duration(1<<(attempt-1))
		if backoff > llmRetryMaxBackoff {
			backoff = llmRetryMaxBackoff
		}

		r.logger.Warn().
			Err(lastErr).
			Str("provider", provider.ID()).
			Str("model", req.Model).
			Int("attempt", attempt).
			Dur("next_timeout", attemptTimeout).
			Dur("backoff", backoff).
			Msg("LLM generation failed; retrying with extended timeout")
		time.Sleep(backoff)
	}

	if lastErr != nil {
		r.logger.Error().
			Err(lastErr).
			Str("provider", provider.ID()).
			Str("model", req.Model).
			Msg("LLM generation failed")
		r.writeTrace(ctx, req, nil, time.Since(start), lastErr)
		return nil, lastErr
	}

	resp.ProcessingTimeMs = time.Since(start).Milliseconds()
	resp.Provider = provider.ID()
	resp.Model = req.Model

	// Log timing at Info level for analysis
	r.logger.Info().
		Str("provider", provider.ID()).
		Str("model", req.Model).
		Int64("duration_ms", resp.ProcessingTimeMs).
		Int("prompt_chars", len(req.Prompt)).
		Int("tokens", resp.TokensUsed).
		Msg("[LLM_TIMING] LLM generation completed")
	r.writeTrace(ctx, req, resp, time.Since(start), nil)

	return resp, nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		errors.Is(err, context.DeadlineExceeded)
}

func (r *Router) writeTrace(ctx context.Context, req GenerateRequest, resp *GenerateResponse, duration time.Duration, err error) {
	if r.traceWriter == nil {
		return
	}
	info, ok := TraceInfoFromContext(ctx)
	if !ok {
		return
	}
	trace := LLMTrace{
		Info:        info,
		Prompt:      req.Prompt,
		Model:       req.Model,
		DurationMs:  duration.Milliseconds(),
		GeneratedAt: time.Now(),
	}
	if resp != nil {
		trace.Output = resp.Text
		trace.TokensUsed = resp.TokensUsed
		if trace.Model == "" {
			trace.Model = resp.Model
		}
	}
	if err != nil {
		msg := err.Error()
		trace.Error = &msg
	}
	if writeErr := r.traceWriter.WriteLLMTrace(ctx, trace); writeErr != nil {
		r.logger.Warn().Err(writeErr).Msg("Failed to write LLM trace")
	}
}

// StreamGenerate streams a completion.
func (r *Router) StreamGenerate(ctx context.Context, req GenerateRequest) (<-chan GenerateChunk, error) {
	provider, model, err := r.GetActiveProvider()
	if err != nil {
		return nil, err
	}

	if req.Model == "" {
		req.Model = model
	}

	return provider.StreamGenerate(ctx, req)
}

// ListProviders returns all registered providers.
func (r *Router) ListProviders() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []ProviderInfo
	for _, p := range r.providers {
		providers = append(providers, ProviderInfo{
			ID:   p.ID(),
			Name: p.Name(),
			Type: p.Type(),
		})
	}

	return providers
}

// GetProviderStatus returns the status of a provider.
func (r *Router) GetProviderStatus(ctx context.Context, providerID string) (*ProviderStatus, error) {
	r.mu.RLock()
	provider, ok := r.providers[providerID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	available, err := provider.IsAvailable(ctx)
	status := &ProviderStatus{
		Info:      ProviderInfo{ID: provider.ID(), Name: provider.Name(), Type: provider.Type()},
		Available: available,
	}

	if err != nil {
		errStr := err.Error()
		status.Error = &errStr
	}

	if available {
		models, _ := provider.ListModels(ctx)
		status.Models = models
	}

	return status, nil
}

// IsAvailable checks if any provider is available.
func (r *Router) IsAvailable(ctx context.Context) bool {
	provider, _, err := r.GetActiveProvider()
	if err != nil {
		return false
	}

	available, _ := provider.IsAvailable(ctx)
	return available
}

// SuggestTags suggests tags for content.
func (r *Router) SuggestTags(ctx context.Context, content string, maxTags int) ([]string, error) {
	// Detect language from content
	isSpanish := detectRomanceLanguage(content)

	var prompt string
	if isSpanish {
		// Use template for tag suggestion (with content instead of summary/description)
		prompt = fmt.Sprintf(`Analiza la siguiente información y sugiere hasta %d tags relevantes en español.
Usa tags estilo slug: máximo 3 palabras unidas por guiones, sin espacios.
Permite acentos, números y guiones. Evita duplicados y variantes del mismo concepto (singular/plural, años, sufijos).
Evita puntuación y mantén cada tag en 32 caracteres o menos. Prioriza relevancia sobre cantidad.
Si no hay tags claramente relevantes, responde [].
Responde SOLO con un array JSON de strings de tags, nada más.

Contenido:
%s

Tags (array JSON):`, maxTags, truncateContent(content, 2000))
	} else {
		prompt = fmt.Sprintf(`Analyze the following content and suggest up to %d relevant tags.
Use slug-style tags: up to 3 words joined by hyphens, no spaces.
Allow accents, numbers, and hyphens. Avoid duplicates and variants of the same concept (singular/plural, years, suffixes).
Avoid punctuation and keep each tag at 32 characters or less. Prefer relevance over quantity.
If no clearly relevant tags exist, return [].
Return only a JSON array of tag strings, nothing else.

Content:
%s

Tags (JSON array):`, maxTags, truncateContent(content, 2000))
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   100,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	arrayParser := NewArrayParser(r.logger)
	result, err := arrayParser.ParseArray(ctx, resp.Text)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SuggestProject suggests a project for content.
// It accepts either raw content (for backward compatibility) or summary + description (preferred).
func (r *Router) SuggestProject(ctx context.Context, content string, existingProjects []string) (string, error) {
	// Use content as fallback since no summary/description provided
	return r.SuggestProjectWithSummary(ctx, "", "", content, "", existingProjects, "")
}

// buildProjectList formats existing projects into a list string
func buildProjectList(existingProjects []string) string {
	if len(existingProjects) == 0 {
		return "None available"
	}
	var builder strings.Builder
	for _, p := range existingProjects {
		builder.WriteString("- ")
		builder.WriteString(p)
		builder.WriteString("\n")
	}
	return builder.String()
}

// getContentSampleForLanguageDetection gets a content sample for language detection
func getContentSampleForLanguageDetection(summary, description, fallbackContent string) (string, error) {
	if summary != "" {
		return summary, nil
	}
	if description != "" {
		return description, nil
	}
	if fallbackContent != "" {
		return fallbackContent, nil
	}
	return "", fmt.Errorf(errNoContentProvided)
}

// detectLanguageForProject determines if content is in a Romance language (Spanish or Portuguese) based on language code or detection
func detectLanguageForProject(languageCode []string, contentSample string) bool {
	if len(languageCode) > 0 && languageCode[0] != "" {
		return languageCode[0] == "es" || languageCode[0] == "pt"
	}
	return detectRomanceLanguage(truncateContent(contentSample, 500))
}

// buildContentSectionForProject builds content section with language-appropriate labels
// relativePath is included to provide context about file location
func buildContentSectionForProject(summary, description, fallbackContent, relativePath string, isSpanish bool) string {
	var builder strings.Builder
	
	// Add path context first - this provides important semantic information
	if relativePath != "" {
		pathAnalyzer := utils.NewPathAnalyzer()
		pathContext := pathAnalyzer.FormatPathForContext(relativePath)
		if pathContext != "" {
			if isSpanish {
				builder.WriteString(fmt.Sprintf("Ubicación del archivo:\n%s\n\n", pathContext))
			} else {
				builder.WriteString(fmt.Sprintf("File location:\n%s\n\n", pathContext))
			}
		}
	}
	
	if summary != "" {
		if isSpanish {
			builder.WriteString(fmt.Sprintf("Resumen:\n%s\n", summary))
		} else {
			builder.WriteString(fmt.Sprintf("Summary:\n%s\n", summary))
		}
		if description != "" {
			if isSpanish {
				builder.WriteString(fmt.Sprintf("\nDescripción:\n%s\n", description))
			} else {
				builder.WriteString(fmt.Sprintf("\nDescription:\n%s\n", description))
			}
		}
	} else if fallbackContent != "" {
		if isSpanish {
			builder.WriteString(fmt.Sprintf("Contenido:\n%s\n", truncateContent(fallbackContent, 1500)))
		} else {
			builder.WriteString(fmt.Sprintf("Content:\n%s\n", truncateContent(fallbackContent, 1500)))
		}
	}
	return builder.String()
}

// buildProjectSuggestionPrompt builds the prompt for project suggestion
func (r *Router) buildProjectSuggestionPrompt(projectList, contentSection string, isSpanish bool) string {
	r.mu.RLock()
	hasPrompts := r.prompts != nil
	var template string
	if hasPrompts {
		template = r.prompts.GetSuggestProject()
	}
	r.mu.RUnlock()

	if hasPrompts && template != "" {
		return fmt.Sprintf(template, projectList, contentSection)
	}

	// Use default prompts
	if isSpanish {
		if !hasPrompts {
			return FormatProjectSuggestion(projectList, contentSection)
		}
		return fmt.Sprintf(`Eres un asistente experto en organización de documentos.

Basándote en la siguiente información, sugiere el proyecto/contexto más apropiado de la lista a continuación.
Si ninguno de los proyectos existentes encaja, sugiere un nuevo nombre de proyecto.

REGLAS CRÍTICAS:
1. El nombre del proyecto DEBE estar en ESPAÑOL, el mismo idioma que el contenido.
2. El nombre debe ser descriptivo y relevante al contenido.
3. El nombre DEBE ser CONCISO: máximo 50 caracteres (preferiblemente 30-40).
4. Evita usar dos puntos (:) en el nombre.
5. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales.

Proyectos existentes:
%s

%s

Proyecto sugerido:`, projectList, contentSection)
	}

	// English default
	return fmt.Sprintf(`Based on the following information, suggest the most appropriate project/context from the list below.
If none of the existing projects fit, suggest a new project name.
IMPORTANT: The project name MUST be in the same language as the content.
Return only the project name, nothing else.

Existing projects:
%s

%s

Suggested project:`, projectList, contentSection)
}

// validateAndRetryProjectName validates project name and retries if needed
func (r *Router) validateAndRetryProjectName(ctx context.Context, prompt, responseText string, stringParser *StringParser, logger zerolog.Logger) (string, error) {
	cleaned := stringParser.ParseString(responseText)

	if len(cleaned) < 3 {
		logger.Warn().
			Str("response", responseText).
			Str("cleaned", cleaned).
			Int("length", len(cleaned)).
			Msg("Project name too short, may be truncated - retrying with longer MaxTokens")
		if len(responseText) < 10 {
			resp2, err2 := r.Generate(ctx, GenerateRequest{
				Prompt:      prompt + "\n\nIMPORTANTE: Responde con el nombre COMPLETO del proyecto, sin truncar.",
				MaxTokens:   300,
				Temperature: 0.2,
			})
			if err2 == nil {
				cleaned2 := stringParser.ParseString(resp2.Text)
				if len(cleaned2) >= 3 {
					logger.Info().Str("retry_result", cleaned2).Msg("Project name retry successful")
					return cleaned2, nil
				}
			}
		}
		return "", fmt.Errorf("project name too short (got %d chars): %s", len(cleaned), cleaned)
	}

	if len(cleaned) > 100 {
		logger.Warn().Str("project", truncateString(cleaned, 50)).Msg("Project name too long, truncating")
		cleaned = cleaned[:100]
	}

	if strings.Contains(cleaned, "import ") || strings.Contains(cleaned, "def ") || strings.Contains(cleaned, "class ") {
		logger.Warn().Str("project", truncateString(cleaned, 50)).Msg("Project name contains code patterns, may need manual review")
	}

	return cleaned, nil
}

// SuggestProjectWithSummary suggests a project using summary + description instead of raw content.
// relativePath provides context about file location for better project assignment.
// languageCode is optional; if provided, it will be used instead of detecting from content.
func (r *Router) SuggestProjectWithSummary(ctx context.Context, summary string, description string, fallbackContent string, relativePath string, existingProjects []string, languageCode ...string) (string, error) {
	projectList := buildProjectList(existingProjects)

	contentSample, err := getContentSampleForLanguageDetection(summary, description, fallbackContent)
	if err != nil {
		return "", err
	}

	isSpanish := detectLanguageForProject(languageCode, contentSample)
	contentSection := buildContentSectionForProject(summary, description, fallbackContent, relativePath, isSpanish)
	prompt := r.buildProjectSuggestionPrompt(projectList, contentSection, isSpanish)

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   200,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	stringParser := NewStringParser(r.logger)
	return r.validateAndRetryProjectName(ctx, prompt, resp.Text, stringParser, r.logger)
}

// GenerateSummary generates a summary of content.
// The summary will be generated in the same language as the content.
func (r *Router) GenerateSummary(ctx context.Context, content string, maxLength int) (string, error) {
	// Detect language from content sample
	contentSample := truncateContent(content, 500)
	isSpanish := detectRomanceLanguage(contentSample)

	var prompt string
	if isSpanish {
		// Use template for summary generation
		prompt = FormatSummary(maxLength, truncateContent(content, 3000))
	} else {
		prompt = fmt.Sprintf(`Summarize the following content in %d words or less.
Be concise and capture the main points.
IMPORTANTE: The summary MUST be in the same language as the content.

Content:
%s

Summary:`, maxLength/5, truncateContent(content, 3000))
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   maxLength / 3,
		Temperature: 0.5,
	})
	if err != nil {
		return "", err
	}

	stringParser := NewStringParser(r.logger)
	return stringParser.ParseString(resp.Text), nil
}

// ClassifyCategory classifies content into one of the provided categories.
// It accepts either raw content (for backward compatibility) or summary + description (preferred).
func (r *Router) ClassifyCategory(ctx context.Context, content string, categories []string) (string, error) {
	return r.ClassifyCategoryWithSummary(ctx, content, "", "", categories)
}

// ClassifyCategoryWithSummary classifies content using summary and description instead of raw content.
// This provides better context than truncated content.
func (r *Router) ClassifyCategoryWithSummary(ctx context.Context, summary string, description string, fallbackContent string, categories []string) (string, error) {
	if len(categories) == 0 {
		return "", fmt.Errorf("no categories provided")
	}
	categoryList := strings.Join(categories, "\n- ")

	// Build content section: prefer summary + description, fallback to truncated content
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf(summarySectionFormat, summary)
		if description != "" {
			contentSection += fmt.Sprintf(descSectionFormat, description)
		}
	} else if fallbackContent != "" {
		// Fallback to truncated content if no summary available
		contentSection = fmt.Sprintf("Contenido (fragmento):\n%s\n", truncateContent(fallbackContent, 2000))
	} else {
		return "", fmt.Errorf(errNoContentProvided)
	}

	// Use template for category classification
	var prompt string
	if categoryList != "" {
		// Custom category list provided
		prompt = fmt.Sprintf(`Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

REGLAS CRÍTICAS DE CLASIFICACIÓN:
1. **DETECCIÓN DE CONTENIDO RELIGIOSO/TEOLÓGICO (PRIORIDAD ALTA)**:
   - Si el documento menciona: "Dios", "santo", "santos", "religión", "teología", "fe", "evangelios", "Jesús", "Cristo", "iglesia", "místico", "espiritual", "oración", "sacramento", "biblia", "sagrado", "divino", "divinidad", "conferencia religiosa", "vida de santos"
   - ENTONCES la categoría DEBE ser: "Religión y Teología"
   - NO importa si también menciona términos científicos o técnicos

2. **OTRAS CATEGORÍAS**:
   - Solo si NO es religioso/teológico, entonces clasifica según el tema principal

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- %s

%s

Ejemplos de clasificación:
- Documento sobre "conferencias sobre existencia de Dios y fe" → Religión y Teología
- Documento sobre "vidas de santos y experiencias místicas" → Religión y Teología
- Documento sobre "religión y ciencia" → Religión y Teología (si menciona términos religiosos)
- Documento sobre "manual de usuario de software" → Documentación Técnica
- Documento sobre "análisis de mercado" → Investigación y Análisis
- Documento sobre "enciclopedia o diccionario" → Educación y Referencia

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).
Si no puedes determinar la categoría, responde: "Sin Clasificar"`, categoryList, contentSection)
	} else {
		// Use default template
		// Remove the summary prefix if present
		summaryPrefix := "Resumen del documento:\n"
		contentForTemplate := strings.TrimPrefix(contentSection, summaryPrefix)
		prompt = FormatCategoryClassification(
			contentForTemplate,
			"",
		)
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   80,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	stringParser := NewStringParser(r.logger)
	response := stringParser.ParseString(resp.Text)
	normalized := strings.Trim(strings.TrimSpace(response), " .,:;\"'")
	for _, category := range categories {
		if strings.EqualFold(normalized, category) {
			return category, nil
		}
	}
	for _, category := range categories {
		if strings.Contains(strings.ToLower(response), strings.ToLower(category)) {
			return category, nil
		}
	}
	return "Sin Clasificar", nil
}

// ClassifyCategoryWithContext classifies content with context from similar files.
// It accepts either raw content (for backward compatibility) or summary + description (preferred).
func (r *Router) ClassifyCategoryWithContext(ctx context.Context, content string, categories []string, similarCategories []string) (string, error) {
	return r.ClassifyCategoryWithContextAndSummary(ctx, content, "", "", categories, similarCategories)
}

// buildContextInfoFromSimilarCategories builds context information from similar file categories
func buildContextInfoFromSimilarCategories(similarCategories []string) string {
	if len(similarCategories) == 0 {
		return ""
	}

	// Count category occurrences for consensus
	categoryCounts := make(map[string]int)
	for _, cat := range similarCategories {
		categoryCounts[cat]++
	}

	// Find most common category
	maxCount := 0
	mostCommon := ""
	for cat, count := range categoryCounts {
		if count > maxCount {
			maxCount = count
			mostCommon = cat
		}
	}

	// Build context with emphasis on consensus
	contextInfo := "\n\n⚠️ INFORMACIÓN CRÍTICA DE ARCHIVOS SIMILARES:\n"
	contextInfo += fmt.Sprintf("Archivos similares en el workspace están categorizados como:\n- %s\n\n", strings.Join(similarCategories, "\n- "))
	if maxCount >= 2 {
		contextInfo += fmt.Sprintf("🔍 CONSENSO DETECTADO: La categoría más común entre archivos similares es \"%s\" (aparece %d veces).\n", mostCommon, maxCount)
		contextInfo += "Esta es una señal FUERTE de que este documento probablemente pertenece a la misma categoría.\n"
	}
	contextInfo += "USA ESTA INFORMACIÓN como señal principal para clasificar este documento.\n"
	return contextInfo
}

// buildContentSectionForClassification builds content section for classification
func buildContentSectionForClassification(summary, description, fallbackContent string) (string, error) {
	if summary != "" {
		contentSection := fmt.Sprintf(summarySectionFormat, summary)
		if description != "" {
			contentSection += fmt.Sprintf(descSectionFormat, description)
		}
		return contentSection, nil
	}
	if fallbackContent != "" {
		return fmt.Sprintf("Contenido (fragmento):\n%s\n", truncateContent(fallbackContent, 2000)), nil
	}
	return "", fmt.Errorf(errNoContentProvided)
}

// matchCategoryResponse matches LLM response to valid category
func matchCategoryResponse(response string, categories []string) string {
	// Validate category against allowed list first
	validated, err := validateCategory(response, categories)
	if err == nil {
		return validated
	}

	// Fallback: try fuzzy matching
	normalized := strings.Trim(strings.TrimSpace(response), " .,:;\"'")
	for _, category := range categories {
		if strings.EqualFold(normalized, category) {
			return category
		}
	}
	for _, category := range categories {
		if strings.Contains(strings.ToLower(response), strings.ToLower(category)) {
			return category
		}
	}
	return "Sin Clasificar"
}

// ClassifyCategoryWithContextAndSummary classifies using summary + description with context from similar files.
func (r *Router) ClassifyCategoryWithContextAndSummary(ctx context.Context, summary string, description string, fallbackContent string, categories []string, similarCategories []string) (string, error) {
	if len(categories) == 0 {
		return "", fmt.Errorf("no categories provided")
	}
	categoryList := strings.Join(categories, "\n- ")

	contextInfo := buildContextInfoFromSimilarCategories(similarCategories)
	contentSection, err := buildContentSectionForClassification(summary, description, fallbackContent)
	if err != nil {
		return "", err
	}

	prompt := FormatClassifyCategory(categoryList, contextInfo, contentSection)

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   80,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	stringParser := NewStringParser(r.logger)
	response := stringParser.ParseString(resp.Text)
	// Additional cleanup: remove trailing punctuation
	response = strings.TrimSuffix(response, ".")
	response = strings.TrimSuffix(response, ",")
	response = strings.TrimSpace(response)

	return matchCategoryResponse(response, categories), nil
}

// SuggestTagsWithContext suggests tags with context from similar files.
// It accepts either raw content (for backward compatibility) or summary + description (preferred).
func (r *Router) SuggestTagsWithContext(ctx context.Context, content string, maxTags int, contextTags []string) ([]string, error) {
	return r.SuggestTagsWithContextAndSummary(ctx, content, "", "", maxTags, contextTags)
}

// SuggestTagsWithContextAndSummary suggests tags using summary + description with context from similar files.
func (r *Router) SuggestTagsWithContextAndSummary(ctx context.Context, summary string, description string, fallbackContent string, maxTags int, contextTags []string) ([]string, error) {
	contextInfo := ""
	if len(contextTags) > 0 {
		contextInfo = fmt.Sprintf("\n\nTags comunes en archivos similares del workspace:\n- %s\n\nConsidera estos tags como referencia para mantener consistencia.", strings.Join(contextTags, "\n- "))
	}

	// Build content section: prefer summary + description, fallback to truncated content
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf("Resumen:\n%s\n", summary)
		if description != "" {
			contentSection += fmt.Sprintf(descSectionFormat, description)
		}
	} else if fallbackContent != "" {
		// Fallback to truncated content if no summary available
		contentSection = fmt.Sprintf("Contenido:\n%s\n", truncateContent(fallbackContent, 2000))
	} else {
		return nil, fmt.Errorf("no content, summary, or description provided")
	}

	// Detect language from content
	isSpanish := detectRomanceLanguage(summary + description + fallbackContent)

	var prompt string
	if isSpanish {
		prompt = fmt.Sprintf(`Analiza la siguiente información y sugiere hasta %d tags relevantes en español.
Usa tags estilo slug: máximo 3 palabras unidas por guiones, sin espacios.
Permite acentos, números y guiones. Evita duplicados y variantes del mismo concepto (singular/plural, años, sufijos).
Evita puntuación y mantén cada tag en 32 caracteres o menos. Prioriza relevancia sobre cantidad.
Si no hay tags claramente relevantes, responde [].
Responde SOLO con un array JSON de strings de tags, nada más.%s

%s

Tags (array JSON):`, maxTags, contextInfo, contentSection)
	} else {
		prompt = fmt.Sprintf(`Analyze the following information and suggest up to %d relevant tags.
Use slug-style tags: up to 3 words joined by hyphens, no spaces.
Allow accents, numbers, and hyphens. Avoid duplicates and variants of the same concept (singular/plural, years, suffixes).
Avoid punctuation and keep each tag at 32 characters or less. Prefer relevance over quantity.
If no clearly relevant tags exist, return [].
Return only a JSON array of tag strings, nothing else.%s

%s

Tags (JSON array):`, maxTags, contextInfo, contentSection)
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   100,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	arrayParser := NewArrayParser(r.logger)
	result, err := arrayParser.ParseArray(ctx, resp.Text)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GenerateSummaryWithContext generates a summary with context from related files.
func (r *Router) GenerateSummaryWithContext(ctx context.Context, content string, maxLength int, contextSnippets []string) (string, error) {
	contextInfo := ""
	if len(contextSnippets) > 0 {
		contextInfo = "\n\nContexto relacionado del workspace (documentos similares encontrados):\n"
		for i, snippet := range contextSnippets {
			if i >= 5 { // Limit to 5 examples
				break
			}
			snippetText := truncateContent(snippet, 300) // Increased from 200 to 300
			contextInfo += fmt.Sprintf("[%d] Documento similar:\n   %s\n\n", i+1, snippetText)
		}
		contextInfo += "Considera este contexto relacionado al generar el resumen. Si hay documentos similares, mantén consistencia en el estilo y terminología.\n"
	}

	// Detect language from content sample
	contentSample := truncateContent(content, 500)
	isSpanish := detectRomanceLanguage(contentSample)

	var prompt string
	if isSpanish {
		prompt = fmt.Sprintf(`Eres un experto en resumir documentos. Resume el siguiente contenido en %d palabras o menos.

INSTRUCCIONES:
1. Sé conciso y captura los puntos principales
2. Identifica: quién, qué, cuándo, dónde, por qué
3. Si es un documento religioso/teológico, menciona el tema espiritual principal
4. Si es un documento técnico, menciona la tecnología o metodología principal
5. El resumen DEBE estar en español, el mismo idioma que el contenido%s

Tipo de documento: Texto
Tamaño aproximado: %d palabras

Contenido:
%s

Resumen:`, maxLength/5, contextInfo, len(strings.Fields(content)), truncateContent(content, 3000))
	} else {
		prompt = fmt.Sprintf(`You are an expert at summarizing documents. Summarize the following content in %d words or less.

INSTRUCTIONS:
1. Be concise and capture the main points
2. Identify: who, what, when, where, why
3. If it's a religious/theological document, mention the main spiritual theme
4. If it's a technical document, mention the main technology or methodology
5. The summary MUST be in the same language as the content%s

Document type: Text
Approximate size: %d words

Content:
%s

Summary:`, maxLength/5, contextInfo, len(strings.Fields(content)), truncateContent(content, 3000))
	}

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   maxLength / 3,
		Temperature: 0.5,
	})
	if err != nil {
		return "", err
	}

	stringParser := NewStringParser(r.logger)
	cleaned := stringParser.ParseString(resp.Text)
	// Additional cleanup: remove trailing punctuation
	cleaned = strings.TrimSuffix(cleaned, ".")
	cleaned = strings.TrimSuffix(cleaned, ",")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned, nil
}

// truncateString truncates a string to max length for logging.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FindRelatedFiles returns related file paths from candidate list.
func (r *Router) FindRelatedFiles(ctx context.Context, content string, candidates []string, maxResults int) ([]string, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidateList := candidates
	if len(candidateList) > 100 {
		candidateList = candidateList[:100]
	}

	prompt := FormatFindRelatedFiles(content, candidateList, maxResults)

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   200,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	arrayParser := NewArrayParser(r.logger)
	result, err := arrayParser.ParseArray(ctx, resp.Text)
	if err != nil {
		return nil, err
	}
	return result, nil
}

var (
	// Spanish common words for language detection
	spanishWords = []string{
		"el", "la", "los", "las", "de", "del", "que", "y", "a", "en", "un", "una", "es", "son",
		"con", "por", "para", "como", "más", "pero", "sus", "le", "ha", "me", "si", "sin", "sobre",
		"este", "esta", "estos", "estas", "ese", "esa", "eso", "esos", "esas",
		"también", "muy", "ya", "todo", "todos", "toda", "todas",
		"ser", "estar", "haber", "tener", "hacer", "decir", "ir", "ver",
	}
	spanishWordsSet = makeSpanishWordsSet()
	// Portuguese common words for language detection
	portugueseWords = []string{
		"o", "a", "os", "as", "de", "do", "da", "dos", "das", "que", "e", "em", "um", "uma", "é", "são",
		"com", "por", "para", "como", "mais", "mas", "seus", "suas", "lhe", "tem", "me", "se", "sem", "sobre",
		"este", "esta", "estes", "estas", "esse", "essa", "isso", "esses", "essas",
		"também", "muito", "já", "todo", "todos", "toda", "todas",
		"ser", "estar", "ter", "fazer", "dizer", "ir", "ver", "pode", "pode", "foi", "foi", "são",
		"não", "não", "quando", "onde", "quem", "qual", "quais", "até", "até", "entre", "durante",
	}
	portugueseWordsSet = makePortugueseWordsSet()
)

// makeSpanishWordsSet creates a map for O(1) lookups
func makeSpanishWordsSet() map[string]bool {
	set := make(map[string]bool, len(spanishWords))
	for _, word := range spanishWords {
		set[word] = true
	}
	return set
}

// makePortugueseWordsSet creates a map for O(1) lookups
func makePortugueseWordsSet() map[string]bool {
	set := make(map[string]bool, len(portugueseWords))
	for _, word := range portugueseWords {
		set[word] = true
	}
	return set
}

// countSpanishWords counts Spanish words in content
func countSpanishWords(contentLower string) (spanishCount, totalWords int) {
	words := strings.Fields(contentLower)
	for _, word := range words {
		word = strings.Trim(word, ".,;:!?()[]{}\"'")
		if len(word) < 2 {
			continue
		}
		totalWords++
		if spanishWordsSet[word] {
			spanishCount++
		}
	}
	return spanishCount, totalWords
}

// countSpanishCharacters counts Spanish-specific characters
func countSpanishCharacters(contentLower string) int {
	count := 0
	for _, char := range contentLower {
		if char == 'á' || char == 'é' || char == 'í' || char == 'ó' || char == 'ú' ||
			char == 'ñ' || char == 'ü' || char == '¿' || char == '¡' {
			count++
		}
	}
	return count
}

// countPortugueseWords counts Portuguese words in content
func countPortugueseWords(contentLower string) (portugueseCount, totalWords int) {
	words := strings.Fields(contentLower)
	for _, word := range words {
		word = strings.Trim(word, ".,;:!?()[]{}\"'")
		if len(word) < 2 {
			continue
		}
		totalWords++
		if portugueseWordsSet[word] {
			portugueseCount++
		}
	}
	return portugueseCount, totalWords
}

// countPortugueseCharacters counts Portuguese-specific characters
func countPortugueseCharacters(contentLower string) int {
	count := 0
	for _, char := range contentLower {
		if char == 'á' || char == 'é' || char == 'í' || char == 'ó' || char == 'ú' ||
			char == 'ã' || char == 'õ' || char == 'â' || char == 'ê' || char == 'ô' ||
			char == 'ç' || char == 'à' || char == 'ü' {
			count++
		}
	}
	return count
}

// detectSpanish detects if content is primarily in Spanish.
// Uses common Spanish words and character patterns.
func detectSpanish(content string) bool {
	if len(content) == 0 {
		return false
	}

	contentLower := strings.ToLower(content)
	spanishCount, totalWords := countSpanishWords(contentLower)
	spanishChars := countSpanishCharacters(contentLower)

	if totalWords == 0 {
		return spanishChars > 0
	}

	spanishWordRatio := float64(spanishCount) / float64(totalWords)
	hasSpanishChars := spanishChars > 0 && len(content) > 50

	return spanishWordRatio > 0.20 || hasSpanishChars
}

// detectPortuguese detects if content is primarily in Portuguese.
// Uses common Portuguese words and character patterns.
func detectPortuguese(content string) bool {
	if len(content) == 0 {
		return false
	}

	contentLower := strings.ToLower(content)
	portugueseCount, totalWords := countPortugueseWords(contentLower)
	portugueseChars := countPortugueseCharacters(contentLower)

	if totalWords == 0 {
		return portugueseChars > 0
	}

	portugueseWordRatio := float64(portugueseCount) / float64(totalWords)
	hasPortugueseChars := portugueseChars > 0 && len(content) > 50

	return portugueseWordRatio > 0.20 || hasPortugueseChars
}

// detectRomanceLanguage detects if content is in a Romance language (Spanish or Portuguese).
// This is useful for selecting appropriate prompt templates.
func detectRomanceLanguage(content string) bool {
	return detectSpanish(content) || detectPortuguese(content)
}

// DetectLanguage uses LLM to detect the language of content.
// Returns ISO 639-1 language code (e.g., "es", "en", "fr", "de", "pt").
// Falls back to heuristic detection if LLM is unavailable.
func (r *Router) DetectLanguage(ctx context.Context, content string) (string, error) {
	contentSample := truncateContent(content, 500)

	// First try heuristic detection for Spanish (fast, no LLM call needed)
	if detectSpanish(contentSample) {
		return "es", nil
	}

	// Then try heuristic detection for Portuguese
	if detectPortuguese(contentSample) {
		return "pt", nil
	}

	// If LLM is not available, use heuristic detection
	if !r.IsAvailable(ctx) {
		// Default to English if not Spanish or Portuguese
		return "en", nil
	}

	// Use LLM for more accurate detection
	contentSampleFull := truncateContent(content, 1000)
	prompt := FormatDetectLanguage(content)

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   5,
		Temperature: 0.1,
	})
	if err != nil {
		// Fallback to heuristic if LLM fails
		if detectSpanish(contentSampleFull) {
			return "es", nil
		}
		if detectPortuguese(contentSampleFull) {
			return "pt", nil
		}
		return "en", nil
	}

	// Clean and normalize the response
	stringParser := NewStringParser(r.logger)
	langCode := strings.ToLower(strings.TrimSpace(stringParser.ParseString(resp.Text)))

	// Validate it's a 2-letter code
	if len(langCode) >= 2 {
		langCode = langCode[:2]
		// Common language codes
		validCodes := map[string]bool{
			"es": true, "en": true, "fr": true, "de": true, "pt": true,
			"it": true, "ru": true, "zh": true, "ja": true, "ko": true,
			"ar": true, "hi": true, "nl": true, "sv": true, "pl": true,
			"tr": true, "vi": true, "th": true, "cs": true, "el": true,
		}
		if validCodes[langCode] {
			return langCode, nil
		}
	}

	// Fallback to heuristic if LLM response is invalid
	if detectSpanish(contentSampleFull) {
		return "es", nil
	}
	if detectPortuguese(contentSampleFull) {
		return "pt", nil
	}
	return "en", nil
}

// Helper functions

// validateCategory validates a category against the allowed list.
func validateCategory(category string, allowedCategories []string) (string, error) {
	category = strings.TrimSpace(category)
	category = strings.TrimSuffix(category, ".")
	category = strings.Trim(category, `"'`)

	for _, valid := range allowedCategories {
		if strings.EqualFold(category, valid) {
			return valid, nil
		}
	}

	return "", fmt.Errorf("categoría '%s' no válida", category)
}

// ExtractContextualInfo extracts structured contextual information from a document
// using LLM with RAG support. This includes authors, publication details, dates,
// locations, people, organizations, and other contextual metadata.
func (r *Router) ExtractContextualInfo(ctx context.Context, content string, summary string, ragContext []string) (string, error) {
	// Build context section from RAG results
	contextInfo := ""
	if len(ragContext) > 0 {
		contextInfo = "\n\nContexto de documentos similares en el workspace:\n"
		for i, snippet := range ragContext {
			if i >= 5 { // Limit to 5 examples
				break
			}
			snippetText := truncateContent(snippet, 300)
			contextInfo += fmt.Sprintf("[%d] %s\n\n", i+1, snippetText)
		}
		contextInfo += "Usa este contexto para mantener consistencia en nombres, fechas y lugares.\n"
	}

	// Use summary if available, otherwise use truncated content
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf(summarySectionFormat, summary)
	} else {
		contentSection = fmt.Sprintf("Contenido del documento (primeras 3000 palabras):\n%s\n", truncateContent(content, 3000))
	}

	// Detect language
	isSpanish := detectRomanceLanguage(content + summary)

	// Use template for contextual info extraction
	prompt := FormatExtractContextualInfo(isSpanish, contextInfo, contentSection)

	resp, err := r.Generate(ctx, GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   4000, // Increased for complex AIContext JSON with many fields
		Temperature: 0.3,  // Lower temperature for more structured output
	})
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

// ExtractContextualInfoParsed extracts contextual information and returns a parsed AIContext.
// fileLastModified is optional and used to validate publication year against file date.
func (r *Router) ExtractContextualInfoParsed(ctx context.Context, content string, summary string, description string, contextSnippets []string, fileLastModified *time.Time, languageCode ...string) (*entity.AIContext, error) {
	// Get RAG context snippets if available
	ragContext := contextSnippets
	if len(ragContext) == 0 {
		// If no context snippets provided, try to get them from RAG
		// This would require access to RAG service, but for now we'll use empty
		ragContext = []string{}
	}

	// Call the existing method
	jsonStr, err := r.ExtractContextualInfo(ctx, content, summary, ragContext)
	if err != nil {
		return nil, err
	}

	// Parse JSON to AIContext using robust parser with file date validation
	return parseAIContextJSON(jsonStr, r.logger, fileLastModified)
}
