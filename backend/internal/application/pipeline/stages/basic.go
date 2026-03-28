// Package stages provides pipeline processing stages.
package stages

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/utils"
)

// BasicStage extracts basic file statistics.
type BasicStage struct{}

// NewBasicStage creates a new basic stats stage.
func NewBasicStage() *BasicStage {
	return &BasicStage{}
}

// Name returns the stage name.
func (s *BasicStage) Name() string {
	return "basic"
}

// Process extracts basic file information.
func (s *BasicStage) Process(ctx context.Context, entry *entity.FileEntry) error {
	info, err := os.Stat(entry.AbsolutePath)
	if err != nil {
		return err
	}

	entry.FileSize = info.Size()
	entry.LastModified = info.ModTime()
	entry.Filename = filepath.Base(entry.AbsolutePath)
	entry.Extension = strings.ToLower(filepath.Ext(entry.AbsolutePath))

	// Calculate relative path depth
	if entry.Enhanced == nil {
		entry.Enhanced = &entity.EnhancedMetadata{}
	}

	// Initialize Stats if needed
	if entry.Enhanced.Stats == nil {
		entry.Enhanced.Stats = &entity.FileStats{}
	}

	// Extract timestamps
	entry.Enhanced.Stats.Size = info.Size()
	entry.Enhanced.Stats.Modified = info.ModTime()
	
	// Try to extract additional timestamps from system-specific info
	sys := info.Sys()
	extractedTimestamps := extractTimestampsFromSys(sys, info)
	
	// Set Created time
	if extractedTimestamps.Created != nil {
		entry.Enhanced.Stats.Created = *extractedTimestamps.Created
	} else {
		// Fallback: use ModTime for Created if not available
		entry.Enhanced.Stats.Created = info.ModTime()
	}
	
	// Set Accessed time
	if extractedTimestamps.Accessed != nil {
		entry.Enhanced.Stats.Accessed = *extractedTimestamps.Accessed
	} else {
		// Fallback: use ModTime for Accessed if not available
		entry.Enhanced.Stats.Accessed = info.ModTime()
	}
	
	// Set Changed time (ctime on Unix)
	entry.Enhanced.Stats.Changed = extractedTimestamps.Changed
	
	// Set Backup time (if available)
	entry.Enhanced.Stats.Backup = extractedTimestamps.Backup

	// Extract file attributes
	entry.Enhanced.Stats.IsReadOnly = (info.Mode()&0222) == 0
	entry.Enhanced.Stats.IsHidden = strings.HasPrefix(filepath.Base(entry.AbsolutePath), ".")

	depth := strings.Count(entry.RelativePath, string(os.PathSeparator))
	entry.Enhanced.Depth = depth
	entry.Enhanced.IndexedState.Basic = true

	// Extract folder name
	dir := filepath.Dir(entry.RelativePath)
	if dir != "." {
		entry.Enhanced.Folder = filepath.Base(dir)
	}

	// Extract path components for semantic analysis
	pathAnalyzer := utils.NewPathAnalyzer()
	entry.Enhanced.PathComponents = pathAnalyzer.ExtractComponents(entry.RelativePath)
	entry.Enhanced.PathPattern = pathAnalyzer.ExtractPattern(entry.RelativePath)

	// Calculate file hashes for duplicate detection (only for files < 100MB to avoid performance issues)
	if entry.FileSize > 0 && entry.FileSize < 100*1024*1024 {
		hashes, err := s.calculateFileHashes(entry.AbsolutePath)
		if err == nil {
			entry.Enhanced.FileHash = hashes
		}
	}

	// Perform basic metadata consistency checks
	consistency := s.checkMetadataConsistency(entry)
	entry.Enhanced.MetadataConsistency = consistency

	return nil
}

// calculateFileHashes calculates MD5, SHA256, and SHA512 hashes of a file.
func (s *BasicStage) calculateFileHashes(filePath string) (*entity.FileHash, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	md5Hash := md5.New()
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()

	multiWriter := io.MultiWriter(md5Hash, sha256Hash, sha512Hash)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return nil, err
	}

	return &entity.FileHash{
		MD5:    hex.EncodeToString(md5Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
		SHA512: hex.EncodeToString(sha512Hash.Sum(nil)),
	}, nil
}

