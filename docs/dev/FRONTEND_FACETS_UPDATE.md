# Frontend Facets Update - Individual Views

## Resumen

Se han creado vistas individuales para todas las facetas numéricas y de fecha disponibles en el backend, permitiendo que cada faceta tenga su propia entrada en el árbol de navegación de Cortex.

## Cambios Realizados

### 1. TermsFacetTreeProvider Actualizado

Se actualizó `TermsFacetTreeProvider` para incluir todas las facetas de términos disponibles en el backend, organizadas en grupos lógicos:

- **Core Facets**: `extension`, `type`, `content_type`, `indexing_status`
- **Organization**: `tag`, `project`, `owner`
- **Content**: `language`, `category`, `author`, `publication_year`, `readability_level`
- **AI Taxonomy**: `domain`, `subdomain`, `topic`, `purpose`, `audience`
- **Enrichment**: `sentiment`, `location`, `organization`, `duplicate_type`
- **Temporal**: `temporal_pattern`

### 2. Nuevas Vistas Numéricas Creadas

Se crearon 5 nuevas instancias de `NumericRangeFacetTreeProvider` para las facetas numéricas:

1. **By Complexity** - `complexity`
   - Rangos: Very Low (0-5), Low (5-10), Medium (10-20), High (20-50), Very High (>50)
   - Fuente: `files.enhanced->CodeMetrics.Complexity`

2. **By Project Score** - `project_score`
   - Rangos: Very Low (0-0.2), Low (0.2-0.4), Medium (0.4-0.6), High (0.6-0.8), Very High (0.8-1.0)
   - Fuente: `project_assignments.score`

3. **By Function Count** - `function_count`
   - Rangos: None (0), Small (1-10), Medium (11-50), Large (51-200), Very Large (>200)
   - Fuente: `files.enhanced->CodeMetrics.FunctionCount`

4. **By Lines of Code** - `lines_of_code`
   - Rangos: Tiny (<100), Small (100-500), Medium (500-2000), Large (2000-10000), Very Large (>10000)
   - Fuente: `files.enhanced->CodeMetrics.LinesOfCode`

5. **By Comment Percentage** - `comment_percentage`
   - Rangos: None (0%), Low (0-10%), Medium (10-25%), High (25-50%), Very High (>50%)
   - Fuente: `files.enhanced->CodeMetrics.CommentPercentage`

### 3. Nuevas Vistas de Fecha Creadas

Se crearon 3 nuevas instancias de `DateRangeFacetTreeProvider` para las facetas de fecha:

1. **By Created Date** - `created`
   - Rangos: Today, Yesterday, This Week, This Month, This Year, Older
   - Fuente: `files.created_at`

2. **By Accessed Date** - `accessed`
   - Rangos: Today, Yesterday, This Week, This Month, This Year, Older
   - Fuente: `files.accessed_at`

3. **By Changed Date** - `changed`
   - Rangos: Today, Yesterday, This Week, This Month, This Year, Older
   - Fuente: `files.changed_at` (ctime)

## Estructura de la UI

Ahora la UI muestra todas las facetas disponibles organizadas así:

### Vistas Existentes
- By Project
- By Tag
- By Type
- By Date (modified)
- By Size
- By Folder
- By Content Type
- Code Metrics
- Documents
- Issues
- File Info
- Biblioteca
- Escritura y Creación
- Colecciones
- Desarrollo
- Gestión
- Jerárquico
- Por Metadatos

### Nuevas Vistas de Facetas
- **Terms Facets** - Selector con todas las facetas de términos
- **Numeric Range Facets** - Por tamaño (existente)
- **By Complexity** - Nueva
- **By Project Score** - Nueva
- **By Function Count** - Nueva
- **By Lines of Code** - Nueva
- **By Comment Percentage** - Nueva
- **Date Range Facets** - Por fecha de modificación (existente)
- **By Created Date** - Nueva
- **By Accessed Date** - Nueva
- **By Changed Date** - Nueva

## Archivos Modificados

1. `src/views/TermsFacetTreeProvider.ts`
   - Actualizado `fieldGroups` para incluir todas las facetas de términos del backend

2. `src/extension.ts`
   - Creadas 5 nuevas instancias de `NumericRangeFacetTreeProvider`
   - Creadas 3 nuevas instancias de `DateRangeFacetTreeProvider`
   - Agregadas todas las nuevas vistas al `CortexTreeProvider`
   - Agregadas todas las nuevas vistas a la función `refreshAllViews`

## Nombres de Campos

Todos los nombres de campos usan los nombres canónicos del backend:
- `size` (no `file_size`)
- `modified` (no `last_modified`)
- `created`, `accessed`, `changed`
- `complexity`, `project_score`, `function_count`, `lines_of_code`, `comment_percentage`

El backend acepta tanto nombres canónicos como aliases, pero usar los canónicos asegura consistencia.

## Estado Final

- **32 facetas implementadas en el backend** ✅
- **Todas las facetas disponibles en la UI** ✅
- **Vistas individuales para cada faceta numérica y de fecha** ✅
- **Selector de campos para facetas de términos** ✅


