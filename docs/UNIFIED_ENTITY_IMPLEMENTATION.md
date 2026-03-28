# Implementación del Modelo Unificado de Entidades

## Resumen

Se ha implementado el modelo unificado de entidades que permite que archivos, carpetas y proyectos sean tratados como entidades semánticamente equivalentes, permitiendo que las facetas los filtren de forma unificada.

## Componentes Implementados

### Backend (Go)

#### 1. ✅ Modelo Entity (`backend/internal/domain/entity/entity.go`)

- **EntityType**: Enum para tipos de entidad (`file`, `folder`, `project`)
- **EntityID**: Identificador unificado con tipo e ID
- **Entity**: Estructura unificada con:
  - Campos comunes (name, path, timestamps, size)
  - Metadata semántica unificada (tags, projects, language, category, author, owner, etc.)
  - Datos específicos por tipo (FileEntityData, FolderEntityData, ProjectEntityData)

#### 2. ✅ Conversiones

- **FromFileEntry**: Convierte `FileEntry` + `FileMetadata` → `Entity`
- **FromFolderEntry**: Convierte `FolderEntry` → `Entity`
- **FromProject**: Convierte `Project` → `Entity`
- **ToFileEntry**, **ToFolderEntry**, **ToProject**: Conversiones inversas

#### 3. ✅ EntityRepository Interface (`backend/internal/domain/repository/entity_repository.go`)

- `GetEntity`: Obtener entidad por ID
- `ListEntities`: Listar entidades con filtros
- `GetEntitiesByFacet`: Obtener entidades por valor de faceta
- `UpdateEntityMetadata`: Actualizar metadata de entidad
- `CountEntitiesByFacet`: Contar entidades por faceta

#### 4. ✅ Extensiones a Modelos Existentes

**ProjectAttributes** (`backend/internal/domain/entity/project.go`):
- Agregados campos: `Tags`, `Language`, `Author`, `Owner`, `Location`, `PublicationYear`, `AISummary`, `AIKeywords`

**FolderMetadata** (`backend/internal/domain/entity/folder.go`):
- Agregados campos: `Author`, `Owner`, `Location`, `PublicationYear`, `Status`, `Priority`, `Visibility`

### Frontend (TypeScript)

#### 1. ✅ Modelo Entity (`src/models/entity.ts`)

- **EntityType**: Tipo union `'file' | 'folder' | 'project'`
- **EntityID**: Interface con `type` e `id`
- **Entity**: Interface unificada con todos los campos semánticos
- **FileEntityData**, **FolderEntityData**, **ProjectEntityData**: Datos específicos por tipo
- **EntityFilters**: Interface para filtrar entidades
- **EntityMetadata**: Interface para actualizar metadata

## Características Semánticas Unificadas

Todas las entidades ahora pueden tener:

| Campo | Archivos | Carpetas | Proyectos |
|-------|----------|----------|-----------|
| **tags** | ✅ | ✅ | ✅ |
| **projects** | ✅ | ✅ | ✅ |
| **language** | ✅ | ✅ | ✅ |
| **category** | ✅ | ✅ | ✅ |
| **author** | ✅ | ✅ | ✅ |
| **owner** | ✅ | ✅ | ✅ |
| **location** | ✅ | ✅ | ✅ |
| **publicationYear** | ✅ | ✅ | ✅ |
| **status** | ✅ | ✅ | ✅ |
| **priority** | ✅ | ✅ | ✅ |
| **visibility** | ✅ | ✅ | ✅ |
| **aiSummary** | ✅ | ✅ | ✅ |
| **aiKeywords** | ✅ | ✅ | ✅ |
| **complexity** | ✅ | ✅ | ⚠️ (calcular) |
| **linesOfCode** | ✅ | ✅ | ⚠️ (calcular) |
| **qualityScore** | ✅ | ⚠️ (calcular) | ⚠️ (calcular) |

## Ejemplo de Uso

### Backend

```go
// Convertir archivo a entidad
file, _ := fileRepo.GetByPath(ctx, workspaceID, "docs/paper.pdf")
metadata, _ := metaRepo.GetByPath(ctx, workspaceID, "docs/paper.pdf")
entity := entity.FromFileEntry(workspaceID, file, metadata)

// Convertir carpeta a entidad
folder, _ := folderRepo.GetByPath(ctx, workspaceID, "research/")
entity := entity.FromFolderEntry(workspaceID, folder)

// Convertir proyecto a entidad
project, _ := projectRepo.GetByID(ctx, workspaceID, projectID)
entity := entity.FromProject(workspaceID, project, documentCount)

// Filtrar entidades por faceta
entities, _ := entityRepo.GetEntitiesByFacet(
    ctx, workspaceID, "tag", "research",
    []entity.EntityType{entity.EntityTypeFile, entity.EntityTypeFolder, entity.EntityTypeProject},
)
```

### Frontend

```typescript
// Obtener entidades por faceta
const entities = await entityClient.getEntitiesByFacet(
  workspaceId,
  'tag',
  'research',
  ['file', 'folder', 'project']
);

// Filtrar entidades
const filtered = await entityClient.listEntities(workspaceId, {
  tags: ['research'],
  language: 'es',
  projects: ['My Project'],
  types: ['file', 'folder', 'project']
});
```

## Próximos Pasos

### Pendiente

