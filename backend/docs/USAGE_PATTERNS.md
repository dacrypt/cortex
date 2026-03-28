# Patrones de Uso - Arquitectura Modular

Este documento describe patrones comunes de uso de las interfaces de servicio en Cortex.

## Patrón 1: Creación de Servicios

### Usando Factory Functions (Recomendado)

```go
import (
    "github.com/dacrypt/cortex/backend/internal/infrastructure/filesystem"
)

// Crear indexer
indexer := filesystem.NewFileIndexer(workspaceRoot, config)

// Crear watcher
watcher, err := filesystem.NewFileWatcher(workspaceRoot, config)
if err != nil {
    return err
}

// O crear ambos a la vez
indexer, watcher, err := filesystem.CreateIndexerAndWatcher(workspaceRoot, config)
if err != nil {
    return err
}
```

### Usando Implementaciones Directas

```go
import (
    "github.com/dacrypt/cortex/backend/internal/infrastructure/filesystem"
)

// Crear directamente (también funciona)
scanner := filesystem.NewScanner(workspaceRoot, config)
watcher, _ := filesystem.NewWatcher(workspaceRoot, config)

// Los tipos concretos implementan las interfaces automáticamente
var indexer service.FileIndexer = scanner
var fileWatcher service.FileWatcher = watcher
```

## Patrón 2: Inyección de Dependencias

### En Handlers

```go
type MyHandler struct {
    indexer service.FileIndexer  // Interface, no tipo concreto
    watcher service.FileWatcher
}

func NewMyHandler(indexer service.FileIndexer, watcher service.FileWatcher) *MyHandler {
    return &MyHandler{
        indexer: indexer,
        watcher: watcher,
    }
}

// Uso
indexer := filesystem.NewFileIndexer(path, config)
watcher, _ := filesystem.NewFileWatcher(path, config)
handler := NewMyHandler(indexer, watcher)
```

### En Tests

```go
func TestMyHandler(t *testing.T) {
    // Crear mocks
    mockIndexer := new(MockFileIndexer)
    mockWatcher := new(MockFileWatcher)
    
    // Configurar mocks
    mockIndexer.On("Scan", mock.Anything, mock.Anything).
        Return([]*entity.FileEntry{testEntry}, nil)
    
    // Inyectar mocks
    handler := NewMyHandler(mockIndexer, mockWatcher)
    
    // Test
    entries, err := handler.indexer.Scan(ctx, nil)
    assert.NoError(t, err)
    assert.Len(t, entries, 1)
}
```

## Patrón 3: Escaneo de Archivos

### Escaneo Completo

```go
ctx := context.Background()
progressCh := make(chan service.ScanProgress, 10)

go func() {
    for progress := range progressCh {
        log.Printf("Progress: %d/%d files", progress.FilesScanned, progress.FilesTotal)
    }
}()

entries, err := indexer.Scan(ctx, progressCh)
if err != nil {
    return err
}

log.Printf("Scanned %d files", len(entries))
```

### Escaneo de Archivo Individual

```go
entry, err := indexer.ScanFile(ctx, "src/main.go")
if err != nil {
    return err
}

log.Printf("File: %s, Size: %d", entry.RelativePath, entry.FileSize)
```

### Verificar Existencia

```go
if indexer.Exists("src/main.go") {
    log.Println("File exists")
}
```

### Leer Contenido

```go
// Leer archivo completo
content, err := indexer.ReadFile("src/main.go")
if err != nil {
    return err
}

// Leer primeros bytes (útil para detección de tipo)
header, err := indexer.ReadFileHead("src/main.go", 512)
if err != nil {
    return err
}
```

## Patrón 4: Monitoreo de Cambios

### Iniciar Watcher

```go
watcher, err := filesystem.NewFileWatcher(workspaceRoot, config)
if err != nil {
    return err
}

if err := watcher.Start(); err != nil {
    return err
}
defer watcher.Stop()
```

### Procesar Eventos

```go
for {
    select {
    case evt := <-watcher.Events():
        switch evt.Type {
        case entity.FileEventCreated:
            log.Printf("File created: %s", evt.RelativePath)
            entry, err := indexer.ScanFile(ctx, evt.RelativePath)
            // Procesar archivo nuevo
            
        case entity.FileEventModified:
            log.Printf("File modified: %s", evt.RelativePath)
            entry, err := indexer.ScanFile(ctx, evt.RelativePath)
            // Reprocesar archivo
            
        case entity.FileEventDeleted:
            log.Printf("File deleted: %s", evt.RelativePath)
            // Eliminar de índice
            
        case entity.FileEventRenamed:
            log.Printf("File renamed: %s -> %s", 
                valueOrEmpty(evt.OldPath), evt.RelativePath)
            // Manejar renombrado
        }
        
    case err := <-watcher.Errors():
        log.Printf("Watcher error: %v", err)
    }
}
```

