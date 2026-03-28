// Package taxonomy provides dynamic taxonomy management using Chain-of-Layer induction.
package taxonomy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

// ServiceConfig configures the taxonomy service.
type ServiceConfig struct {
	MaxLevels        int     // Maximum depth of taxonomy (default: 4)
	MaxNodesPerLevel int     // Maximum nodes per level (default: 20)
	MinConfidence    float64 // Minimum confidence for induction (default: 0.6)
	AutoMerge        bool    // Auto-merge similar nodes (default: true)
	MergeSimilarity  float64 // Similarity threshold for merging (default: 0.85)
}

// DefaultServiceConfig returns the default configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MaxLevels:        4,
		MaxNodesPerLevel: 20,
		MinConfidence:    0.6,
		AutoMerge:        true,
		MergeSimilarity:  0.85,
	}
}

// Service provides taxonomy management operations.
type Service struct {
	config       ServiceConfig
	taxonomyRepo repository.TaxonomyRepository
	fileRepo     repository.FileRepository
	docRepo      repository.DocumentRepository
	llmRouter    LLMRouter
	logger       zerolog.Logger
}

// LLMRouter provides LLM completion capabilities.
type LLMRouter interface {
	Complete(ctx context.Context, prompt string, maxTokens int) (string, error)
}

// NewService creates a new taxonomy service.
func NewService(
	config ServiceConfig,
	taxonomyRepo repository.TaxonomyRepository,
	fileRepo repository.FileRepository,
	docRepo repository.DocumentRepository,
	llmRouter LLMRouter,
	logger zerolog.Logger,
) *Service {
	return &Service{
		config:       config,
		taxonomyRepo: taxonomyRepo,
		fileRepo:     fileRepo,
		docRepo:      docRepo,
		llmRouter:    llmRouter,
		logger:       logger.With().Str("component", "taxonomy-service").Logger(),
	}
}

// GetNode retrieves a taxonomy node by ID.
func (s *Service) GetNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) (*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.GetNode(ctx, workspaceID, nodeID)
}

// GetNodeByPath retrieves a taxonomy node by path.
func (s *Service) GetNodeByPath(ctx context.Context, workspaceID entity.WorkspaceID, path string) (*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.GetNodeByPath(ctx, workspaceID, path)
}

// GetRootNodes retrieves all root taxonomy nodes.
func (s *Service) GetRootNodes(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.GetRootNodes(ctx, workspaceID)
}

// GetChildren retrieves child nodes of a parent.
func (s *Service) GetChildren(ctx context.Context, workspaceID entity.WorkspaceID, parentID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.GetChildren(ctx, workspaceID, parentID)
}

// GetAncestors retrieves ancestors of a node.
func (s *Service) GetAncestors(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) ([]*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.GetAncestors(ctx, workspaceID, nodeID)
}

// ListAll retrieves all taxonomy nodes.
func (s *Service) ListAll(ctx context.Context, workspaceID entity.WorkspaceID) ([]*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.ListAll(ctx, workspaceID)
}

// CreateNode creates a new taxonomy node.
func (s *Service) CreateNode(ctx context.Context, workspaceID entity.WorkspaceID, name string, parentID *entity.TaxonomyNodeID, source entity.TaxonomyNodeSource) (*entity.TaxonomyNode, error) {
	node := entity.NewTaxonomyNode(workspaceID, name, parentID)
	node.Source = source

	// Build path
	if parentID == nil {
		node.Path = name
		node.Level = 0
	} else {
		parent, err := s.taxonomyRepo.GetNode(ctx, workspaceID, *parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent node: %w", err)
		}
		if parent == nil {
			return nil, fmt.Errorf("parent node not found: %s", parentID)
		}
		node.Path = parent.Path + "/" + name
		node.Level = parent.Level + 1
	}

	if err := s.taxonomyRepo.CreateNode(ctx, workspaceID, node); err != nil {
		return nil, err
	}

	return node, nil
}

// UpdateNode updates an existing taxonomy node.
func (s *Service) UpdateNode(ctx context.Context, workspaceID entity.WorkspaceID, node *entity.TaxonomyNode) error {
	return s.taxonomyRepo.UpdateNode(ctx, workspaceID, node)
}

// DeleteNode deletes a taxonomy node.
func (s *Service) DeleteNode(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID) error {
	return s.taxonomyRepo.DeleteNode(ctx, workspaceID, nodeID)
}

