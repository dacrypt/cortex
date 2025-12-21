import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import AdmZip from 'adm-zip';
import { FileTypeDetector } from '../../extractors/FileTypeDetector';
import { DocumentDetector } from '../../extractors/DocumentDetector';
import { MetadataExtractor } from '../../extractors/MetadataExtractor';

describe('extractors', () => {
  it('FileTypeDetector detects text and magic bytes', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-detector-'));
    const textPath = path.join(tmp, 'sample.txt');
    await fs.writeFile(textPath, 'hello world\n');

    const detector = new FileTypeDetector();
    const textInfo = await detector.detectMimeType(textPath);
    assert.strictEqual(textInfo.mimeType, 'text/plain');

    const pngPath = path.join(tmp, 'image.png');
    const png = Buffer.alloc(24);
    png[0] = 0x89;
    png[1] = 0x50;
    png[2] = 0x4e;
    png[3] = 0x47;
    png.writeUInt32BE(32, 16);
    png.writeUInt32BE(16, 20);
    await fs.writeFile(pngPath, png);

    const pngInfo = await detector.detectMimeType(pngPath);
    assert.strictEqual(pngInfo.mimeType, 'image/png');
  });

  it('FileTypeDetector analyzes text and code', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-text-'));
    const filePath = path.join(tmp, 'code.ts');
    await fs.writeFile(filePath, 'import x from "y";\n// comment\n\nclass A {}\n');

    const detector = new FileTypeDetector();
    const text = await detector.analyzeTextFile(filePath);
    assert.strictEqual(text.lineCount, 4);

    const code = await detector.analyzeCodeFile(filePath, 'typescript');
    assert.strictEqual(code.imports, 1);
    assert.strictEqual(code.classes, 1);
  });

  it('DocumentDetector detects types and extracts minimal metadata', async () => {
    const detector = new DocumentDetector();
    assert.strictEqual(detector.isOfficeDocument('.docx'), true);
    assert.strictEqual(detector.isDesignFile('.psd'), true);

    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-doc-'));
    const pdfPath = path.join(tmp, 'file.pdf');
    await fs.writeFile(
      pdfPath,
      '%PDF-1.4\n/Title (Example)\n/Author (Alice)\n/Count 5\n'
    );
    const pdfMeta = await detector.extractPDFMetadata(pdfPath);
    assert.strictEqual(pdfMeta.pdfVersion, '1.4');
    assert.strictEqual(pdfMeta.pageCount, 5);
    assert.strictEqual(pdfMeta.author, 'Alice');

    const docxPath = path.join(tmp, 'file.docx');
    const zip = new AdmZip();
    zip.addFile(
      'docProps/core.xml',
      Buffer.from('<dc:title>Title</dc:title><dc:creator>Bob</dc:creator>')
    );
    zip.addFile('docProps/app.xml', Buffer.from('<Pages>3</Pages>'));
    zip.writeZip(docxPath);
    const wordMeta = await detector.extractWordMetadata(docxPath);
    assert.strictEqual(wordMeta.title, 'Title');
    assert.strictEqual(wordMeta.author, 'Bob');
    assert.strictEqual(wordMeta.pageCount, 3);
  });

  it('MetadataExtractor covers helpers and basic extraction', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-meta-'));
    const filePath = path.join(tmp, 'src', 'file.ts');
    await fs.mkdir(path.dirname(filePath), { recursive: true });
    await fs.writeFile(filePath, 'export const n = 1;\n');

    const extractor = new MetadataExtractor(tmp);
    const folderInfo = extractor.extractFolderInfo('src/file.ts');
    assert.strictEqual(folderInfo.folder, 'src');
    assert.strictEqual(folderInfo.depth, 1);

    assert.strictEqual(extractor.inferLanguage('.ts'), 'TypeScript');
    assert.strictEqual(extractor.categorizeSize(500), 'Tiny');
    assert.strictEqual(extractor.formatSize(1024), '1.0 KB');
    assert.strictEqual(extractor.categorizeDate(Date.now()), 'Last Hour');

    const enhanced = await extractor.extractBasic(
      filePath,
      'src/file.ts',
      '.ts'
    );
    assert.strictEqual(enhanced.indexed?.basic, true);
    assert.strictEqual(enhanced.stats.size > 0, true);

    await extractor.extractMimeTypeMetadata(filePath, enhanced);
    assert.strictEqual(enhanced.indexed?.mime, true);
    assert.ok(enhanced.mimeType);

    await extractor.extractCodeMetadata(filePath, enhanced);
    assert.strictEqual(enhanced.indexed?.code, true);
    assert.ok(enhanced.codeMetadata);
  });
});
