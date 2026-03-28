# Componentes que NO son Facetas

## Resumen

En el sistema Cortex, **conceptualmente todas las vistas que agrupan archivos son facetas**, excepto dos componentes que tienen propósitos diferentes:

## 1. FileInfoTreeProvider ❌ NO es Faceta

**Ubicación**: `src/views/FileInfoTreeProvider.ts`

**Propósito**: Panel de información detallada de un archivo específico

**Por qué NO es faceta**:
- ❌ No agrupa múltiples archivos
- ❌ Muestra información de UN solo archivo a la vez
- ❌ Es un panel de detalles, no una vista de agrupación
- ❌ No permite filtrar/navegar por atributos

**Características**:
- Muestra información completa del archivo actualmente seleccionado
- Incluye: metadata, tags, proyectos, traces, sugerencias AI, etc.
- Vista separada: `cortex-fileInfoView`
- Se actualiza cuando cambia el archivo seleccionado

**Ejemplo de uso**:
```
Usuario selecciona: paper.pdf
FileInfo muestra:
  - Metadata básica
  - Tags: [research, academic]
  - Proyectos: [Research Project]
  - Traces de AI
  - Sugerencias
```

## 2. CortexTreeProvider ❌ NO es Faceta

**Ubicación**: `src/views/CortexTreeProvider.ts`

**Propósito**: Organizador/contenedor que estructura el árbol principal

**Por qué NO es faceta**:
- ❌ No agrupa archivos por atributos
- ❌ Es un "meta-provider" que organiza secciones
- ❌ Delega a otros providers (que SÍ son facetas)
- ❌ No filtra archivos directamente

**Características**:
- Organiza las secciones principales: Navegar, Organizar, Buscar, Analizar, Revisar
- Gestiona la jerarquía del árbol
- Delega a `UnifiedFacetTreeProvider` y otros providers
- Vista principal: `cortex-mainView`

**Estructura**:
```
CortexTreeProvider (organizador)
├── Navegar
│   └── UnifiedFacetTreeProvider (✅ faceta)
├── Organizar
│   └── UnifiedFacetTreeProvider (✅ faceta)
├── Buscar
│   └── UnifiedFacetTreeProvider (✅ faceta)
├── Analizar
│   └── UnifiedFacetTreeProvider (✅ faceta)
└── Revisar
    └── UnifiedFacetTreeProvider (✅ faceta)
```

## Todos los Demás Providers SÍ son Facetas ✅

Todos los demás providers en `src/views/` son facetas porque:

1. ✅ Agrupan archivos por un atributo específico
2. ✅ Permiten filtrar y navegar
3. ✅ Muestran múltiples archivos organizados

### Lista de Providers que SÍ son Facetas:

#### Providers Estándar (BaseFacetTreeProvider)
- ✅ `TermsFacetTreeProvider` - Facetas de términos
- ✅ `NumericRangeFacetTreeProvider` - Facetas numéricas
- ✅ `DateRangeFacetTreeProvider` - Facetas de fecha

#### Providers Especializados
- ✅ `FolderTreeProvider` - Faceta de estructura de carpetas
- ✅ `CodeMetricsTreeProvider` - Faceta de métricas de código
- ✅ `DocumentMetricsTreeProvider` - Faceta de métricas de documentos
- ✅ `IssuesTreeProvider` - Faceta de issues y TODOs
- ✅ `MetadataClassificationTreeProvider` - Faceta de tipos de metadatos

#### Providers de Categorías
- ✅ `WritingCategoryTreeProvider` - Faceta de categoría escritura
- ✅ `CollectionCategoryTreeProvider` - Faceta de categoría colecciones
- ✅ `DevelopmentCategoryTreeProvider` - Faceta de categoría desarrollo
- ✅ `ManagementCategoryTreeProvider` - Faceta de categoría gestión
- ✅ `HierarchicalCategoryTreeProvider` - Faceta de categoría jerárquica
- ✅ `ProjectTaxonomyTreeProvider` - Faceta de taxonomía de proyectos

#### Provider Unificado
- ✅ `UnifiedFacetTreeProvider` - Genera todas las facetas dinámicamente

## Criterios para Identificar si es Faceta

### ✅ ES Faceta si:
1. Agrupa archivos por un atributo o criterio específico
2. Permite navegar y filtrar archivos por ese atributo
3. Muestra múltiples archivos organizados por el atributo

### ❌ NO es Faceta si:
1. Muestra información de UN solo archivo (panel de detalles)
2. Solo organiza/estructura otros providers (contenedor)
3. No agrupa archivos por atributos

## Resumen Visual

```
Cortex Extension
│
├── cortex-mainView (CortexTreeProvider) ❌ NO es faceta
│   └── Organizador que contiene:
│       ├── UnifiedFacetTreeProvider ✅ Faceta
│       └── (otros providers) ✅ Facetas
│
└── cortex-fileInfoView (FileInfoTreeProvider) ❌ NO es faceta
    └── Panel de detalles de un archivo
```

## Conclusión

**Total de componentes**:
- ✅ **Facetas**: ~20+ providers (todos excepto los 2 mencionados)
- ❌ **NO Facetas**: 2 componentes
  - `FileInfoTreeProvider` (panel de detalles)
  - `CortexTreeProvider` (organizador)

**Conceptualmente**: El 95%+ de las vistas son facetas. Solo hay 2 excepciones con propósitos diferentes (detalles y organización).


