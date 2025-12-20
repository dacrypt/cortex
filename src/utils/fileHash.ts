/**
 * Utilities for generating stable file identifiers
 */

import * as crypto from 'crypto';

/**
 * Generate a stable file ID from a relative path
 * Uses SHA-256 hash to create a deterministic identifier
 *
 * @param relativePath - Workspace-relative path
 * @returns Hexadecimal hash string
 */
export function generateFileId(relativePath: string): string {
  return crypto
    .createHash('sha256')
    .update(relativePath)
    .digest('hex');
}

/**
 * Infer file type from extension
 *
 * @param extension - File extension (with or without dot)
 * @returns Normalized file type
 */
export function inferFileType(extension: string): string {
  const ext = extension.toLowerCase().replace(/^\./, '');

  // Map extensions to semantic types
  const typeMap: Record<string, string> = {
    // Code
    'ts': 'typescript',
    'tsx': 'typescript',
    'js': 'javascript',
    'jsx': 'javascript',
    'py': 'python',
    'java': 'java',
    'cpp': 'cpp',
    'c': 'c',
    'go': 'go',
    'rs': 'rust',
    'rb': 'ruby',
    'php': 'php',

    // Markup & Config
    'html': 'html',
    'css': 'css',
    'scss': 'scss',
    'json': 'json',
    'yaml': 'yaml',
    'yml': 'yaml',
    'xml': 'xml',
    'toml': 'toml',

    // Documents
    'md': 'markdown',
    'pdf': 'pdf',
    'doc': 'word',
    'docx': 'word',
    'txt': 'text',

    // Images
    'png': 'image',
    'jpg': 'image',
    'jpeg': 'image',
    'gif': 'image',
    'svg': 'image',
    'webp': 'image',

    // Data
    'csv': 'data',
    'sql': 'sql',
    'db': 'database',
    'sqlite': 'database',
  };

  return typeMap[ext] || ext;
}
