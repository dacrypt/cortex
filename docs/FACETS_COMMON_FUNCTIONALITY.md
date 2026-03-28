# Funcionalidades Comunes en Facetas - Análisis y Simplificación

## Resumen

Análisis de patrones comunes y código duplicado en los providers de facetas para identificar oportunidades de simplificación.

## Funcionalidades Comunes Identificadas

### 1. Event Handling (100% duplicado)

**Patrón actual:**
```typescript
private readonly _onDidChangeTreeData: vscode.EventEmitter<TreeChangeEvent> = 
  new vscode.EventEmitter<TreeChangeEvent>();
readonly onDidChangeTreeData: vscode.Event<TreeChangeEvent> = 
  this._onDidChangeTreeData.event;

refresh(): void {
  this._onDidChangeTreeData.fire();
}
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider`

---

### 2. Caching con TTL (80% duplicado)

**Patrón actual:**
```typescript
private readonly facetCache: Map<string, { data: unknown; timestamp: number }> = new Map();
private readonly CACHE_TTL = 30000; // 30 seconds

// En cada método:
const cached = this.facetCache.get(key);
if (cached && Date.now() - cached.timestamp < this.CACHE_TTL) {
  return cached.data;
}
// ... compute ...
this.facetCache.set(key, { data, timestamp: Date.now() });
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider` con `getCached()`

---

### 3. File Cache Service (100% duplicado)

**Patrón actual:**
```typescript
private readonly fileCacheService: FileCacheService;

constructor(...) {
  const adminClient = new GrpcAdminClient(context);
  this.fileCacheService = FileCacheService.getInstance(adminClient);
  this.fileCacheService.setWorkspaceId(workspaceId);
}

async getFiles(): Promise<any[]> {
  return await this.fileCacheService.getFiles();
}
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider` con `getFiles()`

---

### 4. Creación de Tree Items de Archivo (95% duplicado)

**Patrón actual (repetido en TODOS los providers):**
```typescript
const relativePath = file.relative_path || '';
const absolutePath = path.join(this.workspaceRoot, relativePath);
const uri = vscode.Uri.file(absolutePath);
const filename = path.basename(relativePath);
const activityTime = getFileActivityTimestamp(file);

const item = new XxxTreeItem(
  filename,
  vscode.TreeItemCollapsibleState.None,
  uri,
  true // isFile
);
item.id = `file:xxx:${relativePath}`;
item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
item.description = path.dirname(relativePath);
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider` con `createFileItem()`

**Mejora adicional:** Agregar helper para tooltip y description comunes

---

### 5. Sorting por Actividad (90% duplicado)

**Patrón actual:**
```typescript
import { sortFilesByActivity, getFileActivityTimestamp } from '../utils/fileActivity';

// En cada método:
sortFilesByActivity(files);
// o
files.sort((a, b) => {
  const timeA = getFileActivityTimestamp(a);
  const timeB = getFileActivityTimestamp(b);
  return timeB - timeA;
});
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider` con `sortFilesByActivity()`

---

### 6. Placeholders Vacíos (85% duplicado)

**Patrón actual:**
```typescript
private getEmptyPlaceholder(): XxxTreeItem[] {
  const placeholder = new XxxTreeItem(
    t('noFilesFound') || t('noFacetData', { field: this.facetField }),
    vscode.TreeItemCollapsibleState.None
  );
  placeholder.iconPath = new vscode.ThemeIcon('info');
  placeholder.tooltip = t('noFacetDataTooltip', { field: this.facetField });
  placeholder.id = `placeholder:empty:${this.facetField}`;
  return [placeholder];
}
```

**Simplificación:** ⚠️ Agregar a `BaseFacetTreeProvider`:
```typescript
protected createEmptyPlaceholder(message?: string): T {
  return this.createTreeItem({
    id: `placeholder:empty:${this.config.field}`,
    kind: 'group',
    label: message || t('noFilesFound'),
    collapsibleState: vscode.TreeItemCollapsibleState.None,
    icon: new vscode.ThemeIcon('info'),
  });
}
```

---

### 7. Error Handling (80% duplicado)

**Patrón actual:**
```typescript
catch (error) {
  console.error(`[XxxFacetTree] Error fetching facets:`, error);
  const errorItem = new XxxTreeItem(
    t('errorLoadingFacetField', { field: this.facetField }),
    vscode.TreeItemCollapsibleState.None
  );
  errorItem.iconPath = new vscode.ThemeIcon('error');
  errorItem.tooltip = t('errorLoadingFacets');
  errorItem.id = `error:${this.facetField}`;
  return [errorItem];
}
```

