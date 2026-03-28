/**
 * Interface for metadata storage
 * Allows different implementations (SQLite, JSON, etc.)
 */

import { FileMetadata, MirrorMetadata } from '../models/types';

export type RefreshHandler = () => void;

export interface IMetadataStore {
  initialize(): Promise<void>;
  setRefreshHandler(handler: RefreshHandler): void;
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
  getFilesBySuggestedContext(context: string): string[];
  updateNotes(relativePath: string, notes: string): void;
  updateAISummary(
    relativePath: string,
    summary: string,
    summaryHash: string,
    keyTerms?: string[]
  ): void;
  ensureMetadataForFiles(
    files: Array<{ relativePath: string; extension: string }>
  ): number;
  updateMirrorMetadata(relativePath: string, mirror: MirrorMetadata): void;
  clearMirrorMetadata(relativePath: string): void;
  removeFile(relativePath: string): void;
  getFilesByTag(tag: string): string[];
  getFilesByContext(context: string): string[];
  getFilesByType(type: string): string[];
  getAllTags(): string[];
  getTagCounts(): Map<string, number>;
  getAllContexts(): string[];
  getAllSuggestedContexts(): string[];
  getAllTypes(): string[];
  close(): void;
}
