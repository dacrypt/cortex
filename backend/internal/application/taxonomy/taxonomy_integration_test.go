package taxonomy

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func TestTaxonomyService_CreateAndGetNode(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create root node
	rootNode, err := service.CreateNode(ctx, workspace.ID, "Documents", nil, entity.TaxonomyNodeSourceUser)
	if err != nil {
		t.Fatalf("CreateNode (root) failed: %v", err)
	}

	if rootNode.Name != "Documents" {
		t.Errorf("Expected name 'Documents', got '%s'", rootNode.Name)
	}

	if rootNode.Level != 0 {
		t.Errorf("Expected level 0 for root, got %d", rootNode.Level)
	}

	if rootNode.Path != "Documents" {
		t.Errorf("Expected path 'Documents', got '%s'", rootNode.Path)
	}

	// Create child node
	childNode, err := service.CreateNode(ctx, workspace.ID, "Reports", &rootNode.ID, entity.TaxonomyNodeSourceUser)
	if err != nil {
		t.Fatalf("CreateNode (child) failed: %v", err)
	}

	if childNode.Level != 1 {
		t.Errorf("Expected level 1 for child, got %d", childNode.Level)
	}

	if childNode.Path != "Documents/Reports" {
		t.Errorf("Expected path 'Documents/Reports', got '%s'", childNode.Path)
	}

	// Get node
	retrieved, err := service.GetNode(ctx, workspace.ID, rootNode.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved.ID != rootNode.ID {
		t.Error("GetNode returned different node")
	}
}

func TestTaxonomyService_GetNodeByPath(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create hierarchy
	root, _ := service.CreateNode(ctx, workspace.ID, "Code", nil, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Go", &root.ID, entity.TaxonomyNodeSourceUser)

	// Get by path
	found, err := service.GetNodeByPath(ctx, workspace.ID, "Code/Go")
	if err != nil {
		t.Fatalf("GetNodeByPath failed: %v", err)
	}

	if found == nil {
		t.Fatal("Expected to find node by path")
	}

	if found.Name != "Go" {
		t.Errorf("Expected name 'Go', got '%s'", found.Name)
	}
}

func TestTaxonomyService_GetRootNodes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create multiple root nodes
	rootNames := []string{"Documents", "Code", "Media"}
	for _, name := range rootNames {
		_, err := service.CreateNode(ctx, workspace.ID, name, nil, entity.TaxonomyNodeSourceUser)
		if err != nil {
			t.Fatalf("CreateNode failed: %v", err)
		}
	}

	// Create some child nodes
	roots, _ := service.GetRootNodes(ctx, workspace.ID)
	if len(roots) > 0 {
		service.CreateNode(ctx, workspace.ID, "PDF", &roots[0].ID, entity.TaxonomyNodeSourceUser)
	}

	// Get root nodes
	rootNodes, err := service.GetRootNodes(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("GetRootNodes failed: %v", err)
	}

	if len(rootNodes) != 3 {
		t.Errorf("Expected 3 root nodes, got %d", len(rootNodes))
	}

	for _, node := range rootNodes {
		if node.Level != 0 {
			t.Errorf("Root node has level %d, expected 0", node.Level)
		}
	}
}

func TestTaxonomyService_GetChildren(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create hierarchy
	root, _ := service.CreateNode(ctx, workspace.ID, "Media", nil, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Images", &root.ID, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Videos", &root.ID, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Audio", &root.ID, entity.TaxonomyNodeSourceUser)

	// Get children
	children, err := service.GetChildren(ctx, workspace.ID, root.ID)
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}

	if len(children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(children))
	}

	childNames := make(map[string]bool)
	for _, child := range children {
		childNames[child.Name] = true
	}

	if !childNames["Images"] || !childNames["Videos"] || !childNames["Audio"] {
		t.Error("Missing expected child nodes")
	}
}

