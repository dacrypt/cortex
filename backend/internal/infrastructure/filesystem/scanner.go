// Package filesystem provides file system operations for Cortex.
package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
)

// ScanProgress reports scanning progress.
// This is an alias for service.ScanProgress to maintain backward compatibility.
type ScanProgress = service.ScanProgress

// Scanner scans directories for files.
type Scanner struct {
	workspaceRoot string
	config        *entity.WorkspaceConfig
	mu            sync.RWMutex
}

// NewScanner creates a new file scanner.
func NewScanner(workspaceRoot string, config *entity.WorkspaceConfig) *Scanner {
	cfg := config
	if cfg == nil {
		defaultCfg := entity.DefaultWorkspaceConfig()
		cfg = &defaultCfg
	}

	return &Scanner{
		workspaceRoot: workspaceRoot,
		config:        cfg,
	}
}

// Scan scans the workspace and returns all files.
func (s *Scanner) Scan(ctx context.Context, progress chan<- ScanProgress) ([]*entity.FileEntry, error) {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	var files []*entity.FileEntry
	var mu sync.Mutex
	scanned := 0

	// First pass: count files
	total := 0
	err := filepath.WalkDir(s.workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path
		relPath, _ := filepath.Rel(s.workspaceRoot, path)
		if relPath == "." {
			return nil
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip excluded directories
		if d.IsDir() {
			if config.ShouldExcludePath(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded extensions
		ext := strings.ToLower(filepath.Ext(path))
		if config.ShouldExcludeExtension(ext) {
			return nil
		}

		total++
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Report initial progress
	if progress != nil {
		progress <- ScanProgress{
			FilesScanned: 0,
			FilesTotal:   total,
			Phase:        "scanning",
			Percentage:   0,
		}
	}

	// Second pass: collect file entries
	err = filepath.WalkDir(s.workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path
		relPath, _ := filepath.Rel(s.workspaceRoot, path)
		if relPath == "." {
			return nil
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip excluded directories
		if d.IsDir() {
			if config.ShouldExcludePath(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded extensions
		ext := strings.ToLower(filepath.Ext(path))
		if config.ShouldExcludeExtension(ext) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		// Create file entry
		entry := entity.NewFileEntry(s.workspaceRoot, relPath, info.Size(), info.ModTime())

		mu.Lock()
		files = append(files, entry)
		scanned++
		mu.Unlock()

		// Report progress periodically
		if progress != nil && scanned%100 == 0 {
			percentage := float64(scanned) / float64(total) * 100
			progress <- ScanProgress{
				FilesScanned: scanned,
				FilesTotal:   total,
				CurrentPath:  relPath,
				Phase:        "scanning",
				Percentage:   percentage,
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Report completion
	if progress != nil {
		progress <- ScanProgress{
			FilesScanned: scanned,
			FilesTotal:   total,
			Phase:        "complete",
			Percentage:   100,
			Completed:    true,
		}
	}

	return files, nil
}

// ScanFile scans a single file and returns its entry.
func (s *Scanner) ScanFile(ctx context.Context, relativePath string) (*entity.FileEntry, error) {
	absPath := filepath.Join(s.workspaceRoot, relativePath)

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, os.ErrInvalid
	}

	return entity.NewFileEntry(s.workspaceRoot, relativePath, info.Size(), info.ModTime()), nil
}

// GetFileInfo gets file info without creating an entry.
// This method implements the service.FileIndexer interface.
func (s *Scanner) GetFileInfo(relativePath string) (service.FileInfo, error) {
	absPath := filepath.Join(s.workspaceRoot, relativePath)
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	return NewFileInfoAdapter(info), nil
}

// GetFileInfoOS gets file info as os.FileInfo (for backward compatibility).
func (s *Scanner) GetFileInfoOS(relativePath string) (os.FileInfo, error) {
	absPath := filepath.Join(s.workspaceRoot, relativePath)
	return os.Stat(absPath)
}

// Ensure Scanner implements service.FileIndexer interface.
var _ service.FileIndexer = (*Scanner)(nil)

// Exists checks if a file exists.
func (s *Scanner) Exists(relativePath string) bool {
	absPath := filepath.Join(s.workspaceRoot, relativePath)
	_, err := os.Stat(absPath)
	return err == nil
}

// ReadFile reads the content of a file.
func (s *Scanner) ReadFile(relativePath string) ([]byte, error) {
	absPath := filepath.Join(s.workspaceRoot, relativePath)
	return os.ReadFile(absPath)
}

// ReadFileHead reads the first n bytes of a file.
func (s *Scanner) ReadFileHead(relativePath string, n int) ([]byte, error) {
	absPath := filepath.Join(s.workspaceRoot, relativePath)

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:read], nil
}

// UpdateConfig updates the scanner configuration.
func (s *Scanner) UpdateConfig(config *entity.WorkspaceConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// CollectStats collects statistics about the workspace.
func (s *Scanner) CollectStats(ctx context.Context) (*service.IndexStats, error) {
	stats := &service.IndexStats{
		ExtensionCounts: make(map[string]int),
		FolderCounts:    make(map[string]int),
		SizeBuckets:     make(map[string]int),
	}

	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	err := filepath.WalkDir(s.workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(s.workspaceRoot, path)
		if relPath == "." {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			if config.ShouldExcludePath(relPath) {
				return filepath.SkipDir
			}
			stats.FolderCounts[relPath]++
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if config.ShouldExcludeExtension(ext) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		stats.TotalFiles++
		stats.TotalSize += info.Size()
		stats.ExtensionCounts[ext]++

		// Size buckets
		bucket := getSizeBucket(info.Size())
		stats.SizeBuckets[bucket]++

		// Track most recent
		if stats.MostRecent == nil || info.ModTime().After(*stats.MostRecent) {
			t := info.ModTime()
			stats.MostRecent = &t
		}

		return nil
	})

	return stats, err
}

func getSizeBucket(size int64) string {
	switch {
	case size < 1024:
		return "<1KB"
	case size < 10*1024:
		return "1-10KB"
	case size < 100*1024:
		return "10-100KB"
	case size < 1024*1024:
		return "100KB-1MB"
	case size < 10*1024*1024:
		return "1-10MB"
	case size < 100*1024*1024:
		return "10-100MB"
	default:
		return ">100MB"
	}
}
