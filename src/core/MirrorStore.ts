import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';
import { execFile } from 'child_process';
import { promisify } from 'util';
import AdmZip from 'adm-zip';
import * as XLSX from 'xlsx';

const execFileAsync = promisify(execFile);

type MirrorFormat = 'md' | 'csv';

const MIRROR_FORMATS = new Map<string, MirrorFormat>([
  ['.pdf', 'md'],
  ['.docx', 'md'],
  ['.doc', 'md'],
  ['.pptx', 'md'],
  ['.ppt', 'md'],
  ['.odt', 'md'],
  ['.xlsx', 'csv'],
  ['.xls', 'csv'],
  ['.ods', 'csv'],
]);

export class MirrorStore {
  private mirrorRoot: string;

  constructor(private workspaceRoot: string) {
    this.mirrorRoot = path.join(workspaceRoot, '.cortex', 'mirror');
  }

  isMirrorableExtension(extension: string): boolean {
    return MIRROR_FORMATS.has(extension.toLowerCase());
  }

  getMirrorFormat(extension: string): MirrorFormat | null {
    return MIRROR_FORMATS.get(extension.toLowerCase()) ?? null;
  }

  getMirrorPath(relativePath: string, extension: string): string | null {
    const format = this.getMirrorFormat(extension);
    if (!format) {
      return null;
    }
    return path.join(this.mirrorRoot, `${relativePath}.${format}`);
  }

  async mirrorExists(relativePath: string, extension: string): Promise<boolean> {
    const mirrorPath = this.getMirrorPath(relativePath, extension);
    if (!mirrorPath) {
      return false;
    }
    try {
      await fs.access(mirrorPath);
      return true;
    } catch {
      return false;
    }
  }

  async readMirrorContent(
    relativePath: string,
    extension: string
  ): Promise<string | null> {
    const mirrorPath = this.getMirrorPath(relativePath, extension);
    if (!mirrorPath) {
      return null;
    }
    try {
      return await fs.readFile(mirrorPath, 'utf8');
    } catch {
      return null;
    }
  }

  async ensureMirrorContent(
    absolutePath: string,
    relativePath: string,
    extension: string
  ): Promise<string | null> {
    const mirrorPath = this.getMirrorPath(relativePath, extension);
    if (!mirrorPath) {
      return null;
    }

    let sourceStat: Awaited<ReturnType<typeof fs.stat>> | null = null;
    try {
      sourceStat = await fs.stat(absolutePath);
    } catch {
      return null;
    }

    let mirrorStat: Awaited<ReturnType<typeof fs.stat>> | null = null;
    try {
      mirrorStat = await fs.stat(mirrorPath);
    } catch {
      mirrorStat = null;
    }

    if (
      mirrorStat &&
      sourceStat &&
      mirrorStat.mtimeMs >= sourceStat.mtimeMs &&
      mirrorStat.size > 0
    ) {
      try {
        return await fs.readFile(mirrorPath, 'utf8');
      } catch {
        // Fall through to regenerate if the mirror is unreadable.
      }
    }

    const content = await this.extractContent(absolutePath, extension);
    if (content == null) {
      return null;
    }

    await fs.mkdir(path.dirname(mirrorPath), { recursive: true });
    await fs.writeFile(mirrorPath, content, 'utf8');
    return content;
  }

  async removeMirror(relativePath: string, extension: string): Promise<void> {
    const mirrorPath = this.getMirrorPath(relativePath, extension);
    if (!mirrorPath) {
      return;
    }
    await fs.rm(mirrorPath, { force: true });
  }

  private async extractContent(
    absolutePath: string,
    extension: string
  ): Promise<string | null> {
    const ext = extension.toLowerCase();

    if (ext === '.pdf') {
      return this.extractPdfText(absolutePath);
    }

    if (ext === '.docx') {
      return this.extractDocxText(absolutePath);
    }

    if (ext === '.doc') {
      return this.extractLegacyText(absolutePath, '.docx', (docxPath) =>
        this.extractDocxText(docxPath)
      );
    }

    if (ext === '.pptx') {
      return this.extractPptxText(absolutePath);
    }

    if (ext === '.ppt') {
      return this.extractLegacyText(absolutePath, '.pptx', (pptxPath) =>
        this.extractPptxText(pptxPath)
      );
    }

    if (ext === '.odt') {
      return this.extractOdtText(absolutePath);
    }

    if (ext === '.xlsx' || ext === '.xls' || ext === '.ods') {
      const csv = await this.extractSpreadsheetCsv(absolutePath);
      if (csv != null) {
        return csv;
      }

      if (ext === '.xls') {
        return this.extractLegacyText(absolutePath, '.csv', async (csvPath) =>
          fs.readFile(csvPath, 'utf8')
        );
      }
    }

    return null;
  }

