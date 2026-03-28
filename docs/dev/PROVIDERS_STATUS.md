# Estado de los Providers de Vistas

## Providers Activos (En Uso)

### Providers Principales
- ✅ **UnifiedFacetTreeProvider** - Provider principal que genera todas las facetas dinámicamente
- ✅ **CortexTreeProvider** - Organizador del árbol principal (NO es faceta)
- ✅ **FileInfoTreeProvider** - Panel de detalles de archivo (NO es faceta)

### Providers Base (Usados por FacetProviderFactory)
- ✅ **TermsFacetTreeProvider** - Facetas de términos (usado por FacetProviderFactory)
- ✅ **NumericRangeFacetTreeProvider** - Facetas numéricas (usado por FacetProviderFactory)
- ✅ **DateRangeFacetTreeProvider** - Facetas de fecha (usado por FacetProviderFactory)
- ✅ **FolderTreeProvider** - Faceta de estructura de carpetas (usado por FacetProviderFactory)

## Providers Especializados (Obsoletos pero Mantenidos)

### Providers de Categoría
Estos providers están obsoletos pero se mantienen por compatibilidad y tests:
- ⚠️ **WritingCategoryTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **CollectionCategoryTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **DevelopmentCategoryTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **ManagementCategoryTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **HierarchicalCategoryTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **ProjectTaxonomyTreeProvider** - Reemplazado por UnifiedFacetTreeProvider

**Estado**: No se usan en `extension.ts`, pero:
- Se mantienen para tests
- Pueden ser usados en el futuro si se implementa CategoryFacetProvider unificado

### Providers de Métricas
- ⚠️ **CodeMetricsTreeProvider** - Reemplazado por UnifiedFacetTreeProvider
- ⚠️ **DocumentMetricsTreeProvider** - Reemplazado por UnifiedFacetTreeProvider

**Estado**: No se usan en `extension.ts`, pero:
- Se mantienen para tests
- Pueden ser usados en el futuro si se implementa MetricsFacetProvider unificado

### Providers de Issues y Metadata
- ⚠️ **IssuesTreeProvider** - Reemplazado por UnifiedFacetTreeProvider (usa TermsFacetTreeProvider como fallback)
- ⚠️ **MetadataClassificationTreeProvider** - Reemplazado por UnifiedFacetTreeProvider (usa TermsFacetTreeProvider como fallback)

**Estado**: No se usan en `extension.ts`, pero:
- Se mantienen para tests
- FacetProviderFactory los trata como TermsFacetTreeProvider

## Arquitectura Actual

### Flujo Principal
```
extension.ts
  └── UnifiedFacetTreeProvider (genera todo dinámicamente)
      └── FacetProviderFactory (crea providers bajo demanda)
          ├── TermsFacetTreeProvider
          ├── NumericRangeFacetTreeProvider
          ├── DateRangeFacetTreeProvider
          └── FolderTreeProvider
```

### Providers Obsoletos
Los providers especializados (Category, Metrics, Issues, Metadata) **NO se usan** en el flujo principal porque:
1. `FacetProviderFactory` lanza errores para Category y Metrics
2. `FacetProviderFactory` usa `TermsFacetTreeProvider` como fallback para Issues y Metadata

## Plan de Migración Futuro

### Fase 1: Unificar Providers de Categoría
- [ ] Crear `CategoryFacetProvider` unificado
- [ ] Reemplazar 6 providers de categoría individuales
- [ ] Actualizar `FacetProviderFactory` para usar el nuevo provider

### Fase 2: Unificar Providers de Métricas
- [ ] Crear `MetricsFacetProvider` unificado
- [ ] Reemplazar `CodeMetricsTreeProvider` y `DocumentMetricsTreeProvider`
- [ ] Actualizar `FacetProviderFactory` para usar el nuevo provider

### Fase 3: Unificar Providers de Issues y Metadata
- [ ] Crear `IssuesFacetProvider` unificado
- [ ] Crear `MetadataFacetProvider` unificado
- [ ] Actualizar `FacetProviderFactory` para usar los nuevos providers

### Fase 4: Eliminar Providers Obsoletos
- [ ] Una vez que todos los tipos estén unificados
- [ ] Eliminar providers individuales obsoletos
- [ ] Actualizar tests para usar providers unificados

## Recomendaciones

### Mantener por Ahora
- ✅ Mantener todos los providers para compatibilidad con tests
- ✅ Mantener providers especializados por si se necesitan en el futuro

### Limpiar
- ✅ Ya eliminados: archivos `.example`
- ✅ Ya limpiados: parámetros no usados en comandos
- ⚠️ Pendiente: Considerar marcar providers obsoletos con `@deprecated` JSDoc

### Documentar
- ✅ Este documento explica el estado de cada provider
- ✅ FacetProviderFactory tiene TODOs para futuras mejoras

## Resumen

- **Providers activos**: 7 (3 principales + 4 base)
- **Providers obsoletos pero mantenidos**: 10 (6 categoría + 2 métricas + 2 issues/metadata)
- **Total**: 17 providers

**Estado**: Arquitectura limpia con providers obsoletos mantenidos solo para compatibilidad y tests.


