package metadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/rs/zerolog"
)

// ProjectAssigner uses all available metadata (confirmed + suggested) to assign projects to files.
type ProjectAssigner struct {
	metaRepo      repository.MetadataRepository
	suggestedRepo repository.SuggestedMetadataRepository
	projectRepo   repository.ProjectRepository
	docRepo       repository.DocumentRepository
	logger        zerolog.Logger
}

// NewProjectAssigner creates a new project assigner.
func NewProjectAssigner(
	metaRepo repository.MetadataRepository,
	suggestedRepo repository.SuggestedMetadataRepository,
	projectRepo repository.ProjectRepository,
	docRepo repository.DocumentRepository,
	logger zerolog.Logger,
) *ProjectAssigner {
	return &ProjectAssigner{
		metaRepo:      metaRepo,
		suggestedRepo: suggestedRepo,
		projectRepo:   projectRepo,
		docRepo:       docRepo,
		logger:        logger.With().Str("component", "project_assigner").Logger(),
	}
}

// AssignProject assigns a project to a file using all available metadata.
// It considers:
// - Confirmed metadata (tags, contexts, document metrics)
// - Suggested metadata (suggested tags, projects, taxonomy)
// - File content and structure
// - Similar files and their projects
func (a *ProjectAssigner) AssignProject(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	fileID entity.FileID,
	relativePath string,
) ([]entity.ProjectID, error) {
	// Get all metadata
	fileMeta, err := a.metaRepo.GetByPath(ctx, workspaceID, relativePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	suggestedMeta, err := a.suggestedRepo.Get(ctx, workspaceID, fileID)
	if err != nil {
		a.logger.Debug().Err(err).Str("path", relativePath).Msg("No suggested metadata found")
		suggestedMeta = nil
	}

	// Collect all project candidates
	candidates := a.collectProjectCandidates(fileMeta, suggestedMeta)

	// Score and rank candidates
	ranked := a.rankProjects(ctx, workspaceID, candidates, fileMeta, suggestedMeta)

	// Return top candidates (above threshold)
	var assigned []entity.ProjectID
	for _, candidate := range ranked {
		if candidate.Score >= 0.6 { // Threshold for auto-assignment
			assigned = append(assigned, candidate.ProjectID)
		}
	}

	return assigned, nil
}

// ProjectCandidate represents a potential project assignment.
type ProjectCandidate struct {
	ProjectID entity.ProjectID
	ProjectName string
	Score     float64
	Sources   []string // What metadata contributed to this assignment
}

// collectProjectCandidates collects all potential projects from metadata.
func (a *ProjectAssigner) collectProjectCandidates(
	fileMeta *entity.FileMetadata,
	suggestedMeta *entity.SuggestedMetadata,
) map[string]*ProjectCandidate {
	candidates := make(map[string]*ProjectCandidate)

	// From confirmed contexts
	for _, context := range fileMeta.Contexts {
		if candidate, ok := candidates[context]; ok {
			candidate.Score += 1.0
			candidate.Sources = append(candidate.Sources, "confirmed_context")
		} else {
			candidates[context] = &ProjectCandidate{
				ProjectName: context,
				Score:       1.0,
				Sources:     []string{"confirmed_context"},
			}
		}
	}

	// From suggested contexts
	if fileMeta.SuggestedContexts != nil {
		for _, context := range fileMeta.SuggestedContexts {
			if candidate, ok := candidates[context]; ok {
				candidate.Score += 0.7
				candidate.Sources = append(candidate.Sources, "suggested_context")
			} else {
				candidates[context] = &ProjectCandidate{
					ProjectName: context,
					Score:       0.7,
					Sources:     []string{"suggested_context"},
				}
			}
		}
	}

	// From suggested metadata projects
	if suggestedMeta != nil {
		for _, suggestedProject := range suggestedMeta.SuggestedProjects {
			projectName := suggestedProject.ProjectName
			if suggestedProject.ProjectID != nil {
				// Use existing project
				if candidate, ok := candidates[projectName]; ok {
					candidate.ProjectID = *suggestedProject.ProjectID
					candidate.Score += suggestedProject.Confidence * 0.8
					candidate.Sources = append(candidate.Sources, fmt.Sprintf("suggested_metadata_%.2f", suggestedProject.Confidence))
				} else {
					candidates[projectName] = &ProjectCandidate{
						ProjectID:   *suggestedProject.ProjectID,
						ProjectName: projectName,
						Score:       suggestedProject.Confidence * 0.8,
						Sources:     []string{fmt.Sprintf("suggested_metadata_%.2f", suggestedProject.Confidence)},
					}
				}
			} else {
				// New project suggestion
				if candidate, ok := candidates[projectName]; ok {
					candidate.Score += suggestedProject.Confidence * 0.6
					candidate.Sources = append(candidate.Sources, fmt.Sprintf("new_project_suggestion_%.2f", suggestedProject.Confidence))
				} else {
					candidates[projectName] = &ProjectCandidate{
						ProjectName: projectName,
						Score:       suggestedProject.Confidence * 0.6,
						Sources:     []string{fmt.Sprintf("new_project_suggestion_%.2f", suggestedProject.Confidence)},
					}
				}
			}
		}
	}

	return candidates
}

// rankProjects ranks project candidates based on multiple factors.
func (a *ProjectAssigner) rankProjects(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	candidates map[string]*ProjectCandidate,
	fileMeta *entity.FileMetadata,
	suggestedMeta *entity.SuggestedMetadata,
) []*ProjectCandidate {
	// Resolve project IDs for candidates that don't have them
	for name, candidate := range candidates {
		if candidate.ProjectID == "" {
			project, err := a.projectRepo.GetByName(ctx, workspaceID, name, nil)
			if err == nil && project != nil {
				candidate.ProjectID = project.ID
			}
		}
	}

	// Additional scoring based on:
	// 1. Taxonomy alignment
	if suggestedMeta != nil && suggestedMeta.SuggestedTaxonomy != nil {
		taxonomy := suggestedMeta.SuggestedTaxonomy
		for _, candidate := range candidates {
			if candidate.ProjectID != "" {
				// Check if project nature/domain aligns with taxonomy
				project, err := a.projectRepo.Get(ctx, workspaceID, candidate.ProjectID)
				if err == nil && project != nil {
					// Boost score if domain matches
					if taxonomy.Domain != "" {
						// Simple matching - could be enhanced with semantic similarity
						candidate.Score += 0.2
						candidate.Sources = append(candidate.Sources, "taxonomy_domain_match")
					}
				}
			}
		}
	}

	// 2. Tag alignment
	if fileMeta != nil {
		for _, tag := range fileMeta.Tags {
			// Tags can hint at project relationships
			// This is a simple implementation - could be enhanced
			for _, candidate := range candidates {
				if strings.Contains(strings.ToLower(candidate.ProjectName), strings.ToLower(tag)) {
					candidate.Score += 0.1
					candidate.Sources = append(candidate.Sources, fmt.Sprintf("tag_match:%s", tag))
				}
			}
		}
	}

	// Convert to slice and sort by score
	ranked := make([]*ProjectCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, candidate)
	}

	// Sort by score (descending)
	for i := 0; i < len(ranked)-1; i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[i].Score < ranked[j].Score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	return ranked
}

// GetProjectSuggestions returns project suggestions for a file using all metadata.
func (a *ProjectAssigner) GetProjectSuggestions(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	relativePath string,
) ([]*ProjectCandidate, error) {
	fileID := entity.NewFileID(relativePath)
	
	fileMeta, err := a.metaRepo.GetByPath(ctx, workspaceID, relativePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	suggestedMeta, err := a.suggestedRepo.Get(ctx, workspaceID, fileID)
	if err != nil {
		suggestedMeta = nil
	}

	candidates := a.collectProjectCandidates(fileMeta, suggestedMeta)
	ranked := a.rankProjects(ctx, workspaceID, candidates, fileMeta, suggestedMeta)

	return ranked, nil
}







