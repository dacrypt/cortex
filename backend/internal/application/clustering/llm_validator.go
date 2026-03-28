package clustering

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
)

// LLMValidatorConfig contains configuration for LLM validation.
type LLMValidatorConfig struct {
	ValidationThreshold float64 // Minimum LLM confidence to validate cluster (default: 0.6)
	MaxDocsForContext   int     // Maximum documents to include in LLM context (default: 10)
	GenerateSummary     bool    // Generate cluster summary (default: true)
	GenerateKeywords    bool    // Generate cluster keywords (default: true)
}

// DefaultLLMValidatorConfig returns the default configuration.
func DefaultLLMValidatorConfig() LLMValidatorConfig {
	return LLMValidatorConfig{
		ValidationThreshold: 0.6,
		MaxDocsForContext:   10,
		GenerateSummary:     true,
		GenerateKeywords:    true,
	}
}

// LLMValidator validates and enriches clusters using an LLM.
type LLMValidator struct {
	llmRouter *llm.Router
	docRepo   DocumentInfoProvider
	config    LLMValidatorConfig
	logger    zerolog.Logger
}

// DocumentInfoProvider provides document information for LLM context.
type DocumentInfoProvider interface {
	GetDocumentInfo(ctx context.Context, workspaceID entity.WorkspaceID, docID entity.DocumentID) (*DocumentInfo, error)
}

// DocumentInfo contains basic document information for LLM context.
type DocumentInfo struct {
	ID           entity.DocumentID
	RelativePath string
	Title        string
	Summary      string
	Keywords     []string
}

// NewLLMValidator creates a new LLM validator.
func NewLLMValidator(
	llmRouter *llm.Router,
	docRepo DocumentInfoProvider,
	config LLMValidatorConfig,
	logger zerolog.Logger,
) *LLMValidator {
	return &LLMValidator{
		llmRouter: llmRouter,
		docRepo:   docRepo,
		config:    config,
		logger:    logger.With().Str("component", "llm_validator").Logger(),
	}
}

// ValidationResult contains the result of cluster validation.
type ValidationResult struct {
	IsValid    bool
	Confidence float64
	Reason     string
}

// ValidateCluster validates whether a detected cluster is meaningful.
func (v *LLMValidator) ValidateCluster(
	ctx context.Context,
	cluster *entity.DocumentCluster,
	memberIDs []entity.DocumentID,
	graph *entity.DocumentGraph,
) (bool, error) {
	if v.llmRouter == nil {
		return true, nil // No LLM available, accept all clusters
	}

	// Get document info for context
	docInfos := v.getDocumentInfosForCluster(ctx, cluster.WorkspaceID, cluster, memberIDs)
	if len(docInfos) == 0 {
		return false, fmt.Errorf("no document info available")
	}

	// Build validation prompt
	prompt := v.buildValidationPrompt(docInfos)

	// Call LLM
	response, err := v.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   200,
		Temperature: 0.2,
	})
	if err != nil {
		return false, fmt.Errorf("LLM validation failed: %w", err)
	}

	// Parse response
	result := v.parseValidationResponse(response.Text)

	v.logger.Debug().
		Str("cluster_id", cluster.ID.String()).
		Bool("valid", result.IsValid).
		Float64("confidence", result.Confidence).
		Str("reason", result.Reason).
		Msg("Cluster validation result")

	return result.IsValid && result.Confidence >= v.config.ValidationThreshold, nil
}

// GenerateClusterMetadata generates name, summary, and keywords for a cluster.
func (v *LLMValidator) GenerateClusterMetadata(
	ctx context.Context,
	cluster *entity.DocumentCluster,
	memberIDs []entity.DocumentID,
) error {
	if v.llmRouter == nil {
		return nil
	}

	// Get document info for context
	docInfos := v.getDocumentInfosForCluster(ctx, cluster.WorkspaceID, cluster, memberIDs)
	if len(docInfos) == 0 {
		return fmt.Errorf("no document info available")
	}

	// Build metadata prompt
	prompt := v.buildMetadataPrompt(docInfos)

	// Call LLM
	response, err := v.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   500,
		Temperature: 0.2,
	})
	if err != nil {
		return fmt.Errorf("LLM metadata generation failed: %w", err)
	}

	// Parse response and update cluster
	v.parseMetadataResponse(response.Text, cluster)
	v.sanitizeClusterMetadata(cluster, docInfos)

	v.logger.Debug().
		Str("cluster_id", cluster.ID.String()).
		Str("name", cluster.Name).
		Int("keywords", len(cluster.TopKeywords)).
		Msg("Generated cluster metadata")

	return nil
}

