// Package entity contains domain entities.
package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// FeedbackID uniquely identifies a feedback entry.
type FeedbackID string

// NewFeedbackID creates a new unique FeedbackID.
func NewFeedbackID() FeedbackID {
	return FeedbackID(uuid.New().String())
}

// String returns the string representation of FeedbackID.
func (id FeedbackID) String() string {
	return string(id)
}

// PreferenceID uniquely identifies a learned preference.
type PreferenceID string

// NewPreferenceID creates a new unique PreferenceID.
func NewPreferenceID() PreferenceID {
	return PreferenceID(uuid.New().String())
}

// String returns the string representation of PreferenceID.
func (id PreferenceID) String() string {
	return string(id)
}

// FeedbackActionType represents the type of user action.
type FeedbackActionType string

const (
	// FeedbackActionAccepted indicates user accepted a suggestion.
	FeedbackActionAccepted FeedbackActionType = "accepted"
	// FeedbackActionRejected indicates user rejected a suggestion.
	FeedbackActionRejected FeedbackActionType = "rejected"
	// FeedbackActionCorrected indicates user corrected a suggestion.
	FeedbackActionCorrected FeedbackActionType = "corrected"
	// FeedbackActionModified indicates user modified a suggestion.
	FeedbackActionModified FeedbackActionType = "modified"
	// FeedbackActionIgnored indicates user ignored a suggestion.
	FeedbackActionIgnored FeedbackActionType = "ignored"
)

// SuggestionType represents what kind of suggestion was made.
type SuggestionType string

const (
	// SuggestionTypeTag indicates a tag suggestion.
	SuggestionTypeTag SuggestionType = "tag"
	// SuggestionTypeProject indicates a project suggestion.
	SuggestionTypeProject SuggestionType = "project"
	// SuggestionTypeCategory indicates a category suggestion.
	SuggestionTypeCategory SuggestionType = "category"
	// SuggestionTypeSummary indicates a summary suggestion.
	SuggestionTypeSummary SuggestionType = "summary"
	// SuggestionTypeCluster indicates a cluster suggestion.
	SuggestionTypeCluster SuggestionType = "cluster"
	// SuggestionTypeTaxonomy indicates a taxonomy suggestion.
	SuggestionTypeTaxonomy SuggestionType = "taxonomy"
	// SuggestionTypeSFS indicates an SFS command suggestion.
	SuggestionTypeSFS SuggestionType = "sfs"
)

// FeedbackContext captures the context when feedback was given.
type FeedbackContext struct {
	// File metadata at time of decision
	FileID       string   `json:"file_id,omitempty"`
	FilePath     string   `json:"file_path,omitempty"`
	FileType     string   `json:"file_type,omitempty"`
	FileSize     int64    `json:"file_size,omitempty"`
	FileTags     []string `json:"file_tags,omitempty"`
	FileProjects []string `json:"file_projects,omitempty"`

	// Surrounding context
	FolderPath     string   `json:"folder_path,omitempty"`
	SiblingFiles   []string `json:"sibling_files,omitempty"`
	RecentActivity []string `json:"recent_activity,omitempty"`

	// Session context
	SessionID   string `json:"session_id,omitempty"`
	TimeOfDay   string `json:"time_of_day,omitempty"`
	DayOfWeek   string `json:"day_of_week,omitempty"`
	CommandUsed string `json:"command_used,omitempty"`
}

// ToJSON converts FeedbackContext to JSON string.
func (c *FeedbackContext) ToJSON() (string, error) {
	if c == nil {
		return "{}", nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into FeedbackContext.
func (c *FeedbackContext) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*c = FeedbackContext{}
		return nil
	}
	return json.Unmarshal([]byte(data), c)
}

// Suggestion represents the original suggestion made.
type Suggestion struct {
	Type       SuggestionType `json:"type"`
	Value      string         `json:"value"`       // The suggested value (e.g., tag name, project name)
	Confidence float64        `json:"confidence"`  // AI confidence in the suggestion
	Reasoning  string         `json:"reasoning"`   // Why this was suggested
	Source     string         `json:"source"`      // What generated this (e.g., "clustering", "llm", "pattern")
	Metadata   map[string]any `json:"metadata"`    // Additional metadata
}

