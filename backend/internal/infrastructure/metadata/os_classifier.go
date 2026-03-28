// Package metadata provides OS metadata classification.
package metadata

import (
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// OSContextClassifier classifies OS metadata into taxonomic dimensions.
type OSContextClassifier struct{}

// NewOSContextClassifier creates a new OS context classifier.
func NewOSContextClassifier() *OSContextClassifier {
	return &OSContextClassifier{}
}

// Classify classifies OS metadata into taxonomic dimensions.
func (c *OSContextClassifier) Classify(meta *entity.OSMetadata, entry *entity.FileEntry) *entity.OSContextTaxonomy {
	if meta == nil {
		return nil
	}

	return &entity.OSContextTaxonomy{
		Security:     c.classifySecurity(meta),
		Ownership:    c.classifyOwnership(meta),
		Temporal:     c.classifyTemporal(meta, entry),
		System:       c.classifySystem(meta, entry),
		Organization: c.classifyOrganization(meta),
	}
}

// classifySecurity classifies security-related metadata.
func (c *OSContextClassifier) classifySecurity(meta *entity.OSMetadata) *entity.SecurityTaxonomy {
	if meta.Permissions == nil {
		return nil
	}

	tax := &entity.SecurityTaxonomy{
		SecurityCategory:     []string{},
		SecurityAttributes:   []string{},
		HasACLs:             len(meta.ACLs) > 0,
		ACLComplexity:        "simple",
	}

	perms := meta.Permissions

	// Determine permission level
	if perms.OtherRead || perms.OtherWrite || perms.OtherExecute {
		tax.PermissionLevel = "public"
	} else if perms.GroupRead || perms.GroupWrite || perms.GroupExecute {
		tax.PermissionLevel = "group"
	} else if perms.OwnerRead || perms.OwnerWrite || perms.OwnerExecute {
		tax.PermissionLevel = "private"
	} else {
		tax.PermissionLevel = "restricted"
	}

	// Security categories
	if perms.OwnerRead {
		tax.SecurityCategory = append(tax.SecurityCategory, "readable_by_owner")
	}
	if perms.OwnerWrite {
		tax.SecurityCategory = append(tax.SecurityCategory, "writable_by_owner")
	}
	if perms.OwnerExecute {
		tax.SecurityCategory = append(tax.SecurityCategory, "executable_by_owner")
	}
	if perms.GroupRead {
		tax.SecurityCategory = append(tax.SecurityCategory, "readable_by_group")
	}
	if perms.GroupWrite {
		tax.SecurityCategory = append(tax.SecurityCategory, "writable_by_group")
	}
	if perms.GroupExecute {
		tax.SecurityCategory = append(tax.SecurityCategory, "executable_by_group")
	}
	if perms.OtherRead {
		tax.SecurityCategory = append(tax.SecurityCategory, "readable_by_others")
	}
	if perms.OtherWrite {
		tax.SecurityCategory = append(tax.SecurityCategory, "writable_by_others")
	}
	if perms.OtherExecute {
		tax.SecurityCategory = append(tax.SecurityCategory, "executable_by_others")
	}

	// Security attributes
	if meta.FileAttributes != nil {
		if meta.FileAttributes.IsEncrypted {
			tax.SecurityAttributes = append(tax.SecurityAttributes, "encrypted")
		}
		if meta.FileAttributes.IsImmutable {
			tax.SecurityAttributes = append(tax.SecurityAttributes, "immutable")
		}
		if meta.FileAttributes.IsReadOnly {
			tax.SecurityAttributes = append(tax.SecurityAttributes, "readonly")
		}
	}

	// Check for quarantine (macOS)
	if meta.PlatformSpecific != nil {
		if quarantined, ok := meta.PlatformSpecific["quarantined"].(bool); ok && quarantined {
			tax.SecurityAttributes = append(tax.SecurityAttributes, "quarantined")
		}
	}

	// ACL complexity
	if len(meta.ACLs) > 3 {
		tax.ACLComplexity = "complex"
	}

	return tax
}

// classifyOwnership classifies ownership-related metadata.
func (c *OSContextClassifier) classifyOwnership(meta *entity.OSMetadata) *entity.OwnershipTaxonomy {
	tax := &entity.OwnershipTaxonomy{
		AccessRelations: []string{},
	}

	// Owner type
	if meta.Owner == nil {
		tax.OwnerType = "unknown"
	} else {
		uid := meta.Owner.UID
		if uid == 0 {
			tax.OwnerType = "system"
		} else if uid < 100 {
			tax.OwnerType = "service"
		} else {
			tax.OwnerType = "user"
		}
	}

	// Group category
	if meta.Group == nil {
		tax.GroupCategory = "unknown"
	} else {
		gid := meta.Group.GID
		if gid == 0 {
			tax.GroupCategory = "admin"
		} else if gid < 100 {
			tax.GroupCategory = "service"
		} else if strings.Contains(strings.ToLower(meta.Group.GroupName), "admin") ||
			strings.Contains(strings.ToLower(meta.Group.GroupName), "sudo") {
			tax.GroupCategory = "admin"
		} else if strings.Contains(strings.ToLower(meta.Group.GroupName), "dev") ||
			strings.Contains(strings.ToLower(meta.Group.GroupName), "developer") {
			tax.GroupCategory = "developer"
		} else {
			tax.GroupCategory = "custom"
		}
	}

	// Access relations
	if meta.Permissions != nil {
		if meta.Permissions.OwnerRead || meta.Permissions.OwnerWrite || meta.Permissions.OwnerExecute {
			tax.AccessRelations = append(tax.AccessRelations, "owned_by_user")
		}
		if meta.Permissions.GroupRead || meta.Permissions.GroupWrite || meta.Permissions.GroupExecute {
			tax.AccessRelations = append(tax.AccessRelations, "accessible_by_group")
		}
		if meta.Permissions.OtherRead {
			tax.AccessRelations = append(tax.AccessRelations, "public_read")
		}
	}

	// Ownership pattern
	if meta.Permissions != nil {
		if meta.Permissions.OtherRead || meta.Permissions.OtherWrite {
			tax.OwnershipPattern = "public"
		} else if meta.Permissions.GroupRead || meta.Permissions.GroupWrite {
			tax.OwnershipPattern = "shared_group"
		} else {
			tax.OwnershipPattern = "single_owner"
		}
	}

	return tax
}

// classifyTemporal classifies temporal patterns.
func (c *OSContextClassifier) classifyTemporal(meta *entity.OSMetadata, entry *entity.FileEntry) *entity.TemporalTaxonomy {
	tax := &entity.TemporalTaxonomy{
		TimeCategory:      []string{},
		TemporalRelations: []string{},
	}

	if meta.Timestamps == nil {
		return tax
	}

	now := time.Now()
	ts := meta.Timestamps

	// Temporal pattern
	age := now.Sub(ts.Modified)
	switch {
	case age < 24*time.Hour:
		tax.TemporalPattern = "recent"
	case age < 7*24*time.Hour:
		tax.TemporalPattern = "active"
	case age < 30*24*time.Hour:
		tax.TemporalPattern = "stale"
	default:
		tax.TemporalPattern = "archived"
	}

	// Access frequency (based on access time)
	accessAge := now.Sub(ts.Accessed)
	switch {
	case accessAge < 24*time.Hour:
		tax.AccessFrequency = "frequent"
	case accessAge < 7*24*time.Hour:
		tax.AccessFrequency = "occasional"
	case accessAge < 30*24*time.Hour:
		tax.AccessFrequency = "rare"
	default:
		tax.AccessFrequency = "never"
	}

	// Time categories
	if ts.Created != nil {
		createdAge := now.Sub(*ts.Created)
		if createdAge < 7*24*time.Hour {
			tax.TimeCategory = append(tax.TimeCategory, "created_recently")
		}
	}

	modAge := now.Sub(ts.Modified)
	if modAge < 7*24*time.Hour {
		tax.TimeCategory = append(tax.TimeCategory, "modified_this_week")
	}
	if modAge < 24*time.Hour {
		tax.TimeCategory = append(tax.TimeCategory, "modified_today")
	}

	accAge := now.Sub(ts.Accessed)
	if accAge < 24*time.Hour {
		tax.TimeCategory = append(tax.TimeCategory, "accessed_today")
	}

	return tax
}

// classifySystem classifies system-related metadata.
func (c *OSContextClassifier) classifySystem(meta *entity.OSMetadata, entry *entity.FileEntry) *entity.SystemTaxonomy {
	tax := &entity.SystemTaxonomy{
		SystemAttributes: []string{},
		SystemFeatures:  []string{},
	}

	// System file type
	if meta.PlatformSpecific != nil {
		if isDir, ok := meta.PlatformSpecific["is_directory"].(bool); ok && isDir {
			tax.SystemFileType = "directory"
		} else if isSymlink, ok := meta.PlatformSpecific["is_symlink"].(bool); ok && isSymlink {
			tax.SystemFileType = "symlink"
		} else if isDevice, ok := meta.PlatformSpecific["is_device"].(bool); ok && isDevice {
			tax.SystemFileType = "device"
		} else {
			tax.SystemFileType = "regular"
		}
	} else {
		tax.SystemFileType = "regular"
	}

	// File system category (simplified - would need more info for full classification)
	if meta.FileSystem != nil {
		fsType := strings.ToLower(meta.FileSystem.FileSystemType)
		if strings.Contains(fsType, "network") || strings.Contains(fsType, "nfs") || strings.Contains(fsType, "smb") {
			tax.FileSystemCategory = "network"
		} else if strings.Contains(fsType, "tmpfs") || strings.Contains(fsType, "proc") {
			tax.FileSystemCategory = "virtual"
		} else {
			tax.FileSystemCategory = "local"
		}
	} else {
		tax.FileSystemCategory = "local"
	}

	// System attributes
	if meta.FileAttributes != nil {
		if meta.FileAttributes.IsHidden {
			tax.SystemAttributes = append(tax.SystemAttributes, "hidden")
		}
		if meta.FileAttributes.IsSystem {
			tax.SystemAttributes = append(tax.SystemAttributes, "system")
		}
		if meta.FileAttributes.IsArchive {
			tax.SystemAttributes = append(tax.SystemAttributes, "archive")
		}
		if meta.FileAttributes.IsCompressed {
			tax.SystemAttributes = append(tax.SystemAttributes, "compressed")
		}
	}

	// System features
	if len(meta.ExtendedAttrs) > 0 {
		tax.SystemFeatures = append(tax.SystemFeatures, "extended_attrs")
	}
	if len(meta.ACLs) > 0 {
		tax.SystemFeatures = append(tax.SystemFeatures, "acls")
	}
	if meta.FileSystem != nil && meta.FileSystem.Blocks > 0 {
		tax.SystemFeatures = append(tax.SystemFeatures, "block_allocated")
	}

	return tax
}

// classifyOrganization classifies organizational patterns.
func (c *OSContextClassifier) classifyOrganization(meta *entity.OSMetadata) *entity.OrganizationTaxonomy {
	tax := &entity.OrganizationTaxonomy{
		UserGrouping:   []string{},
		GroupGrouping:  []string{},
		ProjectGrouping: []string{},
		OrgPatterns:    []string{},
	}

	// User grouping
	if meta.Owner != nil {
		tax.UserGrouping = append(tax.UserGrouping, "files_by_"+meta.Owner.Username)
		if meta.Owner.UID == 0 {
			tax.UserGrouping = append(tax.UserGrouping, "files_by_root")
		}
	}

	// Group grouping
	if meta.Group != nil {
		tax.GroupGrouping = append(tax.GroupGrouping, "files_in_"+meta.Group.GroupName)
		if meta.Group.GID == 0 {
			tax.GroupGrouping = append(tax.GroupGrouping, "files_in_admin_group")
		}
	}

	// Organizational patterns
	if meta.Permissions != nil {
		if meta.Permissions.OwnerWrite && !meta.Permissions.GroupWrite && !meta.Permissions.OtherWrite {
			tax.OrgPatterns = append(tax.OrgPatterns, "user_workspace")
		}
		if meta.Permissions.GroupRead || meta.Permissions.GroupWrite {
			tax.OrgPatterns = append(tax.OrgPatterns, "shared_directory")
		}
	}

	return tax
}






