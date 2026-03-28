/**
 * BackendManager - Manages the Cortex backend daemon lifecycle
 * 
 * Automatically starts, monitors, and stops the backend Go process.
 * Handles platform-specific binary paths and configuration.
 */

import * as vscode from "vscode";
import * as path from "path";
import * as fs from "fs-extra";
import { spawn, ChildProcess } from "child_process";
import { GrpcAdminClient } from "./GrpcAdminClient";

export interface BackendConfig {
  grpcAddress?: string;
  httpAddress?: string;
  dataDir?: string;
  logLevel?: string;
  tika?: {
    enabled?: boolean;
    manageProcess?: boolean;
    autoDownload?: boolean;
  };
  llm?: {
    enabled?: boolean;
    endpoint?: string;
  };
}

export class BackendManager {
  private process?: ChildProcess;
  private readonly extensionPath: string;
  private readonly dataDir: string;
  private readonly configPath: string;
  private readonly logPath: string;
  private isStarting = false;
  private isStopping = false;

  constructor(private context: vscode.ExtensionContext) {
    this.extensionPath = context.extensionPath;
    this.dataDir = path.join(context.globalStorageUri.fsPath, "cortex");
    this.configPath = path.join(this.dataDir, "cortexd.yaml");
    this.logPath = path.join(this.dataDir, "cortexd.log");
  }

  /**
   * Starts the backend daemon if not already running.
   */
  async start(): Promise<void> {
    if (this.isStarting) {
      throw new Error("Backend is already starting");
    }

    if (this.isRunning()) {
      vscode.window.showInformationMessage("Cortex backend is already running");
      return;
    }

    this.isStarting = true;

    try {
      // Ensure directories exist
      await fs.ensureDir(this.dataDir);

      // Generate default config if it doesn't exist
      if (!(await fs.pathExists(this.configPath))) {
        await this.generateDefaultConfig();
      }

      // Get backend binary path
      const backendPath = await this.getBackendBinary();

      // Start backend process
      await this.startProcess(backendPath);

      // Wait for backend to be ready
      await this.waitForReady();

      vscode.window.showInformationMessage("Cortex backend started successfully");
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      vscode.window.showErrorMessage(`Failed to start Cortex backend: ${message}`);
      throw error;
    } finally {
      this.isStarting = false;
    }
  }

  /**
   * Stops the backend daemon gracefully.
   */
  async stop(): Promise<void> {
    if (this.isStopping) {
      return;
    }

    if (!this.process) {
      return;
    }

    this.isStopping = true;

    try {
      // Send SIGTERM for graceful shutdown
      if (this.process.pid) {
        process.kill(this.process.pid, "SIGTERM");
      }

      // Wait for process to exit (max 5 seconds)
      const proc = this.process;
      if (proc) {
        await new Promise<void>((resolve) => {
          const timeout = setTimeout(() => {
            if (proc && proc.pid) {
              // Force kill if still running
              process.kill(proc.pid, "SIGKILL");
            }
            resolve();
          }, 5000);

          proc.once("exit", () => {
            clearTimeout(timeout);
            resolve();
          });
        });
      }

      this.process = undefined;
      vscode.window.showInformationMessage("Cortex backend stopped");
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      vscode.window.showWarningMessage(`Error stopping backend: ${message}`);
    } finally {
      this.isStopping = false;
    }
  }

  /**
   * Checks if the backend is currently running.
   */
  isRunning(): boolean {
    return this.process !== undefined && this.process.exitCode === null;
  }

  /**
   * Gets the backend process status.
   */
  getStatus(): { running: boolean; pid?: number } {
    return {
      running: this.isRunning(),
      pid: this.process?.pid,
    };
  }

  /**
   * Gets the backend binary path for the current platform.
   */
  private async getBackendBinary(): Promise<string> {
    const platform = process.platform;
    const arch = process.arch;

    // Map architectures
    const archMap: Record<string, string> = {
      x64: "x64",
      x32: "x64", // 32-bit uses 64-bit binary
      arm64: "arm64",
      arm: "arm64",
    };

    // Map platforms
    const platformMap: Record<string, string> = {
      darwin: "darwin",
      linux: "linux",
      win32: "win32",
    };

    const mappedArch = archMap[arch] || "x64";
    const mappedPlatform = platformMap[platform] || platform;
    const ext = platform === "win32" ? ".exe" : "";

    const binDir = path.join(this.extensionPath, "bin", `${mappedPlatform}-${mappedArch}`);
    const binary = path.join(binDir, `cortexd${ext}`);

    if (!(await fs.pathExists(binary))) {
      throw new Error(
        `Backend binary not found for ${platform}-${arch}. ` +
          `Expected: ${binary}. ` +
          `Please report this issue or build the backend manually.`
      );
    }

    // Make executable on Unix
    if (platform !== "win32") {
      await fs.chmod(binary, 0o755);
    }

    return binary;
  }

