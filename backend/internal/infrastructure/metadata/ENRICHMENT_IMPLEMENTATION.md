# Implementación de Técnicas de Enriquecimiento

## 📋 Resumen

Se han implementado todas las técnicas de enriquecimiento de indexación propuestas. El sistema ahora puede extraer información adicional de documentos usando múltiples técnicas complementarias.

## 🏗️ Arquitectura

### Entidad de Dominio: `EnrichmentData`

Todas las técnicas de enriquecimiento almacenan sus resultados en `entity.EnrichmentData`, que se guarda en `FileMetadata.EnrichmentData`:

```go
type EnrichmentData struct {
    NamedEntities []NamedEntity
    Citations []Citation
    Sentiment *SentimentAnalysis
    Tables []Table
    Formulas []Formula
    Dependencies []Dependency
    Duplicates []DuplicateInfo
    OCRText *OCRResult
    Transcription *TranscriptionResult
    ExtractedAt time.Time
    Source string
}
```

### Stage: `EnrichmentStage`

El `EnrichmentStage` orquesta todas las técnicas de enriquecimiento:

- **Ubicación**: `backend/internal/application/pipeline/stages/enrichment.go`
- **Configuración**: `EnrichmentConfig` permite habilitar/deshabilitar cada técnica
- **Integración**: Se ejecuta después de `AIStage` y `DocumentStage`

## 🔧 Servicios Implementados

### 1. NER (Named Entity Recognition)
- **Archivo**: `backend/internal/infrastructure/llm/ner.go`
- **Método**: `ExtractNamedEntities(ctx, content, summary)`
- **Tecnología**: LLM con prompt especializado
- **Extrae**: Personas, lugares, organizaciones, fechas, cantidades, porcentajes, eventos

### 2. Extracción de Citas
- **Archivo**: `backend/internal/infrastructure/llm/citations.go`
- **Método**: `ExtractCitations(ctx, content, summary)`
- **Tecnología**: LLM con prompt especializado
- **Extrae**: Autores, títulos, años, DOI, URLs, tipo de referencia

### 3. Análisis de Sentimiento
- **Archivo**: `backend/internal/infrastructure/llm/sentiment.go`
- **Método**: `AnalyzeSentiment(ctx, content, summary)`
- **Tecnología**: LLM con prompt especializado
- **Extrae**: Sentimiento general, score, emociones (alegría, tristeza, ira, etc.)

### 4. OCR (Optical Character Recognition)
- **Archivo**: `backend/internal/infrastructure/metadata/ocr_service.go`
- **Método**: `ExtractTextFromImage`, `ExtractTextFromPDF`
- **Tecnología**: Tesseract OCR
- **Requisitos**: Tesseract instalado (`tesseract` command)
- **Extrae**: Texto de imágenes y PDFs escaneados

### 5. Extracción de Tablas
- **Archivo**: `backend/internal/infrastructure/metadata/table_extractor.go`
- **Método**: `ExtractTables`, `ExtractTablesFromMarkdown`
- **Tecnología**: Tabula/Camelot (Python) o parser Markdown nativo
- **Requisitos**: Python3 + tabula-py o camelot-py (opcional)
- **Extrae**: Tablas con headers, filas, columnas

### 6. Extracción de Fórmulas
- **Archivo**: `backend/internal/infrastructure/metadata/formula_extractor.go`
- **Método**: `ExtractFormulas(ctx, content)`
- **Tecnología**: Regex patterns para LaTeX, MathML, expresiones simples
- **Extrae**: Fórmulas matemáticas en varios formatos

### 7. Análisis de Dependencias
- **Archivo**: `backend/internal/infrastructure/metadata/dependency_extractor.go`
- **Método**: `ExtractDependencies(ctx, filePath, content)`
- **Tecnología**: Regex patterns por lenguaje
- **Soporta**: Go, Python, JavaScript/TypeScript, Java, Ruby, Rust
- **Extrae**: Imports, requires, dependencias de código

### 8. Transcripción Audio/Video
- **Archivo**: `backend/internal/infrastructure/metadata/transcription_service.go`
- **Método**: `TranscribeAudio`, `TranscribeVideo`
- **Tecnología**: Whisper (OpenAI)
- **Requisitos**: Whisper instalado (`whisper` o `whisper-ctranslate2`), ffmpeg para video
- **Extrae**: Texto transcrito con timestamps

