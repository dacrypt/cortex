/**
 * FileTypeDetector - Deep file type and content analysis
 */

import * as fs from 'fs/promises';
import * as path from 'path';

export interface MimeTypeInfo {
  mimeType: string; // e.g., "text/html", "image/png"
  category: 'text' | 'image' | 'video' | 'audio' | 'archive' | 'document' | 'code' | 'binary';
  isBinary: boolean;
  encoding?: string; // UTF-8, ASCII, etc.
}

export interface TextMetadata {
  lineCount: number;
  charCount: number;
  wordCount: number;
  blankLines: number;
  encoding: string;
  lineEnding: 'LF' | 'CRLF' | 'CR' | 'MIXED';
  longestLine: number;
}

export interface CodeMetadata {
  linesOfCode: number;
  commentLines: number;
  blankLines: number;
  commentPercentage: number;
  imports: number;
  exports: number;
  functions: number;
  classes: number;
}

export interface ImageMetadata {
  width?: number;
  height?: number;
  aspectRatio?: string;
  colorDepth?: number;
  format: string;
  isVector: boolean;
}

export interface DocumentMetadata {
  author?: string;
  title?: string;
  subject?: string;
  creator?: string;
  creationDate?: number;
  modificationDate?: number;
  pageCount?: number;
  wordCount?: number;
}

export class FileTypeDetector {
  private readonly extensionMimeMap: Record<
    string,
    { mime: string; category: MimeTypeInfo['category'] }
  > = {
    '.html': { mime: 'text/html', category: 'text' },
    '.htm': { mime: 'text/html', category: 'text' },
    '.css': { mime: 'text/css', category: 'text' },
    '.js': { mime: 'text/javascript', category: 'code' },
    '.ts': { mime: 'text/typescript', category: 'code' },
    '.json': { mime: 'application/json', category: 'text' },
    '.xml': { mime: 'application/xml', category: 'text' },
    '.md': { mime: 'text/markdown', category: 'text' },
    '.txt': { mime: 'text/plain', category: 'text' },
    '.png': { mime: 'image/png', category: 'image' },
    '.jpg': { mime: 'image/jpeg', category: 'image' },
    '.jpeg': { mime: 'image/jpeg', category: 'image' },
    '.gif': { mime: 'image/gif', category: 'image' },
    '.svg': { mime: 'image/svg+xml', category: 'image' },
    '.pdf': { mime: 'application/pdf', category: 'document' },
    '.zip': { mime: 'application/zip', category: 'archive' },
    '.mp3': { mime: 'audio/mpeg', category: 'audio' },
    '.mp4': { mime: 'video/mp4', category: 'video' },
  };

  /**
   * Detect MIME type from file content (magic bytes)
   */
  async detectMimeType(absolutePath: string): Promise<MimeTypeInfo> {
    try {
      const ext = path.extname(absolutePath).toLowerCase();
      if (this.extensionMimeMap[ext]) {
        return this.detectFromExtension(ext);
      }

      // Read first 512 bytes for magic byte detection
      const handle = await fs.open(absolutePath, 'r');
      const buffer = Buffer.alloc(512);
      await handle.read(buffer, 0, 512, 0);
      await handle.close();

      const mimeType = this.detectFromMagicBytes(buffer);
      const category = this.categorizeMimeType(mimeType);
      const isBinary = this.isBinaryContent(buffer);
      const encoding = isBinary ? undefined : this.detectEncoding(buffer);

      return { mimeType, category, isBinary, encoding };
    } catch (error) {
      // Fallback to extension-based detection
      const ext = path.extname(absolutePath).toLowerCase();
      return this.detectFromExtension(ext);
    }
  }