// ToJSON converts Suggestion to JSON string.
func (s *Suggestion) ToJSON() (string, error) {
	if s == nil {
		return "{}", nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into Suggestion.
func (s *Suggestion) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*s = Suggestion{}
		return nil
	}
	return json.Unmarshal([]byte(data), s)
}

// Correction represents what the user chose instead.
type Correction struct {
	Value    string         `json:"value"`    // What user chose instead
	Reason   string         `json:"reason"`   // User-provided reason (if any)
	Metadata map[string]any `json:"metadata"` // Additional correction context
}

// ToJSON converts Correction to JSON string.
func (c *Correction) ToJSON() (string, error) {
	if c == nil {
		return "{}", nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into Correction.
func (c *Correction) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*c = Correction{}
		return nil
	}
	return json.Unmarshal([]byte(data), c)
}

// UserFeedback records a user's response to a suggestion.
type UserFeedback struct {
	ID          FeedbackID
	WorkspaceID WorkspaceID
	ActionType  FeedbackActionType
	Suggestion  *Suggestion
	Correction  *Correction        // Non-nil if action is corrected/modified
	Context     *FeedbackContext   // Context at time of decision
	ResponseTime int64             // Milliseconds to respond (measure uncertainty)
	CreatedAt   time.Time
}

// NewUserFeedback creates a new feedback entry.
func NewUserFeedback(
	workspaceID WorkspaceID,
	actionType FeedbackActionType,
	suggestion *Suggestion,
) *UserFeedback {
	return &UserFeedback{
		ID:          NewFeedbackID(),
		WorkspaceID: workspaceID,
		ActionType:  actionType,
		Suggestion:  suggestion,
		CreatedAt:   time.Now(),
	}
}

// WithCorrection adds correction data to the feedback.
func (f *UserFeedback) WithCorrection(correction *Correction) *UserFeedback {
	f.Correction = correction
	return f
}

// WithContext adds context data to the feedback.
func (f *UserFeedback) WithContext(ctx *FeedbackContext) *UserFeedback {
	f.Context = ctx
	return f
}

// WithResponseTime adds response time measurement.
func (f *UserFeedback) WithResponseTime(ms int64) *UserFeedback {
	f.ResponseTime = ms
	return f
}

// PreferenceType represents what kind of preference was learned.
type PreferenceType string

const (
	// PreferenceTypeProjectNaming indicates learned project naming preferences.
	PreferenceTypeProjectNaming PreferenceType = "project_naming"
	// PreferenceTypeCategorization indicates learned categorization preferences.
	PreferenceTypeCategorization PreferenceType = "categorization"
	// PreferenceTypeTagging indicates learned tagging preferences.
	PreferenceTypeTagging PreferenceType = "tagging"
	// PreferenceTypeClustering indicates learned clustering preferences.
	PreferenceTypeClustering PreferenceType = "clustering"
	// PreferenceTypeOrganization indicates learned organization preferences.
	PreferenceTypeOrganization PreferenceType = "organization"
	// PreferenceTypeWorkflow indicates learned workflow preferences.
	PreferenceTypeWorkflow PreferenceType = "workflow"
)

// PreferencePattern describes when this preference applies.
type PreferencePattern struct {
	// File patterns
	FileExtensions []string `json:"file_extensions,omitempty"` // e.g., [".pdf", ".docx"]
	FolderPatterns []string `json:"folder_patterns,omitempty"` // e.g., ["invoices/*", "*/reports"]
	FileSizeRange  []int64  `json:"file_size_range,omitempty"` // [min, max] bytes

	// Content patterns
	ContentTypes   []string `json:"content_types,omitempty"`   // MIME types
	Keywords       []string `json:"keywords,omitempty"`        // Keywords in content/name
	Languages      []string `json:"languages,omitempty"`       // Detected languages

	// Context patterns
	TimePatterns   []string `json:"time_patterns,omitempty"`   // e.g., ["morning", "weekday"]
	ProjectContext []string `json:"project_context,omitempty"` // When in these projects
	TagContext     []string `json:"tag_context,omitempty"`     // When has these tags

	// Custom patterns (for complex rules)
	CustomRules    map[string]any `json:"custom_rules,omitempty"`
}

