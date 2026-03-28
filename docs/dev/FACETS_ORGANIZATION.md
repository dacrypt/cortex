# Organización de Facetas - Análisis y Reestructuración

## Problemas Identificados

### 1. Redundancias de Nombres (Aliases Duplicados)
Actualmente hay múltiples nombres para la misma faceta:
- `type` / `file_type` → Mismo concepto
- `context` / `project` → Mismo concepto  
- `language` / `detected_language` → Mismo concepto
- `ai_category` / `category` → Mismo concepto
- `author` / `authors` → Mismo concepto
- `publication_year` / `year` → Mismo concepto
- `owner` / `owners` → Mismo concepto
- `indexing_status` / `index_status` → Mismo concepto
- `last_modified` / `modified` / `mtime` → Mismo concepto
- `created_at` / `created` → Mismo concepto
- `accessed_at` / `accessed` → Mismo concepto
- `changed_at` / `changed` → Mismo concepto
- `file_size` / `size` → Mismo concepto

### 2. Solapamientos Conceptuales
- **Extension vs Type**: Ambos agrupan por tipo de archivo, pero `type` es una inferencia y `extension` es literal
- **Múltiples facetas de fecha**: Todas usan el mismo patrón de rangos temporales
- **Autor vs People**: Diferentes tablas pero concepto similar

### 3. Falta de Categorización
No hay una estructura clara que agrupe facetas relacionadas.

## Propuesta de Reorganización

### Estructura Propuesta

```
Facetas
├── Core (Siempre disponibles)
│   ├── extension
│   ├── size (ranges)
│   ├── dates (ranges)
│   └── indexing_status
│
├── Organización (User-assigned)
│   ├── tag
│   ├── project (alias: context)
│   └── folder
│
├── Temporal (Date ranges)
│   ├── modified (alias: last_modified, mtime)
│   ├── created (alias: created_at)
│   ├── accessed (alias: accessed_at)
│   └── changed (alias: changed_at)
│
├── Contenido (AI/LLM extracted)
│   ├── language (alias: detected_language)
│   ├── category (alias: ai_category)
│   ├── author (alias: authors)
│   ├── publication_year (alias: year)
│   └── content_type
│
├── Sistema (OS metadata)
│   ├── owner (alias: owners)
│   ├── permissions
│   └── filesystem
│
└── Especializadas (Media-specific)
    ├── image_*
    ├── audio_*
    └── video_*
```

## Plan de Consolidación

### Fase 1: Estandarizar Nombres Principales

**Regla**: Un solo nombre canónico por faceta, con aliases opcionales para compatibilidad.

| Nombre Canónico | Aliases Permitidos | Tipo |
|----------------|-------------------|------|
| `extension` | - | Terms |
| `type` | `file_type` | Terms |
| `size` | `file_size` | NumericRange |
| `tag` | - | Terms |
| `project` | `context` | Terms |
| `language` | `detected_language` | Terms |
| `category` | `ai_category` | Terms |
| `author` | `authors` | Terms |
| `publication_year` | `year` | Terms |
| `owner` | `owners` | Terms |
| `indexing_status` | `index_status` | Terms |
| `modified` | `last_modified`, `mtime` | DateRange |
| `created` | `created_at` | DateRange |
| `accessed` | `accessed_at` | DateRange |
| `changed` | `changed_at` | DateRange |

### Fase 2: Consolidar Lógica Duplicada

#### Consolidar Extension vs Type
**Problema**: `type` actualmente usa `GetExtensionFacet()` como proxy.

**Solución**: 
- `extension`: Agrupa por extensión literal (`.ts`, `.go`, `.pdf`)
- `type`: Agrupa por tipo inferido (`typescript`, `go`, `pdf`) - requiere implementación real

#### Unificar Facetas de Fecha
**Problema**: Todas las facetas de fecha usan el mismo patrón.

**Solución**: Crear función helper genérica `getDateRangeFacetForColumn()` (ya implementada) y usarla para todas.

### Fase 3: Organizar por Categorías

Crear estructura de metadatos para cada faceta:

```go
type FacetDefinition struct {
    CanonicalName string
    Aliases       []string
    Category      FacetCategory
    Type          FacetType
    Description   string
    DataSource    string
    Availability  AvailabilityLevel
}

type FacetCategory string

const (
    FacetCategoryCore         FacetCategory = "core"
    FacetCategoryOrganization FacetCategory = "organization"
    FacetCategoryTemporal     FacetCategory = "temporal"
    FacetCategoryContent      FacetCategory = "content"
    FacetCategorySystem       FacetCategory = "system"
    FacetCategorySpecialized  FacetCategory = "specialized"
)

type AvailabilityLevel string

const (
    AvailabilityAlways      AvailabilityLevel = "always"      // Todos los archivos
    AvailabilityConditional AvailabilityLevel = "conditional" // Solo algunos archivos
    AvailabilityRare        AvailabilityLevel = "rare"        // Muy pocos archivos
)
```

## Implementación Propuesta

### 1. Crear Registry de Facetas

