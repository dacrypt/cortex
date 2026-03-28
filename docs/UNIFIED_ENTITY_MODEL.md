# Modelo Unificado de Entidades - Archivos, Carpetas y Proyectos

## Análisis de Características Comunes

### Características Semánticas Compartidas

Todas las entidades (archivos, carpetas, proyectos) pueden tener las mismas características semánticas:

| Característica | Archivos | Carpetas | Proyectos | Notas |
|----------------|----------|----------|-----------|-------|
| **Tags** | ✅ `FileMetadata.Tags` | ✅ `FolderMetadata.UserTags` | ⚠️ No implementado | Todos pueden tener tags |
| **Proyectos asignados** | ✅ `FileMetadata.Contexts` | ✅ `FolderMetadata.UserProject` | ✅ `Project.ParentID` | Todos tienen relaciones de proyecto |
| **Idioma** | ✅ `EnhancedMetadata.Language` | ✅ `FolderMetadata.DominantLanguage` | ⚠️ No implementado | Todos pueden tener idioma |
| **Categoría/Nature** | ✅ `FileMetadata.AICategory` | ✅ `FolderMetadata.ProjectNature` | ✅ `Project.Nature` | Todos tienen naturaleza/tipo |
| **Autor** | ✅ `DocumentMetrics.Author` | ⚠️ Inferido de archivos | ⚠️ No implementado | Todos pueden tener autor |
| **Fecha creación** | ✅ `FileEntry.CreatedAt` | ✅ `FolderEntry.CreatedAt` | ✅ `Project.CreatedAt` | Todos tienen timestamps |
| **Fecha modificación** | ✅ `FileEntry.LastModified` | ✅ `FolderEntry.UpdatedAt` | ✅ `Project.UpdatedAt` | Todos tienen timestamps |
| **Tamaño** | ✅ `FileEntry.FileSize` | ✅ `FolderMetrics.TotalSize` | ⚠️ Podría calcularse | Todos pueden tener tamaño |
| **Tipo/Extension** | ✅ `FileEntry.Extension` | ✅ `FolderMetadata.DominantFileType` | ⚠️ No aplicable | Archivos y carpetas tienen tipo |
| **Ruta/Path** | ✅ `FileEntry.RelativePath` | ✅ `FolderEntry.RelativePath` | ✅ `Project.Path` | Todos tienen ruta |
| **Estado/Status** | ✅ `IndexedState` | ⚠️ No implementado | ✅ `ProjectAttributes.Status` | Todos pueden tener estado |
| **Prioridad** | ⚠️ No implementado | ⚠️ No implementado | ✅ `ProjectAttributes.Priority` | Todos pueden tener prioridad |
| **Visibilidad** | ⚠️ No implementado | ⚠️ No implementado | ✅ `ProjectAttributes.Visibility` | Todos pueden tener visibilidad |
| **Descripción/Resumen** | ✅ `FileMetadata.AISummary` | ✅ `FolderMetadata.AISummary` | ✅ `Project.Description` | Todos pueden tener descripción |
| **Keywords** | ✅ `FileMetadata.AIKeywords` | ✅ `FolderMetadata.AIKeywords` | ⚠️ No implementado | Todos pueden tener keywords |
| **Owner** | ✅ `OSMetadata.Owner` | ⚠️ Podría inferirse | ⚠️ No implementado | Todos pueden tener propietario |
| **Location** | ✅ `ImageMetadata.GPSLocation` | ⚠️ Podría inferirse | ⚠️ No implementado | Todos pueden tener ubicación |
| **Organization** | ✅ `AIContext.Organizations` | ⚠️ Podría inferirse | ⚠️ No implementado | Todos pueden tener organización |
| **Publication Year** | ✅ `DocumentMetrics.CreatedDate` | ⚠️ Podría inferirse | ⚠️ No implementado | Todos pueden tener año |
| **Sentiment** | ✅ `EnrichmentData.Sentiment` | ⚠️ Podría agregarse | ⚠️ No implementado | Todos pueden tener sentimiento |
| **Complexity** | ✅ `CodeMetrics.Complexity` | ✅ `FolderMetrics.AverageComplexity` | ⚠️ Podría calcularse | Todos pueden tener complejidad |
| **Lines of Code** | ✅ `CodeMetrics.LinesOfCode` | ✅ `FolderMetrics.TotalLinesOfCode` | ⚠️ Podría calcularse | Todos pueden tener LOC |

