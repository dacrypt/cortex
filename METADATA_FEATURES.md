# Cortex Metadata Groupings

## Overview

Cortex now extracts **rich metadata** from files and provides powerful automatic groupings. No tagging required!

## New Views

### 1. By Date (📅)
Automatically groups files by modification time:
- **Last Hour** - Recently modified files
- **Today** - Modified today
- **This Week** - Modified in the last 7 days
- **This Month** - Modified in the last 30 days
- **Last 3 Months** - Recent work
- **Last 6 Months** - Medium-term files
- **This Year** - Current year
- **Older** - Archived content

**Use Cases**:
- Find what you worked on today
- Review this week's changes
- Identify stale files for cleanup
- Track recent activity

**Features**:
- Files sorted by recency (newest first)
- Shows exact modification time on hover
- Updates automatically as files change

---

### 2. By Size (💾)
Groups files by file size:
- **Huge** - >= 1 MB (large assets, videos, databases)
- **Large** - 100 KB - 1 MB (PDFs, images)
- **Medium** - 10 KB - 100 KB (typical source files)
- **Small** - 1 KB - 10 KB (config files)
- **Tiny** - < 1 KB (small configs, placeholders)

**Use Cases**:
- Identify large files consuming disk space
- Find bloated config files
- Optimize bundle size
- Clean up large assets

**Features**:
- Shows total size per category
- Files sorted by size (largest first)
- Human-readable sizes (KB, MB, GB)

---

### 3. By Folder (📁)
Semantic folder structure view:
- Mirrors your workspace structure
- Shows file counts per folder
- Expandable folder tree

**Use Cases**:
- Navigate project structure
- See folder contents at a glance
- Understand project organization
- Find files by location

**Features**:
- Hierarchical tree view
- File counts in parentheses
- Collapsible folders
- Root-level files shown separately

---

## Metadata Extraction

### File Statistics
Automatically extracted:
- **Size** - File size in bytes
- **Modified** - Last modification timestamp
- **Created** - File creation timestamp
- **Accessed** - Last access timestamp
- **Read-only** - Whether file is read-only
- **Hidden** - Whether file is hidden (starts with .)

### Git Metadata (Optional)
If workspace is a Git repository:
- **Last Author** - Who last modified the file
- **Last Commit Date** - When last committed
- **Last Commit Message** - Commit description
- **Branch** - Current Git branch

### Folder Metadata
- **Folder Path** - Directory containing the file
- **Depth** - Folder nesting level
- **Root vs Nested** - Whether in root directory

### Language Detection
Automatically infers programming language:
- TypeScript, JavaScript, Python, Java, Go, Rust, C++, C, Ruby, PHP, Swift, Kotlin, C#
- HTML, CSS, SCSS, SQL
- Shell, Bash, Markdown

---

## How It Works

### Architecture

```
┌──────────────────────────────────────┐
│  MetadataExtractor                   │
│  - extractStats()                    │
│  - extractGitMetadata()              │
│  - extractFolderInfo()               │
│  - inferLanguage()                   │
└──────────────────────────────────────┘
              ↓
┌──────────────────────────────────────┐
│  FileIndexEntry (Enhanced)           │
│  - Basic info (path, name, ext)      │
│  - enhanced: EnhancedMetadata {      │
│      stats: FileStats                │
│      git?: GitMetadata               │
│      folder: string                  │
│      depth: number                   │
│      language?: string               │
│    }                                 │
└──────────────────────────────────────┘
              ↓
┌──────────────────────────────────────┐
│  TreeView Providers                  │
│  - DateTreeProvider                  │
│  - SizeTreeProvider                  │
│  - FolderTreeProvider                │
└──────────────────────────────────────┘
```

### Categorization Logic

**Date Categorization**:
```typescript
const diff = now - timestamp;
if (diff < 1 hour) return 'Last Hour';
if (diff < 1 day) return 'Today';
if (diff < 7 days) return 'This Week';
// ... etc
```

**Size Categorization**:
```typescript
if (bytes < 1 KB) return 'Tiny';
if (bytes < 10 KB) return 'Small';
if (bytes < 100 KB) return 'Medium';
if (bytes < 1 MB) return 'Large';
return 'Huge';
```

---

## Usage Examples

### Example 1: Find Recent Work

**Scenario**: What did I work on today?

**Steps**:
1. Click Cortex icon
2. Expand "By Date"
3. Click "Today"
4. See all files modified today, sorted by time

**Result**: Instant view of today's work

