import * as assert from 'assert';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';
import { WorkerPool } from '../../core/WorkerPool';

describe('WorkerPool', () => {
  it('runs tasks and returns results', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-worker-'));
    const workerPath = path.join(tmp, 'worker.js');
    await fs.writeFile(
      workerPath,
      `
      const { parentPort } = require('worker_threads');
      parentPort.on('message', ({ id, payload }) => {
        parentPort.postMessage({ id, ok: true, result: payload * 2 });
      });
      `,
      'utf8'
    );

    const pool = new WorkerPool<number>(workerPath, 'test', { size: 1 });
    const result = await pool.runTask(21);
    assert.strictEqual(result, 42);
    pool.dispose();
  });

  it('rejects task when worker reports error', async () => {
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'cortex-worker-'));
    const workerPath = path.join(tmp, 'worker.js');
    await fs.writeFile(
      workerPath,
      `
      const { parentPort } = require('worker_threads');
      parentPort.on('message', ({ id }) => {
        parentPort.postMessage({ id, ok: false, error: 'boom' });
      });
      `,
      'utf8'
    );

    const pool = new WorkerPool(workerPath, 'test', { size: 1 });
    let failed = false;
    try {
      await pool.runTask('x');
    } catch (error: any) {
      failed = true;
      assert.strictEqual(error.message, 'boom');
    }
    assert.strictEqual(failed, true);
    pool.dispose();
  });
});
