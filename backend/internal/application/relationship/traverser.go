package relationship

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// Traverser provides graph traversal operations for document relationships.
type Traverser struct {
	relRepo repository.RelationshipRepository
}

// NewTraverser creates a new relationship traverser.
func NewTraverser(relRepo repository.RelationshipRepository) *Traverser {
	return &Traverser{
		relRepo: relRepo,
	}
}

// Traverse performs graph traversal starting from a document.
func (t *Traverser) Traverse(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	startDocID entity.DocumentID,
	relType entity.RelationshipType,
	maxDepth int,
) ([]entity.DocumentID, error) {
	return t.relRepo.Traverse(ctx, workspaceID, startDocID, relType, maxDepth)
}

// GetReplacementChain returns the chain of documents that replace each other.
func (t *Traverser) GetReplacementChain(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) ([]entity.DocumentID, error) {
	return t.relRepo.GetReplacementChain(ctx, workspaceID, docID)
}

// GetDependencyTree returns all documents that a document depends on (transitive).
func (t *Traverser) GetDependencyTree(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	maxDepth int,
) ([]entity.DocumentID, error) {
	return t.Traverse(ctx, workspaceID, docID, entity.RelationshipDependsOn, maxDepth)
}

// GetDependents returns all documents that depend on the given document (reverse).
func (t *Traverser) GetDependents(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
	maxDepth int,
) ([]entity.DocumentID, error) {
	// Traverse incoming "depends_on" relationships
	visited := make(map[entity.DocumentID]bool)
	var result []entity.DocumentID
	queue := []struct {
		docID entity.DocumentID
		depth int
	}{{docID, 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.docID] || current.depth >= maxDepth {
			continue
		}
		visited[current.docID] = true

		// Get incoming "depends_on" relationships
		incoming, err := t.relRepo.GetIncoming(ctx, workspaceID, current.docID, entity.RelationshipDependsOn)
		if err != nil {
			return nil, err
		}

		for _, rel := range incoming {
			if !visited[rel.FromDocument] {
				result = append(result, rel.FromDocument)
				queue = append(queue, struct {
					docID entity.DocumentID
					depth int
				}{rel.FromDocument, current.depth + 1})
			}
		}
	}

	return result, nil
}

// GetRelatedDocuments returns all documents related to a document via any relationship type.
func (t *Traverser) GetRelatedDocuments(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) (map[entity.RelationshipType][]entity.DocumentID, error) {
	result := make(map[entity.RelationshipType][]entity.DocumentID)

	// Get all outgoing relationships
	allOutgoing, err := t.relRepo.GetAllOutgoing(ctx, workspaceID, docID)
	if err != nil {
		return nil, err
	}

	for _, rel := range allOutgoing {
		result[rel.Type] = append(result[rel.Type], rel.ToDocument)
	}

	return result, nil
}

// FindPath finds a path between two documents (if one exists).
func (t *Traverser) FindPath(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	fromDocID, toDocID entity.DocumentID,
	maxDepth int,
) ([]entity.DocumentID, error) {
	if fromDocID == toDocID {
		return []entity.DocumentID{fromDocID}, nil
	}

	// BFS to find path
	visited := make(map[entity.DocumentID]bool)
	parent := make(map[entity.DocumentID]entity.DocumentID)
	queue := []entity.DocumentID{fromDocID}
	visited[fromDocID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Get all outgoing relationships
		allOutgoing, err := t.relRepo.GetAllOutgoing(ctx, workspaceID, current)
		if err != nil {
			return nil, err
		}

		for _, rel := range allOutgoing {
			next := rel.ToDocument
			if visited[next] {
				continue
			}
			visited[next] = true
			parent[next] = current

			if next == toDocID {
				// Reconstruct path
				path := []entity.DocumentID{toDocID}
				for next != fromDocID {
					next = parent[next]
					path = append([]entity.DocumentID{next}, path...)
				}
				return path, nil
			}

			// Check depth
			depth := 0
			temp := next
			for temp != fromDocID {
				depth++
				if depth > maxDepth {
					break
				}
				var ok bool
				temp, ok = parent[temp]
				if !ok {
					break
				}
			}

			if depth <= maxDepth {
				queue = append(queue, next)
			}
		}
	}

	return nil, fmt.Errorf("no path found between documents")
}

