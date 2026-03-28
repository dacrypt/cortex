package clustering

import (
	"context"
	"fmt"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// DocumentInfoProviderImpl implements DocumentInfoProvider using DocumentRepository.
type DocumentInfoProviderImpl struct {
	docRepo repository.DocumentRepository
	metaRepo repository.MetadataRepository
}

// NewDocumentInfoProvider creates a new DocumentInfoProvider implementation.
func NewDocumentInfoProvider(
	docRepo repository.DocumentRepository,
	metaRepo repository.MetadataRepository,
) DocumentInfoProvider {
	return &DocumentInfoProviderImpl{
		docRepo:  docRepo,
		metaRepo: metaRepo,
	}
}

// GetDocumentInfo retrieves document information for LLM context.
func (p *DocumentInfoProviderImpl) GetDocumentInfo(
	ctx context.Context,
	workspaceID entity.WorkspaceID,
	docID entity.DocumentID,
) (*DocumentInfo, error) {
	// Get document
	doc, err := p.docRepo.GetDocument(ctx, workspaceID, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("document not found: %s", docID)
	}

	info := &DocumentInfo{
		ID:           doc.ID,
		RelativePath: doc.RelativePath,
		Title:        doc.Title,
		Summary:      "",
		Keywords:     []string{},
	}

	// Try to get AI summary from file metadata
	if p.metaRepo != nil {
		metadata, err := p.metaRepo.GetByPath(ctx, workspaceID, doc.RelativePath)
		if err == nil && metadata != nil {
			if metadata.AISummary != nil && metadata.AISummary.Summary != "" {
				info.Summary = metadata.AISummary.Summary
			}
			if metadata.AISummary != nil && len(metadata.AISummary.KeyTerms) > 0 {
				info.Keywords = metadata.AISummary.KeyTerms
			}
		}
	}

	// If no AI summary, extract summary from document chunks
	if info.Summary == "" {
		chunks, err := p.docRepo.GetChunksByDocument(ctx, workspaceID, docID)
		if err == nil && len(chunks) > 0 {
			// Use first chunk's text as summary (truncated to ~200 chars)
			summaryText := chunks[0].Text
			if len(summaryText) > 200 {
				summaryText = summaryText[:200] + "..."
			}
			info.Summary = summaryText
		}
	}

	// If still no summary, use title or path
	if info.Summary == "" {
		if doc.Title != "" {
			info.Summary = doc.Title
		} else {
			info.Summary = doc.RelativePath
		}
	}

	return info, nil
}

