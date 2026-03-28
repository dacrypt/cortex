package metadata

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/rs/zerolog"
)

// SuggestionService generates metadata suggestions using RAG and LLM.
type SuggestionService struct {
	ragService  RAGService
	llmService  LLMService
	metaRepo    repository.MetadataRepository
	docRepo     repository.DocumentRepository
	projectRepo repository.ProjectRepository
	logger      zerolog.Logger
}

// RAGService interface for RAG operations.
type RAGService interface {
	Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error)
}

// RAGQueryRequest represents a RAG query request.
type RAGQueryRequest struct {
	WorkspaceID    entity.WorkspaceID
	Query          string
	TopK           int
	GenerateAnswer bool
}

// RAGQueryResponse contains RAG query results.
type RAGQueryResponse struct {
	Answer  string
	Sources []RAGSource
}

// RAGSource represents a retrieved document chunk.
type RAGSource struct {
	DocumentID   entity.DocumentID
	ChunkID      entity.ChunkID
	RelativePath string
	HeadingPath  string
	Snippet      string
	Score        float32
}

// LLMService interface for LLM operations.
type LLMService interface {
	Generate(ctx context.Context, prompt string, maxTokens int) (string, error)
}

// NewSuggestionService creates a new suggestion service.
func NewSuggestionService(
	ragService RAGService,
	llmService LLMService,
	metaRepo repository.MetadataRepository,
	docRepo repository.DocumentRepository,
	projectRepo repository.ProjectRepository,
	logger zerolog.Logger,
) *SuggestionService {
	if ragService == nil || llmService == nil {
		logger.Warn().Msg("RAG or LLM service not available, suggestions will be limited")
	}
	return &SuggestionService{
		ragService:  ragService,
		llmService:  llmService,
		metaRepo:    metaRepo,
		docRepo:     docRepo,
		projectRepo: projectRepo,
		logger:      logger.With().Str("component", "suggestion_service").Logger(),
	}
}

// GenerateSuggestions generates comprehensive metadata suggestions for a file.
func (s *SuggestionService) GenerateSuggestions(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	fileMeta *entity.FileMetadata,
) (*entity.SuggestedMetadata, error) {
	if entry == nil {
		return nil, fmt.Errorf("file entry is required")
	}

	suggested := &entity.SuggestedMetadata{
		FileID:          entry.ID,
		WorkspaceID:     workspaceID,
		RelativePath:    entry.RelativePath,
		SuggestedFields: make(map[string]entity.SuggestedField),
	}

	// Get document content for RAG/LLM analysis
	doc, err := s.docRepo.GetDocumentByPath(ctx, workspaceID, entry.RelativePath)
	if err != nil {
		s.logger.Debug().Err(err).Str("path", entry.RelativePath).Msg("Document not found, using file metadata only")
	}

	// Generate suggestions in parallel
	// 1. Taxonomic classification
	if taxonomy, err := s.generateTaxonomy(ctx, workspaceID, entry, doc); err == nil {
		suggested.SuggestedTaxonomy = taxonomy
	}

	// 2. Tag suggestions
	if tags, err := s.generateTags(ctx, workspaceID, entry, doc, fileMeta); err == nil {
		suggested.SuggestedTags = tags
	}

	// 3. Project suggestions
	if projects, err := s.generateProjects(ctx, workspaceID, entry, doc, fileMeta); err == nil {
		suggested.SuggestedProjects = projects
	}

	// 4. Additional metadata field suggestions
	if fields, err := s.generateFields(ctx, workspaceID, entry, doc, fileMeta); err == nil {
		suggested.SuggestedFields = fields
	}

	// Calculate overall confidence
	suggested.Confidence = s.calculateOverallConfidence(suggested)
	suggested.Source = "rag_llm"

	return suggested, nil
}

