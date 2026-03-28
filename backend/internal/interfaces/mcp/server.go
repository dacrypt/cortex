// Package mcp provides an MCP (Model Context Protocol) server interface for Cortex.
// This allows AI agents (Claude Code, Copilot, Cursor) to query the Cortex
// knowledge graph using structured tool calls instead of reading entire files.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/query"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
)

// Server wraps an MCP server that exposes Cortex knowledge graph to AI agents.
type Server struct {
	mcpServer        *server.MCPServer
	knowledgeHandler *handlers.KnowledgeHandler
	fileHandler      *handlers.FileHandler
	metadataHandler  *handlers.MetadataHandler
	ragHandler       *handlers.RAGHandler
	defaultWorkspace entity.WorkspaceID
	logger           zerolog.Logger
}

// Config holds configuration for the MCP server.
type Config struct {
	KnowledgeHandler *handlers.KnowledgeHandler
	FileHandler      *handlers.FileHandler
	MetadataHandler  *handlers.MetadataHandler
	RAGHandler       *handlers.RAGHandler
	DefaultWorkspace entity.WorkspaceID
	Logger           zerolog.Logger
}

// NewServer creates a new MCP server with Cortex tools.
func NewServer(cfg Config) *Server {
	s := &Server{
		knowledgeHandler: cfg.KnowledgeHandler,
		fileHandler:      cfg.FileHandler,
		metadataHandler:  cfg.MetadataHandler,
		ragHandler:       cfg.RAGHandler,
		defaultWorkspace: cfg.DefaultWorkspace,
		logger:           cfg.Logger,
	}

	s.mcpServer = server.NewMCPServer(
		"cortex",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	s.registerTools()
	return s
}

// ServeStdio starts the MCP server on stdin/stdout.
func (s *Server) ServeStdio() error {
	s.logger.Info().Msg("Starting Cortex MCP server on stdio")
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	// cortex_find — search documents, projects, files
	s.mcpServer.AddTool(s.findTool(), s.handleFind)

	// cortex_show — show content, metadata, outline of a document
	s.mcpServer.AddTool(s.showTool(), s.handleShow)

	// cortex_relations — navigate relationships between documents/projects
	s.mcpServer.AddTool(s.relationsTool(), s.handleRelations)
}

// --- Tool Definitions ---

func (s *Server) findTool() mcp.Tool {
	return mcp.NewTool("cortex_find",
		mcp.WithDescription(`Search the Cortex knowledge graph for documents, projects, or files.

Examples:
  Find all active documents: {"kind": "document", "state": "active"}
  Find projects by nature: {"kind": "project", "nature": "development.software"}
  Find docs with tag: {"kind": "document", "tag": "review"}
  Find docs in project: {"kind": "document", "project": "Nexus Platform"}
  Semantic search: {"kind": "document", "query": "network VPN configuration"}
  Find files by extension: {"kind": "file", "extension": ".pdf"}
  Find recent docs: {"kind": "document", "order_by": "updated", "limit": 10}`),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("What to search for: 'document', 'project', or 'file'"),
			mcp.Enum("document", "project", "file"),
		),
		mcp.WithString("state",
			mcp.Description("Filter documents by state: draft, active, replaced, archived"),
		),
		mcp.WithString("nature",
			mcp.Description("Filter projects by nature (e.g., 'development.software', 'purchase.vehicle')"),
		),
		mcp.WithString("tag",
			mcp.Description("Filter by tag name"),
		),
		mcp.WithString("project",
			mcp.Description("Filter documents by project name"),
		),
		mcp.WithString("query",
			mcp.Description("Semantic search query (uses RAG/embeddings)"),
		),
		mcp.WithString("extension",
			mcp.Description("Filter files by extension (e.g., '.pdf', '.md')"),
		),
		mcp.WithString("order_by",
			mcp.Description("Order results by: 'name', 'updated', 'created', 'size'"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default: 20, max: 100)"),
		),
	)
}