```go
// facets/registry.go
package facets

type Registry struct {
    facets map[string]*FacetDefinition
}

func NewRegistry() *Registry {
    r := &Registry{
        facets: make(map[string]*FacetDefinition),
    }
    r.registerAll()
    return r
}

func (r *Registry) registerAll() {
    // Core
    r.register(&FacetDefinition{
        CanonicalName: "extension",
        Category:      FacetCategoryCore,
        Type:          FacetTypeTerms,
        Description:   "File extension",
        DataSource:    "files.extension",
        Availability:  AvailabilityAlways,
    })
    
    r.register(&FacetDefinition{
        CanonicalName: "type",
        Aliases:       []string{"file_type"},
        Category:      FacetCategoryCore,
        Type:          FacetTypeTerms,
        Description:   "Inferred file type",
        DataSource:    "files.enhanced (inferred)",
        Availability:  AvailabilityAlways,
    })
    
    // ... más facetas
}

func (r *Registry) Resolve(field string) (*FacetDefinition, error) {
    // Primero busca por nombre canónico
    if def, ok := r.facets[field]; ok {
        return def, nil
    }
    
    // Luego busca por alias
    for _, def := range r.facets {
        for _, alias := range def.Aliases {
            if alias == field {
                return def, nil
            }
        }
    }
    
    return nil, fmt.Errorf("unknown facet: %s", field)
}
```

### 2. Refactorizar FacetExecutor

```go
// query/facets.go
func (e *FacetExecutor) ExecuteFacet(
    ctx context.Context,
    workspaceID entity.WorkspaceID,
    req FacetRequest,
    fileIDs []entity.FileID,
) (*FacetResult, error) {
    // Resolver nombre canónico
    registry := facets.NewRegistry()
    def, err := registry.Resolve(req.Field)
    if err != nil {
        return nil, err
    }
    
    // Usar nombre canónico
    req.Field = def.CanonicalName
    
    // Ejecutar según tipo
    switch def.Type {
    case FacetTypeTerms:
        return e.executeTermsFacet(ctx, workspaceID, def, fileIDs)
    case FacetTypeNumericRange:
        return e.executeNumericRangeFacet(ctx, workspaceID, def, fileIDs)
    case FacetTypeDateRange:
        return e.executeDateRangeFacet(ctx, workspaceID, def, fileIDs)
    }
}
```

### 3. Simplificar Switch Statements

En lugar de múltiples `case` statements, usar un mapa de handlers:

```go
type TermsFacetHandler func(ctx context.Context, workspaceID entity.WorkspaceID, fileIDs []entity.FileID) (map[string]int, error)

var termsFacetHandlers = map[string]TermsFacetHandler{
    "extension": func(ctx, wsID, fileIDs) {
        return e.fileRepo.GetExtensionFacet(ctx, wsID, fileIDs)
    },
    "tag": func(ctx, wsID, fileIDs) {
        return e.metaRepo.GetTagCounts(ctx, wsID)
    },
    // ...
}
```

## Beneficios

1. **Menos Código Duplicado**: Un solo lugar para manejar aliases
2. **Mejor Organización**: Facetas agrupadas por categoría
3. **Más Mantenible**: Agregar nuevas facetas es más simple
4. **Mejor Documentación**: Metadata describe cada faceta
5. **Validación Centralizada**: Un solo lugar para validar nombres
6. **Extensibilidad**: Fácil agregar nuevas categorías o tipos

## Migración

### Compatibilidad hacia atrás
- Mantener todos los aliases funcionando
- Agregar warnings en logs cuando se usen aliases (opcional)
- Documentar nombres canónicos preferidos

### Pasos
1. Crear `FacetRegistry` con todas las facetas
2. Refactorizar `FacetExecutor` para usar registry
3. Simplificar handlers con mapas
4. Actualizar documentación
5. (Opcional) Deprecar aliases en futuras versiones

## Facetas por Categoría (Propuesta Final)

### Core (8 facetas)
- `extension` - Extensión de archivo
- `type` - Tipo inferido
- `size` - Tamaño (ranges)
- `indexing_status` - Estado de indexación
- `modified` - Última modificación (ranges)
- `created` - Fecha de creación (ranges)
- `accessed` - Último acceso (ranges)
- `changed` - Cambio de metadata (ranges)

### Organization (3 facetas)
- `tag` - Tags del usuario
- `project` - Proyectos/contextos
- `folder` - Carpetas

### Content (5 facetas)
- `language` - Idioma detectado
- `category` - Categoría AI
- `author` - Autores
- `publication_year` - Año de publicación
- `content_type` - Tipo de contenido

### System (3 facetas)
- `owner` - Propietario del archivo
- `permissions` - Permisos (futuro)
- `filesystem` - Sistema de archivos (futuro)

### Specialized (futuro)
- `image_*` - Facetas específicas de imágenes
- `audio_*` - Facetas específicas de audio
- `video_*` - Facetas específicas de video

## Resumen de Cambios

1. ✅ **Estandarizar nombres**: Un nombre canónico por faceta
2. ✅ **Consolidar lógica**: Eliminar código duplicado
3. ✅ **Categorizar**: Agrupar facetas relacionadas
4. ✅ **Crear registry**: Centralizar definiciones
5. ✅ **Simplificar handlers**: Usar mapas en lugar de switches largos
6. ✅ **Documentar**: Metadata rica para cada faceta