// generateTaxonomy generates taxonomic classifications.
func (s *SuggestionService) generateTaxonomy(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	doc *entity.Document,
) (*entity.SuggestedTaxonomy, error) {
	// Build context from file metadata
	contextParts := []string{
		fmt.Sprintf("File: %s", entry.Filename),
		fmt.Sprintf("Type: %s", entry.Extension),
	}

	if entry.Enhanced != nil {
		if entry.Enhanced.DocumentMetrics != nil {
			if entry.Enhanced.DocumentMetrics.Title != nil {
				contextParts = append(contextParts, fmt.Sprintf("Title: %s", *entry.Enhanced.DocumentMetrics.Title))
			}
			if entry.Enhanced.DocumentMetrics.Author != nil {
				contextParts = append(contextParts, fmt.Sprintf("Author: %s", *entry.Enhanced.DocumentMetrics.Author))
			}
		}
		if entry.Enhanced.MimeType != nil {
			contextParts = append(contextParts, fmt.Sprintf("MIME: %s", entry.Enhanced.MimeType.MimeType))
		}
	}

	// Add document content summary if available (helps with classification)
	if doc != nil {
		// Get document chunks to build content
		chunks, err := s.docRepo.GetChunksByDocument(ctx, workspaceID, doc.ID)
		if err == nil && len(chunks) > 0 {
			// Build content from first few chunks (limit to ~500 chars)
			contentSample := ""
			for _, chunk := range chunks {
				if len(contentSample)+len(chunk.Text) > 500 {
					// Add partial chunk to reach ~500 chars
					remaining := 500 - len(contentSample)
					if remaining > 0 {
						contentSample += chunk.Text[:remaining] + "..."
					}
					break
				}
				contentSample += chunk.Text + "\n\n"
			}
			if contentSample != "" {
				contextParts = append(contextParts, fmt.Sprintf("Content sample: %s", contentSample))
			}
		}
	}

	// Use RAG to find similar files and their classifications
	query := fmt.Sprintf("What is the category, domain, and content type of files similar to: %s", entry.Filename)
	ragResponse, err := s.ragService.Query(ctx, RAGQueryRequest{
		WorkspaceID:    workspaceID,
		Query:          query,
		TopK:           5,
		GenerateAnswer: false,
	})
	if err != nil {
		s.logger.Debug().Err(err).Msg("RAG query failed, using LLM only")
		ragResponse = nil
	}

	// Build LLM prompt
	prompt := s.buildTaxonomyPrompt(contextParts, ragResponse, doc)

	// Generate with LLM
	response, err := s.llmService.Generate(ctx, prompt, 500)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse LLM response (JSON format expected)
	taxonomy := s.parseTaxonomyResponse(response)
	if taxonomy == nil {
		return nil, fmt.Errorf("failed to parse taxonomy response")
	}

	return taxonomy, nil
}

// generateTags generates tag suggestions.
func (s *SuggestionService) generateTags(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	doc *entity.Document,
	fileMeta *entity.FileMetadata,
) ([]entity.SuggestedTag, error) {
	// Use RAG to find similar files and their tags
	query := fmt.Sprintf("What tags are used for files similar to: %s", entry.Filename)
	ragResponse, err := s.ragService.Query(ctx, RAGQueryRequest{
		WorkspaceID:    workspaceID,
		Query:          query,
		TopK:           10,
		GenerateAnswer: false,
	})
	if err != nil {
		s.logger.Debug().Err(err).Msg("RAG query failed for tags")
		ragResponse = nil
	}

	// Extract tags from similar files
	tagFrequency := make(map[string]int)
	if ragResponse != nil {
		for _, source := range ragResponse.Sources {
			// Get metadata for similar file
			similarMeta, err := s.metaRepo.GetByPath(ctx, workspaceID, source.RelativePath)
			if err == nil && similarMeta != nil {
				for _, tag := range similarMeta.Tags {
					normalized := entity.NormalizeTag(tag)
					if normalized != "" && !entity.IsTagGeneric(normalized) {
						tagFrequency[normalized]++
					}
				}
			}
		}
	}

	// Build LLM prompt
	prompt := s.buildTagPrompt(entry, doc, fileMeta, tagFrequency)

	// Generate with LLM
	response, err := s.llmService.Generate(ctx, prompt, 300)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse response
	tags := s.parseTagResponse(response, tagFrequency)

	return tags, nil
}

// generateProjects generates project suggestions.
func (s *SuggestionService) generateProjects(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	doc *entity.Document,
	fileMeta *entity.FileMetadata,
) ([]entity.SuggestedProject, error) {
	// Use RAG to find similar files and their projects
	query := fmt.Sprintf("What projects contain files similar to: %s", entry.Filename)
	ragResponse, err := s.ragService.Query(ctx, RAGQueryRequest{
		WorkspaceID:    workspaceID,
		Query:          query,
		TopK:           10,
		GenerateAnswer: false,
	})
	if err != nil {
		s.logger.Debug().Err(err).Msg("RAG query failed for projects")
		ragResponse = nil
	}

	// Get existing projects
	existingProjects, err := s.projectRepo.List(ctx, workspaceID)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to list existing projects")
	}

	// Build LLM prompt
	prompt := s.buildProjectPrompt(entry, doc, fileMeta, ragResponse, existingProjects)

	// Generate with LLM
	response, err := s.llmService.Generate(ctx, prompt, 400)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse response
	projects := s.parseProjectResponse(response, existingProjects)

	return projects, nil
}

// generateFields generates suggestions for additional metadata fields.
func (s *SuggestionService) generateFields(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	entry *entity.FileEntry,
	doc *entity.Document,
	fileMeta *entity.FileMetadata,
) (map[string]entity.SuggestedField, error) {
	fields := make(map[string]entity.SuggestedField)

	// Suggest fields based on file type and content
	if entry.Enhanced != nil && entry.Enhanced.DocumentMetrics != nil {
		dm := entry.Enhanced.DocumentMetrics

		// Suggest missing fields
		if dm.Title == nil && doc != nil {
			// Extract title from document content
			if title := s.extractTitleFromContent(ctx, workspaceID, doc); title != "" {
				fields["title"] = entity.SuggestedField{
					FieldName:  "title",
					Value:      title,
					Confidence: 0.7,
					Reason:     "Extracted from document content",
					Source:     "content_analysis",
					FieldType:  "string",
				}
			}
		}

		if dm.Author == nil {
			// Try to infer from path or similar files
			if author := s.inferAuthor(ctx, workspaceID, entry, doc); author != "" {
				fields["author"] = entity.SuggestedField{
					FieldName:  "author",
					Value:      author,
					Confidence: 0.5,
					Reason:     "Inferred from file context",
					Source:     "pattern_matching",
					FieldType:  "string",
				}
			}
		}
	}

	return fields, nil
}

