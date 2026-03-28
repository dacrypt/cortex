package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// Constants for organization and reference type normalization
const (
	orgTypeOrganizacion = "organización"
	orgTypeInstitucion  = "institución"
	orgTypeAsociacion   = "asociación"
	orgTypeFundacion    = "fundación"
	refTypeArticulo     = "artículo"
	refTypeSitioWeb     = "sitio web"
)

// Helper functions for parsing JSON values
func getStringFromInterface(v interface{}) *string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		// Clean "null" strings (LLM sometimes returns "null" as string instead of null)
		val = strings.TrimSpace(val)
		if val == "" || strings.EqualFold(val, "null") {
			return nil
		}
		return &val
	case float64:
		// Sometimes numbers are returned as strings
		return nil
	}
	return nil
}

func getIntFromInterface(v interface{}) *int {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		result := int(val)
		return &result
	case int:
		return &val
	case string:
		// Try to parse string as int
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return &result
		}
	}
	return nil
}

// isValidStringValue checks if a string is valid (not empty or "null")
func isValidStringValue(s string) bool {
	return s != "" && !strings.EqualFold(s, "null")
}

// processStringArrayItem processes a single item from an array, returning the string if valid
func processStringArrayItem(item interface{}) string {
	str, ok := item.(string)
	if !ok {
		return ""
	}
	str = strings.TrimSpace(str)
	if isValidStringValue(str) {
		return str
	}
	return ""
}

func getStringArrayFromInterface(v interface{}) []string {
	if v == nil {
		return nil
	}

	var items []interface{}
	switch val := v.(type) {
	case []interface{}:
		items = val
	case []string:
		// Convert []string to []interface{} for unified processing
		items = make([]interface{}, len(val))
		for i, s := range val {
			items[i] = s
		}
	default:
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		if str := processStringArrayItem(item); str != "" {
			result = append(result, str)
		}
	}
	return result
}

func getStringValueFromInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	return ""
}

// Helper functions for parsing specific sections of AIContext

// parseAuthorItem parses a single author item from the raw data
func parseAuthorItem(authorMap map[string]interface{}, seenAuthors map[string]bool, logger zerolog.Logger) *entity.AuthorInfo {
	authorName := getStringValueFromInterface(authorMap["name"])
	if authorName == "" || strings.EqualFold(authorName, "null") {
		logger.Debug().Str("author_name", authorName).Msg("Skipping author with null or empty name")
		return nil
	}

	authorKey := strings.ToLower(authorName)
	if seenAuthors[authorKey] {
		logger.Debug().Str("author_name", authorName).Msg("Skipping duplicate author")
		return nil
	}
	seenAuthors[authorKey] = true

	var affiliation *string
	if aff := getStringFromInterface(authorMap["affiliation"]); aff != nil && *aff != "null" {
		affiliation = aff
	}

	role := getStringValueFromInterface(authorMap["role"])
	if strings.EqualFold(role, "null") {
		role = ""
	}

	return &entity.AuthorInfo{
		Name:        authorName,
		Role:        role,
		Affiliation: affiliation,
		Confidence:  0.8,
	}
}

func parseAuthors(raw map[string]interface{}, logger zerolog.Logger) []entity.AuthorInfo {
	authorsData, ok := raw["authors"].([]interface{})
	if !ok {
		return nil
	}

	var authors []entity.AuthorInfo
	seenAuthors := make(map[string]bool)
	for _, authorItem := range authorsData {
		authorMap, ok := authorItem.(map[string]interface{})
		if !ok {
			continue
		}
		if author := parseAuthorItem(authorMap, seenAuthors, logger); author != nil {
			authors = append(authors, *author)
		}
	}
	return authors
}

func parseContributors(raw map[string]interface{}, authors []entity.AuthorInfo, logger zerolog.Logger) []string {
	contributors := filterGenericPlaceholders(getStringArrayFromInterface(raw["contributors"]))
	if len(authors) == 0 {
		return contributors
	}

	authorNames := make(map[string]bool)
	for _, author := range authors {
		authorNames[strings.ToLower(author.Name)] = true
	}

	filteredContributors := make([]string, 0, len(contributors))
	for _, contrib := range contributors {
		contribLower := strings.ToLower(contrib)
		if !authorNames[contribLower] {
			filteredContributors = append(filteredContributors, contrib)
		} else {
			logger.Debug().Str("contributor", contrib).Msg("Skipping contributor that is already an author")
		}
	}
	return filteredContributors
}

