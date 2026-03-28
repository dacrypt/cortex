import * as path from "path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type LLMClient = grpc.Client & {
  ListProviders: (req: object, cb: grpc.requestCallback<any>) => void;
  GetProviderStatus: (req: object, cb: grpc.requestCallback<any>) => void;
  SuggestProject: (req: object, cb: grpc.requestCallback<any>) => void;
  SuggestTags: (req: object, cb: grpc.requestCallback<any>) => void;
  GenerateSummary: (req: object, cb: grpc.requestCallback<any>) => void;
  GenerateCompletion: (req: object, cb: grpc.requestCallback<any>) => void;
};

export class GrpcLLMClient {
  private client: LLMClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async listProviders(): Promise<any[]> {
    const client = this.getClient();
    const resp = await this.unary(client.ListProviders.bind(client), {});
    return resp?.providers ?? [];
  }

  async getProviderStatus(providerId: string): Promise<any> {
    const client = this.getClient();
    return await this.unary(client.GetProviderStatus.bind(client), {
      provider_id: providerId,
    });
  }

  async suggestProject(request: {
    workspaceId: string;
    relativePath: string;
    content?: string;
    existingProjects?: string[];
  }): Promise<{ project?: string; confidence: number; reason: string }> {
    const client = this.getClient();
    const req: any = {
      workspace_id: request.workspaceId,
      relative_path: request.relativePath,
    };
    if (request.content) {
      req.content = request.content;
    }
    if (request.existingProjects) {
      req.existing_projects = request.existingProjects;
    }
    const resp = await this.unary(client.SuggestProject.bind(client), req);
    return {
      project: resp?.project,
      confidence: resp?.confidence ?? 0.0,
      reason: resp?.reason ?? '',
    };
  }

  async generateCompletion(request: {
    prompt: string;
    model?: string;
    providerId?: string;
    maxTokens?: number;
    temperature?: number;
    timeoutMs?: number;
  }): Promise<string> {
    const client = this.getClient();
    const resp = await this.unary(client.GenerateCompletion.bind(client), {
      prompt: request.prompt,
      provider_id: request.providerId,
      model: request.model,
      max_tokens: request.maxTokens ?? 500,
      temperature: request.temperature ?? 0.3,
      timeout_ms: request.timeoutMs ?? 30000,
    });
    return resp?.text ?? "";
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

  private getClient(): LLMClient {
    const address = this.getAddress();
    if (this.client && this.clientAddress === address) {
      return this.client;
    }

    if (this.client) {
      this.client.close();
    }

    const protoRoot = path.join(
      this.context.extensionPath,
      "backend",
      "api",
      "proto"
    );
    const packageDef = protoLoader.loadSync(
      [
        path.join(protoRoot, "cortex", "v1", "llm.proto"),
        path.join(protoRoot, "cortex", "v1", "common.proto"),
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
    const LLMService = loaded.cortex?.v1?.LLMService;
    if (!LLMService) {
      throw new Error("LLMService not found in proto definitions");
    }

    this.client = new LLMService(
      address,
      grpc.credentials.createInsecure()
    ) as LLMClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
