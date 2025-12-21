/**
 * DocumentDetector - Extracts metadata from Office documents, PDFs, and design files
 */

import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';
import { execFile } from 'child_process';
import { promisify } from 'util';
import AdmZip from 'adm-zip';

const execFileAsync = promisify(execFile);

export interface DocumentMetadata {
  // Common properties
  pageCount?: number;
  wordCount?: number;
  characterCount?: number;

  // Document properties
  title?: string;
  subject?: string;
  author?: string;
  creator?: string;
  keywords?: string[];
  description?: string;

  // Dates
  createdDate?: Date;
  modifiedDate?: Date;

  // Document-specific
  language?: string;
  revision?: string;
  template?: string;

  // Security
  isEncrypted?: boolean;
  hasPassword?: boolean;
}

export interface SpreadsheetMetadata extends DocumentMetadata {
  sheetCount?: number;
  totalRows?: number;
  totalColumns?: number;
  hasFormulas?: boolean;
  hasMacros?: boolean;
  hasCharts?: boolean;
  hasPivotTables?: boolean;
}

export interface PresentationMetadata extends DocumentMetadata {
  slideCount?: number;
  hasAnimations?: boolean;
  hasTransitions?: boolean;
  hasEmbeddedMedia?: boolean;
  hasNotes?: boolean;
  masterSlideCount?: number;
}

export interface DesignMetadata {
  // Photoshop/Design files
  width?: number;
  height?: number;
  resolution?: number; // DPI
  colorMode?: string; // RGB, CMYK, Grayscale, etc.
  bitDepth?: number;
  layerCount?: number;
  hasTransparency?: boolean;

  // Creator info
  software?: string;
  softwareVersion?: string;
  author?: string;

  // File properties
  isFlattened?: boolean;
  compressionMethod?: string;
}

export interface PDFMetadata extends DocumentMetadata {
  pdfVersion?: string;
  isLinearized?: boolean; // Fast web view
  hasJavaScript?: boolean;
  hasAttachments?: boolean;
  hasBookmarks?: boolean;
  hasComments?: boolean;

  // Security
  printAllowed?: boolean;
  copyAllowed?: boolean;
  modifyAllowed?: boolean;
}

export class DocumentDetector {
  private static sofficeAvailable: boolean | null = null;
  private static sofficeCheckPromise: Promise<boolean> | null = null;

  /**
   * Check if LibreOffice (soffice) is available
   */
  private static async checkSofficeAvailable(): Promise<boolean> {
    // If already checked, return cached result
    if (DocumentDetector.sofficeAvailable !== null) {
      return DocumentDetector.sofficeAvailable;
    }

    // If check is in progress, wait for it
    if (DocumentDetector.sofficeCheckPromise) {
      return DocumentDetector.sofficeCheckPromise;
    }

    // Start new check
    DocumentDetector.sofficeCheckPromise = (async () => {
      try {
        await execFileAsync('soffice', ['--version']);
        DocumentDetector.sofficeAvailable = true;
        return true;
      } catch (error: any) {
        if (error?.code === 'ENOENT') {
          DocumentDetector.sofficeAvailable = false;
          // Only log warning once when first detected as missing
          console.warn(
            '[DocumentDetector] LibreOffice (soffice) not found. Legacy Office metadata requires it. Install LibreOffice to extract metadata from .doc, .xls, and .ppt files.'
          );
        } else {
          // Other errors might be temporary, so don't cache
          return false;
        }
        return false;
      } finally {
        DocumentDetector.sofficeCheckPromise = null;
      }
    })();

    return DocumentDetector.sofficeCheckPromise;
  }

  private async convertLegacyOfficeFile(
    absolutePath: string,
    targetExtension: '.docx' | '.xlsx' | '.pptx'
  ): Promise<{ convertedPath: string; cleanup: () => Promise<void> } | null> {
    // Check if soffice is available first (cached check)
    const isAvailable = await DocumentDetector.checkSofficeAvailable();
    if (!isAvailable) {
      return null;
    }

    const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-convert-'));
    const cleanup = async () => {
      await fs.rm(tmpDir, { recursive: true, force: true });
    };

    try {
      await execFileAsync('soffice', [
        '--headless',
        '--convert-to',
        targetExtension.slice(1),
        '--outdir',
        tmpDir,
        absolutePath,
      ]);

      const convertedPath = path.join(
        tmpDir,
        `${path.basename(absolutePath, path.extname(absolutePath))}${targetExtension}`
      );

      await fs.access(convertedPath);
      return { convertedPath, cleanup };
    } catch (error: any) {
      // If we get here, soffice was available but conversion failed
      console.warn(
        `[DocumentDetector] Legacy Office conversion failed for ${absolutePath}:`,
        error
      );
      await cleanup();
      return null;
    }
  }

