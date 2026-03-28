// Package llm provides LLM router and provider implementations.
// This file contains prompt templates inspired by langchaingo patterns.
package llm

import (
	"fmt"
	"strings"
)

// SimplePromptTemplate is a simpler template system that doesn't require langchaingo.
// This is used as a fallback or for simpler cases.
type SimplePromptTemplate struct {
	template string
	variables []string
}

// NewSimplePromptTemplate creates a simple prompt template using Go's fmt.Sprintf style.
// Variables are specified as %s, %d, etc. in the template string.
func NewSimplePromptTemplate(template string) *SimplePromptTemplate {
	// Extract variable names from template (simple implementation)
	// For now, we'll use positional arguments
	return &SimplePromptTemplate{
		template: template,
	}
}

// Format formats the template with the given values.
// Values are passed in order they appear in the template.
func (spt *SimplePromptTemplate) Format(values ...interface{}) string {
	return fmt.Sprintf(spt.template, values...)
}

// PromptTemplateRegistry manages prompt templates for different use cases.
type PromptTemplateRegistry struct {
	templates map[string]*SimplePromptTemplate
}

// NewPromptTemplateRegistry creates a new template registry.
func NewPromptTemplateRegistry() *PromptTemplateRegistry {
	return &PromptTemplateRegistry{
		templates: make(map[string]*SimplePromptTemplate),
	}
}

// Register registers a template with a name.
func (r *PromptTemplateRegistry) Register(name string, template string) {
	r.templates[name] = NewSimplePromptTemplate(template)
}

// Get retrieves a template by name.
func (r *PromptTemplateRegistry) Get(name string) (*SimplePromptTemplate, bool) {
	template, ok := r.templates[name]
	return template, ok
}

// Format formats a template by name with the given values.
func (r *PromptTemplateRegistry) Format(name string, values ...interface{}) (string, error) {
	template, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}
	return template.Format(values...), nil
}

// Predefined prompt templates for common use cases
var (
	// TagSuggestionTemplate suggests tags for content
	TagSuggestionTemplate = `Analiza la siguiente información y sugiere hasta %d tags relevantes en español.
	Usa tags estilo slug: máximo 3 palabras unidas por guiones, sin espacios.
	Permite acentos, números y guiones. Evita duplicados y variantes del mismo concepto (singular/plural, años, sufijos).
	Evita puntuación y mantén cada tag en 32 caracteres o menos. Prioriza relevancia sobre cantidad.
	Si no hay tags claramente relevantes, responde [].
	Responde SOLO con un array JSON de strings de tags, nada más.

Resumen:
%s

Descripción:
%s

Tags (array JSON):`

	// ProjectSuggestionTemplate suggests a project for content
	ProjectSuggestionTemplate = `Eres un asistente experto en organización de documentos.

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

Proyecto sugerido:`

	// SummaryTemplate generates a summary
	SummaryTemplate = `Resume el siguiente contenido en %d palabras o menos.

INSTRUCCIONES:
1. Sé conciso y captura los puntos principales
2. Identifica: quién, qué, cuándo, dónde, por qué
3. Si es un documento religioso/teológico, menciona el tema espiritual principal
4. Si es un documento técnico, menciona la tecnología o metodología principal
5. El resumen DEBE estar en español, el mismo idioma que el contenido
6. Estructura el resumen en 2-3 párrafos, capturando:
   - Tema principal
   - Puntos clave
   - Contexto o relevancia

Contenido:
%s

Resumen:`

	// CategoryClassificationTemplate classifies content into categories
	CategoryClassificationTemplate = `Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- Ciencia y Tecnología
- Arte y Diseño
- Negocios y Finanzas
- Educación y Referencia
- Literatura y Escritura
- Documentación Técnica
- Recursos Humanos
- Marketing y Comunicación
- Legal y Regulatorio
- Salud y Medicina
- Religión y Teología
- Ingeniería y Construcción
- Investigación y Análisis
- Configuración y Administración
- Pruebas y Calidad
- Sin Clasificar

Resumen del documento:
%s

Descripción:
%s

IMPORTANTE: Analiza cuidadosamente el tema principal del documento. Si menciona términos religiosos, teológicos, místicos, vidas de santos, o experiencias espirituales, la categoría correcta es "Religión y Teología".

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).`
)

// FormatTagSuggestion formats the tag suggestion template.
func FormatTagSuggestion(maxTags int, summary, description string) string {
	return fmt.Sprintf(TagSuggestionTemplate, maxTags, summary, description)
}

// FormatProjectSuggestion formats the project suggestion template.
func FormatProjectSuggestion(projectList, contentSection string) string {
	return fmt.Sprintf(ProjectSuggestionTemplate, projectList, contentSection)
}

// FormatSummary formats the summary template.
func FormatSummary(maxLength int, content string) string {
	return fmt.Sprintf(SummaryTemplate, maxLength/5, truncateContent(content, 3000))
}

