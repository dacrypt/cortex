import { IMetadataStore } from "./IMetadataStore";
import { FileMetadata, MirrorMetadata } from "../models/types";
import { GrpcMetadataClient } from "./GrpcMetadataClient";

type RefreshHandler = () => void;

/**
 * Backend metadata response structure from gRPC
 */
interface BackendMetadataResponse {
  file_id?: string;
  relative_path?: string;
  tags?: string[];
  contexts?: string[];
  suggested_contexts?: string[];
  type?: string;
  notes?: string;
  ai_summary?: {
    summary?: string;
    content_hash?: string;
    key_terms?: string[];
  };
  ai_category?: {
    category?: string;
    confidence?: number;
  };
  ai_related?: Array<{
    relative_path?: string;
    similarity?: number;
    reason?: string;
  }>;
  mirror?: {
    format?: string;
    path?: string;
    source_mtime?: number;
    updated_at?: number;
  };
  created_at?: number;
  updated_at?: number;
}

export class BackendMetadataStore implements IMetadataStore {
  private readonly metadataByPath = new Map<string, FileMetadata>();
  private tags: string[] = [];
  private tagCounts = new Map<string, number>();
  private contexts: string[] = [];
  private readonly filesByTag = new Map<string, string[]>();
  private readonly filesByContext = new Map<string, string[]>();
  private readonly suggestionsByContext = new Map<string, string[]>();
  private refreshHandler?: RefreshHandler;

  constructor(
    private readonly client: GrpcMetadataClient,
    private readonly workspaceId: string
  ) {}

  setRefreshHandler(handler: RefreshHandler): void {
    this.refreshHandler = handler;
  }

  async initialize(): Promise<void> {
    await this.refreshBase();
  }

  async refreshBase(): Promise<void> {
    try {
      const [tags, tagCounts, contexts, suggestions] = await Promise.all([
        this.client.getAllTags(this.workspaceId),
        this.client.getTagCounts(this.workspaceId),
        this.client.getAllContexts(this.workspaceId),
        this.client.getSuggestions(this.workspaceId),
      ]);
      this.tags = tags;
      this.tagCounts = tagCounts;
      this.contexts = contexts;
      this.filesByTag.clear();
      this.filesByContext.clear();
      this.suggestionsByContext.clear();
      for (const suggestion of suggestions) {
        const rel = suggestion.relative_path;
        for (const ctx of suggestion.suggested_contexts ?? []) {
          const list = this.suggestionsByContext.get(ctx) ?? [];
          list.push(rel);
          this.suggestionsByContext.set(ctx, list);
        }
      }
      this.refreshHandler?.();
    } catch (error) {
      console.warn("[Cortex] Failed to refresh backend metadata cache:", error);
    }
  }

  seedFromFiles(files: Array<{ relativePath: string; extension: string }>): void {
    const now = Date.now();
    for (const file of files) {
      const existing = this.metadataByPath.get(file.relativePath);
      if (existing) {
        if (!existing.type) {
          existing.type = this.normalizeType(file.extension);
        }
        continue;
      }
      this.metadataByPath.set(file.relativePath, {
        file_id: file.relativePath,
        relativePath: file.relativePath,
        tags: [],
        contexts: [],
        suggestedContexts: [],
        type: this.normalizeType(file.extension),
        created_at: now,
        updated_at: now,
      });
    }
  }

  getOrCreateMetadata(relativePath: string, extension: string): FileMetadata {
    const existing = this.metadataByPath.get(relativePath);
    if (existing) {
      return existing;
    }
    const now = Date.now();
    const metadata: FileMetadata = {
      file_id: relativePath,
      relativePath,
      tags: [],
      contexts: [],
      suggestedContexts: [],
      type: this.normalizeType(extension),
      created_at: now,
      updated_at: now,
    };
    this.metadataByPath.set(relativePath, metadata);
    void this.fetchMetadata(relativePath);
    return metadata;
  }

  getMetadata(fileId: string): FileMetadata | null {
    for (const metadata of this.metadataByPath.values()) {
      if (metadata.file_id === fileId) {
        return metadata;
      }
    }
    return null;
  }

  getMetadataByPath(relativePath: string): FileMetadata | null {
    const existing = this.metadataByPath.get(relativePath) || null;
    if (!existing) {
      void this.fetchMetadata(relativePath);
    }
    return existing;
  }