### 9. Detección de Duplicados
- **Archivo**: `backend/internal/infrastructure/metadata/duplicate_detector.go`
- **Método**: `FindDuplicates(ctx, workspaceID, docID, relativePath)`
- **Tecnología**: Vector similarity search usando embeddings
- **Extrae**: Documentos similares con scores de similitud

### 10. Enriquecimiento con ISBN
- **Archivo**: `backend/internal/infrastructure/metadata/enrichment.go`
- **Método**: `EnrichAIContext(ctx, aiContext)`
- **Tecnología**: APIs externas (Open Library, Google Books)
- **Extrae**: Metadatos completos de libros (título, autores, editorial, portada, etc.)

## 📊 Configuración

### EnrichmentConfig

```go
type EnrichmentConfig struct {
    Enabled              bool
    NEREnabled           bool
    CitationsEnabled     bool
    SentimentEnabled      bool
    OCREnabled           bool
    TablesEnabled        bool
    FormulasEnabled       bool
    DependenciesEnabled   bool
    TranscriptionEnabled  bool
    DuplicateDetectionEnabled bool
    ISBNEnrichmentEnabled bool
}
```

### Ejemplo de Uso

```go
config := EnrichmentConfig{
    Enabled: true,
    NEREnabled: true,
    CitationsEnabled: true,
    SentimentEnabled: true,
    OCREnabled: true,
    TablesEnabled: true,
    FormulasEnabled: true,
    DependenciesEnabled: true,
    TranscriptionEnabled: true,
    DuplicateDetectionEnabled: true,
    ISBNEnrichmentEnabled: true,
}

enrichmentStage := stages.NewEnrichmentStage(
    llmRouter,
    metaRepo,
    docRepo,
    vectorStore,
    embedder,
    logger,
    config,
)
```

## 🔄 Flujo de Procesamiento

1. **EnrichmentStage** recibe un `FileEntry`
2. Obtiene `FileMetadata` y `Document` del repositorio
3. Construye contenido del documento desde chunks
4. Ejecuta cada técnica habilitada:
   - NER (si `NEREnabled`)
   - Citas (si `CitationsEnabled`)
   - Sentimiento (si `SentimentEnabled`)
   - OCR (si `OCREnabled` y archivo es imagen/PDF escaneado)
   - Tablas (si `TablesEnabled`)
   - Fórmulas (si `FormulasEnabled`)
   - Dependencias (si `DependenciesEnabled` y archivo es código)
   - Transcripción (si `TranscriptionEnabled` y archivo es audio/video)
   - Duplicados (si `DuplicateDetectionEnabled`)
   - ISBN (si `ISBNEnrichmentEnabled` y AIContext tiene ISBN)
5. Almacena resultados en `FileMetadata.EnrichmentData`

## 🎯 Integración en el Pipeline

El `EnrichmentStage` debe agregarse al pipeline después de `AIStage` y `DocumentStage`:

```go
orchestrator.AddStage(enrichmentStage)
```

## 📝 Notas de Implementación

### Dependencias Externas

Algunas técnicas requieren herramientas externas:

- **OCR**: Tesseract (`tesseract` command)
- **Tablas**: Python3 + tabula-py o camelot-py (opcional)
- **Transcripción**: Whisper (`whisper` command)
- **Video**: ffmpeg (para extraer audio de video)

Si una herramienta no está disponible, la técnica correspondiente se omite silenciosamente.

### Performance

- **Técnicas LLM** (NER, Citas, Sentimiento): Dependen de la latencia del LLM
- **OCR**: Puede ser lento para documentos grandes
- **Transcripción**: Muy lento para archivos largos
- **Detección de Duplicados**: Requiere búsqueda vectorial (puede ser costosa)

### Fallbacks

- **Tablas**: Si Tabula/Camelot no están disponibles, se usa parser Markdown nativo
- **Citas**: Si el parsing JSON falla, se usa fallback con regex
- **OCR**: Si Tesseract no está disponible, se omite silenciosamente

## 🚀 Próximos Pasos

1. **Integrar en el pipeline principal**: Agregar `EnrichmentStage` al orchestrator
2. **Persistencia**: Asegurar que `EnrichmentData` se guarde correctamente en la base de datos
3. **UI**: Mostrar datos de enriquecimiento en la interfaz
4. **Testing**: Agregar tests para cada técnica
5. **Optimización**: Cachear resultados de técnicas costosas

## 📚 Referencias

- [Tesseract OCR](https://github.com/tesseract-ocr/tesseract)
- [Whisper](https://github.com/openai/whisper)
- [Tabula](https://github.com/tabulapdf/tabula-py)
- [Camelot](https://github.com/camelot-dev/camelot)






