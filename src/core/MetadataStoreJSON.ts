/**
 * MetadataStore - Persistent storage using JSON (no native dependencies)
 * Fallback implementation that doesn't require better-sqlite3
 */

import * as path from "path";
import * as fs from "fs/promises";
import { createWriteStream } from "fs";
import { FileMetadata, MirrorMetadata } from "../models/types";
import { generateFileId, inferFileType } from "../utils/fileHash";
import { IMetadataStore } from "./IMetadataStore";

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
  private saveTimer?: NodeJS.Timeout;
  private saveInFlight?: Promise<void>;
  private saveQueued = false;
  private static readonly MAX_TEXT_CHARS = 200_000;

  constructor(workspaceRoot: string) {
    this.cortexDir = path.join(workspaceRoot, ".cortex");
    this.storePath = path.join(this.cortexDir, "index.json");
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
      const data = await fs.readFile(this.storePath, "utf-8");
      this.store = JSON.parse(data);
      console.log(`[MetadataStore] Loaded from ${this.storePath}`);
    } catch {
      // File doesn't exist, start fresh
      console.log(`[MetadataStore] Initialized new store at ${this.storePath}`);
      await this.save();
    }
  }

  /**
   * Save store to disk
   */
  private async save(): Promise<void> {
    const stream = createWriteStream(this.storePath, { encoding: "utf8" });
    let truncated = false;

    const writeChunk = async (chunk: string) => {
      if (!stream.write(chunk)) {
        await new Promise<void>((resolve, reject) => {
          stream.once("drain", resolve);
          stream.once("error", reject);
        });
      }
    };

    await new Promise<void>((resolve, reject) => {
      stream.once("error", reject);
      stream.once("finish", resolve);
      void (async () => {
        await writeChunk("{");
        await writeChunk('"files":{');
        let first = true;
        for (const [fileId, metadata] of Object.entries(this.store.files)) {
          if (!first) {
            await writeChunk(",");
          }
          first = false;
          await writeChunk(JSON.stringify(fileId));
          await writeChunk(":");
          const safeMetadata = this.sanitizeMetadataForSave(metadata, () => {
            truncated = true;
          });
          await writeChunk(JSON.stringify(safeMetadata));
        }
        await writeChunk('},"tags":{');
        first = true;
        for (const [tag, fileIds] of Object.entries(this.store.tags)) {
          if (!first) {
            await writeChunk(",");
          }
          first = false;
          await writeChunk(JSON.stringify(tag));
          await writeChunk(":");
          await writeChunk(JSON.stringify(fileIds));
        }
        await writeChunk('},"contexts":{');
        first = true;
        for (const [context, fileIds] of Object.entries(this.store.contexts)) {
          if (!first) {
            await writeChunk(",");
          }
          first = false;
          await writeChunk(JSON.stringify(context));
          await writeChunk(":");
          await writeChunk(JSON.stringify(fileIds));
        }
        await writeChunk('},"types":{');
        first = true;
        for (const [type, fileIds] of Object.entries(this.store.types)) {
          if (!first) {
            await writeChunk(",");
          }
          first = false;
          await writeChunk(JSON.stringify(type));
          await writeChunk(":");
          await writeChunk(JSON.stringify(fileIds));
        }
        await writeChunk("}}");
        stream.end();
      })().catch(reject);
    });
    if (truncated) {
      console.warn(
        `[MetadataStore] Truncated large text fields while saving ${this.storePath}`
      );
    }
  }

  private sanitizeMetadataForSave(
    metadata: FileMetadata,
    onTruncate: () => void
  ): FileMetadata {
    const maxChars = MetadataStoreJSON.MAX_TEXT_CHARS;
    let needsClone = false;

    const sanitizeText = (value?: string): string | undefined => {
      if (!value || value.length <= maxChars) {
        return value;
      }
      onTruncate();
      return `${value.slice(0, maxChars)} [truncated]`;
    };

    const notes = sanitizeText(metadata.notes);
    if (notes !== metadata.notes) {
      needsClone = true;
    }

    const aiSummary = sanitizeText(metadata.aiSummary);
    if (aiSummary !== metadata.aiSummary) {
      needsClone = true;
    }

    if (!needsClone) {
      return metadata;
    }

    return {
      ...metadata,
      notes,
      aiSummary,
    };
  }

  private scheduleSave(delayMs = 500): void {
    this.saveQueued = true;
    if (this.saveTimer) {
      return;
    }
    this.saveTimer = setTimeout(() => {
      this.saveTimer = undefined;
      void this.flushSave();
    }, delayMs);
  }

  private async flushSave(): Promise<void> {
    if (this.saveInFlight) {
      try {
        await this.saveInFlight;
      } catch (error) {
        console.error("[MetadataStore] Failed to save store:", error);
      }
      if (this.saveQueued) {
        await this.flushSave();
      }
      return;
    }
    this.saveQueued = false;
    this.saveInFlight = this.save();
    try {
      await this.saveInFlight;
    } catch (error) {
      console.error("[MetadataStore] Failed to save store:", error);
    } finally {
      this.saveInFlight = undefined;
    }
    if (this.saveQueued) {
      await this.flushSave();
    }
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
      suggestedContexts: [],
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

    this.scheduleSave();
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
        suggestedContexts: [],
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
      this.scheduleSave();
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
    this.scheduleSave();
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
    this.scheduleSave();
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
    this.scheduleSave();
  }

  /**
   * Add suggested context to a file
   */
  addSuggestedContext(relativePath: string, context: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    if (!metadata.suggestedContexts) {
      metadata.suggestedContexts = [];
    }

    if (!metadata.suggestedContexts.includes(context)) {
      metadata.suggestedContexts.push(context);
    }

    metadata.updated_at = Date.now();
    this.scheduleSave();
  }

  /**
   * Clear suggested contexts for a file
   */
  clearSuggestedContexts(relativePath: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.suggestedContexts = [];
    metadata.updated_at = Date.now();
    this.scheduleSave();
  }

  /**
   * Get suggested contexts for a file
   */
  getSuggestedContexts(relativePath: string): string[] {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];
    return metadata?.suggestedContexts ?? [];
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
    this.scheduleSave();
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
    this.scheduleSave();
  }

  /**
   * Update cached AI summary data for a file
   */
  updateAISummary(
    relativePath: string,
    summary: string,
    summaryHash: string,
    keyTerms?: string[]
  ): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.aiSummary = summary;
    metadata.aiSummaryHash = summaryHash;
    metadata.aiKeyTerms =
      keyTerms && keyTerms.length > 0 ? keyTerms : undefined;
    metadata.updated_at = Date.now();
    this.scheduleSave();
  }

  /**
   * Update cached mirror metadata for a file
   */
  updateMirrorMetadata(relativePath: string, mirror: MirrorMetadata): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    metadata.mirror = mirror;
    metadata.updated_at = Date.now();
    this.scheduleSave();
  }

  /**
   * Clear cached mirror metadata for a file
   */
  clearMirrorMetadata(relativePath: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    delete metadata.mirror;
    metadata.updated_at = Date.now();
    this.scheduleSave();
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
   * Get all files with a specific suggested context
   */
  getFilesBySuggestedContext(context: string): string[] {
    const results: string[] = [];
    for (const metadata of Object.values(this.store.files)) {
      if (metadata.suggestedContexts?.includes(context)) {
        results.push(metadata.relativePath);
      }
    }
    return results;
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
   * Get tag counts for all tags
   */
  getTagCounts(): Map<string, number> {
    const counts = new Map<string, number>();
    for (const [tag, fileIds] of Object.entries(this.store.tags)) {
      counts.set(tag, fileIds.length);
    }
    return counts;
  }

  /**
   * Get all unique contexts
   */
  getAllContexts(): string[] {
    return Object.keys(this.store.contexts).sort();
  }

  /**
   * Get all unique suggested contexts
   */
  getAllSuggestedContexts(): string[] {
    const contexts = new Set<string>();
    for (const metadata of Object.values(this.store.files)) {
      for (const context of metadata.suggestedContexts ?? []) {
        contexts.add(context);
      }
    }
    return Array.from(contexts).sort();
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

  /**
   * Remove file metadata and indexes
   */
  removeFile(relativePath: string): void {
    const fileId = generateFileId(relativePath);
    const metadata = this.store.files[fileId];

    if (!metadata) {
      return;
    }

    delete this.store.files[fileId];

    const removeFromIndex = (index: Record<string, string[]>) => {
      for (const key of Object.keys(index)) {
        const updated = index[key].filter((id) => id !== fileId);
        if (updated.length === 0) {
          delete index[key];
        } else {
          index[key] = updated;
        }
      }
    };

    removeFromIndex(this.store.tags);
    removeFromIndex(this.store.contexts);
    removeFromIndex(this.store.types);

    this.scheduleSave();
  }
}
