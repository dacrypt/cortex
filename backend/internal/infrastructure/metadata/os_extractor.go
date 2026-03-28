// Package metadata provides OS metadata extraction.
package metadata

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// OSExtractor extracts operating system metadata from files.
type OSExtractor struct {
	logger zerolog.Logger
}

// NewOSExtractor creates a new OS metadata extractor.
func NewOSExtractor(logger zerolog.Logger) *OSExtractor {
	return &OSExtractor{
		logger: logger.With().Str("component", "os_extractor").Logger(),
	}
}

// Extract extracts OS metadata from a file.
func (e *OSExtractor) Extract(ctx context.Context, filePath string) (*entity.OSMetadata, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	meta := &entity.OSMetadata{
		ExtendedAttrs:     make(map[string]string),
		PlatformSpecific: make(map[string]interface{}),
	}

	// Extract basic permissions and ownership
	if err := e.extractPermissions(info, meta); err != nil {
		e.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to extract permissions")
	}

	// Extract timestamps
	e.extractTimestamps(info, meta)

	// Extract file attributes
	e.extractFileAttributes(info, meta)

	// Extract file system info
	e.extractFileSystemInfo(info, meta)

	// Platform-specific extraction
	switch runtime.GOOS {
	case "darwin":
		e.extractmacOS(ctx, filePath, meta)
	case "linux":
		e.extractLinux(ctx, filePath, meta)
	case "windows":
		e.extractWindows(ctx, filePath, meta)
	}

	return meta, nil
}

// extractPermissions extracts permission and ownership information.
func (e *OSExtractor) extractPermissions(info os.FileInfo, meta *entity.OSMetadata) error {
	sys := info.Sys()
	if sys == nil {
		return fmt.Errorf("sys info not available")
	}

	// Extract permissions
	perms := info.Mode()
	meta.Permissions = &entity.PermissionsInfo{
		Octal:         fmt.Sprintf("%04o", perms&os.ModePerm),
		String:        perms.String(),
		OwnerRead:     perms&0400 != 0,
		OwnerWrite:    perms&0200 != 0,
		OwnerExecute:  perms&0100 != 0,
		GroupRead:     perms&0040 != 0,
		GroupWrite:    perms&0020 != 0,
		GroupExecute:  perms&0010 != 0,
		OtherRead:     perms&0004 != 0,
		OtherWrite:    perms&0002 != 0,
		OtherExecute:  perms&0001 != 0,
		SetUID:        perms&os.ModeSetuid != 0,
		SetGID:        perms&os.ModeSetgid != 0,
		StickyBit:     perms&os.ModeSticky != 0,
	}

	// Extract owner and group (Unix-like systems)
	if stat, ok := sys.(*syscall.Stat_t); ok {
		// Owner
		owner, err := user.LookupId(strconv.FormatUint(uint64(stat.Uid), 10))
		if err == nil {
			meta.Owner = &entity.UserInfo{
				UID:      int(stat.Uid),
				Username: owner.Username,
			}
			// Try to get full name from passwd
			if owner.Name != "" {
				meta.Owner.FullName = owner.Name
			}
		} else {
			// Fallback: just UID
			meta.Owner = &entity.UserInfo{
				UID:      int(stat.Uid),
				Username: strconv.FormatUint(uint64(stat.Uid), 10),
			}
		}

		// Group
		group, err := user.LookupGroupId(strconv.FormatUint(uint64(stat.Gid), 10))
		if err == nil {
			meta.Group = &entity.GroupInfo{
				GID:       int(stat.Gid),
				GroupName: group.Name,
				Members:   []string{}, // Group members would require additional system calls
			}
		} else {
			// Fallback: just GID
			meta.Group = &entity.GroupInfo{
				GID:       int(stat.Gid),
				GroupName: strconv.FormatUint(uint64(stat.Gid), 10),
				Members:   []string{},
			}
		}
	}

	return nil
}

// extractTimestamps extracts timestamp information.
func (e *OSExtractor) extractTimestamps(info os.FileInfo, meta *entity.OSMetadata) {
	meta.Timestamps = &entity.OSTimestamps{
		Modified: info.ModTime(),
	}

	sys := info.Sys()
	if sys == nil {
		return
	}

	// Try to extract additional timestamps from syscall.Stat_t
	// Note: Field names vary by platform (Atim on Linux, Atimespec on macOS)
	// For portability, we use ModTime as fallback
	// Platform-specific extractors can override these values
	meta.Timestamps.Accessed = info.ModTime() // Fallback to mod time
	meta.Timestamps.Changed = info.ModTime()  // Fallback to mod time
	// Birth time extraction is platform-specific and handled in platform extractors
}