  /**
   * Detect if file is an Office document
   */
  isOfficeDocument(extension: string): boolean {
    const officeExts = ['.docx', '.doc', '.xlsx', '.xls', '.pptx', '.ppt', '.odt', '.ods', '.odp'];
    return officeExts.includes(extension.toLowerCase());
  }

  /**
   * Detect if file is a design file
   */
  isDesignFile(extension: string): boolean {
    const designExts = ['.psd', '.ai', '.sketch', '.fig', '.xd'];
    return designExts.includes(extension.toLowerCase());
  }

  /**
   * Extract Word document metadata
   */
  async extractWordMetadata(absolutePath: string): Promise<DocumentMetadata> {
    try {
      const zip = new AdmZip(absolutePath);
      const metadata: DocumentMetadata = {};

      // Extract core.xml (document properties)
      const coreXml = zip.getEntry('docProps/core.xml');
      if (coreXml) {
        const content = coreXml.getData().toString('utf8');

        // Parse title
        const titleMatch = content.match(/<dc:title>(.*?)<\/dc:title>/);
        if (titleMatch) metadata.title = titleMatch[1];

        // Parse subject
        const subjectMatch = content.match(/<dc:subject>(.*?)<\/dc:subject>/);
        if (subjectMatch) metadata.subject = subjectMatch[1];

        // Parse author
        const authorMatch = content.match(/<dc:creator>(.*?)<\/dc:creator>/);
        if (authorMatch) metadata.author = authorMatch[1];

        // Parse keywords
        const keywordsMatch = content.match(/<cp:keywords>(.*?)<\/cp:keywords>/);
        if (keywordsMatch) {
          metadata.keywords = keywordsMatch[1].split(/[,;]/).map((k: string) => k.trim());
        }

        // Parse description
        const descMatch = content.match(/<dc:description>(.*?)<\/dc:description>/);
        if (descMatch) metadata.description = descMatch[1];

        // Parse revision
        const revMatch = content.match(/<cp:revision>(.*?)<\/cp:revision>/);
        if (revMatch) metadata.revision = revMatch[1];
      }

      // Extract app.xml (document statistics)
      const appXml = zip.getEntry('docProps/app.xml');
      if (appXml) {
        const content = appXml.getData().toString('utf8');

        // Page count
        const pageMatch = content.match(/<Pages>(.*?)<\/Pages>/);
        if (pageMatch) metadata.pageCount = parseInt(pageMatch[1]);

        // Word count
        const wordMatch = content.match(/<Words>(.*?)<\/Words>/);
        if (wordMatch) metadata.wordCount = parseInt(wordMatch[1]);

        // Character count
        const charMatch = content.match(/<Characters>(.*?)<\/Characters>/);
        if (charMatch) metadata.characterCount = parseInt(charMatch[1]);

        // Template
        const templateMatch = content.match(/<Template>(.*?)<\/Template>/);
        if (templateMatch) metadata.template = templateMatch[1];
      }

      // Check for encryption (encrypted files can't be read as ZIP)
      const stats = await fs.stat(absolutePath);
      metadata.isEncrypted = stats.size > 0 && (!coreXml && !appXml);

      return metadata;
    } catch (error) {
      console.warn(`Failed to extract Word metadata:`, error);
      return { isEncrypted: true }; // Likely encrypted or corrupted
    }
  }

  /**
   * Extract legacy Word (.doc) metadata by converting to .docx
   */
  async extractLegacyWordMetadata(
    absolutePath: string
  ): Promise<DocumentMetadata> {
    const conversion = await this.convertLegacyOfficeFile(absolutePath, '.docx');
    if (!conversion) {
      return {};
    }

    try {
      return await this.extractWordMetadata(conversion.convertedPath);
    } finally {
      await conversion.cleanup();
    }
  }

