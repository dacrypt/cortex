# Enriquecimiento de Metadatos con ISBN

## 📋 Resumen

Este documento describe el sistema de enriquecimiento de metadatos de libros usando ISBN para consultar APIs externas (Open Library, Google Books, Amazon).

## 🎯 Objetivo

Cuando se detecta un ISBN en un documento, el sistema puede consultar APIs externas para enriquecer automáticamente los metadatos con información adicional como:
- Título completo y subtítulo
- Autores (lista completa)
- Editorial y fecha de publicación
- Descripción del libro
- Categorías y género
- Calificaciones y reseñas
- Imagen de portada
- Enlaces a Amazon, Goodreads, etc.
- Información física (páginas, formato, dimensiones)

## 🏗️ Arquitectura

### 1. MetadataEnrichmentService

**Ubicación**: `backend/internal/infrastructure/metadata/enrichment.go`

Servicio que:
- Recibe un ISBN (10 o 13 dígitos)
- Consulta múltiples fuentes en orden de prioridad:
  1. **Open Library** (gratis, sin API key)
  2. **Google Books** (gratis, sin API key)
  3. **Amazon** (requiere API key - pendiente de implementación)
- Retorna metadatos enriquecidos estructurados

### 2. Fuentes de Datos

#### Open Library API
- **URL**: `https://openlibrary.org/api/books?bibkeys=ISBN:{isbn}&format=json&jscmd=data`
- **Ventajas**: Gratis, sin API key, datos bibliográficos completos
- **Datos**: Título, autores, editorial, fecha, páginas, portada, categorías

#### Google Books API
- **URL**: `https://www.googleapis.com/books/v1/volumes?q=isbn:{isbn}`
- **Ventajas**: Gratis, sin API key, incluye descripciones y calificaciones
- **Datos**: Título, autores, descripción, calificaciones, portada, categorías

#### Amazon Product Advertising API (Futuro)
- **Requisitos**: AWS credentials, API key
- **Ventajas**: Datos comerciales, precios, disponibilidad
- **Estado**: Pendiente de implementación

### 3. Estructura de Datos

**`EnrichedMetadata`** (`backend/internal/domain/entity/context.go`)
```go
type EnrichedMetadata struct {
    Title           string
    Subtitle        string
    Authors         []string
    Publisher       string
    PublicationDate string
    ISBN10          string
    ISBN13          string
    Pages           int
    Language        string
    Description     string
    Categories      []string
    Rating          float64
    CoverImageURL   string
    AmazonURL       string
    Source          string
    EnrichedAt      time.Time
}
```

## 🔄 Flujo de Procesamiento

1. **Extracción de ISBN**: El LLM extrae ISBN del documento en `AIContext.ISBN`
2. **Enriquecimiento**: Si se encuentra ISBN, se llama a `EnrichWithISBN()`
3. **Consulta APIs**: Se consultan APIs externas en orden de prioridad
4. **Fusión de Datos**: Los metadatos enriquecidos se fusionan con `AIContext`
5. **Persistencia**: Se guarda `AIContext.EnrichedMetadata` en la base de datos

## 💻 Uso

### Ejemplo Básico

```go
enrichmentService := metadata.NewMetadataEnrichmentService(logger)

// Enriquecer con ISBN directamente
enriched, err := enrichmentService.EnrichWithISBN(ctx, "978-0-123456-78-9")
if err != nil {
    log.Printf("Error enriching: %v", err)
} else {
    log.Printf("Title: %s", enriched.Title)
    log.Printf("Authors: %v", enriched.Authors)
    log.Printf("Publisher: %s", enriched.Publisher)
}
```

### Integración con AIContext

```go
// Después de extraer AIContext con LLM
if aiContext.ISBN != nil {
    enriched, err := enrichmentService.EnrichAIContext(ctx, aiContext)
    if err == nil {
        // Los datos enriquecidos ya están fusionados en aiContext
        aiContext.EnrichedMetadata = &entity.EnrichedMetadata{
            Title:           enriched.Title,
            Authors:         enriched.Authors,
            Publisher:       enriched.Publisher,
            PublicationDate: enriched.PublicationDate,
            Description:     enriched.Description,
            CoverImageURL:   enriched.CoverImageURL,
            Source:          enriched.Source,
            EnrichedAt:      enriched.EnrichedAt,
        }
    }
}
```

## 🔍 Casos de Uso

### 1. Búsqueda de Libros por Autor
- Encontrar todos los libros de un autor específico
- Agrupar por editorial o año

### 2. Catálogo Automático
- Generar catálogo completo con portadas
- Incluir descripciones y categorías

### 3. Recomendaciones
- Usar calificaciones para recomendar libros similares
- Agrupar por género o categoría

### 4. Análisis de Colección
- Estadísticas de editoriales
- Distribución por año de publicación
- Idiomas más comunes

## ⚙️ Configuración

### Timeout
El servicio HTTP tiene un timeout de 10 segundos por defecto. Se puede ajustar:

```go
enrichmentService := &MetadataEnrichmentService{
    httpClient: &http.Client{
        Timeout: 30 * time.Second, // Aumentar timeout si es necesario
    },
    logger: logger,
}
```

### Manejo de Errores

El servicio intenta múltiples fuentes automáticamente:
1. Si Open Library falla, intenta Google Books
2. Si Google Books falla, retorna error
3. Los errores se registran pero no detienen el pipeline

## 🚀 Próximos Pasos

1. **Amazon API**: Implementar consulta a Amazon Product Advertising API
2. **Caché**: Implementar caché de metadatos enriquecidos para evitar consultas repetidas
3. **Validación**: Validar ISBN antes de consultar APIs
4. **Rate Limiting**: Implementar rate limiting para evitar exceder límites de APIs
5. **Fallback**: Implementar estrategias de fallback más robustas

## 📝 Notas

- Las APIs externas pueden tener rate limits
- Algunos libros pueden no estar disponibles en todas las fuentes
- Los datos pueden variar entre fuentes (preferir Open Library para datos bibliográficos)
- El enriquecimiento es opcional - si falla, el documento sigue procesándose normalmente






