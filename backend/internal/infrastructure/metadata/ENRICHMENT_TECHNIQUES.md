# Técnicas de Enriquecimiento de Indexación

## 📋 Resumen

Este documento describe técnicas adicionales de enriquecimiento de indexación que pueden mejorar la calidad y utilidad del índice de documentos.

## 🎯 Técnicas Implementadas

### ✅ Ya Implementadas
1. **Extracción Forense**: Fuentes, imágenes, enlaces, outline/bookmarks
2. **Extracción Contextual con LLM**: Autores, fechas, lugares, personas, organizaciones
3. **Enriquecimiento con ISBN**: Consulta APIs externas (Open Library, Google Books)
4. **NER (Named Entity Recognition)**: Extracción de entidades nombradas usando LLM
5. **Extracción de Citas**: Identificación y extracción de citas bibliográficas
6. **Análisis de Sentimiento**: Determinación de sentimiento y emociones
7. **OCR**: Extracción de texto de imágenes y PDFs escaneados (Tesseract)
8. **Extracción de Tablas**: Identificación y extracción de tablas de documentos
9. **Extracción de Fórmulas**: Detección de fórmulas matemáticas (LaTeX, MathML)
10. **Análisis de Dependencias**: Extracción de dependencias de código fuente
11. **Transcripción Audio/Video**: Transcripción usando Whisper
12. **Detección de Duplicados**: Identificación de documentos duplicados usando embeddings

---

## 🚀 Técnicas Adicionales Recomendadas

### 🔴 Alta Prioridad (Mayor Impacto)

#### 1. OCR (Optical Character Recognition)
**Descripción**: Extraer texto de imágenes y PDFs escaneados

**Herramientas**:
- Tesseract OCR (gratis, open source)
- Google Cloud Vision API (pago, alta precisión)
- AWS Textract (pago, excelente para documentos)

**Beneficios**:
- Indexar PDFs escaneados que actualmente no tienen texto extraíble
- Extraer texto de imágenes embebidas en documentos
- Mejorar búsqueda en documentos históricos escaneados

**Implementación**:
```go
type OCRService struct {
    tesseractPath string
    logger        zerolog.Logger
}

func (s *OCRService) ExtractTextFromImage(ctx context.Context, imagePath string) (string, error)
func (s *OCRService) ExtractTextFromPDF(ctx context.Context, pdfPath string) (string, error)
```

**Prioridad**: 🔴 Alta - Permite indexar documentos que actualmente no son indexables

---

#### 2. Named Entity Recognition (NER)
**Descripción**: Identificar y clasificar entidades nombradas (personas, lugares, organizaciones, fechas)

**Herramientas**:
- spaCy (Python, pero puede usarse via API)
- Stanford NER
- LLM con prompt especializado (ya tenemos LLM)

**Beneficios**:
- Extraer automáticamente personas, lugares, fechas mencionadas
- Crear índices de entidades para búsqueda rápida
- Construir grafos de relaciones entre entidades

**Implementación**:
```go
type NERService struct {
    llmRouter *llm.Router
    logger    zerolog.Logger
}

func (s *NERService) ExtractEntities(ctx context.Context, content string) ([]Entity, error)

type Entity struct {
    Text      string
    Type      string // "PERSON", "LOCATION", "ORGANIZATION", "DATE", "MONEY", etc.
    StartPos  int
    EndPos    int
    Confidence float64
}
```

**Prioridad**: 🔴 Alta - Complementa perfectamente la extracción contextual

---

#### 3. Extracción de Tablas y Estructuras
**Descripción**: Identificar y extraer tablas, listas, estructuras jerárquicas

**Herramientas**:
- Tabula (PDF tables)
- Camelot (PDF tables)
- LLM para estructuras complejas

**Beneficios**:
- Indexar datos estructurados en documentos
- Permitir búsqueda en tablas
- Extraer relaciones de datos tabulares

**Implementación**:
```go
type TableExtractor struct {
    logger zerolog.Logger
}

func (e *TableExtractor) ExtractTables(ctx context.Context, docPath string) ([]Table, error)

type Table struct {
    Rows    [][]string
    Headers []string
    Caption string
    Page    int
}
```

**Prioridad**: 🔴 Alta - Muchos documentos contienen información valiosa en tablas

---

#### 4. Análisis de Citas y Referencias
**Descripción**: Extraer citas bibliográficas y construir red de referencias

**Herramientas**:
- Anystyle (Ruby, pero puede usarse via API)
- Citation.js
- LLM con prompt especializado

