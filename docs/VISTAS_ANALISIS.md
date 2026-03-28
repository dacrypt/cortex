# Análisis de Vistas de Cortex

Este documento analiza el funcionamiento de cada vista en Cortex y verifica que los datos mostrados coincidan con la información de la base de datos del índice.

## Resumen Ejecutivo

Cortex tiene **19 vistas** diferentes que muestran archivos organizados de distintas maneras. Las vistas utilizan dos fuentes principales de datos:

1. **IMetadataStore** (BackendMetadataStore) - Para tags, contexts, types (almacenamiento local SQLite/JSON)
2. **FileCacheService** - Para datos del backend (archivos, metadatos mejorados, métricas)

## Arquitectura de Datos

### Fuentes de Datos

#### 1. IMetadataStore (BackendMetadataStore)
- **Ubicación**: `src/core/BackendMetadataStore.ts`
- **Datos**: Tags, contexts (proyectos), tipos de archivo, notas, sugerencias
- **Acceso**: A través de `metadataStore` inyectado en constructores
- **Métodos clave**:
  - `getAllTags()` - Obtiene todos los tags
  - `getFilesByTag(tag)` - Archivos con un tag específico
  - `getAllContexts()` - Todos los proyectos/contextos
  - `getFilesByContext(context)` - Archivos en un proyecto
  - `getAllTypes()` - Todos los tipos de archivo
  - `getFilesByType(type)` - Archivos de un tipo específico

#### 2. FileCacheService
- **Ubicación**: `src/core/FileCacheService.ts`
- **Datos**: Archivos del backend con metadatos mejorados
- **Cache TTL**: 30 segundos
- **Métodos clave**:
  - `getFiles()` - Obtiene todos los archivos (con cache)
  - `getCachedFiles()` - Obtiene cache actual (puede estar desactualizado)
  - `refresh()` - Fuerza actualización del cache

#### 3. GrpcKnowledgeClient
- **Ubicación**: `src/core/GrpcKnowledgeClient.ts`
- **Datos**: Proyectos del sistema de conocimiento
- **Métodos clave**:
  - `listProjects(workspaceId)` - Lista todos los proyectos
  - `queryDocuments(workspaceId, projectId, includeSubprojects)` - Archivos en un proyecto

## Análisis por Vista

### 1. ContextTreeProvider (Por Proyecto)

**Archivo**: `src/views/ContextTreeProvider.ts`

**Fuente de Datos**:
- **Proyectos**: `GrpcKnowledgeClient.listProjects()` (backend)
- **Archivos en proyecto**: `GrpcKnowledgeClient.queryDocuments()` (backend)
- **Sugerencias RAG**: `GrpcRAGClient.semanticSearch()` (opcional)

**Funcionamiento**:
1. Obtiene proyectos del backend usando `knowledgeClient.listProjects()`
2. Agrupa proyectos similares usando deduplicación (configurable)
3. Para cada proyecto, obtiene archivos usando `queryDocuments()`
4. Opcionalmente muestra sugerencias RAG basadas en similitud semántica

**Verificación de Datos**:
- ✅ Los proyectos vienen directamente del backend (tabla `projects`)
- ✅ Los archivos vienen de la relación proyecto-documento (tabla `project_documents`)
- ⚠️ **Posible problema**: Si el backend no está disponible, muestra placeholder
- ⚠️ **Cache**: No usa cache, siempre consulta el backend

**Recomendaciones**:
- Agregar logging para verificar que los proyectos y archivos coinciden con la BD
- Manejar mejor el caso cuando el backend no está disponible

---

### 2. TagTreeProvider (Por Tag)

**Archivo**: `src/views/TagTreeProvider.ts`

**Fuente de Datos**:
- **Tags**: `metadataStore.getAllTags()` (IMetadataStore)
- **Archivos con tag**: `metadataStore.getFilesByTag(tag)` (IMetadataStore)
- **Conteos**: `metadataStore.getTagCounts()` (IMetadataStore)
- **Sugerencias RAG**: `GrpcRAGClient.semanticSearch()` (opcional)

