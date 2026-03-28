import * as path from "path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type AdminClient = grpc.Client & {
  GetStatus: (req: object, cb: grpc.requestCallback<any>) => void;
  GetMetrics: (req: object, cb: grpc.requestCallback<any>) => void;
  HealthCheck: (req: object, cb: grpc.requestCallback<any>) => void;
  GetConfig: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdateConfig: (req: object, cb: grpc.requestCallback<any>) => void;
  GetConfigVersions: (req: object, cb: grpc.requestCallback<any>) => void;
  GetConfigVersion: (req: object, cb: grpc.requestCallback<any>) => void;
  RestoreConfigVersion: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdatePrompts: (req: object, cb: grpc.requestCallback<any>) => void;
  GetEnv: (req: object, cb: grpc.requestCallback<any>) => void;
  GetLogs: (req: object, cb: grpc.requestCallback<any>) => void;
  StreamPipeline: (req: object) => grpc.ClientReadableStream<any>;
  ListWorkspaces: (req: object) => grpc.ClientReadableStream<any>;
  RegisterWorkspace: (req: object, cb: grpc.requestCallback<any>) => void;
  GetDashboardMetrics: (req: object, cb: grpc.requestCallback<any>) => void;
  GetConfidenceDistribution: (req: object, cb: grpc.requestCallback<any>) => void;
  GetModelDriftReport: (req: object, cb: grpc.requestCallback<any>) => void;
};

type FileClient = grpc.Client & {
  GetFile: (req: object, cb: grpc.requestCallback<any>) => void;
  ListFiles: (req: object) => grpc.ClientReadableStream<any>;
  ProcessFile: (req: object, cb: grpc.requestCallback<any>) => void;
  ScanWorkspace: (req: object) => grpc.ClientReadableStream<any>;
  ListByType: (req: object) => grpc.ClientReadableStream<any>;
  ListByFolder: (req: object) => grpc.ClientReadableStream<any>;
  ListByDate: (req: object) => grpc.ClientReadableStream<any>;
  ListBySize: (req: object) => grpc.ClientReadableStream<any>;
  ListByContentType: (req: object) => grpc.ClientReadableStream<any>;
};

type MetadataClient = grpc.Client & {
  ListProcessingTraces: (req: object, cb: grpc.requestCallback<any>) => void;
};

type AdminSnapshot = {
  status: any;
  metrics: any;
  health: any;
  config: any;
  env: any;
  logs: any;
  workspaces: any[];
  files: any[];
};

export type PipelineEvent = {
  id?: string;
  type?: string;
  file_path?: string;
  stage?: string;
  error?: string;
  timestamp_unix?: number | string;
  workspace_id?: string;
};

export class GrpcAdminClient {
  private client: AdminClient | null = null;
  private fileClient: FileClient | null = null;
  private metadataClient: MetadataClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async getSnapshot(): Promise<AdminSnapshot> {
    const [health, status, metrics, config, env, logs, workspaces] = await Promise.all([
      this.healthCheck(),
      this.getStatus(),
      this.getMetrics(),
      this.getConfig(),
      this.getEnv(),
      this.getLogs(),
      this.listWorkspaces(),
    ]);

    const files = await this.listAllFiles(workspaces);
    return { health, status, metrics, config, env, logs, workspaces, files };
  }