**Beneficios**:
- Construir grafo de citas entre documentos
- Encontrar documentos relacionados por citas
- Analizar impacto y relevancia de documentos

**Implementación**:
```go
type CitationExtractor struct {
    llmRouter *llm.Router
    logger    zerolog.Logger
}

func (e *CitationExtractor) ExtractCitations(ctx context.Context, content string) ([]Citation, error)

type Citation struct {
    Text        string
    Authors     []string
    Title       string
    Year        *int
    DOI         *string
    URL         *string
    Context     string // Texto alrededor de la cita
    Confidence  float64
}
```

**Prioridad**: 🔴 Alta - Permite análisis de relaciones académicas

---

#### 5. Detección de Duplicados y Versiones
**Descripción**: Identificar documentos duplicados o versiones del mismo documento

**Técnicas**:
- Hash de contenido (ya tenemos checksum)
- Similitud semántica con embeddings
- Comparación de estructura

**Beneficios**:
- Evitar indexación duplicada
- Agrupar versiones del mismo documento
- Detectar plagio o copias

**Implementación**:
```go
type DuplicateDetector struct {
    vectorStore repository.VectorStore
    logger      zerolog.Logger
}

func (d *DuplicateDetector) FindDuplicates(ctx context.Context, docID DocumentID, threshold float64) ([]DocumentID, error)
func (d *DuplicateDetector) FindVersions(ctx context.Context, docID DocumentID) ([]DocumentVersion, error)
```

**Prioridad**: 🔴 Alta - Mejora calidad del índice

---

### 🟡 Prioridad Media (Valor Significativo)

#### 6. Análisis de Sentimiento
**Descripción**: Determinar tono y sentimiento del documento

**Herramientas**:
- LLM con prompt especializado
- Análisis de palabras clave emocionales

**Beneficios**:
- Filtrar documentos por tono (positivo, negativo, neutral)
- Analizar opiniones en documentos
- Clasificar documentos por sentimiento

**Prioridad**: 🟡 Media - Útil para ciertos casos de uso

---

#### 7. Extracción de Fórmulas Matemáticas
**Descripción**: Identificar y extraer fórmulas matemáticas en formato LaTeX/MathML

**Herramientas**:
- MathPix API (OCR para fórmulas)
- Detección de patrones LaTeX
- LLM para interpretación

**Beneficios**:
- Indexar documentos técnicos/científicos
- Búsqueda por fórmulas
- Renderizado de fórmulas en UI

**Prioridad**: 🟡 Media - Especializado pero valioso

---

#### 8. Análisis de Dependencias (Código)
**Descripción**: Para archivos de código, extraer dependencias y imports

**Herramientas**:
- Parsers específicos por lenguaje
- Análisis de AST

**Beneficios**:
- Construir grafo de dependencias
- Encontrar archivos relacionados por dependencias
- Analizar arquitectura de código

**Prioridad**: 🟡 Media - Especializado para código

---

#### 9. Extracción de Metadatos de Imágenes (EXIF, GPS)
**Descripción**: Ya tenemos ImageMetadata, pero podemos expandir

**Mejoras**:
- Extracción de GPS para geolocalización
- Análisis de contenido de imagen (objetos, escenas)
- OCR en imágenes embebidas

**Prioridad**: 🟡 Media - Ya parcialmente implementado

---

#### 10. Transcripción de Audio/Video
**Descripción**: Extraer texto de archivos de audio y video

**Herramientas**:
- Whisper (OpenAI, open source)
- Google Speech-to-Text
- ffmpeg + whisper

**Beneficios**:
- Indexar podcasts, videos, grabaciones
- Búsqueda en contenido multimedia
- Subtítulos automáticos

**Prioridad**: 🟡 Media - Requiere procesamiento pesado

---

### 🟢 Prioridad Baja (Nice to Have)

#### 11. Topic Modeling
**Descripción**: Identificar temas principales usando LDA o técnicas similares

**Beneficios**:
- Agrupar documentos por temas
- Descubrir temas emergentes
- Análisis de tendencias

**Prioridad**: 🟢 Baja - Puede ser reemplazado por categorización con LLM

---

#### 12. Clustering de Documentos Similares
**Descripción**: Agrupar documentos similares usando embeddings

**Beneficios**:
- Organización automática
- Descubrimiento de colecciones
- Reducción de ruido

**Prioridad**: 🟢 Baja - Ya tenemos RAG que hace algo similar

---

