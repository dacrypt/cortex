# Revisión del Índice de Base de Datos y Estado del Proceso

## Ubicación de la Base de Datos

Según la configuración (`cortexd.local.yaml`):
- **Data Directory**: `../tmp/cortex-test-data`
- **Database Path**: `tmp/cortex-test-data/cortex.sqlite`

## Esquema de Base de Datos

### Migraciones Aplicadas

El sistema tiene 4 migraciones:

1. **v1: initial_schema** - Tablas base
   - `workspaces` - Workspaces registrados
   - `files` - Índice de archivos con flags de indexación
   - `file_metadata` - Metadata semántica (tags, contexts, AI summaries)
   - `file_tags` - Tags asignados a archivos
   - `file_contexts` - Proyectos/contextos asignados
   - `file_context_suggestions` - Sugerencias de proyectos
   - `tasks` - Tareas del sistema
   - `scheduled_tasks` - Tareas programadas

2. **v2: documents_and_vectors** - Soporte RAG
   - `documents` - Documentos parseados
   - `chunks` - Chunks de documentos
   - `chunk_embeddings` - Vectores de embeddings

3. **v3: ai_metadata_fields** - Campos AI adicionales
   - `ai_category` - Categoría AI
   - `ai_category_confidence` - Confianza de categoría
   - `ai_category_updated_at` - Timestamp de categoría
   - `ai_related` - Archivos relacionados (JSON)

4. **v4: file_processing_traces** - Trazas de procesamiento
   - `file_traces` - Logs de operaciones LLM/pipeline

## Flags de Indexación en `files` Table

Cada archivo tiene flags que indican qué stages del pipeline se han completado:

- `indexed_basic` (INTEGER) - BasicStage completado
- `indexed_mime` (INTEGER) - MimeStage completado
- `indexed_mirror` (INTEGER) - MirrorStage completado
- `indexed_code` (INTEGER) - CodeStage completado
- `indexed_document` (INTEGER) - DocumentStage completado (embeddings creados)

Estos flags se actualizan automáticamente cuando cada stage completa exitosamente.

## Proceso de Indexación Esperado

### Orden del Pipeline:
1. **BasicStage** → `indexed_basic = 1`
2. **MimeStage** → `indexed_mime = 1`
3. **MirrorStage** → `indexed_mirror = 1`
4. **CodeStage** → `indexed_code = 1` (solo para archivos de código)
5. **DocumentStage** → `indexed_document = 1` (crea embeddings)
6. **AIStage** → Actualiza `file_metadata` (tags, categories, summaries, related)

### Verificación del Estado

Para verificar que el proceso está funcionando correctamente, revisa:

1. **Migraciones aplicadas**: Todas las migraciones (1-4) deben estar aplicadas
2. **Flags de indexación**: Los archivos deben tener los flags correspondientes según su tipo
3. **Embeddings creados**: `chunk_embeddings` debe tener entradas para documentos procesados
4. **Metadata AI**: `file_metadata` debe tener `ai_summary`, `ai_category`, etc. para archivos procesados
5. **Trazas**: `file_traces` debe tener registros de operaciones recientes

## Comandos para Verificar

### Usando sqlite3 (si está disponible):

```bash
# Ver migraciones
sqlite3 tmp/cortex-test-data/cortex.sqlite "SELECT version, name FROM _migrations;"

# Contar archivos por estado de indexación
sqlite3 tmp/cortex-test-data/cortex.sqlite "
SELECT 
  COUNT(*) as total,
  SUM(indexed_basic) as basic,
  SUM(indexed_mime) as mime,
  SUM(indexed_mirror) as mirror,
  SUM(indexed_code) as code,
  SUM(indexed_document) as document
FROM files;
"

# Ver documentos y embeddings
sqlite3 tmp/cortex-test-data/cortex.sqlite "
SELECT 
  (SELECT COUNT(*) FROM documents) as documents,
  (SELECT COUNT(*) FROM chunks) as chunks,
  (SELECT COUNT(*) FROM chunk_embeddings) as embeddings;
"

# Ver metadata AI
sqlite3 tmp/cortex-test-data/cortex.sqlite "
SELECT 
  COUNT(*) as total,
  COUNT(ai_summary) as with_summary,
  COUNT(ai_category) as with_category
FROM file_metadata;
"
```

### Usando el script Go:

```bash
cd backend
go run ../scripts/analyze_database.go
```

## Indicadores de Problemas

### ❌ Problemas Potenciales:

1. **Migraciones faltantes**: Si alguna migración (especialmente v2-v4) no está aplicada
2. **Flags inconsistentes**: Archivos con `indexed_document = 1` pero sin embeddings
3. **Embeddings faltantes**: Documentos sin embeddings (puede ser normal si falló el embedding)
4. **Metadata AI faltante**: Archivos procesados sin tags/categories (puede ser normal si AI está deshabilitado)

### ✅ Estado Saludable:

- Todas las migraciones aplicadas
- Archivos tienen flags apropiados según su tipo
- Documentos tienen embeddings (o al menos algunos)
- Metadata AI presente para archivos procesados
- Trazas recientes en `file_traces`

## Notas sobre el Proceso Actual

Según los logs vistos:
- ✅ Pipeline está ejecutándose
- ✅ DocumentStage está procesando archivos
- ✅ AIStage está generando tags, categories, projects
- ⚠️ Algunos embeddings fallan por límite de contexto (ahora corregido con truncamiento)

El proceso debería estar funcionando correctamente después de:
1. Reiniciar el daemon con el código actualizado
2. Verificar que las migraciones estén aplicadas
3. Confirmar que los embeddings se están creando correctamente