func (s *Server) showTool() mcp.Tool {
	return mcp.NewTool("cortex_show",
		mcp.WithDescription(`Show details about a specific document, project, or file.

Examples:
  Document metadata: {"target": "meeting-notes.md", "view": "signature"}
  Document outline: {"target": "architecture.pdf", "view": "outline"}
  Document content: {"target": "readme.md", "view": "body"}
  Project members: {"target": "Nexus Platform", "view": "members"}
  File metadata: {"target": "invoice.xlsx", "view": "metadata"}
  Project overview: {"target": "Tesla Order", "view": "signature"}`),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Name or path of the document, project, or file to inspect"),
		),
		mcp.WithString("view",
			mcp.Required(),
			mcp.Description("What to show: 'signature' (metadata summary), 'outline' (structure/headings), 'body' (full content), 'members' (project documents), 'metadata' (all metadata)"),
			mcp.Enum("signature", "outline", "body", "members", "metadata"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Maximum depth for outline/members (default: 2)"),
		),
	)
}

func (s *Server) relationsTool() mcp.Tool {
	return mcp.NewTool("cortex_relations",
		mcp.WithDescription(`Navigate relationships in the knowledge graph.

Examples:
  Document dependencies: {"target": "api-spec.md", "direction": "outgoing", "type": "depends_on"}
  What references this doc: {"target": "template.md", "direction": "incoming", "type": "references"}
  Find path between docs: {"target": "doc-a.md", "path_to": "doc-b.md"}
  Project relationships: {"target": "Nexus Platform", "kind": "project"}
  All relationships: {"target": "architecture.md"}`),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Name or path of the document or project"),
		),
		mcp.WithString("kind",
			mcp.Description("Target kind: 'document' (default) or 'project'"),
			mcp.Enum("document", "project"),
		),
		mcp.WithString("direction",
			mcp.Description("Relationship direction: 'outgoing' (what target depends on), 'incoming' (what depends on target), 'both' (default)"),
			mcp.Enum("outgoing", "incoming", "both"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by relationship type: 'replaces', 'depends_on', 'belongs_to', 'references', 'parent_of'"),
		),
		mcp.WithString("path_to",
			mcp.Description("Find shortest path from target to this document"),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Maximum traversal depth (default: 3)"),
		),
	)
}

// --- Tool Handlers ---

func (s *Server) handleFind(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	kind := stringParam(request, "kind")
	wsID := s.defaultWorkspace

	switch kind {
	case "project":
		return s.findProjects(ctx, wsID, request)
	case "file":
		return s.findFiles(ctx, wsID, request)
	default:
		return s.findDocuments(ctx, wsID, request)
	}
}

func (s *Server) findProjects(ctx context.Context, wsID entity.WorkspaceID, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projects, err := s.knowledgeHandler.ListProjects(ctx, wsID, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
	}

	natureFilter := stringParam(req, "nature")
	limit := intParam(req, "limit", 20)

	var results []projectSummary
	for _, p := range projects {
		if natureFilter != "" && string(p.Nature) != natureFilter {
			continue
		}
		results = append(results, projectSummary{
			ID:          string(p.ID),
			Name:        p.Name,
			Description: p.Description,
			Nature:      string(p.Nature),
			HasParent:   p.ParentID != nil,
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		})
		if len(results) >= limit {
			break
		}
	}

	return jsonResult(results)
}

func (s *Server) findDocuments(ctx context.Context, wsID entity.WorkspaceID, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Semantic search path
	queryStr := stringParam(req, "query")
	if queryStr != "" && s.ragHandler != nil {
		return s.semanticSearch(ctx, wsID, queryStr, intParam(req, "limit", 10))
	}

	// Structured query path
	qb := query.Query(wsID)
	limit := intParam(req, "limit", 20)
	qb.Limit(limit)

	// Apply filters
	if state := stringParam(req, "state"); state != "" {
		qb.Filter(&query.StateFilter{States: []entity.DocumentState{entity.DocumentState(state)}})
	}

	if tag := stringParam(req, "tag"); tag != "" {
		qb.Filter(&query.TagFilter{Tag: tag})
	}

	if projectName := stringParam(req, "project"); projectName != "" {
		// Resolve project name to ID
		projects, err := s.knowledgeHandler.ListProjects(ctx, wsID, nil)
		if err == nil {
			for _, p := range projects {
				if strings.EqualFold(p.Name, projectName) {
					qb.Filter(&query.ProjectFilter{ProjectID: p.ID, IncludeSubprojects: true})
					break
				}
			}
		}
	}

	if orderBy := stringParam(req, "order_by"); orderBy != "" {
		qb.OrderBy(orderBy, true)
	}

	result, err := s.knowledgeHandler.QueryDocuments(ctx, wsID, qb)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}

	// Resolve document details
	var docs []documentSummary
	for _, docID := range result.DocumentIDs {
		doc, err := s.knowledgeHandler.GetDocument(ctx, wsID, docID)
		if err != nil {
			continue
		}
		ds := documentSummary{
			ID:    string(doc.ID),
			Path:  doc.RelativePath,
			Title: doc.Title,
			State: string(doc.State),
		}
		if !doc.UpdatedAt.IsZero() {
			ds.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
		}
		docs = append(docs, ds)
	}

	return jsonResult(map[string]interface{}{
		"documents": docs,
		"total":     result.Total,
		"has_more":  result.HasMore,
	})
}

