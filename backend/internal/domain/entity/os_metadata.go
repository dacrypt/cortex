// Package entity contains domain entities for OS metadata and user/person models.
package entity

import (
	"time"
)

// OSMetadata contains operating system metadata for a file.
type OSMetadata struct {
	// Permissions and Ownership
	Permissions *PermissionsInfo
	Owner       *UserInfo
	Group       *GroupInfo

	// File Attributes
	FileAttributes *FileAttributes
	ExtendedAttrs  map[string]string // xattr/ADS

	// ACLs
	ACLs []ACLEntry

	// Timestamps
	Timestamps *OSTimestamps

	// File System
	FileSystem *FileSystemInfo

	// Platform-specific data
	PlatformSpecific map[string]interface{}
}

// PermissionsInfo contains detailed permission information.
type PermissionsInfo struct {
	Octal         string // "0644"
	String        string // "-rw-r--r--"
	OwnerRead     bool
	OwnerWrite    bool
	OwnerExecute  bool
	GroupRead     bool
	GroupWrite    bool
	GroupExecute  bool
	OtherRead     bool
	OtherWrite    bool
	OtherExecute  bool
	SetUID        bool
	SetGID        bool
	StickyBit     bool
}

// UserInfo contains user information from the OS.
type UserInfo struct {
	UID      int
	Username string
	FullName string // If available
	HomeDir  string // If available
	Shell    string // If available
}

// GroupInfo contains group information from the OS.
type GroupInfo struct {
	GID       int
	GroupName string
	Members   []string // Group members
}

// FileAttributes contains file attribute flags.
type FileAttributes struct {
	IsReadOnly   bool
	IsHidden     bool
	IsSystem     bool
	IsArchive    bool
	IsCompressed bool
	IsEncrypted  bool
	IsImmutable  bool // Linux
	IsAppendOnly bool // Linux
	IsNoDump     bool // Linux
}

// ACLEntry represents an Access Control List entry.
type ACLEntry struct {
	Type        string // "user", "group", "mask", "other"
	Identity    string // User or group name
	Permissions string // "rwx", "r--", etc.
	Flags       string // Optional flags
}

// OSTimestamps contains OS-level timestamps.
type OSTimestamps struct {
	Created  *time.Time
	Modified time.Time
	Accessed time.Time
	Changed  time.Time // Metadata changed
	Backup   *time.Time
}

// FileSystemInfo contains file system information.
type FileSystemInfo struct {
	MountPoint     string
	DeviceID       string
	FileSystemType string
	BlockSize      int64
	Blocks         int64
	SELinuxContext *string // Linux
}

// OSContextTaxonomy organizes OS metadata into multiple taxonomic dimensions.
type OSContextTaxonomy struct {
	// Security Dimension
	Security *SecurityTaxonomy

	// Ownership Dimension
	Ownership *OwnershipTaxonomy

	// Temporal Dimension
	Temporal *TemporalTaxonomy

	// System Dimension
	System *SystemTaxonomy

	// Organization Dimension
	Organization *OrganizationTaxonomy
}

// SecurityTaxonomy classifies security-related metadata.
type SecurityTaxonomy struct {
	// Permission level
	PermissionLevel string // "public", "group", "private", "restricted"

	// Security categories
	SecurityCategory []string // ["readable_by_group", "writable_by_owner", "executable"]

	// Security attributes
	SecurityAttributes []string // ["encrypted", "immutable", "quarantined"]

	// ACLs present
	HasACLs       bool
	ACLComplexity string // "simple", "complex"
}

// OwnershipTaxonomy classifies ownership-related metadata.
type OwnershipTaxonomy struct {
	// Owner type
	OwnerType string // "user", "system", "service", "unknown"

	// Group category
	GroupCategory string // "admin", "developer", "service", "custom"

	// Access relations
	AccessRelations []string // ["owned_by_user", "accessible_by_group", "public_read"]

	// Ownership patterns
	OwnershipPattern string // "single_owner", "shared_group", "multi_user"
}

// TemporalTaxonomy classifies temporal patterns.
type TemporalTaxonomy struct {
	// Temporal pattern
	TemporalPattern string // "recent", "archived", "active", "stale"

	// Access frequency
	AccessFrequency string // "frequent", "occasional", "rare", "never"

	// Time categories
	TimeCategory []string // ["created_recently", "modified_this_week", "accessed_today"]

	// Temporal relations
	TemporalRelations []string // ["newer_than", "older_than", "same_period"]
}

// SystemTaxonomy classifies system-related metadata.
type SystemTaxonomy struct {
	// System file type
	SystemFileType string // "regular", "directory", "symlink", "device", "special"

	// File system category
	FileSystemCategory string // "local", "network", "removable", "virtual"

	// System attributes
	SystemAttributes []string // ["hidden", "system", "archive", "compressed"]

	// System features
	SystemFeatures []string // ["extended_attrs", "acls", "hard_links", "sparse"]
}

// OrganizationTaxonomy classifies organizational patterns.
type OrganizationTaxonomy struct {
	// User grouping
	UserGrouping []string // ["files_by_david", "files_by_service"]

	// Group grouping
	GroupGrouping []string // ["files_in_admin_group", "files_in_dev_group"]

	// Project grouping (inferred)
	ProjectGrouping []string // ["project_a_files", "project_b_files"]

	// Organizational patterns
	OrgPatterns []string // ["user_workspace", "shared_directory", "project_folder"]
}

// Person represents a human person (can have multiple system users).
type Person struct {
	ID          string
	WorkspaceID string
	Name        string
	Email       *string
	DisplayName string
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SystemUser represents a system OS user account.
type SystemUser struct {
	ID          string
	WorkspaceID string
	PersonID    *string // FK to Person (optional, system user may not have associated person)
	Username    string
	UID         int
	FullName    string
	HomeDir     string
	Shell       string
	System      bool // If it's a system user (root, daemon, etc.)
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FileOwnership represents file ownership relationship.
type FileOwnership struct {
	FileID        string
	WorkspaceID   string
	UserID        string // FK to SystemUser
	OwnershipType string // "owner", "group_member", "other"
	Permissions   string // Specific permissions for this user
	DetectedAt    time.Time
}

// FileAccess represents file access relationship (ACL, etc.).
type FileAccess struct {
	ID          string
	FileID      string
	WorkspaceID string
	UserID      string // FK to SystemUser
	AccessType  string // "read", "write", "execute", "full"
	Source      string // "permissions", "acl", "group_membership"
	DetectedAt  time.Time
}

// ProjectMembership represents membership in projects.
type ProjectMembership struct {
	ProjectID   string
	WorkspaceID string
	PersonID    string // FK to Person
	Role        string // "owner", "contributor", "viewer"
	JoinedAt    time.Time
}






