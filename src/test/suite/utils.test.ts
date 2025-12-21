import * as assert from 'assert';
import { generateFileId, inferFileType } from '../../utils/fileHash';
import {
  isOsTaggingSupported,
  normalizeTag,
  normalizeTags,
} from '../../utils/osTags';
import { formatIndexingMessage } from '../../core/IndexingStatus';
import { updateStoreTags } from '../../utils/tagSync';
import { FileMetadata } from '../../models/types';

describe('utils', () => {
  it('generateFileId is deterministic', () => {
    const first = generateFileId('src/index.ts');
    const second = generateFileId('src/index.ts');
    assert.strictEqual(first, second);
    assert.strictEqual(first.length, 64);
  });

  it('inferFileType maps known extensions and passes through unknowns', () => {
    assert.strictEqual(inferFileType('.ts'), 'typescript');
    assert.strictEqual(inferFileType('md'), 'markdown');
    assert.strictEqual(inferFileType('unknownext'), 'unknownext');
  });

  it('normalizeTag trims and lowercases', () => {
    assert.strictEqual(normalizeTag('  Hello '), 'hello');
    assert.strictEqual(normalizeTag(''), null);
    assert.strictEqual(normalizeTag('   '), null);
  });

  it('normalizeTags deduplicates and drops empty values', () => {
    const tags = normalizeTags([' Tag', 'tag', '', 'Other ']);
    assert.deepStrictEqual(tags.sort(), ['other', 'tag']);
  });

  it('isOsTaggingSupported matches platform', () => {
    assert.strictEqual(isOsTaggingSupported(), process.platform === 'darwin');
  });

  it('isOsTaggingSupported can be disabled via env', () => {
    const previous = process.env.CORTEX_DISABLE_OS_TAGS;
    process.env.CORTEX_DISABLE_OS_TAGS = '1';
    assert.strictEqual(isOsTaggingSupported(), false);
    if (previous === undefined) {
      delete process.env.CORTEX_DISABLE_OS_TAGS;
    } else {
      process.env.CORTEX_DISABLE_OS_TAGS = previous;
    }
  });

  it('formatIndexingMessage formats progress', () => {
    const message = formatIndexingMessage({
      phase: 'basic',
      message: 'Test',
      processed: 2,
      total: 10,
      isIndexing: true,
    });
    assert.strictEqual(message, 'Indexando: Test (2/10)');
  });

  it('updateStoreTags syncs tag sets', () => {
    const meta: FileMetadata = {
      file_id: '1',
      relativePath: 'file.txt',
      tags: ['old'],
      contexts: [],
      type: 'text',
      created_at: 0,
      updated_at: 0,
    };

    const store = {
      getMetadataByPath: () => meta,
      addTag: (_path: string, tag: string) => {
        if (!meta.tags.includes(tag)) {
          meta.tags.push(tag);
        }
      },
      removeTag: (_path: string, tag: string) => {
        meta.tags = meta.tags.filter((t) => t !== tag);
      },
    } as any;

    const changed = updateStoreTags(store, 'file.txt', ['new', 'old']);
    assert.strictEqual(changed, true);
    assert.deepStrictEqual(meta.tags.sort(), ['new', 'old']);
  });
});
