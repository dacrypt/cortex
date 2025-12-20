/**
 * FileScanner - Responsible for scanning workspace files
 */

import * as path from 'path';
import * as fs from 'fs/promises';
import { FileIndexEntry } from '../models/types';
import { BlacklistStore } from './BlacklistStore';

export class FileScanner {
  private workspaceRoot: string;

  // Directories to ignore during scanning
  private readonly IGNORE_DIRS = [
    '.git',
    'node_modules',
    '.vscode',
    '.cortex',
    'dist',
    'build',
    'out',
    '.next',
    'target',
    'bin',
    'obj',
  ];

  constructor(workspaceRoot: string, private blacklistStore?: BlacklistStore) {
    this.workspaceRoot = workspaceRoot;
  }

  /**
   * Scan the entire workspace and return all files
   * Ignores specified directories
   *
   * @returns Array of FileIndexEntry
   */
  async scanWorkspace(
    onProgress?: (processed: number) => void
  ): Promise<FileIndexEntry[]> {
    const files: FileIndexEntry[] = [];
    const progressState = { processed: 0 };
    console.log(`[FileScanner] Starting scan at ${this.workspaceRoot}`);
    await this.scanDirectory(this.workspaceRoot, files, progressState, onProgress);
    console.log(`[FileScanner] Scan complete: ${progressState.processed} files`);
    return files;
  }

  /**
   * Recursively scan a directory
   *
   * @param dirPath - Absolute path to directory
   * @param files - Accumulator for results
   */
  private async scanDirectory(
    dirPath: string,
    files: FileIndexEntry[],
    progressState: { processed: number },
    onProgress?: (processed: number) => void
  ): Promise<void> {
    try {
      const entries = await fs.readdir(dirPath, { withFileTypes: true });

      for (const entry of entries) {
        const fullPath = path.join(dirPath, entry.name);

        const relativePath = path.relative(this.workspaceRoot, fullPath);

        if (this.blacklistStore?.isBlacklisted(relativePath)) {
          continue;
        }

        // Skip ignored directories
        if (entry.isDirectory()) {
          if (this.IGNORE_DIRS.includes(entry.name)) {
            continue;
          }
          // Recursively scan subdirectory
          await this.scanDirectory(fullPath, files, progressState, onProgress);
        } else if (entry.isFile()) {
          // Add file to index
          const fileEntry = await this.createFileEntry(fullPath);
          if (fileEntry) {
            files.push(fileEntry);
            progressState.processed += 1;
            if (progressState.processed % 200 === 0) {
              onProgress?.(progressState.processed);
            }
          }
        }
      }
      onProgress?.(progressState.processed);
    } catch (error) {
      const err = error as NodeJS.ErrnoException;
      console.error(
        `[FileScanner] Error scanning directory: ${dirPath} (${err.code || 'UNKNOWN'})`,
        err.message || err
      );
      if (this.blacklistStore && err.code) {
        const relativeDir = path.relative(this.workspaceRoot, dirPath);
        this.blacklistStore.markRelative(
          relativeDir,
          'dir',
          err.code,
          err.message || 'Unknown error'
        );
      }
    }
  }

  /**
   * Create a FileIndexEntry from a file path
   *
   * @param absolutePath - Absolute path to file
   * @returns FileIndexEntry or null if error
   */
  private async createFileEntry(
    absolutePath: string
  ): Promise<FileIndexEntry | null> {
    try {
      const relativePath = path.relative(this.workspaceRoot, absolutePath);
      const stats = await fs.stat(absolutePath);
      const filename = path.basename(absolutePath);
      const extension = path.extname(absolutePath);

      return {
        absolutePath,
        relativePath,
        filename,
        extension,
        lastModified: stats.mtimeMs,
        fileSize: stats.size,
      };
    } catch (error) {
      const err = error as NodeJS.ErrnoException;
      const relativePath = path.relative(this.workspaceRoot, absolutePath);
      console.error(`Error reading file ${absolutePath}:`, err);
      if (this.blacklistStore && err.code) {
        this.blacklistStore.markRelative(
          relativePath,
          'file',
          err.code,
          err.message || 'Unknown error'
        );
      }
      return null;
    }
  }

  /**
   * Get file entry for a specific path
   *
   * @param absolutePath - Absolute path to file
   * @returns FileIndexEntry or null
   */
  async getFileEntry(absolutePath: string): Promise<FileIndexEntry | null> {
    return this.createFileEntry(absolutePath);
  }
}
