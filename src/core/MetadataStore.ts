/**
 * MetadataStore - Persistent storage for file metadata using SQLite
 */

import * as path from "path";
import * as fs from "fs/promises";
import { FileMetadata, MirrorMetadata } from "../models/types";
import { generateFileId, inferFileType } from "../utils/fileHash";

// We'll use better-sqlite3 for synchronous SQLite access
// Note: This requires the better-sqlite3 package
import Database from "better-sqlite3";

export class MetadataStore {
  private db: Database.Database | null = null;
  private dbPath: string;
  private cortexDir: string;

  constructor(workspaceRoot: string) {
    this.cortexDir = path.join(workspaceRoot, ".cortex");
    this.dbPath = path.join(this.cortexDir, "index.sqlite");
  }

  /**
   * Initialize the database
   * Creates .cortex directory and database file if needed
   */
  async initialize(): Promise<void> {
    // Ensure .cortex directory exists
    await fs.mkdir(this.cortexDir, { recursive: true });

    // Open database
    this.db = new Database(this.dbPath);

    // Create tables
    this.createTables();

    console.log(`[MetadataStore] Initialized at ${this.dbPath}`);
  }

  /**
   * Create database tables
   */
  private createTables(): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    // Main metadata table
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS file_metadata (
        file_id TEXT PRIMARY KEY,
        relative_path TEXT NOT NULL UNIQUE,
        type TEXT NOT NULL,
        notes TEXT,
        ai_summary TEXT,
        ai_summary_hash TEXT,
        ai_key_terms TEXT,
        mirror_format TEXT,
        mirror_path TEXT,
        mirror_source_mtime INTEGER,
        mirror_updated_at INTEGER,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL
      );
    `);

    // Tags table (many-to-many)
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS file_tags (
        file_id TEXT NOT NULL,
        tag TEXT NOT NULL,
        PRIMARY KEY (file_id, tag),
        FOREIGN KEY (file_id) REFERENCES file_metadata(file_id) ON DELETE CASCADE
      );
    `);

    // Contexts table (many-to-many)
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS file_contexts (
        file_id TEXT NOT NULL,
        context TEXT NOT NULL,
        PRIMARY KEY (file_id, context),
        FOREIGN KEY (file_id) REFERENCES file_metadata(file_id) ON DELETE CASCADE
      );
    `);

    this.db.exec(`
      CREATE TABLE IF NOT EXISTS file_context_suggestions (
        file_id TEXT NOT NULL,
        context TEXT NOT NULL,
        PRIMARY KEY (file_id, context),
        FOREIGN KEY (file_id) REFERENCES file_metadata(file_id) ON DELETE CASCADE
      );
    `);

    // Create indexes for performance
    this.db.exec(`
      CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag);
      CREATE INDEX IF NOT EXISTS idx_file_contexts_context ON file_contexts(context);
      CREATE INDEX IF NOT EXISTS idx_file_metadata_type ON file_metadata(type);
      CREATE INDEX IF NOT EXISTS idx_file_context_suggestions_context ON file_context_suggestions(context);
    `);

    this.ensureAiColumns();
    this.ensureMirrorColumns();
  }

  /**
   * Ensure AI summary columns exist for older databases.
   */
  private ensureAiColumns(): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const addColumn = (sql: string) => {
      try {
        this.db?.exec(sql);
      } catch {
        // Column likely already exists.
      }
    };

    addColumn(`ALTER TABLE file_metadata ADD COLUMN ai_summary TEXT;`);
    addColumn(`ALTER TABLE file_metadata ADD COLUMN ai_summary_hash TEXT;`);
    addColumn(`ALTER TABLE file_metadata ADD COLUMN ai_key_terms TEXT;`);
  }

  /**
   * Ensure mirror columns exist for older databases.
   */
  private ensureMirrorColumns(): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const addColumn = (sql: string) => {
      try {
        this.db?.exec(sql);
      } catch {
        // Column likely already exists.
      }
    };

    addColumn(`ALTER TABLE file_metadata ADD COLUMN mirror_format TEXT;`);
    addColumn(`ALTER TABLE file_metadata ADD COLUMN mirror_path TEXT;`);
    addColumn(`ALTER TABLE file_metadata ADD COLUMN mirror_source_mtime INTEGER;`);
    addColumn(`ALTER TABLE file_metadata ADD COLUMN mirror_updated_at INTEGER;`);
  }

  /**
   * Get or create metadata for a file
   *
   * @param relativePath - Workspace-relative path
   * @param extension - File extension
   * @returns FileMetadata
   */
  getOrCreateMetadata(relativePath: string, extension: string): FileMetadata {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    // Try to get existing metadata
    const existing = this.getMetadata(fileId);
    if (existing) {
      return existing;
    }

    // Create new metadata
    const now = Date.now();
    const type = inferFileType(extension);

    const stmt = this.db.prepare(`
      INSERT INTO file_metadata (file_id, relative_path, type, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?)
    `);

    stmt.run(fileId, relativePath, type, now, now);

    return {
      file_id: fileId,
      relativePath,
      tags: [],
      contexts: [],
      suggestedContexts: [],
      type,
      created_at: now,
      updated_at: now,
    };
  }

  /**
   * Ensure metadata entries exist for a batch of files.
   */
  ensureMetadataForFiles(
    files: Array<{ relativePath: string; extension: string }>
  ): number {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    let created = 0;
    const now = Date.now();

    const insert = this.db.prepare(`
      INSERT OR IGNORE INTO file_metadata (file_id, relative_path, type, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?)
    `);

    const transaction = this.db.transaction(
      (batch: Array<{ relativePath: string; extension: string }>) => {
        for (const file of batch) {
          const fileId = generateFileId(file.relativePath);
          const type = inferFileType(file.extension);
          const info = insert.run(
            fileId,
            file.relativePath,
            type,
            now,
            now
          );
          if (info.changes > 0) {
            created += 1;
          }
        }
      }
    );

    transaction(files);
    return created;
  }

  /**
   * Get metadata for a file
   *
   * @param fileId - File ID
   * @returns FileMetadata or null
   */
  getMetadata(fileId: string): FileMetadata | null {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT * FROM file_metadata WHERE file_id = ?
    `);

    const row = stmt.get(fileId) as any;
    if (!row) {
      return null;
    }

    // Get tags
    const tagsStmt = this.db.prepare(`
      SELECT tag FROM file_tags WHERE file_id = ?
    `);
    const tags = (tagsStmt.all(fileId) as any[]).map((r) => r.tag);

    // Get contexts
    const contextsStmt = this.db.prepare(`
      SELECT context FROM file_contexts WHERE file_id = ?
    `);
    const contexts = (contextsStmt.all(fileId) as any[]).map((r) => r.context);

    const suggestionsStmt = this.db.prepare(`
      SELECT context FROM file_context_suggestions WHERE file_id = ?
    `);
    const suggestedContexts = (suggestionsStmt.all(fileId) as any[]).map(
      (r) => r.context
    );

    let aiKeyTerms: string[] | undefined;
    if (row.ai_key_terms) {
      try {
        aiKeyTerms = JSON.parse(row.ai_key_terms);
      } catch {
        aiKeyTerms = undefined;
      }
    }

    const mirror =
      row.mirror_format && row.mirror_path
        ? {
            format: row.mirror_format,
            path: row.mirror_path,
            sourceMtime: row.mirror_source_mtime || 0,
            updatedAt: row.mirror_updated_at || 0,
          }
        : undefined;

    return {
      file_id: row.file_id,
      relativePath: row.relative_path,
      tags,
      contexts,
      suggestedContexts,
      type: row.type,
      notes: row.notes,
      aiSummary: row.ai_summary ?? undefined,
      aiSummaryHash: row.ai_summary_hash ?? undefined,
      aiKeyTerms,
      mirror,
      created_at: row.created_at,
      updated_at: row.updated_at,
    };
  }

  /**
   * Get metadata by relative path
   *
   * @param relativePath - Workspace-relative path
   * @returns FileMetadata or null
   */
  getMetadataByPath(relativePath: string): FileMetadata | null {
    const fileId = generateFileId(relativePath);
    return this.getMetadata(fileId);
  }

  /**
   * Add tag to a file
   *
   * @param relativePath - Workspace-relative path
   * @param tag - Tag to add
   */
  addTag(relativePath: string, tag: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    // Insert tag (ignore if already exists)
    const stmt = this.db.prepare(`
      INSERT OR IGNORE INTO file_tags (file_id, tag)
      VALUES (?, ?)
    `);
    stmt.run(fileId, tag);

    this.touchFile(fileId);
  }

  /**
   * Remove tag from a file
   *
   * @param relativePath - Workspace-relative path
   * @param tag - Tag to remove
   */
  removeTag(relativePath: string, tag: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      DELETE FROM file_tags WHERE file_id = ? AND tag = ?
    `);
    stmt.run(fileId, tag);

    this.touchFile(fileId);
  }

  /**
   * Add context to a file
   *
   * @param relativePath - Workspace-relative path
   * @param context - Context to add
   */
  addContext(relativePath: string, context: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      INSERT OR IGNORE INTO file_contexts (file_id, context)
      VALUES (?, ?)
    `);
    stmt.run(fileId, context);

    this.touchFile(fileId);
  }

  /**
   * Remove context from a file
   *
   * @param relativePath - Workspace-relative path
   * @param context - Context to remove
   */
  removeContext(relativePath: string, context: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      DELETE FROM file_contexts WHERE file_id = ? AND context = ?
    `);
    stmt.run(fileId, context);

    this.touchFile(fileId);
  }

  /**
   * Add suggested context to a file
   *
   * @param relativePath - Workspace-relative path
   * @param context - Suggested context to add
   */
  addSuggestedContext(relativePath: string, context: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      INSERT OR IGNORE INTO file_context_suggestions (file_id, context)
      VALUES (?, ?)
    `);
    stmt.run(fileId, context);

    this.touchFile(fileId);
  }

  /**
   * Clear suggested contexts for a file
   *
   * @param relativePath - Workspace-relative path
   */
  clearSuggestedContexts(relativePath: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      DELETE FROM file_context_suggestions WHERE file_id = ?
    `);
    stmt.run(fileId);

    this.touchFile(fileId);
  }

  /**
   * Get suggested contexts for a file
   *
   * @param relativePath - Workspace-relative path
   * @returns Array of suggested contexts
   */
  getSuggestedContexts(relativePath: string): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);
    const stmt = this.db.prepare(`
      SELECT context FROM file_context_suggestions WHERE file_id = ?
    `);
    return (stmt.all(fileId) as any[]).map((r) => r.context);
  }

  /**
   * Update notes for a file
   *
   * @param relativePath - Workspace-relative path
   * @param notes - Notes text
   */
  updateNotes(relativePath: string, notes: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      UPDATE file_metadata SET notes = ?, updated_at = ?
      WHERE file_id = ?
    `);
    stmt.run(notes, Date.now(), fileId);
  }

  /**
   * Update cached AI summary data for a file
   *
   * @param relativePath - Workspace-relative path
   * @param summary - Summary text
   * @param summaryHash - Content hash for cache validation
   * @param keyTerms - Optional key terms
   */
  updateAISummary(
    relativePath: string,
    summary: string,
    summaryHash: string,
    keyTerms?: string[]
  ): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);
    const keyTermsValue =
      keyTerms && keyTerms.length > 0 ? JSON.stringify(keyTerms) : null;

    const stmt = this.db.prepare(`
      UPDATE file_metadata
      SET ai_summary = ?, ai_summary_hash = ?, ai_key_terms = ?, updated_at = ?
      WHERE file_id = ?
    `);
    stmt.run(summary, summaryHash, keyTermsValue, Date.now(), fileId);
  }

  /**
   * Update cached mirror metadata for a file
   */
  updateMirrorMetadata(relativePath: string, mirror: MirrorMetadata): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);
    const stmt = this.db.prepare(`
      UPDATE file_metadata
      SET mirror_format = ?, mirror_path = ?, mirror_source_mtime = ?, mirror_updated_at = ?, updated_at = ?
      WHERE file_id = ?
    `);
    stmt.run(
      mirror.format,
      mirror.path,
      mirror.sourceMtime,
      mirror.updatedAt,
      Date.now(),
      fileId
    );
  }

  /**
   * Clear cached mirror metadata for a file
   */
  clearMirrorMetadata(relativePath: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);
    const stmt = this.db.prepare(`
      UPDATE file_metadata
      SET mirror_format = NULL, mirror_path = NULL, mirror_source_mtime = NULL, mirror_updated_at = NULL, updated_at = ?
      WHERE file_id = ?
    `);
    stmt.run(Date.now(), fileId);
  }

  /**
   * Remove file metadata and related entries
   */
  removeFile(relativePath: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const fileId = generateFileId(relativePath);
    const stmt = this.db.prepare(`
      DELETE FROM file_metadata WHERE file_id = ?
    `);
    stmt.run(fileId);
  }

  /**
   * Get all files with a specific tag
   *
   * @param tag - Tag name
   * @returns Array of relative paths
   */
  getFilesByTag(tag: string): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT fm.relative_path
      FROM file_metadata fm
      JOIN file_tags ft ON fm.file_id = ft.file_id
      WHERE ft.tag = ?
    `);

    return (stmt.all(tag) as any[]).map((r) => r.relative_path);
  }

  /**
   * Get all files in a specific context
   *
   * @param context - Context name
   * @returns Array of relative paths
   */
  getFilesByContext(context: string): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT fm.relative_path
      FROM file_metadata fm
      JOIN file_contexts fc ON fm.file_id = fc.file_id
      WHERE fc.context = ?
    `);

    return (stmt.all(context) as any[]).map((r) => r.relative_path);
  }

  /**
   * Get all files with a specific suggested context
   *
   * @param context - Suggested context name
   * @returns Array of relative paths
   */
  getFilesBySuggestedContext(context: string): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT fm.relative_path
      FROM file_metadata fm
      JOIN file_context_suggestions fcs ON fm.file_id = fcs.file_id
      WHERE fcs.context = ?
    `);

    return (stmt.all(context) as any[]).map((r) => r.relative_path);
  }

  /**
   * Get all files of a specific type
   *
   * @param type - File type
   * @returns Array of relative paths
   */
  getFilesByType(type: string): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT relative_path FROM file_metadata WHERE type = ?
    `);

    return (stmt.all(type) as any[]).map((r) => r.relative_path);
  }

  /**
   * Get all unique tags
   *
   * @returns Array of tag names
   */
  getAllTags(): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT tag FROM file_tags ORDER BY tag
    `);

    return (stmt.all() as any[]).map((r) => r.tag);
  }

  /**
   * Get tag counts for all tags in a single query
   *
   * @returns Map of tag name to file count
   */
  getTagCounts(): Map<string, number> {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT tag, COUNT(DISTINCT file_id) as count
      FROM file_tags
      GROUP BY tag
      ORDER BY tag
    `);

    const results = stmt.all() as Array<{ tag: string; count: number }>;
    const counts = new Map<string, number>();
    for (const row of results) {
      counts.set(row.tag, row.count);
    }
    return counts;
  }

  /**
   * Get all unique contexts
   *
   * @returns Array of context names
   */
  getAllContexts(): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT context FROM file_contexts ORDER BY context
    `);

    return (stmt.all() as any[]).map((r) => r.context);
  }

  /**
   * Get all unique suggested contexts
   *
   * @returns Array of suggested context names
   */
  getAllSuggestedContexts(): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT context FROM file_context_suggestions ORDER BY context
    `);

    return (stmt.all() as any[]).map((r) => r.context);
  }

  /**
   * Get all unique types
   *
   * @returns Array of type names
   */
  getAllTypes(): string[] {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT type FROM file_metadata ORDER BY type
    `);

    return (stmt.all() as any[]).map((r) => r.type);
  }

  /**
   * Update the updated_at timestamp for a file
   *
   * @param fileId - File ID
   */
  private touchFile(fileId: string): void {
    if (!this.db) {
      throw new Error("Database not initialized");
    }

    const stmt = this.db.prepare(`
      UPDATE file_metadata SET updated_at = ? WHERE file_id = ?
    `);
    stmt.run(Date.now(), fileId);
  }

  /**
   * Close the database connection
   */
  close(): void {
    if (this.db) {
      this.db.close();
      this.db = null;
    }
  }
}