## Patrón 5: Estadísticas del Workspace

```go
stats, err := indexer.CollectStats(ctx)
if err != nil {
    return err
}

log.Printf("Total files: %d", stats.TotalFiles)
log.Printf("Total size: %d bytes", stats.TotalSize)
log.Printf("Extensions: %v", stats.ExtensionCounts)
log.Printf("Size buckets: %v", stats.SizeBuckets)
```

## Patrón 6: Testing con Mocks

### Mock Simple

```go
type MockFileIndexer struct {
    entries []*entity.FileEntry
    err     error
}

func (m *MockFileIndexer) Scan(ctx context.Context, progress chan<- service.ScanProgress) ([]*entity.FileEntry, error) {
    return m.entries, m.err
}

// Implementar otros métodos...
```

### Mock con testify/mock

```go
import "github.com/stretchr/testify/mock"

type MockFileIndexer struct {
    mock.Mock
}

func (m *MockFileIndexer) Scan(ctx context.Context, progress chan<- service.ScanProgress) ([]*entity.FileEntry, error) {
    args := m.Called(ctx, progress)
    return args.Get(0).([]*entity.FileEntry), args.Error(1)
}

// En test
mockIndexer := new(MockFileIndexer)
mockIndexer.On("Scan", mock.Anything, mock.Anything).
    Return([]*entity.FileEntry{testEntry}, nil)
```

## Patrón 7: Cambio de Implementación

### Intercambiar Implementaciones

```go
var indexer service.FileIndexer

// Opción 1: Local filesystem
if useLocal {
    indexer = filesystem.NewFileIndexer(workspaceRoot, config)
}

// Opción 2: Remote (futuro)
if useRemote {
    indexer = remote.NewRemoteIndexer(remoteConfig)
}

// Opción 3: Git-based (futuro)
if useGit {
    indexer = git.NewGitIndexer(gitConfig)
}

// El resto del código no cambia
entries, err := indexer.Scan(ctx, nil)
```

## Patrón 8: Configuración Dinámica

```go
func createIndexer(config *Config) service.FileIndexer {
    switch config.IndexerType {
    case "local":
        return filesystem.NewFileIndexer(config.WorkspaceRoot, config.WorkspaceConfig)
    case "remote":
        return remote.NewRemoteIndexer(config.RemoteConfig)
    default:
        return filesystem.NewFileIndexer(config.WorkspaceRoot, config.WorkspaceConfig)
    }
}
```

## Patrón 9: Múltiples Extractores

```go
extractors := []service.MetadataExtractor{
    basicExtractor,    // Prioridad: 100
    mimeExtractor,     // Prioridad: 90
    codeExtractor,     // Prioridad: 80
}

// Ordenar por prioridad
sort.Slice(extractors, func(i, j int) bool {
    return extractors[i].GetPriority() > extractors[j].GetPriority()
})

// Aplicar extractores
for _, extractor := range extractors {
    if extractor.CanExtract(entry) {
        if err := extractor.Extract(ctx, entry); err != nil {
            log.Warn().Err(err).Msg("Extraction failed")
        }
    }
}
```

## Patrón 10: Manejo de Errores

```go
entry, err := indexer.ScanFile(ctx, relativePath)
if err != nil {
    // Manejar error específico
    if os.IsNotExist(err) {
        log.Warn().Msg("File does not exist")
        return nil
    }
    return err
}

// Verificar que el archivo existe antes de procesar
if !indexer.Exists(relativePath) {
    log.Warn().Msg("File was deleted")
    return nil
}
```

## Mejores Prácticas

1. **Usar interfaces en funciones públicas**: Acepta interfaces, retorna tipos concretos
2. **Factory functions para creación**: Usa `NewFileIndexer()` en lugar de `NewScanner()` directamente
3. **Inyección de dependencias**: Pasa interfaces como parámetros, no tipos concretos
4. **Mocks para testing**: Crea mocks de interfaces, no de implementaciones
5. **Context para cancelación**: Siempre pasa `context.Context` para operaciones asíncronas
6. **Manejo de errores**: Verifica errores y maneja casos específicos
7. **Cerrar recursos**: Usa `defer` para cerrar watchers y otros recursos

## Ejemplos Completos

Ver:
- `backend/internal/interfaces/grpc/handlers/file_handler_test.go` - Tests con mocks
- `backend/cmd/cortexd/watch.go` - Uso en producción
- `backend/internal/infrastructure/filesystem/factory.go` - Factory functions