## Propuesta: Modelo Unificado de Entidades

### Entity Interface (Backend Go)

```go
// EntityType represents the type of entity
type EntityType string

const (
    EntityTypeFile    EntityType = "file"
    EntityTypeFolder EntityType = "folder"
    EntityTypeProject EntityType = "project"
)

// EntityID is a unified identifier for any entity
type EntityID struct {
    Type EntityType
    ID   string // FileID, FolderID, or ProjectID
}

// Entity represents a unified view of files, folders, and projects
type Entity struct {
    // Identity
    ID          EntityID
    Type        EntityType
    WorkspaceID WorkspaceID
    
    // Basic information
    Name        string
    Path        string // Relative path for files/folders, hierarchical path for projects
    Description *string
    
    // Timestamps
    CreatedAt   time.Time
    UpdatedAt   time.Time
    ModifiedAt  *time.Time // For files
    
    // Size (for files and folders)
    Size        *int64
    
    // Semantic metadata (unified)
    Tags        []string
    Projects    []string // Assigned project names/IDs
    Language    *string
    Category    *string // AI category or nature
    Author      *string
    Owner       *string
    Location    *string
    
    // Temporal
    PublicationYear *int
    
    // Quality metrics
    Complexity      *float64
    LinesOfCode    *int
    QualityScore   *float64
    
    // Status
    Status      *string // indexing status, project status, etc.
    Priority    *string
    Visibility  *string
    
    // AI metadata
    AISummary   *string
    AIKeywords  []string
    
    // Type-specific data (preserved for compatibility)
    FileData    *FileEntityData
    FolderData  *FolderEntityData
    ProjectData *ProjectEntityData
}

// FileEntityData contains file-specific data
type FileEntityData struct {
    Extension      string
    MimeType      *string
    ContentType   *string
    CodeMetrics   *CodeMetrics
    DocumentMetrics *DocumentMetrics
    ImageMetadata *ImageMetadata
    AudioMetadata *AudioMetadata
    VideoMetadata *VideoMetadata
    IndexedState  IndexedState
}

// FolderEntityData contains folder-specific data
type FolderEntityData struct {
    Depth           int
    TotalFiles      int
    DirectFiles     int
    Subfolders      int
    FolderMetrics   *FolderMetrics
    DominantFileType *string
}

// ProjectEntityData contains project-specific data
type ProjectEntityData struct {
    Nature      ProjectNature
    Attributes  *ProjectAttributes
    ParentID    *ProjectID
    DocumentCount int
}
```

### Entity Interface (Frontend TypeScript)

```typescript
export type EntityType = 'file' | 'folder' | 'project';

export interface EntityID {
  type: EntityType;
  id: string;
}

export interface Entity {
  // Identity
  id: EntityID;
  type: EntityType;
  workspaceId: string;
  
  // Basic information
  name: string;
  path: string;
  description?: string;
  
  // Timestamps
  createdAt: number;
  updatedAt: number;
  modifiedAt?: number;
  
  // Size
  size?: number;
  
  // Semantic metadata (unified - can be filtered by facets)
  tags?: string[];
  projects?: string[];
  language?: string;
  category?: string;
  author?: string;
  owner?: string;
  location?: string;
  publicationYear?: number;
  
  // Quality metrics
  complexity?: number;
  linesOfCode?: number;
  qualityScore?: number;
  
  // Status
  status?: string;
  priority?: string;
  visibility?: string;
  
  // AI metadata
  aiSummary?: string;
  aiKeywords?: string[];
  
  // Type-specific data
  fileData?: FileEntityData;
  folderData?: FolderEntityData;
  projectData?: ProjectEntityData;
}

export interface FileEntityData {
  extension?: string;
  mimeType?: string;
  contentType?: string;
  codeMetrics?: CodeMetrics;
  documentMetrics?: DocumentMetrics;
  indexedState?: IndexedState;
}

export interface FolderEntityData {
  depth?: number;
  totalFiles?: number;
  directFiles?: number;
  subfolders?: number;
  dominantFileType?: string;
}

export interface ProjectEntityData {
  nature?: string;
  attributes?: ProjectAttributes;
  parentId?: string;
  documentCount?: number;
}
```

