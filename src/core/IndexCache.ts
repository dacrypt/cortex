import * as fs from 'fs/promises';
import * as path from 'path';
import { FileIndexEntry } from '../models/types';

interface IndexCacheData {
  schemaVersion: number;
  entries: FileIndexEntry[];
}

export class IndexCache {
  private cachePath: string;
  private cortexDir: string;
  private readonly schemaVersion = 1;

  constructor(workspaceRoot: string) {
    this.cortexDir = path.join(workspaceRoot, '.cortex');
    this.cachePath = path.join(this.cortexDir, 'index-cache.json');
  }

  async load(): Promise<FileIndexEntry[]> {
    try {
      const raw = await fs.readFile(this.cachePath, 'utf-8');
      const data = JSON.parse(raw) as IndexCacheData;
      if (data.schemaVersion !== this.schemaVersion) {
        return [];
      }
      return Array.isArray(data.entries) ? data.entries : [];
    } catch {
      return [];
    }
  }

  async save(entries: FileIndexEntry[]): Promise<void> {
    await fs.mkdir(this.cortexDir, { recursive: true });
    const payload: IndexCacheData = {
      schemaVersion: this.schemaVersion,
      entries,
    };
    await fs.writeFile(this.cachePath, JSON.stringify(payload), 'utf-8');
  }
}
