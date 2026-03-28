// Package governance provides data governance services including PII detection and retention.
package governance

import (
	"context"
	"regexp"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// PIIType identifies the type of personally identifiable information.
type PIIType string

const (
	PIITypeEmail      PIIType = "email"
	PIITypePhone      PIIType = "phone"
	PIITypeSSN        PIIType = "ssn"
	PIITypeCreditCard PIIType = "credit_card"
	PIITypeName       PIIType = "name"
	PIITypeAddress    PIIType = "address"
	PIITypeIPAddress  PIIType = "ip_address"
)

// PIIAction defines what to do when PII is detected.
type PIIAction string

const (
	PIIActionRedact PIIAction = "redact"
	PIIActionHash   PIIAction = "hash"
	PIIActionFlag   PIIAction = "flag"
	PIIActionLog    PIIAction = "log"
)

// PIIRule defines a rule for detecting PII.
type PIIRule struct {
	Type    PIIType
	Pattern *regexp.Regexp
	Action  PIIAction
}

// PIIMatch represents a detected PII instance.
type PIIMatch struct {
	Type       PIIType `json:"type"`
	Value      string  `json:"value"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
	Redacted   string  `json:"redacted,omitempty"`
	Confidence float64 `json:"confidence"`
}

// PIIPolicy defines the PII handling policy.
type PIIPolicy struct {
	Enabled          bool
	DetectionRules   []PIIRule
	RetentionDays    int
	AnonymizeOnStore bool
	LogDetections    bool
}

// DefaultPIIPolicy returns a default PII policy.
func DefaultPIIPolicy() PIIPolicy {
	return PIIPolicy{
		Enabled:          true,
		DetectionRules:   defaultPIIRules(),
		RetentionDays:    90,
		AnonymizeOnStore: false,
		LogDetections:    true,
	}
}

func defaultPIIRules() []PIIRule {
	return []PIIRule{
		{
			Type:    PIITypeEmail,
			Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			Action:  PIIActionFlag,
		},
		{
			Type:    PIITypePhone,
			Pattern: regexp.MustCompile(`(\+\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
			Action:  PIIActionFlag,
		},
		{
			Type:    PIITypeSSN,
			Pattern: regexp.MustCompile(`\d{3}-\d{2}-\d{4}`),
			Action:  PIIActionRedact,
		},
		{
			Type:    PIITypeCreditCard,
			Pattern: regexp.MustCompile(`\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}`),
			Action:  PIIActionRedact,
		},
		{
			Type:    PIITypeIPAddress,
			Pattern: regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
			Action:  PIIActionFlag,
		},
	}
}

// PIIService handles PII detection and protection.
type PIIService struct {
	policy PIIPolicy
	logger zerolog.Logger
}

// NewPIIService creates a new PII service.
func NewPIIService(policy PIIPolicy, logger zerolog.Logger) *PIIService {
	return &PIIService{
		policy: policy,
		logger: logger.With().Str("component", "pii").Logger(),
	}
}

// ScanForPII scans content for personally identifiable information.
func (s *PIIService) ScanForPII(content string) []PIIMatch {
	if !s.policy.Enabled {
		return nil
	}

	var matches []PIIMatch

	for _, rule := range s.policy.DetectionRules {
		found := rule.Pattern.FindAllStringIndex(content, -1)
		for _, loc := range found {
			value := content[loc[0]:loc[1]]
			match := PIIMatch{
				Type:       rule.Type,
				Value:      value,
				StartPos:   loc[0],
				EndPos:     loc[1],
				Confidence: s.calculateConfidence(rule.Type, value),
			}

			if rule.Action == PIIActionRedact || rule.Action == PIIActionHash {
				match.Redacted = s.redactValue(rule.Type, value)
			}

			matches = append(matches, match)
		}
	}

	if s.policy.LogDetections && len(matches) > 0 {
		s.logger.Debug().
			Int("matches", len(matches)).
			Msg("PII detected in content")
	}

	return matches
}