**Simplificación:** ⚠️ Agregar a `BaseFacetTreeProvider`:
```typescript
protected createErrorItem(error: Error, context?: string): T {
  return this.createTreeItem({
    id: `error:${this.config.field}:${context || 'unknown'}`,
    kind: 'group',
    label: t('errorLoadingFacetField', { field: this.config.field }),
    collapsibleState: vscode.TreeItemCollapsibleState.None,
    icon: new vscode.ThemeIcon('error'),
    tooltip: `${t('errorLoadingFacets')}: ${error.message}`,
  });
}
```

---

### 8. Backend Facet Queries (70% duplicado)

**Patrón actual:**
```typescript
// Terms facets
const response = await this.knowledgeClient.getFacets(
  this.workspaceId,
  [{ field, type: 'terms' }]
);
const result = response?.results?.[0];
const terms = result?.terms?.terms;

// Numeric range facets
const response = await this.knowledgeClient.getFacets(
  this.workspaceId,
  [{ field: this.facetField, type: 'numeric_range' }]
);
const facetResult = response.results[0];
const ranges = facetResult.numeric_range?.ranges;

// Date range facets
const response = await this.knowledgeClient.getFacets(
  this.workspaceId,
  [{ field: this.facetField, type: 'date_range' }]
);
const facetResult = response.results[0];
const ranges = facetResult.date_range?.ranges;
```

**Simplificación:** ⚠️ Agregar helpers a `BaseFacetTreeProvider`:
```typescript
protected async queryBackendFacet(
  field: string,
  type: 'terms' | 'numeric_range' | 'date_range'
): Promise<any> {
  if (!this.knowledgeClient) {
    return null;
  }
  try {
    const response = await this.knowledgeClient.getFacets(
      this.workspaceId,
      [{ field, type }]
    );
    return response?.results?.[0];
  } catch (error) {
    console.warn(`[FacetTree] Backend facets unavailable for ${field}:`, error);
    return null;
  }
}
```

---

### 9. Path Operations (100% duplicado)

**Patrón actual:**
```typescript
import * as path from 'node:path';

const absolutePath = path.join(this.workspaceRoot, relativePath);
const uri = vscode.Uri.file(absolutePath);
const filename = path.basename(relativePath);
const dirname = path.dirname(relativePath);
```

**Simplificación:** ✅ Ya en `BaseFacetTreeProvider` con `getFileUri()`

**Mejora adicional:** Agregar más helpers:
```typescript
protected getRelativePath(absolutePath: string): string {
  return path.relative(this.workspaceRoot, absolutePath);
}

protected getFilename(relativePath: string): string {
  return path.basename(relativePath);
}

protected getDirname(relativePath: string): string {
  return path.dirname(relativePath);
}
```

---

### 10. Tree Item ID Generation (90% duplicado)

**Patrón actual:**
```typescript
item.id = `file:${field}:${relativePath}`;
item.id = `facet:${field}:${value}`;
item.id = `range:${field}:${min}:${max}`;
item.id = `folder:${folderPath}`;
item.id = `project:${projectId}`;
```

**Simplificación:** ⚠️ Agregar helpers a `BaseFacetTreeProvider`:
```typescript
protected generateFileId(relativePath: string, context?: string): string {
  return `file:${this.config.field}${context ? `:${context}` : ''}:${relativePath}`;
}

protected generateFacetId(value: string): string {
  return `facet:${this.config.field}:${value}`;
}

protected generateRangeId(min: number | string, max: number | string): string {
  return `range:${this.config.field}:${min}:${max}`;
}
```

---

### 11. Tooltip y Description Comunes (85% duplicado)

**Patrón actual:**
```typescript
item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
item.description = path.dirname(relativePath);
// o
item.tooltip = `${field}: ${value}\nCount: ${count} files`;
item.description = `${count} files`;
```

**Simplificación:** ⚠️ Agregar helpers a `BaseFacetTreeProvider`:
```typescript
protected createFileTooltip(relativePath: string, file: any): string {
  const activityTime = this.getFileActivityTimestamp(file);
  return `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
}

protected createFacetTooltip(value: string, count: number): string {
  return `${this.config.field}: ${value}\nCount: ${count} files`;
}

