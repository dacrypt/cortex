import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import { IndexStore } from '../../core/IndexStore';
import { MetadataStoreJSON } from '../../core/MetadataStoreJSON';
import { ContextTreeProvider } from '../../views/ContextTreeProvider';
import { TagTreeProvider } from '../../views/TagTreeProvider';
import { TypeTreeProvider } from '../../views/TypeTreeProvider';
import { DateTreeProvider } from '../../views/DateTreeProvider';
import { SizeTreeProvider } from '../../views/SizeTreeProvider';
import { FolderTreeProvider } from '../../views/FolderTreeProvider';
import { ContentTypeTreeProvider } from '../../views/ContentTypeTreeProvider';
import { CodeMetricsTreeProvider } from '../../views/CodeMetricsTreeProvider';
import { DocumentMetricsTreeProvider } from '../../views/DocumentMetricsTreeProvider';
import { IssuesTreeProvider } from '../../views/IssuesTreeProvider';
import { MetadataExtractor } from '../../extractors/MetadataExtractor';
import { BlacklistStore } from '../../core/BlacklistStore';

describe('views', () => {
  async function buildMetadataStore(): Promise<MetadataStoreJSON> {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-view-store-'));
    const store = new MetadataStoreJSON(tmp);
    await store.initialize();
    store.getOrCreateMetadata('src/a.ts', '.ts');
    store.addContext('src/a.ts', 'alpha');
    store.addTag('src/a.ts', 'urgent');
    return store;
  }

  function buildIndexStore(): IndexStore {
    const store = new IndexStore();
    const now = Date.now();
    store.buildIndex([
      {
        absolutePath: '/workspace/src/a.ts',
        relativePath: 'src/a.ts',
        filename: 'a.ts',
        extension: '.ts',
        lastModified: now,
        fileSize: 120,
        enhanced: {
          language: 'TypeScript',
          folder: 'src',
          depth: 1,
          stats: {
            size: 120,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
          mimeType: { mimeType: 'text/typescript', category: 'code', isBinary: false },
          codeMetadata: {
            linesOfCode: 10,
            commentLines: 1,
            blankLines: 1,
            commentPercentage: 10,
            imports: 1,
            exports: 1,
            functions: 1,
            classes: 1,
          },
        },
      },
      {
        absolutePath: '/workspace/docs/report.pdf',
        relativePath: 'docs/report.pdf',
        filename: 'report.pdf',
        extension: '.pdf',
        lastModified: now,
        fileSize: 5000,
        enhanced: {
          folder: 'docs',
          depth: 1,
          stats: {
            size: 5000,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
          mimeType: { mimeType: 'application/pdf', category: 'document', isBinary: true },
          documentMetadata: {
            author: 'Alice',
            pageCount: 4,
            pdfVersion: '1.4',
          },
        },
      },
      {
        absolutePath: '/workspace/design/mock.psd',
        relativePath: 'design/mock.psd',
        filename: 'mock.psd',
        extension: '.psd',
        lastModified: now,
        fileSize: 800000,
        enhanced: {
          folder: 'design',
          depth: 1,
          stats: {
            size: 800000,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
          designMetadata: {
            width: 100,
            height: 50,
            colorMode: 'RGB',
            bitDepth: 8,
            hasTransparency: true,
          },
        },
      },
      {
        absolutePath: '/workspace/misc/missing.txt',
        relativePath: 'misc/missing.txt',
        filename: 'missing.txt',
        extension: '.txt',
        lastModified: now,
        fileSize: 1,
        enhanced: {
          stats: {
            size: 1,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
          folder: 'misc',
          depth: 1,
          error: {
            code: 'ENOENT',
            message: 'Missing',
            operation: 'extractStats',
          },
        },
      },
    ]);

    return store;
  }

  it('Context/Tag/Type providers return root items and children', async () => {
    const workspaceRoot = '/workspace';
    const metadataStore = await buildMetadataStore();
    const indexStore = buildIndexStore();

    const contextProvider = new ContextTreeProvider(
      workspaceRoot,
      metadataStore,
      indexStore
    );
    const contextRoots = await contextProvider.getChildren();
    assert.strictEqual(contextRoots.length, 1);
    assert.ok(String(contextRoots[0].label).includes('alpha'));
    const contextChildren = await contextProvider.getChildren(contextRoots[0]);
    assert.strictEqual(contextChildren.length, 1);

    const tagProvider = new TagTreeProvider(
      workspaceRoot,
      metadataStore,
      indexStore
    );
    const tagRoots = await tagProvider.getChildren();
    assert.strictEqual(tagRoots.length, 1);
    assert.ok(String(tagRoots[0].label).includes('urgent'));
    const tagChildren = await tagProvider.getChildren(tagRoots[0]);
    assert.strictEqual(tagChildren.length, 1);

    const typeProvider = new TypeTreeProvider(
      workspaceRoot,
      metadataStore,
      indexStore
    );
    const typeRoots = await typeProvider.getChildren();
    assert.strictEqual(typeRoots.length, 1);
    assert.ok(String(typeRoots[0].label).includes('typescript'));
  });

  it('Date/Size/Folder providers group index entries', async () => {
    const workspaceRoot = '/workspace';
    const indexStore = buildIndexStore();

    const dateProvider = new DateTreeProvider(workspaceRoot, indexStore);
    const dateRoots = await dateProvider.getChildren();
    assert.ok(String(dateRoots[0].label).includes('Last Hour'));

    const sizeProvider = new SizeTreeProvider(workspaceRoot, indexStore);
    const sizeRoots = await sizeProvider.getChildren();
    assert.ok(sizeRoots.length > 0);

    const folderProvider = new FolderTreeProvider(workspaceRoot, indexStore);
    const folderRoots = await folderProvider.getChildren();
    assert.ok(folderRoots.length > 0);
  });

  it('Content/Code/Document providers build categories', async () => {
    const workspaceRoot = '/workspace';
    const indexStore = buildIndexStore();
    const extractor = new MetadataExtractor(workspaceRoot);

    const contentProvider = new ContentTypeTreeProvider(
      workspaceRoot,
      indexStore,
      extractor
    );
    const contentRoots = await contentProvider.getChildren();
    assert.ok(contentRoots.length > 0);

    const codeProvider = new CodeMetricsTreeProvider(
      workspaceRoot,
      indexStore,
      extractor
    );
    const codeRoots = await codeProvider.getChildren();
    assert.ok(String(codeRoots[0].label).includes('By Size'));
    const codeFiles = await codeProvider.getChildren(codeRoots[0]);
    assert.ok(codeFiles.length > 0);

    const docProvider = new DocumentMetricsTreeProvider(
      workspaceRoot,
      indexStore,
      extractor
    );
    const docRoots = await docProvider.getChildren();
    assert.ok(docRoots.some((item) => String(item.label).includes('PDF')));
    const pdfRoot = docRoots.find((item) => item.metricCategory === 'pdf');
    assert.ok(pdfRoot);
    const pdfFiles = await docProvider.getChildren(pdfRoot);
    assert.ok(pdfFiles.length > 0);
  });

  it('IssuesTreeProvider groups error and blacklist entries', async () => {
    const workspaceRoot = '/workspace';
    const indexStore = buildIndexStore();
    const blacklist = new BlacklistStore(workspaceRoot);
    blacklist.markRelative('secrets.txt', 'file', 'EACCES', 'Denied');

    const issuesProvider = new IssuesTreeProvider(
      workspaceRoot,
      indexStore,
      undefined,
      blacklist
    );
    const roots = await issuesProvider.getChildren();
    assert.ok(roots.some((item) => item.errorCode === 'ENOENT'));
    assert.ok(roots.some((item) => item.errorCode === 'BLACKLIST'));
  });
});
