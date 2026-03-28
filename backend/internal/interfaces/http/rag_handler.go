// Package http provides HTTP endpoints for health, metrics, and RAG queries.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/dacrypt/cortex/backend/internal/application/rag"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// RAGHandler handles /query requests.
type RAGHandler struct {
	service *rag.Service
}

// NewRAGHandler creates a new RAG handler.
func NewRAGHandler(service *rag.Service) *RAGHandler {
	return &RAGHandler{service: service}
}

type queryRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Query       string `json:"query"`
	TopK        int    `json:"top_k"`
}

type queryResponse struct {
	Answer  string       `json:"answer"`
	Sources []rag.Source `json:"sources"`
	Error   *string      `json:"error,omitempty"`
}

// HandleQuery processes a RAG query.
func (h *RAGHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(queryResponse{Error: stringPtr("invalid JSON")})
		return
	}

	workspaceID := entity.WorkspaceID(req.WorkspaceID)
	resp, err := h.service.Query(r.Context(), rag.QueryRequest{
		WorkspaceID:    workspaceID,
		Query:          req.Query,
		TopK:           req.TopK,
		GenerateAnswer: true,
	})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(queryResponse{Error: stringPtr(err.Error())})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(queryResponse{
		Answer:  resp.Answer,
		Sources: resp.Sources,
	})
}

func stringPtr(s string) *string {
	return &s
}
