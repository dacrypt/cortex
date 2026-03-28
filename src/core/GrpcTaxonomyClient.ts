import * as path from "node:path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type TaxonomyClient = grpc.Client & {
  GetRootNodes: (req: object) => grpc.ClientReadableStream<any>;
  GetNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetNodeByPath: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetChildren: (req: object) => grpc.ClientReadableStream<any>;
  GetAncestors: (req: object) => grpc.ClientReadableStream<any>;
  GetNodeFiles: (req: object) => grpc.ClientReadableStream<any>;
  CreateNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  UpdateNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  DeleteNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  AddFileToNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  RemoveFileFromNode: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetFileTaxonomies: (req: object) => grpc.ClientReadableStream<any>;
  SuggestTaxonomy: (req: object) => grpc.ClientReadableStream<any>;
  InduceTaxonomy: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  SearchNodes: (req: object) => grpc.ClientReadableStream<any>;
  GetStats: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
};

export type TaxonomyNodeSource = "USER" | "AI" | "SYSTEM" | "IMPORT" | "MERGED" | "UNKNOWN";
export type TaxonomyMappingSource = "MANUAL" | "AUTO" | "SUGGESTED" | "UNKNOWN";

export interface TaxonomyNode {
  id: string;
  workspaceId: string;
  name: string;
  description: string;
  parentId: string;
  path: string;
  level: number;
  source: TaxonomyNodeSource;
  confidence: number;
  keywords: string[];
  childCount: number;
  docCount: number;
  totalDocCount: number;
  createdAt: number;
  updatedAt: number;
}

export interface TaxonomyFileEntry {
  fileId: string;
  relativePath: string;
  filename: string;
  score: number;
  source: TaxonomyMappingSource;
  assignedAt: number;
}

export interface TaxonomySuggestion {
  nodeId: string;
  nodePath: string;
  nodeName: string;
  isNewNode: boolean;
  newNodeParentId: string;
  confidence: number;
  reasoning: string;
}

export interface TaxonomyInductionResult {
  success: boolean;
  message: string;
  nodesCreated: number;
  nodesMerged: number;
  filesAssigned: number;
  errors: string[];
  durationMs: number;
}

export interface TaxonomyStats {
  totalNodes: number;
  rootNodes: number;
  maxDepth: number;
  filesCategorized: number;
  filesUncategorized: number;
  avgFilesPerNode: number;
  avgCategoriesPerFile: number;
  lastInductionAt: number;
}

export class GrpcTaxonomyClient {
  private client: TaxonomyClient | null = null;
  private clientAddress = "";
  private readonly streamTimeoutMs = 10000;

  constructor(private context: vscode.ExtensionContext) {}

