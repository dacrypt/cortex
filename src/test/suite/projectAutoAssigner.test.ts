import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import { IndexStore } from '../../core/IndexStore';
import { MetadataStoreJSON } from '../../core/MetadataStoreJSON';
import { ProjectAutoAssigner } from '../../core/ProjectAutoAssigner';

describe('ProjectAutoAssigner', () => {
  it('assigns projects within time + folder clusters', async () => {
    const tmp = await fs.mkdtemp(
      path.join(os.tmpdir(), 'cortex-project-assigner-')
    );
    const metadataStore = new MetadataStoreJSON(tmp);
    await metadataStore.initialize();

    const now = Date.now();
    const indexStore = new IndexStore();
    indexStore.buildIndex([
      {
        absolutePath: '/workspace/writing/ch1.md',
        relativePath: 'writing/ch1.md',
        filename: 'ch1.md',
        extension: '.md',
        lastModified: now,
        fileSize: 100,
        enhanced: {
          folder: 'writing',
          depth: 1,
          stats: {
            size: 100,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/writing/ch2.md',
        relativePath: 'writing/ch2.md',
        filename: 'ch2.md',
        extension: '.md',
        lastModified: now,
        fileSize: 120,
        enhanced: {
          folder: 'writing',
          depth: 1,
          stats: {
            size: 120,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/writing/notes.txt',
        relativePath: 'writing/notes.txt',
        filename: 'notes.txt',
        extension: '.txt',
        lastModified: now,
        fileSize: 50,
        enhanced: {
          folder: 'writing',
          depth: 1,
          stats: {
            size: 50,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/other/other.txt',
        relativePath: 'other/other.txt',
        filename: 'other.txt',
        extension: '.txt',
        lastModified: now,
        fileSize: 60,
        enhanced: {
          folder: 'other',
          depth: 1,
          stats: {
            size: 60,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
    ]);

    metadataStore.getOrCreateMetadata('writing/ch1.md', '.md');
    metadataStore.addContext('writing/ch1.md', 'book-draft');
    metadataStore.getOrCreateMetadata('writing/ch2.md', '.md');
    metadataStore.getOrCreateMetadata('writing/notes.txt', '.txt');
    metadataStore.getOrCreateMetadata('other/other.txt', '.txt');

    const assigner = new ProjectAutoAssigner(metadataStore, indexStore, {
      windowMs: 6 * 60 * 60 * 1000,
      minClusterSize: 2,
      dominanceThreshold: 0.6,
    });
    const result = assigner.assignProjects();

    assert.strictEqual(result.assignedCount, 2);
    assert.deepStrictEqual(
      metadataStore.getMetadataByPath('writing/ch2.md')?.contexts ?? [],
      ['book-draft']
    );
    assert.deepStrictEqual(
      metadataStore.getMetadataByPath('writing/notes.txt')?.contexts ?? [],
      ['book-draft']
    );
    assert.deepStrictEqual(
      metadataStore.getMetadataByPath('other/other.txt')?.contexts ?? [],
      []
    );
  });

  it('stores suggestions when confidence is below auto threshold', async () => {
    const tmp = await fs.mkdtemp(
      path.join(os.tmpdir(), 'cortex-project-suggest-')
    );
    const metadataStore = new MetadataStoreJSON(tmp);
    await metadataStore.initialize();

    const now = Date.now();
    const indexStore = new IndexStore();
    indexStore.buildIndex([
      {
        absolutePath: '/workspace/plan/a.md',
        relativePath: 'plan/a.md',
        filename: 'a.md',
        extension: '.md',
        lastModified: now,
        fileSize: 100,
        enhanced: {
          folder: 'plan',
          depth: 1,
          stats: {
            size: 100,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/plan/b.md',
        relativePath: 'plan/b.md',
        filename: 'b.md',
        extension: '.md',
        lastModified: now,
        fileSize: 120,
        enhanced: {
          folder: 'plan',
          depth: 1,
          stats: {
            size: 120,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/plan/c.md',
        relativePath: 'plan/c.md',
        filename: 'c.md',
        extension: '.md',
        lastModified: now,
        fileSize: 140,
        enhanced: {
          folder: 'plan',
          depth: 1,
          stats: {
            size: 140,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
      {
        absolutePath: '/workspace/plan/unassigned.md',
        relativePath: 'plan/unassigned.md',
        filename: 'unassigned.md',
        extension: '.md',
        lastModified: now,
        fileSize: 60,
        enhanced: {
          folder: 'plan',
          depth: 1,
          stats: {
            size: 60,
            created: now,
            modified: now,
            accessed: now,
            isReadOnly: false,
            isHidden: false,
          },
        },
      },
    ]);

    metadataStore.getOrCreateMetadata('plan/a.md', '.md');
    metadataStore.addContext('plan/a.md', 'alpha');
    metadataStore.getOrCreateMetadata('plan/b.md', '.md');
    metadataStore.addContext('plan/b.md', 'alpha');
    metadataStore.getOrCreateMetadata('plan/c.md', '.md');
    metadataStore.addContext('plan/c.md', 'beta');
    metadataStore.getOrCreateMetadata('plan/unassigned.md', '.md');

    const assigner = new ProjectAutoAssigner(metadataStore, indexStore, {
      windowMs: 6 * 60 * 60 * 1000,
      minClusterSize: 2,
      dominanceThreshold: 0.8,
      suggestionThreshold: 0.5,
    });
    const result = assigner.assignProjects();

    assert.strictEqual(result.assignedCount, 0);
    assert.deepStrictEqual(
      metadataStore.getMetadataByPath('plan/unassigned.md')?.contexts ?? [],
      []
    );
    assert.deepStrictEqual(
      metadataStore.getSuggestedContexts('plan/unassigned.md'),
      ['alpha']
    );
  });
});