protected createFileDescription(relativePath: string): string {
  return this.getDirname(relativePath);
}
```

---

### 12. Filtrado de Archivos (75% duplicado)

**Patrón actual:**
```typescript
// Por campo específico
const matchingFiles = filesCache.filter((file) => {
  const value = this.extractFieldValue(file, field);
  return value === term;
});

// Por rango numérico
const matchingFiles = filesCache.filter((file) => {
  const value = this.getNumericFieldValue(file, field);
  return value >= min && value <= max;
});

// Por rango de fecha
const matchingFiles = filesCache.filter((file) => {
  const value = this.getDateFieldValue(file, field);
  return value >= start && value <= end;
});
```

**Simplificación:** ⚠️ Agregar helpers genéricos:
```typescript
protected filterFilesByValue(
  files: any[],
  field: string,
  value: string | number
): any[] {
  return files.filter((file) => {
    const fileValue = this.extractFieldValue(file, field);
    return fileValue === value;
  });
}

protected filterFilesByRange(
  files: any[],
  field: string,
  min: number,
  max: number
): any[] {
  return files.filter((file) => {
    const value = this.getNumericFieldValue(file, field);
    if (max === 0) return value >= min;
    return value >= min && value <= max;
  });
}
```

---

### 13. Normalización de Campos (60% duplicado)

**Patrón actual:**
```typescript
// En NumericRangeFacetTreeProvider
private normalizeNumericField(field: string): string {
  const normalized = field.trim().toLowerCase();
  if (normalized === 'file_size' || normalized === 'size') return 'size';
  // ... más casos
}

// En DateRangeFacetTreeProvider
private normalizeDateField(field: string): string {
  const normalized = field.trim().toLowerCase();
  if (normalized === 'created_at' || normalized === 'created') return 'created';
  // ... más casos
}
```

**Simplificación:** ⚠️ Mover a `FacetRegistry` o crear `FieldNormalizer`:
```typescript
// En FacetRegistry
normalizeField(field: string): string {
  const config = this.get(field);
  return config?.field || field;
}
```

---

### 14. Construcción de Labels con Counts (80% duplicado)

**Patrón actual:**
```typescript
const label = `${value} (${count})`;
const label = `${rangeLabel} (${count})`;
const label = `${folder} (${fileCount})`;
```

**Simplificación:** ⚠️ Agregar helper:
```typescript
protected formatLabelWithCount(label: string, count: number): string {
  return `${label} (${count})`;
}
```

---

## Resumen de Simplificaciones Propuestas

### ✅ Ya Implementado en BaseFacetTreeProvider
1. Event handling
2. Caching con TTL
3. File cache service
4. Creación básica de tree items
5. Sorting por actividad
6. Path operations básicas

### ⚠️ Pendiente de Agregar a BaseFacetTreeProvider

1. **Placeholders vacíos** - `createEmptyPlaceholder()`
2. **Error handling** - `createErrorItem()`
3. **Backend queries** - `queryBackendFacet()`
4. **ID generation** - `generateFileId()`, `generateFacetId()`, `generateRangeId()`
5. **Tooltips comunes** - `createFileTooltip()`, `createFacetTooltip()`
6. **Filtrado de archivos** - `filterFilesByValue()`, `filterFilesByRange()`
7. **Labels con counts** - `formatLabelWithCount()`
8. **Path helpers adicionales** - `getRelativePath()`, `getFilename()`, `getDirname()`

### 📋 Funcionalidades Específicas (No simplificables)

1. **Lógica de negocio específica** - Cada faceta tiene su lógica única
2. **Extracción de valores de campos** - Depende del tipo de faceta
3. **Construcción de rangos** - Específico de numeric/date ranges
4. **Agrupación por categorías** - Específico de category providers

## Impacto Estimado

- **Código duplicado eliminado:** ~40-50%
- **Líneas de código reducidas:** ~2000-3000 líneas
- **Mantenibilidad:** ⬆️ Significativamente mejorada
- **Consistencia:** ⬆️ Todas las facetas usan los mismos helpers
- **Testing:** ⬆️ Más fácil testear funcionalidad común

## Próximos Pasos

1. Agregar helpers faltantes a `BaseFacetTreeProvider`
2. Refactorizar providers uno por uno para usar los helpers
3. Eliminar código duplicado
4. Actualizar tests


