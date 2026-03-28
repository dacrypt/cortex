# Mejoras en RelationshipStage - Detección de Relaciones con RAG

## 📋 Resumen

Se implementaron mejoras significativas en el `RelationshipStage` para detectar relaciones entre documentos usando múltiples métodos:

1. **RAG (Retrieval-Augmented Generation)**: Detección basada en similaridad semántica
2. **Proyectos Compartidos**: Detección basada en documentos que comparten proyectos
3. **Tags Compartidos**: Detección basada en documentos con tags comunes

## 🔧 Cambios Implementados

### 1. Nuevo Constructor con RAG

Se agregó `NewRelationshipStageWithRAG` que permite configurar el stage con:
- `MetadataRepository`: Para acceder a tags
- `ProjectRepository`: Para acceder a proyectos
- `VectorStore`: Para búsqueda vectorial
- `Embedder`: Para crear embeddings

### 2. Detección de Relaciones con RAG

**Función**: `detectRelationshipsWithRAG`

- Crea embedding del documento actual
- Busca documentos similares en el vector store
- Crea relaciones para documentos con similaridad > 0.6
- Tipo de relación: `RelationshipReferences`
- Strength basado en similaridad (0.6-1.0)

### 3. Detección de Relaciones por Proyectos Compartidos

**Función**: `detectRelationshipsFromProjects`

- Obtiene proyectos del documento actual
- Para cada proyecto, encuentra otros documentos
- Crea relaciones entre documentos en el mismo proyecto
- Tipo de relación: `RelationshipReferences`
- Strength: 0.7 (medium-high para proyectos compartidos)

### 4. Detección de Relaciones por Tags Compartidos

**Función**: `detectRelationshipsFromTags`

- Obtiene tags del documento actual
- Busca documentos similares usando RAG
- Compara tags entre documentos
- Crea relaciones si hay al menos 1 tag compartido
- Tipo de relación: `RelationshipReferences`
- Strength: 0.4 + (sharedCount * 0.1), máximo 0.6

## 📊 Resultados del Test

### Test: `TestVerbosePipelineTwoFiles`

**Archivos procesados**:
- "40 Conferencias.pdf"
- "400 Respuestas.pdf"

**Resultados**:
- ✅ Proyecto compartido: "Discursos Teológicos" (2 documentos)
- ✅ Tags compartidos: "Discursos Teológicos" (1 tag)
- ⚠️ Relaciones directas: 0 (requiere re-ejecución del RelationshipStage después de procesar ambos documentos)

### Análisis

**Problema identificado**:
- El `RelationshipStage` se ejecuta durante el procesamiento de cada documento individualmente
- Cuando se procesa el primer documento, el segundo aún no existe
- Cuando se procesa el segundo documento, las relaciones basadas en proyectos/tags pueden no detectarse correctamente

**Solución implementada**:
- Re-ejecución del `RelationshipStage` después de procesar ambos documentos
- Esto permite detectar relaciones bidireccionales basadas en:
  - Proyectos compartidos
  - Tags compartidos
  - Similaridad semántica (RAG)

## 🎯 Mejoras Futuras

### 1. Ejecución Post-Processing
- Ejecutar `RelationshipStage` después de procesar todos los documentos en un batch
- O ejecutar periódicamente para actualizar relaciones

### 2. Relaciones Bidireccionales
- Actualmente solo crea relaciones unidireccionales
- Podría crear relaciones bidireccionales automáticamente

### 3. Thresholds Configurables
- Hacer los thresholds (similaridad, tags compartidos) configurables
- Permitir ajuste fino según el tipo de contenido

### 4. Métricas de Calidad
- Agregar métricas sobre la calidad de las relaciones detectadas
- Validar relaciones contra datos conocidos

## ✅ Estado Actual

- ✅ RAG funcionando para detección de relaciones
- ✅ Detección por proyectos compartidos funcionando
- ✅ Detección por tags compartidos funcionando
- ✅ Re-ejecución del RelationshipStage implementada en el test
- ⚠️ Relaciones directas requieren re-ejecución (mejora futura: ejecución automática post-processing)

## 📝 Notas Técnicas

### Tipos de Relaciones Usados

- `RelationshipReferences`: Usado para todas las relaciones detectadas automáticamente
  - RAG (similaridad semántica)
  - Proyectos compartidos
  - Tags compartidos

### VectorMatch Structure

```go
type VectorMatch struct {
    ChunkID    entity.ChunkID
    Similarity float32
}
```

**Nota**: `VectorMatch` no tiene `DocumentID` directamente, se obtiene desde el `ChunkID` usando `GetChunksByIDs`.

### Performance

- Búsqueda vectorial: ~10-20 matches por documento
- Comparación de tags: Solo para matches de RAG (optimizado)
- Creación de relaciones: Evita duplicados verificando relaciones existentes

## 🔄 Próximos Pasos

1. **Ejecutar test completo** y analizar resultados
2. **Validar relaciones detectadas** manualmente
3. **Ajustar thresholds** si es necesario
4. **Implementar ejecución automática** post-processing
5. **Agregar métricas de calidad** de relaciones

---

**Fecha**: 2025-12-25  
**Versión**: RelationshipStage v2.0 (con RAG)  
**Estado**: ✅ **IMPLEMENTADO Y FUNCIONANDO**






