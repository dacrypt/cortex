package entity

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProjectID uniquely identifies a project.
type ProjectID string

// NewProjectID creates a new unique ProjectID.
func NewProjectID() ProjectID {
	return ProjectID(uuid.New().String())
}

// String returns the string representation of ProjectID.
func (id ProjectID) String() string {
	return string(id)
}

// ProjectNature represents the nature/type of a project.
// This determines how the project should be treated and what behaviors apply.
type ProjectNature string

const (
	// Writing projects
	NatureWritingBook        ProjectNature = "writing.book"
	NatureWritingThesis      ProjectNature = "writing.thesis"
	NatureWritingArticle     ProjectNature = "writing.article"
	NatureWritingDocumentation ProjectNature = "writing.documentation"
	NatureWritingBlog        ProjectNature = "writing.blog"

	// Collections
	NatureCollectionLibrary   ProjectNature = "collection.library"
	NatureCollectionArchive   ProjectNature = "collection.archive"
	NatureCollectionReference ProjectNature = "collection.reference"

	// Development
	NatureDevelopmentSoftware ProjectNature = "development.software"
	NatureDevelopmentERP      ProjectNature = "development.erp"
	NatureDevelopmentWebsite  ProjectNature = "development.website"
	NatureDevelopmentAPI      ProjectNature = "development.api"

	// Management
	NatureManagementBusiness  ProjectNature = "management.business"
	NatureManagementPersonal  ProjectNature = "management.personal"
	NatureManagementFamily    ProjectNature = "management.family"

	// Hierarchical
	NatureHierarchicalParent  ProjectNature = "hierarchical.parent"
	NatureHierarchicalChild   ProjectNature = "hierarchical.child"
	NatureHierarchicalPortfolio ProjectNature = "hierarchical.portfolio"

	// Purchases
	NaturePurchaseVehicle    ProjectNature = "purchase.vehicle"
	NaturePurchaseProperty   ProjectNature = "purchase.property"
	NaturePurchaseEquipment  ProjectNature = "purchase.equipment"
	NaturePurchaseService    ProjectNature = "purchase.service"

	// Education
	NatureEducationCourse     ProjectNature = "education.course"
	NatureEducationResearch   ProjectNature = "education.research"
	NatureEducationSchool    ProjectNature = "education.school"

	// Events
	NatureEventWedding        ProjectNature = "event.wedding"
	NatureEventTravel         ProjectNature = "event.travel"
	NatureEventConference     ProjectNature = "event.conference"

	// Reference
	NatureReferenceKnowledge  ProjectNature = "reference.knowledge_base"
	NatureReferenceTemplate   ProjectNature = "reference.template"
	NatureReferenceArchive    ProjectNature = "reference.archive"

	// Generic/Unknown
	NatureGeneric            ProjectNature = "generic"
)

// String returns the string representation of ProjectNature.
func (n ProjectNature) String() string {
	return string(n)
}

// IsValid checks if the nature is a valid value.
func (n ProjectNature) IsValid() bool {
	valid := []ProjectNature{
		NatureWritingBook, NatureWritingThesis, NatureWritingArticle,
		NatureWritingDocumentation, NatureWritingBlog,
		NatureCollectionLibrary, NatureCollectionArchive, NatureCollectionReference,
		NatureDevelopmentSoftware, NatureDevelopmentERP, NatureDevelopmentWebsite, NatureDevelopmentAPI,
		NatureManagementBusiness, NatureManagementPersonal, NatureManagementFamily,
		NatureHierarchicalParent, NatureHierarchicalChild, NatureHierarchicalPortfolio,
		NaturePurchaseVehicle, NaturePurchaseProperty, NaturePurchaseEquipment, NaturePurchaseService,
		NatureEducationCourse, NatureEducationResearch, NatureEducationSchool,
		NatureEventWedding, NatureEventTravel, NatureEventConference,
		NatureReferenceKnowledge, NatureReferenceTemplate, NatureReferenceArchive,
		NatureGeneric,
	}
	for _, v := range valid {
		if n == v {
			return true
		}
	}
	return false
}

