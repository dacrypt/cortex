/**
 * MetadataStore - Persistent storage using JSON (no native dependencies)
 * Fallback implementation that doesn't require better-sqlite3
 */

import * as path from 'path';
import * as fs from 'fs/promises';
import { FileMetadata } from '../models/types';
import { generateFileId, inferFileType } from '../utils/fileHash';
import { IMetadataStore } from './IMetadataStore';

interface JSONStore {
  files: Record<string, FileMetadata>;
  tags: Record<string, string[]>; // tag -> [file_ids]
  contexts: Record<string, string[]>; // context -> [file_ids]
  types: Record<string, string[]>; // type -> [file_ids]
}

export class MetadataStoreJSON implements IMetadataStore {
  private store: JSONStore;
  private storePath: string;
  private cortexDir: string;

  constructor(workspaceRoot: string) {
    this.cortexDir = path.join(workspaceRoot, '.cortex');
    this.storePath = path.join(this.cortexDir, 'index.json');
    this.store = {
      files: {},
      tags: {},
      contexts: {},
      types: {},
    };
  }

  /**
   * Initialize the store
   */
  async initialize(): Promise<void> {
    // Ensure .cortex directory exists
    await fs.mkdir(this.cortexDir, { recursive: true });

    // Load existing store if it exists
    try {
      const data = await fs.readFile(this.storePath, 'utf-8');
      this.store = JSON.parse(data);
      console.log(`[MetadataStore] Loaded from ${this.storePath}`);
    } catch (error) {
      // File doesn't exist, start fresh
      console.log(`[MetadataStore] Initialized new store at ${this.storePath}`);
      await this.save();
    }
  }

  /**
   * Save store to disk
   */
  private async save(): Promise<void> {
    await fs.writeFile(
      this.storePath,
      JSON.stringify(this.store, null, 2),
      'utf-8'
    );
  }

  /**
   * Get or create metadata for a file
   */
  getOrCreateMetadata(relativePath: string, extension: string): FileMetadata {
    const fileId = generateFileId(relativePath);

    if (this.store.files[fileId]) {
      return this.store.files[fileId];
    }

    // Create new metadata
    const now = Date.now();
    const type = inferFileType(extension);

    const metadata: FileMetadata = {
      file_id: fileId,
      relativePath,
      tags: [],
      contexts: [],
      type,
      created_at: now,
      updated_at: now,
    };

    this.store.files[fileId] = metadata;

    // Add to type index
    if (!this.store.types[type]) {
      this.store.types[type] = [];
    }
    if (!this.store.types[type].includes(fileId)) {
      this.store.types[type].push(fileId);
    }

    this.save();
    return metadata;
  }

  /**
   * Ensure metadata entries exist for a batch of files.
   */
  ensureMetadataForFiles(
    files: Array<{ relativePath: string; extension: string }>
  ): number {
    let created = 0;

    for (const file of files) {
      const fileId = generateFileId(file.relativePath);
      if (this.store.files[fileId]) {
        continue;
      }

      const now = Date.now();
      const type = inferFileType(file.extension);

      const metadata: FileMetadata = {
        file_id: fileId,
        relativePath: file.relativePath,
        tags: [],
        contexts: [],
        type,
        created_at: now,
        updated_at: now,
      };

      this.store.files[fileId] = metadata;

      if (!this.store.types[type]) {
        this.store.types[type] = [];
      }
      if (!this.store.types[type].includes(fileId)) {
        this.store.types[type].push(fileId);
      }

      created += 1;
    }

    if (created > 0) {
      this.save();
    }

    return created;
  }

  /**
   * Get metadata for a file
   */
  getMetadata(fileId: string): FileMetadata | null {
    return this.store.files[fileId] || null;
  }

  /**
   * Get metadata by relative path
   */
  getMetadataByPath(relativePath: string): FileMetadata | null {
    const fileId = generateFileId(relativePath);
    return this.getMetadata(fileId);
  }

  /**
   * Add tag to a file
   */
  addTag(relativePath: string, tag: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    if (!metadata.tags.includes(tag)) {
      metadata.tags.push(tag);
    }

    if (!this.store.tags[tag]) {
      this.store.tags[tag] = [];
    }
    if (!this.store.tags[tag].includes(fileId)) {
      this.store.tags[tag].push(fileId);
    }

    metadata.updated_at = Date.now();
    this.save();
  }

  /**
   * Remove tag from a file
   */
  removeTag(relativePath: string, tag: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.tags = metadata.tags.filter((t) => t !== tag);

    if (this.store.tags[tag]) {
      this.store.tags[tag] = this.store.tags[tag].filter((id) => id !== fileId);
      if (this.store.tags[tag].length === 0) {
        delete this.store.tags[tag];
      }
    }

    metadata.updated_at = Date.now();
    this.save();
  }

  /**
   * Add context to a file
   */
  addContext(relativePath: string, context: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    if (!metadata.contexts.includes(context)) {
      metadata.contexts.push(context);
    }

    if (!this.store.contexts[context]) {
      this.store.contexts[context] = [];
    }
    if (!this.store.contexts[context].includes(fileId)) {
      this.store.contexts[context].push(fileId);
    }

    metadata.updated_at = Date.now();
    this.save();
  }

  /**
   * Remove context from a file
   */
  removeContext(relativePath: string, context: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.contexts = metadata.contexts.filter((c) => c !== context);

    if (this.store.contexts[context]) {
      this.store.contexts[context] = this.store.contexts[context].filter(
        (id) => id !== fileId
      );
      if (this.store.contexts[context].length === 0) {
        delete this.store.contexts[context];
      }
    }

    metadata.updated_at = Date.now();
    this.save();
  }

  /**
   * Update notes for a file
   */
  updateNotes(relativePath: string, notes: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.notes = notes;
    metadata.updated_at = Date.now();
    this.save();
  }

  /**
   * Get all files with a specific tag
   */
  getFilesByTag(tag: string): string[] {
    const fileIds = this.store.tags[tag] || [];
    return fileIds
      .map((id) => this.store.files[id]?.relativePath)
      .filter((path): path is string => path !== undefined);
  }

  /**
   * Get all files in a specific context
   */
  getFilesByContext(context: string): string[] {
    const fileIds = this.store.contexts[context] || [];
    return fileIds
      .map((id) => this.store.files[id]?.relativePath)
      .filter((path): path is string => path !== undefined);
  }

  /**
   * Get all files of a specific type
   */
  getFilesByType(type: string): string[] {
    const fileIds = this.store.types[type] || [];
    return fileIds
      .map((id) => this.store.files[id]?.relativePath)
      .filter((path): path is string => path !== undefined);
  }

  /**
   * Get all unique tags
   */
  getAllTags(): string[] {
    return Object.keys(this.store.tags).sort();
  }

  /**
   * Get all unique contexts
   */
  getAllContexts(): string[] {
    return Object.keys(this.store.contexts).sort();
  }

  /**
   * Get all unique types
   */
  getAllTypes(): string[] {
    return Object.keys(this.store.types).sort();
  }

  /**
   * Close (no-op for JSON)
   */
  close(): void {
    // Nothing to close for JSON
  }
}
