# Cortex AI Features - Setup Guide

Cortex now includes optional AI-powered features that use a local LLM to enhance your file organization workflow.

## Features

1. **AI Tag Suggestions** - Analyzes file content and suggests relevant tags
2. **AI Project Assignment** - Recommends project assignments based on file context
3. **AI File Summaries** - Generates concise summaries that can be saved as notes

## Prerequisites

You'll need a local LLM service running. We recommend **Ollama** for its ease of use and excellent performance.

## Setup Instructions

### 1. Install Ollama

#### macOS
```bash
brew install ollama
```

#### Linux
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

#### Windows
Download from [ollama.com](https://ollama.com/download)

### 2. Start Ollama Service

```bash
ollama serve
```

This starts the Ollama API server at `http://localhost:11434` (default).

### 3. Download a Model

In a new terminal, pull a model. We recommend starting with a smaller, fast model:

```bash
# Fast, lightweight model (recommended for most users)
ollama pull llama3.2

# Alternative models:
ollama pull mistral      # Good balance of speed and capability
ollama pull codellama    # Optimized for code understanding
ollama pull phi3         # Very small and fast
```

### 4. Enable AI Features in Cortex

1. Open VS Code Settings (`Cmd+,` on Mac, `Ctrl+,` on Windows/Linux)
2. Search for "Cortex LLM"
3. Check **"Cortex: Llm > Enabled"**
4. Verify the endpoint is set to `http://localhost:11434` (default)
5. Set your preferred model (e.g., `llama3.2`)

Alternatively, add to your `settings.json`:

```json
{
  "cortex.llm.enabled": true,
  "cortex.llm.endpoint": "http://localhost:11434",
  "cortex.llm.model": "llama3.2",
  "cortex.llm.maxContextTokens": 2000,
  "cortex.llm.autoSummary.enabled": false,
  "cortex.llm.autoSummary.maxFileSize": 250000,
  "cortex.llm.autoSummary.maxConcurrency": 2,
  "cortex.llm.autoIndex.enabled": false,
  "cortex.llm.autoIndex.applyTags": false,
  "cortex.llm.autoIndex.applyProjects": false,
  "cortex.llm.autoIndex.useSuggestedContexts": true,
  "cortex.llm.autoIndex.maxFileSize": 250000,
  "cortex.llm.autoIndex.maxConcurrency": 2
}
```

## Usage

Once enabled, three new commands become available:

### 1. Suggest Tags (AI)

**Command:** `Cortex: Suggest Tags (AI)`

- Open any file
- Run the command from the Command Palette or right-click menu
- AI analyzes the file content and suggests relevant tags
- Select tags to apply

**Example:**
```
File: src/auth/login.ts
Suggested tags: typescript, authentication, backend, security
```

### 2. Suggest Project (AI)

**Command:** `Cortex: Suggest Project (AI)`

- Open a file
- Run the command
- AI analyzes the file and recent work context
- Suggests a project name
- You can accept, edit, or reject the suggestion

**Example:**
```
File: src/auth/login.ts
Suggested project: user-authentication
```

### 3. Generate Summary (AI)

**Command:** `Cortex: Generate File Summary (AI)`

- Open a file
- Run the command
- AI generates a concise summary
- Choose to save as notes, append to existing notes, or copy to clipboard

**Example:**
```
File: src/auth/login.ts
Summary: "Handles user authentication using JWT tokens and validates credentials against the database"
```

## Configuration Options

| Setting | Default | Description |
|---------|---------|-------------|
| `cortex.llm.enabled` | `false` | Enable/disable AI features |
| `cortex.llm.endpoint` | `http://localhost:11434` | LLM API endpoint |
| `cortex.llm.model` | `llama3.2` | Model to use |
| `cortex.llm.maxContextTokens` | `2000` | Max characters to send as context |
| `cortex.llm.autoSummary.enabled` | `false` | Auto-generate cached summaries during indexing |
| `cortex.llm.autoSummary.maxFileSize` | `250000` | Max file size (bytes) to summarize |
| `cortex.llm.autoSummary.maxConcurrency` | `2` | Concurrent summary requests during indexing |
| `cortex.llm.autoIndex.enabled` | `false` | Auto-generate AI tags/projects during indexing |
| `cortex.llm.autoIndex.applyTags` | `false` | Apply AI tags automatically |
| `cortex.llm.autoIndex.applyProjects` | `false` | Apply AI project suggestions automatically |
| `cortex.llm.autoIndex.useSuggestedContexts` | `true` | Store projects as suggestions instead of assigning |
| `cortex.llm.autoIndex.maxFileSize` | `250000` | Max file size (bytes) for tags/projects |
| `cortex.llm.autoIndex.maxConcurrency` | `2` | Concurrent tag/project requests during indexing |

## Using Other LLM Services

### LM Studio

1. Download from [lmstudio.ai](https://lmstudio.ai/)
2. Download a model through the UI
3. Click "Start Server" (usually `http://localhost:1234`)
4. In Cortex settings, set endpoint to `http://localhost:1234`

### LocalAI (Docker)

```bash
docker run -p 8080:8080 -v $PWD/models:/models localai/localai:latest
```

Set endpoint to `http://localhost:8080`

## Troubleshooting

### "LLM service not available" error

**Check if Ollama is running:**
```bash
# Test the endpoint
curl http://localhost:11434/api/tags
```

If this fails:
```bash
# Start Ollama
ollama serve
```

### Slow responses

- Use a smaller model (e.g., `phi3` instead of `llama3.2`)
- Reduce `maxContextTokens` in settings
- Ensure no other heavy processes are running

### Out of memory errors

- Switch to a smaller model
- Reduce `maxContextTokens`
- Close other applications

## Model Recommendations

| Model | Size | Speed | Quality | Best For |
|-------|------|-------|---------|----------|
| `phi3` | Small | ⚡⚡⚡ | Good | Quick suggestions, older hardware |
| `llama3.2` | Medium | ⚡⚡ | Excellent | General use (recommended) |
| `mistral` | Medium | ⚡⚡ | Excellent | Balanced performance |
| `codellama` | Large | ⚡ | Excellent | Code-heavy projects |

## Privacy & Security

- **100% Local**: All AI processing happens on your machine
- **No Cloud**: No data is sent to external servers
- **No Internet Required**: Works completely offline (after initial model download)
- **Your Code Stays Private**: File contents never leave your computer

## Performance Tips

1. **Keep Ollama running** - Starting it takes a few seconds
2. **Pre-load models** - First request downloads the model
3. **Use appropriate models** - Smaller = faster
4. **Adjust context size** - Less context = faster responses
5. **Use auto-summary carefully** - Lower file size and concurrency for faster indexing

## Example Workflow

1. Work on several related files
2. Run `Cortex: Suggest Tags (AI)` on each file
3. Run `Cortex: Suggest Project (AI)` to group them
4. Run `Cortex: Generate Summary (AI)` to document what each file does
5. Use Cortex's tree views to see your AI-organized workspace

## Advanced: Custom Endpoints

If you're using a custom LLM setup:

1. Ensure your service provides an Ollama-compatible API
2. Set the endpoint in Cortex settings
3. Make sure it responds to `/api/generate` and `/api/tags`

## Feedback & Issues

If you encounter issues or have suggestions:
- Check the Debug Console in VS Code (`Cmd+Shift+U` / `Ctrl+Shift+U`)
- Open an issue on the Cortex GitHub repository

---

**Happy organizing with AI! 🤖**