## Beneficios del Modelo Unificado

### 1. Facetas Unificadas

Las facetas pueden filtrar **cualquier tipo de entidad**:

```typescript
// Faceta de tags - funciona para archivos, carpetas Y proyectos
By Tag: "research"
  ├── file: paper.pdf
  ├── folder: research/
  └── project: Research Project

// Faceta de idioma - funciona para archivos, carpetas Y proyectos
By Language: "es"
  ├── file: documento.pdf
  ├── folder: documentos/
  └── project: Proyecto Español

// Faceta de proyecto - funciona para archivos, carpetas Y proyectos
By Project: "My Project"
  ├── file: file1.ts
  ├── folder: src/
  └── project: Subproject
```

### 2. Búsqueda Unificada

```typescript
// Buscar por cualquier faceta, sin importar el tipo
searchEntities({
  tags: ['research'],
  language: 'es',
  project: 'My Project'
})
// Retorna: archivos, carpetas Y proyectos que coincidan
```

### 3. Organización Unificada

```typescript
// Organizar por cualquier faceta
organizeByFacet('author')
// Muestra:
// - Archivos del autor
// - Carpetas con contenido del autor
// - Proyectos del autor
```

## Implementación Propuesta

### Fase 1: Backend - Entity Repository

```go
// EntityRepository provides unified access to files, folders, and projects
type EntityRepository interface {
    // Get entity by ID
    GetEntity(ctx context.Context, workspaceID WorkspaceID, id EntityID) (*Entity, error)
    
    // List entities with filters
    ListEntities(ctx context.Context, workspaceID WorkspaceID, filters EntityFilters) ([]*Entity, error)
    
    // Get entities by facet value
    GetEntitiesByFacet(ctx context.Context, workspaceID WorkspaceID, facet string, value string) ([]*Entity, error)
    
    // Update entity metadata
    UpdateEntityMetadata(ctx context.Context, workspaceID WorkspaceID, id EntityID, metadata EntityMetadata) error
}

// EntityFilters for querying
type EntityFilters struct {
    Types       []EntityType // file, folder, project
    Tags        []string
    Projects    []string
    Language    *string
    Category    *string
    Author      *string
    Owner       *string
    // ... más filtros
}
```

### Fase 2: Backend - Entity Facet Queries

```go
// Extender FacetExecutor para soportar entidades
func (e *FacetExecutor) ExecuteEntityFacet(
    ctx context.Context,
    workspaceID WorkspaceID,
    req FacetRequest,
    entityTypes []EntityType, // []EntityType{EntityTypeFile, EntityTypeFolder, EntityTypeProject}
) (*FacetResult, error) {
    // Query unificado que incluye archivos, carpetas y proyectos
    // Retorna conteos agregados de todos los tipos
}
```

### Fase 3: Frontend - Unified Entity Provider

```typescript
// UnifiedEntityFacetProvider - reemplaza UnifiedFacetTreeProvider
export class UnifiedEntityFacetProvider extends BaseFacetTreeProvider<EntityFacetTreeItem> {
  async getChildren(element?: EntityFacetTreeItem): Promise<EntityFacetTreeItem[]> {
    // Root: categorías
    // Categoría: facetas
    // Faceta: valores
    // Valor: ENTIDADES (archivos, carpetas, proyectos mezclados)
  }
  
  private async getEntitiesForFacetValue(
    field: string,
    value: string
  ): Promise<EntityFacetTreeItem[]> {
    // Query unificado que retorna archivos, carpetas Y proyectos
    const entities = await this.entityClient.getEntitiesByFacet(
      this.workspaceId,
      field,
      value,
      ['file', 'folder', 'project'] // Todos los tipos
    );
    
    return entities.map(entity => this.createEntityItem(entity));
  }
}
```