func (s *Server) semanticSearch(ctx context.Context, wsID entity.WorkspaceID, queryStr string, limit int) (*mcp.CallToolResult, error) {
	resp, err := s.ragHandler.SemanticSearch(ctx, handlers.SemanticSearchRequest{
		WorkspaceID: string(wsID),
		Query:       queryStr,
		TopK:        limit,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("semantic search failed: %v", err)), nil
	}

	var results []map[string]interface{}
	for _, r := range resp.Results {
		results = append(results, map[string]interface{}{
			"path":       r.RelativePath,
			"heading":    r.HeadingPath,
			"snippet":    truncate(r.Snippet, 200),
			"similarity": r.Score,
		})
	}

	return jsonResult(results)
}

func (s *Server) findFiles(ctx context.Context, wsID entity.WorkspaceID, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ext := stringParam(req, "extension")
	limit := intParam(req, "limit", 20)

	var opts handlers.ListFilesOptions
	opts.Limit = limit

	files, _, err := s.fileHandler.ListFiles(ctx, string(wsID), opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list files failed: %v", err)), nil
	}

	var results []fileSummary
	for _, f := range files {
		if ext != "" && f.Extension != ext {
			continue
		}
		fs := fileSummary{
			Path:      f.RelativePath,
			Extension: f.Extension,
			Size:      f.FileSize,
			Modified:  f.LastModified.Format(time.RFC3339),
		}
		if f.Enhanced != nil && f.Enhanced.Language != nil {
			fs.Language = *f.Enhanced.Language
		}
		results = append(results, fs)
		if len(results) >= limit {
			break
		}
	}

	return jsonResult(results)
}

func (s *Server) handleShow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := stringParam(request, "target")
	view := stringParam(request, "view")
	wsID := s.defaultWorkspace

	switch view {
	case "signature":
		return s.showSignature(ctx, wsID, target)
	case "outline":
		return s.showOutline(ctx, wsID, target)
	case "body":
		return s.showBody(ctx, wsID, target)
	case "members":
		return s.showMembers(ctx, wsID, target)
	case "metadata":
		return s.showMetadata(ctx, wsID, target)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown view: %s", view)), nil
	}
}

func (s *Server) showSignature(ctx context.Context, wsID entity.WorkspaceID, target string) (*mcp.CallToolResult, error) {
	// Try as document first
	doc, err := s.resolveDocument(ctx, wsID, target)
	if err == nil && doc != nil {
		sig := map[string]interface{}{
			"type":       "document",
			"id":         string(doc.ID),
			"path":       doc.RelativePath,
			"title":      doc.Title,
			"state":      string(doc.State),
			"created_at": doc.CreatedAt.Format(time.RFC3339),
			"updated_at": doc.UpdatedAt.Format(time.RFC3339),
		}
		if doc.Frontmatter != nil {
			sig["frontmatter"] = doc.Frontmatter
		}
		// Get tags
		meta, err := s.metadataHandler.GetMetadata(ctx, string(wsID), doc.RelativePath)
		if err == nil && meta != nil {
			if len(meta.Tags) > 0 {
				sig["tags"] = meta.Tags
			}
			if len(meta.Contexts) > 0 {
				sig["contexts"] = meta.Contexts
			}
			if meta.AISummary != nil {
				sig["ai_summary"] = meta.AISummary.Summary
			}
		}
		// Get projects
		projectIDs, err := s.knowledgeHandler.GetProjectsForDocument(ctx, wsID, doc.ID)
		if err == nil && len(projectIDs) > 0 {
			sig["projects"] = projectIDs
		}
		return jsonResult(sig)
	}

	// Try as project
	proj, err := s.resolveProject(ctx, wsID, target)
	if err == nil && proj != nil {
		sig := map[string]interface{}{
			"type":        "project",
			"id":          string(proj.ID),
			"name":        proj.Name,
			"description": proj.Description,
			"nature":      string(proj.Nature),
			"created_at":  proj.CreatedAt.Format(time.RFC3339),
			"updated_at":  proj.UpdatedAt.Format(time.RFC3339),
		}
		if proj.ParentID != nil {
			sig["parent_id"] = string(*proj.ParentID)
		}
		if proj.Attributes != nil {
			sig["attributes"] = proj.Attributes
		}
		return jsonResult(sig)
	}

	return mcp.NewToolResultError(fmt.Sprintf("not found: %s", target)), nil
}

