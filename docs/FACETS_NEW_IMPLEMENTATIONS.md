# Nuevas Facetas Implementadas

## Resumen

Se han implementado **7 nuevas facetas** de alta prioridad del análisis, aumentando el total de facetas disponibles de **15 a 22**.

## Nuevas Facetas Implementadas

### 1. Sentiment Facet ✅
- **Nombre canónico**: `sentiment`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Análisis de sentimiento (positive, negative, neutral, mixed)
- **Fuente de datos**: `file_sentiment.overall_sentiment`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetSentimentFacet()`

### 2. Duplicate Type Facet ✅
- **Nombre canónico**: `duplicate_type`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Tipo de relación de duplicados (exact, near, version)
- **Fuente de datos**: `file_duplicates.type`
- **Disponibilidad**: Rare
- **Implementación**: `MetadataRepository.GetDuplicateTypeFacet()`

### 3. Location Facet ✅
- **Nombre canónico**: `location`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Ubicaciones geográficas mencionadas
- **Fuente de datos**: `file_locations`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetLocationFacet()`

### 4. Organization Facet ✅
- **Nombre canónico**: `organization`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Organizaciones mencionadas
- **Fuente de datos**: `file_organizations`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetOrganizationFacet()`

### 5. Content Type Facet ✅
- **Nombre canónico**: `content_type`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Tipo de contenido sugerido por AI
- **Fuente de datos**: `suggested_taxonomy.content_type`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetContentTypeFacet()`

### 6. Purpose Facet ✅
- **Nombre canónico**: `purpose`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Propósito sugerido por AI
- **Fuente de datos**: `suggested_taxonomy.purpose`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetPurposeFacet()`

### 7. Audience Facet ✅
- **Nombre canónico**: `audience`
- **Categoría**: Content
- **Tipo**: Terms
- **Descripción**: Audiencia sugerida por AI
- **Fuente de datos**: `suggested_taxonomy.audience`
- **Disponibilidad**: Conditional
- **Implementación**: `MetadataRepository.GetAudienceFacet()`

## Estado Actual de Facetas

### Total: 22 facetas implementadas

#### Core (8 facetas)
- ✅ extension
- ✅ type
- ✅ size
- ✅ indexing_status
- ✅ modified
- ✅ created
- ✅ accessed
- ✅ changed

#### Organization (2 facetas)
- ✅ tag
- ✅ project

#### Content (11 facetas) ⬆️ +7 nuevas
- ✅ language
- ✅ category
- ✅ author
- ✅ publication_year
- ✅ **sentiment** (nuevo)
- ✅ **duplicate_type** (nuevo)
- ✅ **location** (nuevo)
- ✅ **organization** (nuevo)
- ✅ **content_type** (nuevo)
- ✅ **purpose** (nuevo)
- ✅ **audience** (nuevo)

#### System (1 faceta)
- ✅ owner

## Archivos Modificados

1. **`backend/internal/domain/repository/metadata_repository.go`**
   - Agregados 7 nuevos métodos de interfaz

2. **`backend/internal/infrastructure/persistence/sqlite/metadata_repository.go`**
   - Implementados 7 nuevos métodos SQLite

3. **`backend/internal/application/query/facet_registry.go`**
   - Registradas 7 nuevas facetas en el registry

4. **`backend/internal/application/query/facets.go`**
   - Agregados 7 nuevos handlers al mapa de handlers

## Ejemplos de Uso

### Query Sentiment Facet
```go
facetReq := query.FacetRequest{
    Field: "sentiment",
    Type:  query.FacetTypeTerms,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
// Retorna: positive, negative, neutral, mixed con sus conteos
```

### Query Location Facet
```go
facetReq := query.FacetRequest{
    Field: "location",
    Type:  query.FacetTypeTerms,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
// Retorna: todas las ubicaciones mencionadas con sus conteos
```

### Query Content Type Facet
```go
facetReq := query.FacetRequest{
    Field: "content_type",
    Type:  query.FacetTypeTerms,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
// Retorna: tipos de contenido sugeridos por AI
```

## Próximas Facetas Sugeridas

Basado en el análisis, las siguientes facetas de alta prioridad aún no implementadas:

1. **Code Complexity** - Ranges de complejidad de código
2. **Project Assignment Score** - Rangos de score de asignación
3. **Temporal Pattern** - Patrones temporales (recent, archived, active, stale)
4. **Domain** - Dominio de taxonomía AI
5. **Subdomain** - Subdominio de taxonomía AI
6. **Topics** - Temas sugeridos (many-to-many)
7. **Location Type** - Tipo de ubicación (city, country, region)
8. **Organization Type** - Tipo de organización

## Notas Técnicas

- Todas las nuevas facetas usan `COUNT(DISTINCT file_id)` para evitar duplicados
- Se unen con la tabla `files` para validar que los archivos existen
- Soportan filtrado opcional por `fileIDs`
- Manejan valores NULL usando `COALESCE(..., 'unknown')`
- Ordenan resultados por conteo descendente

## Compatibilidad

✅ **100% Compatible** - No hay breaking changes
✅ **Todas las facetas funcionan con el sistema de registry existente**
✅ **Handlers automáticamente disponibles** a través del mapa de handlers