// Helper methods (stubs - implement with actual logic)

func (s *SuggestionService) buildTaxonomyPrompt(contextParts []string, ragResponse *RAGQueryResponse, doc *entity.Document) string {
	// Build RAG context section if available
	ragContextInfo := ""
	if ragResponse != nil && len(ragResponse.Sources) > 0 {
		// Extract taxonomies from similar files
		taxonomyInfo := make([]string, 0)
		for i, source := range ragResponse.Sources {
			if i >= 3 { // Limit to 3 examples
				break
			}
			// Try to get metadata for similar file to extract taxonomy
			// For now, we'll use the source path as context
			if source.RelativePath != "" {
				taxonomyInfo = append(taxonomyInfo, fmt.Sprintf("- Archivo similar: %s", source.RelativePath))
			}
		}

		if len(taxonomyInfo) > 0 {
			ragContextInfo = fmt.Sprintf(`

⚠️ INFORMACIÓN CRÍTICA DE ARCHIVOS SIMILARES:
Se encontraron archivos similares en el workspace:
%s

USA ESTA INFORMACIÓN como señal principal para clasificar este documento.
Si los archivos similares tienen taxonomías consistentes, usa la misma clasificación.`, strings.Join(taxonomyInfo, "\n"))
		}
	}

	// Build comprehensive prompt for taxonomy classification
	// Use very strict format to force JSON-only response
	return fmt.Sprintf(`You are a JSON API. Return ONLY valid JSON, no other text.

EXAMPLE OF CORRECT RESPONSE:
{"category":"document","subcategory":"book","domain":"academic","contentType":"book","topic":["religion"],"language":"es","reasoning":"Religious academic document"}

REQUIRED FIELDS:
{
  "category": "document|code|media|data",
  "subcategory": "string (e.g., book, article, manual, spreadsheet, image, video)",
  "domain": "software|business|personal|academic",
  "subdomain": "string",
  "contentType": "specification|report|source-code|presentation|book|article|manual|spreadsheet|image|video|audio|data|config|documentation|email|note|receipt|invoice|list|contract|letter|form|memo|proposal|plan|policy|procedure|standard|guideline|checklist|template|questionnaire|survey|application|resume|cv|certificate|license|permit|warranty|guarantee|agreement|lease|deed|will|testament|patent|trademark|copyright|legal-brief|case-study|white-paper|research-paper|thesis|dissertation|journal-article|conference-paper|technical-drawing|blueprint|schematic|diagram|chart|graph|map|timeline|calendar|schedule|budget|financial-statement|balance-sheet|income-statement|cash-flow|tax-return|bank-statement|credit-report|insurance-policy|claim|medical-record|prescription|lab-result|x-ray|scan|diagnosis|treatment-plan|patient-chart|educational-material|lesson-plan|syllabus|curriculum|exam|quiz|assignment|homework|grade-book|transcript|diploma|degree|certificate-of-completion|training-material|workshop-material|seminar-material|webinar-material|tutorial|how-to-guide|faq|knowledge-base|wiki|blog-post|newsletter|press-release|announcement|advertisement|brochure|flyer|poster|banner|catalog|price-list|product-sheet|datasheet|spec-sheet|user-manual|technical-manual|installation-guide|troubleshooting-guide|api-documentation|code-documentation|readme|changelog|release-notes|license-agreement|terms-of-service|privacy-policy|cookie-policy|disclaimer|disclosure|compliance-report|audit-report|risk-assessment|security-report|incident-report|accident-report|inspection-report|test-report|quality-report|performance-report|progress-report|status-report|meeting-minutes|agenda|action-items|decision-record|project-plan|project-charter|requirements-document|design-document|architecture-document|test-plan|test-case|bug-report|feature-request|change-request|code-review|pull-request|commit-message|database-schema|data-model|er-diagram|workflow-diagram|process-map|organizational-chart|org-chart|job-description|job-posting|performance-review|employee-evaluation|payroll|timesheet|expense-report|travel-request|purchase-order|purchase-request|vendor-proposal|rfp|rfq|quote|estimate|bid|sow|msa|nda|mou|loi|po|so|credit-note|debit-note|payment-receipt|payment-confirmation|bank-transfer|wire-transfer|check|money-order|gift-card|voucher|coupon|discount-code|promo-code|loyalty-card|membership-card|id-card|badge|passport|visa|driver-license|social-security-card|birth-certificate|marriage-certificate|death-certificate|divorce-decree|adoption-papers|custody-agreement|power-of-attorney|living-will|healthcare-proxy|insurance-card|medical-id|prescription-label|medication-list|allergy-list|immunization-record|vaccination-record|travel-itinerary|boarding-pass|hotel-reservation|car-rental|flight-ticket|train-ticket|bus-ticket|event-ticket|concert-ticket|movie-ticket|sports-ticket|museum-ticket|parking-ticket|traffic-ticket|citation|subpoena|summons|warrant|court-order|judgment|settlement|release|waiver|affidavit|deposition|testimony|exhibit|evidence|forensic-report|police-report|incident-log|dispatch-log|arrest-record|criminal-record|background-check|credit-check|reference-check|employment-verification|education-verification|identity-verification|address-verification|phone-verification|email-verification|two-factor-code|otp|password-reset|account-activation|welcome-email|confirmation-email|notification-email|alert-email|reminder-email|follow-up-email|thank-you-email|apology-email|complaint-email|feedback-email|survey-email|newsletter-email|marketing-email|promotional-email|transactional-email|system-email|automated-email|spam-email|phishing-email|scam-email|malware-email|other",
  "purpose": "reference|working-draft|final|archive",
  "topic": ["string"],
  "audience": "internal|external|public|private",
  "language": "en|es|fr",
  "reasoning": "string"
}

CONTENT TYPE GUIDELINES (select the MOST APPROPRIATE type based on file content):

**Documents & Text:** specification, report, book, article, manual, presentation, documentation, letter, memo, note, email, form, template, list
**Business & Legal:** contract, agreement, lease, invoice, receipt, quote, proposal, purchase-order, policy, procedure, standard, guideline, compliance-report, audit-report, legal-brief, will, patent, trademark, copyright
**Financial:** financial-statement, balance-sheet, income-statement, cash-flow, tax-return, bank-statement, credit-report, budget, expense-report, payroll, insurance-policy, claim, payment-receipt, check
**Medical & Health:** medical-record, prescription, lab-result, x-ray, scan, diagnosis, treatment-plan, patient-chart, immunization-record, medication-list, allergy-list
**Education:** educational-material, lesson-plan, syllabus, curriculum, exam, quiz, assignment, homework, grade-book, transcript, diploma, degree, certificate-of-completion, training-material, tutorial, how-to-guide
**Research & Academic:** research-paper, thesis, dissertation, journal-article, conference-paper, white-paper, case-study, faq, knowledge-base, wiki
**Marketing & Communication:** blog-post, newsletter, press-release, announcement, advertisement, brochure, flyer, poster, banner, catalog, price-list, product-sheet, datasheet
**Technical & Development:** source-code, config, api-documentation, code-documentation, readme, changelog, release-notes, requirements-document, design-document, architecture-document, test-plan, test-case, bug-report, feature-request, code-review, pull-request, database-schema, data-model, technical-drawing, blueprint, schematic, diagram, chart, graph, troubleshooting-guide, installation-guide, user-manual, technical-manual
**Project Management:** project-plan, project-charter, meeting-minutes, agenda, action-items, decision-record, progress-report, status-report, performance-report, quality-report, test-report, incident-report, security-report
**Data & Analytics:** spreadsheet, data, map, timeline, calendar, schedule
**Media:** image, video, audio
**Personal & Identification:** resume, cv, certificate, license, permit, warranty, guarantee, id-card, badge, passport, visa, driver-license, social-security-card, birth-certificate, marriage-certificate, power-of-attorney, insurance-card
**Travel & Events:** travel-itinerary, boarding-pass, hotel-reservation, car-rental, flight-ticket, train-ticket, bus-ticket, event-ticket, concert-ticket, movie-ticket, sports-ticket, museum-ticket, parking-ticket, traffic-ticket
**Legal & Court:** citation, subpoena, summons, warrant, court-order, judgment, settlement, release, waiver, affidavit, deposition, testimony, exhibit, evidence, forensic-report, police-report, arrest-record, criminal-record
**Verification & Security:** background-check, credit-check, reference-check, employment-verification, education-verification, identity-verification, two-factor-code, otp, password-reset, account-activation
**Email Types (if email content):** welcome-email, confirmation-email, notification-email, alert-email, reminder-email, thank-you-email, complaint-email, feedback-email, newsletter-email, marketing-email, promotional-email, transactional-email, system-email, spam-email, phishing-email, scam-email, malware-email
**Other:** other (use as last resort for anything not covered above)

IMPORTANT: Analyze the file content, name, and type to determine the most appropriate contentType. Do NOT default to "specification" unless the file is actually a specification document.

FORBIDDEN: NO markdown, NO explanations, NO text before or after JSON. ONLY the JSON object.%s

File: %s

Response (JSON only):`, ragContextInfo, strings.Join(contextParts, "\n"))
}

