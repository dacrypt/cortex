/**
 * FileInfoWebview - Modern, visually rich file information panel
 *
 * Features:
 * - Full-width responsive layout with cards
 * - Collapsible sections with state persistence
 * - Interactive tags with remove buttons
 * - Notes section with inline editing
 * - Tag cloud visualization
 * - Project cards with descriptions
 * - Similar files discovery
 * - AI suggestions with accept/reject actions
 * - Quick actions bar
 * - Copy buttons for metadata
 * - Loading states and skeleton UI
 * - Technical metadata in compact grid
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import * as crypto from 'node:crypto';
import { IMetadataStore } from '../core/IMetadataStore';
import { GrpcMetadataClient } from '../core/GrpcMetadataClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcPreferencesClient, FeedbackStats, AISuggestion } from '../core/GrpcPreferencesClient';
import { FileMetadata, SuggestedMetadata } from '../models/types';

interface FileEntry {
  file_id?: string;
  filename?: string;
  extension?: string;
  fileSize?: number;
  lastModified?: number;
  enhanced?: {
    stats?: {
      size?: number;
      created?: number;
      modified?: number;
    };
    folder?: string;
    language?: string;
    mimeType?: string;
    mime_type?: { mime_type?: string; category?: string };
    document_metrics?: Record<string, unknown>;
    indexed?: Record<string, boolean>;
  };
}

interface ProjectInfo {
  id: string;
  name: string;
  description?: string;
  fileCount?: number;
  color?: string;
}

interface SimilarFile {
  relativePath: string;
  similarity: number;
  reason?: string;
  sharedTags?: string[];
  sharedProjects?: string[];
}

export class FileInfoWebviewProvider implements vscode.WebviewViewProvider {
  public static readonly viewType = 'cortex-fileInfoWebview';

  private view?: vscode.WebviewView;
  private currentFile: string | null = null;
  private disposed = false;
  private isLoading = false;
  private fileNotFound = false;
  private cachedMetadata: FileMetadata | null = null;
  private cachedFileEntry: FileEntry | null = null;
  private cachedProjects: ProjectInfo[] = [];
  private cachedSimilarFiles: SimilarFile[] = [];
  private cachedSuggestions: SuggestedMetadata | null = null;
  private cachedFeedbackStats: FeedbackStats | null = null;
  private feedbackAnimationPending: { type: 'accept' | 'reject'; id: string } | null = null;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly workspaceRoot: string,
    private readonly metadataStore: IMetadataStore,
    private readonly metadataClient?: GrpcMetadataClient,
    private readonly adminClient?: GrpcAdminClient,
    private readonly knowledgeClient?: GrpcKnowledgeClient,
    private readonly preferencesClient?: GrpcPreferencesClient,
    private backendWorkspaceId?: string
  ) {}

  public setBackendWorkspaceId(workspaceId: string): void {
    this.backendWorkspaceId = workspaceId;
    if (this.currentFile) {
      void this.updateCurrentFile(this.currentFile);
    }
  }

  public resolveWebviewView(
    webviewView: vscode.WebviewView,
    _context: vscode.WebviewViewResolveContext,
    _token: vscode.CancellationToken
  ): void {
    this.view = webviewView;

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [this.extensionUri],
    };

    webviewView.webview.onDidReceiveMessage(async (message) => {
      await this.handleMessage(message);
    });

    this.updateWebview();
  }

  public async updateCurrentFile(relativePath: string | null): Promise<void> {
    if (this.disposed) return;

    this.currentFile = relativePath;
    this.isLoading = true;
    this.updateWebview(); // Show loading state immediately

    if (relativePath) {
      await this.loadAllData(relativePath);
    } else {
      this.clearCache();
    }

    this.isLoading = false;
    this.updateWebview();
  }

  public refresh(): void {
    if (this.currentFile) {
      void this.updateCurrentFile(this.currentFile);
    }
  }

  public dispose(): void {
    this.disposed = true;
    this.clearCache();
  }

  private clearCache(): void {
    this.cachedMetadata = null;
    this.cachedFileEntry = null;
    this.cachedProjects = [];
    this.cachedSimilarFiles = [];
    this.cachedSuggestions = null;
    this.fileNotFound = false;
    this.cachedFeedbackStats = null;
  }

  private async loadAllData(relativePath: string): Promise<void> {
    // Load metadata from local store
    this.cachedMetadata = this.metadataStore.getMetadataByPath(relativePath);

    // Load data in parallel
    await Promise.all([
      this.loadFileEntry(relativePath),
      this.loadProjects(relativePath),
      this.loadSuggestions(relativePath),
      this.loadSimilarFiles(relativePath),
      this.loadFeedbackStats(),
    ]);
  }

  private async loadFileEntry(relativePath: string): Promise<void> {
    if (!this.adminClient || !this.backendWorkspaceId) return;

    try {
      this.cachedFileEntry = await this.adminClient.getFile(
        this.backendWorkspaceId,
        relativePath
      );
      this.fileNotFound = false;
    } catch (error: any) {
      // Check if it's a NOT_FOUND error (gRPC code 5)
      if (error?.code === 5 || error?.message?.includes('NOT_FOUND') || error?.message?.includes('file not found')) {
        this.fileNotFound = true;
        this.cachedFileEntry = null;
        console.log(`[FileInfoWebview] File not found in backend: ${relativePath}`);
      } else {
        console.warn('[FileInfoWebview] Failed to load file entry:', error);
        this.fileNotFound = false;
      }
    }
  }

  private async loadProjects(relativePath: string): Promise<void> {
    if (!this.knowledgeClient || !this.backendWorkspaceId) return;

    try {
      const normalized = relativePath.replaceAll('\\', '/');
      const documentId = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');
      const projectIds = await this.knowledgeClient.getProjectsForDocument(
        this.backendWorkspaceId,
        documentId
      );

      this.cachedProjects = [];
      for (const projectId of projectIds) {
        const project = await this.knowledgeClient.getProject(this.backendWorkspaceId, projectId);
        if (project) {
          this.cachedProjects.push({
            id: project.id,
            name: project.name,
            description: project.description,
            color: this.getProjectColor(project.name),
          });
        }
      }
    } catch (error) {
      console.warn('[FileInfoWebview] Failed to load projects:', error);
    }
  }

  private async loadSuggestions(relativePath: string): Promise<void> {
    if (!this.metadataClient || !this.backendWorkspaceId) return;

    try {
      this.cachedSuggestions = await this.metadataClient.getSuggestedMetadata(
        this.backendWorkspaceId,
        relativePath
      );
    } catch (error) {
      console.warn('[FileInfoWebview] Failed to load suggestions:', error);
    }
  }

  private async loadSimilarFiles(_relativePath: string): Promise<void> {
    // Use aiRelated from metadata if available
    if (this.cachedMetadata?.aiRelated) {
      this.cachedSimilarFiles = this.cachedMetadata.aiRelated.map(r => ({
        relativePath: r.relativePath,
        similarity: r.similarity || 0.5,
        reason: r.reason,
      }));
      return;
    }

    // Similar files will be populated by AI when available
    this.cachedSimilarFiles = [];
  }

  private async loadFeedbackStats(): Promise<void> {
    if (!this.preferencesClient || !this.backendWorkspaceId) return;

    try {
      this.cachedFeedbackStats = await this.preferencesClient.getFeedbackStats(
        this.backendWorkspaceId
      );
    } catch (error: any) {
      // Handle unimplemented service gracefully
      if (error?.code === 12 || error?.message?.includes('UNIMPLEMENTED')) {
        // Service not available, use empty stats
        this.cachedFeedbackStats = {
          totalFeedback: 0,
          accepted: 0,
          rejected: 0,
          corrected: 0,
          ignored: 0,
          preferencesLearned: 0,
          acceptanceRate: 0,
          avgConfidenceBoost: 0,
          avgResponseTimeMs: 0,
        };
        return;
      }
      console.warn('[FileInfoWebview] Failed to load feedback stats:', error);
    }
  }

  private async recordFeedback(
    action: 'ACCEPTED' | 'REJECTED',
    suggestionType: 'TAG' | 'PROJECT',
    value: string,
    confidence: number
  ): Promise<void> {
    if (!this.preferencesClient || !this.backendWorkspaceId || !this.currentFile) return;

    const suggestion: AISuggestion = {
      type: suggestionType,
      value,
      confidence,
      reasoning: '',
      source: 'ai',
    };

    try {
      const result = await this.preferencesClient.recordFeedback(
        this.backendWorkspaceId,
        action,
        suggestion,
        {
          context: {
            fileId: this.cachedFileEntry?.file_id || '',
            fileType: this.cachedFileEntry?.extension || '',
            folderPath: path.dirname(this.currentFile),
            existingTags: this.cachedMetadata?.tags || [],
            existingProjects: this.cachedProjects.map(p => p.id),
          },
        }
      );

      if (result.preferenceUpdated) {
        // Reload feedback stats to show updated preferences
        await this.loadFeedbackStats();
      }
    } catch (error) {
      console.warn('[FileInfoWebview] Failed to record feedback:', error);
    }
  }

  private async handleMessage(message: { command: string; data?: unknown }): Promise<void> {
    switch (message.command) {
      case 'addTag':
        await vscode.commands.executeCommand('cortex.addTag');
        break;
      case 'removeTag':
        if (typeof message.data === 'string' && this.currentFile) {
          this.metadataStore.removeTag(this.currentFile, message.data);
          this.refresh();
        }
        break;
      case 'assignProject':
        await vscode.commands.executeCommand('cortex.assignContext');
        break;
      case 'acceptTag':
        if (typeof message.data === 'object' && message.data !== null) {
          const { tag, confidence } = message.data as { tag: string; confidence: number };
          await this.recordFeedback('ACCEPTED', 'TAG', tag, confidence);
          await vscode.commands.executeCommand('cortex.acceptSuggestedTag', tag);
        } else if (typeof message.data === 'string') {
          await vscode.commands.executeCommand('cortex.acceptSuggestedTag', message.data);
        }
        this.refresh();
        break;
      case 'rejectTag':
        if (typeof message.data === 'object' && message.data !== null) {
          const { tag, confidence } = message.data as { tag: string; confidence: number };
          await this.recordFeedback('REJECTED', 'TAG', tag, confidence);
          await vscode.commands.executeCommand('cortex.rejectSuggestedTag', tag);
        } else if (typeof message.data === 'string') {
          await vscode.commands.executeCommand('cortex.rejectSuggestedTag', message.data);
        }
        this.refresh();
        break;
      case 'acceptProject':
        if (typeof message.data === 'object' && message.data !== null) {
          const { projectName, confidence } = message.data as { projectName: string; confidence: number };
          await this.recordFeedback('ACCEPTED', 'PROJECT', projectName, confidence);
          await vscode.commands.executeCommand('cortex.acceptSuggestedProject', projectName);
        } else if (typeof message.data === 'string') {
          await vscode.commands.executeCommand('cortex.acceptSuggestedProject', message.data);
        }
        this.refresh();
        break;
      case 'rejectProject':
        if (typeof message.data === 'object' && message.data !== null) {
          const { projectName, confidence } = message.data as { projectName: string; confidence: number };
          await this.recordFeedback('REJECTED', 'PROJECT', projectName, confidence);
          await vscode.commands.executeCommand('cortex.rejectSuggestedProject', projectName);
        } else if (typeof message.data === 'string') {
          await vscode.commands.executeCommand('cortex.rejectSuggestedProject', message.data);
        }
        this.refresh();
        break;
      case 'generateSummary':
        await vscode.commands.executeCommand('cortex.generateSummaryAI');
        this.refresh();
        break;
      case 'openFile':
        if (typeof message.data === 'string') {
          const absolutePath = path.join(this.workspaceRoot, message.data);
          const uri = vscode.Uri.file(absolutePath);
          await vscode.window.showTextDocument(uri);
        }
        break;
      case 'copyToClipboard':
        if (typeof message.data === 'string') {
          await vscode.env.clipboard.writeText(message.data);
          void vscode.window.showInformationMessage('Copied to clipboard');
        }
        break;
      case 'copyPath':
        if (this.currentFile) {
          await vscode.env.clipboard.writeText(this.currentFile);
          void vscode.window.showInformationMessage('Path copied to clipboard');
        }
        break;
      case 'revealInExplorer':
        if (this.currentFile) {
          const absolutePath = path.join(this.workspaceRoot, this.currentFile);
          await vscode.commands.executeCommand('revealFileInOS', vscode.Uri.file(absolutePath));
        }
        break;
      case 'saveNotes':
        if (typeof message.data === 'string' && this.currentFile) {
          this.metadataStore.updateNotes(this.currentFile, message.data);
          this.refresh();
        }
        break;
      case 'refresh':
        this.refresh();
        break;
      case 'rebuildIndex':
        await vscode.commands.executeCommand('cortex.rebuildIndex');
        void vscode.window.showInformationMessage('Rebuilding workspace index...');
        // Refresh after a short delay to allow indexing to start
        setTimeout(() => {
          this.refresh();
        }, 1000);
        break;
    }
  }

  private updateWebview(): void {
    if (!this.view) return;
    this.view.webview.html = this.getWebviewContent();
  }

  private getWebviewContent(): string {
    if (!this.currentFile) {
      return this.getEmptyStateHtml();
    }

    if (this.isLoading) {
      return this.getLoadingStateHtml();
    }

    if (this.fileNotFound) {
      return this.getFileNotFoundHtml();
    }

    const metadata = this.cachedMetadata;
    const fileEntry = this.cachedFileEntry;
    const projects = this.cachedProjects;
    const suggestions = this.cachedSuggestions;
    const similarFiles = this.cachedSimilarFiles;

    const filename = path.basename(this.currentFile);
    const extension = path.extname(this.currentFile).toLowerCase();
    const fileSize = fileEntry?.fileSize || fileEntry?.enhanced?.stats?.size || 0;
    const lastModified = fileEntry?.lastModified || fileEntry?.enhanced?.stats?.modified || 0;
    const tags = metadata?.tags || [];
    const notes = metadata?.notes || '';
    const hasSuggestions = (suggestions?.suggestedTags?.length || 0) +
                          (suggestions?.suggestedProjects?.length || 0) > 0;

    return `<!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>File Info</title>
      <style>
        ${this.getStyles()}
      </style>
    </head>
    <body>
      <!-- Hero Section -->
      <div class="hero">
        <div class="hero-icon">${this.getFileIcon(extension)}</div>
        <div class="hero-info">
          <h1 class="hero-title" title="${this.escapeHtml(this.currentFile)}">${this.escapeHtml(filename)}</h1>
          <div class="hero-meta">
            <span class="badge badge-type">${extension || 'file'}</span>
            <span class="meta-separator">•</span>
            <span>${this.formatFileSize(fileSize)}</span>
            <span class="meta-separator">•</span>
            <span>${this.formatDate(lastModified)}</span>
          </div>
        </div>
        <button class="refresh-btn" onclick="sendMessage('refresh')" title="Refresh">🔄</button>
      </div>

      <!-- Quick Actions -->
      <div class="quick-actions">
        <button class="action-btn primary" onclick="sendMessage('addTag')">
          <span class="icon">🏷️</span> Add Tag
        </button>
        <button class="action-btn primary" onclick="sendMessage('assignProject')">
          <span class="icon">📁</span> Assign Project
        </button>
        <button class="action-btn" onclick="sendMessage('generateSummary')">
          <span class="icon">✨</span> AI Summary
        </button>
        <button class="action-btn" onclick="sendMessage('copyPath')">
          <span class="icon">📋</span> Copy Path
        </button>
        <button class="action-btn" onclick="sendMessage('revealInExplorer')">
          <span class="icon">📂</span> Reveal
        </button>
      </div>

      <div class="sections">
        <!-- Projects Section -->
        ${this.renderCollapsibleCard('projects', '📁 Projects', projects.length,
          projects.length > 0
            ? this.renderProjectCards(projects)
            : '<p class="empty-state">No projects assigned. Click "Assign Project" to add one.</p>'
        )}

        <!-- Tags Section -->
        ${this.renderCollapsibleCard('tags', '🏷️ Tags', tags.length,
          tags.length > 0
            ? this.renderInteractiveTags(tags)
            : '<p class="empty-state">No tags assigned. Click "Add Tag" to add one.</p>'
        )}

        <!-- Notes Section -->
        ${this.renderCollapsibleCard('notes', '📝 Notes', notes ? 1 : 0,
          this.renderNotesSection(notes)
        )}

        <!-- Feedback Stats (if available) -->
        ${this.renderFeedbackStatsIndicator()}

        <!-- AI Suggestions (if available) -->
        ${hasSuggestions ? this.renderCollapsibleCard('suggestions', '✨ AI Suggestions',
          (suggestions?.suggestedTags?.length || 0) + (suggestions?.suggestedProjects?.length || 0),
          this.renderSuggestions(suggestions),
          'glow'
        ) : ''}

        <!-- Similar Files -->
        ${similarFiles.length > 0 ? this.renderCollapsibleCard('similar', '🔗 Similar Files', similarFiles.length,
          this.renderSimilarFiles(similarFiles)
        ) : ''}

        <!-- AI Summary (if available) -->
        ${metadata?.aiSummary ? this.renderCollapsibleCard('summary', '🤖 AI Summary', 0,
          this.renderAISummary(metadata)
        ) : ''}

        <!-- Technical Metadata -->
        ${this.renderCollapsibleCard('metadata', '📊 Technical Details', 0,
          this.renderTechnicalMetadata(fileEntry, metadata)
        )}
      </div>

      <script>
        const vscode = acquireVsCodeApi();

        // Restore collapsed state from localStorage
        const state = vscode.getState() || { collapsed: {} };

        document.querySelectorAll('.card').forEach(card => {
          const id = card.dataset.sectionId;
          if (state.collapsed[id]) {
            card.classList.add('collapsed');
          }
        });

        function sendMessage(command, data) {
          vscode.postMessage({ command, data });
        }

        function toggleSection(sectionId) {
          const card = document.querySelector(\`[data-section-id="\${sectionId}"]\`);
          if (card) {
            card.classList.toggle('collapsed');
            // Save state
            state.collapsed[sectionId] = card.classList.contains('collapsed');
            vscode.setState(state);
          }
        }

        function copyValue(value) {
          sendMessage('copyToClipboard', value);
        }

        // Handle feedback with animation
        function handleFeedback(command, data, button) {
          const item = button.closest('.suggestion-item');
          if (!item) {
            sendMessage(command, data);
            return;
          }

          // Disable buttons to prevent double-click
          const buttons = item.querySelectorAll('button');
          buttons.forEach(btn => btn.disabled = true);

          // Add animation class
          const isAccept = command.includes('accept');
          item.classList.add(isAccept ? 'accepting' : 'rejecting');

          // Send message after short delay to show animation
          setTimeout(() => {
            sendMessage(command, data);
          }, isAccept ? 300 : 200);
        }

        // Notes editing
        let notesTimeout = null;
        const notesTextarea = document.getElementById('notes-textarea');
        if (notesTextarea) {
          notesTextarea.addEventListener('input', (e) => {
            clearTimeout(notesTimeout);
            notesTimeout = setTimeout(() => {
              sendMessage('saveNotes', e.target.value);
            }, 1000); // Auto-save after 1 second of inactivity
          });
        }
      </script>
    </body>
    </html>`;
  }

  private getEmptyStateHtml(): string {
    return `<!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <style>
        body {
          font-family: var(--vscode-font-family);
          color: var(--vscode-foreground);
          background: var(--vscode-editor-background);
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          height: 100vh;
          margin: 0;
          text-align: center;
        }
        .empty-icon { font-size: 48px; opacity: 0.5; margin-bottom: 16px; }
        .empty-title { font-size: 16px; margin-bottom: 8px; }
        .empty-hint { font-size: 12px; color: var(--vscode-descriptionForeground); }
      </style>
    </head>
    <body>
      <div class="empty-icon">📄</div>
      <div class="empty-title">No file selected</div>
      <div class="empty-hint">Open a file to see its information</div>
    </body>
    </html>`;
  }

  private getFileNotFoundHtml(): string {
    const filename = this.currentFile ? path.basename(this.currentFile) : 'Unknown';
    const relativePath = this.currentFile || '';

    return `<!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>File Not Found</title>
      <style>
        ${this.getStyles()}
        body {
          font-family: var(--vscode-font-family);
          color: var(--vscode-foreground);
          background: var(--vscode-editor-background);
          padding: 24px;
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          min-height: 100vh;
          text-align: center;
        }
        .not-found-icon {
          font-size: 64px;
          opacity: 0.6;
          margin-bottom: 16px;
        }
        .not-found-title {
          font-size: 20px;
          font-weight: 600;
          margin-bottom: 8px;
          color: var(--vscode-errorForeground);
        }
        .not-found-filename {
          font-size: 16px;
          font-family: var(--vscode-editor-font-family);
          color: var(--vscode-descriptionForeground);
          margin-bottom: 16px;
          word-break: break-all;
        }
        .not-found-message {
          font-size: 14px;
          color: var(--vscode-descriptionForeground);
          max-width: 500px;
          line-height: 1.5;
          margin-bottom: 24px;
        }
        .not-found-actions {
          display: flex;
          gap: 12px;
          flex-wrap: wrap;
          justify-content: center;
        }
        .action-btn {
          padding: 8px 16px;
          border: 1px solid var(--vscode-button-border);
          background: var(--vscode-button-background);
          color: var(--vscode-button-foreground);
          border-radius: 4px;
          cursor: pointer;
          font-size: 13px;
          transition: opacity 0.2s;
        }
        .action-btn:hover {
          opacity: 0.8;
        }
        .action-btn.secondary {
          background: transparent;
          border-color: var(--vscode-button-secondaryBackground);
          color: var(--vscode-button-secondaryForeground);
        }
      </style>
    </head>
    <body>
      <div class="not-found-icon">📄❌</div>
      <div class="not-found-title">File Not Found in Backend</div>
      <div class="not-found-filename" title="${this.escapeHtml(relativePath)}">${this.escapeHtml(filename)}</div>
      <div class="not-found-message">
        This file is not indexed in the backend. It may have been deleted, moved, or not yet scanned.
        <br><br>
        Try refreshing the workspace index or check if the file exists in the workspace.
      </div>
      <div class="not-found-actions">
        <button class="action-btn" onclick="sendMessage('refresh')">🔄 Refresh</button>
        <button class="action-btn secondary" onclick="sendMessage('rebuildIndex')">🔨 Rebuild Index</button>
      </div>
    </body>
    </html>`;
  }

  private getLoadingStateHtml(): string {
    const filename = this.currentFile ? path.basename(this.currentFile) : 'Loading...';

    return `<!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="UTF-8">
      <style>
        ${this.getStyles()}

        @keyframes shimmer {
          0% { background-position: -200% 0; }
          100% { background-position: 200% 0; }
        }

        .skeleton {
          background: linear-gradient(90deg,
            var(--vscode-editor-background) 25%,
            var(--vscode-list-hoverBackground) 50%,
            var(--vscode-editor-background) 75%);
          background-size: 200% 100%;
          animation: shimmer 1.5s infinite;
          border-radius: 4px;
        }

        .skeleton-text { height: 14px; margin: 8px 0; }
        .skeleton-title { height: 20px; width: 60%; margin: 8px 0; }
        .skeleton-badge { height: 20px; width: 80px; border-radius: 10px; }
        .skeleton-card { height: 100px; margin: 12px 0; }
      </style>
    </head>
    <body>
      <div class="hero">
        <div class="hero-icon">⏳</div>
        <div class="hero-info">
          <h1 class="hero-title">${this.escapeHtml(filename)}</h1>
          <div class="hero-meta">
            <span class="skeleton skeleton-badge"></span>
          </div>
        </div>
      </div>

      <div class="quick-actions">
        <div class="skeleton" style="width: 100px; height: 36px; border-radius: 6px;"></div>
        <div class="skeleton" style="width: 120px; height: 36px; border-radius: 6px;"></div>
        <div class="skeleton" style="width: 100px; height: 36px; border-radius: 6px;"></div>
      </div>

      <div class="sections">
        <div class="card">
          <div class="card-header">
            <div class="skeleton skeleton-text" style="width: 100px;"></div>
          </div>
          <div class="card-content">
            <div class="skeleton skeleton-text" style="width: 80%;"></div>
            <div class="skeleton skeleton-text" style="width: 60%;"></div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <div class="skeleton skeleton-text" style="width: 80px;"></div>
          </div>
          <div class="card-content">
            <div class="skeleton skeleton-text" style="width: 70%;"></div>
            <div class="skeleton skeleton-text" style="width: 50%;"></div>
          </div>
        </div>
      </div>
    </body>
    </html>`;
  }

  private renderCollapsibleCard(id: string, title: string, count: number, content: string, badgeType?: string): string {
    const countBadge = count > 0 ? `<span class="count-badge">${count}</span>` : '';
    const glowBadge = badgeType === 'glow' ? '<span class="glow-badge">NEW</span>' : '';

    return `
      <div class="card" data-section-id="${id}">
        <div class="card-header" onclick="toggleSection('${id}')">
          <div class="card-header-left">
            <span class="collapse-icon">▼</span>
            <h2>${title}</h2>
          </div>
          <div class="card-header-right">
            ${glowBadge}
            ${countBadge}
          </div>
        </div>
        <div class="card-content">
          ${content}
        </div>
      </div>
    `;
  }

  private getStyles(): string {
    return `
      :root {
        --card-bg: var(--vscode-editor-background);
        --card-border: var(--vscode-panel-border);
        --card-hover: var(--vscode-list-hoverBackground);
        --accent: var(--vscode-textLink-foreground);
        --accent-dim: var(--vscode-textLink-activeForeground);
        --success: #4caf50;
        --warning: #ff9800;
        --danger: #f44336;
        --radius: 8px;
        --gap: 12px;
      }

      * { box-sizing: border-box; }

      body {
        font-family: var(--vscode-font-family);
        font-size: 13px;
        color: var(--vscode-foreground);
        background: var(--vscode-editor-background);
        margin: 0;
        padding: 16px;
        line-height: 1.5;
      }

      /* Hero Section */
      .hero {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 16px;
        background: linear-gradient(135deg,
          var(--vscode-sideBar-background) 0%,
          var(--vscode-editor-background) 100%);
        border-radius: var(--radius);
        margin-bottom: 16px;
        border: 1px solid var(--card-border);
      }

      .hero-icon { font-size: 40px; line-height: 1; }
      .hero-info { flex: 1; min-width: 0; }

      .hero-title {
        font-size: 18px;
        font-weight: 600;
        margin: 0 0 6px 0;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .hero-meta {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 12px;
        color: var(--vscode-descriptionForeground);
      }

      .meta-separator { opacity: 0.5; }

      .badge {
        padding: 2px 8px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
      }

      .badge-type {
        background: var(--accent);
        color: var(--vscode-editor-background);
      }

      .refresh-btn {
        background: none;
        border: none;
        font-size: 16px;
        cursor: pointer;
        opacity: 0.6;
        transition: opacity 0.2s, transform 0.2s;
        padding: 8px;
      }

      .refresh-btn:hover {
        opacity: 1;
        transform: rotate(180deg);
      }

      /* Quick Actions */
      .quick-actions {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-bottom: 16px;
      }

      .action-btn {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 8px 12px;
        background: var(--vscode-button-secondaryBackground);
        color: var(--vscode-button-secondaryForeground);
        border: none;
        border-radius: 6px;
        font-size: 12px;
        cursor: pointer;
        transition: all 0.15s ease;
      }

      .action-btn:hover {
        background: var(--vscode-button-secondaryHoverBackground);
        transform: translateY(-1px);
      }

      .action-btn.primary {
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
      }

      .action-btn.primary:hover {
        background: var(--vscode-button-hoverBackground);
      }

      .action-btn .icon { font-size: 14px; }

      /* Sections */
      .sections {
        display: flex;
        flex-direction: column;
        gap: var(--gap);
      }

      /* Cards */
      .card {
        background: var(--card-bg);
        border: 1px solid var(--card-border);
        border-radius: var(--radius);
        overflow: hidden;
        transition: all 0.2s ease;
      }

      .card.collapsed .card-content {
        display: none;
      }

      .card.collapsed .collapse-icon {
        transform: rotate(-90deg);
      }

      .card-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 16px;
        border-bottom: 1px solid var(--card-border);
        background: rgba(255,255,255,0.02);
        cursor: pointer;
        user-select: none;
      }

      .card-header:hover {
        background: rgba(255,255,255,0.04);
      }

      .card-header-left {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .card-header-right {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .collapse-icon {
        font-size: 10px;
        transition: transform 0.2s ease;
        opacity: 0.6;
      }

      .card-header h2 {
        font-size: 13px;
        font-weight: 600;
        margin: 0;
      }

      .count-badge {
        background: var(--vscode-badge-background);
        color: var(--vscode-badge-foreground);
        padding: 2px 8px;
        border-radius: 10px;
        font-size: 11px;
        font-weight: 500;
      }

      .glow-badge {
        background: linear-gradient(135deg, #7c3aed, #a855f7);
        color: white;
        padding: 2px 8px;
        border-radius: 10px;
        font-size: 10px;
        font-weight: 600;
        animation: glow 2s ease-in-out infinite alternate;
      }

      @keyframes glow {
        from { box-shadow: 0 0 4px rgba(124, 58, 237, 0.4); }
        to { box-shadow: 0 0 12px rgba(168, 85, 247, 0.6); }
      }

      .card-content {
        padding: 16px;
      }

      .empty-state {
        color: var(--vscode-descriptionForeground);
        font-style: italic;
        text-align: center;
        padding: 16px;
        margin: 0;
      }

      /* Project Cards */
      .project-list { display: flex; flex-direction: column; gap: 8px; }

      .project-item {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 12px;
        background: var(--vscode-list-hoverBackground);
        border-radius: 6px;
        border-left: 3px solid var(--accent);
      }

      .project-icon { font-size: 20px; line-height: 1; }
      .project-info { flex: 1; min-width: 0; }
      .project-name { font-weight: 500; margin-bottom: 2px; }
      .project-meta { font-size: 11px; color: var(--vscode-descriptionForeground); }

      /* Interactive Tags */
      .tag-cloud {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
      }

      .tag-chip {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        background: var(--vscode-badge-background);
        color: var(--vscode-badge-foreground);
        border-radius: 14px;
        font-size: 12px;
        transition: all 0.15s ease;
      }

      .tag-chip:hover {
        transform: scale(1.02);
        box-shadow: 0 2px 8px rgba(0,0,0,0.2);
      }

      .tag-chip.large {
        font-size: 14px;
        padding: 6px 14px;
        font-weight: 500;
      }

      .tag-chip.medium {
        font-size: 13px;
        padding: 5px 12px;
      }

      .tag-remove {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 16px;
        height: 16px;
        border-radius: 50%;
        background: rgba(255,255,255,0.2);
        border: none;
        color: inherit;
        font-size: 10px;
        cursor: pointer;
        opacity: 0.7;
        transition: all 0.15s ease;
      }

      .tag-remove:hover {
        opacity: 1;
        background: var(--danger);
        color: white;
      }

      /* Notes Section */
      .notes-textarea {
        width: 100%;
        min-height: 80px;
        padding: 10px;
        background: var(--vscode-input-background);
        color: var(--vscode-input-foreground);
        border: 1px solid var(--vscode-input-border);
        border-radius: 6px;
        font-family: inherit;
        font-size: 12px;
        resize: vertical;
        line-height: 1.5;
      }

      .notes-textarea:focus {
        outline: none;
        border-color: var(--accent);
      }

      .notes-hint {
        font-size: 10px;
        color: var(--vscode-descriptionForeground);
        margin-top: 6px;
      }

      /* Similar Files */
      .similar-list { display: flex; flex-direction: column; gap: 6px; }

      .similar-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 10px;
        border-radius: 6px;
        cursor: pointer;
        transition: background 0.15s ease;
      }

      .similar-item:hover { background: var(--card-hover); }

      .similar-icon { font-size: 16px; opacity: 0.7; }
      .similar-info { flex: 1; min-width: 0; }

      .similar-name {
        font-size: 12px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      }

      .similar-reason {
        font-size: 10px;
        color: var(--vscode-descriptionForeground);
        margin-top: 2px;
      }

      .similarity-bar {
        width: 60px;
        height: 4px;
        background: var(--card-border);
        border-radius: 2px;
        overflow: hidden;
      }

      .similarity-fill {
        height: 100%;
        background: var(--success);
        border-radius: 2px;
      }

      /* Suggestions */
      .suggestions-section { margin-bottom: 16px; }
      .suggestions-section:last-child { margin-bottom: 0; }

      .suggestions-title {
        font-size: 11px;
        font-weight: 600;
        color: var(--vscode-descriptionForeground);
        margin-bottom: 8px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .suggestion-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px;
        background: rgba(124, 58, 237, 0.08);
        border-radius: 6px;
        margin-bottom: 6px;
        border: 1px solid rgba(124, 58, 237, 0.2);
      }

      .suggestion-content { flex: 1; }
      .suggestion-name { font-weight: 500; margin-bottom: 2px; }
      .suggestion-confidence { font-size: 10px; color: var(--vscode-descriptionForeground); }
      .suggestion-actions { display: flex; gap: 4px; }

      .suggestion-btn {
        padding: 4px 8px;
        border: none;
        border-radius: 4px;
        font-size: 11px;
        cursor: pointer;
      }

      .suggestion-btn.accept { background: var(--success); color: white; }
      .suggestion-btn.accept:hover { background: #45a049; transform: scale(1.05); }
      .suggestion-btn.reject { background: var(--vscode-button-secondaryBackground); color: var(--vscode-button-secondaryForeground); }
      .suggestion-btn.reject:hover { background: var(--danger); color: white; transform: scale(1.05); }

      .confidence-boost {
        display: inline-block;
        padding: 1px 6px;
        margin-left: 6px;
        background: linear-gradient(135deg, rgba(76, 175, 80, 0.2), rgba(76, 175, 80, 0.1));
        color: var(--success);
        border-radius: 8px;
        font-size: 9px;
        font-weight: 600;
      }

      /* Feedback Stats Banner */
      .feedback-stats-banner {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        background: linear-gradient(135deg, rgba(124, 58, 237, 0.1), rgba(168, 85, 247, 0.05));
        border: 1px solid rgba(124, 58, 237, 0.2);
        border-radius: var(--radius);
        margin-bottom: 12px;
      }

      .feedback-stats-icon { font-size: 24px; }
      .feedback-stats-content { flex: 1; }
      .feedback-stats-title { font-weight: 600; font-size: 13px; margin-bottom: 2px; }
      .feedback-stats-detail { font-size: 11px; color: var(--vscode-descriptionForeground); }

      .feedback-rate-circle {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        background: conic-gradient(var(--success) var(--rate, 0%), var(--card-border) 0%);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 10px;
        font-weight: 600;
        position: relative;
      }

      .feedback-rate-circle::before {
        content: '';
        position: absolute;
        width: 30px;
        height: 30px;
        border-radius: 50%;
        background: var(--vscode-editor-background);
      }

      .feedback-rate-circle span {
        position: relative;
        z-index: 1;
      }

      /* Feedback Animations */
      @keyframes feedbackAccept {
        0% { transform: scale(1); background: rgba(124, 58, 237, 0.08); }
        50% { transform: scale(1.02); background: rgba(76, 175, 80, 0.3); }
        100% { transform: scale(0); opacity: 0; height: 0; padding: 0; margin: 0; }
      }

      @keyframes feedbackReject {
        0% { transform: scale(1); background: rgba(124, 58, 237, 0.08); }
        50% { transform: scale(0.98); background: rgba(244, 67, 54, 0.2); }
        100% { transform: scale(0); opacity: 0; height: 0; padding: 0; margin: 0; }
      }

      .suggestion-item.accepting {
        animation: feedbackAccept 0.5s ease-out forwards;
      }

      .suggestion-item.rejecting {
        animation: feedbackReject 0.4s ease-out forwards;
      }

      .suggestion-item.accepted::after {
        content: '✓ Accepted';
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        color: var(--success);
        font-weight: 600;
      }

      /* Summary */
      .summary-text {
        line-height: 1.6;
        margin: 0 0 12px 0;
      }

      .key-terms { margin-top: 12px; }

      .term-chip {
        display: inline-block;
        padding: 2px 8px;
        margin: 4px 4px 0 0;
        background: var(--vscode-textBlockQuote-background);
        border-radius: 4px;
        font-size: 11px;
      }

      /* Technical Metadata Grid */
      .metadata-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
        gap: 12px;
      }

      .metadata-item {
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .metadata-label {
        font-size: 10px;
        color: var(--vscode-descriptionForeground);
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .metadata-value-row {
        display: flex;
        align-items: center;
        gap: 6px;
      }

      .metadata-value {
        font-size: 12px;
        font-weight: 500;
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .copy-btn {
        background: none;
        border: none;
        font-size: 12px;
        cursor: pointer;
        opacity: 0;
        transition: opacity 0.15s;
        padding: 2px;
      }

      .metadata-item:hover .copy-btn {
        opacity: 0.6;
      }

      .copy-btn:hover {
        opacity: 1 !important;
      }

      /* Pipeline Status */
      .pipeline-status {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid var(--card-border);
      }

      .pipeline-stage {
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 2px 8px;
        background: var(--vscode-badge-background);
        border-radius: 4px;
        font-size: 10px;
      }

      .pipeline-stage.complete { background: rgba(76, 175, 80, 0.2); color: var(--success); }
      .pipeline-stage.pending { opacity: 0.5; }
    `;
  }

  private renderProjectCards(projects: ProjectInfo[]): string {
    return `
      <div class="project-list">
        ${projects.map(p => `
          <div class="project-item" style="border-left-color: ${p.color}">
            <div class="project-icon">📁</div>
            <div class="project-info">
              <div class="project-name">${this.escapeHtml(p.name)}</div>
              ${p.description ? `<div class="project-meta">${this.escapeHtml(p.description)}</div>` : ''}
            </div>
          </div>
        `).join('')}
      </div>
    `;
  }

  private renderInteractiveTags(tags: string[]): string {
    const sortedTags = [...tags].sort((a, b) => b.length - a.length);

    return `
      <div class="tag-cloud">
        ${sortedTags.map((tag, i) => {
          const size = i < 2 ? 'large' : i < 5 ? 'medium' : '';
          return `
            <span class="tag-chip ${size}">
              🏷️ ${this.escapeHtml(tag)}
              <button class="tag-remove" onclick="event.stopPropagation(); sendMessage('removeTag', '${this.escapeHtml(tag)}')" title="Remove tag">×</button>
            </span>
          `;
        }).join('')}
      </div>
    `;
  }

  private renderNotesSection(notes: string): string {
    return `
      <textarea
        id="notes-textarea"
        class="notes-textarea"
        placeholder="Add notes about this file..."
      >${this.escapeHtml(notes)}</textarea>
      <div class="notes-hint">Auto-saves after you stop typing</div>
    `;
  }

  private renderFeedbackStatsIndicator(): string {
    const stats = this.cachedFeedbackStats;
    if (!stats || stats.preferencesLearned === 0) return '';

    const acceptancePercent = stats.totalFeedback > 0
      ? Math.round(stats.acceptanceRate * 100)
      : 0;

    return `
      <div class="feedback-stats-banner">
        <div class="feedback-stats-icon">🧠</div>
        <div class="feedback-stats-content">
          <div class="feedback-stats-title">Learning from your feedback</div>
          <div class="feedback-stats-detail">
            ${stats.preferencesLearned} preference${stats.preferencesLearned === 1 ? '' : 's'} learned
            ${stats.avgConfidenceBoost > 0 ? ` • +${Math.round(stats.avgConfidenceBoost * 100)}% avg boost` : ''}
          </div>
        </div>
        <div class="feedback-stats-rate">
          <div class="feedback-rate-circle" style="--rate: ${acceptancePercent}%">
            <span>${acceptancePercent}%</span>
          </div>
        </div>
      </div>
    `;
  }

  private renderSuggestions(suggestions: SuggestedMetadata | null): string {
    if (!suggestions) return '<p class="empty-state">No suggestions available</p>';

    const suggestedTags = suggestions.suggestedTags || [];
    const suggestedProjects = suggestions.suggestedProjects || [];

    let html = '';

    if (suggestedTags.length > 0) {
      html += `
        <div class="suggestions-section">
          <div class="suggestions-title">Suggested Tags</div>
          ${suggestedTags.slice(0, 5).map(tag => {
            const tagData = JSON.stringify({ tag: tag.tag, confidence: tag.confidence }).replaceAll('"', '&quot;');
            const boostInfo = this.getConfidenceBoostInfo(tag.confidence);
            return `
            <div class="suggestion-item" id="suggestion-tag-${this.escapeHtml(tag.tag)}">
              <div class="suggestion-content">
                <div class="suggestion-name">🏷️ ${this.escapeHtml(tag.tag)}</div>
                <div class="suggestion-confidence">
                  ${(tag.confidence * 100).toFixed(0)}% confidence
                  ${boostInfo ? `<span class="confidence-boost">+${boostInfo}</span>` : ''}
                  ${tag.reason ? ` • ${this.escapeHtml(tag.reason)}` : ''}
                </div>
              </div>
              <div class="suggestion-actions">
                <button class="suggestion-btn accept" onclick="handleFeedback('acceptTag', ${tagData}, this)">✓</button>
                <button class="suggestion-btn reject" onclick="handleFeedback('rejectTag', ${tagData}, this)">✗</button>
              </div>
            </div>
          `;}).join('')}
        </div>
      `;
    }

    if (suggestedProjects.length > 0) {
      html += `
        <div class="suggestions-section">
          <div class="suggestions-title">Suggested Projects</div>
          ${suggestedProjects.slice(0, 3).map(proj => {
            const projData = JSON.stringify({ projectName: proj.projectName, confidence: proj.confidence }).replaceAll('"', '&quot;');
            const boostInfo = this.getConfidenceBoostInfo(proj.confidence);
            return `
            <div class="suggestion-item" id="suggestion-project-${this.escapeHtml(proj.projectName)}">
              <div class="suggestion-content">
                <div class="suggestion-name">📁 ${this.escapeHtml(proj.projectName)}${proj.isNew ? ' <span style="color: var(--warning)">NEW</span>' : ''}</div>
                <div class="suggestion-confidence">
                  ${(proj.confidence * 100).toFixed(0)}% confidence
                  ${boostInfo ? `<span class="confidence-boost">+${boostInfo}</span>` : ''}
                  ${proj.reason ? ` • ${this.escapeHtml(proj.reason)}` : ''}
                </div>
              </div>
              <div class="suggestion-actions">
                <button class="suggestion-btn accept" onclick="handleFeedback('acceptProject', ${projData}, this)">✓</button>
                <button class="suggestion-btn reject" onclick="handleFeedback('rejectProject', ${projData}, this)">✗</button>
              </div>
            </div>
          `;}).join('')}
        </div>
      `;
    }

    return html || '<p class="empty-state">No suggestions available</p>';
  }

  private getConfidenceBoostInfo(confidence: number): string | null {
    // Check if preferences have boosted the confidence
    // This would normally come from the applyPreferences response
    // For now, we show a boost indicator if confidence is unusually high
    if (confidence > 0.85) {
      return 'from preferences';
    }
    return null;
  }

  private renderSimilarFiles(files: SimilarFile[]): string {
    return `
      <div class="similar-list">
        ${files.map(file => {
          const filename = path.basename(file.relativePath);
          const ext = path.extname(file.relativePath);
          const reason = file.reason ||
            (file.sharedTags?.length ? `Tags: ${file.sharedTags.join(', ')}` : '') ||
            (file.sharedProjects?.length ? `Projects: ${file.sharedProjects.join(', ')}` : '');

          return `
            <div class="similar-item" onclick="sendMessage('openFile', '${this.escapeHtml(file.relativePath)}')">
              <div class="similar-icon">${this.getFileIcon(ext)}</div>
              <div class="similar-info">
                <div class="similar-name">${this.escapeHtml(filename)}</div>
                ${reason ? `<div class="similar-reason">${this.escapeHtml(reason)}</div>` : ''}
              </div>
              <div class="similarity-bar">
                <div class="similarity-fill" style="width: ${file.similarity * 100}%"></div>
              </div>
            </div>
          `;
        }).join('')}
      </div>
    `;
  }

  private renderAISummary(metadata: FileMetadata): string {
    return `
      <p class="summary-text">${this.escapeHtml(metadata.aiSummary || '')}</p>
      ${metadata.aiKeyTerms?.length ? `
      <div class="key-terms">
        <strong>Key Terms:</strong>
        ${metadata.aiKeyTerms.map(t => `<span class="term-chip">${this.escapeHtml(t)}</span>`).join('')}
      </div>
      ` : ''}
    `;
  }

  private renderTechnicalMetadata(fileEntry: FileEntry | null, metadata: FileMetadata | null): string {
    const items: Array<{ label: string; value: string; copyable?: boolean }> = [];

    if (this.currentFile) {
      items.push({ label: 'Path', value: this.currentFile, copyable: true });
    }

    if (fileEntry?.enhanced?.mimeType || fileEntry?.enhanced?.mime_type?.mime_type) {
      const mime = fileEntry.enhanced.mimeType || fileEntry.enhanced.mime_type?.mime_type;
      items.push({ label: 'MIME Type', value: mime || '', copyable: true });
    }

    if (fileEntry?.enhanced?.language) {
      items.push({ label: 'Language', value: fileEntry.enhanced.language });
    }

    if (fileEntry?.enhanced?.folder) {
      items.push({ label: 'Folder', value: fileEntry.enhanced.folder, copyable: true });
    }

    if (metadata?.created_at) {
      items.push({ label: 'Indexed', value: this.formatDate(metadata.created_at * 1000) });
    }

    if (metadata?.updated_at) {
      items.push({ label: 'Updated', value: this.formatDate(metadata.updated_at * 1000) });
    }

    // Document metrics
    const docMetrics = fileEntry?.enhanced?.document_metrics;
    if (docMetrics) {
      if (typeof docMetrics.page_count === 'number') {
        items.push({ label: 'Pages', value: String(docMetrics.page_count) });
      }
      if (typeof docMetrics.word_count === 'number') {
        items.push({ label: 'Words', value: this.formatNumber(docMetrics.word_count as number) });
      }
      if (typeof docMetrics.author === 'string' && docMetrics.author) {
        items.push({ label: 'Author', value: docMetrics.author as string, copyable: true });
      }
    }

    // Pipeline status
    const indexed = fileEntry?.enhanced?.indexed;
    let pipelineHtml = '';
    if (indexed) {
      const stages = [
        { key: 'basic', label: 'Basic' },
        { key: 'mime', label: 'MIME' },
        { key: 'code', label: 'Code' },
        { key: 'document', label: 'Doc' },
        { key: 'mirror', label: 'Mirror' },
        { key: 'enrichment', label: 'AI' },
      ];

      pipelineHtml = `
        <div class="pipeline-status">
          ${stages.map(s => {
            const isComplete = indexed[s.key as keyof typeof indexed] === true;
            return `<span class="pipeline-stage ${isComplete ? 'complete' : 'pending'}">${isComplete ? '✓' : '○'} ${s.label}</span>`;
          }).join('')}
        </div>
      `;
    }

    return `
      <div class="metadata-grid">
        ${items.map(item => `
          <div class="metadata-item">
            <span class="metadata-label">${item.label}</span>
            <div class="metadata-value-row">
              <span class="metadata-value" title="${this.escapeHtml(item.value)}">${this.escapeHtml(this.truncate(item.value, 25))}</span>
              ${item.copyable ? `<button class="copy-btn" onclick="copyValue('${this.escapeHtml(item.value)}')" title="Copy">📋</button>` : ''}
            </div>
          </div>
        `).join('')}
      </div>
      ${pipelineHtml}
    `;
  }

  private getFileIcon(extension: string): string {
    const icons: Record<string, string> = {
      '.ts': '📘', '.tsx': '📘', '.js': '📒', '.jsx': '📒',
      '.json': '📋', '.md': '📝', '.txt': '📄', '.pdf': '📕',
      '.doc': '📘', '.docx': '📘', '.xls': '📗', '.xlsx': '📗',
      '.ppt': '📙', '.pptx': '📙', '.png': '🖼️', '.jpg': '🖼️',
      '.jpeg': '🖼️', '.gif': '🖼️', '.svg': '🎨', '.mp3': '🎵',
      '.wav': '🎵', '.mp4': '🎬', '.mov': '🎬', '.zip': '📦',
      '.tar': '📦', '.gz': '📦', '.py': '🐍', '.go': '🔵',
      '.rs': '🦀', '.java': '☕', '.c': '📟', '.cpp': '📟',
      '.h': '📟', '.html': '🌐', '.css': '🎨', '.scss': '🎨',
      '.sql': '🗃️', '.yml': '⚙️', '.yaml': '⚙️', '.toml': '⚙️',
      '.env': '🔐', '.sh': '⌨️', '.bash': '⌨️',
    };
    return icons[extension.toLowerCase()] || '📄';
  }

  private getProjectColor(name: string): string {
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
      hash = name.codePointAt(i)! + ((hash << 5) - hash);
    }
    const hue = Math.abs(hash % 360);
    return `hsl(${hue}, 60%, 50%)`;
  }

  private formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${Number.parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }

  private formatDate(timestamp: number): string {
    if (!timestamp) return 'Unknown';
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    if (diff < 60000) return 'Just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`;

    return date.toLocaleDateString();
  }

  private formatNumber(num: number): string {
    if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
    if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
    return String(num);
  }

  private truncate(str: string, length: number): string {
    if (str.length <= length) return str;
    return str.substring(0, length) + '...';
  }

  private escapeHtml(str: string): string {
    return str
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
  }
}