**Funcionamiento**:
1. Obtiene todos los tags de `metadataStore`
2. Para cada tag, obtiene archivos usando `getFilesByTag()`
3. Opcionalmente muestra sugerencias RAG basadas en el tag

**Verificación de Datos**:
- ✅ Los tags vienen de `IMetadataStore` (que usa BackendMetadataStore)
- ✅ Los archivos vienen de la relación tag-archivo en la BD
- ⚠️ **Posible problema**: Si `metadataStore` no está sincronizado con el backend, puede mostrar datos desactualizados
- ⚠️ **Cache**: No usa cache explícito, depende de `metadataStore`

**Recomendaciones**:
- Verificar que `BackendMetadataStore` esté sincronizado con el backend
- Agregar método para refrescar tags desde el backend

---

### 3. TypeTreeProvider (Por Tipo)

**Archivo**: `src/views/TypeTreeProvider.ts`

**Fuente de Datos**:
- **Tipos**: `metadataStore.getAllTypes()` (IMetadataStore)
- **Archivos por tipo**: `metadataStore.getFilesByType(type)` (IMetadataStore)

**Funcionamiento**:
1. Obtiene todos los tipos de `metadataStore`
2. Para cada tipo, obtiene archivos usando `getFilesByType()`
3. Muestra conteo de archivos por tipo

**Verificación de Datos**:
- ✅ Los tipos vienen de `IMetadataStore`
- ✅ Los archivos vienen de la BD basándose en la extensión
- ⚠️ **Posible problema**: Si `metadataStore` no está sincronizado, puede mostrar datos incorrectos
- ⚠️ **Cache**: No usa cache explícito

**Recomendaciones**:
- Verificar que los tipos coincidan con las extensiones en la BD del backend
- Considerar usar `FileCacheService` para obtener tipos directamente del backend

---

### 4. DateTreeProvider (Por Fecha)

**Archivo**: `src/views/DateTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Fechas**: `file.last_modified` o `file.enhanced.stats.modified`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Agrupa por categorías de fecha (Last Hour, Today, This Week, etc.)
3. Ordena por fecha de modificación (más reciente primero)

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Las fechas vienen de `last_modified` o `enhanced.stats.modified`
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si el cache está desactualizado, puede mostrar fechas incorrectas

**Recomendaciones**:
- Verificar que las fechas en la vista coincidan con `files.last_modified` en la BD
- Considerar refrescar el cache cuando se expande una categoría

---

### 5. SizeTreeProvider (Por Tamaño)

**Archivo**: `src/views/SizeTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Tamaños**: `file.file_size` o `file.enhanced.stats.size`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Agrupa por categorías de tamaño (Massive, Huge, Very Large, etc.)
3. Ordena por tamaño (más grande primero)
4. Muestra total de tamaño por categoría

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Los tamaños vienen de `file_size` o `enhanced.stats.size`
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si el cache está desactualizado, puede mostrar tamaños incorrectos

**Recomendaciones**:
- Verificar que los tamaños en la vista coincidan con `files.file_size` en la BD
- Considerar refrescar el cache cuando se expande una categoría

---

### 6. FolderTreeProvider (Por Carpeta)

**Archivo**: `src/views/FolderTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Rutas**: `file.relative_path`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Construye jerarquía de carpetas basándose en `relative_path`
3. Muestra archivos y subcarpetas en cada nivel

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Las rutas vienen de `files.relative_path`
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si el cache está desactualizado, puede mostrar estructura incorrecta

**Recomendaciones**:
- Verificar que la estructura de carpetas coincida con las rutas en la BD
- Considerar refrescar el cache cuando se expande una carpeta

---

### 7. ContentTypeTreeProvider (Por Tipo de Contenido)

**Archivo**: `src/views/ContentTypeTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **MIME Types**: `file.enhanced.mime_type.category`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Agrupa por categoría MIME (text, code, image, video, audio, archive, document, binary)
3. Muestra metadatos de documentos (páginas, palabras, autor) si están disponibles

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Los MIME types vienen de `enhanced.mime_type.category` (JSON en BD)
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si `enhanced` no está indexado, puede no mostrar categorías

