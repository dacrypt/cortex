package entity

import (
	"time"
)

// SuggestedMetadata contains AI-suggested metadata that hasn't been confirmed by the user.
// This is separate from FileMetadata which contains confirmed metadata.
type SuggestedMetadata struct {
	FileID            FileID
	WorkspaceID       WorkspaceID
	RelativePath      string
	
	// Suggested tags (not yet confirmed)
	SuggestedTags     []SuggestedTag
	
	// Suggested projects/contexts (not yet confirmed)
	SuggestedProjects []SuggestedProject
	
	// Suggested taxonomic classifications
	SuggestedTaxonomy *SuggestedTaxonomy
	
	// Suggested additional metadata fields
	SuggestedFields   map[string]SuggestedField
	
	// Metadata about the suggestions
	Confidence        float64 // Overall confidence (0.0-1.0)
	Source            string  // "rag", "llm", "similarity", "pattern"
	GeneratedAt       time.Time
	UpdatedAt         time.Time
}

// SuggestedTag represents a suggested tag with confidence and reasoning.
type SuggestedTag struct {
	Tag         string
	Confidence  float64 // 0.0-1.0
	Reason      string  // Why this tag was suggested
	Source      string  // "content", "metadata", "similar_files", "llm"
	Category    string  // Optional category grouping
}

// SuggestedProject represents a suggested project assignment.
type SuggestedProject struct {
	ProjectID   *ProjectID // If project exists
	ProjectName string     // If new project suggested
	Confidence  float64    // 0.0-1.0
	Reason      string     // Why this project was suggested
	Source      string     // "rag", "llm", "similarity", "metadata"
	IsNew       bool       // Whether this would create a new project
}

// SuggestedTaxonomy contains taxonomic classifications suggested by AI.
type SuggestedTaxonomy struct {
	// Primary classification
	Category    string  // e.g., "document", "code", "media", "data"
	Subcategory string  // e.g., "legal", "financial", "technical"
	
	// Domain classification
	Domain      string  // e.g., "software", "business", "personal", "academic"
	Subdomain   string  // e.g., "web-development", "accounting", "research"
	
	// Content classification
	ContentType string  // e.g., "specification", "report", "source-code", "presentation"
	Purpose      string  // e.g., "reference", "working-draft", "final", "archive"
	
	// Semantic classification
	Topic       []string // Topics/themes (e.g., ["machine-learning", "python", "tutorial"])
	Audience    string   // e.g., "internal", "external", "public", "private"
	Language    string   // Content language
	
	// Confidence scores
	CategoryConfidence    float64
	DomainConfidence      float64
	ContentTypeConfidence float64
	
	// Reasoning
	Reasoning   string
	Source      string // "rag", "llm", "metadata"
}

// SuggestedField represents a suggested value for a metadata field.
type SuggestedField struct {
	FieldName   string
	Value       interface{} // Can be string, number, array, etc.
	Confidence  float64
	Reason      string
	Source      string
	FieldType   string // "string", "number", "array", "date", etc.
}

// AcceptTag accepts a suggested tag and moves it to confirmed metadata.
func (sm *SuggestedMetadata) AcceptTag(tag string) *SuggestedTag {
	for i, st := range sm.SuggestedTags {
		if st.Tag == tag {
			accepted := sm.SuggestedTags[i]
			sm.SuggestedTags = append(sm.SuggestedTags[:i], sm.SuggestedTags[i+1:]...)
			return &accepted
		}
	}
	return nil
}

// AcceptProject accepts a suggested project and returns it.
func (sm *SuggestedMetadata) AcceptProject(projectName string) *SuggestedProject {
	for i, sp := range sm.SuggestedProjects {
		if sp.ProjectName == projectName || (sp.ProjectID != nil && sp.ProjectName == projectName) {
			accepted := sm.SuggestedProjects[i]
			sm.SuggestedProjects = append(sm.SuggestedProjects[:i], sm.SuggestedProjects[i+1:]...)
			return &accepted
		}
	}
	return nil
}

// RejectTag removes a suggested tag.
func (sm *SuggestedMetadata) RejectTag(tag string) {
	for i, st := range sm.SuggestedTags {
		if st.Tag == tag {
			sm.SuggestedTags = append(sm.SuggestedTags[:i], sm.SuggestedTags[i+1:]...)
			return
		}
	}
}

// RejectProject removes a suggested project.
func (sm *SuggestedMetadata) RejectProject(projectName string) {
	for i, sp := range sm.SuggestedProjects {
		if sp.ProjectName == projectName {
			sm.SuggestedProjects = append(sm.SuggestedProjects[:i], sm.SuggestedProjects[i+1:]...)
			return
		}
	}
}

// HasSuggestions returns true if there are any suggestions.
func (sm *SuggestedMetadata) HasSuggestions() bool {
	return len(sm.SuggestedTags) > 0 ||
		len(sm.SuggestedProjects) > 0 ||
		sm.SuggestedTaxonomy != nil ||
		len(sm.SuggestedFields) > 0
}

// GetTopSuggestions returns the top N suggestions by confidence.
func (sm *SuggestedMetadata) GetTopSuggestions(n int) (tags []SuggestedTag, projects []SuggestedProject) {
	// Sort tags by confidence
	tags = make([]SuggestedTag, len(sm.SuggestedTags))
	copy(tags, sm.SuggestedTags)
	// TODO: Sort by confidence
	
	// Sort projects by confidence
	projects = make([]SuggestedProject, len(sm.SuggestedProjects))
	copy(projects, sm.SuggestedProjects)
	// TODO: Sort by confidence
	
	if len(tags) > n {
		tags = tags[:n]
	}
	if len(projects) > n {
		projects = projects[:n]
	}
	
	return tags, projects
}







