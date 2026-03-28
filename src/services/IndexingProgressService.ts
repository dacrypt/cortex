/**
 * IndexingProgressService - Shows progress bar when files are being indexed
 */

import * as vscode from 'vscode';
import { GrpcAdminClient, PipelineEvent } from '../core/GrpcAdminClient';

interface FileProgress {
  filePath: string;
  stage: string;
  status: 'processing' | 'completed' | 'failed';
  startedAt: number;
}

export class IndexingProgressService {
  private adminClient: GrpcAdminClient;
  private workspaceId?: string;
  private stopPipelineStream?: () => void;
  private activeFiles = new Map<string, FileProgress>();
  private statusBarItem: vscode.StatusBarItem;
  private progressNotification?: vscode.Progress<{ message?: string; increment?: number }>;
  private progressCancellationToken?: vscode.CancellationTokenSource;
  private isShowingProgress = false;
  private disposed = false; // Track if service has been disposed
  private activeTimers = new Set<NodeJS.Timeout>(); // Track all active timers

  constructor(context: vscode.ExtensionContext, workspaceId?: string) {
    this.adminClient = new GrpcAdminClient(context);
    this.workspaceId = workspaceId;

    // Create status bar item
    this.statusBarItem = vscode.window.createStatusBarItem(
      vscode.StatusBarAlignment.Right,
      100
    );
    this.statusBarItem.command = 'cortex.openPipelineProgress';
    this.statusBarItem.tooltip = 'Click to view detailed pipeline progress';
    this.statusBarItem.hide();
    context.subscriptions.push(this.statusBarItem);
  }

  /**
   * Start listening to pipeline events
   */
  start(): void {
    if (this.stopPipelineStream) {
      // Already started
      return;
    }

    if (!this.workspaceId) {
      console.warn('[IndexingProgressService] No workspace ID, cannot start event stream');
      return;
    }

    console.log('[IndexingProgressService] Starting progress monitoring for workspace:', this.workspaceId);

    this.stopPipelineStream = this.adminClient.subscribePipelineEvents(
      (event: PipelineEvent) => {
        this.handlePipelineEvent(event);
      },
      this.workspaceId
    );
  }

  /**
   * Stop listening to pipeline events
   */
  stop(): void {
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
      this.stopPipelineStream = undefined;
    }

    // Clear all active timers
    this.activeTimers.forEach(timer => clearTimeout(timer));
    this.activeTimers.clear();

    this.hideProgress();
    
    try {
      this.statusBarItem.hide();
    } catch (error) {
      // Ignore errors if status bar item is already disposed
    }
    
    this.activeFiles.clear();
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

    const filePath = event.file_path || '';
    if (!filePath) {
      return;
    }

    const eventType = event.type || '';
    const stage = event.stage || '';