func validatePublicationYear(year *int, fileLastModified *time.Time, logger zerolog.Logger) *int {
	if year == nil || fileLastModified == nil {
		if year == nil && fileLastModified != nil {
			fileYear := fileLastModified.Year()
			logger.Debug().Int("file_year", fileYear).Msg("LLM didn't provide publication year, using file year as fallback")
			return &fileYear
		}
		return year
	}

	fileYear := fileLastModified.Year()
	currentYear := time.Now().Year()

	if *year > currentYear {
		logger.Debug().Int("llm_year", *year).Int("file_year", fileYear).Int("current_year", currentYear).
			Msg("Publication year from LLM is in future, using file year")
		return &fileYear
	}

	if *year > fileYear+2 {
		logger.Debug().Int("llm_year", *year).Int("file_year", fileYear).
			Msg("Publication year from LLM is significantly after file date, using file year")
		return &fileYear
	}

	if *year < fileYear-200 && *year > currentYear {
		logger.Debug().Int("llm_year", *year).Int("file_year", fileYear).
			Msg("Publication year from LLM is in future, using file year")
		return &fileYear
	}

	return year
}

func parseStringOrArrayField(raw map[string]interface{}, fieldName string) *string {
	field, ok := raw[fieldName]
	if !ok || field == nil {
		return nil
	}

	var result string
	if str, ok := field.(string); ok && str != "" && str != "null" {
		result = str
	} else if arr, ok := field.([]interface{}); ok && len(arr) > 0 {
		items := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok && str != "" && str != "null" {
				items = append(items, str)
			}
		}
		if len(items) > 0 {
			result = strings.Join(items, ", ")
		}
	}

	if result == "" {
		return nil
	}
	return &result
}

// extractPeriodFromString extracts period from a string value
func extractPeriodFromString(v string) string {
	if v != "" && v != "null" {
		return v
	}
	return ""
}

// extractPeriodFromArray extracts period from an array value
func extractPeriodFromArray(arr []interface{}) string {
	if len(arr) == 0 {
		return ""
	}
	periods := make([]string, 0, len(arr))
	for _, item := range arr {
		if str, ok := item.(string); ok && str != "" && str != "null" {
			periods = append(periods, str)
		}
	}
	if len(periods) > 0 {
		return strings.Join(periods, ", ")
	}
	return ""
}

func parseHistoricalPeriod(raw map[string]interface{}) *string {
	hp := raw["historical_period"]
	if hp == nil {
		return nil
	}

	var periodStr string
	switch v := hp.(type) {
	case string:
		periodStr = extractPeriodFromString(v)
	case []interface{}:
		periodStr = extractPeriodFromArray(v)
	}

	if periodStr == "" || isGenericPlaceholder(periodStr) {
		return nil
	}

	periodStr = strings.TrimSpace(periodStr)
	periodStr = strings.Join(strings.Fields(periodStr), " ")
	return &periodStr
}

func parseLocations(raw map[string]interface{}) []entity.LocationInfo {
	var locations []entity.LocationInfo
	if locationsData, ok := raw["locations"].([]interface{}); ok {
		for _, locItem := range locationsData {
			if locMap, ok := locItem.(map[string]interface{}); ok {
				loc := entity.LocationInfo{
					Name: getStringValueFromInterface(locMap["name"]),
					Type: getStringValueFromInterface(locMap["type"]),
				}
				if ctx := getStringFromInterface(locMap["context"]); ctx != nil {
					loc.Context = *ctx
				}
				if loc.Name != "" {
					locations = append(locations, loc)
				}
			}
		}
	}
	return locations
}