  async getStatus(): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetStatus.bind(client), {});
  }

  async getMetrics(): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetMetrics.bind(client), {});
  }

  async getConfig(): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetConfig.bind(client), {});
  }

  async updateConfig(config: any, persist = true): Promise<any> {
    const client = this.getClient();
    return this.unary(client.UpdateConfig.bind(client), { config, persist });
  }

  async getConfigVersions(limit = 50, offset = 0): Promise<{ versions: any[]; total: number }> {
    const client = this.getClient();
    return this.unary(client.GetConfigVersions.bind(client), { limit, offset });
  }

  async getConfigVersion(versionId: string): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetConfigVersion.bind(client), { version_id: versionId });
  }

  async restoreConfigVersion(versionId: string, createBackup = true, description = ""): Promise<any> {
    const client = this.getClient();
    return this.unary(client.RestoreConfigVersion.bind(client), {
      version_id: versionId,
      create_backup: createBackup,
      description,
    });
  }

  async updatePrompts(prompts: any, createVersion = true, description = ""): Promise<any> {
    const client = this.getClient();
    return this.unary(client.UpdatePrompts.bind(client), {
      prompts,
      create_version: createVersion,
      description,
    });
  }

  async getEnv(): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetEnv.bind(client), {});
  }

  async getLogs(tailLines = 200): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetLogs.bind(client), { tail_lines: tailLines });
  }

  subscribePipelineEvents(
    onEvent: (event: PipelineEvent) => void,
    workspaceId?: string
  ): () => void {
    const client = this.getClient();
    const stream = client.StreamPipeline(
      workspaceId ? { workspace_id: workspaceId } : {}
    );
    stream.on("data", (event: PipelineEvent) => onEvent(event));
    stream.on("error", (err: Error) => {
      console.warn("[Cortex Admin] Pipeline stream error", err);
    });
    return () => stream.cancel();
  }

  async healthCheck(): Promise<any> {
    const client = this.getClient();
    return this.unary(client.HealthCheck.bind(client), {});
  }

  async listWorkspaces(): Promise<any[]> {
    const client = this.getClient();
    return new Promise((resolve, reject) => {
      const stream = client.ListWorkspaces({});
      const items: any[] = [];

      stream.on("data", (item: any) => items.push(item));
      stream.on("error", (err: Error) => reject(err));
      stream.on("end", () => resolve(items));
    });
  }

  async registerWorkspace(path: string, name?: string): Promise<any> {
    const client = this.getClient();
    return this.unary(client.RegisterWorkspace.bind(client), {
      path,
      name,
    });
  }

  async listFiles(workspaceId: string): Promise<any[]> {
    const client = this.getFileClient();
    return this.listFilesForWorkspace(client, workspaceId);
  }

  async getFile(workspaceId: string, relativePath: string): Promise<any> {
    const client = this.getFileClient();
    return this.unary(client.GetFile.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
    });
  }

  async processFile(workspaceId: string, relativePath: string): Promise<any> {
    const client = this.getFileClient();
    return this.unary(client.ProcessFile.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
    });
  }

  async scanWorkspace(workspaceId: string, path: string, forceFullScan = true): Promise<void> {
    const client = this.getFileClient();
    return new Promise((resolve, reject) => {
      const stream = client.ScanWorkspace({
        workspace_id: workspaceId,
        path,
        force_full_scan: forceFullScan,
      });
      stream.on("data", () => undefined);
      stream.on("error", (err: Error) => reject(err));
      stream.on("end", () => resolve());
    });
  }

  async listProcessingTraces(
    workspaceId: string,
    relativePath?: string,
    limit = 50
  ): Promise<any[]> {
    const client = this.getMetadataClient();
    const resp = await this.unary(client.ListProcessingTraces.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      limit,
    });
    return resp?.traces ?? [];
  }

  /**
   * Fetches AI metadata quality dashboard metrics.
   * @param workspaceId The workspace ID
   * @param periodHours Time period in hours (default: 24)
   */
  async getDashboardMetrics(workspaceId: string, periodHours = 24): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetDashboardMetrics.bind(client), {
      workspace_id: workspaceId,
      period_hours: periodHours,
    });
  }

  /**
   * Fetches confidence score distributions by category.
   * @param workspaceId The workspace ID
   */
  async getConfidenceDistribution(workspaceId: string): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetConfidenceDistribution.bind(client), {
      workspace_id: workspaceId,
    });
  }

  /**
   * Fetches model drift detection report.
   * @param workspaceId The workspace ID
   */
  async getModelDriftReport(workspaceId: string): Promise<any> {
    const client = this.getClient();
    return this.unary(client.GetModelDriftReport.bind(client), {
      workspace_id: workspaceId,
    });
  }

  async listByType(workspaceId: string, extension: string): Promise<any[]> {
    const client = this.getFileClient();
    return this.streamToArray(
      client.ListByType({ workspace_id: workspaceId, extension })
    );
  }

  async listByFolder(workspaceId: string, folder: string, recursive = false): Promise<any[]> {
    const client = this.getFileClient();
    return this.streamToArray(
      client.ListByFolder({ workspace_id: workspaceId, folder, recursive })
    );
  }

  async listByDate(workspaceId: string, startDate?: number, endDate?: number): Promise<any[]> {
    const client = this.getFileClient();
    return this.streamToArray(
      client.ListByDate({
        workspace_id: workspaceId,
        start_date: startDate,
        end_date: endDate,
      })
    );
  }

  async listBySize(workspaceId: string, minSize?: number, maxSize?: number): Promise<any[]> {
    const client = this.getFileClient();
    return this.streamToArray(
      client.ListBySize({
        workspace_id: workspaceId,
        min_size: minSize,
        max_size: maxSize,
      })
    );
  }

  async listByContentType(workspaceId: string, category: string): Promise<any[]> {
    const client = this.getFileClient();
    return this.streamToArray(
      client.ListByContentType({ workspace_id: workspaceId, category })
    );
  }

  private streamToArray(stream: grpc.ClientReadableStream<any>): Promise<any[]> {
    return new Promise((resolve, reject) => {
      const items: any[] = [];
      stream.on("data", (item: any) => items.push(item));
      stream.on("error", (err: Error) => reject(err));
      stream.on("end", () => resolve(items));
    });
  }

  async listAllFiles(workspaces: any[]): Promise<any[]> {
    const client = this.getFileClient();
    const files: any[] = [];

    for (const workspace of workspaces) {
      const workspaceId = workspace.id;
      if (!workspaceId) {
        continue;
      }
      const chunk = await this.listFilesForWorkspace(client, workspaceId);
      for (const file of chunk) {
        files.push({
          workspaceId,
          workspaceName: workspace.name || workspace.path || workspaceId,
          entry: file,
        });
      }
    }

    console.log(`[Cortex Admin] Loaded ${files.length} indexed files`);
    return files;
  }

  private unary(
    call: (req: object, cb: grpc.requestCallback<any>) => void,
    req: object
  ): Promise<any> {
    return new Promise((resolve, reject) => {
      call(req, (err, resp) => {
        if (err) {
          reject(err);
          return;
        }
        resolve(resp);
      });
    });
  }

  private getClient(): AdminClient {
    const address = this.getAddress();
    if (this.client && this.clientAddress === address) {
      return this.client;
    }

    if (this.client) {
      this.client.close();
    }
    if (this.fileClient) {
      this.fileClient.close();
      this.fileClient = null;
    }
    if (this.metadataClient) {
      this.metadataClient.close();
      this.metadataClient = null;
    }

    const protoRoot = path.join(this.context.extensionPath, "backend", "api", "proto");
    const packageDef = protoLoader.loadSync(
      [
        path.join(protoRoot, "cortex", "v1", "admin.proto"),
        path.join(protoRoot, "cortex", "v1", "metadata.proto"),
        path.join(protoRoot, "cortex", "v1", "common.proto"),
        path.join(protoRoot, "cortex", "v1", "file.proto"),
        path.join(protoRoot, "cortex", "v1", "task.proto"),
        path.join(protoRoot, "cortex", "v1", "llm.proto"),
      ],
      {
        includeDirs: [protoRoot],
        keepCase: true,
        longs: String,
        enums: String,
        defaults: true,
        oneofs: true,
      }
    );

    const loaded = grpc.loadPackageDefinition(packageDef) as any;
    const AdminService = loaded.cortex?.v1?.AdminService;
    const FileService = loaded.cortex?.v1?.FileService;
    const MetadataService = loaded.cortex?.v1?.MetadataService;
    if (!AdminService) {
      throw new Error("AdminService not found in proto definitions");
    }
    if (!FileService) {
      throw new Error("FileService not found in proto definitions");
    }
    if (!MetadataService) {
      throw new Error("MetadataService not found in proto definitions");
    }

    // Configure client with extended timeouts for long-running streams
    const channelOptions = {
      'grpc.keepalive_time_ms': 300000,        // 5 minutes
      'grpc.keepalive_timeout_ms': 10000,      // 10 seconds
      'grpc.keepalive_permit_without_calls': true,
      'grpc.http2.max_pings_without_data': 0,  // Unlimited pings
      'grpc.http2.min_time_between_pings_ms': 10000,  // 10 seconds
      'grpc.http2.min_ping_interval_without_data_ms': 300000,  // 5 minutes
    };

    this.client = new AdminService(
      address,
      grpc.credentials.createInsecure(),
      channelOptions
    ) as AdminClient;
    this.fileClient = new FileService(
      address,
      grpc.credentials.createInsecure(),
      channelOptions
    ) as FileClient;
    this.metadataClient = new MetadataService(
      address,
      grpc.credentials.createInsecure(),
      channelOptions
    ) as MetadataClient;
    this.clientAddress = address;
    return this.client;
  }

  private getMetadataClient(): MetadataClient {
    this.getClient();
    if (!this.metadataClient) {
      throw new Error("Metadata client not initialized");
    }
    return this.metadataClient;
  }

  private getFileClient(): FileClient {
    if (this.client && this.clientAddress === this.getAddress() && this.fileClient) {
      return this.fileClient;
    }
    this.getClient();
    if (!this.fileClient) {
      throw new Error("FileService client not initialized");
    }
    return this.fileClient;
  }

  private listFilesForWorkspace(client: FileClient, workspaceId: string): Promise<any[]> {
    const limit = 500;
    let offset = 0;
    const items: any[] = [];

    const fetchPage = (): Promise<void> =>
      new Promise((resolve, reject) => {
        const stream = client.ListFiles({
          workspace_id: workspaceId,
          pagination: { offset, limit },
        });

        let count = 0;
        stream.on("data", (item: any) => {
          items.push(item);
          count += 1;
        });
        stream.on("error", (err: Error) => reject(err));
        stream.on("end", () => {
          console.log(
            `[Cortex Admin] Workspace ${workspaceId} page ${offset}-${offset + limit} -> ${count} files`
          );
          if (count < limit) {
            resolve();
          } else {
            offset += limit;
            resolve(fetchPage());
          }
        });
      });

    return fetchPage().then(() => items);
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
