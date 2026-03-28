# Gap Analysis: Backend Facets vs Frontend UI

## Resumen

El backend tiene **32 facetas implementadas**, pero el frontend solo muestra un subconjunto limitado en los `fieldGroups` de `TermsFacetTreeProvider`.

## Facetas Disponibles en el Backend

### Terms Facets (22 facetas)
1. ✅ `extension` - En frontend
2. ✅ `type` - En frontend
3. ✅ `tag` - En frontend
4. ✅ `project` (alias: `context`) - En frontend
5. ✅ `content_type` - En frontend
6. ✅ `language` - **FALTA en frontend**
7. ✅ `category` - **FALTA en frontend**
8. ✅ `author` - **FALTA en frontend**
9. ✅ `publication_year` - **FALTA en frontend**
10. ✅ `owner` - **FALTA en frontend**
11. ✅ `indexing_status` - **FALTA en frontend**
12. ✅ `sentiment` - **FALTA en frontend**
13. ✅ `duplicate_type` - **FALTA en frontend**
14. ✅ `location` - **FALTA en frontend**
15. ✅ `organization` - **FALTA en frontend**
16. ✅ `purpose` - **FALTA en frontend**
17. ✅ `audience` - **FALTA en frontend**
18. ✅ `domain` - **FALTA en frontend**
19. ✅ `subdomain` - **FALTA en frontend**
20. ✅ `topic` - **FALTA en frontend**
21. ✅ `temporal_pattern` - **FALTA en frontend**
22. ✅ `readability_level` - **FALTA en frontend**

### Numeric Range Facets (6 facetas)
1. ✅ `size` - En frontend (SizeTreeProvider)
2. ✅ `complexity` - **FALTA en frontend**
3. ✅ `project_score` - **FALTA en frontend**
4. ✅ `function_count` - **FALTA en frontend**
5. ✅ `lines_of_code` - **FALTA en frontend**
6. ✅ `comment_percentage` - **FALTA en frontend**

### Date Range Facets (4 facetas)
1. ✅ `modified` - En frontend (DateTreeProvider)
2. ✅ `created` - **FALTA en frontend**
3. ✅ `accessed` - **FALTA en frontend**
4. ✅ `changed` - **FALTA en frontend**

## Facetas en Frontend que NO están en Backend

El frontend tiene algunos campos que no están implementados en el backend:
- `folder` - No es una faceta del backend (es una vista de FolderTreeProvider)
- `document_author`, `document_title`, `document_category`, etc. - Estos parecen ser campos específicos de documentos, no facetas genéricas del backend
- `image_artist`, `image_camera`, `audio_*`, `video_*` - Campos específicos de media que no están como facetas en el backend

## Recomendaciones

### 1. Actualizar TermsFacetTreeProvider fieldGroups

Agregar todas las facetas de términos disponibles:

```typescript
this.fieldGroups = [
  {
    key: 'core',
    label: 'Core Facets',
    fields: [
      { field: 'extension', label: 'By Extension' },
      { field: 'type', label: 'By Type' },
      { field: 'content_type', label: 'By Content Type' },
      { field: 'indexing_status', label: 'By Indexing Status' },
    ],
  },
  {
    key: 'organization',
    label: 'Organization',
    fields: [
      { field: 'tag', label: 'By Tag' },
      { field: 'project', label: 'By Project' },
      { field: 'owner', label: 'By Owner' },
    ],
  },
  {
    key: 'content',
    label: 'Content',
    fields: [
      { field: 'language', label: 'By Language' },
      { field: 'category', label: 'By Category' },
      { field: 'author', label: 'By Author' },
      { field: 'publication_year', label: 'By Publication Year' },
      { field: 'readability_level', label: 'By Readability Level' },
    ],
  },
  {
    key: 'ai_taxonomy',
    label: 'AI Taxonomy',
    fields: [
      { field: 'domain', label: 'By Domain' },
      { field: 'subdomain', label: 'By Subdomain' },
      { field: 'topic', label: 'By Topic' },
      { field: 'purpose', label: 'By Purpose' },
      { field: 'audience', label: 'By Audience' },
    ],
  },
  {
    key: 'enrichment',
    label: 'Enrichment',
    fields: [
      { field: 'sentiment', label: 'By Sentiment' },
      { field: 'location', label: 'By Location' },
      { field: 'organization', label: 'By Organization' },
      { field: 'duplicate_type', label: 'By Duplicate Type' },
    ],
  },
  {
    key: 'temporal',
    label: 'Temporal',
    fields: [
      { field: 'temporal_pattern', label: 'By Access Pattern' },
    ],
  },
];
```

### 2. Actualizar NumericRangeFacetTreeProvider

Agregar opciones para todas las facetas numéricas:

```typescript
// Agregar selector de campo o configuración para:
- complexity
- project_score
- function_count
- lines_of_code
- comment_percentage
```

### 3. Actualizar DateRangeFacetTreeProvider

Agregar opciones para todas las facetas de fecha:

```typescript
// Agregar selector de campo o configuración para:
- created
- accessed
- changed
```

### 4. Considerar un Endpoint para Listar Facetas Disponibles

En lugar de hardcodear las facetas en el frontend, el backend podría exponer un endpoint que liste todas las facetas disponibles con sus metadatos (tipo, categoría, descripción, etc.) desde el `FacetRegistry`.

## Estado Actual

- **Backend**: 32 facetas implementadas ✅
- **Frontend**: ~6-8 facetas visibles en fieldGroups ⚠️
- **Gap**: ~24-26 facetas disponibles pero no accesibles desde la UI



