import { execFile } from 'child_process';
import { promisify } from 'util';

const execFileAsync = promisify(execFile);

export const TAG_MAX_LENGTH = 32;
export const TAG_MAX_WORDS = 3;

export function isOsTaggingSupported(): boolean {
  if (process.env.CORTEX_DISABLE_OS_TAGS === '1') {
    return false;
  }
  return process.platform === 'darwin';
}

export function normalizeTag(tag: string): string | null {
  const trimmed = tag.trim().toLowerCase();
  if (!trimmed) {
    return null;
  }
  const withoutHash = trimmed.replace(/^#+/, '');
  const dashed = withoutHash.replace(/[\s_]+/g, '-');
  const cleaned = dashed.replace(/[^\p{L}\p{N}-]+/gu, '');
  const collapsed = cleaned.replace(/-+/g, '-').replace(/^-|-$/g, '');
  if (!collapsed || collapsed.length > TAG_MAX_LENGTH) {
    return null;
  }
  if (collapsed.split('-').length > TAG_MAX_WORDS) {
    return null;
  }
  return collapsed;
}

export function normalizeTags(tags: string[]): string[] {
  const normalized = new Set<string>();
  for (const tag of tags) {
    const value = normalizeTag(tag);
    if (value) {
      normalized.add(value);
    }
  }
  return Array.from(normalized);
}

export async function readOsTags(
  absolutePath: string
): Promise<string[] | null> {
  if (!isOsTaggingSupported()) {
    return null;
  }

  const { stdout } = await execFileAsync('mdls', [
    '-name',
    'kMDItemUserTags',
    '-raw',
    absolutePath,
  ]);
  const raw = stdout.trim();
  if (!raw || raw === '(null)' || raw === '()') {
    return [];
  }

  const matches = raw.match(/"([^"]*)"/g);
  if (!matches) {
    return [];
  }

  return matches.map((match) =>
    match
      .slice(1, -1)
      .replace(/\\n\\d+$/, '')
      .replace(/\n\d+$/, '')
  );
}

export async function writeOsTags(
  absolutePath: string,
  tags: string[]
): Promise<void> {
  if (!isOsTaggingSupported()) {
    return;
  }

  const escapedPath = absolutePath
    .replace(/\\/g, '\\\\')
    .replace(/"/g, '\\"');
  const tagList = tags
    .map((tag) => `"${tag.replace(/"/g, '\\"')}"`)
    .join(', ');
  const script = `tell application "Finder" to set tags of (POSIX file "${escapedPath}") to {${tagList}}`;

  await execFileAsync('osascript', ['-e', script]);
}
