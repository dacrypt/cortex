// Package llm provides LLM router and provider implementations.
// This file contains output parsers that integrate langchaingo where useful
// while maintaining robust custom parsers for complex cases.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/outputparser"
)

// JSONParser is a robust JSON output parser with retry and validation.
// Inspired by langchaingo's output parser patterns.
type JSONParser struct {
	logger     zerolog.Logger
	maxRetries int
}

// NewJSONParser creates a new JSON parser with retry capability.
func NewJSONParser(logger zerolog.Logger) *JSONParser {
	return &JSONParser{
		logger:     logger.With().Str("component", "json_parser").Logger(),
		maxRetries: 3,
	}
}

// ParseJSON parses a JSON string from LLM response with automatic cleanup and retry.
// It handles markdown code blocks, trailing text, and malformed JSON.
func (p *JSONParser) ParseJSON(ctx context.Context, response string, target interface{}) error {
	cleaned := p.cleanResponse(response)

	var lastErr error
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		cleaned = p.getCleanedResponse(response, attempt, cleaned)
		if attempt > 0 {
			p.logger.Debug().
				Int("attempt", attempt+1).
				Str("cleaned_preview", truncateString(cleaned, 200)).
				Msg("Retrying JSON parse with aggressive cleaning")
		}

		if err := json.Unmarshal([]byte(cleaned), target); err == nil {
			p.logSuccess(attempt)
			return nil
		} else {
			lastErr = err
			if p.tryRepairIncompleteJSON(cleaned, err, attempt, target) {
				return nil
			}
		}
	}

	return fmt.Errorf("failed to parse JSON after %d attempts: %w (cleaned preview: %s)",
		p.maxRetries, lastErr, truncateString(cleaned, 200))
}

// getCleanedResponse returns the cleaned response based on attempt number.
func (p *JSONParser) getCleanedResponse(response string, attempt int, currentCleaned string) string {
	if attempt > 0 {
		return p.cleanResponseAggressive(response)
	}
	return currentCleaned
}

// logSuccess logs successful parse if it required retries.
func (p *JSONParser) logSuccess(attempt int) {
	if attempt > 0 {
		p.logger.Info().
			Int("attempts", attempt+1).
			Msg("JSON parse succeeded after retry")
	}
}

// tryRepairIncompleteJSON attempts to repair incomplete JSON on the last attempt.
func (p *JSONParser) tryRepairIncompleteJSON(cleaned string, err error, attempt int, target interface{}) bool {
	if attempt != p.maxRetries-1 {
		return false
	}
	if !strings.Contains(err.Error(), "unexpected end of JSON input") {
		return false
	}

	repaired := p.repairIncompleteJSON(cleaned)
	if repaired == cleaned {
		return false
	}

	if repairErr := json.Unmarshal([]byte(repaired), target); repairErr == nil {
		p.logger.Warn().
			Msg("Successfully repaired incomplete JSON by closing brackets/braces")
		return true
	}
	return false
}

// cleanResponse performs standard cleaning of LLM response.
func (p *JSONParser) cleanResponse(response string) string {
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = stripJSONComments(cleaned)

	// Extract the last complete JSON object (most likely the actual response)
	// This handles cases where there are example JSON objects before the actual response
	jsonObj := p.extractLastCompleteJSON(cleaned)
	if jsonObj != "" {
		return jsonObj
	}

	// Fallback: Extract JSON object if wrapped in text
	if idx := strings.Index(cleaned, "{"); idx > 0 {
		cleaned = cleaned[idx:]
	}
	if idx := strings.LastIndex(cleaned, "}"); idx > 0 && idx < len(cleaned)-1 {
		cleaned = cleaned[:idx+1]
	}

	return cleaned
}

// cleanResponseAggressive performs more aggressive cleaning for retry attempts.
func (p *JSONParser) cleanResponseAggressive(response string) string {
	cleaned := p.removeMarkdownAndArtifacts(response)
	cleaned = stripJSONComments(cleaned)
	cleaned = p.extractJSONObject(cleaned)

	// If still no valid JSON found, filter out explanatory text and try again
	if !strings.HasPrefix(cleaned, "{") {
		cleaned = p.filterExplanatoryText(cleaned)
		cleaned = p.extractJSONObject(cleaned)
		cleaned = p.extractJSONWithRegex(cleaned)
	}

	// Fix common JSON issues (trailing commas)
	cleaned = p.fixTrailingCommas(cleaned)
	return cleaned
}

