# Patrones de Extractores - Guía de Uso

Este documento describe cómo usar los extractores de metadatos en el pipeline de Cortex.

## Concepto

Los **extractores** son componentes que extraen metadatos de archivos. Implementan la interfaz `service.MetadataExtractor` y pueden:

- Extraer metadatos de archivos
- Decidir si pueden procesar un archivo específico
- Tener una prioridad de ejecución

## Tipos de Extractores

### 1. Extractores Basados en Stages

Los stages del pipeline pueden usarse como extractores mediante adaptadores:

```go
import (
    "github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
    "github.com/dacrypt/cortex/backend/internal/domain/service"
)

// Crear extractores desde stages
basicStage := stages.NewBasicStage()
mimeStage := stages.NewMimeStage()
codeStage := stages.NewCodeStage()

// Convertir a extractores con prioridades
extractors := stages.ExtractorsFromStages(
    []stages.Stage{basicStage, mimeStage, codeStage},
    []int{100, 90, 80}, // Prioridades
)
```

### 2. Extractores Dedicados

También hay extractores dedicados que implementan directamente la interfaz:

```go
extractors := stages.CreateDefaultExtractors()
// Retorna: [BasicMetadataExtractor, MimeMetadataExtractor, CodeMetadataExtractor]
```

### 3. Extractores Personalizados

Puedes crear tus propios extractores:

```go
type CustomExtractor struct {
    priority int
}

func (e *CustomExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
    // Tu lógica de extracción
    if entry.Enhanced == nil {
        entry.Enhanced = &entity.EnhancedMetadata{}
    }
    // Modificar entry.Enhanced según sea necesario
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
    stages.NewBasicMetadataExtractor(),
    &CustomExtractor{priority: 75},
}
```

## Uso en el Pipeline

### Opción 1: Usar Extractores en Lugar de Stages

```go
// Crear extractores
extractors := stages.CreateDefaultExtractors()

// Aplicar a un archivo
entry := entity.NewFileEntry(workspaceRoot, "test.go", 100, time.Now())
err := stages.ExtractWithExtractors(ctx, entry, extractors)
if err != nil {
    log.Error().Err(err).Msg("Extraction failed")
}
```

### Opción 2: Usar Extractores Dentro de un Stage

```go
type MetadataExtractionStage struct {
    extractors []service.MetadataExtractor
    logger     zerolog.Logger
}

func NewMetadataExtractionStage(extractors []service.MetadataExtractor, logger zerolog.Logger) *MetadataExtractionStage {
    return &MetadataExtractionStage{
        extractors: extractors,
        logger:     logger,
    }
}

func (s *MetadataExtractionStage) Name() string {
    return "metadata_extraction"
}

func (s *MetadataExtractionStage) Process(ctx context.Context, entry *entity.FileEntry) error {
    return stages.ExtractWithExtractors(ctx, entry, s.extractors)
}
```

### Opción 3: Ordenar Extractores por Prioridad

```go
import "sort"

extractors := []service.MetadataExtractor{
    stages.NewCodeMetadataExtractor(),     // Priority: 80
    stages.NewBasicMetadataExtractor(),    // Priority: 100
    stages.NewMimeMetadataExtractor(),     // Priority: 90
}

// Ordenar por prioridad (mayor primero)
sort.Slice(extractors, func(i, j int) bool {
    return extractors[i].GetPriority() > extractors[j].GetPriority()
})

// Ahora están ordenados: Basic (100), Mime (90), Code (80)
```

## Prioridades Recomendadas

| Extractores | Prioridad | Razón |
|------------|-----------|-------|
| Basic | 100 | Información básica necesaria para todo |
| MIME | 90 | Necesario para determinar tipo de archivo |
| Code | 80 | Análisis específico de código |
| OS Metadata | 70 | Metadatos del sistema operativo |
| Document | 60 | Extracción de contenido de documentos |
| AI/LLM | 50 | Procesamiento con AI (más lento) |

## Ejemplo Completo

