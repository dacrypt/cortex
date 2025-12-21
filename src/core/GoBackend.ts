/**
 * GoBackend - Client para comunicarse con un backend Go empaquetado
 * 
 * Este módulo maneja:
 * - Detección del binario correcto según la plataforma
 * - Inicio y gestión del proceso Go
 * - Comunicación mediante stdin/stdout (JSON-RPC style)
 * - Comunicación mediante HTTP (si el backend expone un servidor)
 */

import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs/promises';
import { spawn, ChildProcess } from 'child_process';
import { EventEmitter } from 'events';

export interface GoBackendConfig {
  /** Ruta base donde están los binarios (relativa a extensionPath) */
  binariesPath?: string;
  /** Nombre del binario (sin extensión ni plataforma) */
  binaryName?: string;
  /** Puerto para comunicación HTTP (si se usa) */
  httpPort?: number;
  /** Timeout para operaciones (ms) */
  timeout?: number;
}

export interface BackendRequest {
  id: string;
  method: string;
  params?: any;
}

export interface BackendResponse {
  id: string;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
}

/**
 * Cliente para comunicarse con el backend Go
 */
export class GoBackend extends EventEmitter {
  private process: ChildProcess | null = null;
  private binaryPath: string | null = null;
  private isRunning = false;
  private requestIdCounter = 0;
  private pendingRequests = new Map<string, {
    resolve: (value: BackendResponse) => void;
    reject: (error: Error) => void;
    timeout: NodeJS.Timeout;
  }>();

  constructor(
    private context: vscode.ExtensionContext,
    private config: GoBackendConfig = {}
  ) {
    super();
  }

  /**
   * Obtiene la ruta del binario según la plataforma
   */
  private getBinaryPath(): string {
    if (this.binaryPath) {
      return this.binaryPath;
    }

    const binariesPath = this.config.binariesPath || 'bin';
    const binaryName = this.config.binaryName || 'cortex-backend';
    
    // Determinar plataforma y arquitectura
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

  /**
   * Verifica si el binario existe
   */
  async checkBinaryExists(): Promise<boolean> {
    try {
      const binaryPath = this.getBinaryPath();
      await fs.access(binaryPath);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Inicia el backend Go
   */
  async start(): Promise<void> {
    if (this.isRunning) {
      return;
    }

    const binaryPath = this.getBinaryPath();
    const exists = await this.checkBinaryExists();
    
    if (!exists) {
      throw new Error(
        `Backend binary not found: ${binaryPath}\n` +
        `Make sure to build and include binaries for your platform in the 'bin' directory.`
      );
    }

    // Hacer el binario ejecutable (en Unix)
    if (os.platform() !== 'win32') {
      try {
        await fs.chmod(binaryPath, 0o755);
      } catch (error) {
        console.warn(`[GoBackend] Failed to chmod binary:`, error);
      }
    }

    // Iniciar el proceso
    this.process = spawn(binaryPath, [], {
      stdio: ['pipe', 'pipe', 'pipe'],
      cwd: this.context.extensionPath,
    });

    this.isRunning = true;
    this.emit('started');

    // Manejar stdout (respuestas JSON)
    let buffer = '';
    this.process.stdout?.on('data', (data: Buffer) => {
      buffer += data.toString();
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.trim()) {
          try {
            const response: BackendResponse = JSON.parse(line);
            this.handleResponse(response);
          } catch (error) {
            console.error(`[GoBackend] Failed to parse response:`, line, error);
          }
        }
      }
    });

    // Manejar stderr
    this.process.stderr?.on('data', (data: Buffer) => {
      const message = data.toString();
      console.log(`[GoBackend] stderr:`, message);
      this.emit('stderr', message);
    });

    // Manejar cierre del proceso
    this.process.on('exit', (code, signal) => {
      this.isRunning = false;
      this.process = null;
      this.emit('exited', code, signal);
      
      // Rechazar todas las peticiones pendientes
      for (const [id, request] of this.pendingRequests.entries()) {
        clearTimeout(request.timeout);
        request.reject(new Error(`Backend process exited (code: ${code}, signal: ${signal})`));
      }
      this.pendingRequests.clear();
    });

    // Esperar a que el backend esté listo (opcional)
    await this.waitForReady();
  }

  /**
   * Espera a que el backend esté listo
   */
  private async waitForReady(timeout = 5000): Promise<void> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error('Backend startup timeout'));
      }, timeout);

      // Intentar hacer un ping
      this.call('ping', {})
        .then(() => {
          clearTimeout(timer);
          resolve();
        })
        .catch(() => {
          // Si falla, asumimos que está listo de todas formas
          clearTimeout(timer);
          resolve();
        });
    });
  }

  /**
   * Detiene el backend
   */
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

      // Intentar cerrar gracefully
      this.process.kill('SIGTERM');

      // Si no responde, forzar cierre
      setTimeout(() => {
        if (this.process) {
          this.process.kill('SIGKILL');
        }
        resolve();
      }, 2000);
    });
  }

  /**
   * Envía una petición al backend y espera respuesta
   */
  async call(method: string, params?: any): Promise<any> {
    if (!this.isRunning || !this.process) {
      throw new Error('Backend is not running. Call start() first.');
    }

    const id = `req_${++this.requestIdCounter}`;
    const request: BackendRequest = {
      id,
      method,
      params,
    };

    return new Promise((resolve, reject) => {
      const timeout = this.config.timeout || 30000;
      const timeoutId = setTimeout(() => {
        this.pendingRequests.delete(id);
        reject(new Error(`Request timeout: ${method}`));
      }, timeout);

      this.pendingRequests.set(id, {
        resolve: (response) => {
          clearTimeout(timeoutId);
          if (response.error) {
            reject(new Error(response.error.message));
          } else {
            resolve(response.result);
          }
        },
        reject: (error) => {
          clearTimeout(timeoutId);
          reject(error);
        },
        timeout: timeoutId,
      });

      // Enviar petición
      const requestLine = JSON.stringify(request) + '\n';
      this.process?.stdin?.write(requestLine);
    });
  }

  /**
   * Maneja una respuesta del backend
   */
  private handleResponse(response: BackendResponse): void {
    const request = this.pendingRequests.get(response.id);
    if (request) {
      this.pendingRequests.delete(response.id);
      request.resolve(response);
    } else {
      // Respuesta sin petición pendiente (notificación o error)
      this.emit('notification', response);
    }
  }

  /**
   * Verifica si el backend está corriendo
   */
  get running(): boolean {
    return this.isRunning;
  }
}


