/**
 * Interface for metadata storage
 * Allows different implementations (SQLite, JSON, etc.)
 */

import { FileMetadata } from '../models/types';

export interface IMetadataStore {
  initialize(): Promise<void>;
  getOrCreateMetadata(relativePath: string, extension: string): FileMetadata;
  getMetadata(fileId: string): FileMetadata | null;
  getMetadataByPath(relativePath: string): FileMetadata | null;
  addTag(relativePath: string, tag: string): void;
  removeTag(relativePath: string, tag: string): void;
  addContext(relativePath: string, context: string): void;
  removeContext(relativePath: string, context: string): void;
  addSuggestedContext(relativePath: string, context: string): void;
  clearSuggestedContexts(relativePath: string): void;
  getSuggestedContexts(relativePath: string): string[];
  updateNotes(relativePath: string, notes: string): void;
  getFilesByTag(tag: string): string[];
  getFilesByContext(context: string): string[];
  getFilesByType(type: string): string[];
  getAllTags(): string[];
  getAllContexts(): string[];
  getAllTypes(): string[];
  close(): void;
}