  /**
   * Extract Excel metadata
   */
  async extractExcelMetadata(absolutePath: string): Promise<SpreadsheetMetadata> {
    try {
      const zip = new AdmZip(absolutePath);
      const metadata: SpreadsheetMetadata = {};

      // Extract core properties (similar to Word)
      const coreXml = zip.getEntry('docProps/core.xml');
      if (coreXml) {
        const content = coreXml.getData().toString('utf8');

        const titleMatch = content.match(/<dc:title>(.*?)<\/dc:title>/);
        if (titleMatch) metadata.title = titleMatch[1];

        const authorMatch = content.match(/<dc:creator>(.*?)<\/dc:creator>/);
        if (authorMatch) metadata.author = authorMatch[1];
      }

      // Count sheets
      const workbookXml = zip.getEntry('xl/workbook.xml');
      if (workbookXml) {
        const content = workbookXml.getData().toString('utf8');
        const sheetMatches = content.match(/<sheet /g);
        metadata.sheetCount = sheetMatches ? sheetMatches.length : 0;
      }

      // Check for VBA macros
      const vbaEntry = zip.getEntry('xl/vbaProject.bin');
      metadata.hasMacros = !!vbaEntry;

      // Check for shared strings (indicates text content)
      const sharedStrings = zip.getEntry('xl/sharedStrings.xml');
      if (sharedStrings) {
        const content = sharedStrings.getData().toString('utf8');
        const stringMatches = content.match(/<si>/g);
        // Approximate word count from unique strings
        metadata.wordCount = stringMatches ? stringMatches.length : 0;
      }

      // Check for charts
      const chartEntries = zip.getEntries().filter((e: AdmZip.IZipEntry) => e.entryName.startsWith('xl/charts/'));
      metadata.hasCharts = chartEntries.length > 0;

      // Simple formula detection (would need to parse sheet XMLs for accuracy)
      const sheet1 = zip.getEntry('xl/worksheets/sheet1.xml');
      if (sheet1) {
        const content = sheet1.getData().toString('utf8');
        metadata.hasFormulas = content.includes('<f>');

        // Rough row count from first sheet
        const rowMatches = content.match(/<row /g);
        metadata.totalRows = rowMatches ? rowMatches.length : 0;
      }

      return metadata;
    } catch (error) {
      console.warn(`Failed to extract Excel metadata:`, error);
      return { isEncrypted: true };
    }
  }

  /**
   * Extract legacy Excel (.xls) metadata by converting to .xlsx
   */
  async extractLegacyExcelMetadata(
    absolutePath: string
  ): Promise<SpreadsheetMetadata> {
    const conversion = await this.convertLegacyOfficeFile(absolutePath, '.xlsx');
    if (!conversion) {
      return {};
    }

    try {
      return await this.extractExcelMetadata(conversion.convertedPath);
    } finally {
      await conversion.cleanup();
    }
  }

