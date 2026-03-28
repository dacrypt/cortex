# Extracción de Información Contextual con LLM y RAG

## 📋 Resumen

Este documento describe el sistema de extracción de información contextual de documentos usando LLM (Large Language Models) con soporte RAG (Retrieval-Augmented Generation).

## 🎯 Objetivo

Extraer información contextual estructurada de documentos que ayude a identificar:
- **Autores y contribuidores**: Autores principales, co-autores, editores, traductores
- **Información de publicación**: Editorial, año, lugar, ISBN/ISSN
- **Temporalidad**: Fechas importantes, períodos históricos
- **Geografía**: Lugares mencionados, regiones relevantes
- **Personas**: Personas importantes mencionadas con sus roles
- **Organizaciones**: Instituciones, organizaciones mencionadas
- **Eventos**: Eventos históricos mencionados
- **Referencias**: Referencias bibliográficas
- **Metadatos adicionales**: Género, tema, audiencia, idioma original

## 🏗️ Arquitectura

### 1. Entidades de Dominio

**`AIContext`** (`backend/internal/domain/entity/context.go`)
- Contiene toda la información contextual extraída
- Estructuras anidadas para autores, ubicaciones, personas, organizaciones, eventos, referencias
- Campos de confianza y metadatos de extracción

**`FileMetadata.AIContext`**
- Campo agregado a `FileMetadata` para almacenar el contexto extraído
- Se persiste en la base de datos junto con otros metadatos

### 2. LLM Router

**`ExtractContextualInfo()`** (`backend/internal/infrastructure/llm/router.go`)
- Método que extrae información contextual usando LLM
- Soporta RAG: usa contexto de documentos similares del workspace
- Genera respuesta en formato JSON estructurado
- Detecta idioma automáticamente (español/inglés)
- Temperatura baja (0.3) para respuestas más estructuradas

### 3. Prompt Engineering

El prompt está diseñado para:
- Extraer información estructurada en JSON
- Usar contexto RAG para mantener consistencia
- Ser preciso con fechas y nombres
- Manejar campos opcionales (null si no disponibles)

## 📊 Estructura de Datos

### AIContext
```go
type AIContext struct {
    Authors          []AuthorInfo
    Editors          []string
    Translators      []string
    Contributors     []string
    Publisher        *string
    PublicationYear  *int
    PublicationPlace *string
    ISBN             *string
    ISSN             *string
    DocumentDate     *time.Time
    HistoricalPeriod *string
    Locations        []LocationInfo
    PeopleMentioned  []PersonInfo
    Organizations    []OrgInfo
    HistoricalEvents []EventInfo
    References       []ReferenceInfo
    OriginalLanguage *string
    Genre            *string
    Subject          *string
    Audience         *string
    Confidence       float64
    ExtractedAt      time.Time
    Source           string
}
```

## 🔄 Flujo de Procesamiento

1. **DocumentStage**: Extrae contenido del documento
2. **AIStage**: Genera resumen y categoría
3. **ContextStage** (a implementar): 
   - Usa RAG para encontrar documentos similares
   - Llama a `ExtractContextualInfo()` con contenido, resumen y contexto RAG
   - Parsea respuesta JSON
   - Guarda en `FileMetadata.AIContext`

## 🚀 Próximos Pasos

1. **Crear ContextStage** o extender AIStage
2. **Parser JSON**: Crear función para parsear respuesta JSON del LLM
3. **Persistencia**: Actualizar repositorio para guardar AIContext
4. **Integración RAG**: Usar RAG service para encontrar documentos similares
5. **Testing**: Agregar tests y mostrar en verbose test

## 💡 Casos de Uso

### Búsqueda por Autor
- Encontrar todos los documentos de un autor específico
- Agrupar documentos por autor

### Búsqueda Temporal
- Filtrar documentos por año de publicación
- Encontrar documentos de un período histórico

### Búsqueda Geográfica
- Filtrar por lugar de publicación
- Encontrar documentos relacionados con una región

### Análisis de Redes
- Visualizar relaciones entre personas mencionadas
- Mapear organizaciones e instituciones

### Referencias Cruzadas
- Encontrar documentos que citan otros documentos
- Construir bibliografías automáticas

## 🔍 Ejemplo de Uso

```go
// En AIStage o ContextStage
ragService := rag.NewService(...)
similarDocs, _ := ragService.Search(ctx, workspaceID, content, 5)

ragContext := make([]string, len(similarDocs))
for i, doc := range similarDocs {
    ragContext[i] = doc.Summary
}

jsonResponse, err := llmRouter.ExtractContextualInfo(
    ctx,
    content,
    summary,
    ragContext,
)

// Parsear JSON y crear AIContext
context := parseContextualInfo(jsonResponse)
fileMeta.AIContext = context
```

## 📝 Notas

- La extracción usa RAG para mantener consistencia con otros documentos del workspace
- El formato JSON permite fácil parsing y validación
- Los campos opcionales permiten manejar documentos con información incompleta
- La confianza puede usarse para filtrar o priorizar información