  addTag(relativePath: string, tag: string): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    if (!metadata.tags.includes(tag)) {
      metadata.tags.push(tag);
    }
    this.tagCounts.set(tag, (this.tagCounts.get(tag) ?? 0) + 1);
    if (!this.tags.includes(tag)) {
      this.tags.push(tag);
    }
    this.filesByTag.delete(tag);
    void this.client.addTag(this.workspaceId, relativePath, tag);
    this.refreshHandler?.();
  }

  removeTag(relativePath: string, tag: string): void {
    const metadata = this.getMetadataByPath(relativePath);
    if (metadata) {
      metadata.tags = metadata.tags.filter((t) => t !== tag);
    }
    const current = this.tagCounts.get(tag);
    if (current) {
      this.tagCounts.set(tag, Math.max(0, current - 1));
    }
    this.filesByTag.delete(tag);
    void this.client.removeTag(this.workspaceId, relativePath, tag);
    this.refreshHandler?.();
  }

  addContext(relativePath: string, context: string): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    if (!metadata.contexts.includes(context)) {
      metadata.contexts.push(context);
    }
    if (!this.contexts.includes(context)) {
      this.contexts.push(context);
    }
    this.filesByContext.delete(context);
    void this.client.addContext(this.workspaceId, relativePath, context);
    this.refreshHandler?.();
  }

  removeContext(relativePath: string, context: string): void {
    const metadata = this.getMetadataByPath(relativePath);
    if (metadata) {
      metadata.contexts = metadata.contexts.filter((c) => c !== context);
    }
    this.filesByContext.delete(context);
    void this.client.removeContext(this.workspaceId, relativePath, context);
    this.refreshHandler?.();
  }

  addSuggestedContext(relativePath: string, context: string): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    metadata.suggestedContexts = Array.from(
      new Set([...(metadata.suggestedContexts ?? []), context])
    );
    const list = this.suggestionsByContext.get(context) ?? [];
    list.push(relativePath);
    this.suggestionsByContext.set(context, list);
    void this.client.addSuggestedContext(
      this.workspaceId,
      relativePath,
      context
    );
    this.refreshHandler?.();
  }

  clearSuggestedContexts(relativePath: string): void {
    const metadata = this.getMetadataByPath(relativePath);
    if (!metadata?.suggestedContexts?.length) {
      return;
    }
    for (const context of metadata.suggestedContexts) {
      const list = this.suggestionsByContext.get(context);
      if (list) {
        this.suggestionsByContext.set(
          context,
          list.filter((path) => path !== relativePath)
        );
      }
      void this.client.dismissSuggestion(
        this.workspaceId,
        relativePath,
        context
      );
    }
    metadata.suggestedContexts = [];
    this.refreshHandler?.();
  }

  getSuggestedContexts(relativePath: string): string[] {
    return this.metadataByPath.get(relativePath)?.suggestedContexts ?? [];
  }

  getFilesBySuggestedContext(context: string): string[] {
    return this.suggestionsByContext.get(context) ?? [];
  }

  updateNotes(relativePath: string, notes: string): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    metadata.notes = notes;
    metadata.updated_at = Date.now();
    void this.client.updateNotes(this.workspaceId, relativePath, notes);
    this.refreshHandler?.();
  }

  updateAISummary(
    relativePath: string,
    summary: string,
    summaryHash: string,
    keyTerms?: string[]
  ): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    metadata.aiSummary = summary;
    metadata.aiSummaryHash = summaryHash;
    metadata.aiKeyTerms = keyTerms;
    metadata.updated_at = Date.now();
    void this.client.updateAISummary(
      this.workspaceId,
      relativePath,
      summary,
      summaryHash,
      keyTerms ?? []
    );
    this.refreshHandler?.();
  }

  ensureMetadataForFiles(
    files: Array<{ relativePath: string; extension: string }>
  ): number {
    let created = 0;
    for (const file of files) {
      if (!this.metadataByPath.has(file.relativePath)) {
        this.getOrCreateMetadata(file.relativePath, file.extension);
        created += 1;
      }
    }
    return created;
  }

  updateMirrorMetadata(relativePath: string, mirror: MirrorMetadata): void {
    const metadata = this.getOrCreateMetadata(relativePath, "");
    metadata.mirror = mirror;
    metadata.updated_at = Date.now();
    this.refreshHandler?.();
  }

  clearMirrorMetadata(relativePath: string): void {
    const metadata = this.getMetadataByPath(relativePath);
    if (metadata?.mirror) {
      delete metadata.mirror;
      metadata.updated_at = Date.now();
      this.refreshHandler?.();
    }
  }

  removeFile(relativePath: string): void {
    this.metadataByPath.delete(relativePath);
    for (const [tag, list] of this.filesByTag.entries()) {
      this.filesByTag.set(
        tag,
        list.filter((path) => path !== relativePath)
      );
    }
    for (const [context, list] of this.filesByContext.entries()) {
      this.filesByContext.set(
        context,
        list.filter((path) => path !== relativePath)
      );
    }
    for (const [context, list] of this.suggestionsByContext.entries()) {
      this.suggestionsByContext.set(
        context,
        list.filter((path) => path !== relativePath)
      );
    }
  }

  getFilesByTag(tag: string): string[] {
    const cached = this.filesByTag.get(tag);
    if (cached) {
      return cached;
    }
    void this.fetchFilesByTag(tag);
    return [];
  }

  getFilesByContext(context: string): string[] {
    const cached = this.filesByContext.get(context);
    if (cached) {
      return cached;
    }
    void this.fetchFilesByContext(context);
    return [];
  }

  getFilesByType(type: string): string[] {
    const matches: string[] = [];
    for (const [relativePath, metadata] of this.metadataByPath.entries()) {
      if (metadata.type === type) {
        matches.push(relativePath);
      }
    }
    return matches;
  }

  getAllTags(): string[] {
    return this.tags.slice();
  }

  getTagCounts(): Map<string, number> {
    return new Map(this.tagCounts);
  }

  getAllContexts(): string[] {
    return this.contexts.slice();
  }

  getAllSuggestedContexts(): string[] {
    return Array.from(this.suggestionsByContext.keys());
  }

  getAllTypes(): string[] {
    const types = new Set<string>();
    for (const metadata of this.metadataByPath.values()) {
      if (metadata.type) {
        types.add(metadata.type);
      }
    }
    return Array.from(types).sort((a, b) => a.localeCompare(b));
  }

  /**
   * Close method required by IMetadataStore interface.
   * No cleanup needed for backend store as it uses gRPC client.
   */
  close(): void {
    // No-op: gRPC client lifecycle is managed elsewhere
  }

  /**
   * Minimal type normalization - just removes leading dot and lowercases
   * The backend provides the authoritative type via FileEntityData.ContentType
   * This is only used as a fallback when backend metadata is not yet available
   */
  private normalizeType(extension: string): string {
    const normalized = extension.toLowerCase().replace(/^\./, "");
    return normalized || "unknown";
  }

  private async fetchMetadata(relativePath: string): Promise<void> {
    try {
      const metadata = await this.client.getMetadataByPath(
        this.workspaceId,
        relativePath
      );
      if (!metadata) {
        return;
      }
      this.metadataByPath.set(relativePath, this.mapMetadata(metadata));
      this.refreshHandler?.();
    } catch (error: unknown) {
      // NOT_FOUND (code 5) is a valid case - file may not have metadata yet
      // Don't log it as an error, just silently handle it
      const err = error as { code?: number; message?: string };
      if (err?.code === 5 || err?.message?.includes('NOT_FOUND') || err?.message?.includes('metadata not found')) {
        // File doesn't have metadata yet - this is normal for newly indexed files
        return;
      }
      // Only log actual errors (not NOT_FOUND)
      console.warn(
        `[Cortex] Failed to fetch metadata for ${relativePath}:`,
        error
      );
    }
  }

  private async fetchFilesByTag(tag: string): Promise<void> {
    try {
      const entries = await this.client.listByTag(this.workspaceId, tag);
      const paths = entries.map((entry) => entry.relative_path);
      this.filesByTag.set(tag, paths);
      this.refreshHandler?.();
    } catch (error) {
      console.warn(`[Cortex] Failed to list files by tag ${tag}:`, error);
    }
  }

  private async fetchFilesByContext(context: string): Promise<void> {
    try {
      const entries = await this.client.listByContext(
        this.workspaceId,
        context
      );
      const paths = entries.map((entry) => entry.relative_path);
      this.filesByContext.set(context, paths);
      this.refreshHandler?.();
    } catch (error) {
      console.warn(
        `[Cortex] Failed to list files by context ${context}:`,
        error
      );
    }
  }

  private mapMetadata(metadata: BackendMetadataResponse): FileMetadata {
    return {
      file_id: metadata.file_id ?? "",
      relativePath: metadata.relative_path ?? "",
      tags: metadata.tags ?? [],
      contexts: metadata.contexts ?? [],
      suggestedContexts: metadata.suggested_contexts ?? [],
      type: metadata.type ?? "unknown",
      notes: metadata.notes ?? undefined,
      aiSummary: metadata.ai_summary?.summary ?? undefined,
      aiSummaryHash: metadata.ai_summary?.content_hash ?? undefined,
      aiKeyTerms: metadata.ai_summary?.key_terms ?? undefined,
      aiCategory: metadata.ai_category?.category ?? undefined,
      aiCategoryConfidence: metadata.ai_category?.confidence ?? undefined,
      aiRelated: Array.isArray(metadata.ai_related)
        ? metadata.ai_related.map((item) => ({
            relativePath: item.relative_path ?? "",
            similarity: item.similarity ?? undefined,
            reason: item.reason ?? undefined,
          }))
        : undefined,
      mirror: metadata.mirror
        ? {
            format: (metadata.mirror.format === 'md' || metadata.mirror.format === 'csv')
              ? metadata.mirror.format
              : 'md' as const, // Default to 'md' if format is invalid
            path: metadata.mirror.path ?? "",
            sourceMtime: metadata.mirror.source_mtime ?? 0,
            updatedAt: metadata.mirror.updated_at ?? 0,
          }
        : undefined,
      created_at: metadata.created_at ?? Date.now(),
      updated_at: metadata.updated_at ?? Date.now(),
    };
  }
}
