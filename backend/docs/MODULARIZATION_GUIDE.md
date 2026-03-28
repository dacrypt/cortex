# Guía de Modularización - Arquitectura con Interfaces

## Resumen

Se ha implementado la **Opción 1: Modularizar y abstraer** para hacer el sistema más mantenible y fácil de testear. Se han creado interfaces claras en el dominio que permiten:

1. **Fácil testing**: Mockear componentes individuales
2. **Intercambiabilidad**: Cambiar implementaciones sin afectar el resto del sistema
3. **Mantenibilidad**: Código más claro y organizado
4. **Extensibilidad**: Agregar nuevas implementaciones fácilmente

## Interfaces Creadas

### 1. `service.FileIndexer`

**Ubicación**: `backend/internal/domain/service/file_indexer.go`

**Propósito**: Abstrae las operaciones de indexación de archivos.

**Métodos**:
- `Scan(ctx, progress) ([]*FileEntry, error)` - Escanea el workspace completo
- `ScanFile(ctx, relativePath) (*FileEntry, error)` - Escanea un archivo individual
- `GetFileInfo(relativePath) (FileInfo, error)` - Obtiene información de un archivo
- `Exists(relativePath) bool` - Verifica si un archivo existe
- `ReadFile(relativePath) ([]byte, error)` - Lee el contenido de un archivo
- `ReadFileHead(relativePath, n) ([]byte, error)` - Lee los primeros n bytes
- `CollectStats(ctx) (*IndexStats, error)` - Recolecta estadísticas del workspace
- `UpdateConfig(config *WorkspaceConfig)` - Actualiza la configuración

**Implementación actual**: `filesystem.Scanner`

**Ejemplo de uso**:
```go
var indexer service.FileIndexer = filesystem.NewScanner(workspaceRoot, config)

// Escanear workspace
entries, err := indexer.Scan(ctx, progressChan)

// Escanear archivo individual
entry, err := indexer.ScanFile(ctx, "src/main.go")
```

### 2. `service.FileWatcher`

**Ubicación**: `backend/internal/domain/service/file_watcher.go`

**Propósito**: Abstrae el monitoreo de cambios en el sistema de archivos.

**Métodos**:
- `Start() error` - Inicia el watcher
- `Stop() error` - Detiene el watcher
- `Events() <-chan WatchEvent` - Canal de eventos
- `Errors() <-chan error` - Canal de errores

**Implementación actual**: `filesystem.Watcher`

**Ejemplo de uso**:
```go
var watcher service.FileWatcher
watcher, err := filesystem.NewWatcher(workspaceRoot, config)
if err != nil {
    return err
}

if err := watcher.Start(); err != nil {
    return err
}
defer watcher.Stop()

for {
    select {
    case evt := <-watcher.Events():
        handleEvent(evt)
    case err := <-watcher.Errors():
        handleError(err)
    }
}
```

### 3. `service.MetadataExtractor`

**Ubicación**: `backend/internal/domain/service/metadata_extractor.go`

**Propósito**: Abstrae la extracción de metadatos de archivos.

**Métodos**:
- `Extract(ctx, entry) error` - Extrae metadatos (modifica entry in-place)
- `CanExtract(entry) bool` - Verifica si puede procesar el archivo
- `GetPriority() int` - Prioridad de ejecución (mayor = antes)

**Ejemplo de uso**:
```go
extractors := []service.MetadataExtractor{
    mimeExtractor,
    osMetadataExtractor,
    codeExtractor,
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

### 4. `service.ContentExtractor`

**Ubicación**: `backend/internal/domain/service/metadata_extractor.go`

**Propósito**: Abstrae la extracción de contenido de texto de archivos (PDF, Office, etc.).

**Métodos**:
- `ExtractContent(ctx, entry) (string, error)` - Extrae texto del archivo
- `CanExtract(entry) bool` - Verifica si puede procesar el archivo
- `GetSupportedMimeTypes() []string` - MIME types soportados
- `GetSupportedExtensions() []string` - Extensiones soportadas

**Ejemplo de uso**:
```go
extractors := []service.ContentExtractor{
    pdfExtractor,
    officeExtractor,
}

for _, extractor := range extractors {
    if extractor.CanExtract(entry) {
        content, err := extractor.ExtractContent(ctx, entry)
        if err != nil {
            continue
        }
        // Usar contenido extraído
    }
}
```

### 5. `service.DocumentClassifier`

**Ubicación**: `backend/internal/domain/service/metadata_extractor.go`

**Propósito**: Abstrae la clasificación de documentos usando AI.

**Métodos**:
- `Classify(ctx, entry, content) (*ClassificationResult, error)` - Clasifica un documento
- `SuggestProjects(ctx, entry, content) ([]string, error)` - Sugiere proyectos
- `SuggestTags(ctx, entry, content) ([]string, error)` - Sugiere tags
- `GenerateSummary(ctx, entry, content) (string, error)` - Genera resumen

**Ejemplo de uso**:
```go
classifier := aiClassifier

// Clasificar documento
result, err := classifier.Classify(ctx, entry, content)
if err != nil {
    return err
}

