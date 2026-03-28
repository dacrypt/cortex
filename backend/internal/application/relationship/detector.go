package relationship

import (
	"regexp"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Detector detects relationships between documents from various sources.
type Detector struct {
	relRepo repository.RelationshipRepository
}

// NewDetector creates a new relationship detector.
func NewDetector(relRepo repository.RelationshipRepository) *Detector {
	return &Detector{
		relRepo: relRepo,
	}
}

// DetectFromFrontmatter extracts explicit relationships from document frontmatter.
func (d *Detector) DetectFromFrontmatter(doc *entity.Document, frontmatter map[string]interface{}) []*entity.DocumentRelationship {
	var relationships []*entity.DocumentRelationship

	// Check for explicit relationship fields
	relationshipFields := []string{
		"replaces",
		"depends_on",
		"belongs_to",
		"references",
		"related",
	}

	for _, field := range relationshipFields {
		if val, ok := frontmatter[field]; ok {
			relType := mapFieldToRelationshipType(field)
			if relType == "" {
				continue
			}

			// Handle string or array of strings
			switch v := val.(type) {
			case string:
				if v != "" {
					// This is a relative path - we'll need to resolve it to DocumentID
					relationships = append(relationships, &entity.DocumentRelationship{
						FromDocument: doc.ID,
						ToDocument:   entity.DocumentID(""), // Will be resolved later
						Type:         relType,
						Strength:     1.0,
						Metadata: map[string]interface{}{
							"source":      "frontmatter",
							"source_path": v,
						},
					})
				}
			case []interface{}:
				for _, item := range v {
					if path, ok := item.(string); ok && path != "" {
						relationships = append(relationships, &entity.DocumentRelationship{
							FromDocument: doc.ID,
							ToDocument:   entity.DocumentID(""), // Will be resolved later
							Type:         relType,
							Strength:     1.0,
							Metadata: map[string]interface{}{
								"source":      "frontmatter",
								"source_path": path,
							},
						})
					}
				}
			}
		}
	}

	return relationships
}

// DetectFromContent extracts relationships from document content (Markdown links).
func (d *Detector) DetectFromContent(doc *entity.Document, content string) []*entity.DocumentRelationship {
	var relationships []*entity.DocumentRelationship

	// Match Markdown links: [text](path.md) or [text](./path.md)
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+\.md)\)`)
	matches := linkPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		linkPath := match[2]

		// Normalize path (remove leading ./ or ../)
		linkPath = strings.TrimPrefix(linkPath, "./")
		linkPath = strings.TrimPrefix(linkPath, "../")

		// Skip external links
		if strings.HasPrefix(linkPath, "http://") || strings.HasPrefix(linkPath, "https://") {
			continue
		}

		// Create reference relationship
		relationships = append(relationships, &entity.DocumentRelationship{
			FromDocument: doc.ID,
			ToDocument:   entity.DocumentID(""), // Will be resolved later
			Type:         entity.RelationshipReferences,
			Strength:     0.8, // Links are strong but not as strong as explicit frontmatter
			Metadata: map[string]interface{}{
				"source":      "content_link",
				"source_path": linkPath,
				"link_text":   match[1],
			},
		})
	}

	return relationships
}

// DetectFromProjectMembership infers relationships based on shared project membership.
func (d *Detector) DetectFromProjectMembership(
	docID entity.DocumentID,
	projectIDs []entity.ProjectID,
	getProjectDocuments func(projectID entity.ProjectID) ([]entity.DocumentID, error),
) []*entity.DocumentRelationship {
	var relationships []*entity.DocumentID

	// Collect all documents in the same projects
	docSet := make(map[entity.DocumentID]bool)
	for _, projectID := range projectIDs {
		docs, err := getProjectDocuments(projectID)
		if err != nil {
			continue
		}
		for _, otherDocID := range docs {
			if otherDocID != docID {
				docSet[otherDocID] = true
			}
		}
	}

	// Create "references" relationships with lower strength
	for otherDocID := range docSet {
		relationships = append(relationships, &otherDocID)
	}

	// Convert to DocumentRelationship (this is a simplified version)
	// In practice, you'd want to create actual relationship objects
	var result []*entity.DocumentRelationship
	for _, otherDocID := range relationships {
		result = append(result, &entity.DocumentRelationship{
			FromDocument: docID,
			ToDocument:   *otherDocID,
			Type:         entity.RelationshipReferences,
			Strength:     0.3, // Weak relationship based on project membership
			Metadata: map[string]interface{}{
				"source": "project_membership",
			},
		})
	}

	return result
}

// ResolveRelationships resolves relative paths in relationships to DocumentIDs.
func (d *Detector) ResolveRelationships(
	relationships []*entity.DocumentRelationship,
	docRelativePath string,
	resolvePath func(relativePath string) (entity.DocumentID, error),
) ([]*entity.DocumentRelationship, error) {
	resolved := make([]*entity.DocumentRelationship, 0, len(relationships))

	for _, rel := range relationships {
		// If ToDocument is empty, try to resolve from metadata
		if rel.ToDocument == "" {
			sourcePath, ok := rel.Metadata["source_path"].(string)
			if !ok || sourcePath == "" {
				continue // Skip unresolved relationships
			}

			// Resolve relative path
			resolvedPath := resolveRelativePath(docRelativePath, sourcePath)
			docID, err := resolvePath(resolvedPath)
			if err != nil {
				// Document not found - skip this relationship
				continue
			}

			rel.ToDocument = docID
		}

		// Validate relationship
		if rel.FromDocument == rel.ToDocument {
			continue // Skip self-references
		}

		resolved = append(resolved, rel)
	}

	return resolved, nil
}

// mapFieldToRelationshipType maps frontmatter field names to relationship types.
func mapFieldToRelationshipType(field string) entity.RelationshipType {
	switch field {
	case "replaces":
		return entity.RelationshipReplaces
	case "depends_on":
		return entity.RelationshipDependsOn
	case "belongs_to":
		return entity.RelationshipBelongsTo
	case "references", "related":
		return entity.RelationshipReferences
	default:
		return ""
	}
}

// resolveRelativePath resolves a relative path from a source document path.
func resolveRelativePath(sourcePath, targetPath string) string {
	// Simple resolution - in production, use proper path resolution
	if strings.HasPrefix(targetPath, "/") {
		return strings.TrimPrefix(targetPath, "/")
	}

	// If target starts with ./, remove it
	targetPath = strings.TrimPrefix(targetPath, "./")

	// For now, return as-is (assumes same directory)
	// In production, use filepath.Join and filepath.Dir for proper resolution
	return targetPath
}