// AddFileToNode maps a file to a taxonomy node.
func (s *Service) AddFileToNode(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, nodeID entity.TaxonomyNodeID, source entity.TaxonomyNodeSource) error {
	mapping := entity.NewFileTaxonomyMapping(workspaceID, fileID, nodeID, source)
	return s.taxonomyRepo.AddFileMapping(ctx, mapping)
}

// RemoveFileFromNode removes a file from a taxonomy node.
func (s *Service) RemoveFileFromNode(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID, nodeID entity.TaxonomyNodeID) error {
	return s.taxonomyRepo.RemoveFileMapping(ctx, workspaceID, fileID, nodeID)
}

// GetFileTaxonomies retrieves all taxonomy nodes for a file.
func (s *Service) GetFileTaxonomies(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.TaxonomyNode, error) {
	mappings, err := s.taxonomyRepo.GetFileMappings(ctx, workspaceID, fileID)
	if err != nil {
		return nil, err
	}

	nodes := make([]*entity.TaxonomyNode, 0, len(mappings))
	for _, mapping := range mappings {
		node, err := s.taxonomyRepo.GetNode(ctx, workspaceID, mapping.NodeID)
		if err != nil {
			s.logger.Debug().Err(err).Str("node_id", mapping.NodeID.String()).Msg("Failed to get node")
			continue
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// GetNodeFiles retrieves all files in a taxonomy node.
func (s *Service) GetNodeFiles(ctx context.Context, workspaceID entity.WorkspaceID, nodeID entity.TaxonomyNodeID, includeDescendants bool) ([]entity.FileID, error) {
	return s.taxonomyRepo.GetNodeFiles(ctx, workspaceID, nodeID, includeDescendants)
}

// SearchNodes searches for taxonomy nodes.
func (s *Service) SearchNodes(ctx context.Context, workspaceID entity.WorkspaceID, query string, limit int) ([]*entity.TaxonomyNode, error) {
	return s.taxonomyRepo.SearchNodes(ctx, workspaceID, query, limit)
}

// MergeNodes merges source node into target node.
func (s *Service) MergeNodes(ctx context.Context, workspaceID entity.WorkspaceID, sourceID, targetID entity.TaxonomyNodeID) error {
	return s.taxonomyRepo.MergeNodes(ctx, workspaceID, sourceID, targetID)
}

// GetStats retrieves taxonomy statistics.
func (s *Service) GetStats(ctx context.Context, workspaceID entity.WorkspaceID) (*repository.TaxonomyStats, error) {
	return s.taxonomyRepo.GetTaxonomyStats(ctx, workspaceID)
}

// InduceTaxonomy runs the Chain-of-Layer taxonomy induction process.
func (s *Service) InduceTaxonomy(ctx context.Context, req *entity.TaxonomyInductionRequest) (*entity.TaxonomyInductionResult, error) {
	s.logger.Info().
		Str("workspace_id", req.WorkspaceID.String()).
		Int("max_levels", req.MaxLevels).
		Msg("Starting taxonomy induction")

	result := &entity.TaxonomyInductionResult{}

	if req.MaxLevels == 0 {
		req.MaxLevels = s.config.MaxLevels
	}
	if req.MaxNodesPerLevel == 0 {
		req.MaxNodesPerLevel = s.config.MaxNodesPerLevel
	}

	// Collect content summaries and existing categories
	contentContext, err := s.collectContentContext(ctx, req.WorkspaceID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to collect content context: %v", err))
		return result, nil
	}

	// Get existing taxonomy for evolution
	existingNodes := []*entity.TaxonomyNode{}
	if req.IncludeExisting {
		existingNodes, err = s.taxonomyRepo.ListAll(ctx, req.WorkspaceID)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to get existing taxonomy")
		}
	}

	// Chain-of-Layer: Induce each layer progressively
	for level := 0; level < req.MaxLevels; level++ {
		nodesCreated, err := s.induceLayer(ctx, req, level, contentContext, existingNodes, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("layer %d induction failed: %v", level, err))
			break
		}
		result.NodesCreated += nodesCreated

		if nodesCreated == 0 {
			s.logger.Info().Int("level", level).Msg("No new nodes created, stopping induction")
			break
		}

		// Update existing nodes for next level
		existingNodes, _ = s.taxonomyRepo.ListAll(ctx, req.WorkspaceID)
	}

	// Auto-assign files to taxonomy nodes
	mappingsAdded, err := s.autoAssignFiles(ctx, req.WorkspaceID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("auto-assign failed: %v", err))
	}
	result.MappingsAdded = mappingsAdded

	// Auto-merge similar nodes if enabled
	if s.config.AutoMerge {
		nodesMerged, err := s.autoMergeSimilar(ctx, req.WorkspaceID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("auto-merge failed: %v", err))
		}
		result.NodesMerged = nodesMerged
	}

	s.logger.Info().
		Int("nodes_created", result.NodesCreated).
		Int("nodes_merged", result.NodesMerged).
		Int("mappings_added", result.MappingsAdded).
		Int("errors", len(result.Errors)).
		Msg("Taxonomy induction complete")

	return result, nil
}