// Aplicar sugerencias
if len(result.SuggestedProjects) > 0 {
    // Asignar proyectos sugeridos
}
```

## Estado Actual

### ✅ Completado

1. **Interfaces creadas** en `backend/internal/domain/service/`
2. **Scanner implementa FileIndexer** - `filesystem.Scanner` ahora implementa la interfaz
3. **Watcher implementa FileWatcher** - `filesystem.Watcher` ahora implementa la interfaz
4. **Adaptadores creados** - `filesystem/adapter.go` para compatibilidad

### 🔄 Pendiente (Migración Gradual)

El código existente aún usa tipos concretos (`*filesystem.Scanner`, `*filesystem.Watcher`) en varios lugares:

- `backend/cmd/cortexd/watch.go`
- `backend/cmd/cortexd/main.go`
- `backend/internal/interfaces/grpc/handlers/file_handler.go`

**Estrategia de migración**:
1. Los tipos concretos siguen funcionando (compatibilidad hacia atrás)
2. Gradualmente reemplazar usos con interfaces donde tenga sentido
3. Nuevo código debe usar las interfaces directamente

## Beneficios de la Nueva Arquitectura

### 1. Testing

**Antes**:
```go
// Difícil de testear - requiere sistema de archivos real
scanner := filesystem.NewScanner("/real/path", config)
entries, err := scanner.Scan(ctx, nil)
```

**Ahora**:
```go
// Fácil de mockear
type mockIndexer struct {
    entries []*entity.FileEntry
    err     error
}

func (m *mockIndexer) Scan(ctx context.Context, progress chan<- service.ScanProgress) ([]*entity.FileEntry, error) {
    return m.entries, m.err
}

// En tests
var indexer service.FileIndexer = &mockIndexer{
    entries: testEntries,
}
```

### 2. Intercambiabilidad

**Ejemplo**: Cambiar de filesystem local a remoto:

```go
// Antes: Código acoplado a filesystem.Scanner
scanner := filesystem.NewScanner(path, config)

// Ahora: Fácil de cambiar
var indexer service.FileIndexer
if useRemote {
    indexer = remote.NewRemoteIndexer(remoteConfig)
} else {
    indexer = filesystem.NewScanner(path, config)
}

// El resto del código no cambia
entries, err := indexer.Scan(ctx, progress)
```

### 3. Extensibilidad

**Agregar nuevo extractor**:

```go
type CustomExtractor struct {
    priority int
}

func (e *CustomExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
    // Lógica personalizada
    return nil
}

func (e *CustomExtractor) CanExtract(entry *entity.FileEntry) bool {
    return entry.Extension == ".custom"
}

func (e *CustomExtractor) GetPriority() int {
    return e.priority
}

// Usar
extractors := []service.MetadataExtractor{
    basicExtractor,
    mimeExtractor,
    &CustomExtractor{priority: 10}, // Nuevo extractor
}
```

## Próximos Pasos

### Corto Plazo

1. **Migrar handlers gRPC** para usar interfaces
2. **Crear tests unitarios** usando mocks de las interfaces
3. **Documentar** cada interfaz con ejemplos

### Mediano Plazo

1. **Refactorizar pipeline stages** para usar `MetadataExtractor`
2. **Crear implementaciones alternativas** (ej: RemoteIndexer para testing)
3. **Agregar validación** de interfaces en tiempo de compilación

### Largo Plazo

1. **Plugin system** basado en interfaces
2. **Múltiples implementaciones** (local, S3, Git, etc.)
3. **Métricas y observabilidad** por interfaz

## Ejemplos de Uso

### Ejemplo 1: Test Unitario

```go
func TestPipelineWithMockIndexer(t *testing.T) {
    mockIndexer := &MockFileIndexer{
        entries: []*entity.FileEntry{
            entity.NewFileEntry("/root", "test.txt", 100, time.Now()),
        },
    }
    
    orchestrator := pipeline.NewOrchestrator(publisher, logger)
    
    // Usar mock en lugar de filesystem real
    entries, _ := mockIndexer.Scan(ctx, nil)
    for _, entry := range entries {
        err := orchestrator.Process(ctx, entry)
        assert.NoError(t, err)
    }
}
```

### Ejemplo 2: Múltiples Extractores

```go
func setupExtractors() []service.MetadataExtractor {
    return []service.MetadataExtractor{
        stages.NewBasicStage(),      // Prioridad: 100
        stages.NewMimeStage(),       // Prioridad: 90
        stages.NewCodeStage(),       // Prioridad: 80
        stages.NewOSMetadataStage(), // Prioridad: 70
    }
}

func processFile(ctx context.Context, entry *entity.FileEntry, extractors []service.MetadataExtractor) error {
    // Ordenar por prioridad
    sort.Slice(extractors, func(i, j int) bool {
        return extractors[i].GetPriority() > extractors[j].GetPriority()
    })
    
    for _, extractor := range extractors {
        if extractor.CanExtract(entry) {
            if err := extractor.Extract(ctx, entry); err != nil {
                return err
            }
        }
    }
    return nil
}
```

### Ejemplo 3: Configuración Dinámica

```go
func createIndexer(config *Config) service.FileIndexer {
    switch config.IndexerType {
    case "local":
        return filesystem.NewScanner(config.WorkspaceRoot, config.WorkspaceConfig)
    case "remote":
        return remote.NewRemoteIndexer(config.RemoteConfig)
    case "git":
        return git.NewGitIndexer(config.GitConfig)
    default:
        return filesystem.NewScanner(config.WorkspaceRoot, config.WorkspaceConfig)
    }
}
```

## Conclusión

La modularización con interfaces proporciona:

✅ **Mejor testabilidad** - Mocks fáciles de crear
✅ **Mayor flexibilidad** - Intercambiar implementaciones
✅ **Código más limpio** - Separación clara de responsabilidades
✅ **Extensibilidad** - Agregar nuevas funcionalidades sin romper código existente

El sistema mantiene **compatibilidad hacia atrás** mientras permite migración gradual a la nueva arquitectura.