func (s *SuggestionService) parseTaxonomyResponse(response string) *entity.SuggestedTaxonomy {
	// Use robust JSON parser with retry capability
	parser := llm.NewJSONParser(s.logger)
	ctx := context.Background()

	// Parse JSON response from LLM
	var raw struct {
		Category    string   `json:"category"`
		Subcategory string   `json:"subcategory"`
		Domain      string   `json:"domain"`
		Subdomain   string   `json:"subdomain"`
		ContentType string   `json:"contentType"`
		Purpose     string   `json:"purpose"`
		Topic       []string `json:"topic"`
		Audience    string   `json:"audience"`
		Language    string   `json:"language"`
		Reasoning   string   `json:"reasoning"`
	}

	if err := parser.ParseJSON(ctx, response, &raw); err != nil {
		s.logger.Warn().Err(err).Str("response", truncateString(response, 200)).Msg("Failed to parse taxonomy JSON, using defaults")
		// Return default taxonomy if parsing fails
		return &entity.SuggestedTaxonomy{
			Category:              "document",
			Domain:                "general",
			ContentType:           "document",
			CategoryConfidence:    0.5, // Lower confidence for fallback
			DomainConfidence:      0.5,
			ContentTypeConfidence: 0.5,
			Source:                "llm_fallback",
		}
	}

	// Calculate confidence based on completeness of response
	confidence := 0.7
	if raw.Category != "" && raw.Domain != "" && raw.ContentType != "" {
		confidence = 0.8
	}
	if raw.Reasoning != "" {
		confidence += 0.1 // Higher confidence if reasoning is provided
		if confidence > 0.95 {
			confidence = 0.95
		}
	}

	return &entity.SuggestedTaxonomy{
		Category:              raw.Category,
		Subcategory:           raw.Subcategory,
		Domain:                raw.Domain,
		Subdomain:             raw.Subdomain,
		ContentType:           raw.ContentType,
		Purpose:               raw.Purpose,
		Topic:                 raw.Topic,
		Audience:              raw.Audience,
		Language:              raw.Language,
		CategoryConfidence:    confidence,
		DomainConfidence:      confidence,
		ContentTypeConfidence: confidence,
		Reasoning:             raw.Reasoning,
		Source:                "llm",
	}
}

