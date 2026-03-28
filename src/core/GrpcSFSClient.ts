import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import * as path from 'path';

// SFS Operation types
export type SFSOperationType =
  | 'unknown'
  | 'group'
  | 'find'
  | 'tag'
  | 'untag'
  | 'assign'
  | 'unassign'
  | 'create'
  | 'merge'
  | 'rename'
  | 'summarize'
  | 'relate'
  | 'query';

// File change represents a single change to a file
export interface FileChange {
  file_id: string;
  relative_path: string;
  operation: SFSOperationType;
  before_value: string;
  after_value: string;
  target: string;
}

// Command result from executing a command
export interface SFSCommandResult {
  success: boolean;
  operation: SFSOperationType;
  changes: FileChange[];
  explanation: string;
  error_message: string;
  files_affected: number;
  undo_command: string;
}

// Preview result showing what would happen
export interface SFSPreviewResult {
  operation: SFSOperationType;
  planned_changes: FileChange[];
  explanation: string;
  files_affected: number;
  confidence: number;
  warnings: string[];
  alternative_interpretations: string[];
}

// Command suggestion
export interface CommandSuggestion {
  command: string;
  description: string;
  operation: SFSOperationType;
  relevance: number;
  category: string;
}

// Command history entry
export interface SFSCommandHistoryEntry {
  id: string;
  workspace_id: string;
  command: string;
  operation: SFSOperationType;
  success: boolean;
  files_affected: number;
  executed_at: number;
  result_summary: string;
}

// Proto enum to string mapping
const protoOperationToString = (op: number): SFSOperationType => {
  const mapping: { [key: number]: SFSOperationType } = {
    0: 'unknown',
    1: 'group',
    2: 'find',
    3: 'tag',
    4: 'untag',
    5: 'assign',
    6: 'unassign',
    7: 'create',
    8: 'merge',
    9: 'rename',
    10: 'summarize',
    11: 'relate',
    12: 'query',
  };
  return mapping[op] || 'unknown';
};

export class GrpcSFSClient {
  private client: any;
  private connected: boolean = false;
  private endpoint: string;

  constructor(endpoint: string = 'localhost:50051') {
    this.endpoint = endpoint;
  }

  async connect(): Promise<void> {
    if (this.connected) {
      return;
    }

    const protoPath = path.join(__dirname, '../../backend/api/proto/cortex/v1/sfs.proto');

    const packageDefinition = protoLoader.loadSync(protoPath, {
      keepCase: true,
      longs: String,
      enums: Number,
      defaults: true,
      oneofs: true,
      includeDirs: [path.join(__dirname, '../../backend/api/proto')],
    });

    const protoDescriptor = grpc.loadPackageDefinition(packageDefinition) as any;
    const SFSService = protoDescriptor.cortex.v1.SemanticFileSystemService;

    this.client = new SFSService(
      this.endpoint,
      grpc.credentials.createInsecure()
    );

    this.connected = true;
  }

  async disconnect(): Promise<void> {
    if (this.client) {
      grpc.closeClient(this.client);
      this.connected = false;
    }
  }

  // Execute a natural language command
  async executeCommand(
    workspaceId: string,
    command: string,
    contextFileIds: string[] = [],
    dryRun: boolean = false
  ): Promise<SFSCommandResult> {
    await this.connect();

    return new Promise((resolve, reject) => {
      this.client.ExecuteCommand(
        {
          workspace_id: workspaceId,
          command: command,
          dry_run: dryRun,
          context_file_ids: contextFileIds,
        },
        (error: grpc.ServiceError | null, response: any) => {
          if (error) {
            reject(error);
            return;
          }

          resolve({
            success: response.success,
            operation: protoOperationToString(response.operation),
            changes: (response.changes || []).map((c: any) => ({
              file_id: c.file_id,
              relative_path: c.relative_path,
              operation: protoOperationToString(c.operation),
              before_value: c.before_value || '',
              after_value: c.after_value || '',
              target: c.target || '',
            })),
            explanation: response.explanation || '',
            error_message: response.error_message || '',
            files_affected: response.files_affected || 0,
            undo_command: response.undo_command || '',
          });
        }
      );
    });
  }

  // Preview a command without executing
  async previewCommand(
    workspaceId: string,
    command: string,
    contextFileIds: string[] = []
  ): Promise<SFSPreviewResult> {
    await this.connect();

    return new Promise((resolve, reject) => {
      this.client.PreviewCommand(
        {
          workspace_id: workspaceId,
          command: command,
          dry_run: true,
          context_file_ids: contextFileIds,
        },
        (error: grpc.ServiceError | null, response: any) => {
          if (error) {
            reject(error);
            return;
          }

          resolve({
            operation: protoOperationToString(response.operation),
            planned_changes: (response.planned_changes || []).map((c: any) => ({
              file_id: c.file_id,
              relative_path: c.relative_path,
              operation: protoOperationToString(c.operation),
              before_value: c.before_value || '',
              after_value: c.after_value || '',
              target: c.target || '',
            })),
            explanation: response.explanation || '',
            files_affected: response.files_affected || 0,
            confidence: response.confidence || 0,
            warnings: response.warnings || [],
            alternative_interpretations: response.alternative_interpretations || [],
          });
        }
      );
    });
  }

  // Get command suggestions based on context
  async suggestCommands(
    workspaceId: string,
    partialCommand: string = '',
    contextFileIds: string[] = [],
    limit: number = 10
  ): Promise<CommandSuggestion[]> {
    await this.connect();

    return new Promise((resolve, reject) => {
      const suggestions: CommandSuggestion[] = [];

      const stream = this.client.SuggestCommands({
        workspace_id: workspaceId,
        partial_command: partialCommand,
        context_file_ids: contextFileIds,
        limit: limit,
      });

      stream.on('data', (suggestion: any) => {
        suggestions.push({
          command: suggestion.command,
          description: suggestion.description || '',
          operation: protoOperationToString(suggestion.operation),
          relevance: suggestion.relevance || 0,
          category: suggestion.category || '',
        });
      });

      stream.on('error', (error: grpc.ServiceError) => {
        reject(error);
      });

      stream.on('end', () => {
        resolve(suggestions);
      });
    });
  }

  // Get command history for workspace
  async getCommandHistory(
    workspaceId: string,
    limit: number = 20,
    sinceTimestamp?: number
  ): Promise<SFSCommandHistoryEntry[]> {
    await this.connect();

    return new Promise((resolve, reject) => {
      const history: SFSCommandHistoryEntry[] = [];

      const stream = this.client.GetCommandHistory({
        workspace_id: workspaceId,
        limit: limit,
        since_timestamp: sinceTimestamp || 0,
      });

      stream.on('data', (entry: any) => {
        history.push({
          id: entry.id,
          workspace_id: entry.workspace_id,
          command: entry.command,
          operation: protoOperationToString(entry.operation),
          success: entry.success,
          files_affected: entry.files_affected || 0,
          executed_at: parseInt(entry.executed_at) || 0,
          result_summary: entry.result_summary || '',
        });
      });

      stream.on('error', (error: grpc.ServiceError) => {
        reject(error);
      });

      stream.on('end', () => {
        resolve(history);
      });
    });
  }
}
