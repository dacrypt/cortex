# Contributing to Cortex

Thank you for your interest in contributing to Cortex! This guide will help you get started.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- No tolerance for harassment or discrimination

## Getting Started

### 1. Set Up Development Environment

Follow [SETUP.md](SETUP.md) to install dependencies and run the extension.

### 2. Understand the Architecture

Read [ARCHITECTURE.md](ARCHITECTURE.md) to understand the design principles and component structure.

### 3. Check the Roadmap

See [ROADMAP.md](ROADMAP.md) for planned features. Pick something that interests you!

## Development Workflow

### Making Changes

1. **Fork the repository**
   ```bash
   git clone https://github.com/yourusername/cortex.git
   cd cortex
   ```

2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
   - Follow the existing code style (TypeScript, eslint rules)
   - Add comments for complex logic
   - Keep functions small and focused

4. **Test your changes**
   - Run the extension in Extension Development Host (F5)
   - Test all affected commands
   - Verify views update correctly

5. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

   Use conventional commits:
   - `feat:` New feature
   - `fix:` Bug fix
   - `docs:` Documentation changes
   - `refactor:` Code refactoring
   - `test:` Adding tests
   - `chore:` Maintenance tasks

6. **Push and create a PR**
   ```bash
   git push origin feature/your-feature-name
   ```

   Then open a Pull Request on GitHub.

## Code Style

### TypeScript

- **Use strict typing** - Avoid `any`, prefer specific types
- **Null safety** - Check for null/undefined
- **Async/await** - Prefer over callbacks/promises
- **Arrow functions** - For short functions

```typescript
// GOOD
async function getMetadata(fileId: string): Promise<FileMetadata | null> {
  if (!this.db) {
    throw new Error('Database not initialized');
  }
  return this.db.prepare('SELECT * FROM files WHERE id = ?').get(fileId);
}

// AVOID
function getMetadata(fileId: any, callback: Function): void {
  this.db.prepare('SELECT * FROM files WHERE id = ' + fileId).get(callback);
}
```

### Comments

- **Explain why, not what** - Code should be self-explanatory
- **JSDoc for public APIs** - Document parameters and return types
- **Inline comments for complex logic** - Help future maintainers

```typescript
/**
 * Generate a stable file ID from a relative path
 * Uses SHA-256 hash to create a deterministic identifier
 *
 * @param relativePath - Workspace-relative path
 * @returns Hexadecimal hash string
 */
export function generateFileId(relativePath: string): string {
  return crypto.createHash('sha256').update(relativePath).digest('hex');
}
```

### File Organization

- **One class per file** - Makes code easier to navigate
- **Group related files** - core/, views/, commands/, etc.
- **Barrel exports** - Use index.ts for public APIs

## Architecture Guidelines

### Separation of Concerns

- **Core layer** - Business logic, no UI dependencies
- **View layer** - UI components, no direct metadata access
- **Command layer** - Orchestration, delegates to core and views

### Data Flow

```
User Action → Command → MetadataStore → Database
                      ↓
                  Refresh Views ← TreeProvider queries MetadataStore
```

### Error Handling

- **Validate inputs** - Check for null, undefined, empty strings
- **Show user-friendly messages** - Use `vscode.window.showErrorMessage`
- **Log errors** - Use `console.error` for debugging

```typescript
try {
  metadataStore.addTag(relativePath, tag);
} catch (error) {
  console.error('Failed to add tag:', error);
  vscode.window.showErrorMessage(`Could not add tag: ${error.message}`);
}
```

## Testing

### Manual Testing

1. **Test happy paths** - Normal usage
2. **Test edge cases** - Empty workspace, no files, etc.
3. **Test error cases** - Invalid input, missing files
4. **Test performance** - Large workspaces (10k+ files)

### Automated Testing (Future)

We plan to add:
- Unit tests (Jest)
- Integration tests (VS Code test runner)
- E2E tests (automated Extension Development Host)

## Pull Request Guidelines

### Before Submitting

- [ ] Code compiles without errors (`npm run compile`)
- [ ] No linter errors (`npm run lint`)
- [ ] Tested manually in Extension Development Host
- [ ] Updated README/docs if needed
- [ ] Commit messages follow conventional commits

### PR Description Template

```markdown
## Description
Brief description of what this PR does.

## Motivation
Why is this change needed? What problem does it solve?

## Changes
- List of specific changes
- File by file if large PR

## Testing
How did you test this? Steps to reproduce:
1. Step one
2. Step two
3. Expected result

## Screenshots (if applicable)
Before/after screenshots for UI changes

## Checklist
- [ ] Code compiles
- [ ] No linter errors
- [ ] Tested manually
- [ ] Docs updated
```

## Bug Reports

When reporting bugs, include:

1. **VS Code version** - Help → About
2. **Cortex version** - Check package.json
3. **Steps to reproduce** - Detailed, numbered steps
4. **Expected behavior** - What should happen
5. **Actual behavior** - What actually happens
6. **Logs** - From Debug Console (View → Debug Console)
7. **Screenshots** - If relevant

## Feature Requests

When suggesting features:

1. **Use case** - Why do you need this?
2. **Proposed solution** - How would it work?
3. **Alternatives** - Other ways to solve the problem
4. **Compatibility** - Does it fit the core principles?

## Documentation

Help improve docs:

- **Fix typos** - Even small fixes are appreciated!
- **Add examples** - Real-world usage scenarios
- **Clarify confusing sections** - If you struggled, others will too
- **Translate** - Help make Cortex accessible globally

## Community

- **GitHub Discussions** - Ask questions, share ideas
- **GitHub Issues** - Report bugs, request features
- **Pull Requests** - Contribute code

## Recognition

Contributors will be:
- Listed in CONTRIBUTORS.md
- Mentioned in release notes
- Appreciated by the community! 🎉

---

**Thank you for contributing to Cortex!** Every contribution, big or small, makes a difference.