  /**
   * Starts the backend process.
   */
  private async startProcess(binaryPath: string): Promise<void> {
    const args = [
      "--config",
      this.configPath,
      "--data-dir",
      this.dataDir,
    ];

    // Create log file stream
    const logStream = await fs.createWriteStream(this.logPath, { flags: "a" });
    const logLine = (data: Buffer) => {
      const timestamp = new Date().toISOString();
      logStream.write(`[${timestamp}] ${data}`);
    };

    this.process = spawn(binaryPath, args, {
      stdio: ["ignore", "pipe", "pipe"],
      detached: false,
      cwd: this.dataDir,
    });

    // Log stdout
    this.process.stdout?.on("data", (data: Buffer) => {
      logLine(data);
      console.log(`[Backend] ${data.toString().trim()}`);
    });

    // Log stderr
    this.process.stderr?.on("data", (data: Buffer) => {
      logLine(data);
      console.error(`[Backend] ${data.toString().trim()}`);
    });

    // Handle process exit
    this.process.on("exit", (code, signal) => {
      if (code !== 0 && code !== null && !this.isStopping) {
        vscode.window.showErrorMessage(
          `Cortex backend exited unexpectedly (code: ${code}, signal: ${signal})`
        );
      }
      this.process = undefined;
    });

    // Handle process errors
    this.process.on("error", (error) => {
      vscode.window.showErrorMessage(`Failed to start backend process: ${error.message}`);
      this.process = undefined;
    });
  }

  /**
   * Waits for the backend to be ready (gRPC server responding).
   */
  private async waitForReady(timeout = 30000): Promise<void> {
    const startTime = Date.now();
    const checkInterval = 500;
    const config = vscode.workspace.getConfiguration("cortex");
    const grpcAddress = config.get<string>("grpc.address", "127.0.0.1:50051");

    while (Date.now() - startTime < timeout) {
      try {
        // Try to connect via gRPC
        const client = new GrpcAdminClient(this.context);
        await client.healthCheck();
        return; // Success!
      } catch (error) {
        // Not ready yet, wait and retry
        await new Promise((resolve) => setTimeout(resolve, checkInterval));
      }
    }

    throw new Error(
      `Backend failed to start within ${timeout}ms. ` +
        `Check logs at: ${this.logPath}`
    );
  }

  /**
   * Generates default configuration file.
   */
  private async generateDefaultConfig(): Promise<void> {
    const config: BackendConfig = {
      grpcAddress: "127.0.0.1:50051",
      httpAddress: "127.0.0.1:8081",
      dataDir: this.dataDir,
      logLevel: "info",
      tika: {
        enabled: false, // Disabled by default
        manageProcess: true,
        autoDownload: true,
      },
      llm: {
        enabled: false, // Disabled by default
        endpoint: "http://localhost:11434",
      },
    };

    // Convert to YAML-like structure (simplified)
    const yaml = this.configToYAML(config);
    await fs.writeFile(this.configPath, yaml, "utf8");
  }

  /**
   * Converts config object to YAML string (simplified).
   */
  private configToYAML(config: BackendConfig): string {
    // Simple YAML generation (could use a library like js-yaml)
    let yaml = `# Cortex Backend Configuration (Auto-generated)\n\n`;
    yaml += `grpc_address: "${config.grpcAddress || "127.0.0.1:50051"}"\n`;
    yaml += `http_address: "${config.httpAddress || "127.0.0.1:8081"}"\n`;
    yaml += `data_dir: "${this.dataDir}"\n`;
    yaml += `log_level: "${config.logLevel || "info"}"\n\n`;

    if (config.tika) {
      yaml += `tika:\n`;
      yaml += `  enabled: ${config.tika.enabled ?? false}\n`;
      yaml += `  manage_process: ${config.tika.manageProcess ?? true}\n`;
      yaml += `  auto_download: ${config.tika.autoDownload ?? true}\n`;
      yaml += `  endpoint: "http://localhost:9998"\n`;
      yaml += `  port: 9998\n\n`;
    }

    if (config.llm) {
      yaml += `llm:\n`;
      yaml += `  enabled: ${config.llm.enabled ?? false}\n`;
      yaml += `  endpoint: "${config.llm.endpoint || "http://localhost:11434"}"\n\n`;
    }

    return yaml;
  }

  /**
   * Gets the log file path.
   */
  getLogPath(): string {
    return this.logPath;
  }

  /**
   * Gets the configuration file path.
   */
  getConfigPath(): string {
    return this.configPath;
  }

  /**
   * Gets the data directory path.
   */
  getDataDir(): string {
    return this.dataDir;
  }
}

