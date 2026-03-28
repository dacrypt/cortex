# ¿Qué son Facetas en el Árbol de Cortex?

## Definición de Faceta

Una **faceta** en Cortex es una **agrupación de archivos por un atributo específico** que permite filtrar y navegar el workspace de manera semántica.

**Conceptualmente, TODAS las vistas en el árbol de Cortex son facetas** - todas agrupan archivos por algún atributo o criterio. La diferencia es principalmente técnica (qué provider usan), no conceptual.

### Tipos de Facetas por Implementación

Las facetas se implementan con diferentes providers según su naturaleza:

- `TermsFacetTreeProvider` - Para facetas de términos (valores discretos)
- `NumericRangeFacetTreeProvider` - Para facetas numéricas (rangos)
- `DateRangeFacetTreeProvider` - Para facetas de fecha (rangos temporales)
- `FolderTreeProvider` - Para faceta de estructura de carpetas
- `CategoryTreeProviders` - Para facetas de categorías jerárquicas
- `CodeMetricsTreeProvider` - Para faceta de métricas de código
- `DocumentMetricsTreeProvider` - Para faceta de métricas de documentos
- `IssuesTreeProvider` - Para faceta de issues y problemas
- `MetadataClassificationTreeProvider` - Para faceta de tipos de metadatos

## Facetas del Sistema

### Facetas de Términos (TermsFacetTreeProvider)

**Atributo agrupado**: Valores discretos de un campo específico

Estas facetas agrupan archivos por valores discretos:

1. **`extension`** - Por extensión de archivo (`.ts`, `.go`, `.pdf`, etc.)
2. **`type`** - Por tipo de archivo inferido
3. **`tag`** - Por etiquetas asignadas por el usuario
4. **`content_type`** - Por tipo de contenido (MIME category)
5. **`language`** - Por idioma detectado (es, en, fr, etc.)
6. **`category`** - Por categoría AI asignada
7. **`author`** - Por autor extraído
8. **`publication_year`** - Por año de publicación
9. **`owner`** - Por propietario del archivo (OS)
10. **`indexing_status`** - Por estado de indexación
11. **`sentiment`** - Por sentimiento (positive, negative, neutral, mixed)
12. **`duplicate_type`** - Por tipo de duplicado (exact, near, version)
13. **`location`** - Por ubicación geográfica extraída
14. **`organization`** - Por organización mencionada
15. **`purpose`** - Por propósito sugerido por AI
16. **`audience`** - Por audiencia sugerida por AI
17. **`domain`** - Por dominio de taxonomía AI
18. **`subdomain`** - Por subdominio de taxonomía AI
19. **`topic`** - Por tema sugerido por AI
20. **`temporal_pattern`** - Por patrón temporal (recent, occasional, rare, never)
21. **`readability_level`** - Por nivel de legibilidad
22. **`image_format`** - Por formato de imagen (JPEG, PNG, etc.)
23. **`camera_make`** - Por marca de cámara (EXIF)
24. **`audio_genre`** - Por género musical (ID3 tags)
25. **`audio_artist`** - Por artista (ID3 tags)
26. **`video_resolution`** - Por resolución de video
27. **`video_codec`** - Por codec de video
28. **`permission`** - Por permisos de archivo

### Facetas Numéricas (NumericRangeFacetTreeProvider)

**Atributo agrupado**: Rangos numéricos de un campo específico

1. **`size`** - Por tamaño de archivo (Tiny, Small, Medium, Large, Very Large)
2. **`complexity`** - Por complejidad de código (Very Low, Low, Medium, High, Very High)
3. **`project_score`** - Por score de asignación de proyecto (0.0 - 1.0)
4. **`function_count`** - Por número de funciones (None, Small, Medium, Large, Very Large)
5. **`lines_of_code`** - Por líneas de código (Tiny, Small, Medium, Large, Very Large)
6. **`comment_percentage`** - Por porcentaje de comentarios (None, Low, Medium, High, Very High)
7. **`content_quality`** - Por calidad de contenido (score ranges)
8. **`image_dimensions`** - Por dimensiones de imagen (width × height ranges)
9. **`image_color_depth`** - Por profundidad de color (bits per pixel)
10. **`image_iso`** - Por ISO de cámara
11. **`image_aperture`** - Por apertura (f-stop ranges)
12. **`image_focal_length`** - Por distancia focal
13. **`audio_duration`** - Por duración de audio
14. **`audio_bitrate`** - Por bitrate de audio
15. **`audio_sample_rate`** - Por sample rate de audio
16. **`video_duration`** - Por duración de video
17. **`video_bitrate`** - Por bitrate de video
18. **`video_frame_rate`** - Por frame rate de video
19. **`language_confidence`** - Por confianza de detección de idioma