// removeMarkdownAndArtifacts removes markdown code blocks and common LLM artifacts.
func (p *JSONParser) removeMarkdownAndArtifacts(response string) string {
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Remove common LLM artifacts (English)
	englishPrefixes := []string{"Here is the JSON:", "JSON:", "Response:"}
	for _, prefix := range englishPrefixes {
		cleaned = strings.TrimPrefix(cleaned, prefix)
		cleaned = strings.TrimSpace(cleaned)
	}

	// Remove common LLM artifacts (Spanish)
	spanishPrefixes := []string{"Aquí está el JSON:", "JSON:", "Respuesta:", "El JSON es:"}
	for _, prefix := range spanishPrefixes {
		cleaned = strings.TrimPrefix(cleaned, prefix)
		cleaned = strings.TrimSpace(cleaned)
	}

	return cleaned
}

// extractJSONObject attempts to extract a JSON object from the text.
func (p *JSONParser) extractJSONObject(text string) string {
	// First, try to extract the last complete JSON object
	if jsonObj := p.extractLastCompleteJSON(text); jsonObj != "" {
		return jsonObj
	}

	// Fallback: try to extract JSON object directly if it exists
	return p.extractJSONByBraces(text)
}

// extractJSONByBraces extracts JSON by finding brace boundaries.
func (p *JSONParser) extractJSONByBraces(text string) string {
	firstBrace := strings.Index(text, "{")
	lastBrace := strings.LastIndex(text, "}")
	if firstBrace < 0 || lastBrace <= firstBrace {
		return text
	}

	potentialJSON := text[firstBrace : lastBrace+1]
	if p.isValidJSONStructure(potentialJSON) {
		return potentialJSON
	}
	return text
}

// isValidJSONStructure checks if a string has valid JSON structure (balanced braces and key-value pairs).
func (p *JSONParser) isValidJSONStructure(text string) bool {
	openCount := strings.Count(text, "{")
	closeCount := strings.Count(text, "}")
	return openCount == closeCount && openCount > 0 && strings.Contains(text, ":")
}

// filterExplanatoryText filters out markdown and explanatory text lines.
func (p *JSONParser) filterExplanatoryText(text string) string {
	lines := strings.Split(text, "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if p.shouldSkipLine(trimmed) {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	return strings.Join(filteredLines, "\n")
}

// shouldSkipLine determines if a line should be skipped during filtering.
func (p *JSONParser) shouldSkipLine(trimmed string) bool {
	// Skip markdown list items
	if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") {
		return true
	}
	// Skip markdown headers
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	// Skip comments
	if strings.HasPrefix(trimmed, "//") {
		return true
	}
	// Skip explanatory text patterns (English)
	englishPatterns := []string{"Based on", "I would classify", "The file"}
	for _, pattern := range englishPatterns {
		if strings.HasPrefix(trimmed, pattern) {
			return true
		}
	}
	// Skip explanatory text patterns (Spanish) - case insensitive
	trimmedLower := strings.ToLower(trimmed)
	spanishPatterns := []string{
		"basado en", "puedo clasificar", "el archivo", "según el contenido",
		"de acuerdo con", "clasificarlo", "puedo clasificarlo",
	}
	for _, pattern := range spanishPatterns {
		if strings.HasPrefix(trimmedLower, pattern) {
			return true
		}
	}
	return false
}

// extractJSONWithRegex uses regex as a last resort to find JSON objects.
func (p *JSONParser) extractJSONWithRegex(text string) string {
	if strings.HasPrefix(text, "{") {
		return text
	}

	jsonPattern := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonPattern.FindString(text)
	if matches != "" && p.isValidJSONStructure(matches) {
		return matches
	}
	return text
}

// fixTrailingCommas removes trailing commas before closing braces/brackets.
func (p *JSONParser) fixTrailingCommas(text string) string {
	re := regexp.MustCompile(`,(\s*[}\]])`)
	return re.ReplaceAllString(text, "$1")
}

// extractLastCompleteJSON finds and extracts the last complete JSON object from the response.
// This is useful when LLM responses include example JSON followed by the actual response.
func (p *JSONParser) extractLastCompleteJSON(text string) string {
	if len(text) == 0 {
		return ""
	}

	parser := newJSONObjectParser(text)
	return parser.extract()
}