// FormatCategoryClassification formats the category classification template.
func FormatCategoryClassification(summary, description string) string {
	return fmt.Sprintf(CategoryClassificationTemplate, summary, description)
}

// ExtractContextualInfoTemplate extracts contextual information from documents
var ExtractContextualInfoTemplateES = `Eres un experto bibliotecario y archivista. Analiza el siguiente documento y extrae TODA la información contextual relevante en formato JSON.

INSTRUCCIONES:
1. Extrae información sobre autores, editores, traductores, contribuidores
2. Identifica editorial, año de publicación, lugar de publicación
3. Extrae fechas importantes, períodos históricos mencionados
4. Identifica lugares geográficos relevantes
5. Lista personas importantes mencionadas (con su rol)
6. Identifica organizaciones, instituciones mencionadas
7. Extrae eventos históricos mencionados
8. Identifica referencias bibliográficas si las hay
9. Detecta idioma original si es una traducción
10. Identifica género literario, tema, audiencia

FORMATO DE RESPUESTA (JSON estricto):
{
  "authors": [{"name": "Nombre completo", "role": "autor|co-autor|contribuidor", "affiliation": "institución si aplica"}],
  "editors": ["Editor 1", "Editor 2"],
  "translators": ["Traductor 1"],
  "contributors": ["Contribuidor 1"],
  "publisher": "Nombre de la editorial",
  "publication_year": 2024,
  "publication_place": "Ciudad, País (formato: Ciudad, País)",
  "isbn": "ISBN-10 o ISBN-13 sin guiones (ej: 9781234567890)",
  "issn": "ISSN en formato XXXX-XXXX (ej: 1234-5678)",
  "document_date": "YYYY-MM-DD o null",
  "historical_period": "Período histórico si aplica (puede ser string o array, ej: \"Medieval\" o [\"Medieval\", \"Renacimiento\"])",
  "locations": [{"name": "Lugar", "type": "ciudad|país|región|estado|provincia|continente|municipio", "context": "cómo es relevante"}],
  "people_mentioned": [{"name": "Nombre", "role": "rol (santo, científico, etc.)", "context": "cómo se menciona"}],
  "organizations": [{"name": "Organización", "type": "iglesia|universidad|gobierno|empresa|organización|institución|asociación|fundación|ong", "context": "relevancia"}],
  "historical_events": [{"name": "Evento", "date": "YYYY-MM-DD o null", "location": "Lugar (o null si no aplica)", "context": "cómo se menciona"}],
  "references": [{"title": "Título", "author": "Autor", "year": 2024, "type": "libro|artículo|sitio web|conferencia|tesis|informe|documento|papel|otro"}],
  "original_language": "idioma original si es traducción",
  "genre": "género literario",
  "subject": "tema principal",
  "audience": "audiencia objetivo"
}

IMPORTANTE:
- Responde SOLO con JSON válido, sin texto adicional
- Usa null para campos no disponibles
- Si no encuentras información para un campo, usa null o array vacío
- Sé preciso con fechas y nombres
- NUNCA uses placeholders genéricos como "Traductor 1", "Contribuidor 1", "Editor 1", etc.
- Si no conoces el nombre real, usa null o array vacío en lugar de placeholders
- Para organizaciones: si todos los campos son "null", usa array vacío en lugar de [{"name": "null", ...}]
- Para ISBN/ISSN: usa solo números, sin guiones ni espacios (se normalizarán automáticamente)
- Para períodos históricos: puedes usar string simple o array si hay múltiples períodos
- NO dupliques información: si alguien es autor, no lo incluyas también como contributor
- NO dupliques ubicaciones, personas, eventos o referencias
- Usa el contexto de documentos similares para mantener consistencia%s

%s

Información contextual (JSON):`

