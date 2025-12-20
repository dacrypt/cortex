# 🚀 START HERE - Test Cortex Extension

## Everything is ready! Just follow these 3 steps:

### Step 1: Press F5
- Make sure you have the `cortex` folder open in VS Code
- Press **F5** on your keyboard (or click Run → Start Debugging)
- A new VS Code window will open titled **"[Extension Development Host]"**

### Step 2: Open a test folder
In the new Extension Development Host window:
- File → Open Folder
- Choose any folder with files (your workspace, a project, anything)

### Step 3: Check it works
Look for these signs of success:

✅ **Cortex icon appears** in the Activity Bar (left sidebar)
- It's a brain/network icon with connected nodes

✅ **Click the icon** → You should see three views:
- "BY CONTEXT"
- "BY TAG"
- "BY TYPE" ← This should already show your file types!

✅ **Try adding a tag**:
1. Open any file
2. Command Palette (`Cmd+Shift+P`)
3. Type: "Cortex: Add tag to current file"
4. Enter: `test`
5. Check sidebar → "BY TAG" → should show "test (1)"

## What if it doesn't work?

### Check the Debug Console:
In the **original** VS Code window (not Extension Development Host):
1. View → Debug Console
2. Look for error messages
3. You should see:
   ```
   [Cortex] Activating extension...
   [Cortex] Extension activated successfully
   ```

### Common fixes:
- **Reload**: Press `Cmd+R` (or `Ctrl+R`) in the Extension Development Host
- **Restart**: Stop debugging (Shift+F5) and press F5 again
- **Check**: Make sure you opened a **folder**, not just files

## Next Steps

If it works:
- ✅ Read [EXAMPLES.md](EXAMPLES.md) for real-world usage scenarios
- ✅ Check [README.md](README.md) for full documentation
- ✅ See [ROADMAP.md](ROADMAP.md) for future features

If it doesn't work:
- ❌ Check the Debug Console for errors
- ❌ Share the error message
- ❌ See [TEST_CHECKLIST.md](TEST_CHECKLIST.md) for detailed troubleshooting

---

**Current Status:**
- ✅ Code compiled successfully
- ✅ No TypeScript errors
- ✅ Using JSON storage (no native module issues)
- ✅ Launch configuration ready
- ✅ All files in place

**Just press F5 to test!** 🎯
