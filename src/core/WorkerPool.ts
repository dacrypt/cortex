import { Worker } from 'worker_threads';
import * as os from 'os';

export interface WorkerPoolOptions {
  size?: number;
}

interface Task<T> {
  id: number;
  payload: T;
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}

interface WorkerSlot {
  worker: Worker;
  busy: boolean;
  currentTaskId?: number;
}

export class WorkerPool<TPayload = unknown> {
  private workers: WorkerSlot[] = [];
  private queue: Task<TPayload>[] = [];
  private pending = new Map<number, Task<TPayload>>();
  private nextId = 1;

  constructor(
    private workerPath: string,
    private label: string,
    options: WorkerPoolOptions = {}
  ) {
    const cpuCount = os.cpus().length;
    const defaultSize = Math.max(1, Math.min(6, cpuCount - 1));
    const size = options.size ?? defaultSize;

    for (let i = 0; i < size; i += 1) {
      this.workers.push({ worker: this.spawnWorker(), busy: false });
    }
  }

  runTask(payload: TPayload): Promise<unknown> {
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      console.log(`[WorkerPool:${this.label}] queued task ${id}`);
      this.queue.push({ id, payload, resolve, reject });
      this.processQueue();
    });
  }

  dispose(): void {
    for (const slot of this.workers) {
      slot.worker.terminate();
    }
    this.workers = [];
    this.queue = [];
    this.pending.clear();
  }

  private spawnWorker(): Worker {
    const worker = new Worker(this.workerPath);
    worker.on('message', (message) => this.onMessage(worker, message));
    worker.on('error', (error) => this.onError(worker, error));
    worker.on('exit', (code) => this.onExit(worker, code));
    return worker;
  }

  private onMessage(worker: Worker, message: { id: number; ok: boolean; result?: unknown; error?: string }): void {
    const task = this.pending.get(message.id);
    if (!task) {
      return;
    }
    this.pending.delete(message.id);
    const slot = this.workers.find((w) => w.worker === worker);
    if (slot) {
      slot.busy = false;
      slot.currentTaskId = undefined;
    }

    if (message.ok) {
      task.resolve(message.result);
    } else {
      task.reject(new Error(message.error || 'Worker task failed'));
    }

    this.processQueue();
  }

  private onError(worker: Worker, error: Error): void {
    console.error('[WorkerPool] Worker error:', error);
    const slot = this.workers.find((w) => w.worker === worker);
    if (slot) {
      slot.busy = false;
      if (slot.currentTaskId !== undefined) {
        const task = this.pending.get(slot.currentTaskId);
        if (task) {
          this.pending.delete(slot.currentTaskId);
          task.reject(error);
        }
      }
      slot.currentTaskId = undefined;
    }
    this.processQueue();
  }

  private onExit(worker: Worker, code: number): void {
    const slotIndex = this.workers.findIndex((w) => w.worker === worker);
    if (slotIndex === -1) {
      return;
    }
    const slot = this.workers[slotIndex];
    if (slot.currentTaskId !== undefined) {
      const task = this.pending.get(slot.currentTaskId);
      if (task) {
        this.pending.delete(slot.currentTaskId);
        task.reject(new Error('Worker exited while running task'));
      }
    }
    this.workers.splice(slotIndex, 1);
    if (code !== 0) {
      this.workers.push({ worker: this.spawnWorker(), busy: false });
    }
  }

  private processQueue(): void {
    let idleSlot = this.workers.find((slot) => !slot.busy);
    while (idleSlot) {
      const task = this.queue.shift();
      if (!task) {
        return;
      }

      idleSlot.busy = true;
      idleSlot.currentTaskId = task.id;
      this.pending.set(task.id, task);
      console.log(`[WorkerPool:${this.label}] start task ${task.id}`);
      idleSlot.worker.postMessage({ id: task.id, payload: task.payload });
      idleSlot = this.workers.find((slot) => !slot.busy);
    }
  }
}
