import * as path from "path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import { Entity, EntityID, EntityType, EntityFilters, EntityMetadata } from "../models/entity";

type EntityClient = grpc.Client & {
  GetEntity: (req: object, cb: grpc.requestCallback<any>) => void;
  ListEntities: (req: object, cb: grpc.requestCallback<any>) => void;
  GetEntitiesByFacet: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdateEntityMetadata: (req: object, cb: grpc.requestCallback<any>) => void;
  CountEntitiesByFacet: (req: object, cb: grpc.requestCallback<any>) => void;
};

export class GrpcEntityClient {
  private client: EntityClient | null = null;
  private clientAddress = "";
  private context: vscode.ExtensionContext;

  constructor(context: vscode.ExtensionContext) {
    this.context = context;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("backend.address", "localhost:50051");
  }

  private getClient(): EntityClient {
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
        path.join(protoRoot, "cortex", "v1", "entity.proto"),
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
    const EntityService = loaded.cortex?.v1?.EntityService;
    if (!EntityService) {
      throw new Error("EntityService not found in proto definitions");
    }

    this.client = new EntityService(
      address,
      grpc.credentials.createInsecure()
    ) as EntityClient;
    this.clientAddress = address;

    return this.client;
  }

  /**
   * Get a single entity by ID
   */
  async getEntity(workspaceId: string, id: EntityID): Promise<Entity | null> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.GetEntity(
        {
          workspace_id: workspaceId,
          id: {
            type: id.type,
            id: id.id,
          },
        },
        (err: grpc.ServiceError | null, response: any) => {
          if (err) {
            console.error("[GrpcEntityClient] GetEntity error:", err);
            resolve(null);
            return;
          }
          resolve(this.protoToEntity(response));
        }
      );
    });
  }

  /**
   * List entities with filters
   */
  async listEntities(workspaceId: string, filters: EntityFilters): Promise<Entity[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.ListEntities(
        {
          workspace_id: workspaceId,
          ...filters,
        },
        (err: grpc.ServiceError | null, response: any) => {
          if (err) {
            console.error("[GrpcEntityClient] ListEntities error:", err);
            resolve([]);
            return;
          }
          const entities = (response.entities || []).map((e: any) => this.protoToEntity(e));
          resolve(entities);
        }
      );
    });
  }

  /**
   * Get entities matching a facet value
   */
  async getEntitiesByFacet(
    workspaceId: string,
    facet: string,
    value: string,
    types: EntityType[] = ["file", "folder", "project"],
    timeoutMs: number = 10000
  ): Promise<Entity[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      
      let timeoutId: NodeJS.Timeout | null = null;
      let completed = false;
      
      // Set up timeout to reject if deadline is exceeded
      // Note: We can't easily cancel gRPC unary calls, but the timeout will prevent hanging
      timeoutId = setTimeout(() => {
        if (!completed) {
          completed = true;
          reject(new Error(`GetEntitiesByFacet timeout after ${timeoutMs}ms for facet=${facet}, value=${value}`));
        }
      }, timeoutMs);
      
      // Call the method - in gRPC-js, unary methods don't return the call directly
      // We'll handle cancellation via the timeout promise race instead
      client.GetEntitiesByFacet(
        {
          workspace_id: workspaceId,
          facet,
          value,
          types: types.map((t) => this.entityTypeToProto(t)),
          limit: 1000,
          offset: 0,
        },
        (err: grpc.ServiceError | null, response: any) => {
          if (completed) {
            return; // Already handled by timeout
          }
          completed = true;
          if (timeoutId) {
            clearTimeout(timeoutId);
          }
          if (err) {
            console.error("[GrpcEntityClient] GetEntitiesByFacet error:", err);
            // If it's a deadline exceeded or cancelled error, reject instead of resolving empty
            if (err.code === grpc.status.DEADLINE_EXCEEDED || err.code === grpc.status.CANCELLED) {
              reject(new Error(`GetEntitiesByFacet ${err.code === grpc.status.DEADLINE_EXCEEDED ? 'deadline exceeded' : 'cancelled'} for facet=${facet}, value=${value}`));
              return;
            }
            resolve([]);
            return;
          }
          const entities = (response.entities || []).map((e: any) => this.protoToEntity(e));
          resolve(entities);
        }
      );
    });
  }

  /**
   * Update entity metadata
   */
  async updateEntityMetadata(
    workspaceId: string,
    id: EntityID,
    metadata: EntityMetadata
  ): Promise<Entity | null> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.UpdateEntityMetadata(
        {
          workspace_id: workspaceId,
          id: {
            type: id.type,
            id: id.id,
          },
          metadata,
        },
        (err: grpc.ServiceError | null, response: any) => {
          if (err) {
            console.error("[GrpcEntityClient] UpdateEntityMetadata error:", err);
            resolve(null);
            return;
          }
          resolve(this.protoToEntity(response));
        }
      );
    });
  }

  /**
   * Count entities matching a facet value
   */
  async countEntitiesByFacet(
    workspaceId: string,
    facet: string,
    value: string,
    types: EntityType[] = ["file", "folder", "project"]
  ): Promise<number> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.CountEntitiesByFacet(
        {
          workspace_id: workspaceId,
          facet,
          value,
          types: types.map((t) => this.entityTypeToProto(t)),
        },
        (err: grpc.ServiceError | null, response: any) => {
          if (err) {
            console.error("[GrpcEntityClient] CountEntitiesByFacet error:", err);
            resolve(0);
            return;
          }
          resolve(response.count || 0);
        }
      );
    });
  }

  /**
   * Convert protobuf entity to Entity
   */
  private protoToEntity(proto: any): Entity {
    const entity: Entity = {
      id: {
        type: this.protoToEntityType(proto.type),
        id: proto.id?.id || proto.id,
      },
      type: this.protoToEntityType(proto.type),
      workspaceId: proto.workspace_id,
      name: proto.name,
      path: proto.path,
      createdAt: proto.created_at,
      updatedAt: proto.updated_at,
    };

    if (proto.description) entity.description = proto.description;
    if (proto.modified_at) entity.modifiedAt = proto.modified_at;
    if (proto.size) entity.size = proto.size;

    // Semantic metadata
    if (proto.tags) entity.tags = proto.tags;
    if (proto.projects) entity.projects = proto.projects;
    if (proto.language) entity.language = proto.language;
    if (proto.category) entity.category = proto.category;
    if (proto.author) entity.author = proto.author;
    if (proto.owner) entity.owner = proto.owner;
    if (proto.location) entity.location = proto.location;
    if (proto.publication_year) entity.publicationYear = proto.publication_year;
    if (proto.complexity) entity.complexity = proto.complexity;
    if (proto.lines_of_code) entity.linesOfCode = proto.lines_of_code;
    if (proto.quality_score) entity.qualityScore = proto.quality_score;
    if (proto.status) entity.status = proto.status;
    if (proto.priority) entity.priority = proto.priority;
    if (proto.visibility) entity.visibility = proto.visibility;
    if (proto.ai_summary) entity.aiSummary = proto.ai_summary;
    if (proto.ai_keywords) entity.aiKeywords = proto.ai_keywords;

    // Type-specific data
    if (proto.file_data) {
      try {
        entity.fileData = JSON.parse(proto.file_data);
      } catch (e) {
        console.warn("[GrpcEntityClient] Failed to parse file_data:", e);
      }
    }
    if (proto.folder_data) {
      try {
        entity.folderData = JSON.parse(proto.folder_data);
      } catch (e) {
        console.warn("[GrpcEntityClient] Failed to parse folder_data:", e);
      }
    }
    if (proto.project_data) {
      try {
        entity.projectData = JSON.parse(proto.project_data);
      } catch (e) {
        console.warn("[GrpcEntityClient] Failed to parse project_data:", e);
      }
    }

    return entity;
  }

  /**
   * Convert EntityType to protobuf enum
   */
  private entityTypeToProto(type: EntityType): number {
    switch (type) {
      case "file":
        return 1; // ENTITY_TYPE_FILE
      case "folder":
        return 2; // ENTITY_TYPE_FOLDER
      case "project":
        return 3; // ENTITY_TYPE_PROJECT
      default:
        return 0; // ENTITY_TYPE_UNSPECIFIED
    }
  }

  /**
   * Convert protobuf enum to EntityType
   */
  private protoToEntityType(type: number | string): EntityType {
    if (typeof type === "string") {
      // Handle string enum values like "ENTITY_TYPE_FILE" -> "file"
      const normalized = type.toUpperCase();
      if (normalized.includes("FILE")) {
        return "file";
      } else if (normalized.includes("FOLDER")) {
        return "folder";
      } else if (normalized.includes("PROJECT")) {
        return "project";
      }
      // If it's already a valid EntityType, return it
      if (type === "file" || type === "folder" || type === "project") {
        return type as EntityType;
      }
      // Default to file if unknown
      return "file";
    }
    switch (type) {
      case 1:
        return "file";
      case 2:
        return "folder";
      case 3:
        return "project";
      default:
        return "file";
    }
  }

  /**
   * Close the client connection
   */
  close(): void {
    if (this.client) {
      this.client.close();
      this.client = null;
    }
  }
}