// collectContentContext gathers content information for taxonomy induction.
func (s *Service) collectContentContext(ctx context.Context, workspaceID entity.WorkspaceID) (string, error) {
	// Collect file types, folder names, and existing metadata
	var context strings.Builder

	// Get folder distribution
	files, err := s.fileRepo.List(ctx, workspaceID, repository.FileListOptions{Limit: 1000})
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	folderCounts := make(map[string]int)
	extensionCounts := make(map[string]int)
	for _, file := range files {
		parts := strings.Split(file.RelativePath, "/")
		if len(parts) > 1 {
			folder := parts[0]
			folderCounts[folder]++
		}
		extensionCounts[file.Extension]++
	}

	context.WriteString("=== Folder Structure ===\n")
	for folder, count := range folderCounts {
		context.WriteString(fmt.Sprintf("- %s: %d files\n", folder, count))
	}

	context.WriteString("\n=== File Types ===\n")
	for ext, count := range extensionCounts {
		context.WriteString(fmt.Sprintf("- %s: %d files\n", ext, count))
	}

	// Sample document titles
	context.WriteString("\n=== Sample Files ===\n")
	sampleCount := 0
	for _, file := range files {
		if sampleCount >= 20 {
			break
		}
		context.WriteString(fmt.Sprintf("- %s\n", file.RelativePath))
		sampleCount++
	}

	return context.String(), nil
}