func (s *SuggestionService) buildTagPrompt(entry *entity.FileEntry, doc *entity.Document, fileMeta *entity.FileMetadata, tagFreq map[string]int) string {
	// Build summary and description from metadata
	summary := ""
	description := ""
	if fileMeta != nil {
		if fileMeta.AISummary != nil {
			summary = fileMeta.AISummary.Summary
		}
		if len(fileMeta.Tags) > 0 {
			description = strings.Join(fileMeta.Tags, ", ")
		}
	}

	// Build content section
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf("Resumen:\n%s\n", summary)
		if description != "" {
			contentSection += fmt.Sprintf("Descripción:\n%s\n", description)
		}
	} else {
		contentSection = fmt.Sprintf("Archivo: %s\nTipo: %s\n", entry.Filename, entry.Extension)
	}

	type tagCount struct {
		tag   string
		count int
	}
	tagCounts := make([]tagCount, 0, len(tagFreq))
	for tag, count := range tagFreq {
		if entity.IsTagGeneric(tag) {
			continue
		}
		tagCounts = append(tagCounts, tagCount{tag: tag, count: count})
	}
	sort.Slice(tagCounts, func(i, j int) bool {
		if tagCounts[i].count == tagCounts[j].count {
			return tagCounts[i].tag < tagCounts[j].tag
		}
		return tagCounts[i].count > tagCounts[j].count
	})

	// Build context from similar files (top N)
	contextInfo := ""
	if len(tagCounts) > 0 {
		limit := 10
		if len(tagCounts) < limit {
			limit = len(tagCounts)
		}
		tags := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			tags = append(tags, tagCounts[i].tag)
		}
		contextInfo = fmt.Sprintf("\n\nTags comunes en archivos similares del workspace:\n- %s\n\nConsidera estos tags como referencia para mantener consistencia.", strings.Join(tags, "\n- "))
	}

	// Build few-shot examples with most frequent tags
	examplesSection := ""
	if len(tagCounts) > 0 {
		topTags := make([]string, 0, 3)
		for i := 0; i < len(tagCounts) && i < 3; i++ {
			topTags = append(topTags, tagCounts[i].tag)
		}
		if len(topTags) > 0 {
			examplesSection = fmt.Sprintf(`

EJEMPLOS DE BUENOS TAGS (de archivos similares):
- %s

Usa estos como referencia para mantener consistencia en el estilo y formato.`, strings.Join(topTags, "\n- "))
		}
	}

	return fmt.Sprintf(`Analiza la siguiente información y sugiere hasta 5 tags relevantes en español.
	Usa tags estilo slug: máximo 3 palabras unidas por guiones, sin espacios.
	Permite acentos, números y guiones. Evita duplicados y variantes del mismo concepto (singular/plural, años, sufijos).
	Evita puntuación y mantén cada tag en 32 caracteres o menos. Prioriza relevancia sobre cantidad.
	Si no hay tags claramente relevantes, responde [].
	Responde SOLO con un array JSON de strings de tags, nada más.%s%s

%s

Tags (array JSON):`, contextInfo, examplesSection, contentSection)
}

