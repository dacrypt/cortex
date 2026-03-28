import * as path from "node:path";
import * as vscode from "vscode";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

type PreferencesClient = grpc.Client & {
  RecordFeedback: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetPreferences: (req: object) => grpc.ClientReadableStream<any>;
  GetPreference: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  ApplyPreferences: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetFeedbackStats: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  GetFeedbackHistory: (req: object) => grpc.ClientReadableStream<any>;
  ConsolidatePatterns: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
  Cleanup: (req: object, cb: (err: grpc.ServiceError | null, resp: any) => void) => void;
};

export type FeedbackAction = "ACCEPTED" | "REJECTED" | "CORRECTED" | "IGNORED" | "UNKNOWN";
export type SuggestionType = "TAG" | "PROJECT" | "CATEGORY" | "CLUSTER" | "RELATIONSHIP" | "UNKNOWN";
export type PreferenceType = "TAGGING" | "CATEGORIZATION" | "CLUSTERING" | "NAMING" | "UNKNOWN";

export interface AISuggestion {
  type: SuggestionType;
  value: string;
  confidence: number;
  reasoning: string;
  source: string;
}

export interface Correction {
  correctedValue: string;
  correctionReason: string;
}

export interface FeedbackContext {
  fileId: string;
  fileType: string;
  folderPath: string;
  existingTags: string[];
  existingProjects: string[];
  metadata?: Record<string, string>;
}

export interface UserFeedback {
  id: string;
  workspaceId: string;
  action: FeedbackAction;
  suggestion: AISuggestion;
  correction?: Correction;
  context: FeedbackContext;
  responseTimeMs: number;
  createdAt: number;
}

export interface PreferencePattern {
  fileExtensions: string[];
  folderPatterns: string[];
  keywords: string[];
  metadataPatterns?: Record<string, string>;
}

export interface PreferenceBehavior {
  confidenceBoost: number;
  preferredTags: string[];
  avoidedTags: string[];
  preferredProjects: string[];
  avoidedProjects: string[];
  namingRules?: Record<string, string>;
}

export interface LearnedPreference {
  id: string;
  workspaceId: string;
  type: PreferenceType;
  pattern: PreferencePattern;
  behavior: PreferenceBehavior;
  confidence: number;
  exampleCount: number;
  createdAt: number;
  updatedAt: number;
  lastUsedAt: number;
}

export interface FeedbackStats {
  totalFeedback: number;
  accepted: number;
  rejected: number;
  corrected: number;
  ignored: number;
  preferencesLearned: number;
  acceptanceRate: number;
  avgConfidenceBoost: number;
  avgResponseTimeMs: number;
}

export interface RecordFeedbackResponse {
  success: boolean;
  feedbackId: string;
  preferenceUpdated: boolean;
  preferenceId: string;
  message: string;
}

export interface ApplyPreferencesResponse {
  modifiedSuggestion: AISuggestion;
  confidenceDelta: number;
  appliedPreferences: string[];
  explanation: string;
}

export class GrpcPreferencesClient {
  private client: PreferencesClient | null = null;
  private clientAddress = "";

  constructor(private context: vscode.ExtensionContext) {}

  async recordFeedback(
    workspaceId: string,
    action: FeedbackAction,
    suggestion: AISuggestion,
    options?: {
      correction?: Correction;
      context?: FeedbackContext;
      responseTimeMs?: number;
    }
  ): Promise<RecordFeedbackResponse> {
    const client = this.getClient();
    const resp = await this.unary(client.RecordFeedback.bind(client), {
      workspace_id: workspaceId,
      action: this.feedbackActionToProto(action),
      suggestion: {
        type: this.suggestionTypeToProto(suggestion.type),
        value: suggestion.value,
        confidence: suggestion.confidence,
        reasoning: suggestion.reasoning,
        source: suggestion.source,
      },
      correction: options?.correction ? {
        corrected_value: options.correction.correctedValue,
        correction_reason: options.correction.correctionReason,
      } : undefined,
      context: options?.context ? {
        file_id: options.context.fileId,
        file_type: options.context.fileType,
        folder_path: options.context.folderPath,
        existing_tags: options.context.existingTags,
        existing_projects: options.context.existingProjects,
        metadata: options.context.metadata,
      } : undefined,
      response_time_ms: options?.responseTimeMs ?? 0,
    });
    return {
      success: resp?.success ?? false,
      feedbackId: resp?.feedback_id ?? "",
      preferenceUpdated: resp?.preference_updated ?? false,
      preferenceId: resp?.preference_id ?? "",
      message: resp?.message ?? "",
    };
  }

