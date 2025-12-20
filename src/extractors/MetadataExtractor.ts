/**
 * MetadataExtractor - Extracts rich metadata from files
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import { exec } from 'child_process';
import { promisify } from 'util';
import {
  FileTypeDetector,
  MimeTypeInfo,
  TextMetadata,
  CodeMetadata,
  ImageMetadata,
} from './FileTypeDetector';
import {
  DocumentDetector,
  DocumentMetadata,
  SpreadsheetMetadata,
  PresentationMetadata,
  DesignMetadata,
  PDFMetadata,
} from './DocumentDetector';

const execAsync = promisify(exec);

export interface FileStats {
  size: number;
  created: number;
  modified: number;
  accessed: number;
  isReadOnly: boolean;
  isHidden: boolean;
}

export interface GitMetadata {
  lastAuthor?: string;
  lastCommitDate?: number;
  lastCommitMessage?: string;
  branch?: string;
}

export interface EnhancedMetadata {
  stats: FileStats;
  git?: GitMetadata;
  folder: string;
  depth: number; // Folder depth
  language?: string; // Programming language
  indexed?: {
    basic?: boolean;
    mime?: boolean;
    code?: boolean;
    document?: boolean;
  };

  // Deep file analysis
  mimeType?: MimeTypeInfo;
  textMetadata?: TextMetadata;
  codeMetadata?: CodeMetadata;
  imageMetadata?: ImageMetadata;

  // Document metadata
  documentMetadata?: DocumentMetadata | SpreadsheetMetadata | PresentationMetadata | PDFMetadata;
  designMetadata?: DesignMetadata;

  // Error tracking
  error?: {
    code: string;
    message: string;
    operation: string; // What operation failed
  };
}

export class MetadataExtractor {
  private detector: FileTypeDetector;
  private documentDetector: DocumentDetector;

  constructor(private workspaceRoot: string) {
    this.detector = new FileTypeDetector();
    this.documentDetector = new DocumentDetector();
  }

  /**
   * Extract file statistics
   */
  async extractStats(absolutePath: string): Promise<FileStats> {
    try {
      const stats = await fs.stat(absolutePath);
      const filename = path.basename(absolutePath);

      return {
        size: stats.size,
        created: stats.birthtimeMs,
        modified: stats.mtimeMs,
        accessed: stats.atimeMs,
        isReadOnly: (stats.mode & 0o200) === 0, // Check write permission
        isHidden: filename.startsWith('.'),
      };
    } catch (error: any) {
      // Log specific errors for debugging
      if (error.code === 'ENAMETOOLONG') {
        console.warn(`[MetadataExtractor] Path too long (${absolutePath.length} chars):`, absolutePath.substring(0, 100) + '...');
      } else if (error.code === 'ENOENT') {
        console.warn(`[MetadataExtractor] File not found:`, absolutePath);
      } else if (error.code === 'EACCES') {
        console.warn(`[MetadataExtractor] Permission denied:`, absolutePath);
      } else {
        console.warn(`[MetadataExtractor] Error reading stats for ${path.basename(absolutePath)}:`, error.code || error.message);
      }

      // Return defaults - file will still be indexed with error info
      return {
        size: 0,
        created: 0,
        modified: 0,
        accessed: 0,
        isReadOnly: false,
        isHidden: false,
      };
    }
  }

  /**
   * Extract Git metadata for a file
   */
  async extractGitMetadata(
    absolutePath: string
  ): Promise<GitMetadata | undefined> {
    try {
      // Check if in git repo
      const relativePath = path.relative(this.workspaceRoot, absolutePath);

      // Get last commit info for this file
      const { stdout: logOutput } = await execAsync(
        `git log -1 --format="%an|%at|%s" -- "${relativePath}"`,
        { cwd: this.workspaceRoot }
      );

      if (!logOutput.trim()) {
        return undefined;
      }

      const [author, timestamp, message] = logOutput.trim().split('|');

      // Get current branch
      const { stdout: branchOutput } = await execAsync(
        'git rev-parse --abbrev-ref HEAD',
        { cwd: this.workspaceRoot }
      );

      return {
        lastAuthor: author,
        lastCommitDate: parseInt(timestamp) * 1000,
        lastCommitMessage: message,
        branch: branchOutput.trim(),
      };
    } catch (error) {
      // Not a git repo or file not tracked
      return undefined;
    }
  }

  /**
   * Extract folder and depth information
   */
  extractFolderInfo(relativePath: string): { folder: string; depth: number } {
    const folder = path.dirname(relativePath);
    const depth = folder === '.' ? 0 : folder.split(path.sep).length;

    return { folder, depth };
  }

  /**
   * Infer programming language from file extension
   */
  inferLanguage(extension: string): string | undefined {
    const ext = extension.toLowerCase().replace(/^\./, '');

    const languageMap: Record<string, string> = {
      ts: 'TypeScript',
      tsx: 'TypeScript',
      js: 'JavaScript',
      jsx: 'JavaScript',
      py: 'Python',
      java: 'Java',
      cpp: 'C++',
      c: 'C',
      go: 'Go',
      rs: 'Rust',
      rb: 'Ruby',
      php: 'PHP',
      swift: 'Swift',
      kt: 'Kotlin',
      cs: 'C#',
      html: 'HTML',
      css: 'CSS',
      scss: 'SCSS',
      sql: 'SQL',
      sh: 'Shell',
      bash: 'Shell',
      md: 'Markdown',
    };

    return languageMap[ext];
  }

  /**
   * Extract basic metadata (fast, synchronous where possible)
   */
  async extractBasic(
    absolutePath: string,
    relativePath: string,
    extension: string
  ): Promise<EnhancedMetadata> {
    const { folder, depth } = this.extractFolderInfo(relativePath);
    const language = this.inferLanguage(extension);

    let stats: FileStats;
    let error: any = undefined;

    try {
      stats = await this.extractStats(absolutePath);
    } catch (err: any) {
      // Capture the error for the Issues view
      error = {
        code: err.code || 'UNKNOWN',
        message: err.message || 'Unknown error',
        operation: 'extractStats',
      };

      // Use default stats
      stats = {
        size: 0,
        created: 0,
        modified: 0,
        accessed: 0,
        isReadOnly: false,
        isHidden: false,
      };
    }

    const metadata: EnhancedMetadata = {
      stats,
      folder,
      depth,
      language,
      indexed: {
        basic: true,
      },
    };

    if (error) {
      metadata.error = error;
    }

    return metadata;
  }

  /**
   * Extract MIME type metadata (for ContentType view)
   */
  async extractMimeTypeMetadata(
    absolutePath: string,
    enhanced: EnhancedMetadata
  ): Promise<void> {
    try {
      const mimeType = await this.detector.detectMimeType(absolutePath);

      // Override MIME category to 'code' if we detected a programming language
      if (enhanced.language && (mimeType.category === 'text' || mimeType.category === 'binary')) {
        mimeType.category = 'code';
      }

      enhanced.mimeType = mimeType;
    } catch (error) {
      console.warn(`Failed to extract MIME type for ${absolutePath}:`, error);
    } finally {
      if (!enhanced.indexed) {
        enhanced.indexed = {};
      }
      enhanced.indexed.mime = true;
    }
  }

  /**
   * Extract code metrics (for CodeMetrics view)
   */
  async extractCodeMetadata(
    absolutePath: string,
    enhanced: EnhancedMetadata
  ): Promise<void> {
    try {
      // Debug checks
      if (!enhanced.language) {
        console.log(`[MetadataExtractor] Skipping ${absolutePath}: No language detected`);
        return;
      }

      if (enhanced.stats.size >= 1024 * 1024) {
        console.log(`[MetadataExtractor] Skipping ${absolutePath}: File too large (${enhanced.stats.size} bytes)`);
        return;
      }

      console.log(`[MetadataExtractor] Processing ${absolutePath} (${enhanced.language}, ${enhanced.stats.size} bytes)`);

      // Extract text metadata first (needed for code metrics)
      if (!enhanced.textMetadata) {
        console.log(`[MetadataExtractor] Extracting text metadata first...`);
        enhanced.textMetadata = await this.detector.analyzeTextFile(absolutePath);
        console.log(`[MetadataExtractor] Text metadata: ${enhanced.textMetadata.lineCount} lines`);
      }

      // Extract code metrics
      console.log(`[MetadataExtractor] Extracting code metrics...`);
      enhanced.codeMetadata = await this.detector.analyzeCodeFile(
        absolutePath,
        enhanced.language
      );
      console.log(`[MetadataExtractor] ✓ Code metrics: ${enhanced.codeMetadata.linesOfCode} LOC, ${enhanced.codeMetadata.commentPercentage.toFixed(1)}% comments`);
      if (!enhanced.indexed) {
        enhanced.indexed = {};
      }
      enhanced.indexed.code = true;
    } catch (error) {
      console.error(`[MetadataExtractor] ERROR extracting code metadata for ${absolutePath}:`, error);
      throw error; // Re-throw to see in caller
    }
  }

  /**
   * Extract image metadata (for image-related views)
   */
  async extractImageMetadata(
    absolutePath: string,
    enhanced: EnhancedMetadata
  ): Promise<void> {
    try {
      // Only extract if MIME type indicates image
      if (enhanced.mimeType?.category === 'image') {
        enhanced.imageMetadata = await this.detector.analyzeImageFile(absolutePath);
      }
    } catch (error) {
      console.warn(`Failed to extract image metadata for ${absolutePath}:`, error);
    }
  }

  /**
   * Extract document metadata (for Office documents, PDFs, design files)
   */
  async extractDocumentMetadata(
    absolutePath: string,
    enhanced: EnhancedMetadata,
    extension: string
  ): Promise<void> {
    try {
      const ext = extension.toLowerCase();

      // Word documents
      if (ext === '.docx') {
        console.log(`[MetadataExtractor] Extracting Word metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractWordMetadata(absolutePath);
        console.log(`[MetadataExtractor] ✓ Word: ${enhanced.documentMetadata.pageCount || '?'} pages, ${enhanced.documentMetadata.wordCount || '?'} words`);
      } else if (ext === '.doc') {
        console.log(`[MetadataExtractor] Extracting legacy Word metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractLegacyWordMetadata(absolutePath);
        console.log(`[MetadataExtractor] ✓ Word (legacy): ${enhanced.documentMetadata.pageCount || '?'} pages, ${enhanced.documentMetadata.wordCount || '?'} words`);
      }
      // Excel spreadsheets
      else if (ext === '.xlsx') {
        console.log(`[MetadataExtractor] Extracting Excel metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractExcelMetadata(absolutePath);
        const excelMeta = enhanced.documentMetadata as SpreadsheetMetadata;
        console.log(`[MetadataExtractor] ✓ Excel: ${excelMeta.sheetCount || '?'} sheets, formulas: ${excelMeta.hasFormulas ? 'yes' : 'no'}`);
      } else if (ext === '.xls') {
        console.log(`[MetadataExtractor] Extracting legacy Excel metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractLegacyExcelMetadata(absolutePath);
        const excelMeta = enhanced.documentMetadata as SpreadsheetMetadata;
        console.log(`[MetadataExtractor] ✓ Excel (legacy): ${excelMeta.sheetCount || '?'} sheets, formulas: ${excelMeta.hasFormulas ? 'yes' : 'no'}`);
      }
      // PowerPoint presentations
      else if (ext === '.pptx') {
        console.log(`[MetadataExtractor] Extracting PowerPoint metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractPowerPointMetadata(absolutePath);
        const pptMeta = enhanced.documentMetadata as PresentationMetadata;
        console.log(`[MetadataExtractor] ✓ PowerPoint: ${pptMeta.slideCount || '?'} slides, animations: ${pptMeta.hasAnimations ? 'yes' : 'no'}`);
      } else if (ext === '.ppt') {
        console.log(`[MetadataExtractor] Extracting legacy PowerPoint metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractLegacyPowerPointMetadata(absolutePath);
        const pptMeta = enhanced.documentMetadata as PresentationMetadata;
        console.log(`[MetadataExtractor] ✓ PowerPoint (legacy): ${pptMeta.slideCount || '?'} slides, animations: ${pptMeta.hasAnimations ? 'yes' : 'no'}`);
      }
      // PDF documents
      else if (ext === '.pdf') {
        console.log(`[MetadataExtractor] Extracting PDF metadata...`);
        enhanced.documentMetadata = await this.documentDetector.extractPDFMetadata(absolutePath);
        const pdfMeta = enhanced.documentMetadata as PDFMetadata;
        console.log(`[MetadataExtractor] ✓ PDF: ${pdfMeta.pageCount || '?'} pages, encrypted: ${pdfMeta.isEncrypted ? 'yes' : 'no'}`);
      }
      // Photoshop files
      else if (ext === '.psd') {
        console.log(`[MetadataExtractor] Extracting PSD metadata...`);
        enhanced.designMetadata = await this.documentDetector.extractPSDMetadata(absolutePath);
        console.log(`[MetadataExtractor] ✓ PSD: ${enhanced.designMetadata.width}×${enhanced.designMetadata.height}, ${enhanced.designMetadata.colorMode}`);
      }
      if (!enhanced.indexed) {
        enhanced.indexed = {};
      }
      enhanced.indexed.document = true;
    } catch (error) {
      console.error(`[MetadataExtractor] ERROR extracting document metadata for ${absolutePath}:`, error);
    }
  }

  /**
   * Extract all metadata for a file (comprehensive extraction)
   */
  async extractAll(
    absolutePath: string,
    relativePath: string,
    extension: string
  ): Promise<EnhancedMetadata> {
    const [stats, git, mimeType] = await Promise.all([
      this.extractStats(absolutePath),
      this.extractGitMetadata(absolutePath),
      this.detector.detectMimeType(absolutePath).catch(() => undefined),
    ]);

    const { folder, depth } = this.extractFolderInfo(relativePath);
    const language = this.inferLanguage(extension);

    // Override MIME category to 'code' if we detected a programming language
    // This fixes the issue where code files without magic bytes are categorized as 'text'
    if (language && mimeType && (mimeType.category === 'text' || mimeType.category === 'binary')) {
      mimeType.category = 'code';
    }

    // Extract deep metadata based on file category
    let textMetadata: TextMetadata | undefined;
    let codeMetadata: CodeMetadata | undefined;
    let imageMetadata: ImageMetadata | undefined;

    try {
      if (mimeType?.category === 'text' || mimeType?.category === 'code') {
        // Only extract for reasonably sized text files
        if (stats.size < 1024 * 1024) {
          // < 1 MB
          textMetadata = await this.detector
            .analyzeTextFile(absolutePath)
            .catch(() => undefined);

          // Extract code metadata if we have a programming language
          if (language) {
            codeMetadata = await this.detector
              .analyzeCodeFile(absolutePath, language)
              .catch(() => undefined);
          }
        }
      }

      if (mimeType?.category === 'image') {
        imageMetadata = await this.detector
          .analyzeImageFile(absolutePath)
          .catch(() => undefined);
      }
    } catch (error) {
      // Ignore metadata extraction errors
      console.warn(`Failed to extract deep metadata for ${relativePath}:`, error);
    }

    return {
      stats,
      git,
      folder,
      depth,
      language,
      mimeType,
      textMetadata,
      codeMetadata,
      imageMetadata,
    };
  }

  /**
   * Categorize file size
   */
  categorizeSize(bytes: number): string {
    if (bytes < 1024) return 'Tiny'; // < 1 KB
    if (bytes < 10 * 1024) return 'Small'; // < 10 KB
    if (bytes < 100 * 1024) return 'Medium'; // < 100 KB
    if (bytes < 1024 * 1024) return 'Large'; // < 1 MB
    return 'Huge'; // >= 1 MB
  }

  /**
   * Categorize file by modification date
   */
  categorizeDate(timestamp: number): string {
    const now = Date.now();
    const diff = now - timestamp;

    const minute = 60 * 1000;
    const hour = 60 * minute;
    const day = 24 * hour;
    const week = 7 * day;
    const month = 30 * day;

    if (diff < hour) return 'Last Hour';
    if (diff < day) return 'Today';
    if (diff < week) return 'This Week';
    if (diff < month) return 'This Month';
    if (diff < 3 * month) return 'Last 3 Months';
    if (diff < 6 * month) return 'Last 6 Months';
    if (diff < 12 * month) return 'This Year';
    return 'Older';
  }

  /**
   * Format file size for display
   */
  formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024)
      return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  /**
   * Format date for display
   */
  formatDate(timestamp: number): string {
    const date = new Date(timestamp);
    const now = new Date();

    if (date.toDateString() === now.toDateString()) {
      return `Today ${date.toLocaleTimeString()}`;
    }

    return date.toLocaleString();
  }
}
