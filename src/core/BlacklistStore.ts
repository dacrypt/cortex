import * as fs from 'fs/promises';
import * as path from 'path';

export type BlacklistEntryType = 'file' | 'dir';

export interface BlacklistEntry {
  path: string; // workspace-relative
  type: BlacklistEntryType;
  code: string;
  message: string;
  lastSeen: number;
  count: number;
}

interface BlacklistData {
  schemaVersion: number;
  entries: BlacklistEntry[];
}

export class BlacklistStore {
  private readonly schemaVersion = 1;
  private entries = new Map<string, BlacklistEntry>();
  private blacklistPath: string;
  private saveTimer: NodeJS.Timeout | undefined;

  constructor(private workspaceRoot: string) {
    const cortexDir = path.join(workspaceRoot, '.cortex');
    this.blacklistPath = path.join(cortexDir, 'blacklist.json');
  }

  async load(): Promise<void> {
    try {
      const raw = await fs.readFile(this.blacklistPath, 'utf-8');
      const data = JSON.parse(raw) as BlacklistData;
      if (data.schemaVersion !== this.schemaVersion) {
        return;
      }
      for (const entry of data.entries || []) {
        this.entries.set(this.key(entry.path, entry.type), entry);
      }
    } catch {
      // No-op when file missing or invalid
    }
  }

  getEntries(): BlacklistEntry[] {
    return Array.from(this.entries.values());
  }

  isBlacklisted(relativePath: string): boolean {
    if (!relativePath) {
      return false;
    }
    if (this.entries.has(this.key(relativePath, 'file'))) {
      return true;
    }
    for (const entry of this.entries.values()) {
      if (entry.type !== 'dir') continue;
      if (relativePath === entry.path || relativePath.startsWith(entry.path + path.sep)) {
        return true;
      }
    }
    return false;
  }

  markRelative(
    relativePath: string,
    type: BlacklistEntryType,
    code: string,
    message: string
  ): void {
    if (!relativePath || relativePath === '.' || relativePath === path.sep) {
      return;
    }
    const key = this.key(relativePath, type);
    const existing = this.entries.get(key);
    if (existing) {
      existing.count += 1;
      existing.lastSeen = Date.now();
      existing.code = code;
      existing.message = message;
    } else {
      this.entries.set(key, {
        path: relativePath,
        type,
        code,
        message,
        lastSeen: Date.now(),
        count: 1,
      });
    }
    this.scheduleSave();
  }

  markAbsolute(
    absolutePath: string,
    type: BlacklistEntryType,
    code: string,
    message: string
  ): void {
    const relativePath = path.relative(this.workspaceRoot, absolutePath);
    this.markRelative(relativePath, type, code, message);
  }

  private scheduleSave(): void {
    if (this.saveTimer) {
      clearTimeout(this.saveTimer);
    }
    this.saveTimer = setTimeout(() => {
      this.save().catch(() => undefined);
    }, 1500);
  }

  private async save(): Promise<void> {
    const cortexDir = path.dirname(this.blacklistPath);
    await fs.mkdir(cortexDir, { recursive: true });
    const payload: BlacklistData = {
      schemaVersion: this.schemaVersion,
      entries: this.getEntries(),
    };
    await fs.writeFile(this.blacklistPath, JSON.stringify(payload), 'utf-8');
  }

  private key(relativePath: string, type: BlacklistEntryType): string {
    return `${type}:${relativePath}`;
  }
}