func (s *SuggestionService) parseTagResponse(response string, tagFreq map[string]int) []entity.SuggestedTag {
	// Use robust ArrayParser with automatic fallback
	parser := llm.NewArrayParser(s.logger)
	ctx := context.Background()

	// Parse array (handles JSON arrays and comma-separated lists)
	tags, err := parser.ParseArray(ctx, response)
	if err != nil {
		s.logger.Warn().Err(err).Str("response", truncateString(response, 200)).Msg("Failed to parse tag response, trying fallback")
		// Fallback: try to extract tags manually
		tags = s.extractTagsFromText(response)
	}

	return s.convertTagsToSuggested(tags, tagFreq)
}

// convertTagsToSuggested converts tag strings to SuggestedTag entities.
func (s *SuggestionService) convertTagsToSuggested(tags []string, tagFreq map[string]int) []entity.SuggestedTag {
	// Convert to SuggestedTag with confidence scores
	result := make([]entity.SuggestedTag, 0, len(tags))
	parsedCount := 0
	skippedCount := 0

	for _, tag := range tags {
		rawTag := strings.TrimSpace(tag)
		rawTag = strings.Trim(rawTag, `"'`)

		// Validate tag format
		if rawTag == "" {
			skippedCount++
			continue
		}
		// Reject tags that look like code
		if strings.Contains(rawTag, "(") && strings.Contains(rawTag, ")") && strings.Contains(rawTag, "=") {
			s.logger.Debug().Str("tag", rawTag).Msg("Tag looks like code, skipping")
			skippedCount++
			continue
		}

		normalized := entity.NormalizeTag(rawTag)
		if normalized == "" {
			s.logger.Debug().Str("tag", rawTag).Msg("Tag did not normalize, skipping")
			skippedCount++
			continue
		}
		if entity.IsTagGeneric(normalized) {
			s.logger.Debug().Str("tag", normalized).Msg("Tag too generic, skipping")
			skippedCount++
			continue
		}
		if isTagSimilarToAny(normalized, result) {
			s.logger.Debug().Str("tag", normalized).Msg("Tag similar to existing suggestion, skipping")
			skippedCount++
			continue
		}

		// Calculate confidence based on frequency in similar files
		confidence := 0.7 // Default confidence
		if freq, ok := tagFreq[normalized]; ok {
			// Higher frequency = higher confidence (capped at 0.95)
			confidence = 0.7 + float64(freq)*0.05
			if confidence > 0.95 {
				confidence = 0.95
			}
		}

		result = append(result, entity.SuggestedTag{
			Tag:        normalized,
			Confidence: confidence,
			Source:     "llm",
		})
		parsedCount++
	}

	// Log parsing metrics
	if skippedCount > 0 {
		s.logger.Info().
			Int("parsed", parsedCount).
			Int("skipped", skippedCount).
			Int("total", len(tags)).
			Msg("Tag parsing completed with some skipped tags")
	} else {
		s.logger.Debug().
			Int("parsed", parsedCount).
			Msg("Tag parsing completed successfully")
	}

	return result
}

