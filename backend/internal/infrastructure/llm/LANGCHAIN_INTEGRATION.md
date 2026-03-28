# Integración de LangChainGo en Cortex

## Resumen

Se ha integrado **langchaingo** (LangChain para Go) en Cortex para mejorar la robustez y mantenibilidad del procesamiento de LLMs. La integración se ha realizado de forma pragmática, aplicando los conceptos y patrones de LangChain sin depender completamente de todas sus APIs.

## Cambios Implementados

### 1. Output Parsers Robustos (`parsers.go`)

Se creó un módulo de parsers inspirado en langchaingo que proporciona:

- **JSONParser**: Parser robusto con retry automático (hasta 3 intentos)
  - Limpieza automática de markdown code blocks
  - Extracción de JSON de texto envolvente
  - Limpieza agresiva en reintentos (remueve comas finales, comentarios, etc.)
  - Manejo estructurado de errores

- **StringParser**: Parser para respuestas simples de texto
  - Limpieza automática de comillas y puntuación
  - Remoción de markdown code blocks

- **ArrayParser**: Parser para arrays de strings (tags, etc.)
  - Soporta JSON arrays y listas separadas por comas
  - Validación y limpieza de cada elemento

**Uso:**
```go
parser := NewJSONParser(logger)
var result MyStruct
err := parser.ParseJSON(ctx, llmResponse, &result)
```

### 2. Refactorización de Context Parser

Se refactorizó `context_parser.go` para usar el nuevo `JSONParser`:

- **Antes**: Parsing manual frágil con múltiples intentos de limpieza
- **Después**: Uso del `JSONParser` robusto con retry automático
- **Beneficio**: Código más limpio, más robusto, y mejor manejo de errores

### 3. Sistema de Prompt Templates (`prompt_templates.go`)

Se creó un sistema de templates estructurado:

- **SimplePromptTemplate**: Template simple usando `fmt.Sprintf`
- **PromptTemplateRegistry**: Registro centralizado de templates
- **Templates Predefinidos**:
  - `TagSuggestionTemplate`: Para sugerencias de tags
  - `ProjectSuggestionTemplate`: Para sugerencias de proyectos
  - `SummaryTemplate`: Para generación de resúmenes
  - `CategoryClassificationTemplate`: Para clasificación de categorías

**Uso:**
```go
// Usar template predefinido
prompt := FormatTagSuggestion(maxTags, summary, description)

// O usar registry
registry := NewPromptTemplateRegistry()
registry.Register("my_template", "Template: %s")
prompt, _ := registry.Format("my_template", value)
```

## Beneficios

### Robustez
- ✅ Parsing con retry automático reduce fallos por respuestas malformadas
- ✅ Limpieza agresiva en reintentos mejora la tasa de éxito
- ✅ Manejo estructurado de errores facilita debugging

### Mantenibilidad
- ✅ Templates centralizados facilitan actualización de prompts
- ✅ Código más limpio y reutilizable
- ✅ Separación de responsabilidades (parsing vs. lógica de negocio)

### Calidad
- ✅ Mejor manejo de edge cases (markdown, comillas, puntuación)
- ✅ Validación automática de respuestas
- ✅ Logging mejorado para debugging

## Próximos Pasos (Opcional)

### Chains Simples
Se pueden crear chains simples para flujos comunes:

```go
// Ejemplo conceptual
type RAGChain struct {
    embedder Embedder
    vectorStore VectorStore
    llmRouter *Router
    parser *JSONParser
}

func (c *RAGChain) Execute(ctx context.Context, query string) (Result, error) {
    // 1. Create embedding
    vector, _ := c.embedder.Embed(ctx, query)
    
    // 2. Search vector store
    matches, _ := c.vectorStore.Search(ctx, vector, 10)
    
    // 3. Build prompt with context
    prompt := buildRAGPrompt(query, matches)
    
    // 4. Call LLM
    response, _ := c.llmRouter.Generate(ctx, prompt)
    
    // 5. Parse response
    var result Result
    _ = c.parser.ParseJSON(ctx, response.Text, &result)
    
    return result, nil
}
```

### Integración con LangChain Agents (Futuro)
Para tareas más complejas que requieren múltiples pasos o decisiones, se podría integrar langchaingo's agents:

```go
// Ejemplo futuro
agent := langchaingo.NewAgent(...)
result, err := agent.Run(ctx, task)
```

## Dependencias

- `github.com/tmc/langchaingo v0.1.14`: Framework LangChain para Go

## Archivos Modificados

1. `backend/internal/infrastructure/llm/parsers.go` (nuevo)
2. `backend/internal/infrastructure/llm/prompt_templates.go` (nuevo)
3. `backend/internal/infrastructure/llm/context_parser.go` (refactorizado)
4. `backend/internal/infrastructure/llm/router.go` (actualizado para usar nuevo parser)
5. `backend/go.mod` (agregada dependencia langchaingo)

## Notas

- La integración es **pragmática**: usa conceptos de LangChain pero no depende completamente de todas sus APIs
- Los parsers son **independientes** y pueden usarse sin langchaingo si es necesario
- Los templates son **simples** y no requieren dependencias adicionales
- El código es **backward compatible** con el código existente

## Referencias

- [LangChainGo GitHub](https://github.com/tmc/langchaingo)
- [LangChain Documentation](https://python.langchain.com/docs/get_started/introduction)