**Recomendaciones**:
- Verificar que los MIME types coincidan con `enhanced.mime_type` en la BD
- Verificar que los archivos con `indexed_mime = 1` tengan `enhanced.mime_type`

---

### 8. CodeMetricsTreeProvider (Métricas de Código)

**Archivo**: `src/views/CodeMetricsTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Métricas**: `file.enhanced.code_metrics`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Filtra archivos con `enhanced.code_metrics`
3. Agrupa por diferentes métricas (por tamaño, comentarios, complejidad, etc.)
4. Muestra top 10 archivos más grandes, bien comentados, etc.

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Las métricas vienen de `enhanced.code_metrics` (JSON en BD)
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si `indexed_code = 0`, no habrá métricas disponibles

**Recomendaciones**:
- Verificar que los archivos con `indexed_code = 1` tengan `enhanced.code_metrics`
- Verificar que las métricas calculadas coincidan con los datos en la BD

---

### 9. DocumentMetricsTreeProvider (Métricas de Documentos)

**Archivo**: `src/views/DocumentMetricsTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Métricas**: `file.enhanced.document_metrics`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Filtra archivos con `enhanced.document_metrics`
3. Agrupa por tipo de documento (Word, Excel, PowerPoint, PDF)
4. Agrupa por autor si está disponible

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Las métricas vienen de `enhanced.document_metrics` (JSON en BD)
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Si `indexed_document = 0`, no habrá métricas disponibles

**Recomendaciones**:
- Verificar que los archivos con `indexed_document = 1` tengan `enhanced.document_metrics`
- Verificar que las métricas (páginas, palabras, autor) coincidan con los datos en la BD

---

### 10. IssuesTreeProvider (Problemas)