```go
package main

import (
    "context"
    "github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
    "github.com/dacrypt/cortex/backend/internal/domain/entity"
    "github.com/dacrypt/cortex/backend/internal/domain/service"
)

func processFile(ctx context.Context, entry *entity.FileEntry) error {
    // Crear extractores
    extractors := []service.MetadataExtractor{
        stages.NewBasicMetadataExtractor(),    // 100
        stages.NewMimeMetadataExtractor(),     // 90
        stages.NewCodeMetadataExtractor(),     // 80
    }
    
    // Aplicar extractores
    for _, extractor := range extractors {
        if !extractor.CanExtract(entry) {
            continue // Saltar si no puede procesar
        }
        
        if err := extractor.Extract(ctx, entry); err != nil {
            // Log error pero continuar con otros extractores
            log.Warn().Err(err).
                Str("extractor", extractor.GetPriority()).
                Str("file", entry.RelativePath).
                Msg("Extraction failed")
            continue
        }
    }
    
    return nil
}
```

## Ventajas de Usar Extractores

### 1. Flexibilidad

Puedes combinar extractores de diferentes fuentes:

```go
extractors := []service.MetadataExtractor{
    stages.NewBasicMetadataExtractor(),
    customExtractor,
    pluginExtractor,
}
```

### 2. Testabilidad

Fácil crear mocks para testing:

```go
type MockExtractor struct {
    mock.Mock
}

func (m *MockExtractor) Extract(ctx context.Context, entry *entity.FileEntry) error {
    args := m.Called(ctx, entry)
    return args.Error(0)
}

func (m *MockExtractor) CanExtract(entry *entity.FileEntry) bool {
    args := m.Called(entry)
    return args.Bool(0)
}

func (m *MockExtractor) GetPriority() int {
    return 100
}
```

### 3. Composición

Puedes crear pipelines de extracción complejos:

```go
// Extractores básicos
basicExtractors := stages.CreateDefaultExtractors()

// Extractores avanzados
advancedExtractors := []service.MetadataExtractor{
    documentExtractor,
    aiExtractor,
}

// Combinar
allExtractors := append(basicExtractors, advancedExtractors...)
```

### 4. Condicionalidad

Los extractores pueden decidir si procesar un archivo:

```go
for _, extractor := range extractors {
    if !extractor.CanExtract(entry) {
        continue // Saltar si no puede procesar
    }
    // Procesar...
}
```

## Migración de Stages a Extractores

### Antes (Usando Stages)

```go
orchestrator := pipeline.NewOrchestrator(publisher, logger)
orchestrator.AddStage(stages.NewBasicStage())
orchestrator.AddStage(stages.NewMimeStage())
orchestrator.AddStage(stages.NewCodeStage())

err := orchestrator.Process(ctx, entry)
```

### Después (Usando Extractores)

```go
extractors := stages.CreateDefaultExtractors()
err := stages.ExtractWithExtractors(ctx, entry, extractors)
```

### Híbrido (Stages + Extractores)

```go
// Usar extractores dentro de un stage
extractorStage := NewMetadataExtractionStage(
    stages.CreateDefaultExtractors(),
    logger,
)
orchestrator.AddStage(extractorStage)
```

## Mejores Prácticas

1. **Ordenar por prioridad**: Siempre ordena extractores por prioridad antes de procesar
2. **Manejo de errores**: No fallar todo el pipeline si un extractor falla
3. **CanExtract primero**: Siempre verifica `CanExtract()` antes de llamar `Extract()`
4. **Prioridades consistentes**: Usa un rango consistente de prioridades (0-100)
5. **Documentar prioridades**: Documenta por qué cada extractor tiene su prioridad

## Conclusión

Los extractores proporcionan una forma flexible y testeable de extraer metadatos. Pueden usarse:

- Como reemplazo de stages en algunos casos
- Dentro de stages para modularidad
- En pipelines personalizados
- Para testing con mocks

La arquitectura modular permite mezclar y combinar extractores según las necesidades del proyecto.






