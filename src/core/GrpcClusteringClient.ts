import * as path from "node:path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type ClusteringClient = grpc.Client & {
  GetClusters: (req: object) => grpc.ClientReadableStream<any>;
  GetCluster: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetClusterMembers: (req: object) => grpc.ClientReadableStream<any>;
  GetDocumentGraph: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetDocumentClusters: (req: object) => grpc.ClientReadableStream<any>;
  RunClustering: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  MergeClusters: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  DisbandCluster: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetClusterStats: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
};

export interface DocumentCluster {
  id: string;
  workspaceId: string;
  name: string;
  summary: string;
  status: string;
  confidence: number;
  memberCount: number;
  centralNodes: string[];
  topEntities: string[];
  topKeywords: string[];
  createdAt: number;
  updatedAt: number;
  projectId?: string;
}

export interface ClusterMember {
  documentId: string;
  relativePath: string;
  filename: string;
  membershipScore: number;
  isCentral: boolean;
  addedAt: number;
}

export interface ClusterMembership {
  clusterId: string;
  clusterName: string;
  score: number;
  isCentral: boolean;
}

export interface ClusterNode {
  id: string;
  label: string;
  nodeType: string;
  clusterId?: string;
  x?: number;
  y?: number;
}

export interface ClusterEdge {
  fromId: string;
  toId: string;
  edgeType: string;
  weight: number;
  detail?: string;
}

export interface DocumentGraphData {
  nodes: ClusterNode[];
  edges: ClusterEdge[];
  totalNodes: number;
  totalEdges: number;
}

export interface ClusteringResult {
  success: boolean;
  message: string;
  clustersCreated: number;
  clustersUpdated: number;
  documentsAssigned: number;
  projectsCreated: number;
  errors: string[];
  durationMs: number;
}

export interface ClusterStats {
  totalClusters: number;
  activeClusters: number;
  documentsClustered: number;
  documentsUnclustered: number;
  avgClusterSize: number;
  avgClusterConfidence: number;
  lastClusteringAt: number;
}

