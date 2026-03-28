/**
 * Unified Entity Model - Frontend TypeScript
 * 
 * Represents files, folders, and projects as unified entities
 * that can be filtered by the same facets.
 */

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
  relativePath?: string;
  absolutePath?: string;
}

export interface FolderEntityData {
  depth?: number;
  totalFiles?: number;
  directFiles?: number;
  subfolders?: number;
  dominantFileType?: string;
  relativePath?: string;
  absolutePath?: string;
}

export interface ProjectEntityData {
  nature?: string;
  attributes?: ProjectAttributes;
  parentId?: string;
  documentCount?: number;
}

// Type definitions for nested data
export interface CodeMetrics {
  linesOfCode?: number;
  commentLines?: number;
  functionCount?: number;
  classCount?: number;
  complexity?: number;
}

export interface DocumentMetrics {
  pageCount?: number;
  wordCount?: number;
  author?: string;
  title?: string;
  createdDate?: number;
}

export interface IndexedState {
  basic?: boolean;
  mime?: boolean;
  code?: boolean;
  document?: boolean;
  mirror?: boolean;
}

export interface ProjectAttributes {
  temporality?: string;
  collaboration?: string;
  priority?: string;
  status?: string;
  visibility?: string;
  tags?: string[];
  language?: string;
  author?: string;
  owner?: string;
  location?: string;
  publicationYear?: number;
  aiSummary?: string;
  aiKeywords?: string[];
}

// Entity filters for querying
export interface EntityFilters {
  types?: EntityType[];
  tags?: string[];
  projects?: string[];
  language?: string;
  category?: string;
  author?: string;
  owner?: string;
  location?: string;
  publicationYear?: number;
  status?: string;
  priority?: string;
  visibility?: string;
  complexityMin?: number;
  complexityMax?: number;
  sizeMin?: number;
  sizeMax?: number;
  createdAfter?: number;
  createdBefore?: number;
  updatedAfter?: number;
  updatedBefore?: number;
  limit?: number;
  offset?: number;
}

// Entity metadata for updates
export interface EntityMetadata {
  tags?: string[];
  projects?: string[];
  language?: string;
  category?: string;
  author?: string;
  owner?: string;
  location?: string;
  publicationYear?: number;
  status?: string;
  priority?: string;
  visibility?: string;
  aiSummary?: string;
  aiKeywords?: string[];
  description?: string;
}