var ExtractContextualInfoTemplateEN = `You are an expert librarian and archivist. Analyze the following document and extract ALL relevant contextual information in JSON format.

INSTRUCTIONS:
1. Extract information about authors, editors, translators, contributors
2. Identify publisher, publication year, publication place
3. Extract important dates, historical periods mentioned
4. Identify relevant geographic locations
5. List important people mentioned (with their role)
6. Identify organizations, institutions mentioned
7. Extract historical events mentioned
8. Identify bibliographic references if any
9. Detect original language if it's a translation
10. Identify literary genre, subject, audience

RESPONSE FORMAT (strict JSON):
{
  "authors": [{"name": "Full name", "role": "author|co-author|contributor", "affiliation": "institution if applicable"}],
  "editors": ["Editor 1", "Editor 2"],
  "translators": ["Translator 1"],
  "contributors": ["Contributor 1"],
  "publisher": "Publisher name",
  "publication_year": 2024,
  "publication_place": "City, Country (format: City, Country)",
  "isbn": "ISBN-10 or ISBN-13 without hyphens (e.g., 9781234567890)",
  "issn": "ISSN in format XXXX-XXXX (e.g., 1234-5678)",
  "document_date": "YYYY-MM-DD or null",
  "historical_period": "Historical period if applicable (can be string or array, e.g., \"Medieval\" or [\"Medieval\", \"Renaissance\"])",
  "locations": [{"name": "Place", "type": "city|country|region|state|province|continent|municipality", "context": "how it's relevant"}],
  "people_mentioned": [{"name": "Name", "role": "role (saint, scientist, etc.)", "context": "how mentioned"}],
  "organizations": [{"name": "Organization", "type": "church|university|government|company|organization|institution|association|foundation|npo", "context": "relevance"}],
  "historical_events": [{"name": "Event", "date": "YYYY-MM-DD or null", "location": "Place (or null if not applicable)", "context": "how mentioned"}],
  "references": [{"title": "Title", "author": "Author", "year": 2024, "type": "book|article|website|conference|thesis|report|document|paper|other"}],
  "original_language": "original language if translation",
  "genre": "literary genre",
  "subject": "main subject",
  "audience": "target audience"
}

IMPORTANT:
- Respond ONLY with valid JSON, no additional text
- Use null for unavailable fields
- If you don't find information for a field, use null or empty array
- Be precise with dates and names
- NEVER use generic placeholders like "Translator 1", "Contributor 1", "Editor 1", etc.
- If you don't know the real name, use null or empty array instead of placeholders
- For organizations: if all fields are "null", use empty array instead of [{"name": "null", ...}]
- For ISBN/ISSN: use only numbers, without hyphens or spaces (will be normalized automatically)
- For historical periods: you can use simple string or array if there are multiple periods
- DO NOT duplicate information: if someone is an author, do not include them also as contributor
- DO NOT duplicate locations, people, events, or references
- Use context from similar documents to maintain consistency%s

%s

Contextual information (JSON):`

// FormatExtractContextualInfo formats the contextual info extraction template
func FormatExtractContextualInfo(isSpanish bool, contextInfo, contentSection string) string {
	if isSpanish {
		return fmt.Sprintf(ExtractContextualInfoTemplateES, contextInfo, contentSection)
	}
	return fmt.Sprintf(ExtractContextualInfoTemplateEN, contextInfo, contentSection)
}

// ClassifyCategoryTemplate classifies documents into library categories
var ClassifyCategoryTemplate = `Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- %s%s

%s

IMPORTANTE: Analiza cuidadosamente el tema principal del documento. Si menciona términos religiosos, teológicos, místicos, vidas de santos, o experiencias espirituales, la categoría correcta es "Religión y Teología".

Ejemplos de clasificación:
- Documento sobre "vidas de santos y experiencias místicas" → Religión y Teología
- Documento sobre "manual de usuario de software" → Documentación Técnica
- Documento sobre "análisis de mercado" → Investigación y Análisis
- Documento sobre "enciclopedia o diccionario" → Educación y Referencia

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).
Si no puedes determinar la categoría, responde: "Sin Clasificar"`

// FormatClassifyCategory formats the category classification template
func FormatClassifyCategory(categoryList, contextInfo, contentSection string) string {
	return fmt.Sprintf(ClassifyCategoryTemplate, categoryList, contextInfo, contentSection)
}

// FindRelatedFilesTemplate finds related files in workspace
var FindRelatedFilesTemplate = `You are analyzing a file to find related files in the workspace.

Content preview (first 1000 chars):
%s

Available files in workspace:
  - %s

Based on the current file's content, identify files that are likely related or work together with it.
Consider:
- Import/export relationships
- Similar functionality or domain
- Test files for implementation files
- Configuration files
- Documentation

Return ONLY a JSON array of file paths (relative paths from the list above), nothing else.
Example: ["src/auth/login.ts", "src/auth/types.ts", "test/auth.test.ts"]
Maximum %d files.`

// FormatFindRelatedFiles formats the find related files template
func FormatFindRelatedFiles(content string, candidateList []string, maxResults int) string {
	truncatedContent := truncateContent(content, 1000)
	return fmt.Sprintf(FindRelatedFilesTemplate, truncatedContent, strings.Join(candidateList, "\n  - "), maxResults)
}

// DetectLanguageTemplate detects the primary language of text
var DetectLanguageTemplate = `Detect the primary language of the following text content.
Return ONLY the ISO 639-1 language code (2 letters) in lowercase.
Examples: "es" for Spanish, "en" for English, "fr" for French, "de" for German, "pt" for Portuguese, "it" for Italian, "ru" for Russian, "zh" for Chinese, "ja" for Japanese, "ko" for Korean.

Content:
%s

Language code:`

// FormatDetectLanguage formats the language detection template
func FormatDetectLanguage(content string) string {
	contentSample := truncateContent(content, 1000)
	return fmt.Sprintf(DetectLanguageTemplate, contentSample)
}

// truncateContent truncates content to maxLen characters, adding ellipsis if needed
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