// parsePersonItem parses a single person item from the raw data
func parsePersonItem(personMap map[string]interface{}, logger zerolog.Logger) *entity.PersonInfo {
	personName := getStringValueFromInterface(personMap["name"])
	if personName == "" || strings.EqualFold(personName, "null") || isGenericPlaceholder(personName) {
		logger.Debug().Str("person_name", personName).
			Msg("Skipping person with null, empty, or generic placeholder name")
		return nil
	}

	role := getStringValueFromInterface(personMap["role"])
	if strings.EqualFold(role, "null") {
		role = ""
	}

	person := &entity.PersonInfo{
		Name:       personName,
		Role:       role,
		Confidence: 0.8,
	}
	if ctx := getStringFromInterface(personMap["context"]); ctx != nil && *ctx != "" && *ctx != "null" {
		person.Context = *ctx
	}
	return person
}

func parsePeople(raw map[string]interface{}, logger zerolog.Logger) []entity.PersonInfo {
	peopleData, ok := raw["people_mentioned"].([]interface{})
	if !ok {
		return nil
	}

	var people []entity.PersonInfo
	for _, personItem := range peopleData {
		personMap, ok := personItem.(map[string]interface{})
		if !ok {
			continue
		}
		if person := parsePersonItem(personMap, logger); person != nil {
			people = append(people, *person)
		}
	}
	return people
}

// parseOrgItem parses a single organization item from the raw data
func parseOrgItem(orgMap map[string]interface{}) *entity.OrgInfo {
	orgName := getStringValueFromInterface(orgMap["name"])
	orgTypeStr := getStringValueFromInterface(orgMap["type"])
	orgContext := getStringFromInterface(orgMap["context"])

	// Filter out objects where all fields are null or empty
	// This handles cases where LLM returns {"name": "null", "type": "null", "context": "null"}
	hasValidName := orgName != "" && !strings.EqualFold(orgName, "null")
	hasValidType := orgTypeStr != "" && !strings.EqualFold(orgTypeStr, "null")
	hasValidContext := orgContext != nil && *orgContext != "" && !strings.EqualFold(*orgContext, "null")

	// If all fields are null/empty, skip this organization
	if !hasValidName && !hasValidType && !hasValidContext {
		return nil
	}

	// Name is required, so if it's null/empty, skip
	if !hasValidName {
		return nil
	}

	orgType := normalizeOrganizationType(orgTypeStr)
	org := &entity.OrgInfo{
		Name:       orgName,
		Type:       orgType,
		Confidence: 0.8,
	}
	if hasValidContext {
		org.Context = *orgContext
	}
	return org
}

func parseOrganizations(raw map[string]interface{}) []entity.OrgInfo {
	orgsData, ok := raw["organizations"].([]interface{})
	if !ok {
		return nil
	}

	var orgs []entity.OrgInfo
	for _, orgItem := range orgsData {
		orgMap, ok := orgItem.(map[string]interface{})
		if !ok {
			continue
		}
		if org := parseOrgItem(orgMap); org != nil {
			orgs = append(orgs, *org)
		}
	}
	return orgs
}

// parseEventDate parses the date field from an event map
func parseEventDate(eventMap map[string]interface{}, logger zerolog.Logger) *time.Time {
	dateStr := getStringFromInterface(eventMap["date"])
	if dateStr == nil || *dateStr == "" || *dateStr == "null" {
		return nil
	}
	eventDate, err := time.Parse("2006-01-02", *dateStr)
	if err != nil {
		logger.Debug().Str("date_str", *dateStr).Err(err).Msg("Failed to parse event date, skipping")
		return nil
	}
	return &eventDate
}

// parseEventItem parses a single event item from the raw data
func parseEventItem(eventMap map[string]interface{}, seenEvents map[string]bool, logger zerolog.Logger) *entity.EventInfo {
	eventNamePtr := getStringFromInterface(eventMap["name"])
	if eventNamePtr == nil || *eventNamePtr == "null" {
		return nil
	}
	eventName := *eventNamePtr

	if eventName == "" {
		return nil
	}

	eventKey := strings.ToLower(eventName)
	if seenEvents[eventKey] {
		logger.Debug().Str("event_name", eventName).Msg("Skipping duplicate event")
		return nil
	}
	seenEvents[eventKey] = true

	event := &entity.EventInfo{
		Name:       eventName,
		Confidence: 0.8,
	}

	if eventDate := parseEventDate(eventMap, logger); eventDate != nil {
		event.Date = eventDate
	}

	if loc := getStringFromInterface(eventMap["location"]); loc != nil && *loc != "" && *loc != "null" {
		event.Location = loc
	}

	if ctx := getStringFromInterface(eventMap["context"]); ctx != nil && *ctx != "" && *ctx != "null" {
		event.Context = *ctx
	}

	return event
}