  async getRootNodes(workspaceId: string): Promise<TaxonomyNode[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetRootNodes({ workspace_id: workspaceId }),
      this.mapNode.bind(this)
    );
  }

  async getNode(workspaceId: string, nodeId: string): Promise<TaxonomyNode | null> {
    const client = this.getClient();
    const resp = await this.unary(client.GetNode.bind(client), {
      workspace_id: workspaceId,
      node_id: nodeId,
    });
    return resp ? this.mapNode(resp) : null;
  }

  async getNodeByPath(workspaceId: string, nodePath: string): Promise<TaxonomyNode | null> {
    const client = this.getClient();
    const resp = await this.unary(client.GetNodeByPath.bind(client), {
      workspace_id: workspaceId,
      path: nodePath,
    });
    return resp ? this.mapNode(resp) : null;
  }

  async getChildren(workspaceId: string, parentId: string): Promise<TaxonomyNode[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetChildren({
        workspace_id: workspaceId,
        parent_id: parentId,
      }),
      this.mapNode.bind(this)
    );
  }

  async getAncestors(workspaceId: string, nodeId: string): Promise<TaxonomyNode[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetAncestors({
        workspace_id: workspaceId,
        node_id: nodeId,
      }),
      this.mapNode.bind(this)
    );
  }

  async getNodeFiles(
    workspaceId: string,
    nodeId: string,
    includeDescendants: boolean = false
  ): Promise<TaxonomyFileEntry[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetNodeFiles({
        workspace_id: workspaceId,
        node_id: nodeId,
        include_descendants: includeDescendants,
      }),
      (f: any) => ({
        fileId: f.file_id ?? "",
        relativePath: f.relative_path ?? "",
        filename: f.filename ?? "",
        score: f.score ?? 0,
        source: this.mapMappingSource(f.source),
        assignedAt: Number(f.assigned_at ?? 0),
      })
    );
  }

  async createNode(
    workspaceId: string,
    name: string,
    parentId?: string,
    description?: string,
    source?: TaxonomyNodeSource,
    keywords?: string[]
  ): Promise<TaxonomyNode> {
    const client = this.getClient();
    const resp = await this.unary(client.CreateNode.bind(client), {
      workspace_id: workspaceId,
      name,
      parent_id: parentId ?? "",
      description: description ?? "",
      source: this.nodeSourceToProto(source ?? "USER"),
      keywords: keywords ?? [],
    });
    return this.mapNode(resp);
  }

  async updateNode(
    workspaceId: string,
    nodeId: string,
    name?: string,
    description?: string,
    keywords?: string[]
  ): Promise<TaxonomyNode> {
    const client = this.getClient();
    const resp = await this.unary(client.UpdateNode.bind(client), {
      workspace_id: workspaceId,
      node_id: nodeId,
      name: name ?? "",
      description: description ?? "",
      keywords: keywords ?? [],
    });
    return this.mapNode(resp);
  }

  async deleteNode(
    workspaceId: string,
    nodeId: string,
    deleteChildren: boolean = false,
    reassignFiles: boolean = true
  ): Promise<{ success: boolean; nodesDeleted: number; filesReassigned: number; message: string }> {
    const client = this.getClient();
    const resp = await this.unary(client.DeleteNode.bind(client), {
      workspace_id: workspaceId,
      node_id: nodeId,
      delete_children: deleteChildren,
      reassign_files: reassignFiles,
    });
    return {
      success: resp?.success ?? false,
      nodesDeleted: resp?.nodes_deleted ?? 0,
      filesReassigned: resp?.files_reassigned ?? 0,
      message: resp?.message ?? "",
    };
  }

  async addFileToNode(
    workspaceId: string,
    fileId: string,
    nodeId: string,
    source?: TaxonomyMappingSource,
    score?: number
  ): Promise<{ success: boolean; message: string }> {
    const client = this.getClient();
    const resp = await this.unary(client.AddFileToNode.bind(client), {
      workspace_id: workspaceId,
      file_id: fileId,
      node_id: nodeId,
      source: this.mappingSourceToProto(source ?? "MANUAL"),
      score: score ?? 1.0,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
    };
  }

  async removeFileFromNode(
    workspaceId: string,
    fileId: string,
    nodeId: string
  ): Promise<{ success: boolean; message: string }> {
    const client = this.getClient();
    const resp = await this.unary(client.RemoveFileFromNode.bind(client), {
      workspace_id: workspaceId,
      file_id: fileId,
      node_id: nodeId,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
    };
  }

  async getFileTaxonomies(workspaceId: string, fileId: string): Promise<TaxonomyNode[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetFileTaxonomies({
        workspace_id: workspaceId,
        file_id: fileId,
      }),
      this.mapNode.bind(this)
    );
  }

  async suggestTaxonomy(
    workspaceId: string,
    fileId: string,
    maxSuggestions: number = 5
  ): Promise<TaxonomySuggestion[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.SuggestTaxonomy({
        workspace_id: workspaceId,
        file_id: fileId,
        max_suggestions: maxSuggestions,
      }),
      (s: any) => ({
        nodeId: s.node_id ?? "",
        nodePath: s.node_path ?? "",
        nodeName: s.node_name ?? "",
        isNewNode: s.is_new_node ?? false,
        newNodeParentId: s.new_node_parent_id ?? "",
        confidence: s.confidence ?? 0,
        reasoning: s.reasoning ?? "",
      })
    );
  }

  async induceTaxonomy(
    workspaceId: string,
    options?: {
      includeFiles?: boolean;
      includeProjects?: boolean;
      includeClusters?: boolean;
      maxLevels?: number;
      maxNodesPerLevel?: number;
    }
  ): Promise<TaxonomyInductionResult> {
    const client = this.getClient();
    const resp = await this.unary(client.InduceTaxonomy.bind(client), {
      workspace_id: workspaceId,
      include_files: options?.includeFiles ?? true,
      include_projects: options?.includeProjects ?? true,
      include_clusters: options?.includeClusters ?? true,
      max_levels: options?.maxLevels ?? 3,
      max_nodes_per_level: options?.maxNodesPerLevel ?? 10,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
      nodesCreated: resp?.nodes_created ?? 0,
      nodesMerged: resp?.nodes_merged ?? 0,
      filesAssigned: resp?.files_assigned ?? 0,
      errors: resp?.errors ?? [],
      durationMs: Number(resp?.duration_ms ?? 0),
    };
  }

  async searchNodes(workspaceId: string, query: string, limit: number = 20): Promise<TaxonomyNode[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.SearchNodes({
        workspace_id: workspaceId,
        query,
        limit,
      }),
      this.mapNode.bind(this)
    );
  }

  async getStats(workspaceId: string): Promise<TaxonomyStats> {
    const client = this.getClient();
    const resp = await this.unary(client.GetStats.bind(client), {
      workspace_id: workspaceId,
    });
    return {
      totalNodes: resp?.total_nodes ?? 0,
      rootNodes: resp?.root_nodes ?? 0,
      maxDepth: resp?.max_depth ?? 0,
      filesCategorized: resp?.files_categorized ?? 0,
      filesUncategorized: resp?.files_uncategorized ?? 0,
      avgFilesPerNode: resp?.avg_files_per_node ?? 0,
      avgCategoriesPerFile: resp?.avg_categories_per_file ?? 0,
      lastInductionAt: Number(resp?.last_induction_at ?? 0),
    };
  }

  private mapNode(n: any): TaxonomyNode {
    return {
      id: n.id ?? "",
      workspaceId: n.workspace_id ?? "",
      name: n.name ?? "",
      description: n.description ?? "",
      parentId: n.parent_id ?? "",
      path: n.path ?? "",
      level: n.level ?? 0,
      source: this.mapNodeSource(n.source),
      confidence: n.confidence ?? 0,
      keywords: n.keywords ?? [],
      childCount: n.child_count ?? 0,
      docCount: n.doc_count ?? 0,
      totalDocCount: n.total_doc_count ?? 0,
      createdAt: Number(n.created_at ?? 0),
      updatedAt: Number(n.updated_at ?? 0),
    };
  }

  private mapNodeSource(source: any): TaxonomyNodeSource {
    if (typeof source === "string") {
      if (source.includes("USER")) return "USER";
      if (source.includes("AI")) return "AI";
      if (source.includes("SYSTEM")) return "SYSTEM";
      if (source.includes("IMPORT")) return "IMPORT";
      if (source.includes("MERGED")) return "MERGED";
    }
    return "UNKNOWN";
  }

  private mapMappingSource(source: any): TaxonomyMappingSource {
    if (typeof source === "string") {
      if (source.includes("MANUAL")) return "MANUAL";
      if (source.includes("AUTO")) return "AUTO";
      if (source.includes("SUGGESTED")) return "SUGGESTED";
    }
    return "UNKNOWN";
  }

  private nodeSourceToProto(source: TaxonomyNodeSource): number {
    switch (source) {
      case "USER": return 1;
      case "AI": return 2;
      case "SYSTEM": return 3;
      case "IMPORT": return 4;
      case "MERGED": return 5;
      default: return 0;
    }
  }

  private mappingSourceToProto(source: TaxonomyMappingSource): number {
    switch (source) {
      case "MANUAL": return 1;
      case "AUTO": return 2;
      case "SUGGESTED": return 3;
      default: return 0;
    }
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

  private streamToArray<T>(
    stream: grpc.ClientReadableStream<any>,
    mapper: (item: any) => T,
    timeoutMs: number = this.streamTimeoutMs
  ): Promise<T[]> {
    return new Promise((resolve, reject) => {
      const items: T[] = [];
      let settled = false;
      const timer = setTimeout(() => {
        if (settled) {
          return;
        }
        settled = true;
        const timeoutError = new Error(`Taxonomy request timed out after ${timeoutMs}ms`);
        (timeoutError as { code?: number }).code = 4;
        try {
          stream.cancel();
        } catch {
          // Ignore stream cancellation errors
        }
        reject(timeoutError);
      }, timeoutMs);

      const cleanup = () => {
        if (settled) {
          return;
        }
        settled = true;
        clearTimeout(timer);
      };

      stream.on("data", (item) => {
        items.push(mapper(item));
      });
      stream.on("error", (error) => {
        cleanup();
        reject(error);
      });
      stream.on("end", () => {
        cleanup();
        resolve(items);
      });
    });
  }

  private getClient(): TaxonomyClient {
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
        path.join(protoRoot, "cortex", "v1", "taxonomy.proto"),
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
    const TaxonomyService = loaded.cortex?.v1?.TaxonomyService;
    if (!TaxonomyService) {
      throw new Error("TaxonomyService not found in proto definitions");
    }

    this.client = new TaxonomyService(
      address,
      grpc.credentials.createInsecure()
    ) as TaxonomyClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
