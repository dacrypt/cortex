/**
 * MetadataStore - Persistent storage for file metadata using SQLite
 */

import * as path from 'path';
import * as fs from 'fs/promises';
import { FileMetadata } from '../models/types';
import { generateFileId, inferFileType } from '../utils/fileHash';

// We'll use better-sqlite3 for synchronous SQLite access
// Note: This requires the better-sqlite3 package
import Database from 'better-sqlite3';

export class MetadataStore {
  private db: Database.Database | null = null;
  private dbPath: string;
  private cortexDir: string;

  constructor(workspaceRoot: string) {
    this.cortexDir = path.join(workspaceRoot, '.cortex');
    this.dbPath = path.join(this.cortexDir, 'index.sqlite');
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
      throw new Error('Database not initialized');
    }

    // Main metadata table
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS file_metadata (
        file_id TEXT PRIMARY KEY,
        relative_path TEXT NOT NULL UNIQUE,
        type TEXT NOT NULL,
        notes TEXT,
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

    // Create indexes for performance
    this.db.exec(`
      CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag);
      CREATE INDEX IF NOT EXISTS idx_file_contexts_context ON file_contexts(context);
      CREATE INDEX IF NOT EXISTS idx_file_metadata_type ON file_metadata(type);
    `);
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
      throw new Error('Database not initialized');
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
      type,
      created_at: now,
      updated_at: now,
    };
  }

  /**
   * Get metadata for a file
   *
   * @param fileId - File ID
   * @returns FileMetadata or null
   */
  getMetadata(fileId: string): FileMetadata | null {
    if (!this.db) {
      throw new Error('Database not initialized');
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

    return {
      file_id: row.file_id,
      relativePath: row.relative_path,
      tags,
      contexts,
      type: row.type,
      notes: row.notes,
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
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      DELETE FROM file_contexts WHERE file_id = ? AND context = ?
    `);
    stmt.run(fileId, context);

    this.touchFile(fileId);
  }

  /**
   * Update notes for a file
   *
   * @param relativePath - Workspace-relative path
   * @param notes - Notes text
   */
  updateNotes(relativePath: string, notes: string): void {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    const fileId = generateFileId(relativePath);

    const stmt = this.db.prepare(`
      UPDATE file_metadata SET notes = ?, updated_at = ?
      WHERE file_id = ?
    `);
    stmt.run(notes, Date.now(), fileId);
  }

  /**
   * Get all files with a specific tag
   *
   * @param tag - Tag name
   * @returns Array of relative paths
   */
  getFilesByTag(tag: string): string[] {
    if (!this.db) {
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
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
   * Get all files of a specific type
   *
   * @param type - File type
   * @returns Array of relative paths
   */
  getFilesByType(type: string): string[] {
    if (!this.db) {
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT tag FROM file_tags ORDER BY tag
    `);

    return (stmt.all() as any[]).map((r) => r.tag);
  }

  /**
   * Get all unique contexts
   *
   * @returns Array of context names
   */
  getAllContexts(): string[] {
    if (!this.db) {
      throw new Error('Database not initialized');
    }

    const stmt = this.db.prepare(`
      SELECT DISTINCT context FROM file_contexts ORDER BY context
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
      throw new Error('Database not initialized');
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
      throw new Error('Database not initialized');
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
