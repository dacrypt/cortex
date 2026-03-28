# Oportunidades de Migración a langchaingo

## 📋 Resumen Ejecutivo

Análisis completo de qué funcionalidades de langchaingo podemos usar para reducir nuestro código y confiar en una librería madura en lugar de mantener código duplicado.

## 🎯 Oportunidades Identificadas

### 1. ✅ **Text Splitters** - ALTA PRIORIDAD

**Ubicación Actual**: `backend/internal/application/pipeline/stages/document.go`
- `parseSections()` - parsing manual de markdown
- `buildChunks()` - construcción manual de chunks
- `splitParagraphs()` - splitting simple
- `splitByTokens()` - splitting por tokens

**Lo que ofrece langchaingo**:
- `MarkdownTextSplitter` - Splitter especializado para markdown
  - Respeta jerarquía de headings
  - Maneja code blocks
  - Soporta overlap entre chunks
  - Mantiene estructura de markdown
- `RecursiveCharacter` - Splitter recursivo genérico
  - Separadores configurables
  - Chunk size y overlap configurables
  - Función de longitud personalizable

**Código Actual** (~300 líneas):
```go
func parseSections(body string, startLine int) []mdSection {
    // Parsing manual de headings, stack de jerarquía, etc.
}

func buildChunks(entry *entity.FileEntry, docID entity.DocumentID, 
                 sections []mdSection, minTokens, maxTokens int) []*entity.Chunk {
    // Construcción manual de chunks con lógica de tokens
}
```

**Código con langchaingo** (~50 líneas):
```go
import "github.com/tmc/langchaingo/textsplitter"

func buildChunksWithLangchain(text string, docID entity.DocumentID, 
                               headingPath string) []*entity.Chunk {
    splitter := textsplitter.NewMarkdownTextSplitter(
        textsplitter.WithChunkSize(800),
        textsplitter.WithChunkOverlap(200),
        textsplitter.WithKeepHeadingHierarchy(true),
    )
    
    chunks, err := splitter.SplitText(text)
    if err != nil {
        return nil
    }
    
    // Convertir a nuestros chunks
    result := make([]*entity.Chunk, 0, len(chunks))
    for i, chunkText := range chunks {
        result = append(result, &entity.Chunk{
            ID:          entity.NewChunkID(docID, i+1, headingPath),
            DocumentID:  docID,
            Text:        chunkText,
            TokenCount:  countTokens(chunkText),
            // ...
        })
    }
    return result
}
```

**Beneficios**:
- ✅ Reduce ~250 líneas de código
- ✅ Mejor manejo de edge cases (code blocks, tables, etc.)
- ✅ Mantiene jerarquía de headings automáticamente
- ✅ Overlap entre chunks para mejor contexto
- ✅ Probado y mantenido por la comunidad

**Impacto**: 🔴 **ALTO** - Reduce código significativo y mejora calidad

---

### 2. ✅ **RAG Chains** - ALTA PRIORIDAD

**Ubicación Actual**: 
- `backend/internal/infrastructure/llm/rag_chain.go` (nuestro RAGChain)
- `backend/internal/application/rag/service.go` (lógica RAG manual)

**Lo que ofrece langchaingo**:
- `RetrievalQA` - Chain completa para RAG
  - Integra retriever + combine documents chain
  - Maneja source documents
  - Abstracción clara del flujo
- `LoadStuffQA` - Chain para combinar documentos
- `ConversationalRetrievalQA` - RAG con memoria/conversación

**Código Actual** (~200 líneas):
```go
// Nuestro RAGChain manual
func (c *RAGChain) ExecuteWithSources(ctx context.Context, query string, sources []RAGSource) (*RAGResult, error) {
    // 1. Build context
    contextStr := c.buildContext(sources)
    // 2. Generate with LLM
    prompt := c.llmRouter.GetRAGAnswerPrompt(contextStr, query)
    response, err := c.llmRouter.Generate(ctx, GenerateRequest{...})
    // 3. Clean answer
    cleanedAnswer := stringParser.ParseString(response.Text)
    return &RAGResult{Answer: cleanedAnswer, Sources: sources}, nil
}
```

