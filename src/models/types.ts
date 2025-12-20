/**
 * Core type definitions for Cortex
 */

import { EnhancedMetadata } from '../extractors/MetadataExtractor';

/**
 * In-memory index entry for a workspace file
 */
export interface FileIndexEntry {
  absolutePath: string;
  relativePath: string;
  filename: string;
  extension: string;
  lastModified: number; // Unix timestamp
  fileSize: number; // bytes
  enhanced?: EnhancedMetadata; // Rich metadata
}

/**
 * Persistent metadata for a file
 */
export interface FileMetadata {
  file_id: string; // Stable hash of relative path
  relativePath: string; // For reference
  tags: string[];
  contexts: string[]; // Projects, clients, cases, etc.
  type: string; // Inferred from extension (ts, pdf, md, etc.)
  notes?: string;
  created_at: number; // Unix timestamp
  updated_at: number; // Unix timestamp
}

/**
 * Tree item types for virtual views
 */
export enum CortexTreeItemType {
  Context = 'context',
  Tag = 'tag',
  FileType = 'fileType',
  File = 'file',
}

/**
 * Data structure for tree items
 */
export interface CortexTreeItem {
  type: CortexTreeItemType;
  label: string;
  filePath?: string; // Only for File type
  children?: CortexTreeItem[];
}