## Características Semánticas a Implementar

### Para Proyectos (faltantes)
- ✅ Nature (ya existe)
- ⚠️ Tags (agregar)
- ⚠️ Language (agregar)
- ⚠️ Author (agregar)
- ⚠️ Location (agregar)
- ⚠️ Organization (agregar)
- ⚠️ Publication Year (agregar)
- ⚠️ AISummary (agregar)
- ⚠️ AIKeywords (agregar)
- ⚠️ Size (calcular de documentos)

### Para Carpetas (faltantes)
- ✅ UserTags (ya existe)
- ✅ DominantLanguage (ya existe)
- ⚠️ Author (inferir de archivos)
- ⚠️ Location (inferir de archivos)
- ⚠️ Organization (inferir de archivos)
- ⚠️ Publication Year (inferir de archivos)
- ⚠️ Status (agregar)
- ⚠️ Priority (agregar)
- ⚠️ Visibility (agregar)

### Normalización de Campos

Todos los campos semánticos deben normalizarse:

```go
// EntityMetadata - campos unificados
type EntityMetadata struct {
    Tags            []string
    Projects        []string
    Language        *string
    Category        *string // Normalizado: AI category, folder nature, project nature
    Author          *string
    Owner           *string
    Location        *string
    PublicationYear *int
    Status          *string
    Priority        *string
    Visibility      *string
    AISummary       *string
    AIKeywords      []string
    // ... más campos comunes
}
```

## Estructura de Base de Datos Propuesta

### Tabla Unificada (Opcional)

```sql
CREATE TABLE entities (
    id TEXT PRIMARY KEY, -- "file:hash" | "folder:hash" | "project:uuid"
    workspace_id TEXT NOT NULL,
    type TEXT NOT NULL, -- 'file', 'folder', 'project'
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    description TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    modified_at INTEGER,
    size INTEGER,
    
    -- Semantic metadata (JSON)
    tags TEXT, -- JSON array
    projects TEXT, -- JSON array
    language TEXT,
    category TEXT,
    author TEXT,
    owner TEXT,
    location TEXT,
    publication_year INTEGER,
    status TEXT,
    priority TEXT,
    visibility TEXT,
    ai_summary TEXT,
    ai_keywords TEXT, -- JSON array
    
    -- Type-specific data (JSON)
    type_data TEXT, -- JSON with FileEntityData, FolderEntityData, or ProjectEntityData
    
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

CREATE INDEX idx_entities_workspace_type ON entities(workspace_id, type);
CREATE INDEX idx_entities_tags ON entities(workspace_id, json_extract(tags, '$'));
CREATE INDEX idx_entities_projects ON entities(workspace_id, json_extract(projects, '$'));
CREATE INDEX idx_entities_language ON entities(workspace_id, language);
CREATE INDEX idx_entities_category ON entities(workspace_id, category);
```

### O Mantener Tablas Separadas con Vista Unificada

```sql
-- Vista unificada
CREATE VIEW unified_entities AS
SELECT 
    'file' as type,
    'file:' || id as entity_id,
    workspace_id,
    filename as name,
    relative_path as path,
    -- ... campos unificados
FROM files
UNION ALL
SELECT 
    'folder' as type,
    'folder:' || id as entity_id,
    workspace_id,
    name,
    relative_path as path,
    -- ... campos unificados
FROM folders
UNION ALL
SELECT 
    'project' as type,
    'project:' || id as entity_id,
    workspace_id,
    name,
    path,
    -- ... campos unificados
FROM projects;
```

## Impacto en Facetas

### Facetas que Funcionan para Todos los Tipos

