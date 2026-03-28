package entity

import (
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// WorkspaceID is a unique identifier for a workspace.
type WorkspaceID string

// NewWorkspaceID creates a new unique WorkspaceID.
func NewWorkspaceID() WorkspaceID {
	return WorkspaceID(uuid.New().String())
}

// String returns the string representation of WorkspaceID.
func (id WorkspaceID) String() string {
	return string(id)
}

// Workspace represents a registered workspace.
type Workspace struct {
	ID          WorkspaceID
	Path        string
	Name        string
	Active      bool
	LastIndexed *time.Time
	FileCount   int
	Config      WorkspaceConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewWorkspace creates a new workspace.
func NewWorkspace(path string, name string) *Workspace {
	if name == "" {
		name = filepath.Base(path)
	}
	now := time.Now()

	return &Workspace{
		ID:        NewWorkspaceID(),
		Path:      path,
		Name:      name,
		Active:    true,
		Config:    DefaultWorkspaceConfig(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// DataDir returns the data directory for this workspace.
func (w *Workspace) DataDir() string {
	return filepath.Join(w.Path, ".cortex")
}

// DatabasePath returns the SQLite database path for this workspace.
func (w *Workspace) DatabasePath() string {
	return filepath.Join(w.DataDir(), "index.sqlite")
}

// MirrorDir returns the mirror directory for extracted content.
func (w *Workspace) MirrorDir() string {
	return filepath.Join(w.DataDir(), "mirror")
}

// WorkspaceConfig contains workspace-specific settings.
type WorkspaceConfig struct {
	// Paths to exclude from indexing
	ExcludedPaths []string

	// Extensions to exclude from indexing
	ExcludedExtensions []string

	// Whether to automatically index on file changes
	AutoIndex bool

	// Whether LLM features are enabled for this workspace
	LLMEnabled bool

	// Custom settings
	CustomSettings map[string]string
}

// DefaultWorkspaceConfig returns the default workspace configuration.
func DefaultWorkspaceConfig() WorkspaceConfig {
	return WorkspaceConfig{
		ExcludedPaths: []string{
			".git",
			".svn",
			".hg",
			"node_modules",
			"vendor",
			".vscode",
			".idea",
			".cortex",
			"dist",
			"build",
			"out",
			".next",
			"target",
			"bin",
			"obj",
			"__pycache__",
			".pytest_cache",
			".mypy_cache",
			"coverage",
			".nyc_output",
		},
		ExcludedExtensions: []string{
			".exe",
			".dll",
			".so",
			".dylib",
			".a",
			".o",
			".obj",
			".pyc",
			".pyo",
			".class",
			".jar",
			".war",
			".ear",
		},
		AutoIndex:      true,
		LLMEnabled:     false,
		CustomSettings: make(map[string]string),
	}
}

// ShouldExcludePath returns true if the path should be excluded from indexing.
func (c *WorkspaceConfig) ShouldExcludePath(relativePath string) bool {
	for _, excluded := range c.ExcludedPaths {
		// Check if path starts with excluded directory
		if matched, _ := filepath.Match(excluded+"/*", relativePath); matched {
			return true
		}
		if matched, _ := filepath.Match(excluded, relativePath); matched {
			return true
		}
		// Check if any path component matches
		for dir := relativePath; dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			if filepath.Base(dir) == excluded {
				return true
			}
		}
	}
	return false
}

// ShouldExcludeExtension returns true if the extension should be excluded.
func (c *WorkspaceConfig) ShouldExcludeExtension(ext string) bool {
	for _, excluded := range c.ExcludedExtensions {
		if ext == excluded {
			return true
		}
	}
	return false
}