  /**
   * Detect MIME type from magic bytes (file signature)
   */
  private detectFromMagicBytes(buffer: Buffer): string {
    // PNG
    if (
      buffer[0] === 0x89 &&
      buffer[1] === 0x50 &&
      buffer[2] === 0x4e &&
      buffer[3] === 0x47
    ) {
      return 'image/png';
    }

    // JPEG
    if (buffer[0] === 0xff && buffer[1] === 0xd8 && buffer[2] === 0xff) {
      return 'image/jpeg';
    }

    // GIF
    if (
      buffer[0] === 0x47 &&
      buffer[1] === 0x49 &&
      buffer[2] === 0x46 &&
      buffer[3] === 0x38
    ) {
      return 'image/gif';
    }

    // PDF
    if (
      buffer[0] === 0x25 &&
      buffer[1] === 0x50 &&
      buffer[2] === 0x44 &&
      buffer[3] === 0x46
    ) {
      return 'application/pdf';
    }

    // ZIP (also used by DOCX, XLSX, etc.)
    if (buffer[0] === 0x50 && buffer[1] === 0x4b) {
      const str = buffer.toString('utf8', 0, 100);
      if (str.includes('word/')) return 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';
      if (str.includes('xl/')) return 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
      if (str.includes('ppt/')) return 'application/vnd.openxmlformats-officedocument.presentationml.presentation';
      return 'application/zip';
    }

    // GZIP
    if (buffer[0] === 0x1f && buffer[1] === 0x8b) {
      return 'application/gzip';
    }

    // TAR
    if (buffer[257] === 0x75 && buffer[258] === 0x73 && buffer[259] === 0x74 && buffer[260] === 0x61 && buffer[261] === 0x72) {
      return 'application/x-tar';
    }

    // MP3
    if (
      (buffer[0] === 0x49 && buffer[1] === 0x44 && buffer[2] === 0x33) || // ID3
      (buffer[0] === 0xff && (buffer[1] & 0xe0) === 0xe0) // MPEG frame sync
    ) {
      return 'audio/mpeg';
    }

    // WAV
    if (
      buffer[0] === 0x52 &&
      buffer[1] === 0x49 &&
      buffer[2] === 0x46 &&
      buffer[3] === 0x46 &&
      buffer[8] === 0x57 &&
      buffer[9] === 0x41 &&
      buffer[10] === 0x56 &&
      buffer[11] === 0x45
    ) {
      return 'audio/wav';
    }

    // MP4/M4V
    if (buffer.toString('utf8', 4, 8) === 'ftyp') {
      return 'video/mp4';
    }

    // WebM
    if (
      buffer[0] === 0x1a &&
      buffer[1] === 0x45 &&
      buffer[2] === 0xdf &&
      buffer[3] === 0xa3
    ) {
      return 'video/webm';
    }

    // SQLite
    if (buffer.toString('utf8', 0, 13) === 'SQLite format') {
      return 'application/x-sqlite3';
    }

    // Text-based detection
    if (this.isTextContent(buffer)) {
      const str = buffer.toString('utf8', 0, 100).toLowerCase();

      // HTML
      if (str.includes('<!doctype html') || str.includes('<html')) {
        return 'text/html';
      }

      // XML
      if (str.startsWith('<?xml')) {
        return 'application/xml';
      }

      // JSON
      if (str.trim().startsWith('{') || str.trim().startsWith('[')) {
        try {
          JSON.parse(buffer.toString('utf8'));
          return 'application/json';
        } catch {
          // Not valid JSON
        }
      }

      // SVG
      if (str.includes('<svg')) {
        return 'image/svg+xml';
      }

      return 'text/plain';
    }

    return 'application/octet-stream'; // Binary fallback
  }

  /**
   * Detect MIME type from file extension (fallback)
   */
  private detectFromExtension(ext: string): MimeTypeInfo {
    const info = this.extensionMimeMap[ext] || {
      mime: 'application/octet-stream',
      category: 'binary',
    };

    return {
      mimeType: info.mime,
      category: info.category,
      isBinary: info.category === 'binary' || info.category === 'image' || info.category === 'video' || info.category === 'audio',
      encoding: undefined,
    };
  }

  /**
   * Categorize MIME type
   */
  private categorizeMimeType(mimeType: string): MimeTypeInfo['category'] {
    if (mimeType.startsWith('text/')) return 'text';
    if (mimeType.startsWith('image/')) return 'image';
    if (mimeType.startsWith('video/')) return 'video';
    if (mimeType.startsWith('audio/')) return 'audio';
    if (mimeType.includes('zip') || mimeType.includes('tar') || mimeType.includes('gzip')) return 'archive';
    if (mimeType.includes('pdf') || mimeType.includes('document') || mimeType.includes('word')) return 'document';
    if (mimeType.includes('javascript') || mimeType.includes('typescript') || mimeType.includes('python')) return 'code';
    return 'binary';
  }

  /**
   * Check if content is binary
   */
  private isBinaryContent(buffer: Buffer): boolean {
    // Check for null bytes (common in binary files)
    for (let i = 0; i < Math.min(512, buffer.length); i++) {
      if (buffer[i] === 0) {
        return true;
      }
    }
    return false;
  }

  /**
   * Check if content is text
   */
  private isTextContent(buffer: Buffer): boolean {
    return !this.isBinaryContent(buffer);
  }

  /**
   * Detect text encoding
   */
  private detectEncoding(buffer: Buffer): string {
    // Check for BOM (Byte Order Mark)
    if (buffer[0] === 0xef && buffer[1] === 0xbb && buffer[2] === 0xbf) {
      return 'UTF-8';
    }
    if (buffer[0] === 0xff && buffer[1] === 0xfe) {
      return 'UTF-16LE';
    }
    if (buffer[0] === 0xfe && buffer[1] === 0xff) {
      return 'UTF-16BE';
    }

    // Check if valid UTF-8
    try {
      buffer.toString('utf8');
      return 'UTF-8';
    } catch {
      return 'ASCII';
    }
  }