// ToJSON converts PreferencePattern to JSON string.
func (p *PreferencePattern) ToJSON() (string, error) {
	if p == nil {
		return "{}", nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into PreferencePattern.
func (p *PreferencePattern) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*p = PreferencePattern{}
		return nil
	}
	return json.Unmarshal([]byte(data), p)
}

// PreferenceBehavior describes the learned behavior to apply.
type PreferenceBehavior struct {
	// For tagging
	PreferredTags    []string `json:"preferred_tags,omitempty"`
	AvoidedTags      []string `json:"avoided_tags,omitempty"`

	// For project assignment
	PreferredProjects []string `json:"preferred_projects,omitempty"`
	AvoidedProjects   []string `json:"avoided_projects,omitempty"`

	// For naming
	NamingTemplate    string   `json:"naming_template,omitempty"`
	NamingPrefixes    []string `json:"naming_prefixes,omitempty"`
	NamingSuffixes    []string `json:"naming_suffixes,omitempty"`

	// For organization
	PreferredStructure string         `json:"preferred_structure,omitempty"` // flat, hierarchical, by-date, by-type
	GroupingPreference string         `json:"grouping_preference,omitempty"` // how to group files

	// Action preferences
	AutoAccept     bool    `json:"auto_accept,omitempty"`     // Automatically accept matching suggestions
	ConfidenceBoost float64 `json:"confidence_boost,omitempty"` // Boost confidence for matching patterns

	// Custom behaviors
	CustomActions map[string]any `json:"custom_actions,omitempty"`
}

// ToJSON converts PreferenceBehavior to JSON string.
func (b *PreferenceBehavior) ToJSON() (string, error) {
	if b == nil {
		return "{}", nil
	}
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into PreferenceBehavior.
func (b *PreferenceBehavior) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*b = PreferenceBehavior{}
		return nil
	}
	return json.Unmarshal([]byte(data), b)
}

// LearnedPreference represents a preference learned from user feedback.
type LearnedPreference struct {
	ID          PreferenceID
	WorkspaceID WorkspaceID
	Type        PreferenceType
	Pattern     *PreferencePattern  // When this preference applies
	Behavior    *PreferenceBehavior // What behavior to apply
	Confidence  float64             // How confident we are in this preference (0-1)
	Examples    int                 // Number of examples that reinforced this
	LastUsed    *time.Time          // When this preference was last applied
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewLearnedPreference creates a new learned preference.
func NewLearnedPreference(
	workspaceID WorkspaceID,
	prefType PreferenceType,
	pattern *PreferencePattern,
	behavior *PreferenceBehavior,
) *LearnedPreference {
	now := time.Now()
	return &LearnedPreference{
		ID:          NewPreferenceID(),
		WorkspaceID: workspaceID,
		Type:        prefType,
		Pattern:     pattern,
		Behavior:    behavior,
		Confidence:  0.5, // Start with medium confidence
		Examples:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Reinforce increases confidence based on new supporting evidence.
func (p *LearnedPreference) Reinforce() {
	p.Examples++
	// Increase confidence asymptotically toward 1.0
	p.Confidence = p.Confidence + (1.0-p.Confidence)*0.1
	now := time.Now()
	p.LastUsed = &now
	p.UpdatedAt = now
}

// Weaken decreases confidence based on contradicting evidence.
func (p *LearnedPreference) Weaken() {
	// Decrease confidence but don't go below 0.1
	p.Confidence = p.Confidence * 0.9
	if p.Confidence < 0.1 {
		p.Confidence = 0.1
	}
	p.UpdatedAt = time.Now()
}

// MatchesContext checks if this preference applies to the given context.
func (p *LearnedPreference) MatchesContext(ctx *FeedbackContext) bool {
	if p.Pattern == nil || ctx == nil {
		return false
	}

	// Check file extensions
	if len(p.Pattern.FileExtensions) > 0 && ctx.FileType != "" {
		found := false
		for _, ext := range p.Pattern.FileExtensions {
			if ext == ctx.FileType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check folder patterns (simplified - would need glob matching)
	if len(p.Pattern.FolderPatterns) > 0 && ctx.FolderPath != "" {
		found := false
		for _, pattern := range p.Pattern.FolderPatterns {
			if ctx.FolderPath == pattern { // Simplified matching
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
