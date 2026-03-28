import * as assert from 'node:assert';
import { generateFileId, inferFileType } from '../../utils/fileHash';
import {
  isOsTaggingSupported,
  normalizeTag,
  normalizeTags,
  TAG_MAX_LENGTH,
  TAG_MAX_WORDS,
} from '../../utils/osTags';
import { updateStoreTags } from '../../utils/tagSync';
import { FileMetadata } from '../../models/types';
import { IMetadataStore } from '../../core/IMetadataStore';

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

  it('normalizeTag formats slug-style tags', () => {
    assert.strictEqual(normalizeTag('  Hello World '), 'hello-world');
    assert.strictEqual(normalizeTag('#Tag'), 'tag');
    assert.strictEqual(normalizeTag(''), null);
    assert.strictEqual(normalizeTag('   '), null);
    assert.strictEqual(normalizeTag('a'.repeat(TAG_MAX_LENGTH + 1)), null);
    assert.strictEqual(
      normalizeTag(['one', 'two', 'three', 'four'].join('-')),
      null
    );
    assert.strictEqual(
      normalizeTag(['uno', 'dos', 'tres'].join('-')),
      'uno-dos-tres'
    );
    assert.strictEqual(TAG_MAX_WORDS, 3);
  });

  it('normalizeTags deduplicates and drops empty values', () => {
    const tags = normalizeTags([' Tag', 'tag', '', 'Other ']);
    const sortedTags = [...tags].sort((a, b) => a.localeCompare(b));
    assert.deepStrictEqual(sortedTags, ['other', 'tag']);
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
    } as unknown as IMetadataStore;

    const changed = updateStoreTags(store, 'file.txt', ['new', 'old']);
    assert.strictEqual(changed, true);
    const sortedTags = [...meta.tags].sort((a, b) => a.localeCompare(b));
    assert.deepStrictEqual(sortedTags, ['new', 'old']);
  });
});