// extractTagsFromText extracts tags from text when JSON parsing fails.
func (s *SuggestionService) extractTagsFromText(text string) []string {
	// Try to find array-like structure
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		text = text[start+1 : end]
	}

	// Split by comma and clean
	parts := strings.Split(text, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		tag = strings.Trim(tag, `"'[]{}`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func isTagSimilarToAny(tag string, existing []entity.SuggestedTag) bool {
	for _, candidate := range existing {
		if entity.AreTagsSimilar(tag, candidate.Tag) {
			return true
		}
	}
	return false
}

func (s *SuggestionService) buildProjectPrompt(entry *entity.FileEntry, doc *entity.Document, fileMeta *entity.FileMetadata, ragResponse *RAGQueryResponse, existingProjects []*entity.Project) string {
	// Build project list
	projectList := "None available"
	if len(existingProjects) > 0 {
		projectNames := make([]string, len(existingProjects))
		for i, p := range existingProjects {
			projectNames[i] = p.Name
		}
		projectList = strings.Join(projectNames, "\n- ")
	}

	// Build summary and description
	summary := ""
	description := ""
	if fileMeta != nil {
		if fileMeta.AISummary != nil {
			summary = fileMeta.AISummary.Summary
		}
		if len(fileMeta.Tags) > 0 {
			description = strings.Join(fileMeta.Tags, ", ")
		}
	}

	// Build content section
	contentSection := ""
	if summary != "" {
		contentSection = fmt.Sprintf("Resumen:\n%s\n", summary)
		if description != "" {
			contentSection += fmt.Sprintf("Descripción:\n%s\n", description)
		}
	} else {
		contentSection = fmt.Sprintf("Archivo: %s\nTipo: %s\n", entry.Filename, entry.Extension)
	}

	// Add RAG context if available
	ragContext := ""
	if ragResponse != nil && len(ragResponse.Sources) > 0 {
		ragContext = "\n\nArchivos similares encontrados en el workspace:\n"
		for i, source := range ragResponse.Sources {
			if i >= 3 { // Limit to 3 examples
				break
			}
			ragContext += fmt.Sprintf("- %s (relevancia: %.2f)\n", source.RelativePath, source.Score)
		}
		ragContext += "\nConsidera estos archivos similares al sugerir el proyecto."
	}

	return fmt.Sprintf(`Eres un asistente experto en organización de documentos.

Basándote en la siguiente información, sugiere el proyecto/contexto más apropiado de la lista a continuación.
Si ninguno de los proyectos existentes encaja, sugiere un nuevo nombre de proyecto.

REGLAS CRÍTICAS (OBLIGATORIAS):
1. El nombre del proyecto DEBE estar en ESPAÑOL, el mismo idioma que el contenido.
2. El nombre debe ser descriptivo y relevante al contenido.
3. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales, sin espacios al inicio o final.
4. NO incluyas código, funciones, o ejemplos. SOLO el nombre del proyecto.
5. NO incluyas markdown, JSON, o cualquier otro formato. SOLO texto plano con el nombre.
6. Si incluyes puntuación o código, la respuesta será rechazada.

EJEMPLOS DE FORMATO CORRECTO:
- "Literatura Religiosa"
- "Vidas de Santos"
- "Documentos Teológicos"

EJEMPLOS DE FORMATO INCORRECTO (NO HACER):
- Código Python o funciones
- JSON objects
- Texto con formato especial

Proyectos existentes:
%s%s

%s

Proyecto sugerido:`, projectList, ragContext, contentSection)
}

func (s *SuggestionService) parseProjectResponse(response string, existingProjects []*entity.Project) []entity.SuggestedProject {
	// If response contains code-like patterns (import, def, class, etc.), extract project name from first line or title
	if strings.Contains(response, "import ") || strings.Contains(response, "def ") || strings.Contains(response, "class ") {
		s.logger.Warn().Str("response", truncateString(response, 200)).Msg("LLM returned code instead of project name, attempting extraction")
		// Try to extract project name from first meaningful line
		lines := strings.Split(response, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip code-like lines
			if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
				continue
			}
			// Skip empty lines
			if line == "" {
				continue
			}
			// If line looks like a project name (not too long, no special chars), use it
			if len(line) < 100 && !strings.Contains(line, "(") && !strings.Contains(line, "=") {
				response = line
				break
			}
		}
	}

	// Try to parse as JSON array first using ArrayParser
	arrayParser := llm.NewArrayParser(s.logger)
	ctx := context.Background()
	projects, err := arrayParser.ParseArray(ctx, response)
	if err == nil && len(projects) > 0 {
		// Successfully parsed as JSON array
		stringParser := llm.NewStringParser(s.logger)
		result := make([]entity.SuggestedProject, 0, len(projects))
		for _, projectName := range projects {
			projectName = stringParser.ParseString(projectName)
			if projectName == "" || len(projectName) > 100 {
				continue
			}

			// Check if it matches an existing project
			confidence := 0.7
			for _, existing := range existingProjects {
				if strings.EqualFold(projectName, existing.Name) {
					confidence = 0.9            // Higher confidence for existing projects
					projectName = existing.Name // Use exact name
					break
				}
			}

			result = append(result, entity.SuggestedProject{
				ProjectName: projectName,
				Confidence:  confidence,
				Source:      "llm",
			})
		}
		return result
	}

	// Fallback: treat as single project name using StringParser
	stringParser := llm.NewStringParser(s.logger)
	projectName := stringParser.ParseString(response)

	// Validate project name format
	if projectName == "" {
		s.logger.Warn().Str("response", truncateString(response, 200)).Msg("Could not extract project name from response (empty after cleaning)")
		return nil
	}
	if len(projectName) > 100 {
		s.logger.Warn().Str("project", truncateString(projectName, 50)).Int("length", len(projectName)).Msg("Project name too long, truncating")
		projectName = projectName[:100]
	}
	// Reject if still looks like code
	if strings.Contains(projectName, "import ") || strings.Contains(projectName, "def ") || strings.Contains(projectName, "class ") {
		s.logger.Warn().Str("project", truncateString(projectName, 50)).Msg("Project name still contains code patterns after cleaning, rejecting")
		return nil
	}

	// Check if it matches an existing project
	confidence := 0.7
	for _, existing := range existingProjects {
		if strings.EqualFold(projectName, existing.Name) {
			confidence = 0.9
			projectName = existing.Name
			s.logger.Debug().Str("project", projectName).Msg("Matched existing project")
			break
		}
	}

	s.logger.Info().
		Str("project", projectName).
		Float64("confidence", confidence).
		Bool("is_new", confidence < 0.9).
		Msg("Project suggestion parsed successfully")

	return []entity.SuggestedProject{
		{
			ProjectName: projectName,
			Confidence:  confidence,
			Source:      "llm",
		},
	}
}

// cleanProjectName cleans a project name string (deprecated: use StringParser instead).
// Kept for backward compatibility but delegates to StringParser.
func cleanProjectName(name string) string {
	// Use StringParser for consistent cleaning
	parser := llm.NewStringParser(zerolog.Nop())
	return parser.ParseString(name)
}

// truncateString truncates a string to max length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (s *SuggestionService) extractTitleFromContent(ctx context.Context, workspaceID entity.WorkspaceID, doc *entity.Document) string {
	if doc == nil {
		return ""
	}

	// If document already has a title, use it
	if doc.Title != "" && doc.Title != "Document" {
		return doc.Title
	}

	// Try to extract from chunks (first chunk often contains title)
	chunks, err := s.docRepo.GetChunksByDocument(ctx, workspaceID, doc.ID)
	if err != nil || len(chunks) == 0 {
		return ""
	}

	// Get first chunk and look for title patterns
	firstChunk := chunks[0].Text
	lines := strings.Split(firstChunk, "\n")

	// Look for common title patterns
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip if line is too long (probably not a title)
		if len(line) > 200 {
			continue
		}

		// Skip if line looks like code or metadata
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "---") {
			continue
		}

		// If line is reasonably short and doesn't look like body text, it might be a title
		if len(line) > 5 && len(line) < 150 && !strings.HasSuffix(line, ".") {
			// Check if it's all uppercase (common for titles)
			if strings.ToUpper(line) == line && len(line) < 80 {
				return line
			}
			// Check if it starts with capital and is short
			if len(line) < 100 && len(line) > 0 && strings.HasPrefix(line, strings.ToUpper(string(line[0]))) {
				return line
			}
		}
	}

	return ""
}

