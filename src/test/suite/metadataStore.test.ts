import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import { MetadataStoreJSON } from '../../core/MetadataStoreJSON';

describe('MetadataStoreJSON', () => {
  it('creates and updates metadata with tags and contexts', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-store-'));
    const store = new MetadataStoreJSON(tmp);
    await store.initialize();

    const meta = store.getOrCreateMetadata('src/file.ts', '.ts');
    assert.strictEqual(meta.relativePath, 'src/file.ts');
    assert.strictEqual(store.getAllTypes().includes('typescript'), true);

    store.addTag('src/file.ts', 'important');
    store.addContext('src/file.ts', 'project-a');
    store.updateNotes('src/file.ts', 'note');

    const updated = store.getMetadata(meta.file_id);
    assert.strictEqual(updated?.tags.includes('important'), true);
    assert.strictEqual(updated?.contexts.includes('project-a'), true);
    assert.strictEqual(updated?.notes, 'note');

    store.removeTag('src/file.ts', 'important');
    store.removeContext('src/file.ts', 'project-a');
    assert.strictEqual(store.getFilesByTag('important').length, 0);
    assert.strictEqual(store.getFilesByContext('project-a').length, 0);
  });

  it('ensureMetadataForFiles returns created count', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-store-'));
    const store = new MetadataStoreJSON(tmp);
    await store.initialize();

    const created = store.ensureMetadataForFiles([
      { relativePath: 'a.txt', extension: '.txt' },
      { relativePath: 'b.md', extension: '.md' },
    ]);

    assert.strictEqual(created, 2);
    assert.strictEqual(store.getAllTypes().length, 2);
  });
});
