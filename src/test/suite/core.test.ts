import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import { IndexStore } from '../../core/IndexStore';
import { IndexCache } from '../../core/IndexCache';
import { BlacklistStore } from '../../core/BlacklistStore';
import { FileScanner } from '../../core/FileScanner';

describe('core', () => {
  it('IndexStore builds and queries', () => {
    const store = new IndexStore();
    const files = [
      {
        absolutePath: '/tmp/a.ts',
        relativePath: 'src/a.ts',
        filename: 'a.ts',
        extension: '.ts',
        lastModified: 10,
        fileSize: 100,
      },
      {
        absolutePath: '/tmp/b.md',
        relativePath: 'docs/b.md',
        filename: 'b.md',
        extension: '.md',
        lastModified: 20,
        fileSize: 200,
      },
    ];
    store.buildIndex(files);
    assert.strictEqual(store.getAllFiles().length, 2);
    assert.strictEqual(store.getFile('src/a.ts')?.filename, 'a.ts');
    assert.strictEqual(store.getFilesByExtension('ts').length, 1);
    assert.strictEqual(store.searchFiles('b').length, 1);

    const stats = store.getStats();
    assert.strictEqual(stats.totalFiles, 2);
    assert.strictEqual(stats.totalSize, 300);
    assert.strictEqual(stats.extensionCounts['.ts'], 1);
  });

  it('IndexCache saves and loads', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-cache-'));
    const cache = new IndexCache(tmp);
    const entries = [
      {
        absolutePath: path.join(tmp, 'file.txt'),
        relativePath: 'file.txt',
        filename: 'file.txt',
        extension: '.txt',
        lastModified: 0,
        fileSize: 5,
      },
    ];

    await cache.save(entries);
    const loaded = await cache.load();
    assert.strictEqual(loaded.length, 1);
    assert.strictEqual(loaded[0].relativePath, 'file.txt');
  });

  it('BlacklistStore tracks entries', () => {
    const store = new BlacklistStore('/workspace');
    store.markRelative('secret.txt', 'file', 'EACCES', 'Denied');
    store.markRelative('build', 'dir', 'EACCES', 'Denied');

    assert.strictEqual(store.isBlacklisted('secret.txt'), true);
    assert.strictEqual(store.isBlacklisted('build/output.log'), true);
    assert.strictEqual(store.getEntries().length, 2);
  });

  it('FileScanner skips ignored and blacklisted entries', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-scan-'));
    await fs.mkdir(path.join(tmp, '.git'));
    await fs.mkdir(path.join(tmp, 'node_modules'));
    await fs.mkdir(path.join(tmp, 'src'));
    await fs.writeFile(path.join(tmp, '.git/ignored.txt'), 'nope');
    await fs.writeFile(path.join(tmp, 'node_modules/ignored.js'), 'nope');
    await fs.writeFile(path.join(tmp, 'src/keep.txt'), 'ok');
    await fs.writeFile(path.join(tmp, 'skip.txt'), 'skip');

    const blacklist = new BlacklistStore(tmp);
    blacklist.markRelative('skip.txt', 'file', 'EACCES', 'Denied');

    const scanner = new FileScanner(tmp, blacklist);
    const files = await scanner.scanWorkspace();
    const relativePaths = files.map((f) => f.relativePath).sort();
    assert.deepStrictEqual(relativePaths, ['src/keep.txt']);
  });
});
