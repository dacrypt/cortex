import * as path from "node:path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type RAGClient = grpc.Client & {
  Query: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  SemanticSearch: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetIndexStats: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
};

export interface RAGSource {
  documentId: string;
  chunkId: string;
  relativePath: string;
  headingPath: string;
  snippet: string;
  score: number;
}

export interface RAGQueryResponse {
  answer: string;
  sources: RAGSource[];
}

export class GrpcRAGClient {
  private client: RAGClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async query(workspaceId: string, query: string, topK: number = 5): Promise<RAGQueryResponse> {
    const client = this.getClient();
    const resp = await this.unary(client.Query.bind(client), {
      workspace_id: workspaceId,
      query: query,
      top_k: topK,
    });
    return {
      answer: resp?.answer ?? "",
      sources: resp?.sources ?? [],
    };
  }

  async semanticSearch(workspaceId: string, query: string, topK: number = 5): Promise<RAGSource[]> {
    const client = this.getClient();
    const resp = await this.unary(client.SemanticSearch.bind(client), {
      workspace_id: workspaceId,
      query: query,
      top_k: topK,
    });
    return resp?.results ?? [];
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

  private getClient(): RAGClient {
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
        path.join(protoRoot, "cortex", "v1", "rag.proto"),
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
    const RAGService = loaded.cortex?.v1?.RAGService;
    if (!RAGService) {
      throw new Error("RAGService not found in proto definitions");
    }

    this.client = new RAGService(
      address,
      grpc.credentials.createInsecure()
    ) as RAGClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