func (s *Server) showOutline(ctx context.Context, wsID entity.WorkspaceID, target string) (*mcp.CallToolResult, error) {
	// Try as document — show heading structure
	doc, err := s.resolveDocument(ctx, wsID, target)
	if err != nil || doc == nil {
		// Try as project — show sub-projects
		proj, err := s.resolveProject(ctx, wsID, target)
		if err != nil || proj == nil {
			return mcp.NewToolResultError(fmt.Sprintf("not found: %s", target)), nil
		}
		children, err := s.knowledgeHandler.GetProjectChildren(ctx, wsID, proj.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get children: %v", err)), nil
		}
		var items []map[string]string
		for _, c := range children {
			items = append(items, map[string]string{
				"id":     string(c.ID),
				"name":   c.Name,
				"nature": string(c.Nature),
			})
		}
		return jsonResult(map[string]interface{}{
			"project":     proj.Name,
			"subprojects": items,
		})
	}

	// Get file entry for content structure
	file, err := s.fileHandler.GetFile(ctx, string(wsID), doc.RelativePath)
	if err != nil || file == nil || file.Enhanced == nil || file.Enhanced.ContentStructure == nil {
		return jsonResult(map[string]interface{}{
			"document": doc.Title,
			"path":     doc.RelativePath,
			"headings": []string{},
			"note":     "No structural data available (file may not have been indexed with document stage)",
		})
	}

	cs := file.Enhanced.ContentStructure
	var headings []map[string]interface{}
	for _, h := range cs.Headings {
		headings = append(headings, map[string]interface{}{
			"level": h.Level,
			"text":  h.Text,
			"line":  h.Line,
		})
	}

	return jsonResult(map[string]interface{}{
		"document":      doc.Title,
		"path":          doc.RelativePath,
		"headings":      headings,
		"section_count": cs.SectionCount,
		"has_toc":       cs.HasTOC,
	})
}

func (s *Server) showBody(ctx context.Context, wsID entity.WorkspaceID, target string) (*mcp.CallToolResult, error) {
	doc, err := s.resolveDocument(ctx, wsID, target)
	if err != nil || doc == nil {
		return mcp.NewToolResultError(fmt.Sprintf("document not found: %s", target)), nil
	}

	// Try mirror content first (extracted text for non-text files)
	meta, err := s.metadataHandler.GetMetadata(ctx, string(wsID), doc.RelativePath)
	if err == nil && meta != nil && meta.Mirror != nil {
		return mcp.NewToolResultText(fmt.Sprintf("# %s\n\nSource: %s\nExtraction: %s (confidence: %.0f%%)\n\n---\n\n[Content available at mirror path: %s]",
			doc.Title, doc.RelativePath, meta.Mirror.ExtractionMethod,
			meta.Mirror.ExtractionConfidence*100, meta.Mirror.Path)), nil
	}

	// Return basic info
	return jsonResult(map[string]interface{}{
		"document": doc.Title,
		"path":     doc.RelativePath,
		"state":    string(doc.State),
		"note":     "Use the file system to read the full content at the path above",
	})
}