// extractFileAttributes extracts file attribute flags.
func (e *OSExtractor) extractFileAttributes(info os.FileInfo, meta *entity.OSMetadata) {
	meta.FileAttributes = &entity.FileAttributes{
		IsReadOnly: info.Mode()&0200 == 0, // No write permission for owner
		IsHidden:   strings.HasPrefix(info.Name(), "."),
	}

	// Additional attributes from mode
	mode := info.Mode()
	if mode&os.ModeDir != 0 {
		meta.PlatformSpecific["is_directory"] = true
	}
	if mode&os.ModeSymlink != 0 {
		meta.PlatformSpecific["is_symlink"] = true
	}
	if mode&os.ModeDevice != 0 {
		meta.PlatformSpecific["is_device"] = true
	}
	if mode&os.ModeNamedPipe != 0 {
		meta.PlatformSpecific["is_named_pipe"] = true
	}
	if mode&os.ModeSocket != 0 {
		meta.PlatformSpecific["is_socket"] = true
	}
}

// extractFileSystemInfo extracts file system information.
func (e *OSExtractor) extractFileSystemInfo(info os.FileInfo, meta *entity.OSMetadata) {
	sys := info.Sys()
	if sys == nil {
		return
	}

	if stat, ok := sys.(*syscall.Stat_t); ok {
		meta.FileSystem = &entity.FileSystemInfo{
			Blocks:     int64(stat.Blocks),
			BlockSize:  512, // Default block size
		}
	}
}

// extractmacOS extracts macOS-specific metadata.
func (e *OSExtractor) extractmacOS(ctx context.Context, filePath string, meta *entity.OSMetadata) {
	// Extract extended attributes (xattr)
	if output, err := exec.CommandContext(ctx, "xattr", "-l", filePath).Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				meta.ExtendedAttrs[key] = value

				// Special handling for Finder tags
				if key == "com.apple.metadata:_kMDItemUserTags" {
					meta.PlatformSpecific["finder_tags"] = value
				}
				if key == "com.apple.quarantine" {
					meta.PlatformSpecific["quarantined"] = true
				}
			}
		}
	}

	// Extract ACLs
	if output, err := exec.CommandContext(ctx, "ls", "-le", filePath).Output(); err == nil {
		// Parse ACL entries from ls -le output
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
				// This is likely an ACL entry
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					acl := entity.ACLEntry{
						Type:        parts[0],
						Identity:    parts[1],
						Permissions: strings.Join(parts[2:], " "),
					}
					meta.ACLs = append(meta.ACLs, acl)
				}
			}
		}
	}
}

// extractLinux extracts Linux-specific metadata.
func (e *OSExtractor) extractLinux(ctx context.Context, filePath string, meta *entity.OSMetadata) {
	// Extract extended attributes
	if output, err := exec.CommandContext(ctx, "getfattr", "-d", "-m", "-", filePath).Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# file:") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.Trim(parts[0], `"`)
				value := strings.Trim(parts[1], `"`)
				meta.ExtendedAttrs[key] = value
			}
		}
	}

	// Extract ACLs
	if output, err := exec.CommandContext(ctx, "getfacl", filePath).Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				acl := entity.ACLEntry{
					Type:        parts[0],
					Identity:    parts[1],
					Permissions: strings.Join(parts[2:], " "),
				}
				meta.ACLs = append(meta.ACLs, acl)
			}
		}
	}

	// Extract SELinux context
	if output, err := exec.CommandContext(ctx, "getfattr", "-n", "security.selinux", filePath).Output(); err == nil {
		if len(output) > 0 {
			context := strings.TrimSpace(string(output))
			if meta.FileSystem == nil {
				meta.FileSystem = &entity.FileSystemInfo{}
			}
			meta.FileSystem.SELinuxContext = &context
		}
	}

	// Check for immutable flag (Linux)
	if output, err := exec.CommandContext(ctx, "lsattr", filePath).Output(); err == nil {
		fields := strings.Fields(string(output))
		if len(fields) >= 2 {
			attrs := fields[0]
			if strings.Contains(attrs, "i") {
				meta.FileAttributes.IsImmutable = true
			}
			if strings.Contains(attrs, "a") {
				meta.FileAttributes.IsAppendOnly = true
			}
		}
	}
}

// extractWindows extracts Windows-specific metadata.
func (e *OSExtractor) extractWindows(ctx context.Context, filePath string, meta *entity.OSMetadata) {
	// Extract file attributes using attrib command
	if output, err := exec.CommandContext(ctx, "attrib", filePath).Output(); err == nil {
		attrs := strings.TrimSpace(string(output))
		if strings.Contains(attrs, "R") {
			meta.FileAttributes.IsReadOnly = true
		}
		if strings.Contains(attrs, "H") {
			meta.FileAttributes.IsHidden = true
		}
		if strings.Contains(attrs, "S") {
			meta.FileAttributes.IsSystem = true
		}
		if strings.Contains(attrs, "A") {
			meta.FileAttributes.IsArchive = true
		}
	}

	// Extract ACLs using icacls
	if output, err := exec.CommandContext(ctx, "icacls", filePath).Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, filePath) {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				acl := entity.ACLEntry{
					Identity:    parts[0],
					Permissions: strings.Join(parts[1:], " "),
				}
				meta.ACLs = append(meta.ACLs, acl)
			}
		}
	}
}