func parseHistoricalEvents(raw map[string]interface{}, logger zerolog.Logger) []entity.EventInfo {
	eventsData, ok := raw["historical_events"].([]interface{})
	if !ok {
		return nil
	}

	var events []entity.EventInfo
	seenEvents := make(map[string]bool)
	for _, eventItem := range eventsData {
		eventMap, ok := eventItem.(map[string]interface{})
		if !ok {
			continue
		}
		if event := parseEventItem(eventMap, seenEvents, logger); event != nil {
			events = append(events, *event)
		}
	}
	return events
}

// validateReferenceYear validates and returns a reference year if valid
func validateReferenceYear(year *int, refTitle string, logger zerolog.Logger) *int {
	if year == nil {
		return nil
	}
	currentYear := time.Now().Year()
	if *year > 0 && *year <= currentYear+1 {
		return year
	}
	logger.Debug().Int("year", *year).Int("current_year", currentYear).
		Str("title", refTitle).Msg("Reference year seems invalid, skipping")
	return nil
}

// parseRefItem parses a single reference item from the raw data
func parseRefItem(refMap map[string]interface{}, logger zerolog.Logger) *entity.ReferenceInfo {
	refTitle := getStringValueFromInterface(refMap["title"])
	refTypeStr := getStringValueFromInterface(refMap["type"])
	refAuthor := getStringFromInterface(refMap["author"])
	refYear := getIntFromInterface(refMap["year"])
	refURL := getStringFromInterface(refMap["url"])

	// Filter out objects where all fields are null or empty
	// This handles cases where LLM returns {"title": "null", "author": "null", "year": 2024, "type": "null"}
	hasValidTitle := refTitle != "" && !strings.EqualFold(refTitle, "null")
	hasValidType := refTypeStr != "" && !strings.EqualFold(refTypeStr, "null")
	hasValidAuthor := refAuthor != nil && *refAuthor != "" && !strings.EqualFold(*refAuthor, "null")
	hasValidYear := refYear != nil && *refYear > 0
	hasValidURL := refURL != nil && *refURL != "" && !strings.EqualFold(*refURL, "null")

	// If all fields are null/empty, skip this reference
	if !hasValidTitle && !hasValidType && !hasValidAuthor && !hasValidYear && !hasValidURL {
		return nil
	}

	// Title is required, so if it's null/empty, skip
	if !hasValidTitle {
		return nil
	}

	ref := &entity.ReferenceInfo{
		Title: refTitle,
		Type:  normalizeReferenceType(refTypeStr),
	}

	if hasValidAuthor {
		ref.Author = refAuthor
	}

	if year := validateReferenceYear(refYear, refTitle, logger); year != nil {
		ref.Year = year
	}

	if hasValidURL {
		ref.URL = refURL
	}

	return ref
}

func parseReferences(raw map[string]interface{}, logger zerolog.Logger) []entity.ReferenceInfo {
	refsData, ok := raw["references"].([]interface{})
	if !ok {
		return nil
	}

	var refs []entity.ReferenceInfo
	for _, refItem := range refsData {
		refMap, ok := refItem.(map[string]interface{})
		if !ok {
			continue
		}
		if ref := parseRefItem(refMap, logger); ref != nil {
			refs = append(refs, *ref)
		}
	}
	return refs
}

// parseSimpleStringField parses a simple string field from raw data
func parseSimpleStringField(raw map[string]interface{}, fieldName string) *string {
	if val := getStringFromInterface(raw[fieldName]); val != nil && *val != "null" {
		return val
	}
	return nil
}

// parseIdentifierField parses and normalizes identifier fields (ISBN, ISSN, DOI)
func parseIdentifierField(raw map[string]interface{}, fieldName string, normalizeFunc func(string) string) *string {
	val := getStringFromInterface(raw[fieldName])
	if val == nil || *val == "null" {
		return nil
	}
	if normalized := normalizeFunc(*val); normalized != "" {
		return &normalized
	}
	return nil
}