func (s *Server) showMembers(ctx context.Context, wsID entity.WorkspaceID, target string) (*mcp.CallToolResult, error) {
	proj, err := s.resolveProject(ctx, wsID, target)
	if err != nil || proj == nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", target)), nil
	}

	// Get project documents via assignments
	assignments, err := s.knowledgeHandler.ListProjectAssignmentsByProject(ctx, wsID, proj.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list assignments: %v", err)), nil
	}

	var members []map[string]interface{}
	for _, a := range assignments {
		m := map[string]interface{}{
			"file_id": string(a.FileID),
			"status":  string(a.Status),
		}
		// Try to get file details
		if a.ProjectName != "" {
			m["project_name"] = a.ProjectName
		}
		members = append(members, m)
	}

	// Also get subprojects
	children, err := s.knowledgeHandler.GetProjectChildren(ctx, wsID, proj.ID)
	if err == nil {
		var subprojects []string
		for _, c := range children {
			subprojects = append(subprojects, c.Name)
		}
		return jsonResult(map[string]interface{}{
			"project":     proj.Name,
			"members":     members,
			"member_count": len(members),
			"subprojects": subprojects,
		})
	}

	return jsonResult(map[string]interface{}{
		"project":      proj.Name,
		"members":      members,
		"member_count": len(members),
	})
}

func (s *Server) showMetadata(ctx context.Context, wsID entity.WorkspaceID, target string) (*mcp.CallToolResult, error) {
	meta, err := s.metadataHandler.GetMetadata(ctx, string(wsID), target)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("metadata not found for: %s", target)), nil
	}

	result := map[string]interface{}{
		"path":       meta.RelativePath,
		"type":       meta.Type,
		"tags":       meta.Tags,
		"contexts":   meta.Contexts,
		"created_at": meta.CreatedAt.Format(time.RFC3339),
		"updated_at": meta.UpdatedAt.Format(time.RFC3339),
	}
	if meta.Notes != nil {
		result["notes"] = *meta.Notes
	}
	if meta.DetectedLanguage != nil {
		result["language"] = *meta.DetectedLanguage
	}
	if meta.AISummary != nil {
		result["ai_summary"] = meta.AISummary.Summary
		result["ai_key_terms"] = meta.AISummary.KeyTerms
	}
	if meta.AICategory != nil {
		result["ai_category"] = meta.AICategory.Category
	}
	if len(meta.SuggestedContexts) > 0 {
		result["suggested_contexts"] = meta.SuggestedContexts
	}

	return jsonResult(result)
}

func (s *Server) handleRelations(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := stringParam(request, "target")
	kind := stringParam(request, "kind")
	if kind == "" {
		kind = "document"
	}
	direction := stringParam(request, "direction")
	if direction == "" {
		direction = "both"
	}
	relTypeStr := stringParam(request, "type")
	pathTo := stringParam(request, "path_to")
	maxDepth := intParam(request, "max_depth", 3)
	wsID := s.defaultWorkspace

	if kind == "project" {
		return s.projectRelations(ctx, wsID, target, relTypeStr)
	}

	// Document relations
	doc, err := s.resolveDocument(ctx, wsID, target)
	if err != nil || doc == nil {
		return mcp.NewToolResultError(fmt.Sprintf("document not found: %s", target)), nil
	}

	// Path finding
	if pathTo != "" {
		targetDoc, err := s.resolveDocument(ctx, wsID, pathTo)
		if err != nil || targetDoc == nil {
			return mcp.NewToolResultError(fmt.Sprintf("target document not found: %s", pathTo)), nil
		}
		path, err := s.knowledgeHandler.FindPath(ctx, wsID, doc.ID, targetDoc.ID, maxDepth)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("path finding failed: %v", err)), nil
		}
		return jsonResult(map[string]interface{}{
			"from":       target,
			"to":         pathTo,
			"path":       path,
			"path_length": len(path),
		})
	}

	// Get relationships
	var relType *entity.RelationshipType
	if relTypeStr != "" {
		rt := entity.RelationshipType(relTypeStr)
		relType = &rt
	}

	rels, err := s.knowledgeHandler.GetDocumentRelationships(ctx, wsID, doc.ID, relType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get relationships: %v", err)), nil
	}

	var outgoing, incoming []relationshipSummary
	for _, r := range rels {
		rs := relationshipSummary{
			Type:     string(r.Type),
			Strength: r.Strength,
		}
		if r.FromDocument == doc.ID {
			rs.Target = string(r.ToDocument)
			outgoing = append(outgoing, rs)
		} else {
			rs.Target = string(r.FromDocument)
			incoming = append(incoming, rs)
		}
	}

	result := map[string]interface{}{
		"document": target,
	}
	if direction == "outgoing" || direction == "both" {
		result["outgoing"] = outgoing
	}
	if direction == "incoming" || direction == "both" {
		result["incoming"] = incoming
	}

	return jsonResult(result)
}