  private async extractPdfText(absolutePath: string): Promise<string | null> {
    try {
      const pdfParse = require('pdf-parse') as (
        data: Buffer,
        options?: Record<string, unknown>
      ) => Promise<{ text?: string }>;
      const data = await fs.readFile(absolutePath);
      const result = await pdfParse(data);
      return result.text?.trim() ?? '';
    } catch (error) {
      console.warn(`[MirrorStore] PDF extraction failed for ${absolutePath}:`, error);
      return null;
    }
  }

  private extractXmlText(xml: string, tagRegex: RegExp): string[] {
    const results: string[] = [];
    let match: RegExpExecArray | null;
    while ((match = tagRegex.exec(xml))) {
      const raw = match[1] ?? '';
      const cleaned = this.decodeXmlEntities(raw.replace(/<[^>]+>/g, ''));
      if (cleaned.trim().length > 0) {
        results.push(cleaned);
      }
    }
    return results;
  }

  private async extractDocxText(absolutePath: string): Promise<string | null> {
    try {
      const zip = new AdmZip(absolutePath);
      const entry = zip.getEntry('word/document.xml');
      if (!entry) {
        return '';
      }
      const xml = entry.getData().toString('utf8');
      const paragraphs = xml
        .split(/<\/w:p>/)
        .map((chunk) => this.extractXmlText(chunk, /<w:t[^>]*>([\s\S]*?)<\/w:t>/g).join(''))
        .map((text) => text.trim())
        .filter(Boolean);
      return paragraphs.join('\n\n');
    } catch (error) {
      console.warn(`[MirrorStore] DOCX extraction failed for ${absolutePath}:`, error);
      return null;
    }
  }

  private async extractPptxText(absolutePath: string): Promise<string | null> {
    try {
      const zip = new AdmZip(absolutePath);
      const entries = zip
        .getEntries()
        .filter((entry) => entry.entryName.startsWith('ppt/slides/slide'));
      if (entries.length === 0) {
        return '';
      }
      const slideTexts = entries.map((entry) => {
        const xml = entry.getData().toString('utf8');
        return this.extractXmlText(xml, /<a:t[^>]*>([\s\S]*?)<\/a:t>/g).join(' ');
      });
      return slideTexts.map((text) => text.trim()).filter(Boolean).join('\n\n');
    } catch (error) {
      console.warn(`[MirrorStore] PPTX extraction failed for ${absolutePath}:`, error);
      return null;
    }
  }

  private async extractOdtText(absolutePath: string): Promise<string | null> {
    try {
      const zip = new AdmZip(absolutePath);
      const entry = zip.getEntry('content.xml');
      if (!entry) {
        return '';
      }
      const xml = entry.getData().toString('utf8');
      const paragraphs = this.extractXmlText(
        xml,
        /<text:p[^>]*>([\s\S]*?)<\/text:p>/g
      );
      return paragraphs.join('\n\n');
    } catch (error) {
      console.warn(`[MirrorStore] ODT extraction failed for ${absolutePath}:`, error);
      return null;
    }
  }

  private async extractSpreadsheetCsv(
    absolutePath: string
  ): Promise<string | null> {
    try {
      const workbook = XLSX.readFile(absolutePath, { cellDates: true });
      const sheetName = workbook.SheetNames[0];
      if (!sheetName) {
        return '';
      }
      const sheet = workbook.Sheets[sheetName];
      return XLSX.utils.sheet_to_csv(sheet);
    } catch (error) {
      console.warn(
        `[MirrorStore] Spreadsheet extraction failed for ${absolutePath}:`,
        error
      );
      return null;
    }
  }

  private async extractLegacyText<T>(
    absolutePath: string,
    targetExtension: '.docx' | '.pptx' | '.csv',
    extractor: (convertedPath: string) => Promise<T>
  ): Promise<T | null> {
    const conversion = await this.convertLegacyOfficeFile(
      absolutePath,
      targetExtension
    );
    if (!conversion) {
      return null;
    }

    try {
      return await extractor(conversion.convertedPath);
    } finally {
      await conversion.cleanup();
    }
  }

  private async convertLegacyOfficeFile(
    absolutePath: string,
    targetExtension: '.docx' | '.pptx' | '.csv'
  ): Promise<{ convertedPath: string; cleanup: () => Promise<void> } | null> {
    const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-mirror-'));
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
      if (error?.code === 'ENOENT') {
        console.warn(
          '[MirrorStore] LibreOffice (soffice) not found. Legacy formats require it.'
        );
      } else {
        console.warn(
          `[MirrorStore] Legacy conversion failed for ${absolutePath}:`,
          error
        );
      }
      await cleanup();
      return null;
    }
  }

  private decodeXmlEntities(value: string): string {
    return value
      .replace(/&lt;/g, '<')
      .replace(/&gt;/g, '>')
      .replace(/&amp;/g, '&')
      .replace(/&quot;/g, '"')
      .replace(/&apos;/g, "'");
  }
}
