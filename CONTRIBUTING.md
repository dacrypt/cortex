# Contributing to Cortex

Thank you for your interest in contributing to Cortex. This guide covers everything you need to get started.

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Node.js | 18+ | Extension build and test |
| Go | 1.22+ | Backend daemon |
| VS Code | 1.80+ | Extension host |
| Ollama | latest | AI features (optional) |
| protoc | 3+ | Protocol buffer compiler (only if editing .proto files) |

### First-Time Setup

```bash
# Clone the repository
git clone https://github.com/dacrypt/cortex.git
cd cortex

# Install extension dependencies
npm install

# Build the extension
npm run compile

# Build the backend
cd backend
make build
```

### Running Locally

**Terminal 1 — Backend:**

```bash
cd backend
cp configs/cortexd.yaml.example cortexd.local.yaml
# Edit cortexd.local.yaml: set watch_paths to a test directory
make run
```

**Terminal 2 — Extension:**

Open the project in VS Code and press `F5`. This launches a second VS Code window (the Extension Development Host) with the extension loaded.

## Project Structure

The codebase is split into two main parts:

### Extension (`src/`)

TypeScript code that runs inside VS Code.

| Directory | Contents |
|-----------|----------|
| `src/core/` | gRPC client wrappers (`GrpcAdminClient.ts`, `GrpcMetadataClient.ts`, etc.) |
| `src/views/` | Tree view providers — facet-based architecture with a base class in `views/base/` |
| `src/frontend/` | WebView panels (admin dashboard, metrics, pipeline progress) |
| `src/commands/` | VS Code command handlers — one file per command |
| `src/services/` | Cross-cutting services (realtime updates, progress tracking, AI quality) |
| `src/models/` | TypeScript interfaces and type definitions |

### Backend (`backend/`)

Go code that runs as a standalone daemon.

| Directory | Contents |
|-----------|----------|
| `cmd/cortexd/` | Entry point — dependency injection, server startup |
| `api/proto/` | Protocol Buffer definitions for all gRPC services |
| `internal/domain/` | Domain entities and repository interfaces |
| `internal/application/` | Business logic — pipeline stages, services |
| `internal/infrastructure/` | Implementations — SQLite repos, LLM providers, file watcher |
| `internal/interfaces/grpc/` | gRPC handlers and proto-to-domain adapters |

The backend follows clean architecture: domain → application → infrastructure → interfaces.

## Making Changes

### Adding a VS Code Command

1. Create `src/commands/yourCommand.ts` with an exported function
2. Register it in `src/extension.ts` inside `activate()`
3. Add the command entry to `package.json` under `contributes.commands`
4. If it needs a menu entry, add it under `contributes.menus`

### Adding a Tree View (Facet)

1. Create a provider extending `BaseFacetTreeProvider` in `src/views/`
2. Implement the `IFacetProvider` interface from `src/views/contracts/`
3. Register it in the facet registry (`src/views/contracts/FacetRegistry.ts`)
4. Add the view ID to `package.json` under `contributes.views` if it's a standalone view

### Adding a gRPC Service Method

1. Edit the `.proto` file in `backend/api/proto/cortex/v1/`
2. Run `make proto` in the backend directory to regenerate Go code
3. Implement the handler in `backend/internal/interfaces/grpc/handlers/`
4. Add the adapter in `backend/internal/interfaces/grpc/adapters/`
5. Create or update the TypeScript client in `src/core/Grpc*Client.ts`

### Adding a Pipeline Stage

1. Create the stage in `backend/internal/application/pipeline/stages/`
2. Implement the `Stage` interface
3. Register it in the pipeline orchestrator configuration
4. Add event publishing for frontend progress tracking

## Code Style

### TypeScript

- Use `import` with `node:` prefix for Node.js builtins (`import * as path from "node:path"`)
- Prefer `async/await` over raw Promises
- One command per file in `src/commands/`
- Follow existing patterns — look at similar files before writing new ones

### Go

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `zerolog` for structured logging
- Repository methods return domain types, never proto types
- Adapters in `interfaces/grpc/adapters/` handle all proto conversion

### Commits

- Write clear, descriptive commit messages
- Use imperative mood: "Add tag suggestion command" not "Added tag suggestion command"
- Reference issues when applicable: "Fix #42: handle empty workspace gracefully"

## Testing

### Extension Tests

```bash
npm test
```

Tests are in `src/test/suite/`. Use the helpers in `src/test/helpers/testHelpers.ts` for mocking:

- `createMockContext()` — mock VS Code ExtensionContext
- `createMockMetadataStore()` — mock IMetadataStore
- `withMockedFileCache()` — mock FileCacheService

### Backend Tests

```bash
cd backend
make test
```

For PDF extraction tests (requires `pdfinfo` and `exiftool`):

```bash
make install-pdf-tools   # Installs poppler-utils and exiftool
make test-pdf
```

### Integration Tests

Integration tests require the backend daemon to be running. They are located in files ending with `Integration.test.ts` and are skipped automatically if the backend is not available.

## Pull Requests

1. Fork the repository and create a branch from `main`
2. Make your changes and ensure tests pass
3. Write or update tests for your changes
4. Update documentation if you changed behavior or added features
5. Submit a pull request with a clear description of what and why

### PR Checklist

- [ ] Code compiles without errors (`npm run compile` and `make build`)
- [ ] Tests pass (`npm test` and `make test`)
- [ ] Linter passes (`npm run lint`)
- [ ] New features have tests
- [ ] Documentation is updated if needed

## Reporting Issues

When filing a bug report, please include:

- Your OS and VS Code version
- Steps to reproduce the issue
- Expected vs. actual behavior
- Relevant logs from the Debug Console or daemon output

## Questions

If you have questions about the codebase or how to implement something, open a [Discussion](https://github.com/dacrypt/cortex/discussions) on GitHub.