export class GrpcClusteringClient {
  private client: ClusteringClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async getClusters(workspaceId: string): Promise<DocumentCluster[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetClusters({ workspace_id: workspaceId }),
      this.mapCluster
    );
  }

  async getCluster(workspaceId: string, clusterId: string): Promise<DocumentCluster | null> {
    const client = this.getClient();
    const resp = await this.unary(client.GetCluster.bind(client), {
      workspace_id: workspaceId,
      cluster_id: clusterId,
    });
    return resp ? this.mapCluster(resp) : null;
  }

  async getClusterMembers(workspaceId: string, clusterId: string): Promise<ClusterMember[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetClusterMembers({
        workspace_id: workspaceId,
        cluster_id: clusterId,
      }),
      (m: any) => ({
        documentId: m.document_id ?? "",
        relativePath: m.relative_path ?? "",
        filename: m.filename ?? "",
        membershipScore: m.membership_score ?? 0,
        isCentral: m.is_central ?? false,
        addedAt: Number(m.added_at ?? 0),
      })
    );
  }

  async getDocumentGraph(workspaceId: string, minEdgeWeight: number = 0.1): Promise<DocumentGraphData> {
    const client = this.getClient();
    const resp = await this.unary(client.GetDocumentGraph.bind(client), {
      workspace_id: workspaceId,
      min_edge_weight: minEdgeWeight,
    });
    return {
      nodes: (resp?.nodes ?? []).map((n: any) => ({
        id: n.id ?? "",
        label: n.label ?? "",
        nodeType: n.node_type ?? "document",
        clusterId: n.cluster_id,
        x: n.x,
        y: n.y,
      })),
      edges: (resp?.edges ?? []).map((e: any) => ({
        fromId: e.from_id ?? "",
        toId: e.to_id ?? "",
        edgeType: e.edge_type ?? "semantic",
        weight: e.weight ?? 0,
        detail: e.detail,
      })),
      totalNodes: resp?.total_nodes ?? 0,
      totalEdges: resp?.total_edges ?? 0,
    };
  }

  async getDocumentClusters(workspaceId: string, documentId: string): Promise<ClusterMembership[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetDocumentClusters({
        workspace_id: workspaceId,
        document_id: documentId,
      }),
      (m: any) => ({
        clusterId: m.cluster_id ?? "",
        clusterName: m.cluster_name ?? "",
        score: m.score ?? 0,
        isCentral: m.is_central ?? false,
      })
    );
  }

  async runClustering(workspaceId: string, forceRebuild: boolean = false): Promise<ClusteringResult> {
    const client = this.getClient();
    const resp = await this.unary(client.RunClustering.bind(client), {
      workspace_id: workspaceId,
      force_rebuild: forceRebuild,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
      clustersCreated: resp?.clusters_created ?? 0,
      clustersUpdated: resp?.clusters_updated ?? 0,
      documentsAssigned: resp?.documents_assigned ?? 0,
      projectsCreated: resp?.projects_created ?? 0,
      errors: resp?.errors ?? [],
      durationMs: Number(resp?.duration_ms ?? 0),
    };
  }

  async mergeClusters(workspaceId: string, targetId: string, sourceId: string): Promise<ClusteringResult> {
    const client = this.getClient();
    const resp = await this.unary(client.MergeClusters.bind(client), {
      workspace_id: workspaceId,
      target_cluster_id: targetId,
      source_cluster_id: sourceId,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
      clustersCreated: resp?.clusters_created ?? 0,
      clustersUpdated: resp?.clusters_updated ?? 0,
      documentsAssigned: resp?.documents_assigned ?? 0,
      projectsCreated: resp?.projects_created ?? 0,
      errors: resp?.errors ?? [],
      durationMs: Number(resp?.duration_ms ?? 0),
    };
  }

  async disbandCluster(workspaceId: string, clusterId: string): Promise<ClusteringResult> {
    const client = this.getClient();
    const resp = await this.unary(client.DisbandCluster.bind(client), {
      workspace_id: workspaceId,
      cluster_id: clusterId,
    });
    return {
      success: resp?.success ?? false,
      message: resp?.message ?? "",
      clustersCreated: resp?.clusters_created ?? 0,
      clustersUpdated: resp?.clusters_updated ?? 0,
      documentsAssigned: resp?.documents_assigned ?? 0,
      projectsCreated: resp?.projects_created ?? 0,
      errors: resp?.errors ?? [],
      durationMs: Number(resp?.duration_ms ?? 0),
    };
  }

  async getClusterStats(workspaceId: string): Promise<ClusterStats> {
    const client = this.getClient();
    const resp = await this.unary(client.GetClusterStats.bind(client), {
      workspace_id: workspaceId,
    });
    return {
      totalClusters: resp?.total_clusters ?? 0,
      activeClusters: resp?.active_clusters ?? 0,
      documentsClustered: resp?.documents_clustered ?? 0,
      documentsUnclustered: resp?.documents_unclustered ?? 0,
      avgClusterSize: resp?.avg_cluster_size ?? 0,
      avgClusterConfidence: resp?.avg_cluster_confidence ?? 0,
      lastClusteringAt: Number(resp?.last_clustering_at ?? 0),
    };
  }

  private mapCluster(c: any): DocumentCluster {
    return {
      id: c.id ?? "",
      workspaceId: c.workspace_id ?? "",
      name: c.name ?? "",
      summary: c.summary ?? "",
      status: c.status ?? "UNKNOWN",
      confidence: c.confidence ?? 0,
      memberCount: c.member_count ?? 0,
      centralNodes: c.central_nodes ?? [],
      topEntities: c.top_entities ?? [],
      topKeywords: c.top_keywords ?? [],
      createdAt: Number(c.created_at ?? 0),
      updatedAt: Number(c.updated_at ?? 0),
      projectId: c.project_id,
    };
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
    mapper: (item: any) => T
  ): Promise<T[]> {
    return new Promise((resolve, reject) => {
      const items: T[] = [];
      stream.on("data", (item) => {
        items.push(mapper(item));
      });
      stream.on("error", reject);
      stream.on("end", () => resolve(items));
    });
  }

  private getClient(): ClusteringClient {
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
        path.join(protoRoot, "cortex", "v1", "clustering.proto"),
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
    const ClusteringService = loaded.cortex?.v1?.ClusteringService;
    if (!ClusteringService) {
      throw new Error("ClusteringService not found in proto definitions");
    }

    this.client = new ClusteringService(
      address,
      grpc.credentials.createInsecure()
    ) as ClusteringClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
