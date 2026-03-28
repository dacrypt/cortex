# Plan de Refactorización de Facetas

## Objetivo

Refactorizar todas las facetas para tener homogeneidad en términos de estructura de software, interfaces, y patrones de implementación.

## Estado Actual

Actualmente tenemos múltiples providers con estructuras similares pero no idénticas:
- `TermsFacetTreeProvider` - Facetas de términos
- `NumericRangeFacetTreeProvider` - Facetas numéricas
- `DateRangeFacetTreeProvider` - Facetas de fecha
- `FolderTreeProvider` - Faceta de estructura
- `CategoryTreeProviders` (5 providers) - Facetas de categorías
- `CodeMetricsTreeProvider` - Faceta de métricas de código
- `DocumentMetricsTreeProvider` - Faceta de métricas de documentos
- `IssuesTreeProvider` - Faceta de issues
- `MetadataClassificationTreeProvider` - Faceta de metadatos

## Arquitectura Nueva

### 1. Interfaces Base (`src/views/contracts/IFacetProvider.ts`)

- `IFacetProvider<T>` - Interface base para todos los providers
- `IFacetTreeItem` - Interface base para todos los tree items
- `FacetConfig` - Configuración de facetas
- `FacetType` - Enum de tipos de facetas
- `FacetCategory` - Enum de categorías

### 2. Registro de Facetas (`src/views/contracts/FacetRegistry.ts`)

- `FacetRegistry` - Registro centralizado de todas las facetas
- Configuración de facetas con metadatos
- Resolución de aliases
- Agrupación por categoría y tipo

### 3. Clase Base (`src/views/base/BaseFacetTreeProvider.ts`)

- `BaseFacetTreeProvider<T>` - Clase base abstracta
- Funcionalidad común:
  - Event handling
  - Caching
  - File cache service integration
  - Common tree item creation
  - File URI creation
  - Activity sorting

### 4. Implementaciones Específicas

Cada tipo de faceta extiende `BaseFacetTreeProvider`:

- `TermsFacetTreeProvider` - Extiende base, implementa términos
- `NumericRangeFacetTreeProvider` - Extiende base, implementa rangos numéricos
- `DateRangeFacetTreeProvider` - Extiende base, implementa rangos de fecha
- `StructureFacetTreeProvider` - Extiende base, implementa estructura (folder)
- `CategoryFacetTreeProvider` - Extiende base, implementa categorías (genérico)
- `MetricsFacetTreeProvider` - Extiende base, implementa métricas (genérico)
- `IssuesFacetTreeProvider` - Extiende base, implementa issues
- `MetadataFacetTreeProvider` - Extiende base, implementa metadatos

## Plan de Implementación

### Fase 1: Infraestructura Base ✅

- [x] Crear interfaces base (`IFacetProvider.ts`)
- [x] Crear registro de facetas (`FacetRegistry.ts`)
- [x] Crear clase base (`BaseFacetTreeProvider.ts`)

### Fase 2: Refactorizar Providers Estándar

- [ ] Refactorizar `TermsFacetTreeProvider`
- [ ] Refactorizar `NumericRangeFacetTreeProvider`
- [ ] Refactorizar `DateRangeFacetTreeProvider`

### Fase 3: Refactorizar Providers Especializados

- [ ] Refactorizar `FolderTreeProvider` → `StructureFacetTreeProvider`
- [ ] Consolidar `CategoryTreeProviders` → `CategoryFacetTreeProvider` genérico
- [ ] Refactorizar `CodeMetricsTreeProvider` y `DocumentMetricsTreeProvider` → `MetricsFacetTreeProvider`
- [ ] Refactorizar `IssuesTreeProvider` → `IssuesFacetTreeProvider`
- [ ] Refactorizar `MetadataClassificationTreeProvider` → `MetadataFacetTreeProvider`

### Fase 4: Actualizar Integración

- [ ] Actualizar `extension.ts` para usar nuevas interfaces
- [ ] Actualizar registros de vistas
- [ ] Actualizar comandos relacionados

### Fase 5: Testing y Documentación

- [ ] Probar todas las facetas
- [ ] Actualizar documentación
- [ ] Actualizar ejemplos

## Beneficios

1. **Homogeneidad**: Todas las facetas siguen el mismo patrón
2. **Mantenibilidad**: Código común en un solo lugar
3. **Extensibilidad**: Fácil agregar nuevas facetas
4. **Type Safety**: Interfaces TypeScript fuertes
5. **Configuración Centralizada**: Todas las facetas en un registro
6. **Reutilización**: Funcionalidad común compartida

## Migración

La migración será gradual:
1. Crear nuevas implementaciones junto a las antiguas
2. Migrar una faceta a la vez
3. Probar cada migración
4. Eliminar código antiguo cuando todo esté migrado

## Notas

- Mantener compatibilidad con el backend existente
- No cambiar la API pública de VS Code
- Mantener el mismo comportamiento visual
- Asegurar que el rendimiento no se degrade