// checkMetadataConsistency performs basic consistency checks on file metadata.
func (s *BasicStage) checkMetadataConsistency(entry *entity.FileEntry) *entity.MetadataConsistency {
	consistency := &entity.MetadataConsistency{
		Score:    1.0,
		Issues:   []string{},
		Warnings: []string{},
	}

	if entry.Enhanced == nil || entry.Enhanced.Stats == nil {
		return consistency
	}

	stats := entry.Enhanced.Stats

	// Check timestamp consistency: Created <= Modified <= Accessed
	if !stats.Created.IsZero() && !stats.Modified.IsZero() {
		if stats.Created.After(stats.Modified) {
			consistency.Issues = append(consistency.Issues, "Creation time is after modification time")
			consistency.Score -= 0.3
		}
	}

	if !stats.Modified.IsZero() && !stats.Accessed.IsZero() {
		if stats.Modified.After(stats.Accessed) {
			consistency.Warnings = append(consistency.Warnings, "Modification time is after access time (unusual but possible)")
			consistency.Score -= 0.1
		}
	}

	if stats.Changed != nil && !stats.Modified.IsZero() {
		if stats.Changed.After(stats.Modified) {
			// This is actually normal - ctime can be after mtime if metadata changed
			// Just log as info, not an issue
		}
	}

	// Check file size consistency
	if entry.FileSize < 0 {
		consistency.Issues = append(consistency.Issues, "File size is negative")
		consistency.Score -= 0.5
	}

	// Normalize score to 0.0-1.0 range
	if consistency.Score < 0.0 {
		consistency.Score = 0.0
	}
	if consistency.Score > 1.0 {
		consistency.Score = 1.0
	}

	return consistency
}

// TimestampInfo holds extracted timestamp information.
type TimestampInfo struct {
	Created *time.Time
	Accessed *time.Time
	Changed *time.Time
	Backup *time.Time
}

// extractTimestampsFromSys extracts timestamps from system-specific file info.
// This function handles platform-specific implementations.
func extractTimestampsFromSys(sys interface{}, info os.FileInfo) TimestampInfo {
	result := TimestampInfo{}
	
	// Platform-specific extraction will be implemented based on build tags
	// For now, we'll use a cross-platform approach that works on Unix-like systems
	// On Windows, we'll need different handling
	
	// Try to extract from Unix stat structure
	if unixTimestamps := extractUnixTimestamps(sys, info); unixTimestamps != nil {
		result.Created = unixTimestamps.Created
		result.Accessed = unixTimestamps.Accessed
		result.Changed = unixTimestamps.Changed
		result.Backup = unixTimestamps.Backup
		return result
	}
	
	// Fallback: use ModTime for Created and Accessed
	modTime := info.ModTime()
	result.Created = &modTime
	result.Accessed = &modTime
	// Changed time is same as ModTime on systems where we can't get ctime
	result.Changed = &modTime
	
	return result
}

// extractUnixTimestamps is implemented in platform-specific files (timestamps_unix.go, timestamps_windows.go)

// DateRange represents a date range for grouping.
type DateRange string

const (
	DateRangeToday     DateRange = "today"
	DateRangeYesterday DateRange = "yesterday"
	DateRangeThisWeek  DateRange = "this_week"
	DateRangeThisMonth DateRange = "this_month"
	DateRangeOlder     DateRange = "older"
)

// GetDateRange returns the date range for a given time.
func GetDateRange(t time.Time) DateRange {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)

	fileDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	switch {
	case !fileDate.Before(today):
		return DateRangeToday
	case !fileDate.Before(yesterday):
		return DateRangeYesterday
	case !fileDate.Before(weekAgo):
		return DateRangeThisWeek
	case !fileDate.Before(monthAgo):
		return DateRangeThisMonth
	default:
		return DateRangeOlder
	}
}

// SizeRange represents a size range for grouping.
type SizeRange string

const (
	SizeRangeTiny   SizeRange = "tiny"   // < 1 KB
	SizeRangeSmall  SizeRange = "small"  // 1 KB - 100 KB
	SizeRangeMedium SizeRange = "medium" // 100 KB - 1 MB
	SizeRangeLarge  SizeRange = "large"  // 1 MB - 10 MB
	SizeRangeHuge   SizeRange = "huge"   // > 10 MB
)

// GetSizeRange returns the size range for a given file size.
func GetSizeRange(size int64) SizeRange {
	const (
		KB = 1024
		MB = 1024 * KB
	)

	switch {
	case size < KB:
		return SizeRangeTiny
	case size < 100*KB:
		return SizeRangeSmall
	case size < MB:
		return SizeRangeMedium
	case size < 10*MB:
		return SizeRangeLarge
	default:
		return SizeRangeHuge
	}
}