### Facetas de Fecha (DateRangeFacetTreeProvider)

**Atributo agrupado**: Rangos temporales de un campo de fecha específico

1. **`modified`** - Por fecha de modificación (Today, This Week, This Month, etc.)
2. **`created`** - Por fecha de creación
3. **`accessed`** - Por fecha de último acceso
4. **`changed`** - Por fecha de cambio de metadatos (ctime)

### Facetas de Estructura (FolderTreeProvider)

**Atributo agrupado**: Ruta de carpeta (path components)

1. **`folder`** - "Carpetas"
   - **Qué es**: Agrupa archivos por estructura física de carpetas del workspace
   - **Atributo**: `relative_path` (componentes de ruta)
   - **Ubicación en árbol**: `Navegar > Carpetas`

### Facetas de Categorías Jerárquicas (CategoryTreeProviders)

**Atributo agrupado**: Categoría de proyecto (project nature/category)

1. **`writing_category`** - "Escritura"
   - **Atributo**: Proyectos con naturaleza "writing" o categoría relacionada
   - **Ubicación en árbol**: `Organizar > Escritura`

2. **`collection_category`** - "Colecciones"
   - **Atributo**: Proyectos con naturaleza "collection" o categoría relacionada
   - **Ubicación en árbol**: `Organizar > Colecciones`

3. **`development_category`** - "Desarrollo"
   - **Atributo**: Proyectos con naturaleza "development" o categoría relacionada
   - **Ubicación en árbol**: `Organizar > Desarrollo`

4. **`management_category`** - "Gestión"
   - **Atributo**: Proyectos con naturaleza "management" o categoría relacionada
   - **Ubicación en árbol**: `Organizar > Gestión`

5. **`hierarchical_category`** - "Jerarquía"
   - **Atributo**: Proyectos con estructura jerárquica
   - **Ubicación en árbol**: `Organizar > Jerarquía`

### Facetas de Métricas (CodeMetricsTreeProvider, DocumentMetricsTreeProvider)

**Atributo agrupado**: Rangos de métricas específicas

1. **`code_metrics`** - "Código"
   - **Atributo**: Métricas de código (LOC, funciones, complejidad, etc.) - agrupa por archivo con métricas similares
   - **Ubicación en árbol**: `Analizar > Código`

2. **`document_metrics`** - "Documentos"
   - **Atributo**: Métricas de documentos (páginas, palabras, caracteres, etc.) - agrupa por archivo con métricas similares
   - **Ubicación en árbol**: `Analizar > Documentos`

### Facetas de Issues (IssuesTreeProvider)

**Atributo agrupado**: Tipo de issue o presencia de issues

1. **`issue_type`** - "Issues y TODOs"
   - **Atributo**: Tipo de issue (TODO, FIXME, BUG, etc.) o archivos con/sin issues
   - **Ubicación en árbol**: `Revisar > Issues y TODOs`

### Facetas de Metadatos (MetadataClassificationTreeProvider)

**Atributo agrupado**: Tipo de metadato disponible

1. **`metadata_type`** - "Metadatos"
   - **Atributo**: Tipo de metadato (author, title, creator, producer, genre, year, etc.)
   - **Ubicación en árbol**: `Buscar > Metadatos`

### Vista de Detalles (FileInfoTreeProvider)

**Nota**: Esta vista NO es una faceta porque muestra información de UN archivo específico, no agrupa múltiples archivos.

1. **`file_info`** - Panel de Información
   - **Qué es**: Muestra información detallada del archivo actualmente seleccionado
   - **Por qué no es faceta**: Es un panel de detalles, no una vista de agrupación
   - **Ubicación**: Vista separada (`cortex-fileInfoView`)

## Resumen Visual del Árbol

**Todas las vistas son facetas** (excepto FileInfo que es un panel de detalles):