// induceLayer induces taxonomy nodes for a specific level.
func (s *Service) induceLayer(
	ctx context.Context,
	req *entity.TaxonomyInductionRequest,
	level int,
	contentContext string,
	existingNodes []*entity.TaxonomyNode,
	result *entity.TaxonomyInductionResult,
) (int, error) {
	if s.llmRouter == nil {
		return 0, fmt.Errorf("LLM router not configured")
	}

	// Build prompt for this layer
	prompt := s.buildLayerPrompt(level, contentContext, existingNodes, req.SeedCategories)

	response, err := s.llmRouter.Complete(ctx, prompt, 1000)
	if err != nil {
		return 0, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response into categories
	categories := s.parseCategories(response)
	if len(categories) == 0 {
		return 0, nil
	}

	// Limit nodes per level
	if len(categories) > req.MaxNodesPerLevel {
		categories = categories[:req.MaxNodesPerLevel]
	}

	nodesCreated := 0
	for _, cat := range categories {
		// Determine parent for this category
		var parentID *entity.TaxonomyNodeID
		if level > 0 && cat.ParentPath != "" {
			parent, _ := s.taxonomyRepo.GetNodeByPath(ctx, req.WorkspaceID, cat.ParentPath)
			if parent != nil {
				parentID = &parent.ID
			}
		}

		// Check if node already exists
		existing, _ := s.taxonomyRepo.GetNodeByName(ctx, req.WorkspaceID, cat.Name, parentID)
		if existing != nil {
			// Update confidence if higher
			if cat.Confidence > existing.Confidence {
				existing.Confidence = cat.Confidence
				existing.Keywords = append(existing.Keywords, cat.Keywords...)
				s.taxonomyRepo.UpdateNode(ctx, req.WorkspaceID, existing)
				result.NodesUpdated++
			}
			continue
		}

		// Create new node
		node, err := s.CreateNode(ctx, req.WorkspaceID, cat.Name, parentID, entity.TaxonomyNodeSourceInferred)
		if err != nil {
			s.logger.Debug().Err(err).Str("name", cat.Name).Msg("Failed to create node")
			continue
		}

		node.Description = cat.Description
		node.Confidence = cat.Confidence
		node.Keywords = cat.Keywords
		s.taxonomyRepo.UpdateNode(ctx, req.WorkspaceID, node)

		nodesCreated++
	}

	return nodesCreated, nil
}

// buildLayerPrompt builds the LLM prompt for a specific layer.
func (s *Service) buildLayerPrompt(level int, contentContext string, existingNodes []*entity.TaxonomyNode, seedCategories []string) string {
	var prompt strings.Builder

	prompt.WriteString("You are helping organize a file collection into a hierarchical taxonomy.\n\n")

	if level == 0 {
		prompt.WriteString("Task: Propose TOP-LEVEL categories (max 10) for organizing these files.\n")
		prompt.WriteString("These should be broad, non-overlapping categories.\n\n")
	} else {
		prompt.WriteString(fmt.Sprintf("Task: Propose SUBCATEGORIES (level %d) for the existing categories.\n", level))
		prompt.WriteString("Only add subcategories where they would help organize the content better.\n\n")
	}

	prompt.WriteString("Content Analysis:\n")
	prompt.WriteString(contentContext)
	prompt.WriteString("\n")

	if len(existingNodes) > 0 {
		prompt.WriteString("\nExisting Categories:\n")
		for _, node := range existingNodes {
			if node.Level == level-1 || (level == 0 && node.ParentID == nil) {
				prompt.WriteString(fmt.Sprintf("- %s (path: %s, files: %d)\n", node.Name, node.Path, node.DocCount))
			}
		}
	}

	if len(seedCategories) > 0 {
		prompt.WriteString("\nSuggested Categories to Consider:\n")
		for _, cat := range seedCategories {
			prompt.WriteString(fmt.Sprintf("- %s\n", cat))
		}
	}

	prompt.WriteString(`
Output Format (one per line):
CATEGORY|parent_path|description|confidence|keywords

Example:
Documents||General documents and text files|0.9|documents,text,files
Code||Source code and programming files|0.95|code,programming,source
Projects/Active|Projects|Currently active projects|0.8|active,ongoing
`)

	return prompt.String()
}

// CategoryCandidate represents a parsed category from LLM response.
type CategoryCandidate struct {
	Name        string
	ParentPath  string
	Description string
	Confidence  float64
	Keywords    []string
}

// parseCategories parses the LLM response into category candidates.
func (s *Service) parseCategories(response string) []CategoryCandidate {
	var categories []CategoryCandidate

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "Example:") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || name == "CATEGORY" {
			continue
		}

		cat := CategoryCandidate{
			Name:       name,
			Confidence: 0.7, // Default confidence
		}

		if len(parts) > 1 {
			cat.ParentPath = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			cat.Description = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			if conf, err := parseFloat(parts[3]); err == nil {
				cat.Confidence = conf
			}
		}
		if len(parts) > 4 {
			keywords := strings.Split(parts[4], ",")
			for _, kw := range keywords {
				if kw = strings.TrimSpace(kw); kw != "" {
					cat.Keywords = append(cat.Keywords, kw)
				}
			}
		}

		categories = append(categories, cat)
	}

	return categories
}

// autoAssignFiles assigns files to taxonomy nodes based on content matching.
func (s *Service) autoAssignFiles(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	// Get all taxonomy nodes
	nodes, err := s.taxonomyRepo.ListAll(ctx, workspaceID)
	if err != nil {
		return 0, err
	}

	if len(nodes) == 0 {
		return 0, nil
	}

	// Get all files
	files, err := s.fileRepo.List(ctx, workspaceID, repository.FileListOptions{Limit: 10000})
	if err != nil {
		return 0, err
	}

	mappingsAdded := 0
	for _, file := range files {
		// Find best matching node based on path and keywords
		bestNode, score := s.findBestMatchingNode(file, nodes)
		if bestNode != nil && score >= s.config.MinConfidence {
			mapping := entity.NewFileTaxonomyMapping(workspaceID, file.ID, bestNode.ID, entity.TaxonomyNodeSourceInferred)
			mapping.Score = score

			if err := s.taxonomyRepo.AddFileMapping(ctx, mapping); err != nil {
				s.logger.Debug().Err(err).Str("file", file.RelativePath).Msg("Failed to add mapping")
				continue
			}
			mappingsAdded++
		}
	}

	return mappingsAdded, nil
}