// parseDocumentDate parses the document_date field
func parseDocumentDate(raw map[string]interface{}, logger zerolog.Logger) *time.Time {
	docDateStr := getStringFromInterface(raw["document_date"])
	if docDateStr == nil || *docDateStr == "" || *docDateStr == "null" {
		return nil
	}
	docDate, err := time.Parse("2006-01-02", *docDateStr)
	if err != nil {
		logger.Debug().Str("date_str", *docDateStr).Err(err).Msg("Failed to parse document date, skipping")
		return nil
	}
	return &docDate
}

// parseSimpleFields parses all simple string fields into the AIContext
func parseSimpleFields(raw map[string]interface{}, aiContext *entity.AIContext) {
	aiContext.Publisher = parseSimpleStringField(raw, "publisher")
	aiContext.PublicationPlace = parseSimpleStringField(raw, "publication_place")
	aiContext.Edition = parseSimpleStringField(raw, "edition")
	// Additional validation: ensure no "null" strings are stored
	if aiContext.Publisher != nil && strings.EqualFold(*aiContext.Publisher, "null") {
		aiContext.Publisher = nil
	}
	if aiContext.PublicationPlace != nil && strings.EqualFold(*aiContext.PublicationPlace, "null") {
		aiContext.PublicationPlace = nil
	}
	if aiContext.Edition != nil && strings.EqualFold(*aiContext.Edition, "null") {
		aiContext.Edition = nil
	}
}

// parseIdentifierFields parses all identifier fields into the AIContext
func parseIdentifierFields(raw map[string]interface{}, aiContext *entity.AIContext) {
	aiContext.ISBN = parseIdentifierField(raw, "isbn", normalizeISBN)
	aiContext.ISSN = parseIdentifierField(raw, "issn", normalizeISSN)
	aiContext.DOI = parseIdentifierField(raw, "doi", normalizeDOI)
}

// parseOriginalLanguage parses the original_language field
func parseOriginalLanguage(raw map[string]interface{}, aiContext *entity.AIContext) {
	if origLang := getStringFromInterface(raw["original_language"]); origLang != nil {
		if strings.EqualFold(*origLang, "null") {
			aiContext.OriginalLanguage = nil
		} else {
			aiContext.OriginalLanguage = origLang
		}
	}
}

// parseComplexFields parses all complex array/list fields into the AIContext
func parseComplexFields(raw map[string]interface{}, aiContext *entity.AIContext, logger zerolog.Logger) {
	aiContext.Locations = parseLocations(raw)
	aiContext.PeopleMentioned = parsePeople(raw, logger)
	aiContext.Organizations = parseOrganizations(raw)
	aiContext.HistoricalEvents = parseHistoricalEvents(raw, logger)
	aiContext.References = parseReferences(raw, logger)
}

// parseAIContextJSON parses the JSON response from ExtractContextualInfo into an AIContext entity.
// This function uses the robust JSONParser for reliable parsing with retry capability.
// fileLastModified is optional and used to validate publication year.
func parseAIContextJSON(jsonStr string, logger zerolog.Logger, fileLastModified *time.Time) (*entity.AIContext, error) {
	// Use robust JSON parser with retry capability
	parser := NewJSONParser(logger)

	// Parse JSON into intermediate structure with flexible types
	var raw map[string]interface{}
	if err := parser.ParseJSON(context.Background(), jsonStr, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse AIContext JSON: %w", err)
	}

	// Convert to AIContext
	aiContext := entity.NewAIContext()

	// Parse authors
	aiContext.Authors = parseAuthors(raw, logger)

	// Simple fields - filter out generic placeholders
	aiContext.Editors = filterGenericPlaceholders(getStringArrayFromInterface(raw["editors"]))
	aiContext.Translators = filterGenericPlaceholders(getStringArrayFromInterface(raw["translators"]))
	aiContext.Contributors = parseContributors(raw, aiContext.Authors, logger)

	// Parse simple string fields
	parseSimpleFields(raw, aiContext)

	// Publication year
	aiContext.PublicationYear = validatePublicationYear(getIntFromInterface(raw["publication_year"]), fileLastModified, logger)

	// Parse identifier fields
	parseIdentifierFields(raw, aiContext)

	// Original language
	parseOriginalLanguage(raw, aiContext)

	// String or array fields
	aiContext.Genre = parseStringOrArrayField(raw, "genre")
	aiContext.Subject = parseStringOrArrayField(raw, "subject")
	aiContext.Audience = parseStringOrArrayField(raw, "audience")
	aiContext.HistoricalPeriod = parseHistoricalPeriod(raw)

	// Document date
	aiContext.DocumentDate = parseDocumentDate(raw, logger)

	// Complex fields
	parseComplexFields(raw, aiContext, logger)

	// Set metadata
	aiContext.ExtractedAt = time.Now()
	aiContext.Source = "llm_rag"
	aiContext.Confidence = 0.8 // Default confidence

	return aiContext, nil
}