func (s *Server) projectRelations(ctx context.Context, wsID entity.WorkspaceID, target, relTypeStr string) (*mcp.CallToolResult, error) {
	proj, err := s.resolveProject(ctx, wsID, target)
	if err != nil || proj == nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", target)), nil
	}

	var relType *entity.RelationshipType
	if relTypeStr != "" {
		rt := entity.RelationshipType(relTypeStr)
		relType = &rt
	}

	rels, err := s.knowledgeHandler.GetProjectRelationships(ctx, wsID, proj.ID, relType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get project relationships: %v", err)), nil
	}

	var relationships []map[string]interface{}
	for _, r := range rels {
		relationships = append(relationships, map[string]interface{}{
			"from":        string(r.FromProjectID),
			"to":          string(r.ToProjectID),
			"type":        string(r.Type),
			"description": r.Description,
		})
	}

	return jsonResult(map[string]interface{}{
		"project":       proj.Name,
		"relationships": relationships,
	})
}

// --- Resolution helpers ---

func (s *Server) resolveDocument(ctx context.Context, wsID entity.WorkspaceID, target string) (*entity.Document, error) {
	// Try as document ID
	doc, err := s.knowledgeHandler.GetDocument(ctx, wsID, entity.DocumentID(target))
	if err == nil && doc != nil {
		return doc, nil
	}

	// Try constructing ID from path (Cortex convention: SHA-256 of "doc:" + path)
	// But for simplicity, search by path
	qb := query.Query(wsID).Limit(1)
	result, err := s.knowledgeHandler.QueryDocuments(ctx, wsID, qb)
	if err != nil {
		return nil, err
	}

	// Linear scan to match by path (could be optimized with a path index)
	for _, docID := range result.DocumentIDs {
		doc, err := s.knowledgeHandler.GetDocument(ctx, wsID, docID)
		if err != nil {
			continue
		}
		if strings.HasSuffix(doc.RelativePath, target) || strings.EqualFold(doc.Title, target) {
			return doc, nil
		}
	}

	// Broader search
	qb = query.Query(wsID).Limit(100)
	result, err = s.knowledgeHandler.QueryDocuments(ctx, wsID, qb)
	if err != nil {
		return nil, err
	}

	for _, docID := range result.DocumentIDs {
		doc, err := s.knowledgeHandler.GetDocument(ctx, wsID, docID)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(doc.RelativePath), strings.ToLower(target)) ||
			strings.Contains(strings.ToLower(doc.Title), strings.ToLower(target)) {
			return doc, nil
		}
	}

	return nil, fmt.Errorf("document not found: %s", target)
}

func (s *Server) resolveProject(ctx context.Context, wsID entity.WorkspaceID, target string) (*entity.Project, error) {
	// Try as project ID
	proj, err := s.knowledgeHandler.GetProject(ctx, wsID, entity.ProjectID(target))
	if err == nil && proj != nil {
		return proj, nil
	}

	// Search by name
	projects, err := s.knowledgeHandler.ListProjects(ctx, wsID, nil)
	if err != nil {
		return nil, err
	}

	for _, p := range projects {
		if strings.EqualFold(p.Name, target) || strings.Contains(strings.ToLower(p.Name), strings.ToLower(target)) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("project not found: %s", target)
}

// --- Helper types ---

type projectSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Nature      string `json:"nature"`
	HasParent   bool   `json:"has_parent,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type documentSummary struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type fileSummary struct {
	Path      string `json:"path"`
	Extension string `json:"extension"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	Language  string `json:"language,omitempty"`
}

type relationshipSummary struct {
	Target   string  `json:"target"`
	Type     string  `json:"type"`
	Strength float64 `json:"strength"`
}

// --- Utility functions ---

func stringParam(req mcp.CallToolRequest, name string) string {
	args := req.GetArguments()
	if v, ok := args[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intParam(req mcp.CallToolRequest, name string, defaultVal int) int {
	args := req.GetArguments()
	if v, ok := args[name]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}

func jsonResult(data interface{}) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