  /**
   * Extract PowerPoint metadata
   */
  async extractPowerPointMetadata(absolutePath: string): Promise<PresentationMetadata> {
    try {
      const zip = new AdmZip(absolutePath);
      const metadata: PresentationMetadata = {};

      // Extract core properties
      const coreXml = zip.getEntry('docProps/core.xml');
      if (coreXml) {
        const content = coreXml.getData().toString('utf8');

        const titleMatch = content.match(/<dc:title>(.*?)<\/dc:title>/);
        if (titleMatch) metadata.title = titleMatch[1];

        const authorMatch = content.match(/<dc:creator>(.*?)<\/dc:creator>/);
        if (authorMatch) metadata.author = authorMatch[1];
      }

      // Extract app properties
      const appXml = zip.getEntry('docProps/app.xml');
      if (appXml) {
        const content = appXml.getData().toString('utf8');

        // Slide count
        const slideMatch = content.match(/<Slides>(.*?)<\/Slides>/);
        if (slideMatch) metadata.slideCount = parseInt(slideMatch[1]);

        // Word count
        const wordMatch = content.match(/<Words>(.*?)<\/Words>/);
        if (wordMatch) metadata.wordCount = parseInt(wordMatch[1]);

        // Notes count
        const notesMatch = content.match(/<Notes>(.*?)<\/Notes>/);
        metadata.hasNotes = notesMatch ? parseInt(notesMatch[1]) > 0 : false;
      }

      // Count slides directly
      const slideEntries = zip.getEntries().filter((e: AdmZip.IZipEntry) =>
        e.entryName.match(/ppt\/slides\/slide\d+\.xml/)
      );
      if (!metadata.slideCount) {
        metadata.slideCount = slideEntries.length;
      }

      // Check for animations
      let hasAnimations = false;
      for (const slideEntry of slideEntries.slice(0, 5)) { // Check first 5 slides
        const content = slideEntry.getData().toString('utf8');
        if (content.includes('<p:timing>') || content.includes('<p:anim')) {
          hasAnimations = true;
          break;
        }
      }
      metadata.hasAnimations = hasAnimations;

      // Check for transitions
      let hasTransitions = false;
      for (const slideEntry of slideEntries.slice(0, 5)) {
        const content = slideEntry.getData().toString('utf8');
        if (content.includes('<p:transition')) {
          hasTransitions = true;
          break;
        }
      }
      metadata.hasTransitions = hasTransitions;

      // Check for embedded media
      const mediaEntries = zip.getEntries().filter((e: AdmZip.IZipEntry) =>
        e.entryName.startsWith('ppt/media/')
      );
      metadata.hasEmbeddedMedia = mediaEntries.length > 0;

      // Count master slides
      const masterEntries = zip.getEntries().filter((e: AdmZip.IZipEntry) =>
        e.entryName.match(/ppt\/slideMasters\//)
      );
      metadata.masterSlideCount = masterEntries.length;

      return metadata;
    } catch (error) {
      console.warn(`Failed to extract PowerPoint metadata:`, error);
      return { isEncrypted: true };
    }
  }

  /**
   * Extract legacy PowerPoint (.ppt) metadata by converting to .pptx
   */
  async extractLegacyPowerPointMetadata(
    absolutePath: string
  ): Promise<PresentationMetadata> {
    const conversion = await this.convertLegacyOfficeFile(absolutePath, '.pptx');
    if (!conversion) {
      return {};
    }

    try {
      return await this.extractPowerPointMetadata(conversion.convertedPath);
    } finally {
      await conversion.cleanup();
    }
  }

  /**
   * Extract PSD (Photoshop) metadata
   */
  async extractPSDMetadata(absolutePath: string): Promise<DesignMetadata> {
    try {
      const buffer = await fs.readFile(absolutePath);
      const metadata: DesignMetadata = {};

      // PSD signature: "8BPS"
      if (buffer.toString('utf8', 0, 4) !== '8BPS') {
        throw new Error('Not a valid PSD file');
      }

      // Version (bytes 4-5): should be 1
      const version = buffer.readUInt16BE(4);

      // Dimensions
      metadata.height = buffer.readUInt32BE(14);
      metadata.width = buffer.readUInt32BE(18);

      // Color mode (bytes 24-25)
      const colorModeNum = buffer.readUInt16BE(24);
      const colorModes = ['Bitmap', 'Grayscale', 'Indexed', 'RGB', 'CMYK', 'Multichannel', 'Duotone', 'Lab'];
      metadata.colorMode = colorModes[colorModeNum] || 'Unknown';

      // Bit depth (bytes 22-23)
      metadata.bitDepth = buffer.readUInt16BE(22);

      // Number of channels (indicates transparency)
      const channels = buffer.readUInt16BE(12);
      metadata.hasTransparency = channels === 4 || channels > 3; // Alpha channel

      // Layer count (requires parsing layer info section - complex)
      // For now, we'll skip detailed layer parsing

      metadata.software = 'Adobe Photoshop';

      return metadata;
    } catch (error) {
      console.warn(`Failed to extract PSD metadata:`, error);
      return {};
    }
  }

  /**
   * Extract basic PDF metadata
   */
  async extractPDFMetadata(absolutePath: string): Promise<PDFMetadata> {
    try {
      const buffer = await fs.readFile(absolutePath);
      const content = buffer.toString('utf8', 0, Math.min(10000, buffer.length));
      const metadata: PDFMetadata = {};

      // PDF version
      const versionMatch = content.match(/%PDF-(\d+\.\d+)/);
      if (versionMatch) metadata.pdfVersion = versionMatch[1];

      // Linearized (fast web view)
      metadata.isLinearized = content.includes('/Linearized');

      // Page count (rough estimate from /Count in catalog)
      const pageMatch = content.match(/\/Count\s+(\d+)/);
      if (pageMatch) metadata.pageCount = parseInt(pageMatch[1]);

      // Security/Encryption
      metadata.isEncrypted = content.includes('/Encrypt');

      // JavaScript
      metadata.hasJavaScript = content.includes('/JavaScript') || content.includes('/JS');

      // Metadata (look for Info dictionary)
      const titleMatch = content.match(/\/Title\s*\((.*?)\)/);
      if (titleMatch) metadata.title = titleMatch[1];

      const authorMatch = content.match(/\/Author\s*\((.*?)\)/);
      if (authorMatch) metadata.author = authorMatch[1];

      const subjectMatch = content.match(/\/Subject\s*\((.*?)\)/);
      if (subjectMatch) metadata.subject = subjectMatch[1];

      const keywordsMatch = content.match(/\/Keywords\s*\((.*?)\)/);
      if (keywordsMatch) {
        metadata.keywords = keywordsMatch[1].split(/[,;]/).map((k: string) => k.trim());
      }

      const creatorMatch = content.match(/\/Creator\s*\((.*?)\)/);
      if (creatorMatch) metadata.creator = creatorMatch[1];

      return metadata;
    } catch (error) {
      console.warn(`Failed to extract PDF metadata:`, error);
      return {};
    }
  }
}