1. **Tags** - ✅ Archivos, ✅ Carpetas, ⚠️ Proyectos (agregar)
2. **Projects** - ✅ Archivos, ✅ Carpetas, ✅ Proyectos (parent)
3. **Language** - ✅ Archivos, ✅ Carpetas, ⚠️ Proyectos (agregar)
4. **Category/Nature** - ✅ Archivos, ✅ Carpetas, ✅ Proyectos
5. **Author** - ✅ Archivos, ⚠️ Carpetas (inferir), ⚠️ Proyectos (agregar)
6. **Owner** - ✅ Archivos, ⚠️ Carpetas (inferir), ⚠️ Proyectos (agregar)
7. **Location** - ✅ Archivos, ⚠️ Carpetas (inferir), ⚠️ Proyectos (agregar)
8. **Publication Year** - ✅ Archivos, ⚠️ Carpetas (inferir), ⚠️ Proyectos (agregar)
9. **Created Date** - ✅ Todos
10. **Modified Date** - ✅ Todos
11. **Status** - ✅ Archivos (indexing), ⚠️ Carpetas (agregar), ✅ Proyectos
12. **Priority** - ⚠️ Archivos (agregar), ⚠️ Carpetas (agregar), ✅ Proyectos
13. **Visibility** - ⚠️ Archivos (agregar), ⚠️ Carpetas (agregar), ✅ Proyectos

### Facetas Específicas de Tipo

Algunas facetas solo aplican a ciertos tipos:

- **Extension** - Solo archivos
- **MimeType** - Solo archivos
- **CodeMetrics** - Archivos y carpetas (agregado)
- **ImageMetadata** - Solo archivos
- **AudioMetadata** - Solo archivos
- **VideoMetadata** - Solo archivos
- **FolderMetrics** - Solo carpetas
- **ProjectAttributes** - Solo proyectos

## Plan de Implementación

### Fase 1: Análisis y Diseño ✅
- [x] Analizar características comunes
- [x] Diseñar modelo unificado
- [x] Identificar campos faltantes

### Fase 2: Backend - Entity Model
- [ ] Crear `Entity` struct unificado
- [ ] Crear `EntityRepository` interface
- [ ] Implementar conversión FileEntry → Entity
- [ ] Implementar conversión FolderEntry → Entity
- [ ] Implementar conversión Project → Entity
- [ ] Extender `FacetExecutor` para entidades

### Fase 3: Backend - Metadata Unificación
- [ ] Agregar tags a proyectos
- [ ] Agregar language a proyectos
- [ ] Agregar author a proyectos y carpetas
- [ ] Agregar location a proyectos y carpetas
- [ ] Agregar status a carpetas
- [ ] Agregar priority/visibility a archivos y carpetas

### Fase 4: Frontend - Unified Entity Provider
- [ ] Crear `UnifiedEntityFacetProvider`
- [ ] Actualizar facetas para incluir todos los tipos
- [ ] Actualizar UI para mostrar tipo de entidad (icono diferente)

### Fase 5: Testing y Refinamiento
- [ ] Probar facetas con todos los tipos
- [ ] Verificar performance
- [ ] Ajustar UI/UX

## Ejemplo de Uso

```typescript
// Buscar todas las entidades con tag "research"
const entities = await entityClient.getEntitiesByFacet(
  workspaceId,
  'tag',
  'research',
  ['file', 'folder', 'project']
);

// Resultado:
// [
//   { type: 'file', name: 'paper.pdf', path: 'docs/paper.pdf', tags: ['research'] },
//   { type: 'folder', name: 'research', path: 'research/', tags: ['research'] },
//   { type: 'project', name: 'Research Project', path: 'Research Project', tags: ['research'] }
// ]

// Filtrar por múltiples facetas
const filtered = await entityClient.listEntities(workspaceId, {
  tags: ['research'],
  language: 'es',
  projects: ['My Project']
});
// Retorna archivos, carpetas Y proyectos que cumplan TODAS las condiciones
```

## Conclusión

Sí, **proyectos, archivos y carpetas pueden tener las mismas características semánticas** y ser filtrados por facetas de forma unificada. El modelo propuesto:

1. ✅ Unifica las características semánticas comunes
2. ✅ Permite que las facetas filtren cualquier tipo de entidad
3. ✅ Mantiene compatibilidad con datos existentes
4. ✅ Extiende funcionalidad a proyectos y carpetas
5. ✅ Simplifica la UI (un solo árbol para todo)

La implementación requiere:
- Modelo unificado de entidades
- Extensión de metadata a proyectos y carpetas
- Actualización de facetas para incluir todos los tipos
- UI que muestre el tipo de entidad claramente