**Archivo**: `src/views/IssuesTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Errores**: `file.enhanced.error`

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Filtra archivos con `enhanced.error`
3. Agrupa por código de error (ENAMETOOLONG, ENOENT, EACCES, etc.)
4. Muestra detalles del error en tooltip

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend (tabla `files`)
- ✅ Los errores vienen de `enhanced.error` (JSON en BD)
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos
- ⚠️ **Posible problema**: Los errores pueden estar desactualizados si el archivo fue corregido

**Recomendaciones**:
- Verificar que los errores en la vista coincidan con `enhanced.error` en la BD
- Considerar refrescar el cache cuando se muestra esta vista

---

### 11. FileInfoTreeProvider (Información del Archivo)

**Archivo**: `src/views/FileInfoTreeProvider.ts`

**Fuente de Datos**:
- **Metadatos**: `metadataStore.getMetadataByPath()` (IMetadataStore)
- **Archivo del backend**: `GrpcMetadataClient.getFile()` (backend)
- **Traces**: `GrpcMetadataClient.getTraces()` (backend)
- **Proyectos**: `GrpcKnowledgeClient.queryDocuments()` (backend)

**Funcionamiento**:
1. Muestra información detallada del archivo actualmente abierto
2. Combina datos de `metadataStore` y del backend
3. Muestra metadatos técnicos, tags, proyectos, notas, resúmenes AI, traces LLM

**Verificación de Datos**:
- ✅ Combina datos de múltiples fuentes
- ✅ Los metadatos vienen de `IMetadataStore` y del backend
- ⚠️ **Posible problema**: Puede haber inconsistencias entre `metadataStore` y el backend
- ⚠️ **Cache**: No usa cache explícito, consulta cada vez

**Recomendaciones**:
- Verificar que los datos de `metadataStore` y del backend coincidan
- Considerar usar una única fuente de verdad

---

### 12. CategoryTreeProvider (Por Categoría/Biblioteca)

**Archivo**: `src/views/CategoryTreeProvider.ts`

**Fuente de Datos**:
- **Archivos**: `FileCacheService.getFiles()` (backend)
- **Categorías**: Basadas en metadatos del archivo (probablemente `enhanced`)

**Funcionamiento**:
1. Obtiene todos los archivos de `FileCacheService`
2. Agrupa por categorías (biblioteca)
3. Muestra archivos en cada categoría

**Verificación de Datos**:
- ✅ Los archivos vienen directamente del backend
- ⚠️ **Posible problema**: No está claro cómo se determinan las categorías
- ✅ **Cache**: Usa `FileCacheService` con TTL de 30 segundos

**Recomendaciones**:
- Revisar cómo se determinan las categorías
- Verificar que las categorías coincidan con los datos en la BD

---

### 13-19. Otras Vistas de Categorías

Las siguientes vistas parecen ser variaciones de categorías:
- `ProjectTaxonomyTreeProvider` - Taxonomía de proyectos
- `MetadataClassificationTreeProvider` - Clasificación de metadatos
- `WritingCategoryTreeProvider` - Categorías de escritura
- `CollectionCategoryTreeProvider` - Categorías de colección
- `DevelopmentCategoryTreeProvider` - Categorías de desarrollo
- `ManagementCategoryTreeProvider` - Categorías de gestión
- `HierarchicalCategoryTreeProvider` - Categorías jerárquicas

**Nota**: Estas vistas no fueron analizadas en detalle, pero probablemente siguen el mismo patrón que `CategoryTreeProvider`.

## Problemas Identificados

### 1. Inconsistencia entre IMetadataStore y Backend

**Problema**: Algunas vistas usan `IMetadataStore` (BackendMetadataStore) mientras que otras usan `FileCacheService` directamente. Esto puede causar inconsistencias.

**Impacto**: 
- Tags y proyectos pueden no estar sincronizados
- Los conteos pueden no coincidir

**Solución recomendada**:
- Usar una única fuente de verdad (backend)
- Sincronizar `BackendMetadataStore` con el backend regularmente
- O migrar todas las vistas a usar `FileCacheService` y `GrpcKnowledgeClient`

### 2. Cache Desactualizado

**Problema**: `FileCacheService` tiene un TTL de 30 segundos, pero algunas vistas pueden mostrar datos desactualizados.

**Impacto**:
- Archivos nuevos pueden no aparecer inmediatamente
- Cambios en metadatos pueden no reflejarse

**Solución recomendada**:
- Reducir TTL o refrescar cache cuando sea necesario
- Agregar método para invalidar cache específico

### 3. Falta de Verificación de Datos

**Problema**: No hay verificación automática de que los datos mostrados coincidan con la BD.

**Impacto**:
- Errores silenciosos
- Datos incorrectos mostrados al usuario

**Solución recomendada**:
- Agregar logging para verificar datos
- Crear script de verificación que compare vista con BD

## Recomendaciones Generales

1. **Estandarizar Fuentes de Datos**:
   - Todas las vistas deberían usar el backend como fuente de verdad
   - `IMetadataStore` debería ser solo un cache del backend

2. **Mejorar Cache**:
   - Implementar invalidación inteligente del cache
   - Agregar eventos para refrescar cache cuando sea necesario

3. **Agregar Verificación**:
   - Crear script de verificación que compare datos de vistas con BD
   - Agregar logging detallado para debugging

4. **Documentar Vistas**:
   - Documentar cómo cada vista obtiene sus datos
   - Documentar qué campos de la BD usa cada vista

5. **Testing**:
   - Crear tests que verifiquen que las vistas muestran datos correctos
   - Tests de integración con la BD

## Script de Verificación

Se recomienda crear un script que:

1. Obtenga datos directamente de la BD del backend
2. Obtenga datos de cada vista
3. Compare y reporte diferencias
4. Genere un reporte de inconsistencias

Este script debería ejecutarse periódicamente para detectar problemas temprano.






