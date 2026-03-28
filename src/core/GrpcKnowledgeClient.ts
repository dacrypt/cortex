import * as path from "path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type KnowledgeClient = grpc.Client & {
  ListProjects: (req: object) => grpc.ClientReadableStream<any>;
  GetProject: (req: object, cb: grpc.requestCallback<any>) => void;
  CreateProject: (req: object, cb: grpc.requestCallback<any>) => void;
  UpdateProject: (req: object, cb: grpc.requestCallback<any>) => void;
  GetProjectChildren: (req: object) => grpc.ClientReadableStream<any>;
  GetProjectsForDocument: (req: object) => grpc.ClientReadableStream<any>;
  QueryDocuments: (req: object) => grpc.ClientReadableStream<any>;
  AddDocumentToProject: (req: object, cb: grpc.requestCallback<any>) => void;
  RemoveDocumentFromProject: (req: object, cb: grpc.requestCallback<any>) => void;
  GetFacets: (req: object, cb: grpc.requestCallback<any>) => void;
  ListProjectAssignmentsByFile: (req: object) => grpc.ClientReadableStream<any>;
  UpdateProjectAssignmentStatus: (req: object, cb: grpc.requestCallback<any>) => void;
  // Poly-hierarchy methods
  GetAllProjectMembershipsForDocument: (req: object) => grpc.ClientReadableStream<any>;
  UpdateDocumentProjectRole: (req: object, cb: grpc.requestCallback<any>) => void;
};

export interface DocumentInfo {
  id: string;
  path: string;
  title?: string;
}

export interface Project {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  nature?: string;  // Project nature/type
  attributes?: string;  // JSON string with project attributes
  parent_id?: string;
  created_at: number;
  updated_at: number;
}

export interface ProjectAssignment {
  workspace_id: string;
  file_id: string;
  project_id?: string;
  project_name: string;
  score: number;
  sources: string[];
  status: string;
  created_at: number;
  updated_at: number;
}

export type DocumentProjectRole = 'primary' | 'related' | 'archive' | 'unspecified';

export interface DocumentProjectMembership {
  workspace_id: string;
  project_id: string;
  project_name: string;
  document_id: string;
  role: DocumentProjectRole;
  score: number;
  added_at: number;
}

export class GrpcKnowledgeClient {
  private client: KnowledgeClient | null = null;
  private clientAddress = "";
  private context: vscode.ExtensionContext;

  constructor(context: vscode.ExtensionContext) {
    this.context = context;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("backend.address", "localhost:50051");
  }

  private getClient(): KnowledgeClient {
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
        path.join(protoRoot, "cortex", "v1", "knowledge.proto"),
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
    const KnowledgeService = loaded.cortex?.v1?.KnowledgeService;
    if (!KnowledgeService) {
      throw new Error("KnowledgeService not found in proto definitions");
    }

    this.client = new KnowledgeService(
      address,
      grpc.credentials.createInsecure()
    ) as KnowledgeClient;
    this.clientAddress = address;
    return this.client;
  }

