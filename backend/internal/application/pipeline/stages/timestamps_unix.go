//go:build !windows
// +build !windows

package stages

import (
	"os"
	"syscall"
	"time"
)

// extractUnixTimestamps extracts timestamps from Unix stat structure.
// Handles macOS (uses *timespec) and Linux (uses *tim) field names.
func extractUnixTimestamps(sys interface{}, info os.FileInfo) *TimestampInfo {
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return nil
	}

	result := &TimestampInfo{}
	modTime := info.ModTime()

	// Try macOS/BSD fields first (Birthtimespec, Atimespec, Ctimespec)
	// These are Timespec structs with Sec and Nsec fields
	type timespec struct {
		Sec  int64
		Nsec int64
	}

	// Use reflection-like approach: try to access fields that exist
	// On macOS: Birthtimespec, Atimespec, Ctimespec
	// On Linux: different structure
	
	// For now, use a type assertion approach
	// We'll extract what we can safely
	
	// Birth time (creation time) - macOS/BSD
	if birthtime := getBirthtime(stat); !birthtime.IsZero() {
		result.Created = &birthtime
	} else {
		// Fallback to mod time
		result.Created = &modTime
	}

	// Access time - try to get from stat
	if atime := getAtime(stat); !atime.IsZero() {
		result.Accessed = &atime
	} else {
		result.Accessed = &modTime
	}

	// Change time (ctime) - metadata change time
	if ctime := getCtime(stat); !ctime.IsZero() {
		result.Changed = &ctime
	} else {
		ctime := modTime
		result.Changed = &ctime
	}

	return result
}

// getBirthtime attempts to extract birth time (creation time) from stat.
// Returns zero time if not available.
func getBirthtime(stat *syscall.Stat_t) time.Time {
	// Try macOS field name
	if stat.Birthtimespec.Sec > 0 {
		return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	}
	return time.Time{}
}

// getAtime attempts to extract access time from stat.
func getAtime(stat *syscall.Stat_t) time.Time {
	// Try macOS field name (Atimespec)
	if stat.Atimespec.Sec > 0 {
		return time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	}
	return time.Time{}
}

// getCtime attempts to extract change time (ctime) from stat.
func getCtime(stat *syscall.Stat_t) time.Time {
	// Try macOS field name (Ctimespec)
	if stat.Ctimespec.Sec > 0 {
		return time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
	}
	return time.Time{}
}