  async getPreferences(
    workspaceId: string,
    options?: {
      typeFilter?: PreferenceType;
      minConfidence?: number;
      limit?: number;
      offset?: number;
    }
  ): Promise<LearnedPreference[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetPreferences({
        workspace_id: workspaceId,
        type_filter: options?.typeFilter ? this.preferenceTypeToProto(options.typeFilter) : 0,
        min_confidence: options?.minConfidence ?? 0,
        limit: options?.limit ?? 100,
        offset: options?.offset ?? 0,
      }),
      this.mapPreference.bind(this)
    );
  }

  async getPreference(workspaceId: string, preferenceId: string): Promise<LearnedPreference | null> {
    const client = this.getClient();
    const resp = await this.unary(client.GetPreference.bind(client), {
      workspace_id: workspaceId,
      preference_id: preferenceId,
    });
    return resp ? this.mapPreference(resp) : null;
  }

  async applyPreferences(
    workspaceId: string,
    suggestion: AISuggestion,
    context?: FeedbackContext
  ): Promise<ApplyPreferencesResponse> {
    const client = this.getClient();
    const resp = await this.unary(client.ApplyPreferences.bind(client), {
      workspace_id: workspaceId,
      suggestion: {
        type: this.suggestionTypeToProto(suggestion.type),
        value: suggestion.value,
        confidence: suggestion.confidence,
        reasoning: suggestion.reasoning,
        source: suggestion.source,
      },
      context: context ? {
        file_id: context.fileId,
        file_type: context.fileType,
        folder_path: context.folderPath,
        existing_tags: context.existingTags,
        existing_projects: context.existingProjects,
        metadata: context.metadata,
      } : undefined,
    });
    return {
      modifiedSuggestion: {
        type: this.mapSuggestionType(resp?.modified_suggestion?.type),
        value: resp?.modified_suggestion?.value ?? "",
        confidence: resp?.modified_suggestion?.confidence ?? 0,
        reasoning: resp?.modified_suggestion?.reasoning ?? "",
        source: resp?.modified_suggestion?.source ?? "",
      },
      confidenceDelta: resp?.confidence_delta ?? 0,
      appliedPreferences: resp?.applied_preferences ?? [],
      explanation: resp?.explanation ?? "",
    };
  }

  async getFeedbackStats(workspaceId: string, sinceTimestamp?: number): Promise<FeedbackStats> {
    const client = this.getClient();
    const resp = await this.unary(client.GetFeedbackStats.bind(client), {
      workspace_id: workspaceId,
      since_timestamp: sinceTimestamp ?? 0,
    });
    return {
      totalFeedback: resp?.total_feedback ?? 0,
      accepted: resp?.accepted ?? 0,
      rejected: resp?.rejected ?? 0,
      corrected: resp?.corrected ?? 0,
      ignored: resp?.ignored ?? 0,
      preferencesLearned: resp?.preferences_learned ?? 0,
      acceptanceRate: resp?.acceptance_rate ?? 0,
      avgConfidenceBoost: resp?.avg_confidence_boost ?? 0,
      avgResponseTimeMs: resp?.avg_response_time_ms ?? 0,
    };
  }

  async getFeedbackHistory(
    workspaceId: string,
    options?: {
      actionFilter?: FeedbackAction;
      limit?: number;
      offset?: number;
    }
  ): Promise<UserFeedback[]> {
    const client = this.getClient();
    return this.streamToArray(
      client.GetFeedbackHistory({
        workspace_id: workspaceId,
        action_filter: options?.actionFilter ? this.feedbackActionToProto(options.actionFilter) : 0,
        limit: options?.limit ?? 100,
        offset: options?.offset ?? 0,
      }),
      this.mapFeedback.bind(this)
    );
  }

  async consolidatePatterns(workspaceId: string, minExamples: number = 3): Promise<{
    success: boolean;
    patternsCreated: number;
    patternsMerged: number;
    patternsStrengthened: number;
    message: string;
  }> {
    const client = this.getClient();
    const resp = await this.unary(client.ConsolidatePatterns.bind(client), {
      workspace_id: workspaceId,
      min_examples: minExamples,
    });
    return {
      success: resp?.success ?? false,
      patternsCreated: resp?.patterns_created ?? 0,
      patternsMerged: resp?.patterns_merged ?? 0,
      patternsStrengthened: resp?.patterns_strengthened ?? 0,
      message: resp?.message ?? "",
    };
  }

  async cleanup(
    workspaceId: string,
    options?: {
      lowConfidenceThreshold?: number;
      staleDays?: number;
    }
  ): Promise<{
    success: boolean;
    preferencesRemoved: number;
    feedbackArchived: number;
    message: string;
  }> {
    const client = this.getClient();
    const resp = await this.unary(client.Cleanup.bind(client), {
      workspace_id: workspaceId,
      low_confidence_threshold: options?.lowConfidenceThreshold ?? 0.2,
      stale_days: options?.staleDays ?? 90,
    });
    return {
      success: resp?.success ?? false,
      preferencesRemoved: resp?.preferences_removed ?? 0,
      feedbackArchived: resp?.feedback_archived ?? 0,
      message: resp?.message ?? "",
    };
  }

  private mapPreference(p: any): LearnedPreference {
    return {
      id: p.id ?? "",
      workspaceId: p.workspace_id ?? "",
      type: this.mapPreferenceType(p.type),
      pattern: {
        fileExtensions: p.pattern?.file_extensions ?? [],
        folderPatterns: p.pattern?.folder_patterns ?? [],
        keywords: p.pattern?.keywords ?? [],
        metadataPatterns: p.pattern?.metadata_patterns,
      },
      behavior: {
        confidenceBoost: p.behavior?.confidence_boost ?? 0,
        preferredTags: p.behavior?.preferred_tags ?? [],
        avoidedTags: p.behavior?.avoided_tags ?? [],
        preferredProjects: p.behavior?.preferred_projects ?? [],
        avoidedProjects: p.behavior?.avoided_projects ?? [],
        namingRules: p.behavior?.naming_rules,
      },
      confidence: p.confidence ?? 0,
      exampleCount: p.example_count ?? 0,
      createdAt: Number(p.created_at ?? 0),
      updatedAt: Number(p.updated_at ?? 0),
      lastUsedAt: Number(p.last_used_at ?? 0),
    };
  }

  private mapFeedback(f: any): UserFeedback {
    return {
      id: f.id ?? "",
      workspaceId: f.workspace_id ?? "",
      action: this.mapFeedbackAction(f.action),
      suggestion: {
        type: this.mapSuggestionType(f.suggestion?.type),
        value: f.suggestion?.value ?? "",
        confidence: f.suggestion?.confidence ?? 0,
        reasoning: f.suggestion?.reasoning ?? "",
        source: f.suggestion?.source ?? "",
      },
      correction: f.correction ? {
        correctedValue: f.correction.corrected_value ?? "",
        correctionReason: f.correction.correction_reason ?? "",
      } : undefined,
      context: {
        fileId: f.context?.file_id ?? "",
        fileType: f.context?.file_type ?? "",
        folderPath: f.context?.folder_path ?? "",
        existingTags: f.context?.existing_tags ?? [],
        existingProjects: f.context?.existing_projects ?? [],
        metadata: f.context?.metadata,
      },
      responseTimeMs: Number(f.response_time_ms ?? 0),
      createdAt: Number(f.created_at ?? 0),
    };
  }

  private mapFeedbackAction(action: any): FeedbackAction {
    if (typeof action === "string") {
      if (action.includes("ACCEPTED")) return "ACCEPTED";
      if (action.includes("REJECTED")) return "REJECTED";
      if (action.includes("CORRECTED")) return "CORRECTED";
      if (action.includes("IGNORED")) return "IGNORED";
    }
    return "UNKNOWN";
  }

  private mapSuggestionType(type: any): SuggestionType {
    if (typeof type === "string") {
      if (type.includes("TAG")) return "TAG";
      if (type.includes("PROJECT")) return "PROJECT";
      if (type.includes("CATEGORY")) return "CATEGORY";
      if (type.includes("CLUSTER")) return "CLUSTER";
      if (type.includes("RELATIONSHIP")) return "RELATIONSHIP";
    }
    return "UNKNOWN";
  }

  private mapPreferenceType(type: any): PreferenceType {
    if (typeof type === "string") {
      if (type.includes("TAGGING")) return "TAGGING";
      if (type.includes("CATEGORIZATION")) return "CATEGORIZATION";
      if (type.includes("CLUSTERING")) return "CLUSTERING";
      if (type.includes("NAMING")) return "NAMING";
    }
    return "UNKNOWN";
  }

  private feedbackActionToProto(action: FeedbackAction): number {
    switch (action) {
      case "ACCEPTED": return 1;
      case "REJECTED": return 2;
      case "CORRECTED": return 3;
      case "IGNORED": return 4;
      default: return 0;
    }
  }

  private suggestionTypeToProto(type: SuggestionType): number {
    switch (type) {
      case "TAG": return 1;
      case "PROJECT": return 2;
      case "CATEGORY": return 3;
      case "CLUSTER": return 4;
      case "RELATIONSHIP": return 5;
      default: return 0;
    }
  }

  private preferenceTypeToProto(type: PreferenceType): number {
    switch (type) {
      case "TAGGING": return 1;
      case "CATEGORIZATION": return 2;
      case "CLUSTERING": return 3;
      case "NAMING": return 4;
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

  private getClient(): PreferencesClient {
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
        path.join(protoRoot, "cortex", "v1", "preferences.proto"),
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
    const PreferencesService = loaded.cortex?.v1?.PreferencesService;
    if (!PreferencesService) {
      throw new Error("PreferencesService not found in proto definitions");
    }

    this.client = new PreferencesService(
      address,
      grpc.credentials.createInsecure()
    ) as PreferencesClient;
    this.clientAddress = address;
    return this.client;
  }

  private getAddress(): string {
    const cfg = vscode.workspace.getConfiguration("cortex");
    return cfg.get<string>("grpc.address", "127.0.0.1:50051");
  }
}