  async listProjects(workspaceId: string, parentId?: string): Promise<Project[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = { workspace_id: workspaceId };
      if (parentId) {
        req.parent_id = parentId;
      }

      console.log('[GrpcKnowledgeClient] Calling ListProjects:', { workspaceId, parentId });
      const stream = client.ListProjects(req);
      const projects: Project[] = [];

      stream.on("data", (project: Project) => {
        console.log('[GrpcKnowledgeClient] Received project:', project.name, project.id);
        projects.push(project);
      });

      stream.on("end", () => {
        console.log('[GrpcKnowledgeClient] ListProjects stream ended, total projects:', projects.length);
        resolve(projects);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        console.error('[GrpcKnowledgeClient] ListProjects error:', err);
        reject(err);
      });
    });
  }

  async listProjectAssignmentsByFile(workspaceId: string, fileId: string): Promise<ProjectAssignment[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = { workspace_id: workspaceId, file_id: fileId };
      const stream = client.ListProjectAssignmentsByFile(req);
      const assignments: ProjectAssignment[] = [];

      stream.on("data", (assignment: ProjectAssignment) => {
        assignments.push(assignment);
      });

      stream.on("end", () => {
        resolve(assignments);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        reject(err);
      });
    });
  }

  async updateProjectAssignmentStatus(
    workspaceId: string,
    fileId: string,
    projectName: string,
    status: string
  ): Promise<ProjectAssignment> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = {
        workspace_id: workspaceId,
        file_id: fileId,
        project_name: projectName,
        status,
      };
      client.UpdateProjectAssignmentStatus(req, (err, resp) => {
        if (err) {
          reject(err);
          return;
        }
        resolve(resp as ProjectAssignment);
      });
    });
  }

  async getProject(workspaceId: string, projectId: string): Promise<Project | null> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.GetProject(
        { workspace_id: workspaceId, project_id: projectId },
        (err: grpc.ServiceError | null, resp: Project) => {
          if (err) {
            reject(err);
            return;
          }
          resolve(resp);
        }
      );
    });
  }

  async getProjectChildren(workspaceId: string, projectId: string): Promise<Project[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const stream = client.GetProjectChildren({
        workspace_id: workspaceId,
        project_id: projectId,
      });

      const projects: Project[] = [];

      stream.on("data", (project: Project) => {
        projects.push(project);
      });

      stream.on("end", () => {
        resolve(projects);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        reject(err);
      });
    });
  }

  async getProjectsForDocument(workspaceId: string, documentId: string): Promise<string[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      
      // Check if the method exists (graceful fallback if proto not regenerated yet)
      if (!client.GetProjectsForDocument || typeof client.GetProjectsForDocument !== 'function') {
        console.warn('[GrpcKnowledgeClient] GetProjectsForDocument not available in proto, returning empty array. Regenerate proto files.');
        resolve([]);
        return;
      }

      const stream = client.GetProjectsForDocument({
        workspace_id: workspaceId,
        document_id: documentId,
      });

      const projectIds: string[] = [];

      stream.on("data", (data: any) => {
        // The stream returns DocumentID messages with an 'id' field
        const projectId = typeof data === 'string' ? data : (data.id || data);
        projectIds.push(projectId);
      });

      stream.on("end", () => {
        resolve(projectIds);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        reject(err);
      });
    });
  }

  async queryDocuments(
    workspaceId: string,
    projectId?: string,
    includeSubprojects: boolean = false
  ): Promise<DocumentInfo[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = {
        workspace_id: workspaceId,
        filters: [],
      };

      if (projectId) {
        req.filters = [
          {
            project: {
              project_id: projectId,
              include_subprojects: includeSubprojects,
            },
          },
        ];
      }

      const stream = client.QueryDocuments(req);
      const documents: DocumentInfo[] = [];

      stream.on("data", (docInfo: DocumentInfo) => {
        documents.push(docInfo);
      });

      stream.on("end", () => {
        resolve(documents);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        reject(err);
      });
    });
  }

  async createProject(
    workspaceId: string,
    name: string,
    description?: string,
    parentId?: string
  ): Promise<Project> {
    return this.createProjectWithNature(workspaceId, name, description, parentId, 'generic');
  }

  async createProjectWithNature(
    workspaceId: string,
    name: string,
    description?: string,
    parentId?: string,
    nature: string = 'generic',
    attributes?: string
  ): Promise<Project> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = {
        workspace_id: workspaceId,
        name: name,
        nature: nature,
      };
      if (description) {
        req.description = description;
      }
      if (parentId) {
        req.parent_id = parentId;
      }
      if (attributes) {
        req.attributes = attributes;
      }

      client.CreateProject(req, (err: grpc.ServiceError | null, resp: Project) => {
        if (err) {
          reject(err);
          return;
        }
        resolve(resp);
      });
    });
  }

  async updateProject(
    workspaceId: string,
    projectId: string,
    updates: {
      name?: string;
      description?: string;
      nature?: string;
      attributes?: string;
      parent_id?: string;
    }
  ): Promise<Project> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const req: any = {
        workspace_id: workspaceId,
        project_id: projectId,
      };
      if (updates.name !== undefined) {
        req.name = updates.name;
      }
      if (updates.description !== undefined) {
        req.description = updates.description;
      }
      if (updates.nature !== undefined) {
        req.nature = updates.nature;
      }
      if (updates.attributes !== undefined) {
        req.attributes = updates.attributes;
      }
      if (updates.parent_id !== undefined) {
        req.parent_id = updates.parent_id;
      }

      client.UpdateProject(req, (err: grpc.ServiceError | null, resp: Project) => {
        if (err) {
          reject(err);
          return;
        }
        resolve(resp);
      });
    });
  }

  async getProjectByName(
    workspaceId: string,
    name: string,
    parentId?: string
  ): Promise<Project | null> {
    // List all projects and find by name
    const projects = await this.listProjects(workspaceId, parentId);
    return projects.find(p => p.name === name) || null;
  }

  async getDocumentIdByPath(
    workspaceId: string,
    relativePath: string,
    adminClient: any
  ): Promise<string | null> {
    try {
      // Use admin client to get file, then extract document ID
      const file = await adminClient.getFile(workspaceId, relativePath);
      if (!file || !file.file_id) {
        return null;
      }
      // Convert file_id to document_id using the same hash function
      // DocumentID is SHA256("doc:" + normalized_path)
      const crypto = require('crypto');
      const normalized = relativePath.replace(/\\/g, '/');
      const hash = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');
      return hash;
    } catch (error) {
      console.error(`[GrpcKnowledgeClient] Failed to get document ID for ${relativePath}:`, error);
      return null;
    }
  }

  async addDocumentToProject(
    workspaceId: string,
    projectId: string,
    documentId: string,
    role: DocumentProjectRole = 'primary'
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      const protoRole = this.roleToProto(role);
      client.AddDocumentToProject(
        {
          workspace_id: workspaceId,
          project_id: projectId,
          document_id: documentId,
          role: protoRole,
        },
        (err: grpc.ServiceError | null, resp: any) => {
          if (err) {
            reject(err);
            return;
          }
          if (!resp || !resp.success) {
            reject(new Error('Failed to add document to project'));
            return;
          }
          resolve();
        }
      );
    });
  }

  async removeDocumentFromProject(
    workspaceId: string,
    projectId: string,
    documentId: string
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      client.RemoveDocumentFromProject(
        {
          workspace_id: workspaceId,
          project_id: projectId,
          document_id: documentId,
        },
        (err: grpc.ServiceError | null, resp: any) => {
          if (err) {
            reject(err);
            return;
          }
          if (!resp || !resp.success) {
            reject(new Error('Failed to remove document from project'));
            return;
          }
          resolve();
        }
      );
    });
  }

  async getFacets(
    workspaceId: string,
    facets: Array<{ field: string; type: string }>,
    filters?: any[],
    timeoutMs: number = 10000
  ): Promise<any> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();
      
      // Check if the method exists
      if (!client.GetFacets || typeof client.GetFacets !== 'function') {
        console.warn('[GrpcKnowledgeClient] GetFacets not available in proto, returning empty. Regenerate proto files.');
        resolve({ results: [] });
        return;
      }

      let timeoutId: NodeJS.Timeout | null = null;
      let completed = false;
      
      // Set up timeout to reject if deadline is exceeded
      // Note: We can't easily cancel gRPC unary calls, but the timeout will prevent hanging
      timeoutId = setTimeout(() => {
        if (!completed) {
          completed = true;
          reject(new Error(`GetFacets timeout after ${timeoutMs}ms`));
        }
      }, timeoutMs);

      const req: any = {
        workspace_id: workspaceId,
        facets: facets.map(f => ({
          field: f.field,
          type: f.type === 'terms' ? 'FACET_TYPE_TERMS' :
                f.type === 'numeric_range' ? 'FACET_TYPE_NUMERIC_RANGE' :
                f.type === 'date_range' ? 'FACET_TYPE_DATE_RANGE' :
                'FACET_TYPE_UNSPECIFIED'
        }))
      };

      if (filters && filters.length > 0) {
        req.filters = filters;
      }

      client.GetFacets(req, (err: grpc.ServiceError | null, resp: any) => {
        if (completed) {
          return; // Already handled by timeout
        }
        completed = true;
        if (timeoutId) {
          clearTimeout(timeoutId);
        }
        if (err) {
          // If it's a deadline exceeded or cancelled error, provide clearer message
          if (err.code === grpc.status.DEADLINE_EXCEEDED || err.code === grpc.status.CANCELLED) {
            reject(new Error(`GetFacets ${err.code === grpc.status.DEADLINE_EXCEEDED ? 'deadline exceeded' : 'cancelled'} after ${timeoutMs}ms`));
            return;
          }
          reject(err);
          return;
        }
        resolve(resp);
      });
    });
  }

  // Poly-hierarchy methods

  async getAllProjectMembershipsForDocument(
    workspaceId: string,
    documentId: string
  ): Promise<DocumentProjectMembership[]> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();

      // Check if the method exists (graceful fallback)
      if (!client.GetAllProjectMembershipsForDocument || typeof client.GetAllProjectMembershipsForDocument !== 'function') {
        console.warn('[GrpcKnowledgeClient] GetAllProjectMembershipsForDocument not available. Regenerate proto files.');
        resolve([]);
        return;
      }

      const stream = client.GetAllProjectMembershipsForDocument({
        workspace_id: workspaceId,
        document_id: documentId,
      });

      const memberships: DocumentProjectMembership[] = [];

      stream.on("data", (data: any) => {
        memberships.push(this.parseDocumentProjectMembership(data));
      });

      stream.on("end", () => {
        resolve(memberships);
      });

      stream.on("error", (err: grpc.ServiceError) => {
        reject(err);
      });
    });
  }

  async updateDocumentProjectRole(
    workspaceId: string,
    projectId: string,
    documentId: string,
    role: DocumentProjectRole
  ): Promise<DocumentProjectMembership> {
    return new Promise((resolve, reject) => {
      const client = this.getClient();

      // Check if the method exists
      if (!client.UpdateDocumentProjectRole || typeof client.UpdateDocumentProjectRole !== 'function') {
        reject(new Error('UpdateDocumentProjectRole not available. Regenerate proto files.'));
        return;
      }

      const protoRole = this.roleToProto(role);

      client.UpdateDocumentProjectRole(
        {
          workspace_id: workspaceId,
          project_id: projectId,
          document_id: documentId,
          role: protoRole,
        },
        (err: grpc.ServiceError | null, resp: any) => {
          if (err) {
            reject(err);
            return;
          }
          resolve(this.parseDocumentProjectMembership(resp));
        }
      );
    });
  }

  private parseDocumentProjectMembership(data: any): DocumentProjectMembership {
    return {
      workspace_id: data.workspace_id || '',
      project_id: data.project_id || '',
      project_name: data.project_name || '',
      document_id: data.document_id || '',
      role: this.protoToRole(data.role),
      score: data.score || 0,
      added_at: data.added_at || 0,
    };
  }

  private roleToProto(role: DocumentProjectRole): string {
    switch (role) {
      case 'primary':
        return 'DOCUMENT_PROJECT_ROLE_PRIMARY';
      case 'related':
        return 'DOCUMENT_PROJECT_ROLE_RELATED';
      case 'archive':
        return 'DOCUMENT_PROJECT_ROLE_ARCHIVE';
      default:
        return 'DOCUMENT_PROJECT_ROLE_UNSPECIFIED';
    }
  }

  private protoToRole(protoRole: string | number): DocumentProjectRole {
    if (typeof protoRole === 'number') {
      switch (protoRole) {
        case 1: return 'primary';
        case 2: return 'related';
        case 3: return 'archive';
        default: return 'unspecified';
      }
    }
    switch (protoRole) {
      case 'DOCUMENT_PROJECT_ROLE_PRIMARY':
        return 'primary';
      case 'DOCUMENT_PROJECT_ROLE_RELATED':
        return 'related';
      case 'DOCUMENT_PROJECT_ROLE_ARCHIVE':
        return 'archive';
      default:
        return 'unspecified';
    }
  }
}
