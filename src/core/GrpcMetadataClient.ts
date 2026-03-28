import * as path from "path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type MetadataClient = grpc.Client & {
  AddTag: (req: object, cb: grpc.requestCallback<any>) => void;
  RemoveTag: (req: object, cb: grpc.requestCallback<any>) => void;
  ListByTag: (req: object) => grpc.ClientReadableStream<any>;
  GetAllTags: (req: object, cb: grpc.requestCallback<any>) => void;
  GetTagCounts: (req: object, cb: grpc.requestCallback<any>) => void;
  AddContext: (req: object, cb: grpc.requestCallback<any>) => void;
  RemoveContext: (req: object, cb: grpc.requestCallback<any>) => void;
  ListByContext: (req: object) => grpc.ClientReadableStream<any>;
  GetAllContexts: (req: object, cb: grpc.requestCallback<any>) => void;
  AddSuggestedContext: (req: object, cb: grpc.requestCallback<any>) => void;
  AcceptSuggestion: (req: object, cb: grpc.requestCallback<any>) => void;
  DismissSuggestion: (req: object, cb: grpc.requestCallback<any>) => void;
  GetSuggestions: (req: object, cb: grpc.requestCallback<any>) => void;
  GetSuggestedMetadata: (req: object, cb: grpc.requestCallback<any>) => void;
  GetMetadata: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdateNotes: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdateAISummary: (req: object, cb: grpc.requestCallback<any>) => void;
  ListProcessingTraces: (req: object, cb: grpc.requestCallback<any>) => void;
};

export class GrpcMetadataClient {
  private client: MetadataClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async getAllTags(workspaceId: string): Promise<string[]> {
    const client = this.getClient();
    const resp = await this.unary(client.GetAllTags.bind(client), {
      workspace_id: workspaceId,
    });
    return resp?.tags ?? [];
  }

  async getTagCounts(workspaceId: string): Promise<Map<string, number>> {
    const client = this.getClient();
    const resp = await this.unary(client.GetTagCounts.bind(client), {
      workspace_id: workspaceId,
    });
    const entries = Object.entries(resp?.counts ?? {});
    return new Map(entries.map(([key, value]) => [key, Number(value)]));
  }

  async listByTag(workspaceId: string, tag: string): Promise<any[]> {
    const client = this.getClient();
    return this.streamToArray(client.ListByTag({ workspace_id: workspaceId, tag }));
  }

  async getAllContexts(workspaceId: string): Promise<string[]> {
    const client = this.getClient();
    const resp = await this.unary(client.GetAllContexts.bind(client), {
      workspace_id: workspaceId,
    });
    return resp?.contexts ?? [];
  }

