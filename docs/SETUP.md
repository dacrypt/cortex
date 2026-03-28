# Cortex Setup Guide

This guide walks you through setting up the Cortex development environment and testing the extension.

## Prerequisites

### For VS Code Extension
- **Node.js** 16.x or higher
- **npm** 7.x or higher
- **VS Code** 1.80.0 or higher
- **Git** (for version control)
- **LibreOffice (soffice)** (optional) for extracting metadata from legacy Office files (.doc/.xls/.ppt)

### For Go Backend (Optional)
- **Go** 1.21 or higher
- **Ollama** (optional) for AI features - see [LLM_SETUP.md](LLM_SETUP.md)

## Installation

### 1. Install Dependencies

```bash
cd cortex
npm install
```

This will install:
- TypeScript compiler
- VS Code extension types
- better-sqlite3 (SQLite library)
- ESLint and related tools

**Note on better-sqlite3**: This package includes native bindings. If you encounter installation errors, you may need to install build tools:

**macOS**:
```bash
xcode-select --install
```

**Windows**:
```bash
npm install --global windows-build-tools
```

**Linux**:
```bash
sudo apt-get install build-essential
```

**Optional: LibreOffice (legacy Office metadata)**

**macOS**:
```bash
brew install --cask libreoffice
```

**Windows**:
- Download and install from https://www.libreoffice.org/download/

**Linux**:
```bash
sudo apt-get install libreoffice
```

### 2. Compile TypeScript

```bash
npm run compile
```

This compiles all TypeScript files in `src/` to JavaScript in `out/`.

**Watch Mode** (auto-compile on save):
```bash
npm run watch
```

## Running the Extension

### Method 1: Extension Development Host (F5)

1. Open the `cortex` folder in VS Code
2. Press `F5` (or Run → Start Debugging)
3. A new VS Code window opens with the extension loaded
4. Open any workspace/folder in the Extension Development Host

### Method 2: Manual Launch

1. Open VS Code
2. Go to Run and Debug sidebar (`Cmd+Shift+D` or `Ctrl+Shift+D`)
3. Select "Run Extension" from dropdown
4. Click green play button

## Testing the Extension

### Initial Test: Verify Activation

1. Open a workspace in the Extension Development Host
2. Check the Debug Console (View → Debug Console) for:
   ```
   [Cortex] Activating extension...
   [Cortex] Workspace root: /path/to/workspace
   [IndexStore] Built index with 123 files
   [MetadataStore] Initialized at /path/to/.cortex/index.sqlite
   [Cortex] Extension activated successfully
   ```

3. Verify the Cortex icon appears in the Activity Bar (left sidebar)

### Test 1: Index Workspace

1. Click the Cortex icon in Activity Bar
2. You should see multiple views:
   - **By Project** (shows "No contexts yet")
   - **By Tag** (shows "No tags yet")
   - **By Type** (shows all file types with counts)
   - **By Date** (automatically groups by modification time)
   - **By Size** (automatically groups by file size)
   - **By Folder** (shows folder structure)
   - **By Content Type** (groups by MIME type)
   - **Code Metrics** (code analysis metrics)
   - **Documents** (document statistics)
   - **Issues** (TODOs and issues)
   - **File Info** (file details panel)
   - **Biblioteca** (categorization view)

3. Check the `.cortex/` directory was created:
   ```bash
   ls -la .cortex/
   # Should show: index.sqlite
   ```

### Test 2: Add a Tag

1. Open any file in the workspace
2. Open Command Palette (`Cmd+Shift+P` or `Ctrl+Shift+P`)
3. Type "Cortex: Add tag to current file"
4. Enter a tag name (e.g., "important")
5. Check:
   - Success message appears
   - "By Tag" view now shows "important (1)"
   - Expanding the tag shows your file

### Test 3: Assign a Context

1. With a file open:
2. Command Palette → "Cortex: Assign context to current file"
3. Enter a context name (e.g., "project-alpha")
4. Check:
   - "By Context" view shows "project-alpha (1)"
   - Expanding shows your file

### Test 4: Multiple Files

1. Tag multiple files with the same tag
2. Assign multiple files to the same context
3. Verify:
   - Counts update in tree views
   - All files appear under their respective groups

### Test 5: Click to Open

1. In any Cortex view, click a file
2. Verify the file opens in the editor

### Test 6: Rebuild Index

1. Command Palette → "Cortex: Rebuild Index"
2. Progress notification appears
3. Success message shows file count
4. All views refresh

### Test 7: File Watcher

1. Create a new file in the workspace (e.g., `test.txt`)
2. Check "By Type" view - should update automatically
3. Delete the file
4. Verify it disappears from the view

## Debugging

### Enable Verbose Logging

Check the Debug Console (View → Debug Console) for all Cortex logs.

### Inspect SQLite Database

Install a SQLite viewer:
```bash
npm install -g sqlite3
```