  /**
   * Analyze text file
   */
  async analyzeTextFile(absolutePath: string): Promise<TextMetadata> {
    const content = await fs.readFile(absolutePath, 'utf8');
    const lines = content.split(/\r?\n/);

    const lineCount = lines.length;
    const charCount = content.length;
    const wordCount = content.split(/\s+/).filter((w) => w.length > 0).length;
    const blankLines = lines.filter((line) => line.trim().length === 0).length;

    // Detect line ending
    let lineEnding: TextMetadata['lineEnding'] = 'LF';
    const hasCRLF = content.includes('\r\n');
    const hasCR = content.includes('\r') && !hasCRLF;
    if (hasCRLF && hasCR) lineEnding = 'MIXED';
    else if (hasCRLF) lineEnding = 'CRLF';
    else if (hasCR) lineEnding = 'CR';

    const longestLine = Math.max(...lines.map((line) => line.length));

    return {
      lineCount,
      charCount,
      wordCount,
      blankLines,
      encoding: 'UTF-8', // Already read as UTF-8
      lineEnding,
      longestLine,
    };
  }

  /**
   * Analyze code file
   */
  async analyzeCodeFile(absolutePath: string, language: string): Promise<CodeMetadata> {
    const content = await fs.readFile(absolutePath, 'utf8');
    const lines = content.split(/\r?\n/);

    let commentLines = 0;
    let blankLines = 0;
    let imports = 0;
    let exports = 0;
    let functions = 0;
    let classes = 0;

    const commentPatterns = this.getCommentPatterns(language);

    for (const line of lines) {
      const trimmed = line.trim();

      if (trimmed.length === 0) {
        blankLines++;
        continue;
      }

      // Check for comments
      if (commentPatterns.some((pattern) => pattern.test(trimmed))) {
        commentLines++;
        continue;
      }

      // Count imports/exports (JS/TS)
      if (/^import\s/.test(trimmed)) imports++;
      if (/^export\s/.test(trimmed)) exports++;

      // Count functions
      if (/(function\s|const\s+\w+\s*=\s*\(|function\*)/.test(trimmed)) {
        functions++;
      }

      // Count classes
      if (/^class\s/.test(trimmed)) classes++;
    }

    const linesOfCode = lines.length - commentLines - blankLines;
    const commentPercentage =
      lines.length > 0 ? (commentLines / lines.length) * 100 : 0;

    return {
      linesOfCode,
      commentLines,
      blankLines,
      commentPercentage,
      imports,
      exports,
      functions,
      classes,
    };
  }

  /**
   * Get comment patterns for language
   */
  private getCommentPatterns(language: string): RegExp[] {
    const patterns: Record<string, RegExp[]> = {
      javascript: [/^\/\//, /^\/\*/, /^\*/],
      typescript: [/^\/\//, /^\/\*/, /^\*/],
      python: [/^#/],
      ruby: [/^#/],
      java: [/^\/\//, /^\/\*/, /^\*/],
      cpp: [/^\/\//, /^\/\*/, /^\*/],
      c: [/^\/\//, /^\/\*/, /^\*/],
      go: [/^\/\//, /^\/\*/, /^\*/],
      rust: [/^\/\//, /^\/\*/, /^\*/],
      css: [/^\/\*/, /^\*/],
      html: [/^<!--/],
      xml: [/^<!--/],
    };

    return patterns[language.toLowerCase()] || [/^\/\//];
  }

  /**
   * Analyze image file (basic detection without external libraries)
   */
  async analyzeImageFile(absolutePath: string): Promise<ImageMetadata> {
    const buffer = await fs.readFile(absolutePath);
    const ext = path.extname(absolutePath).toLowerCase();

    let width: number | undefined;
    let height: number | undefined;
    let format = ext.substring(1).toUpperCase();
    let isVector = false;

    // PNG dimensions
    if (buffer[0] === 0x89 && buffer[1] === 0x50) {
      width = buffer.readUInt32BE(16);
      height = buffer.readUInt32BE(20);
      format = 'PNG';
    }

    // JPEG dimensions (simplified)
    if (buffer[0] === 0xff && buffer[1] === 0xd8) {
      // JPEG parsing is complex, skip for now
      format = 'JPEG';
    }

    // GIF dimensions
    if (buffer[0] === 0x47 && buffer[1] === 0x49) {
      width = buffer.readUInt16LE(6);
      height = buffer.readUInt16LE(8);
      format = 'GIF';
    }

    // SVG (vector)
    if (ext === '.svg') {
      isVector = true;
      format = 'SVG';
    }

    const aspectRatio =
      width && height ? `${width}:${height}` : undefined;

    return {
      width,
      height,
      aspectRatio,
      format,
      isVector,
    };
  }
}
