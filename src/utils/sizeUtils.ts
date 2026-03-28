/**
 * Size utility functions for categorizing files by size
 */

export function categorizeSize(sizeBytes: number): string {
  const kb = sizeBytes / 1024;
  const mb = kb / 1024;
  const gb = mb / 1024;

  if (sizeBytes === 0) {
    return 'Empty';
  } else if (sizeBytes < 1024) {
    return 'Tiny (< 1 KB)';
  } else if (kb < 100) {
    return 'Small (< 100 KB)';
  } else if (mb < 1) {
    return 'Medium (< 1 MB)';
  } else if (mb < 10) {
    return 'Large (< 10 MB)';
  } else if (mb < 100) {
    return 'Very Large (< 100 MB)';
  } else if (gb < 1) {
    return 'Huge (< 1 GB)';
  } else {
    return 'Massive (>= 1 GB)';
  }
}

export function formatSize(sizeBytes: number): string {
  const kb = sizeBytes / 1024;
  const mb = kb / 1024;
  const gb = mb / 1024;

  if (sizeBytes < 1024) {
    return `${sizeBytes} B`;
  } else if (kb < 1024) {
    return `${kb.toFixed(1)} KB`;
  } else if (mb < 1024) {
    return `${mb.toFixed(1)} MB`;
  } else {
    return `${gb.toFixed(2)} GB`;
  }
}