// ProjectAttributes contains additional attributes that influence project treatment.
type ProjectAttributes struct {
	Temporality   string                 `json:"temporality,omitempty"`   // "temporary" | "ongoing"
	Collaboration string                 `json:"collaboration,omitempty"` // "individual" | "team" | "organization"
	Priority      string                 `json:"priority,omitempty"`     // "low" | "medium" | "high" | "critical"
	Status        string                 `json:"status,omitempty"`       // "planning" | "active" | "on-hold" | "completed" | "archived"
	Visibility    string                 `json:"visibility,omitempty"`  // "private" | "shared" | "public"
	Metadata      map[string]interface{} `json:"metadata,omitempty"`     // Type-specific metadata
	
	// Unified entity metadata (for facet filtering)
	Tags            []string `json:"tags,omitempty"`            // Tags for the project
	Language        *string  `json:"language,omitempty"`        // Primary language
	Author          *string  `json:"author,omitempty"`           // Project author/owner
	Owner           *string  `json:"owner,omitempty"`           // Project owner
	Location        *string  `json:"location,omitempty"`        // Project location
	PublicationYear *int     `json:"publication_year,omitempty"` // Publication/creation year
	AISummary       *string  `json:"ai_summary,omitempty"`     // AI-generated summary
	AIKeywords      []string `json:"ai_keywords,omitempty"`     // AI-generated keywords
}

// ToJSON converts ProjectAttributes to JSON string.
func (a *ProjectAttributes) ToJSON() (string, error) {
	if a == nil {
		return "{}", nil
	}
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses JSON string into ProjectAttributes.
func (a *ProjectAttributes) FromJSON(data string) error {
	if data == "" || data == "{}" {
		*a = ProjectAttributes{}
		return nil
	}
	return json.Unmarshal([]byte(data), a)
}

// Project represents a hierarchical project in the knowledge graph.
type Project struct {
	ID          ProjectID
	WorkspaceID WorkspaceID
	Name        string
	Description string
	Nature      ProjectNature      // Nature/type of the project
	Attributes  *ProjectAttributes // Additional attributes (nullable)
	ParentID    *ProjectID         // Nil for root projects
	Path        string             // Hierarchical path: "parent/child/grandchild"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProject creates a new project with default nature (generic).
func NewProject(workspaceID WorkspaceID, name string, parentID *ProjectID) *Project {
	return NewProjectWithNature(workspaceID, name, parentID, NatureGeneric)
}

// NewProjectWithNature creates a new project with a specific nature.
func NewProjectWithNature(workspaceID WorkspaceID, name string, parentID *ProjectID, nature ProjectNature) *Project {
	now := time.Now()
	path := calculateProjectPath(name, parentID)

	// Validate nature
	if !nature.IsValid() {
		nature = NatureGeneric
	}

	return &Project{
		ID:          NewProjectID(),
		WorkspaceID: workspaceID,
		Name:        name,
		Nature:      nature,
		Attributes:  &ProjectAttributes{},
		ParentID:    parentID,
		Path:        path,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// IsRoot returns true if this is a root project (no parent).
func (p *Project) IsRoot() bool {
	return p.ParentID == nil
}

// calculateProjectPath computes the hierarchical path for a project.
// If parentID is nil, returns just the name.
// Otherwise, constructs path as "parent_path/name".
func calculateProjectPath(name string, parentID *ProjectID) string {
	// This is a placeholder - actual implementation will need to query parent
	// For now, if parent is nil, return name; otherwise return "parent/name"
	// The repository will handle full path calculation
	if parentID == nil {
		return name
	}
	// Repository will need to fetch parent and build full path
	return name // Temporary - repository will update this
}

// UpdatePath updates the project path based on parent hierarchy.
// This should be called by the repository after parent is set.
func (p *Project) UpdatePath(parentPath string) {
	if parentPath == "" {
		p.Path = p.Name
	} else {
		p.Path = filepath.Join(parentPath, p.Name)
	}
}

// GetPathComponents returns the path as a slice of components.
func (p *Project) GetPathComponents() []string {
	if p.Path == "" {
		return []string{p.Name}
	}
	return strings.Split(p.Path, "/")
}

// ProjectDocumentRole represents the role of a document in a project.
type ProjectDocumentRole string

const (
	// ProjectDocumentRolePrimary indicates the document is a primary document for the project.
	ProjectDocumentRolePrimary ProjectDocumentRole = "primary"
	// ProjectDocumentRoleReference indicates the document is referenced by the project.
	ProjectDocumentRoleReference ProjectDocumentRole = "reference"
	// ProjectDocumentRoleArchive indicates the document is archived for the project.
	ProjectDocumentRoleArchive ProjectDocumentRole = "archive"
)

// ProjectDocument represents the relationship between a project and a document.
type ProjectDocument struct {
	WorkspaceID WorkspaceID
	ProjectID   ProjectID
	DocumentID  DocumentID
	Role        ProjectDocumentRole
	AddedAt     time.Time
}

