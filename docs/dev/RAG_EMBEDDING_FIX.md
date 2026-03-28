# Fix: Compatibilidad RAG con Límites de Embedding

## Problema

Los logs mostraban errores:
```
embedding request failed: status 500, body: {"error":"the input length exceeds the context length"}
```

Esto ocurría porque:
1. **DocumentStage**: Intentaba crear embeddings de chunks que podían ser demasiado largos
2. **AIStage**: Intentaba crear embeddings del contenido completo del archivo (varios miles de caracteres)

Los modelos de embedding como `nomic-embed-text` tienen límites de contexto (típicamente ~6000-8000 caracteres).

## Solución Implementada

### 1. Función Helper de Truncamiento

Agregada función `truncateForEmbedding()` en `AIStage`:
- Trunca texto a máximo 6000 caracteres (límite seguro)
- Trunca en límite de palabra cuando es posible
- Mantiene el 90% del texto si el espacio está cerca del final

```go
func truncateForEmbedding(text string, maxChars int) string {
    if maxChars <= 0 {
        maxChars = 6000 // Safe default
    }
    if len(text) <= maxChars {
        return text
    }
    // Truncate at word boundary
    truncated := text[:maxChars]
    lastSpace := strings.LastIndex(truncated, " ")
    if lastSpace > maxChars*9/10 {
        truncated = truncated[:lastSpace]
    }
    return truncated + "..."
}
```

### 2. DocumentStage Mejorado

**Antes**: Fallaba completamente si un chunk era demasiado largo.

**Después**:
- Trunca chunks que excedan 6000 caracteres
- Continúa procesando otros chunks aunque uno falle
- Permite indexación parcial de documentos grandes
- Solo falla si NO se creó ningún embedding

```go
// Truncate chunk text if too long
chunkText := chunk.Text
if len(chunkText) > 6000 {
    // Truncate at word boundary
    truncated := chunkText[:6000]
    lastSpace := strings.LastIndex(truncated, " ")
    if lastSpace > 5400 {
        chunkText = truncated[:lastSpace] + "..."
    } else {
        chunkText = truncated + "..."
    }
}

vector, err := s.embedder.Embed(ctx, chunkText)
if err != nil {
    // Log error but continue with other chunks
    continue
}
```

### 3. AIStage Mejorado

Todos los métodos RAG ahora truncan el contenido antes de crear embeddings:

- `classifyCategoryWithRAG()` - Trunca a 6000 chars
- `suggestTagsWithRAG()` - Trunca a 6000 chars
- `suggestProjectWithRAG()` - Trunca a 6000 chars
- `findRelatedFilesWithRAG()` - Trunca a 6000 chars, retorna lista vacía en error (no falla)
- `generateSummaryWithRAG()` - Trunca a 6000 chars

**Manejo de Errores Mejorado**:
- Si el embedding falla, hace fallback a métodos no-RAG
- `findRelatedFilesWithRAG()` retorna lista vacía en lugar de error
- Logs de debug para troubleshooting

## Beneficios

1. ✅ **Robustez**: No falla completamente si un embedding es demasiado largo
2. ✅ **Indexación Parcial**: Documentos grandes se indexan parcialmente
3. ✅ **Fallback Automático**: Si RAG falla, usa métodos estándar
4. ✅ **Mejor UX**: El pipeline continúa procesando otros archivos

## Límites Configurados

- **Máximo para embeddings**: 6000 caracteres
- **Truncamiento**: En límite de palabra cuando es posible
- **Tolerancia a errores**: Continúa con otros chunks/archivos

## Resultado Esperado

Los logs ahora mostrarán:
- ✅ Embeddings creados exitosamente (aunque truncados)
- ✅ Fallback a métodos no-RAG si embedding falla (sin error fatal)
- ✅ Indexación parcial de documentos grandes
- ✅ Pipeline continúa procesando todos los archivos

## Notas

- El límite de 6000 caracteres es conservador para `nomic-embed-text`
- Si usas otro modelo de embedding, ajusta el límite según su documentación
- Los chunks en DocumentStage ya están limitados a ~800 tokens, pero el límite de caracteres es más seguro







