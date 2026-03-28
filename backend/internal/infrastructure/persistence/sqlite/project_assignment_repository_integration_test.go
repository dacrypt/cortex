package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

func TestProjectAssignmentRepositoryFlows(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	projectRepo := NewProjectRepository(conn)
	project := entity.NewProject(workspace.ID, "Project Alpha", nil)
	if err := projectRepo.Create(ctx, workspace.ID, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	repo := NewProjectAssignmentRepository(conn)
	fileID := entity.NewFileID("docs/readme.md")

	assignment := &entity.ProjectAssignment{
		WorkspaceID: workspace.ID,
		FileID:      fileID,
		ProjectID:   project.ID,
		ProjectName: project.Name,
		Score:       0.82,
		Sources:     []string{"metadata_context"},
		Status:      entity.ProjectAssignmentAuto,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.Upsert(ctx, assignment); err != nil {
		t.Fatalf("upsert assignment: %v", err)
	}

	secondary := &entity.ProjectAssignment{
		WorkspaceID: workspace.ID,
		FileID:      fileID,
		ProjectName: "Project Beta",
		Score:       0.6,
		Sources:     []string{"llm_fallback"},
		Status:      entity.ProjectAssignmentSuggested,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := repo.Upsert(ctx, secondary); err != nil {
		t.Fatalf("upsert secondary assignment: %v", err)
	}

	assignments, err := repo.ListByFile(ctx, workspace.ID, fileID)
	if err != nil {
		t.Fatalf("list by file: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	byProject, err := repo.ListByProject(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatalf("list by project: %v", err)
	}
	if len(byProject) != 1 {
		t.Fatalf("expected 1 assignment for project, got %d", len(byProject))
	}

	assignment.Score = 0.9
	assignment.UpdatedAt = time.Now()
	if err := repo.Upsert(ctx, assignment); err != nil {
		t.Fatalf("upsert updated assignment: %v", err)
	}

	assignments, err = repo.ListByFile(ctx, workspace.ID, fileID)
	if err != nil {
		t.Fatalf("list by file after update: %v", err)
	}
	if assignments[0].Score < 0.9 {
		t.Fatalf("expected updated score, got %f", assignments[0].Score)
	}

	if err := repo.DeleteByFile(ctx, workspace.ID, fileID); err != nil {
		t.Fatalf("delete by file: %v", err)
	}
	assignments, err = repo.ListByFile(ctx, workspace.ID, fileID)
	if err != nil {
		t.Fatalf("list by file after delete: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected no assignments after delete, got %d", len(assignments))
	}
}