func (s *SuggestionService) inferAuthor(ctx context.Context, workspaceID entity.WorkspaceID, entry *entity.FileEntry, doc *entity.Document) string {
	// First, check if document metadata has author
	if entry.Enhanced != nil && entry.Enhanced.DocumentMetrics != nil {
		if entry.Enhanced.DocumentMetrics.Author != nil && *entry.Enhanced.DocumentMetrics.Author != "" {
			return *entry.Enhanced.DocumentMetrics.Author
		}
	}

	// Try to infer from file path patterns
	// Common patterns: "Author - Title.pdf", "Author/Title.pdf", "Author_Title.pdf"
	pathParts := strings.Split(entry.RelativePath, "/")
	filename := pathParts[len(pathParts)-1]

	// Remove extension
	filename = strings.TrimSuffix(filename, entry.Extension)

	// Try patterns like "Author - Title" or "Author_Title"
	separators := []string{" - ", "_", " -", "- ", " – ", " — "}
	for _, sep := range separators {
		if idx := strings.Index(filename, sep); idx > 0 {
			author := strings.TrimSpace(filename[:idx])
			// Validate author name (should be reasonable length and not look like a date)
			if len(author) > 2 && len(author) < 100 && !strings.HasPrefix(author, "20") {
				return author
			}
		}
	}

	// Try to extract from document content (first chunk, look for "Author:" or similar)
	if doc != nil {
		chunks, err := s.docRepo.GetChunksByDocument(ctx, workspaceID, doc.ID)
		if err == nil && len(chunks) > 0 {
			firstChunk := chunks[0].Text
			lines := strings.Split(firstChunk, "\n")

			// Look for author patterns in first few lines
			for i, line := range lines {
				if i > 10 { // Only check first 10 lines
					break
				}
				line = strings.TrimSpace(line)
				lineLower := strings.ToLower(line)

				// Check for "Author:" or "Autor:" patterns
				authorPatterns := []string{"author:", "autor:", "by ", "por ", "escrito por", "written by"}
				for _, pattern := range authorPatterns {
					if idx := strings.Index(lineLower, pattern); idx >= 0 {
						author := strings.TrimSpace(line[idx+len(pattern):])
						// Clean up author (remove trailing punctuation, etc.)
						author = strings.Trim(author, ".,;:()[]{}")
						if len(author) > 2 && len(author) < 100 {
							return author
						}
					}
				}
			}
		}
	}

	return ""
}

func (s *SuggestionService) calculateOverallConfidence(suggested *entity.SuggestedMetadata) float64 {
	// Calculate weighted average of all suggestion confidences
	if !suggested.HasSuggestions() {
		return 0.0
	}

	total := 0.0
	count := 0

	for _, tag := range suggested.SuggestedTags {
		total += tag.Confidence
		count++
	}

	for _, project := range suggested.SuggestedProjects {
		total += project.Confidence
		count++
	}

	if suggested.SuggestedTaxonomy != nil {
		total += suggested.SuggestedTaxonomy.CategoryConfidence
		count++
	}

	if count == 0 {
		return 0.0
	}

	return total / float64(count)
}
