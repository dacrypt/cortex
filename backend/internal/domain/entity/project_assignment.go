package entity

import "time"

// ProjectAssignmentStatus represents the state of a project assignment.
type ProjectAssignmentStatus string

const (
	ProjectAssignmentAuto      ProjectAssignmentStatus = "auto"
	ProjectAssignmentSuggested ProjectAssignmentStatus = "suggested"
	ProjectAssignmentRejected  ProjectAssignmentStatus = "rejected"
	ProjectAssignmentManual    ProjectAssignmentStatus = "manual"
)

// ProjectAssignment represents a scored assignment between a file and a project.
// ProjectID may be empty for suggestions that have not been created as projects yet.
type ProjectAssignment struct {
	WorkspaceID WorkspaceID
	FileID      FileID
	ProjectID   ProjectID
	ProjectName string
	Score       float64
	Sources     []string
	Status      ProjectAssignmentStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsValid checks if the assignment status is valid.
func (s ProjectAssignmentStatus) IsValid() bool {
	switch s {
	case ProjectAssignmentAuto, ProjectAssignmentSuggested, ProjectAssignmentRejected, ProjectAssignmentManual:
		return true
	default:
		return false
	}
}