// Redact redacts all PII from content.
func (s *PIIService) Redact(content string) string {
	if !s.policy.Enabled {
		return content
	}

	result := content

	for _, rule := range s.policy.DetectionRules {
		if rule.Action == PIIActionRedact {
			result = rule.Pattern.ReplaceAllStringFunc(result, func(match string) string {
				return s.redactValue(rule.Type, match)
			})
		}
	}

	return result
}

// ValidateForStorage checks if content is safe to store based on PII policy.
func (s *PIIService) ValidateForStorage(ctx context.Context, content string) (bool, []PIIMatch) {
	matches := s.ScanForPII(content)

	// Check for any matches that require redaction
	for _, match := range matches {
		for _, rule := range s.policy.DetectionRules {
			if rule.Type == match.Type && rule.Action == PIIActionRedact {
				return false, matches
			}
		}
	}

	return true, matches
}

// PrepareForStorage prepares content for storage by applying PII policy.
func (s *PIIService) PrepareForStorage(ctx context.Context, content string) (string, []PIIMatch) {
	matches := s.ScanForPII(content)

	if s.policy.AnonymizeOnStore {
		return s.Redact(content), matches
	}

	return content, matches
}

// Helper methods

func (s *PIIService) calculateConfidence(piiType PIIType, value string) float64 {
	// Simple confidence scoring based on pattern match quality
	switch piiType {
	case PIITypeEmail:
		if strings.Contains(value, "@") && strings.Contains(value, ".") {
			return 0.95
		}
		return 0.7
	case PIITypeSSN:
		return 0.99 // Very specific pattern
	case PIITypeCreditCard:
		if s.validateLuhn(value) {
			return 0.99
		}
		return 0.6
	case PIITypePhone:
		// Check for realistic phone number format
		digits := regexp.MustCompile(`\d`).FindAllString(value, -1)
		if len(digits) >= 10 && len(digits) <= 15 {
			return 0.85
		}
		return 0.5
	case PIITypeIPAddress:
		return 0.8
	default:
		return 0.7
	}
}

func (s *PIIService) redactValue(piiType PIIType, value string) string {
	switch piiType {
	case PIITypeEmail:
		parts := strings.Split(value, "@")
		if len(parts) == 2 {
			return "[REDACTED_EMAIL]@" + parts[1]
		}
		return "[REDACTED_EMAIL]"
	case PIITypeSSN:
		return "***-**-" + value[len(value)-4:]
	case PIITypeCreditCard:
		return "****-****-****-" + value[len(value)-4:]
	case PIITypePhone:
		return "[REDACTED_PHONE]"
	case PIITypeIPAddress:
		return "[REDACTED_IP]"
	default:
		return "[REDACTED]"
	}
}

// validateLuhn performs a simple Luhn check for credit card numbers.
func (s *PIIService) validateLuhn(cardNumber string) bool {
	// Remove non-digits
	digits := regexp.MustCompile(`\d`).FindAllString(cardNumber, -1)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	var sum int
	alt := false

	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i][0] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}

	return sum%10 == 0
}

// PIIScanResult contains the result of a PII scan on a file.
type PIIScanResult struct {
	FileID       entity.FileID `json:"file_id"`
	RelativePath string        `json:"relative_path"`
	Matches      []PIIMatch    `json:"matches"`
	RiskLevel    string        `json:"risk_level"` // "low", "medium", "high", "critical"
	SafeToStore  bool          `json:"safe_to_store"`
}

// ScanFile scans a file's content for PII.
func (s *PIIService) ScanFile(ctx context.Context, fileID entity.FileID, relativePath, content string) *PIIScanResult {
	matches := s.ScanForPII(content)

	result := &PIIScanResult{
		FileID:       fileID,
		RelativePath: relativePath,
		Matches:      matches,
		SafeToStore:  true,
	}

	// Determine risk level
	if len(matches) == 0 {
		result.RiskLevel = "low"
	} else {
		highRiskCount := 0
		for _, match := range matches {
			switch match.Type {
			case PIITypeSSN, PIITypeCreditCard:
				highRiskCount++
				result.SafeToStore = false
			}
		}

		if highRiskCount > 0 {
			result.RiskLevel = "critical"
		} else if len(matches) > 5 {
			result.RiskLevel = "high"
		} else {
			result.RiskLevel = "medium"
		}
	}

	return result
}
