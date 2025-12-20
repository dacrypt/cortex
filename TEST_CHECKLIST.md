# Cortex Extension - Test Checklist

## Pre-Test Verification
- [x] Code compiled successfully (`npm run compile`)
- [x] All TypeScript files compiled to JavaScript in `out/`
- [x] MetadataStoreJSON is being used (no native dependencies)
- [x] Package.json configured correctly

## How to Test

### Step 1: Launch Extension Development Host
1. Open the `cortex` folder in VS Code
2. Press **F5** (or click Run → Start Debugging)
3. A new VS Code window will open with "[Extension Development Host]" in the title

### Step 2: Open a Test Workspace
In the Extension Development Host window:
1. File → Open Folder
2. Choose any folder with some files (your test workspace)

### Step 3: Verify Activation
1. Open Debug Console in the main VS Code window (View → Debug Console)
2. You should see:
   ```
   [Cortex] Activating extension...
   [Cortex] Workspace root: /path/to/your/workspace
   [MetadataStore] Initialized new store at /path/.cortex/index.json
   [IndexStore] Built index with X files
   [Cortex] Indexed X files
   [Cortex] Extension activated successfully
   ```

**If you see these messages**: ✅ Extension activated successfully!

**If you see errors**: Check the Debug Console for the error message

### Step 4: Verify UI Elements

In the Extension Development Host window:

- [ ] **Activity Bar**: Cortex icon appears (brain/network icon)
- [ ] **Sidebar**: Click Cortex icon, should show three views:
  - [ ] "BY CONTEXT" (may be empty initially)
  - [ ] "BY TAG" (may be empty initially)
  - [ ] "BY TYPE" (should show file types)

**Expected**: "BY TYPE" should automatically show categories like:
- `typescript (X files)`
- `javascript (X files)`
- `json (X files)`
- etc.

### Step 5: Test Commands

#### Test 1: Add Tag
1. Open any file in the workspace
2. Command Palette (`Cmd+Shift+P` or `Ctrl+Shift+P`)
3. Type: "Cortex: Add tag to current file"
4. Enter tag name: `test`
5. Expected:
   - [ ] Success message: "Added tag 'test' to [filename]"
   - [ ] Cortex sidebar → "BY TAG" → shows "test (1)"
   - [ ] Click "test" → expands → shows your file
   - [ ] Click the file → opens it

#### Test 2: Assign Context
1. With a file open
2. Command Palette → "Cortex: Assign context to current file"
3. Enter context name: `demo-project`
4. Expected:
   - [ ] Success message
   - [ ] Cortex sidebar → "BY CONTEXT" → shows "demo-project (1)"
   - [ ] Expand → shows your file

#### Test 3: Multiple Tags/Contexts
1. Open a different file
2. Add tag: `important`
3. Add context: `demo-project` (same as before)
4. Expected:
   - [ ] "important (1)" appears in tags
   - [ ] "demo-project (2)" now shows 2 files
   - [ ] Expanding shows both files

#### Test 4: Rebuild Index
1. Command Palette → "Cortex: Rebuild Index"
2. Expected:
   - [ ] Progress notification appears
   - [ ] Success message: "Cortex: Index rebuilt (X files)"
   - [ ] All views refresh

### Step 6: Verify Metadata Storage

1. In the workspace, check for `.cortex/` directory
2. Open `.cortex/index.json`
3. Should see JSON structure:
   ```json
   {
     "files": { ... },
     "tags": { ... },
     "contexts": { ... },
     "types": { ... }
   }
   ```

### Step 7: Test File Operations

#### Create New File
1. In the workspace, create a new file: `test.md`
2. Expected:
   - [ ] File automatically appears in "BY TYPE" → "markdown"

#### Delete File
1. Delete a file from the workspace
2. Expected:
   - [ ] File disappears from all views

## Common Issues

### Issue: Commands not found
**Symptom**: "command 'cortex.addTag' not found"
**Check**:
- Debug Console for activation errors
- Extension actually activated (check for "[Cortex] Extension activated successfully")

### Issue: "No data provider registered"
**Symptom**: Views show error message
**Check**:
- Debug Console for errors during TreeView registration
- Extension activated fully

### Issue: Extension doesn't activate
**Check**:
- You opened a folder/workspace (not just files)
- Debug Console for errors
- Package.json has correct activation events

### Issue: Can't find Cortex icon
**Look for**: Brain/network icon in Activity Bar (left sidebar)
**If missing**: Check Debug Console for errors during viewContainer registration

## Success Criteria

✅ **Extension is working if**:
1. All activation messages appear in Debug Console
2. Cortex icon appears in Activity Bar
3. Three views render in sidebar
4. "Add tag" command works and file appears under tag
5. "Assign context" command works
6. Files are clickable and open correctly
7. `.cortex/index.json` is created and contains metadata

## If Something Fails

1. **Check Debug Console first** - errors will be shown there
2. **Reload Extension Development Host**: `Cmd+R` or `Ctrl+R`
3. **Restart debugging**: Stop (Shift+F5) and start again (F5)
4. **Check this repo's issues**: [GitHub Issues]
5. **Report the error**: Include Debug Console output

---

**Ready to test!** Press F5 and follow the steps above.