Query the database:
```bash
sqlite3 .cortex/index.sqlite

# List tables
.tables

# View all metadata
SELECT * FROM file_metadata;

# View all tags
SELECT * FROM file_tags;

# View all contexts
SELECT * FROM file_contexts;

# Exit
.exit
```

### Common Issues

#### Extension doesn't activate
- Check VS Code version (must be 1.80.0+)
- Look for errors in Debug Console
- Try reloading window (`Cmd+R` / `Ctrl+R`)

#### SQLite errors
- Ensure better-sqlite3 installed correctly
- Check file permissions on `.cortex/` directory
- Try deleting `.cortex/` and rebuilding index

#### Files not appearing
- Check they're not in ignored directories
- Verify workspace root is correct
- Rebuild index manually

#### Views not refreshing
- Check Debug Console for errors
- Try collapsing and expanding tree nodes
- Reload window

## Project Structure Verification

After setup, your project should look like this:

```
cortex/
├── src/                          # VS Code Extension (TypeScript)
│   ├── extension.ts             # Main entry point
│   ├── core/                     # Core components
│   │   ├── FileScanner.ts        # Workspace scanning
│   │   ├── IndexStore.ts         # In-memory index
│   │   └── MetadataStore.ts      # SQLite persistence
│   ├── views/                    # Tree view providers
│   │   ├── ContextTreeProvider.ts
│   │   ├── TagTreeProvider.ts
│   │   └── ...
│   ├── commands/                  # VS Code commands
│   └── services/                  # Services (LLM, etc.)
├── backend/                      # Go Backend Daemon
│   ├── cmd/cortexd/              # Daemon entry point
│   ├── internal/
│   │   ├── domain/                # Domain entities
│   │   ├── application/           # Application services
│   │   ├── infrastructure/        # Infrastructure (SQLite, LLM)
│   │   └── interfaces/            # gRPC handlers
│   ├── api/proto/                 # gRPC proto definitions
│   └── configs/                   # Configuration files
├── docs/                          # Documentation
├── resources/                     # Resources (icons)
├── package.json                   # Extension manifest
├── tsconfig.json                  # TypeScript config
└── .cortex/                       # Workspace metadata (created at runtime)
    └── index.sqlite               # SQLite database
```

## Backend Go Setup (Optional)

Cortex includes a Go backend daemon for advanced features and integrations.

### 1. Build the Daemon

```bash
cd backend
go build -o cortexd ./cmd/cortexd
```

### 2. Configuration

Create a local configuration file:

```bash
cp backend/configs/cortexd.yaml.example backend/cortexd.local.yaml
```

Edit `cortexd.local.yaml` to configure:
- Workspace root path
- Database location
- LLM endpoint (for AI features)
- gRPC address

### 3. Run the Daemon

```bash
./cortexd --config cortexd.local.yaml
```

### 4. Test gRPC Health

```bash
go test -v ./internal/interfaces/grpc -run TestServerHealthCheck
```

### 5. Connect VS Code Extension

The VS Code extension can connect to the Go backend via gRPC. Configure the address in VS Code settings:

```json
{
  "cortex.grpc.address": "127.0.0.1:50051"
}
```

## Packaging the Extension

To create a `.vsix` file for distribution:

1. Install vsce (VS Code Extension Manager):
   ```bash
   npm install -g @vscode/vsce
   ```

2. Package the extension:
   ```bash
   vsce package
   ```

3. Install locally:
   - VS Code → Extensions → "..." menu → "Install from VSIX"
   - Select the generated `.vsix` file

## Publishing (Optional)

To publish to VS Code Marketplace:

1. Create a publisher account at https://marketplace.visualstudio.com/
2. Generate a Personal Access Token
3. Login with vsce:
   ```bash
   vsce login <publisher-name>
   ```
4. Publish:
   ```bash
   vsce publish
   ```

## Next Steps

1. **Expand Test Coverage**
   - Add more fixtures for documents and images
   - Extend command and view provider scenarios
   - Consider E2E flows in Extension Development Host

2. **Performance Testing**
   - Test with large workspaces (10k+ files)
   - Benchmark indexing speed
   - Profile memory usage

3. **User Feedback**
   - Gather feedback on UX
   - Identify missing features
   - Prioritize roadmap

4. **Documentation**
   - Add inline JSDoc comments
   - Create video walkthrough
   - Write blog post

## Development Tips

### Hot Reload
- Keep `npm run watch` running
- After code changes, reload Extension Development Host (`Cmd+R`)
- SQLite changes persist (delete `.cortex/` to reset)

### Debugging TreeViews
- Add breakpoints in TreeProvider methods
- Use `console.log()` liberally (shows in Debug Console)
- Test with small workspaces first

### TypeScript Tips
- Run `npm run lint` before committing
- Fix type errors immediately (don't use `any`)
- Use VS Code's TypeScript language server for autocomplete

---

**Ready to code!** Open [extension.ts](src/extension.ts) and start exploring.