#### 13. Análisis de Cambios Temporales
**Descripción**: Detectar cambios en documentos a lo largo del tiempo

**Beneficios**:
- Historial de versiones
- Análisis de evolución
- Detección de cambios significativos

**Prioridad**: 🟢 Baja - Requiere seguimiento temporal

---

#### 14. Resolución de Entidades (Entity Linking)
**Descripción**: Vincular entidades extraídas con bases de conocimiento (Wikidata, DBpedia)

**Herramientas**:
- Wikidata API
- DBpedia
- LLM para matching

**Beneficios**:
- Enriquecer entidades con información externa
- Construir conocimiento estructurado
- Mejorar búsqueda semántica

**Prioridad**: 🟢 Baja - Complejo pero poderoso

---

#### 15. Normalización de Nombres
**Descripción**: Normalizar variantes de nombres (ej: "J. Smith" = "John Smith")

**Técnicas**:
- Algoritmos de matching de nombres
- LLM para desambiguación
- Bases de datos de nombres

**Beneficios**:
- Mejor agrupación de autores
- Reducción de duplicados
- Búsqueda más robusta

**Prioridad**: 🟢 Baja - Útil pero complejo

---

## 📊 Matriz de Priorización

| Técnica | Impacto | Esfuerzo | Prioridad | Estado |
|---------|---------|----------|-----------|--------|
| OCR | 🔴 Alto | 🟡 Medio | 🔴 Alta | ⏳ Pendiente |
| NER | 🔴 Alto | 🟢 Bajo | 🔴 Alta | ⏳ Pendiente |
| Tablas | 🔴 Alto | 🟡 Medio | 🔴 Alta | ⏳ Pendiente |
| Citas | 🔴 Alto | 🟡 Medio | 🔴 Alta | ⏳ Pendiente |
| Duplicados | 🔴 Alto | 🟢 Bajo | 🔴 Alta | ⏳ Pendiente |
| Sentimiento | 🟡 Medio | 🟢 Bajo | 🟡 Media | ⏳ Pendiente |
| Fórmulas | 🟡 Medio | 🔴 Alto | 🟡 Media | ⏳ Pendiente |
| Dependencias | 🟡 Medio | 🔴 Alto | 🟡 Media | ⏳ Pendiente |
| Audio/Video | 🟡 Medio | 🔴 Alto | 🟡 Media | ⏳ Pendiente |
| Topic Modeling | 🟢 Bajo | 🟡 Medio | 🟢 Baja | ⏳ Pendiente |

## 🎯 Recomendación de Implementación

### Fase 1 (Inmediato)
1. **NER** - Usar LLM existente, bajo esfuerzo, alto impacto
2. **Detección de Duplicados** - Usar embeddings existentes, bajo esfuerzo
3. **Extracción de Citas** - Usar LLM, medio esfuerzo, alto impacto

### Fase 2 (Corto Plazo)
4. **OCR** - Requiere Tesseract, medio esfuerzo, alto impacto
5. **Extracción de Tablas** - Requiere herramientas especializadas

### Fase 3 (Mediano Plazo)
6. **Análisis de Sentimiento** - Fácil con LLM
7. **Transcripción Audio/Video** - Requiere Whisper

## 💡 Consideraciones Técnicas

### Performance
- Algunas técnicas son costosas computacionalmente (OCR, transcripción)
- Implementar caché y procesamiento asíncrono
- Usar workers para tareas pesadas

### Precisión
- LLM puede tener errores - validar resultados críticos
- Combinar múltiples fuentes cuando sea posible
- Permitir corrección manual

### Escalabilidad
- Procesar en background para técnicas costosas
- Priorizar documentos importantes
- Implementar rate limiting para APIs externas

## 🔗 Integración con Pipeline

Todas estas técnicas pueden integrarse como **stages adicionales** en el pipeline:

```go
// Nuevos stages propuestos
stages.NewOCRStage(...)           // Para imágenes y PDFs escaneados
stages.NewNERStage(...)            // Named Entity Recognition
stages.NewTableExtractionStage(...) // Extracción de tablas
stages.NewCitationStage(...)       // Extracción de citas
stages.NewDuplicateDetectionStage(...) // Detección de duplicados
```

## 📝 Notas Finales

- Priorizar técnicas que usen infraestructura existente (LLM, embeddings)
- Implementar técnicas costosas de forma asíncrona
- Mantener flexibilidad para agregar nuevas técnicas
- Documentar precisión y limitaciones de cada técnica