---

### Example 2: Identify Large Files

**Scenario**: Reduce repository size

**Steps**:
1. Expand "By Size"
2. Click "Huge (X files, Y MB)"
3. Review large files
4. Delete or compress unnecessary files

**Result**: Easy identification of size hogs

---

### Example 3: Explore Project Structure

**Scenario**: Understand a new codebase

**Steps**:
1. Expand "By Folder"
2. Navigate through folders
3. See file counts to understand organization
4. Click files to preview

**Result**: Quick codebase comprehension

---

## Performance

### Extraction Speed
- **File Stats**: <1ms per file (filesystem API)
- **Git Metadata**: ~10-50ms per file (spawns git process)
- **Total**: 1,000 files indexed in ~1-5 seconds

### Caching
- Metadata cached in `FileIndexEntry.enhanced`
- No re-extraction unless file changes
- Incremental updates via file watcher

### Memory Usage
- ~500 bytes per file for enhanced metadata
- 10,000 files = ~5 MB additional memory
- Negligible for typical workspaces

---

## Future Enhancements

### Planned Features

1. **By Author** (Git)
   - Group files by last Git author
   - "Files modified by Alice"
   - Team contribution view

2. **By Language**
   - Group by programming language
   - "All TypeScript files"
   - Better than "By Type"

3. **By Git Status**
   - Modified, Staged, Untracked
   - Uncommitted changes view

4. **By Activity**
   - Frequently edited files
   - Recently accessed
   - Abandoned files (not touched in months)

5. **Custom Metadata**
   - Extract from file content (comments, headers)
   - Parse package.json, tsconfig.json
   - Custom extractors via plugins

6. **Search Filters**
   - Combine criteria: "Large TypeScript files from this week"
   - Boolean queries
   - Saved searches

---

## Configuration (Future)

```json
{
  "cortex.metadata.enableGit": true,
  "cortex.metadata.sizeCategories": {
    "tiny": 1024,
    "small": 10240,
    "medium": 102400,
    "large": 1048576
  },
  "cortex.metadata.dateCategories": {
    "today": 86400000,
    "thisWeek": 604800000,
    "thisMonth": 2592000000
  }
}
```

---

## Comparison: Manual vs Automatic

| Feature | Manual Tags | Automatic Metadata |
|---------|-------------|-------------------|
| **Setup** | Requires tagging | Zero configuration |
| **Maintenance** | Manual updates | Automatic |
| **Coverage** | Incomplete | 100% of files |
| **Use Case** | Semantic meaning | Temporal/structural |
| **Examples** | "urgent", "review" | "Today", "Large" |

**Best Practice**: Use **both**!
- Manual tags for semantic organization ("project-alpha", "needs-review")
- Automatic metadata for temporal/structural views ("Today", "Huge files")

---

## API Reference

### MetadataExtractor

```typescript
class MetadataExtractor {
  extractStats(absolutePath: string): Promise<FileStats>
  extractGitMetadata(absolutePath: string): Promise<GitMetadata | undefined>
  extractFolderInfo(relativePath: string): { folder: string; depth: number }
  inferLanguage(extension: string): string | undefined
  extractAll(absolutePath, relativePath, extension): Promise<EnhancedMetadata>

  // Categorization
  categorizeSize(bytes: number): string
  categorizeDate(timestamp: number): string

  // Formatting
  formatSize(bytes: number): string
  formatDate(timestamp: number): string
}
```

### Types

```typescript
interface FileStats {
  size: number;
  created: number;
  modified: number;
  accessed: number;
  isReadOnly: boolean;
  isHidden: boolean;
}

interface GitMetadata {
  lastAuthor?: string;
  lastCommitDate?: number;
  lastCommitMessage?: string;
  branch?: string;
}

interface EnhancedMetadata {
  stats: FileStats;
  git?: GitMetadata;
  folder: string;
  depth: number;
  language?: string;
}
```

---

## Testing

### Test Date View
1. Modify a file → Should appear in "Today"
2. Create a new file → Should appear in "Last Hour"
3. Check old files → Should be in "Older"

### Test Size View
1. Create large file (>1MB) → Should appear in "Huge"
2. Create small config → Should appear in "Tiny"
3. Check categories → Should show total sizes

### Test Folder View
1. Expand folders → Should show structure
2. Check file counts → Should be accurate
3. Create new folder → Should appear automatically

---

**Cortex Metadata** - Automatic, powerful, zero-configuration file organization.