// getDocumentInfos retrieves document info for the given IDs.
func (v *LLMValidator) getDocumentInfos(ctx context.Context, workspaceID entity.WorkspaceID, memberIDs []entity.DocumentID) []*DocumentInfo {
	var infos []*DocumentInfo

	limit := v.config.MaxDocsForContext
	if len(memberIDs) < limit {
		limit = len(memberIDs)
	}

	for i := 0; i < limit; i++ {
		if v.docRepo == nil {
			// Create minimal info from ID
			infos = append(infos, &DocumentInfo{
				ID: memberIDs[i],
			})
			continue
		}

		info, err := v.docRepo.GetDocumentInfo(ctx, workspaceID, memberIDs[i])
		if err != nil {
			continue
		}
		if info != nil {
			infos = append(infos, info)
		}
	}

	return infos
}

func (v *LLMValidator) getDocumentInfosForCluster(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	cluster *entity.DocumentCluster,
	memberIDs []entity.DocumentID,
) []*DocumentInfo {
	if cluster == nil || len(cluster.CentralNodes) == 0 {
		return v.getDocumentInfos(ctx, workspaceID, v.orderMembers(memberIDs))
	}

	memberSet := make(map[entity.DocumentID]struct{}, len(memberIDs))
	for _, id := range memberIDs {
		memberSet[id] = struct{}{}
	}

	ordered := make([]entity.DocumentID, 0, len(memberIDs))
	for _, id := range cluster.CentralNodes {
		if _, ok := memberSet[id]; ok {
			ordered = append(ordered, id)
			delete(memberSet, id)
		}
	}

	remaining := make([]entity.DocumentID, 0, len(memberSet))
	for id := range memberSet {
		remaining = append(remaining, id)
	}
	ordered = append(ordered, v.orderMembers(remaining)...)

	return v.getDocumentInfos(ctx, workspaceID, ordered)
}

func (v *LLMValidator) orderMembers(memberIDs []entity.DocumentID) []entity.DocumentID {
	ordered := make([]entity.DocumentID, len(memberIDs))
	copy(ordered, memberIDs)
	sort.Slice(ordered, func(i, j int) bool {
		return string(ordered[i]) < string(ordered[j])
	})
	return ordered
}