```
Cortex
├── Navegar (TODAS son facetas)
│   ├── Carpetas (✅ faceta: folder - estructura física)
│   ├── Extensiones (✅ faceta: extension)
│   └── Tipo Contenido (✅ faceta: content_type)
│
├── Organizar (TODAS son facetas)
│   ├── Etiquetas (✅ faceta: tag)
│   ├── Escritura (✅ faceta: writing_category)
│   ├── Colecciones (✅ faceta: collection_category)
│   ├── Desarrollo (✅ faceta: development_category)
│   ├── Gestión (✅ faceta: management_category)
│   └── Jerarquía (✅ faceta: hierarchical_category)
│
├── Buscar (TODAS son facetas)
│   ├── Fecha (✅ faceta: modified)
│   ├── Tamaño (✅ faceta: size)
│   ├── Autor (✅ faceta: author)
│   ├── Idioma (✅ faceta: language)
│   ├── Categoría AI (✅ faceta: category)
│   ├── Dominio (✅ faceta: domain)
│   ├── Tema (✅ faceta: topic)
│   ├── Propietario (✅ faceta: owner)
│   └── Metadatos (✅ faceta: metadata_type)
│
├── Analizar (TODAS son facetas)
│   ├── Código (✅ faceta: code_metrics)
│   ├── Complejidad (✅ faceta: complexity)
│   ├── Líneas (✅ faceta: lines_of_code)
│   ├── Documentos (✅ faceta: document_metrics)
│   ├── Legibilidad (✅ faceta: readability_level)
│   ├── Calidad (✅ faceta: content_quality)
│   ├── Imágenes (✅ faceta: image_format)
│   ├── Audio (✅ faceta: audio_genre)
│   └── Video (✅ faceta: video_resolution)
│
└── Revisar (TODAS son facetas)
    ├── Issues y TODOs (✅ faceta: issue_type)
    └── Duplicados (✅ faceta: duplicate_type)
```

## Criterios para Identificar Facetas

**Conceptualmente, TODAS las vistas que agrupan archivos son facetas.**

Una vista es una **faceta** si:

1. ✅ Agrupa archivos por **un atributo o criterio específico** (ej: idioma, autor, tamaño, carpeta, categoría, métrica)
2. ✅ Permite navegar y filtrar archivos por ese atributo
3. ✅ Muestra múltiples archivos organizados por el atributo

**Diferencia técnica (no conceptual):**

- **Facetas estándar**: Usan `TermsFacetTreeProvider`, `NumericRangeFacetTreeProvider`, o `DateRangeFacetTreeProvider` y están registradas en el `FacetRegistry` del backend
- **Facetas especializadas**: Usan providers personalizados (`FolderTreeProvider`, `CategoryTreeProviders`, etc.) pero conceptualmente son facetas porque agrupan por un atributo

**NO es una faceta:**

1. ❌ Vista de detalles de un archivo específico (ej: `FileInfoTreeProvider` - muestra info de UN archivo, no agrupa múltiples)

## Estado Actual

- **Facetas estándar en backend**: ~32 facetas registradas en `FacetRegistry`
- **Facetas especializadas**: ~8-10 facetas con providers personalizados
- **Total de facetas conceptuales**: ~40-42 facetas
- **Vista de detalles (NO faceta)**: 1 (`FileInfoTreeProvider`)

### Desglose por Tipo

- **Facetas de términos**: ~28 facetas
- **Facetas numéricas**: ~19 facetas
- **Facetas de fecha**: 4 facetas
- **Facetas de estructura**: 1 faceta (folder)
- **Facetas de categorías**: 5 facetas (writing, collection, development, management, hierarchical)
- **Facetas de métricas**: 2 facetas (code_metrics, document_metrics)
- **Facetas de issues**: 1 faceta (issue_type)
- **Facetas de metadatos**: 1 faceta (metadata_type)

## Referencias

- [FACETS_ANALYSIS.md](./FACETS_ANALYSIS.md) - Análisis completo de todas las facetas disponibles
- [FRONTEND_BACKEND_FACETS_GAP.md](./FRONTEND_BACKEND_FACETS_GAP.md) - Gap entre backend y frontend
- [VIEWS_CONTRACT.md](./VIEWS_CONTRACT.md) - Contrato de vistas y estructura estándar