1. **Extender FacetExecutor** (`backend/internal/application/query/facet_executor.go`)
   - Agregar soporte para entidades en lugar de solo archivos
   - Permitir especificar tipos de entidad en facetas

2. **Implementar EntityRepository** (`backend/internal/infrastructure/persistence/sqlite/entity_repository.go`)
   - Implementar consultas unificadas sobre files, folders, projects
   - Optimizar para performance

3. **Crear gRPC Service** (`backend/api/proto/cortex/v1/entity.proto`)
   - Definir mensajes para Entity
   - Crear EntityService con métodos unificados

4. **Actualizar UnifiedFacetTreeProvider** (`src/views/UnifiedFacetTreeProvider.ts`)
   - Usar Entity en lugar de solo archivos
   - Mostrar iconos diferentes por tipo de entidad
   - Permitir filtrar por tipo de entidad

5. **Crear EntityClient** (`src/core/GrpcEntityClient.ts`)
   - Cliente gRPC para operaciones con entidades
   - Integrar con UnifiedFacetTreeProvider

## Impacto en Facetas

### Facetas que Ahora Funcionan para Todos los Tipos

Todas las facetas de términos ahora pueden filtrar archivos, carpetas Y proyectos:

- **tag**: Archivos, carpetas, proyectos
- **project**: Archivos, carpetas, proyectos (parent para proyectos)
- **language**: Archivos, carpetas, proyectos
- **category**: Archivos (AICategory), carpetas (ProjectNature), proyectos (Nature)
- **author**: Archivos, carpetas (inferido), proyectos
- **owner**: Archivos, carpetas (inferido), proyectos
- **location**: Archivos, carpetas (inferido), proyectos
- **publication_year**: Archivos, carpetas (inferido), proyectos
- **status**: Archivos (indexing), carpetas, proyectos
- **priority**: Archivos, carpetas, proyectos
- **visibility**: Archivos, carpetas, proyectos

### Facetas Numéricas

- **size**: Archivos, carpetas (TotalSize), proyectos (calcular)
- **complexity**: Archivos, carpetas (AverageComplexity), proyectos (calcular)
- **lines_of_code**: Archivos, carpetas (TotalLinesOfCode), proyectos (calcular)

### Facetas de Fecha

- **created**: Todos los tipos
- **modified/updated**: Todos los tipos

## Estructura de Datos

### EntityID Format

```
file:sha256hash
folder:sha256hash
project:uuid
```

### Entity Path

- **Files**: `relative/path/to/file.ext`
- **Folders**: `relative/path/to/folder/`
- **Projects**: `parent/child/grandchild` (hierarchical path)

## Compatibilidad

### Backward Compatibility

- Los modelos existentes (`FileEntry`, `FolderEntry`, `Project`) se mantienen intactos
- Las conversiones son unidireccionales (Entity → Original) para compatibilidad
- Los repositorios existentes siguen funcionando

### Migration Path

1. **Fase 1**: Modelo Entity creado ✅
2. **Fase 2**: EntityRepository implementado (pendiente)
3. **Fase 3**: Facetas actualizadas para usar entidades (pendiente)
4. **Fase 4**: Frontend actualizado para mostrar entidades unificadas (pendiente)
5. **Fase 5**: Migración gradual de código existente (pendiente)

## Testing

### Tests Necesarios

1. **Conversiones**
   - Test `FromFileEntry` con diferentes tipos de archivos
   - Test `FromFolderEntry` con carpetas vacías y con contenido
   - Test `FromProject` con diferentes natures
   - Test conversiones inversas

2. **EntityRepository**
   - Test `GetEntity` para cada tipo
   - Test `ListEntities` con diferentes filtros
   - Test `GetEntitiesByFacet` para diferentes facetas
   - Test `UpdateEntityMetadata`

3. **Facetas Unificadas**
   - Test facetas que funcionan para todos los tipos
   - Test conteos agregados por tipo
   - Test filtros combinados

## Notas de Implementación

### Campos Inferidos

Algunos campos se infieren de los contenidos:

- **Carpetas**: `author`, `owner`, `location`, `publicationYear` se infieren de los archivos contenidos
- **Proyectos**: `size`, `complexity`, `linesOfCode` se calculan de los documentos asociados

### Normalización

- **Category**: Se normaliza de diferentes fuentes:
  - Archivos: `AICategory.Category`
  - Carpetas: `FolderMetadata.ProjectNature`
  - Proyectos: `Project.Nature`

- **Projects**: Se normaliza de diferentes fuentes:
  - Archivos: `FileMetadata.Contexts`
  - Carpetas: `FolderMetadata.UserProject`
  - Proyectos: `Project.ParentID` (convertido a nombre)

## Estado

✅ **FASE 1 COMPLETA**: Modelo unificado implementado
- Modelo Entity creado (backend y frontend)
- Conversiones implementadas
- Extensiones a modelos existentes
- EntityRepository interface definido

✅ **FASE 2 COMPLETA**: Extensión de servicios
- FacetExecutor extendido con `ExecuteEntityFacet`
- Soporte para facetas de entidades (files, folders, projects)
- gRPC proto definido (`entity.proto`)

⏳ **FASE 3 PENDIENTE**: Implementación completa
- EntityRepository implementación SQLite
- gRPC service handler implementado
- UnifiedFacetTreeProvider actualizado
- EntityClient creado
- UI actualizada para mostrar tipos de entidad