// jsonObjectParser tracks state while parsing JSON objects from text.
type jsonObjectParser struct {
	text          string
	lastValidJSON string
	depth         int
	startPos      int
	inString      bool
	escapeNext    bool
}

// newJSONObjectParser creates a new JSON object parser.
func newJSONObjectParser(text string) *jsonObjectParser {
	return &jsonObjectParser{
		text:     text,
		startPos: -1,
	}
}

// extract finds and returns the last valid JSON object.
func (p *jsonObjectParser) extract() string {
	for i, char := range p.text {
		if p.handleEscape(char) {
			continue
		}

		if p.handleString(char) {
			continue
		}

		if !p.inString {
			p.handleBraces(char, i)
		}
	}

	return p.lastValidJSON
}

// handleEscape processes escape sequences.
func (p *jsonObjectParser) handleEscape(char rune) bool {
	if p.escapeNext {
		p.escapeNext = false
		return true
	}
	if char == '\\' {
		p.escapeNext = true
		return true
	}
	return false
}

// handleString processes string delimiters.
func (p *jsonObjectParser) handleString(char rune) bool {
	if char == '"' && !p.escapeNext {
		p.inString = !p.inString
		return true
	}
	return false
}

// handleBraces processes opening and closing braces.
func (p *jsonObjectParser) handleBraces(char rune, pos int) {
	switch char {
	case '{':
		if p.depth == 0 {
			p.startPos = pos
		}
		p.depth++
	case '}':
		p.depth--
		if p.depth == 0 && p.startPos >= 0 {
			p.validateAndStoreJSON(pos)
			p.startPos = -1
		}
	}
}

// validateAndStoreJSON validates and stores a potential JSON object.
func (p *jsonObjectParser) validateAndStoreJSON(endPos int) {
	potentialJSON := p.text[p.startPos : endPos+1]
	if !strings.Contains(potentialJSON, ":") {
		return
	}

	var test interface{}
	if err := json.Unmarshal([]byte(potentialJSON), &test); err == nil {
		p.lastValidJSON = potentialJSON
	}
}

// repairIncompleteJSON attempts to repair truncated JSON by closing brackets and braces.
// This is useful when LLM responses are cut off due to token limits.
func (p *JSONParser) repairIncompleteJSON(jsonStr string) string {
	cleaned := strings.TrimSpace(jsonStr)
	if len(cleaned) == 0 {
		return jsonStr
	}

	// Count open/close brackets and braces
	openBraces := strings.Count(cleaned, "{")
	closeBraces := strings.Count(cleaned, "}")
	openBrackets := strings.Count(cleaned, "[")
	closeBrackets := strings.Count(cleaned, "]")

	// If JSON appears complete, return as-is
	if openBraces == closeBraces && openBrackets == closeBrackets {
		return cleaned
	}

	// Check if we're in the middle of a string (odd number of unescaped quotes)
	// This is a simple heuristic - count quotes that aren't escaped
	quoteCount := 0
	escaped := false
	for i := 0; i < len(cleaned); i++ {
		if cleaned[i] == '\\' && i+1 < len(cleaned) {
			escaped = true
			continue
		}
		if cleaned[i] == '"' && !escaped {
			quoteCount++
		}
		escaped = false
	}

	// If we're in the middle of a string, close it
	if quoteCount%2 != 0 {
		cleaned += `"`
	}

	// Close arrays first (they're usually nested inside objects)
	for i := openBrackets - closeBrackets; i > 0; i-- {
		cleaned += "]"
	}

	// Close objects
	for i := openBraces - closeBraces; i > 0; i-- {
		cleaned += "}"
	}

	return cleaned
}

// StringParser is a parser for simple string responses with cleanup.
type StringParser struct {
	logger zerolog.Logger
}

// NewStringParser creates a new string parser.
func NewStringParser(logger zerolog.Logger) *StringParser {
	return &StringParser{
		logger: logger.With().Str("component", "string_parser").Logger(),
	}
}