// findBestMatchingNode finds the best taxonomy node for a file.
func (s *Service) findBestMatchingNode(file *entity.FileEntry, nodes []*entity.TaxonomyNode) (*entity.TaxonomyNode, float64) {
	var bestNode *entity.TaxonomyNode
	var bestScore float64

	filePath := strings.ToLower(file.RelativePath)
	fileName := strings.ToLower(file.Filename)

	for _, node := range nodes {
		score := 0.0
		nodeName := strings.ToLower(node.Name)
		nodePath := strings.ToLower(node.Path)

		// Path contains node name
		if strings.Contains(filePath, nodeName) {
			score += 0.5
		}

		// Path starts with node path
		if strings.HasPrefix(filePath, nodePath+"/") {
			score += 0.3
		}

		// Filename contains keywords
		for _, kw := range node.Keywords {
			if strings.Contains(fileName, strings.ToLower(kw)) {
				score += 0.1
			}
		}

		// Prefer leaf nodes (more specific)
		if node.ChildCount == 0 {
			score += 0.1
		}

		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	return bestNode, bestScore
}

// autoMergeSimilar merges similar taxonomy nodes.
func (s *Service) autoMergeSimilar(ctx context.Context, workspaceID entity.WorkspaceID) (int, error) {
	nodes, err := s.taxonomyRepo.ListAll(ctx, workspaceID)
	if err != nil {
		return 0, err
	}

	merged := 0
	mergedIDs := make(map[string]bool)

	for i, nodeA := range nodes {
		if mergedIDs[nodeA.ID.String()] {
			continue
		}

		for j := i + 1; j < len(nodes); j++ {
			nodeB := nodes[j]
			if mergedIDs[nodeB.ID.String()] {
				continue
			}

			// Only merge nodes at the same level with same parent
			if nodeA.Level != nodeB.Level {
				continue
			}
			if (nodeA.ParentID == nil) != (nodeB.ParentID == nil) {
				continue
			}
			if nodeA.ParentID != nil && nodeB.ParentID != nil && *nodeA.ParentID != *nodeB.ParentID {
				continue
			}

			// Check similarity
			similarity := s.calculateNodeSimilarity(nodeA, nodeB)
			if similarity >= s.config.MergeSimilarity {
				// Keep the node with more documents
				var source, target *entity.TaxonomyNode
				if nodeA.DocCount >= nodeB.DocCount {
					target = nodeA
					source = nodeB
				} else {
					target = nodeB
					source = nodeA
				}

				if err := s.taxonomyRepo.MergeNodes(ctx, workspaceID, source.ID, target.ID); err != nil {
					s.logger.Debug().Err(err).Msg("Failed to merge nodes")
					continue
				}

				mergedIDs[source.ID.String()] = true
				merged++
			}
		}
	}

	return merged, nil
}

// calculateNodeSimilarity calculates similarity between two nodes.
func (s *Service) calculateNodeSimilarity(a, b *entity.TaxonomyNode) float64 {
	// Simple name-based similarity
	nameA := strings.ToLower(a.Name)
	nameB := strings.ToLower(b.Name)

	if nameA == nameB {
		return 1.0
	}

	// Check if one contains the other
	if strings.Contains(nameA, nameB) || strings.Contains(nameB, nameA) {
		return 0.8
	}

	// Keyword overlap
	kwA := make(map[string]bool)
	for _, kw := range a.Keywords {
		kwA[strings.ToLower(kw)] = true
	}

	overlap := 0
	for _, kw := range b.Keywords {
		if kwA[strings.ToLower(kw)] {
			overlap++
		}
	}

	if len(a.Keywords)+len(b.Keywords) > 0 {
		return float64(overlap*2) / float64(len(a.Keywords)+len(b.Keywords))
	}

	return 0.0
}

// SuggestTaxonomy suggests taxonomy categorization for a file.
func (s *Service) SuggestTaxonomy(ctx context.Context, workspaceID entity.WorkspaceID, fileID entity.FileID) ([]*entity.TaxonomySuggestion, error) {
	if s.llmRouter == nil {
		return nil, fmt.Errorf("LLM router not configured")
	}

	// Get file info
	file, err := s.fileRepo.GetByID(ctx, workspaceID, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return nil, fmt.Errorf("file not found")
	}

	// Get existing taxonomy
	nodes, err := s.taxonomyRepo.ListAll(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list taxonomy: %w", err)
	}

	// Build prompt
	prompt := s.buildSuggestionPrompt(file, nodes)

	response, err := s.llmRouter.Complete(ctx, prompt, 500)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse suggestions
	suggestions := s.parseSuggestions(workspaceID, entity.DocumentID(fileID.String()), response, nodes)
	return suggestions, nil
}

// buildSuggestionPrompt builds the prompt for taxonomy suggestion.
func (s *Service) buildSuggestionPrompt(file *entity.FileEntry, nodes []*entity.TaxonomyNode) string {
	var prompt strings.Builder

	prompt.WriteString("Suggest the best taxonomy category for this file.\n\n")
	prompt.WriteString(fmt.Sprintf("File: %s\n", file.RelativePath))
	prompt.WriteString(fmt.Sprintf("Type: %s\n", file.Extension))
	prompt.WriteString(fmt.Sprintf("Size: %d bytes\n\n", file.FileSize))

	prompt.WriteString("Available Categories:\n")
	for _, node := range nodes {
		prompt.WriteString(fmt.Sprintf("- %s (path: %s)\n", node.Name, node.Path))
	}

	prompt.WriteString(`
Output Format:
PATH|confidence|reasoning

Example:
Documents/Reports|0.85|The file name suggests it's a report document

If no existing category fits, suggest a new one:
NEW:Category Name|0.7|Reasoning for new category
`)

	return prompt.String()
}

// parseSuggestions parses taxonomy suggestions from LLM response.
func (s *Service) parseSuggestions(workspaceID entity.WorkspaceID, docID entity.DocumentID, response string, nodes []*entity.TaxonomyNode) []*entity.TaxonomySuggestion {
	var suggestions []*entity.TaxonomySuggestion

	nodesByPath := make(map[string]*entity.TaxonomyNode)
	for _, n := range nodes {
		nodesByPath[n.Path] = n
	}

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		pathOrNew := strings.TrimSpace(parts[0])
		confidence := 0.7
		reasoning := ""

		if len(parts) > 1 {
			if conf, err := parseFloat(parts[1]); err == nil {
				confidence = conf
			}
		}
		if len(parts) > 2 {
			reasoning = strings.TrimSpace(parts[2])
		}

		suggestion := &entity.TaxonomySuggestion{
			DocumentID:    docID,
			Confidence:    confidence,
			Reasoning:     reasoning,
		}

		if strings.HasPrefix(pathOrNew, "NEW:") {
			// New category suggestion
			newName := strings.TrimPrefix(pathOrNew, "NEW:")
			suggestion.NewNodeName = strings.TrimSpace(newName)
			suggestion.SuggestedPath = newName
		} else {
			// Existing category
			suggestion.SuggestedPath = pathOrNew
			if node, ok := nodesByPath[pathOrNew]; ok {
				suggestion.NodeID = &node.ID
			}
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

// EvolveTaxonomy evolves the taxonomy based on new content and feedback.
func (s *Service) EvolveTaxonomy(ctx context.Context, workspaceID entity.WorkspaceID) (*entity.TaxonomyInductionResult, error) {
	// Run induction with existing taxonomy
	return s.InduceTaxonomy(ctx, &entity.TaxonomyInductionRequest{
		WorkspaceID:      workspaceID,
		MaxLevels:        s.config.MaxLevels,
		MaxNodesPerLevel: s.config.MaxNodesPerLevel,
		IncludeExisting:  true,
	})
}

// Helper functions

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// EnsureRootCategories creates default root categories if none exist.
func (s *Service) EnsureRootCategories(ctx context.Context, workspaceID entity.WorkspaceID, categories []string) error {
	existing, err := s.taxonomyRepo.GetRootNodes(ctx, workspaceID)
	if err != nil {
		return err
	}

	existingNames := make(map[string]bool)
	for _, node := range existing {
		existingNames[node.Name] = true
	}

	for _, name := range categories {
		if existingNames[name] {
			continue
		}

		node := entity.NewTaxonomyNode(workspaceID, name, nil)
		node.Path = name
		node.Level = 0
		node.Source = entity.TaxonomyNodeSourceSystem
		node.CreatedAt = time.Now()
		node.UpdatedAt = time.Now()

		if err := s.taxonomyRepo.CreateNode(ctx, workspaceID, node); err != nil {
			s.logger.Warn().Err(err).Str("name", name).Msg("Failed to create root category")
		}
	}

	return nil
}