// buildValidationPrompt builds the prompt for cluster validation.
func (v *LLMValidator) buildValidationPrompt(docInfos []*DocumentInfo) string {
	var sb strings.Builder

	sb.WriteString(`You are a document clustering assistant. Analyze if the following documents form a coherent group that should be clustered together.
Use the dominant topic and language visible in the documents. Be strict when documents are mixed or unrelated.

Documents in proposed cluster:
`)

	for i, info := range docInfos {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		if info.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", info.Title))
		}
		if info.RelativePath != "" {
			sb.WriteString(fmt.Sprintf("   Path: %s\n", info.RelativePath))
		}
		if info.Summary != "" {
			sb.WriteString(fmt.Sprintf("   Summary: %s\n", truncate(info.Summary, 200)))
		}
		if len(info.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("   Keywords: %s\n", strings.Join(info.Keywords, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`
Respond in JSON format:
{
  "is_valid": true/false,
  "confidence": 0.0-1.0,
  "reason": "Brief explanation of why these documents do/don't belong together"
}

Only output the JSON, nothing else.`)

	return sb.String()
}

// buildMetadataPrompt builds the prompt for cluster metadata generation.
func (v *LLMValidator) buildMetadataPrompt(docInfos []*DocumentInfo) string {
	var sb strings.Builder

	sb.WriteString(`You are a document organization assistant. Generate a concise name, summary, and keywords for a cluster of related documents.
Constraints:
- Name must be specific and descriptive, 2-5 words.
- Avoid generic names like "Documents", "Files", "Notes", "Misc", "General", "Cluster".
- Use the same language as the majority of document titles/summaries.
- Avoid punctuation, quotes, and counts.

Documents in cluster:
`)

	for i, info := range docInfos {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		if info.Title != "" {
			sb.WriteString(fmt.Sprintf("Title: %s\n", info.Title))
		}
		if info.RelativePath != "" {
			sb.WriteString(fmt.Sprintf("   Path: %s\n", info.RelativePath))
		}
		if info.Summary != "" {
			sb.WriteString(fmt.Sprintf("   Summary: %s\n", truncate(info.Summary, 150)))
		}
		if len(info.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("   Keywords: %s\n", strings.Join(info.Keywords, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`
Respond in JSON format:
{
  "name": "Short descriptive name for this cluster (2-5 words)",
  "summary": "One sentence describing what these documents have in common",
  "keywords": ["keyword1", "keyword2", "keyword3"],
  "entities": ["main entity/topic 1", "main entity/topic 2"]
}

Only output the JSON, nothing else.`)

	return sb.String()
}

// parseValidationResponse parses the LLM validation response.
func (v *LLMValidator) parseValidationResponse(response string) ValidationResult {
	result := ValidationResult{
		IsValid:    false,
		Confidence: 0.0,
		Reason:     "",
	}

	// Try to extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		v.logger.Warn().Str("response", truncate(response, 100)).Msg("Could not extract JSON from validation response")
		return result
	}

	var parsed struct {
		IsValid    bool    `json:"is_valid"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		v.logger.Warn().Err(err).Str("json", jsonStr).Msg("Failed to parse validation JSON")
		return result
	}

	result.IsValid = parsed.IsValid
	result.Confidence = parsed.Confidence
	result.Reason = parsed.Reason

	return result
}

// parseMetadataResponse parses the LLM metadata response and updates the cluster.
func (v *LLMValidator) parseMetadataResponse(response string, cluster *entity.DocumentCluster) {
	// Try to extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		v.logger.Warn().Str("response", truncate(response, 100)).Msg("Could not extract JSON from metadata response")
		return
	}

	var parsed struct {
		Name     string   `json:"name"`
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
		Entities []string `json:"entities"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		v.logger.Warn().Err(err).Str("json", jsonStr).Msg("Failed to parse metadata JSON")
		return
	}

	if parsed.Name != "" {
		cluster.Name = parsed.Name
	}
	if parsed.Summary != "" {
		cluster.Summary = parsed.Summary
	}
	if len(parsed.Keywords) > 0 {
		cluster.TopKeywords = parsed.Keywords
	}
	if len(parsed.Entities) > 0 {
		cluster.TopEntities = parsed.Entities
	}
}

func (v *LLMValidator) sanitizeClusterMetadata(cluster *entity.DocumentCluster, docInfos []*DocumentInfo) {
	if cluster == nil {
		return
	}

	name := normalizeClusterName(cluster.Name)
	if name == "" || isGenericClusterName(name) || countWords(name) < 2 {
		derived := deriveClusterName(cluster, docInfos)
		if derived != "" {
			name = derived
		}
	}
	if name != "" {
		cluster.Name = clampNameLength(name, 50)
	}

	if len(cluster.TopKeywords) == 0 {
		cluster.TopKeywords = deriveKeywords(docInfos, 5)
	}
}

func normalizeClusterName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `"'`)
	name = strings.Trim(name, ".,;:-_()[]{}")
	name = strings.Join(strings.Fields(name), " ")
	return name
}

func isGenericClusterName(name string) bool {
	words := strings.Fields(strings.ToLower(name))
	if len(words) == 0 {
		return true
	}
	generic := map[string]bool{
		"document": true, "documents": true, "doc": true, "docs": true,
		"file": true, "files": true, "item": true, "items": true,
		"note": true, "notes": true, "misc": true, "miscellaneous": true,
		"general": true, "other": true, "data": true, "content": true,
		"cluster": true, "collection": true, "group": true,
	}
	for _, word := range words {
		if !generic[word] {
			return false
		}
	}
	return true
}

func deriveClusterName(cluster *entity.DocumentCluster, docInfos []*DocumentInfo) string {
	candidates := make([]string, 0, 4)

	keywordCandidates := cluster.TopKeywords
	if len(keywordCandidates) == 0 {
		keywordCandidates = deriveKeywords(docInfos, 4)
	}
	for _, kw := range keywordCandidates {
		token := normalizeToken(kw)
		if token != "" && !isStopWord(token) {
			candidates = append(candidates, token)
		}
		if len(candidates) >= 3 {
			break
		}
	}

	if len(candidates) == 0 {
		for _, info := range docInfos {
			parts := tokenize(info.Title)
			for _, part := range parts {
				if !isStopWord(part) {
					candidates = append(candidates, part)
				}
				if len(candidates) >= 3 {
					break
				}
			}
			if len(candidates) >= 3 {
				break
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	for i, token := range candidates {
		candidates[i] = titleize(token)
	}
	return strings.Join(candidates, " ")
}

func deriveKeywords(docInfos []*DocumentInfo, max int) []string {
	seen := make(map[string]bool)
	keywords := make([]string, 0, max)
	for _, info := range docInfos {
		for _, kw := range info.Keywords {
			token := normalizeToken(kw)
			if token == "" || isStopWord(token) || seen[token] {
				continue
			}
			seen[token] = true
			keywords = append(keywords, token)
			if len(keywords) >= max {
				return keywords
			}
		}
	}
	return keywords
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	separators := func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}
	raw := strings.FieldsFunc(text, separators)
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		norm := normalizeToken(token)
		if norm != "" {
			out = append(out, norm)
		}
	}
	return out
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(token))
	token = strings.Trim(token, ".,;:-_()[]{}")
	if len(token) < 3 {
		return ""
	}
	return token
}

func titleize(token string) string {
	if token == "" {
		return ""
	}
	if len(token) == 1 {
		return strings.ToUpper(token)
	}
	return strings.ToUpper(token[:1]) + token[1:]
}

func isStopWord(token string) bool {
	switch token {
	case "the", "and", "for", "with", "this", "that", "from", "into", "onto", "over", "under", "about", "between",
		"las", "los", "una", "unos", "unas", "del", "por", "para", "con", "sin", "sobre", "entre", "como", "que",
		"de", "y", "o", "en", "a", "la", "el", "un":
		return true
	}
	return false
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func clampNameLength(name string, max int) string {
	if len(name) <= max {
		return name
	}
	truncated := name[:max]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		return strings.TrimSpace(truncated[:lastSpace])
	}
	return strings.TrimSpace(truncated)
}

// extractJSON extracts JSON from a string that might contain other text.
func extractJSON(s string) string {
	// Find JSON object bounds
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

// truncate truncates a string to the given length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SuggestClusterName generates a name suggestion for a cluster using LLM.
func (v *LLMValidator) SuggestClusterName(ctx context.Context, workspaceID entity.WorkspaceID, memberIDs []entity.DocumentID) (string, error) {
	if v.llmRouter == nil {
		return "", fmt.Errorf("LLM not available")
	}

	docInfos := v.getDocumentInfos(ctx, workspaceID, memberIDs)
	if len(docInfos) == 0 {
		return "", fmt.Errorf("no document info available")
	}

	var pathList []string
	for _, info := range docInfos {
		if info.RelativePath != "" {
			pathList = append(pathList, info.RelativePath)
		} else if info.Title != "" {
			pathList = append(pathList, info.Title)
		}
	}

	prompt := fmt.Sprintf(`Generate a short, descriptive name (2-5 words) for a group of documents containing:
%s

Respond with just the name, nothing else.`, strings.Join(pathList, "\n"))

	response, err := v.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   20,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	// Clean up response
	name := strings.TrimSpace(response.Text)
	name = regexp.MustCompile(`["\n\r]`).ReplaceAllString(name, "")
	if len(name) > 50 {
		name = name[:50]
	}

	return name, nil
}

// ValidateMembership checks if a document should belong to a cluster.
func (v *LLMValidator) ValidateMembership(
	ctx context.Context,
	cluster *entity.DocumentCluster,
	document *DocumentInfo,
) (bool, float64, error) {
	if v.llmRouter == nil {
		return true, 1.0, nil
	}

	prompt := fmt.Sprintf(`Does the following document belong to the cluster "%s"?

Cluster description: %s

Document:
- Path: %s
- Title: %s
- Summary: %s

Respond in JSON format:
{"belongs": true/false, "confidence": 0.0-1.0, "reason": "brief explanation"}

Only output JSON.`,
		cluster.Name,
		cluster.Summary,
		document.RelativePath,
		document.Title,
		truncate(document.Summary, 200),
	)

	response, err := v.llmRouter.Generate(ctx, llm.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   100,
		Temperature: 0.2,
	})
	if err != nil {
		return false, 0, err
	}

	jsonStr := extractJSON(response.Text)
	if jsonStr == "" {
		return false, 0, fmt.Errorf("could not parse response")
	}

	var parsed struct {
		Belongs    bool    `json:"belongs"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return false, 0, err
	}

	return parsed.Belongs, parsed.Confidence, nil
}
