/**
 * RealtimeUpdateService - Listens to backend events and updates UI in real-time
 */

import * as vscode from 'vscode';
import { GrpcAdminClient, PipelineEvent } from '../core/GrpcAdminClient';

export interface TreeProvider {
  refresh(): void;
}

// Pipeline stages that should trigger UI refresh
const REFRESH_STAGES = new Set([
  'ai', 'document', 'relationship',  // Original stages
  'metadata', 'project_inference', 'suggestion', 'folder_index', 'code', 'mirror',  // Added stages
  'basic', 'mime', 'enrichment', 'state', 'temporal_cluster'  // Additional stages for completeness
]);

export class RealtimeUpdateService {
  private adminClient: GrpcAdminClient;
  private workspaceId?: string;
  private stopPipelineStream?: () => void;
  private refreshCallbacks: Set<() => void> = new Set();
  private debounceTimer?: NodeJS.Timeout;
  private readonly DEBOUNCE_MS = 500; // Debounce refresh calls to avoid excessive updates
  private disposed = false; // Track if service has been disposed

  constructor(context: vscode.ExtensionContext, workspaceId?: string) {
    this.adminClient = new GrpcAdminClient(context);
    this.workspaceId = workspaceId;
  }

  /**
   * Register a callback to be called when UI should refresh
   */
  registerRefreshCallback(callback: () => void): void {
    this.refreshCallbacks.add(callback);
  }

  /**
   * Unregister a refresh callback
   */
  unregisterRefreshCallback(callback: () => void): void {
    this.refreshCallbacks.delete(callback);
  }

  /**
   * Start listening to backend events
   */
  start(): void {
    if (this.stopPipelineStream) {
      // Already started
      return;
    }

    if (!this.workspaceId) {
      console.warn('[RealtimeUpdateService] No workspace ID, cannot start event stream');
      return;
    }

    console.log('[RealtimeUpdateService] Starting event stream for workspace:', this.workspaceId);

    this.stopPipelineStream = this.adminClient.subscribePipelineEvents(
      (event: PipelineEvent) => {
        this.handlePipelineEvent(event);
      },
      this.workspaceId
    );
  }

  /**
   * Stop listening to backend events
   */
  stop(): void {
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
      this.stopPipelineStream = undefined;
    }

    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = undefined;
    }

    // Only log if not disposed (to avoid logging after channel is closed)
    if (!this.disposed) {
      try {
        console.log('[RealtimeUpdateService] Stopped event stream');
      } catch (error) {
        // Ignore logging errors during disposal
      }
    }
  }

  /**
   * Update workspace ID and restart stream if needed
   */
  setWorkspaceId(workspaceId: string | undefined): void {
    if (this.workspaceId === workspaceId) {
      return;
    }

    const wasRunning = !!this.stopPipelineStream;
    this.stop();
    this.workspaceId = workspaceId;

    if (wasRunning && workspaceId) {
      this.start();
    }
  }

  /**
   * Handle pipeline events from backend
   */
  private handlePipelineEvent(event: PipelineEvent): void {
    // Don't process events if disposed
    if (this.disposed) {
      return;
    }

    // Refresh on completion/failure or when significant stages complete
    const shouldRefresh =
      event.type === 'pipeline.completed' ||
      event.type === 'pipeline.failed' ||
      (event.type === 'pipeline.progress' && event.stage && REFRESH_STAGES.has(event.stage));

    if (shouldRefresh) {
      try {
        console.log('[RealtimeUpdateService] Pipeline event received, scheduling UI refresh:', {
          type: event.type,
          stage: event.stage,
          file: event.file_path
        });
      } catch (error) {
        // Ignore logging errors
      }

      this.debouncedRefresh();
    }
  }

  /**
   * Trigger refresh of all registered callbacks with debouncing
   */
  private debouncedRefresh(): void {
    // Don't schedule refresh if disposed
    if (this.disposed) {
      return;
    }

    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }

    this.debounceTimer = setTimeout(() => {
      // Check again if disposed before executing
      if (this.disposed) {
        return;
      }

      try {
        console.log('[RealtimeUpdateService] Refreshing UI (callbacks:', this.refreshCallbacks.size, ')');
      } catch (error) {
        // Ignore logging errors
      }

      this.refreshCallbacks.forEach(callback => {
        try {
          callback();
        } catch (error) {
          // Silently ignore errors during disposal
          if (!this.disposed) {
            try {
              console.error('[RealtimeUpdateService] Error in refresh callback:', error);
            } catch (logError) {
              // Ignore logging errors
            }
          }
        }
      });
      this.debounceTimer = undefined;
    }, this.DEBOUNCE_MS);
  }

  /**
   * Manually trigger a refresh (useful for testing or manual updates)
   */
  refresh(): void {
    this.debouncedRefresh();
  }

  /**
   * Dispose of the service
   */
  dispose(): void {
    this.disposed = true;
    this.stop();
    this.refreshCallbacks.clear();
  }
}

