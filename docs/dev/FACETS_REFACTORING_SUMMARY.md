# Facets Refactoring Summary

## Cambios Realizados

### 1. Creación de FacetRegistry

Se creó un sistema centralizado de registro de facetas en `backend/internal/application/query/facet_registry.go` que:

- **Define nombres canónicos** para cada faceta
- **Soporta aliases** para compatibilidad hacia atrás
- **Categoriza facetas** por tipo (Core, Organization, Temporal, Content, System, Specialized)
- **Proporciona metadata** sobre cada faceta (descripción, fuente de datos, disponibilidad)

### 2. Consolidación de Aliases

Se eliminaron redundancias en el código consolidando múltiples nombres para la misma faceta:

| Nombre Canónico | Aliases Consolidados |
|----------------|---------------------|
| `extension` | - |
| `type` | `file_type` |
| `size` | `file_size` |
| `tag` | - |
| `project` | `context` |
| `language` | `detected_language` |
| `category` | `ai_category` |
| `author` | `authors` |
| `publication_year` | `year` |
| `owner` | `owners` |
| `indexing_status` | `index_status` |
| `modified` | `last_modified`, `mtime` |
| `created` | `created_at` |
| `accessed` | `accessed_at` |
| `changed` | `changed_at` |

### 3. Refactorización de FacetExecutor

El `FacetExecutor` ahora:
- Usa el `FacetRegistry` para resolver nombres de facetas
- Normaliza automáticamente aliases a nombres canónicos
- Simplifica el código eliminando casos duplicados en los switch statements

### 4. Organización por Categorías

Las facetas están organizadas en las siguientes categorías:

#### Core (8 facetas)
- `extension` - Extensión de archivo
- `type` - Tipo inferido
- `size` - Tamaño (ranges)
- `indexing_status` - Estado de indexación
- `modified` - Última modificación (ranges)
- `created` - Fecha de creación (ranges)
- `accessed` - Último acceso (ranges)
- `changed` - Cambio de metadata (ranges)

#### Organization (2 facetas)
- `tag` - Tags del usuario
- `project` - Proyectos/contextos

#### Content (4 facetas)
- `language` - Idioma detectado
- `category` - Categoría AI
- `author` - Autores
- `publication_year` - Año de publicación

#### System (1 faceta)
- `owner` - Propietario del archivo

## Beneficios

1. **Menos Código Duplicado**: Un solo lugar para manejar aliases
2. **Mejor Organización**: Facetas agrupadas por categoría
3. **Más Mantenible**: Agregar nuevas facetas es más simple
4. **Mejor Documentación**: Metadata describe cada faceta
5. **Validación Centralizada**: Un solo lugar para validar nombres
6. **Compatibilidad**: Todos los aliases siguen funcionando

## Compatibilidad

✅ **Todos los aliases siguen funcionando** - No hay breaking changes
✅ **El código existente no necesita cambios** - La normalización es transparente
✅ **Nuevas facetas pueden agregarse fácilmente** - Solo registrar en el registry

## Próximos Pasos

1. Implementar facetas especializadas (image_*, audio_*, video_*)
2. Agregar facetas de sistema (permissions, filesystem)
3. Implementar facetas de contenido adicionales (content_type, purpose, audience)
4. Crear API para listar todas las facetas disponibles por categoría

## Archivos Modificados

- `backend/internal/application/query/facet_registry.go` (nuevo)
- `backend/internal/application/query/facets.go` (refactorizado)
- `docs/FACETS_ORGANIZATION.md` (nuevo - plan de organización)
- `docs/FACETS_REFACTORING_SUMMARY.md` (este archivo)