  async listByContext(workspaceId: string, context: string): Promise<any[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.ListByContext({ workspace_id: workspaceId, context })
    );
  }

  async getSuggestions(workspaceId: string): Promise<any[]> {
    const client = this.getClient();
    const resp = await this.unary(client.GetSuggestions.bind(client), {
      workspace_id: workspaceId,
    });
    return resp?.suggestions ?? [];
  }

  /**
   * Get suggested metadata for a specific file
   */
  async getSuggestedMetadata(
    workspaceId: string,
    relativePath: string
  ): Promise<any | null> {
    const client = this.getClient();
    try {
      const resp = await this.unary(client.GetSuggestedMetadata.bind(client), {
        workspace_id: workspaceId,
        relative_path: relativePath,
      });
      return resp || null;
    } catch (error) {
      console.warn(`[GrpcMetadataClient] Failed to get suggested metadata: ${error}`);
      return null;
    }
  }

  /**
   * Accept a suggested tag
   */
  async acceptSuggestedTag(
    workspaceId: string,
    fileId: string,
    tag: string
  ): Promise<void> {
    const client = this.getClient();
    await this.unary(client.AcceptSuggestion?.bind(client) || client.AddTag.bind(client), {
      workspace_id: workspaceId,
      file_id: fileId,
      tag: tag,
      suggestion_type: 'tag',
    });
  }

  /**
   * Accept a suggested project
   */
  async acceptSuggestedProject(
    workspaceId: string,
    fileId: string,
    projectName: string
  ): Promise<void> {
    const client = this.getClient();
    await this.unary(client.AcceptSuggestion?.bind(client) || client.AddContext.bind(client), {
      workspace_id: workspaceId,
      file_id: fileId,
      project_name: projectName,
      suggestion_type: 'project',
    });
  }

  /**
   * Reject a suggested tag
   */
  async rejectSuggestedTag(
    workspaceId: string,
    fileId: string,
    tag: string
  ): Promise<void> {
    const client = this.getClient();
    await this.unary(client.DismissSuggestion?.bind(client) || (() => {}), {
      workspace_id: workspaceId,
      file_id: fileId,
      tag: tag,
      suggestion_type: 'tag',
    });
  }

  /**
   * Reject a suggested project
   */
  async rejectSuggestedProject(
    workspaceId: string,
    fileId: string,
    projectName: string
  ): Promise<void> {
    const client = this.getClient();
    await this.unary(client.DismissSuggestion?.bind(client) || (() => {}), {
      workspace_id: workspaceId,
      file_id: fileId,
      project_name: projectName,
      suggestion_type: 'project',
    });
  }

  async getMetadataByPath(
    workspaceId: string,
    relativePath: string
  ): Promise<any> {
    const client = this.getClient();
    try {
      return await this.unary(client.GetMetadata.bind(client), {
        workspace_id: workspaceId,
        relative_path: relativePath,
      });
    } catch (error: any) {
      // NOT_FOUND (code 5) is a valid case - file may not have metadata yet
      // Return null instead of throwing
      if (error?.code === 5 || error?.message?.includes('NOT_FOUND') || error?.message?.includes('metadata not found')) {
        return null;
      }
      // Re-throw other errors
      throw error;
    }
  }

  async listProcessingTraces(
    workspaceId: string,
    relativePath?: string,
    limit = 100
  ): Promise<any[]> {
    const client = this.getClient();
    const resp = await this.unary(client.ListProcessingTraces.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      limit,
    });
    return resp?.traces ?? [];
  }

  async addTag(
    workspaceId: string,
    relativePath: string,
    tag: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.AddTag.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      tag,
    });
  }

  async removeTag(
    workspaceId: string,
    relativePath: string,
    tag: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.RemoveTag.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      tag,
    });
  }

  async addContext(
    workspaceId: string,
    relativePath: string,
    contextName: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.AddContext.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      context: contextName,
    });
  }

  async removeContext(
    workspaceId: string,
    relativePath: string,
    contextName: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.RemoveContext.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      context: contextName,
    });
  }

  async addSuggestedContext(
    workspaceId: string,
    relativePath: string,
    contextName: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.AddSuggestedContext.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      context: contextName,
    });
  }

  async acceptSuggestion(
    workspaceId: string,
    relativePath: string,
    contextName: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.AcceptSuggestion.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      context: contextName,
    });
  }

  async dismissSuggestion(
    workspaceId: string,
    relativePath: string,
    contextName: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.DismissSuggestion.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      context: contextName,
    });
  }

  async updateNotes(
    workspaceId: string,
    relativePath: string,
    notes: string
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.UpdateNotes.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      notes,
    });
  }

  async updateAISummary(
    workspaceId: string,
    relativePath: string,
    summary: string,
    contentHash: string,
    keyTerms: string[] = []
  ): Promise<any> {
    const client = this.getClient();
    return this.unary(client.UpdateAISummary.bind(client), {
      workspace_id: workspaceId,
      relative_path: relativePath,
      summary,
      content_hash: contentHash,
      key_terms: keyTerms,
    });
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

  private streamToArray(stream: grpc.ClientReadableStream<any>): Promise<any[]> {
    return new Promise((resolve, reject) => {
      const items: any[] = [];
      stream.on("data", (item: any) => items.push(item));
      stream.on("error", (err: Error) => reject(err));
      stream.on("end", () => resolve(items));
    });
  }

  private getClient(): MetadataClient {
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
        path.join(protoRoot, "cortex", "v1", "metadata.proto"),
        path.join(protoRoot, "cortex", "v1", "file.proto"),
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
    const MetadataService = loaded.cortex?.v1?.MetadataService;
    if (!MetadataService) {
      throw new Error("MetadataService not found in proto definitions");
    }

    this.client = new MetadataService(
      address,
      grpc.credentials.createInsecure()
    ) as MetadataClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