    if (eventType === 'pipeline.started') {
      // File started processing
      this.activeFiles.set(filePath, {
        filePath,
        stage: 'start',
        status: 'processing',
        startedAt: event.timestamp_unix ? Number(event.timestamp_unix) * 1000 : Date.now(),
      });
      this.updateProgress();
    } else if (eventType === 'pipeline.progress') {
      // File progressing through stages
      const progress = this.activeFiles.get(filePath);
      if (progress) {
        progress.stage = stage;
        progress.status = 'processing';
        this.updateProgress();
      }
    } else if (eventType === 'pipeline.completed') {
      // File completed
      const progress = this.activeFiles.get(filePath);
      if (progress) {
        progress.status = 'completed';
        progress.stage = 'complete';
      }
      // Remove after a short delay to show completion
      const timer = setTimeout(() => {
        this.activeTimers.delete(timer);
        if (!this.disposed) {
          this.activeFiles.delete(filePath);
          this.updateProgress();
        }
      }, 2000);
      this.activeTimers.add(timer);
    } else if (eventType === 'pipeline.failed') {
      // File failed
      const progress = this.activeFiles.get(filePath);
      if (progress) {
        progress.status = 'failed';
      }
      // Remove after a short delay
      const timer = setTimeout(() => {
        this.activeTimers.delete(timer);
        if (!this.disposed) {
          this.activeFiles.delete(filePath);
          this.updateProgress();
        }
      }, 5000);
      this.activeTimers.add(timer);
    }
  }

  /**
   * Update progress display
   */
  private updateProgress(): void {
    // Don't update UI if disposed
    if (this.disposed) {
      return;
    }

    try {
      const processingFiles = Array.from(this.activeFiles.values()).filter(
        f => f.status === 'processing'
      );
      const completedFiles = Array.from(this.activeFiles.values()).filter(
        f => f.status === 'completed'
      );
      const failedFiles = Array.from(this.activeFiles.values()).filter(
        f => f.status === 'failed'
      );

      const totalActive = this.activeFiles.size;

      if (totalActive === 0) {
        // No files being processed
        this.hideProgress();
        try {
          this.statusBarItem.hide();
        } catch (error) {
          // Ignore errors if status bar item is disposed
        }
        return;
      }

      // Update status bar
      if (processingFiles.length > 0) {
        const currentFile = processingFiles[0];
        const fileName = currentFile.filePath.split('/').pop() || currentFile.filePath;
        this.statusBarItem.text = `$(sync~spin) Indexing: ${fileName} (${processingFiles.length})`;
        this.statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.prominentBackground');
        this.statusBarItem.show();
      } else if (completedFiles.length > 0 || failedFiles.length > 0) {
        this.statusBarItem.text = `$(check) Indexing: ${completedFiles.length} done, ${failedFiles.length} failed`;
        this.statusBarItem.backgroundColor = undefined;
        this.statusBarItem.show();
      }

      // Show progress notification if there are files being processed
      if (processingFiles.length > 0 && !this.isShowingProgress) {
        this.showProgress(processingFiles);
      } else if (processingFiles.length > 0 && this.isShowingProgress) {
        this.updateProgressNotification(processingFiles);
      } else if (processingFiles.length === 0 && this.isShowingProgress) {
        this.hideProgress();
      }
    } catch (error) {
      // Silently ignore errors during disposal
      if (!this.disposed) {
        try {
          console.error('[IndexingProgressService] Error updating progress:', error);
        } catch (logError) {
          // Ignore logging errors
        }
      }
    }
  }

  /**
   * Show progress notification
   */
  private showProgress(processingFiles: FileProgress[]): void {
    if (this.isShowingProgress || this.disposed) {
      return;
    }

    try {
      this.isShowingProgress = true;
      this.progressCancellationToken = new vscode.CancellationTokenSource();

      const currentFile = processingFiles[0];
      const fileName = currentFile.filePath.split('/').pop() || currentFile.filePath;

      vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: 'Cortex: Indexing Files',
          cancellable: false,
        },
        async (progress, token) => {
          if (this.disposed) {
            return;
          }

          this.progressNotification = progress;
          token.onCancellationRequested(() => {
            this.hideProgress();
          });

          // Update progress periodically
          const updateInterval = setInterval(() => {
            if (this.disposed || token.isCancellationRequested) {
              clearInterval(updateInterval);
              this.activeTimers.delete(updateInterval as any);
              return;
            }

            const currentProcessing = Array.from(this.activeFiles.values()).filter(
              f => f.status === 'processing'
            );

            if (currentProcessing.length === 0) {
              clearInterval(updateInterval);
              this.activeTimers.delete(updateInterval as any);
              this.hideProgress();
              return;
            }

            try {
              const current = currentProcessing[0];
              const currentFileName = current.filePath.split('/').pop() || current.filePath;
              const stageName = this.getStageDisplayName(current.stage);
              
              // Calculate progress percentage based on stage
              const stageProgress = this.getStageProgress(current.stage);
              
              progress.report({
                message: `${currentFileName} - ${stageName} (${currentProcessing.length} file${currentProcessing.length > 1 ? 's' : ''} in queue)`,
                increment: stageProgress,
              });
            } catch (error) {
              // Ignore errors during disposal
              if (!this.disposed) {
                clearInterval(updateInterval);
                this.activeTimers.delete(updateInterval as any);
              }
            }
          }, 500);
          this.activeTimers.add(updateInterval as any);

          // Wait until no more files are processing
          return new Promise<void>((resolve) => {
            const checkInterval = setInterval(() => {
              if (this.disposed) {
                clearInterval(checkInterval);
                clearInterval(updateInterval);
                this.activeTimers.delete(checkInterval as any);
                this.activeTimers.delete(updateInterval as any);
                resolve();
                return;
              }

              const stillProcessing = Array.from(this.activeFiles.values()).filter(
                f => f.status === 'processing'
              );
              if (stillProcessing.length === 0) {
                clearInterval(checkInterval);
                clearInterval(updateInterval);
                this.activeTimers.delete(checkInterval as any);
                this.activeTimers.delete(updateInterval as any);
                resolve();
              }
            }, 1000);
            this.activeTimers.add(checkInterval as any);
          });
        }
      ).then(() => {
        if (!this.disposed) {
          this.isShowingProgress = false;
          this.progressCancellationToken = undefined;
        }
      }, (error: unknown) => {
        // Ignore errors during disposal
        if (!this.disposed) {
          try {
            console.error('[IndexingProgressService] Error showing progress:', error);
          } catch (logError) {
            // Ignore logging errors
          }
        }
        this.isShowingProgress = false;
        this.progressCancellationToken = undefined;
      });
    } catch (error) {
      // Ignore errors during disposal
      if (!this.disposed) {
        try {
          console.error('[IndexingProgressService] Error showing progress:', error);
        } catch (logError) {
          // Ignore logging errors
        }
      }
      this.isShowingProgress = false;
    }
  }

  /**
   * Update progress notification
   */
  private updateProgressNotification(processingFiles: FileProgress[]): void {
    if (!this.progressNotification || !this.isShowingProgress || this.disposed) {
      return;
    }

    try {
      const current = processingFiles[0];
      const currentFileName = current.filePath.split('/').pop() || current.filePath;
      const stageName = this.getStageDisplayName(current.stage);
      const stageProgress = this.getStageProgress(current.stage);
      
      this.progressNotification.report({
        message: `${currentFileName} - ${stageName} (${processingFiles.length} file${processingFiles.length > 1 ? 's' : ''} in queue)`,
        increment: stageProgress,
      });
    } catch (error) {
      // Ignore errors during disposal
      if (!this.disposed) {
        try {
          console.error('[IndexingProgressService] Error updating progress notification:', error);
        } catch (logError) {
          // Ignore logging errors
        }
      }
    }
  }

  /**
   * Hide progress notification
   */
  private hideProgress(): void {
    if (this.progressCancellationToken) {
      this.progressCancellationToken.cancel();
      this.progressCancellationToken.dispose();
      this.progressCancellationToken = undefined;
    }
    this.isShowingProgress = false;
    this.progressNotification = undefined;
  }

  /**
   * Get display name for pipeline stage
   */
  private getStageDisplayName(stage: string): string {
    const stageNames: Record<string, string> = {
      'start': 'Starting',
      'basic': 'Basic Info',
      'mime': 'MIME Detection',
      'mirror': 'Content Extraction',
      'code': 'Code Analysis',
      'document': 'Document Parsing',
      'ai': 'AI Processing',
      'relationship': 'Relationships',
      'state': 'State Inference',
      'complete': 'Complete',
    };
    return stageNames[stage] || stage;
  }

  /**
   * Get progress percentage for a stage (0-100)
   */
  private getStageProgress(stage: string): number {
    const stageOrder = ['start', 'basic', 'mime', 'mirror', 'code', 'document', 'ai', 'relationship', 'state', 'complete'];
    const stageIndex = stageOrder.indexOf(stage);
    if (stageIndex === -1) {
      return 0;
    }
    // Each stage represents roughly 10% progress
    return Math.min((stageIndex + 1) * 10, 100);
  }

  /**
   * Dispose of the service
   */
  dispose(): void {
    this.disposed = true;
    this.stop();
    
    try {
      this.statusBarItem.dispose();
    } catch (error) {
      // Ignore errors if already disposed
    }
  }
}