**Código con langchaingo** (~100 líneas):
```go
import (
    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/schema"
)

// Crear retriever que implementa schema.Retriever
type CortexRetriever struct {
    vectorStore repository.VectorStore
    embedder    embedding.Embedder
    workspaceID entity.WorkspaceID
}

func (r *CortexRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]schema.Document, error) {
    // Nuestra lógica de búsqueda vectorial
    vector, _ := r.embedder.Embed(ctx, query)
    matches, _ := r.vectorStore.Search(ctx, r.workspaceID, vector, 10)
    // Convertir a schema.Document
    docs := make([]schema.Document, len(matches))
    for i, match := range matches {
        docs[i] = schema.Document{
            PageContent: match.Snippet,
            Metadata: map[string]interface{}{
                "path": match.RelativePath,
                "score": match.Score,
            },
        }
    }
    return docs, nil
}

// Usar RetrievalQA chain
func (s *Service) QueryWithChain(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
    retriever := &CortexRetriever{
        vectorStore: s.vectorStore,
        embedder:    s.embedder,
        workspaceID: req.WorkspaceID,
    }
    
    // Crear LLM wrapper que implementa llms.Model
    llmModel := NewLangchainLLMWrapper(s.llmRouter)
    
    // Crear chain
    qaChain := chains.NewRetrievalQAFromLLM(llmModel, retriever)
    
    // Ejecutar
    result, err := chains.Call(ctx, qaChain, map[string]any{
        "query": req.Query,
    })
    
    return &QueryResponse{
        Answer: result["text"].(string),
        Sources: extractSources(result),
    }, nil
}
```

**Beneficios**:
- ✅ Abstracción estándar y probada
- ✅ Soporte para diferentes tipos de chains
- ✅ Integración con memoria/conversación
- ✅ Menos código que mantener

**Impacto**: 🔴 **ALTO** - Simplifica lógica RAG significativamente

---

### 3. ⚠️ **Prompts/Templates** - MEDIA PRIORIDAD

**Ubicación Actual**: 
- `backend/internal/infrastructure/llm/prompt_templates.go` (nuestro sistema)
- `backend/internal/infrastructure/llm/router.go` (prompts inline)

**Lo que ofrece langchaingo**:
- `prompts` package con templates estructurados
- `PromptTemplate` con variables
- `FewShotPromptTemplate` para ejemplos
- `ChatPromptTemplate` para conversaciones

**Beneficios**:
- ✅ Templates más estructurados
- ✅ Validación de variables
- ✅ Soporte para few-shot learning

**Impacto**: 🟡 **MEDIO** - Mejora organización pero no reduce código significativamente

---

### 4. ⚠️ **Memory/Conversation** - BAJA PRIORIDAD

**Ubicación Actual**: No tenemos memoria/conversación persistente

**Lo que ofrece langchaingo**:
- `memory` package con diferentes tipos:
  - `SimpleMemory` - Memoria simple
  - `ConversationBufferMemory` - Buffer de conversación
  - `ConversationSummaryMemory` - Memoria con resumen

**Beneficios**:
- ✅ Si en el futuro queremos agregar conversación
- ✅ Útil para chatbots o asistentes interactivos

**Impacto**: 🟢 **BAJO** - No es prioridad actual pero útil para futuro

---

### 5. ⚠️ **Document Loaders** - BAJA PRIORIDAD

**Ubicación Actual**: Tenemos extractores propios (PDF, Office, etc.)

**Lo que ofrece langchaingo**:
- `documentloaders` package con loaders para:
  - PDF
  - Text files
  - Web pages
  - etc.