func TestTaxonomyService_AddAndRemoveFileMapping(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Create a file
	fileRepo := sqlite.NewFileRepository(conn)
	file := entity.NewFileEntry("/tmp/test-workspace", "docs/readme.md", 1024, time.Now())
	file.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
	if err := fileRepo.Upsert(ctx, workspace.ID, file); err != nil {
		t.Fatalf("create file: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create taxonomy node
	node, _ := service.CreateNode(ctx, workspace.ID, "Documentation", nil, entity.TaxonomyNodeSourceUser)

	// Add file to node
	err = service.AddFileToNode(ctx, workspace.ID, file.ID, node.ID, entity.TaxonomyNodeSourceUser)
	if err != nil {
		t.Fatalf("AddFileToNode failed: %v", err)
	}

	// Get file taxonomies
	taxonomies, err := service.GetFileTaxonomies(ctx, workspace.ID, file.ID)
	if err != nil {
		t.Fatalf("GetFileTaxonomies failed: %v", err)
	}

	if len(taxonomies) != 1 {
		t.Errorf("Expected 1 taxonomy, got %d", len(taxonomies))
	}

	if len(taxonomies) > 0 && taxonomies[0].Name != "Documentation" {
		t.Errorf("Expected taxonomy 'Documentation', got '%s'", taxonomies[0].Name)
	}

	// Remove file from node
	err = service.RemoveFileFromNode(ctx, workspace.ID, file.ID, node.ID)
	if err != nil {
		t.Fatalf("RemoveFileFromNode failed: %v", err)
	}

	// Verify removal
	taxonomies, err = service.GetFileTaxonomies(ctx, workspace.ID, file.ID)
	if err != nil {
		t.Fatalf("GetFileTaxonomies after remove failed: %v", err)
	}

	if len(taxonomies) != 0 {
		t.Errorf("Expected 0 taxonomies after removal, got %d", len(taxonomies))
	}
}

func TestTaxonomyService_PolyHierarchy(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Create a file
	fileRepo := sqlite.NewFileRepository(conn)
	file := entity.NewFileEntry("/tmp/test-workspace", "api-docs.pdf", 2048, time.Now())
	file.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
	if err := fileRepo.Upsert(ctx, workspace.ID, file); err != nil {
		t.Fatalf("create file: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create multiple taxonomy nodes
	docsNode, _ := service.CreateNode(ctx, workspace.ID, "Documentation", nil, entity.TaxonomyNodeSourceUser)
	apiNode, _ := service.CreateNode(ctx, workspace.ID, "API", nil, entity.TaxonomyNodeSourceUser)

	// Add file to multiple categories (poly-hierarchy)
	service.AddFileToNode(ctx, workspace.ID, file.ID, docsNode.ID, entity.TaxonomyNodeSourceUser)
	service.AddFileToNode(ctx, workspace.ID, file.ID, apiNode.ID, entity.TaxonomyNodeSourceUser)

	// Get file taxonomies
	taxonomies, err := service.GetFileTaxonomies(ctx, workspace.ID, file.ID)
	if err != nil {
		t.Fatalf("GetFileTaxonomies failed: %v", err)
	}

	if len(taxonomies) != 2 {
		t.Errorf("Expected 2 taxonomies (poly-hierarchy), got %d", len(taxonomies))
	}
}

func TestTaxonomyService_GetNodeFiles(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Create files
	fileRepo := sqlite.NewFileRepository(conn)
	files := []*entity.FileEntry{
		entity.NewFileEntry("/tmp/test-workspace", "file1.txt", 100, time.Now()),
		entity.NewFileEntry("/tmp/test-workspace", "file2.txt", 200, time.Now()),
		entity.NewFileEntry("/tmp/test-workspace", "file3.txt", 300, time.Now()),
	}

	for _, f := range files {
		f.Enhanced = &entity.EnhancedMetadata{IndexedState: entity.IndexedState{Basic: true}}
		if err := fileRepo.Upsert(ctx, workspace.ID, f); err != nil {
			t.Fatalf("create file: %v", err)
		}
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create node and add files
	node, _ := service.CreateNode(ctx, workspace.ID, "TextFiles", nil, entity.TaxonomyNodeSourceUser)
	for _, f := range files {
		service.AddFileToNode(ctx, workspace.ID, f.ID, node.ID, entity.TaxonomyNodeSourceUser)
	}

	// Get node files
	fileIDs, err := service.GetNodeFiles(ctx, workspace.ID, node.ID, false)
	if err != nil {
		t.Fatalf("GetNodeFiles failed: %v", err)
	}

	if len(fileIDs) != 3 {
		t.Errorf("Expected 3 files, got %d", len(fileIDs))
	}
}

func TestTaxonomyService_SearchNodes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create nodes
	nodeNames := []string{"Documents", "Documentation", "Data", "Downloads"}
	for _, name := range nodeNames {
		service.CreateNode(ctx, workspace.ID, name, nil, entity.TaxonomyNodeSourceUser)
	}

	// Search for "doc"
	results, err := service.SearchNodes(ctx, workspace.ID, "doc", 10)
	if err != nil {
		t.Fatalf("SearchNodes failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results matching 'doc', got %d", len(results))
	}

	for _, r := range results {
		if r.Name != "Documents" && r.Name != "Documentation" {
			t.Errorf("Unexpected result: %s", r.Name)
		}
	}
}

func TestTaxonomyService_DeleteNode(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create node
	node, _ := service.CreateNode(ctx, workspace.ID, "ToDelete", nil, entity.TaxonomyNodeSourceUser)

	// Delete node
	err = service.DeleteNode(ctx, workspace.ID, node.ID)
	if err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// Verify deletion
	deleted, err := service.GetNode(ctx, workspace.ID, node.ID)
	if err == nil && deleted != nil {
		t.Error("Expected node to be deleted")
	}
}

func TestTaxonomyService_EnsureRootCategories(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Ensure default categories
	defaultCategories := []string{"Documents", "Code", "Media", "Archives"}
	err = service.EnsureRootCategories(ctx, workspace.ID, defaultCategories)
	if err != nil {
		t.Fatalf("EnsureRootCategories failed: %v", err)
	}

	// Verify categories were created
	roots, err := service.GetRootNodes(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("GetRootNodes failed: %v", err)
	}

	if len(roots) != 4 {
		t.Errorf("Expected 4 root categories, got %d", len(roots))
	}

	// Call again - should be idempotent
	err = service.EnsureRootCategories(ctx, workspace.ID, defaultCategories)
	if err != nil {
		t.Fatalf("EnsureRootCategories (2nd call) failed: %v", err)
	}

	roots, _ = service.GetRootNodes(ctx, workspace.ID)
	if len(roots) != 4 {
		t.Errorf("Expected still 4 root categories after 2nd call, got %d", len(roots))
	}
}

func TestTaxonomyService_GetStats(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/test-workspace", "test-workspace")
	workspaceRepo := sqlite.NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	taxonomyRepo := sqlite.NewTaxonomyRepository(conn)
	fileRepo := sqlite.NewFileRepository(conn)
	docRepo := sqlite.NewDocumentRepository(conn)
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, fileRepo, docRepo, nil, logger)

	// Create hierarchy
	root, _ := service.CreateNode(ctx, workspace.ID, "Root", nil, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Child1", &root.ID, entity.TaxonomyNodeSourceUser)
	service.CreateNode(ctx, workspace.ID, "Child2", &root.ID, entity.TaxonomyNodeSourceUser)

	// Get stats
	stats, err := service.GetStats(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalNodes < 3 {
		t.Errorf("Expected at least 3 nodes, got %d", stats.TotalNodes)
	}

	if stats.MaxDepth < 1 {
		t.Errorf("Expected max depth at least 1, got %d", stats.MaxDepth)
	}
}

func TestServiceConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := DefaultServiceConfig()

	if config.MaxLevels != 4 {
		t.Errorf("Expected MaxLevels 4, got %d", config.MaxLevels)
	}

	if config.MaxNodesPerLevel != 20 {
		t.Errorf("Expected MaxNodesPerLevel 20, got %d", config.MaxNodesPerLevel)
	}

	if config.MinConfidence != 0.6 {
		t.Errorf("Expected MinConfidence 0.6, got %f", config.MinConfidence)
	}

	if !config.AutoMerge {
		t.Error("Expected AutoMerge to be true")
	}

	if config.MergeSimilarity != 0.85 {
		t.Errorf("Expected MergeSimilarity 0.85, got %f", config.MergeSimilarity)
	}
}

func TestParseFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"0.5", 0.5, false},
		{"0.85", 0.85, false},
		{"1.0", 1.0, false},
		{" 0.9 ", 0.9, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseFloat(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s'", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("parseFloat(%s) = %f, expected %f", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestCalculateNodeSimilarity(t *testing.T) {
	t.Parallel()

	taxonomyRepo := sqlite.NewTaxonomyRepository(nil) // Won't be used
	logger := zerolog.Nop()
	config := DefaultServiceConfig()

	service := NewService(config, taxonomyRepo, nil, nil, nil, logger)
	workspaceID := entity.NewWorkspaceID()

	tests := []struct {
		name       string
		nodeA      *entity.TaxonomyNode
		nodeB      *entity.TaxonomyNode
		minSim     float64
		maxSim     float64
	}{
		{
			name:   "identical names",
			nodeA:  &entity.TaxonomyNode{Name: "Documents"},
			nodeB:  &entity.TaxonomyNode{Name: "Documents"},
			minSim: 1.0,
			maxSim: 1.0,
		},
		{
			name:   "one contains other",
			nodeA:  &entity.TaxonomyNode{Name: "Documents"},
			nodeB:  &entity.TaxonomyNode{Name: "Document"},
			minSim: 0.7,
			maxSim: 0.9,
		},
		{
			name:   "completely different",
			nodeA:  &entity.TaxonomyNode{Name: "Code"},
			nodeB:  &entity.TaxonomyNode{Name: "Images"},
			minSim: 0.0,
			maxSim: 0.5,
		},
		{
			name:   "keyword overlap",
			nodeA:  &entity.TaxonomyNode{Name: "Reports", Keywords: []string{"report", "document", "pdf"}},
			nodeB:  &entity.TaxonomyNode{Name: "Documents", Keywords: []string{"document", "text", "file"}},
			minSim: 0.0,
			maxSim: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.nodeA.ID = entity.NewTaxonomyNodeID()
			tt.nodeA.WorkspaceID = workspaceID
			tt.nodeB.ID = entity.NewTaxonomyNodeID()
			tt.nodeB.WorkspaceID = workspaceID

			sim := service.calculateNodeSimilarity(tt.nodeA, tt.nodeB)

			if sim < tt.minSim || sim > tt.maxSim {
				t.Errorf("calculateNodeSimilarity() = %f, expected between %f and %f",
					sim, tt.minSim, tt.maxSim)
			}
		})
	}
}