// ParseString parses a string response with automatic cleanup.
func (p *StringParser) ParseString(response string) string {
	cleaned := strings.TrimSpace(response)

	// Remove markdown code blocks
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Remove quotes if the entire response is quoted
	if len(cleaned) >= 2 {
		if (cleaned[0] == '"' && cleaned[len(cleaned)-1] == '"') ||
			(cleaned[0] == '\'' && cleaned[len(cleaned)-1] == '\'') {
			cleaned = cleaned[1 : len(cleaned)-1]
		}
	}

	// Remove trailing punctuation that LLMs sometimes add
	// Remove in reverse order to handle multiple punctuation marks
	for {
		original := cleaned
		cleaned = strings.TrimSuffix(cleaned, ".")
		cleaned = strings.TrimSuffix(cleaned, ",")
		cleaned = strings.TrimSuffix(cleaned, ";")
		cleaned = strings.TrimSuffix(cleaned, ":")
		if cleaned == original {
			break // No more punctuation to remove
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// ArrayParser is a parser for string arrays (like tags).
type ArrayParser struct {
	logger zerolog.Logger
	parser *JSONParser
}

// NewArrayParser creates a new array parser.
func NewArrayParser(logger zerolog.Logger) *ArrayParser {
	return &ArrayParser{
		logger: logger.With().Str("component", "array_parser").Logger(),
		parser: NewJSONParser(logger),
	}
}

// ParseArray parses a string array from LLM response.
// Uses langchaingo's CommaSeparatedList as a fallback for simple cases.
func (p *ArrayParser) ParseArray(ctx context.Context, response string) ([]string, error) {
	// Try parsing as JSON array first (our robust parser)
	if result, err := p.parseAsJSONArray(ctx, response); err == nil {
		return result, nil
	}

	// Fallback 1: Try langchaingo's CommaSeparatedList parser
	if result := p.parseWithLangchain(response); len(result) > 0 {
		return result, nil
	}

	// Fallback 2: Manual parsing as comma-separated list
	return p.parseAsCommaSeparated(response), nil
}

// parseAsJSONArray attempts to parse the response as a JSON array.
func (p *ArrayParser) parseAsJSONArray(ctx context.Context, response string) ([]string, error) {
	var result []string
	if err := p.parser.ParseJSON(ctx, response, &result); err != nil {
		return nil, err
	}
	return p.cleanAndValidateItems(result), nil
}

// parseWithLangchain attempts to parse using langchaingo's CommaSeparatedList parser.
func (p *ArrayParser) parseWithLangchain(response string) []string {
	langchainParser := outputparser.NewCommaSeparatedList()
	langchainResult, err := langchainParser.Parse(response)
	if err != nil {
		return nil
	}

	cleaned := p.cleanAndValidateItems(langchainResult)
	if len(cleaned) > 0 {
		p.logger.Debug().Msg("Array parsed successfully using langchaingo CommaSeparatedList")
		return cleaned
	}
	return nil
}

// parseAsCommaSeparated manually parses a comma-separated list.
func (p *ArrayParser) parseAsCommaSeparated(response string) []string {
	cleaned := p.prepareCommaSeparatedText(response)
	items := strings.Split(cleaned, ",")

	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		item = strings.Trim(item, "[]")
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// prepareCommaSeparatedText cleans the response for comma-separated parsing.
func (p *ArrayParser) prepareCommaSeparatedText(response string) string {
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	// Remove brackets if present
	cleaned = strings.TrimPrefix(cleaned, "[")
	cleaned = strings.TrimSuffix(cleaned, "]")
	return strings.TrimSpace(cleaned)
}

// cleanAndValidateItems cleans and validates array items.
func (p *ArrayParser) cleanAndValidateItems(items []string) []string {
	cleaned := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		item = strings.Trim(item, "[]")
		if item != "" && len(item) <= 100 {
			cleaned = append(cleaned, item)
		}
	}
	return cleaned
}

func stripJSONComments(input string) string {
	if input == "" {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input))

	inString := false
	escaped := false
	i := 0
	for i < len(input) {
		ch := input[i]
		if inString {
			builder.WriteByte(ch)
			if escaped {
				escaped = false
			} else {
				if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					inString = false
				}
			}
			i++
			continue
		}

		if ch == '"' {
			inString = true
			builder.WriteByte(ch)
			i++
			continue
		}

		if ch == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				i += 2
				for i < len(input) && input[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
					i++
				}
				if i+1 < len(input) {
					i += 2
				}
				continue
			}
		}

		builder.WriteByte(ch)
		i++
	}
	return builder.String()
}
