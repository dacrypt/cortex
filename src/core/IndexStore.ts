/**
 * IndexStore - In-memory index of all workspace files
 */

import { FileIndexEntry } from '../models/types';

export class IndexStore {
  // Map: relativePath -> FileIndexEntry
  private index: Map<string, FileIndexEntry> = new Map();

  /**
   * Build the index from scanned files
   *
   * @param files - Array of FileIndexEntry from scanner
   */
  buildIndex(files: FileIndexEntry[]): void {
    this.index.clear();

    for (const file of files) {
      this.index.set(file.relativePath, file);
    }

    console.log(`[IndexStore] Built index with ${this.index.size} files`);
  }

  /**
   * Get all files in the index
   *
   * @returns Array of all FileIndexEntry
   */
  getAllFiles(): FileIndexEntry[] {
    return Array.from(this.index.values());
  }

  /**
   * Get a specific file by relative path
   *
   * @param relativePath - Workspace-relative path
   * @returns FileIndexEntry or undefined
   */
  getFile(relativePath: string): FileIndexEntry | undefined {
    return this.index.get(relativePath);
  }

  /**
   * Add or update a file in the index
   *
   * @param file - FileIndexEntry to add/update
   */
  upsertFile(file: FileIndexEntry): void {
    this.index.set(file.relativePath, file);
  }

  /**
   * Remove a file from the index
   *
   * @param relativePath - Workspace-relative path
   */
  removeFile(relativePath: string): void {
    this.index.delete(relativePath);
  }

  /**
   * Get files by extension
   *
   * @param extension - File extension (with or without dot)
   * @returns Array of matching files
   */
  getFilesByExtension(extension: string): FileIndexEntry[] {
    const normalizedExt = extension.startsWith('.')
      ? extension
      : `.${extension}`;

    return this.getAllFiles().filter(
      (file) => file.extension === normalizedExt
    );
  }

  /**
   * Search files by filename pattern
   *
   * @param pattern - Search pattern (case-insensitive)
   * @returns Array of matching files
   */
  searchFiles(pattern: string): FileIndexEntry[] {
    const lowerPattern = pattern.toLowerCase();

    return this.getAllFiles().filter((file) =>
      file.filename.toLowerCase().includes(lowerPattern)
    );
  }

  /**
   * Get index statistics
   *
   * @returns Stats object
   */
  getStats(): {
    totalFiles: number;
    totalSize: number;
    extensionCounts: Record<string, number>;
  } {
    const files = this.getAllFiles();
    const extensionCounts: Record<string, number> = {};

    let totalSize = 0;

    for (const file of files) {
      totalSize += file.fileSize;

      const ext = file.extension || '(no extension)';
      extensionCounts[ext] = (extensionCounts[ext] || 0) + 1;
    }

    return {
      totalFiles: files.length,
      totalSize,
      extensionCounts,
    };
  }

  /**
   * Clear the entire index
   */
  clear(): void {
    this.index.clear();
  }
}