**Consideración**:
- Nuestros extractores son más especializados (metadata, EXIF, etc.)
- langchaingo loaders son más básicos
- **Recomendación**: Mantener nuestros extractores, pero podríamos usar loaders para casos simples

**Impacto**: 🟢 **BAJO** - Nuestros extractores son superiores

---

## 📊 Plan de Migración Recomendado

### Fase 1: Text Splitters (Alta Prioridad) ⭐

**Esfuerzo**: Medio (2-3 días)
**Beneficio**: Alto (reduce ~250 líneas, mejora calidad)

1. Reemplazar `parseSections()` y `buildChunks()` con `MarkdownTextSplitter`
2. Mantener lógica de enriquecimiento de metadata (único a nosotros)
3. Adaptar nuestros `Chunk` entities a los resultados de langchaingo
4. Tests para verificar que funciona igual o mejor

**Código a eliminar**:
- `parseSections()` (~50 líneas)
- `buildChunks()` (~100 líneas)
- `splitParagraphs()` (~10 líneas)
- `splitByTokens()` (~15 líneas)
- Lógica de parsing de headings (~30 líneas)

**Total**: ~205 líneas eliminadas

---

### Fase 2: RAG Chains (Alta Prioridad) ⭐

**Esfuerzo**: Medio-Alto (3-4 días)
**Beneficio**: Alto (simplifica lógica, abstracción estándar)

1. Crear `CortexRetriever` que implementa `schema.Retriever`
2. Crear wrapper `LangchainLLMWrapper` que implementa `llms.Model`
3. Migrar `rag/service.go` para usar `RetrievalQA` chain
4. Mantener nuestro `RAGChain` como fallback o eliminar si no se usa

**Código a simplificar**:
- `rag_chain.go` - Podría eliminarse o simplificarse
- Lógica manual en `rag/service.go` - Simplificada con chain

---

### Fase 3: Output Parsers (Ya hecho) ✅

**Estado**: Completado
- Usamos `CommaSeparatedList` de langchaingo como fallback
- Mantenemos nuestros parsers robustos como primarios

---

## 🎯 Decisión Final

### ✅ **MIGRAR** (Alta Prioridad):

1. **Text Splitters** - Reducción significativa de código, mejor calidad
2. **RAG Chains** - Abstracción estándar, menos código que mantener

### ⚠️ **EVALUAR** (Media Prioridad):

3. **Prompts/Templates** - Mejora organización pero no reduce código mucho

### ❌ **NO MIGRAR** (Baja Prioridad o No Aplicable):

4. **Document Loaders** - Nuestros extractores son superiores
5. **Memory** - No es prioridad actual
6. **Agents** - No los usamos actualmente

---

## 💡 Recomendación Final

**Empezar con Fase 1 (Text Splitters)** porque:
- ✅ Mayor reducción de código (~205 líneas)
- ✅ Mejor calidad (mejor manejo de edge cases)
- ✅ Menos código que mantener
- ✅ Probado por la comunidad

**Luego Fase 2 (RAG Chains)** porque:
- ✅ Simplifica lógica compleja
- ✅ Abstracción estándar
- ✅ Facilita futuras mejoras (memoria, etc.)

**Total estimado de código eliminado**: ~300-400 líneas
**Tiempo estimado**: 5-7 días de desarrollo + testing

---

## 🔍 Consideraciones

### Ventajas de usar langchaingo:
- ✅ Código probado y mantenido por la comunidad
- ✅ Menos código que mantener nosotros
- ✅ Estándares de la industria
- ✅ Facilita onboarding de nuevos desarrolladores

### Desventajas:
- ⚠️ Dependencia externa adicional
- ⚠️ Necesitamos adaptar nuestros tipos a los de langchaingo
- ⚠️ Podría requerir cambios en tests

### Mitigación:
- ✅ langchaingo es estable y bien mantenido
- ✅ Podemos mantener wrappers para nuestros tipos
- ✅ Tests exhaustivos durante migración