// filterGenericPlaceholders filters out generic placeholder names like "Traductor 1", "Contribuidor 1", etc.
func filterGenericPlaceholders(items []string) []string {
	if items == nil {
		return nil
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		// Filter out generic placeholders
		if !isGenericPlaceholder(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// Helper function to check if a string contains only digits
func isAllDigits(s string) bool {
	if len(s) == 0 || len(s) > 2 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Helper function to check if a string matches a numbered pattern
func matchesNumberedPattern(s string, pattern string) bool {
	prefix := pattern + " "
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := strings.TrimPrefix(s, prefix)
	return isAllDigits(rest)
}

// isGenericPlaceholder checks if a string is a generic placeholder like "Traductor 1", "Contribuidor 1", etc.
func isGenericPlaceholder(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))

	// Exact match patterns
	exactPatterns := map[string]bool{
		"traductor 1": true, "traductor 2": true, "traductor 3": true,
		"translator 1": true, "translator 2": true, "translator 3": true,
		"contribuidor 1": true, "contribuidor 2": true, "contribuidor 3": true,
		"contributor 1": true, "contributor 2": true, "contributor 3": true,
		"contribuyente 1": true, "contribuyente 2": true, "contribuyente 3": true,
		"editor 1": true, "editor 2": true, "editor 3": true,
		"autor 1": true, "autor 2": true, "autor 3": true,
		"author 1": true, "author 2": true, "author 3": true,
		"persona 1": true, "persona 2": true, "persona 3": true,
		"person 1": true, "person 2": true, "person 3": true,
		"nombre 1": true, "nombre 2": true, "nombre 3": true,
		"name 1": true, "name 2": true, "name 3": true,
	}

	if exactPatterns[s] {
		return true
	}

	// Numbered patterns (e.g., "Traductor 1", "Contribuidor 2")
	numberedPatterns := []string{
		"traductor", "translator", "contribuidor", "contributor",
		"contribuyente", "editor", "autor", "author",
		"persona", "person", "nombre", "name",
	}

	for _, pattern := range numberedPatterns {
		if matchesNumberedPattern(s, pattern) {
			return true
		}
	}

	return false
}

// normalizeISBN normalizes and validates ISBN format.
// Removes hyphens and spaces, validates length (10 or 13 digits).
func normalizeISBN(isbn string) string {
	// Remove hyphens, spaces, and convert to uppercase
	cleaned := strings.ReplaceAll(isbn, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ToUpper(cleaned)

	// Remove "ISBN" prefix if present
	cleaned = strings.TrimPrefix(cleaned, "ISBN")
	cleaned = strings.TrimSpace(cleaned)

	// Validate: ISBN-10 is 10 chars, ISBN-13 is 13 chars
	// Allow X as check digit for ISBN-10
	if len(cleaned) == 10 || len(cleaned) == 13 {
		// Basic validation: should be mostly digits (ISBN-10 can end with X)
		valid := true
		for i, r := range cleaned {
			if i == len(cleaned)-1 && r == 'X' && len(cleaned) == 10 {
				continue // X is valid as last char in ISBN-10
			}
			if r < '0' || r > '9' {
				valid = false
				break
			}
		}
		if valid {
			return cleaned
		}
	}

	// If doesn't match format, return empty (will be filtered)
	return ""
}

// normalizeISSN normalizes and validates ISSN format.
// ISSN is 8 characters: XXXX-XXXX (with hyphen) or XXXX XXXX (with space).
func normalizeISSN(issn string) string {
	// Remove hyphens and spaces
	cleaned := strings.ReplaceAll(issn, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ToUpper(cleaned)

	// Remove "ISSN" prefix if present
	cleaned = strings.TrimPrefix(cleaned, "ISSN")
	cleaned = strings.TrimSpace(cleaned)

	// Validate: ISSN is 8 characters, last can be X
	if len(cleaned) == 8 {
		valid := true
		for i, r := range cleaned {
			if i == 7 && r == 'X' {
				continue // X is valid as last char
			}
			if r < '0' || r > '9' {
				valid = false
				break
			}
		}
		if valid {
			// Format as XXXX-XXXX
			return cleaned[:4] + "-" + cleaned[4:]
		}
	}

	return ""
}

// normalizeDOI normalizes and validates DOI format.
// DOI format: 10.XXXX/YYYY or doi:10.XXXX/YYYY
func normalizeDOI(doi string) string {
	cleaned := strings.TrimSpace(doi)
	cleaned = strings.ToLower(cleaned)

	// Remove "doi:" prefix if present
	cleaned = strings.TrimPrefix(cleaned, "doi:")
	cleaned = strings.TrimPrefix(cleaned, "doi")
	cleaned = strings.TrimSpace(cleaned)

	// DOI should start with "10."
	if strings.HasPrefix(cleaned, "10.") {
		// Basic validation: should have format 10.XXXX/YYYY
		parts := strings.Split(cleaned, "/")
		if len(parts) == 2 && len(parts[0]) > 3 && len(parts[1]) > 0 {
			return cleaned
		}
	}

	return ""
}

// normalizeOrganizationType normalizes organization types to standard values.
func normalizeOrganizationType(orgType string) string {
	orgType = strings.ToLower(strings.TrimSpace(orgType))

	// Map common variations to standard types
	typeMap := map[string]string{
		"iglesia":      "iglesia",
		"church":       "iglesia",
		"universidad":  "universidad",
		"university":   "universidad",
		"gobierno":     "gobierno",
		"government":   "gobierno",
		"empresa":      "empresa",
		"company":      "empresa",
		"organización": orgTypeOrganizacion,
		"organization": orgTypeOrganizacion,
		"institución":  orgTypeInstitucion,
		"institution":  orgTypeInstitucion,
		"asociación":   orgTypeAsociacion,
		"association":  orgTypeAsociacion,
		"fundación":    orgTypeFundacion,
		"foundation":   orgTypeFundacion,
		"ong":          "ong",
		"npo":          "ong",
		"nonprofit":    "ong",
	}

	if normalized, ok := typeMap[orgType]; ok {
		return normalized
	}

	// If not in map but not empty/null, return original (capitalized)
	if orgType != "" && orgType != "null" {
		return strings.Title(orgType)
	}

	// Default to empty string if invalid
	return ""
}

// normalizeReferenceType normalizes reference types to standard values.
func normalizeReferenceType(refType string) string {
	refType = strings.ToLower(strings.TrimSpace(refType))

	// Map common variations to standard types
	typeMap := map[string]string{
		"libro":       "libro",
		"book":        "libro",
		"artículo":    refTypeArticulo,
		"article":     refTypeArticulo,
		"sitio web":   refTypeSitioWeb,
		"website":     refTypeSitioWeb,
		"web":         refTypeSitioWeb,
		"conferencia": "conferencia",
		"conference":  "conferencia",
		"tesis":       "tesis",
		"thesis":      "tesis",
		"informe":     "informe",
		"report":      "informe",
		"documento":   "documento",
		"document":    "documento",
		"papel":       "papel",
		"paper":       "papel",
		"otro":        "otro",
		"other":       "otro",
	}

	if normalized, ok := typeMap[refType]; ok {
		return normalized
	}

	// If not in map but not empty/null, return original (capitalized)
	if refType != "" && refType != "null" {
		return strings.Title(refType)
	}

	// Default to "otro" if invalid
	return "otro"
}
