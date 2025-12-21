/**
 * GoBackendHTTP - Cliente HTTP para comunicarse con backend Go
 * 
 * Alternativa al GoBackend que usa HTTP en lugar de stdin/stdout
 * Útil cuando el backend Go expone un servidor HTTP
 */

import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs/promises';
import { spawn, ChildProcess } from 'child_process';
import { EventEmitter } from 'events';
import * as http from 'http';

export interface GoBackendHTTPConfig {
  binariesPath?: string;
  binaryName?: string;
  port?: number;
  host?: string;
  timeout?: number;
}

export class GoBackendHTTP extends EventEmitter {
  private process: ChildProcess | null = null;
  private binaryPath: string | null = null;
  private isRunning = false;
  private baseUrl: string;

  constructor(
    private context: vscode.ExtensionContext,
    private config: GoBackendHTTPConfig = {}
  ) {
    super();
    const port = config.port || 8080;
    const host = config.host || 'localhost';
    this.baseUrl = `http://${host}:${port}`;
  }

  private getBinaryPath(): string {
    if (this.binaryPath) {
      return this.binaryPath;
    }

    const binariesPath = this.config.binariesPath || 'bin';
    const binaryName = this.config.binaryName || 'cortex-backend';
    
    let platform: string;
    let arch: string;
    let ext = '';

    switch (os.platform()) {
      case 'win32':
        platform = 'windows';
        ext = '.exe';
        break;
      case 'darwin':
        platform = 'darwin';
        break;
      case 'linux':
        platform = 'linux';
        break;
      default:
        throw new Error(`Unsupported platform: ${os.platform()}`);
    }

    switch (os.arch()) {
      case 'x64':
        arch = 'amd64';
        break;
      case 'arm64':
        arch = 'arm64';
        break;
      case 'ia32':
        arch = '386';
        break;
      default:
        arch = os.arch();
    }

    const binaryPath = path.join(
      this.context.extensionPath,
      binariesPath,
      `${binaryName}-${platform}-${arch}${ext}`
    );

    this.binaryPath = binaryPath;
    return binaryPath;
  }

  async checkBinaryExists(): Promise<boolean> {
    try {
      const binaryPath = this.getBinaryPath();
      await fs.access(binaryPath);
      return true;
    } catch {
      return false;
    }
  }

  async start(): Promise<void> {
    if (this.isRunning) {
      return;
    }

    const binaryPath = this.getBinaryPath();
    const exists = await this.checkBinaryExists();
    
    if (!exists) {
      throw new Error(`Backend binary not found: ${binaryPath}`);
    }

    if (os.platform() !== 'win32') {
      try {
        await fs.chmod(binaryPath, 0o755);
      } catch (error) {
        console.warn(`[GoBackendHTTP] Failed to chmod binary:`, error);
      }
    }

    const port = this.config.port || 8080;
    this.process = spawn(binaryPath, ['--port', port.toString()], {
      stdio: ['pipe', 'pipe', 'pipe'],
      cwd: this.context.extensionPath,
    });

    this.isRunning = true;
    this.emit('started');

    this.process.stderr?.on('data', (data: Buffer) => {
      const message = data.toString();
      console.log(`[GoBackendHTTP] stderr:`, message);
      this.emit('stderr', message);
    });

    this.process.on('exit', (code, signal) => {
      this.isRunning = false;
      this.process = null;
      this.emit('exited', code, signal);
    });

    // Esperar a que el servidor esté listo
    await this.waitForServer();
  }

  private async waitForServer(maxAttempts = 30, delay = 200): Promise<void> {
    for (let i = 0; i < maxAttempts; i++) {
      try {
        const response = await this.request('GET', '/health');
        if (response.statusCode === 200) {
          return;
        }
      } catch {
        // Servidor aún no está listo
      }
      await new Promise(resolve => setTimeout(resolve, delay));
    }
    throw new Error('Backend server failed to start');
  }

  async stop(): Promise<void> {
    if (!this.isRunning || !this.process) {
      return;
    }

    return new Promise((resolve) => {
      if (!this.process) {
        resolve();
        return;
      }

      this.process.once('exit', () => {
        this.isRunning = false;
        this.process = null;
        resolve();
      });

      this.process.kill('SIGTERM');
      setTimeout(() => {
        if (this.process) {
          this.process.kill('SIGKILL');
        }
        resolve();
      }, 2000);
    });
  }

  private request(method: string, path: string, body?: any): Promise<http.IncomingMessage> {
    return new Promise((resolve, reject) => {
      const url = new URL(path, this.baseUrl);
      const options: http.RequestOptions = {
        method,
        path: url.pathname + url.search,
        headers: {
          'Content-Type': 'application/json',
        },
      };

      const req = http.request(options, (res) => {
        resolve(res);
      });

      req.on('error', reject);

      if (body) {
        req.write(JSON.stringify(body));
      }

      req.end();
    });
  }

  async call(method: string, params?: any): Promise<any> {
    if (!this.isRunning) {
      throw new Error('Backend is not running');
    }

    const response = await this.request('POST', '/api', {
      method,
      params,
    });

    return new Promise((resolve, reject) => {
      let data = '';
      response.on('data', (chunk) => {
        data += chunk.toString();
      });
      response.on('end', () => {
        try {
          const result = JSON.parse(data);
          if (response.statusCode === 200) {
            resolve(result);
          } else {
            reject(new Error(result.error || 'Request failed'));
          }
        } catch (error) {
          reject(error);
        }
      });
      response.on('error', reject);
    });
  }

  get running(): boolean {
    return this.isRunning;
  }
}


