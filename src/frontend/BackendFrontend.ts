import * as vscode from "vscode";
import { GrpcAdminClient } from "../core/GrpcAdminClient";
import { GrpcMetadataClient } from "../core/GrpcMetadataClient";
import { GrpcLLMClient } from "../core/GrpcLLMClient";
import { GrpcKnowledgeClient } from "../core/GrpcKnowledgeClient";

type WebviewMessage = {
  type: string;
  [key: string]: unknown;
};

export class BackendFrontend {
  private static currentPanel: BackendFrontend | undefined;

  private readonly panel: vscode.WebviewPanel;
  private readonly disposables: vscode.Disposable[] = [];
  private readonly adminClient: GrpcAdminClient;
  private readonly metadataClient: GrpcMetadataClient;
  private readonly llmClient: GrpcLLMClient;
  private readonly knowledgeClient: GrpcKnowledgeClient;
  private stopPipelineStream?: () => void;
  private activeWorkspaceId?: string;

  static show(context: vscode.ExtensionContext): void {
    if (BackendFrontend.currentPanel) {
      BackendFrontend.currentPanel.panel.reveal(vscode.ViewColumn.One);
      BackendFrontend.currentPanel.refresh();
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      "cortexBackendFrontend",
      "Cortex Backend",
      vscode.ViewColumn.One,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
        localResourceRoots: [
          vscode.Uri.joinPath(context.extensionUri, "resources"),
        ],
      }
    );

    BackendFrontend.currentPanel = new BackendFrontend(panel, context);
    void BackendFrontend.currentPanel.initialize();
  }

  private constructor(
    panel: vscode.WebviewPanel,
    private context: vscode.ExtensionContext
  ) {
    this.panel = panel;
    this.adminClient = new GrpcAdminClient(context);
    this.metadataClient = new GrpcMetadataClient(context);
    this.llmClient = new GrpcLLMClient(context);
    this.knowledgeClient = new GrpcKnowledgeClient(context);

    this.panel.webview.html = this.getHtml();
    this.panel.onDidDispose(() => this.dispose(), null, this.disposables);
    this.panel.webview.onDidReceiveMessage(
      (msg) => this.handleMessage(msg),
      null,
      this.disposables
    );
  }

  private async initialize(): Promise<void> {
    this.startPipelineStream();
    await this.refresh();
  }

  private dispose(): void {
    BackendFrontend.currentPanel = undefined;
    while (this.disposables.length) {
      const item = this.disposables.pop();
      item?.dispose();
    }
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
      this.stopPipelineStream = undefined;
    }
  }

  private async refresh(): Promise<void> {
    this.panel.webview.postMessage({ type: "loading", value: true });
    try {
      // Load all data in parallel
      const [snapshot, tags, contexts, providers] = await Promise.all([
        this.adminClient.getSnapshot(),
        this.getTags(),
        this.getContexts(),
        this.getLLMProviders(),
      ]);

      this.panel.webview.postMessage({
        type: "data",
        data: {
          snapshot,
          tags,
          contexts,
          providers,
        },
      });

      // Set active workspace
      const firstWorkspace = snapshot.workspaces?.[0];
      if (firstWorkspace?.id) {
        this.activeWorkspaceId = firstWorkspace.id;
        this.panel.webview.postMessage({
          type: "activeWorkspace",
          workspaceId: firstWorkspace.id,
        });
      }
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.panel.webview.postMessage({ type: "loading", value: false });
    }
  }

  private async getTags(): Promise<{ tags: string[]; counts: Record<string, number> }> {
    if (!this.activeWorkspaceId) {
      return { tags: [], counts: {} };
    }
    try {
      const [tags, countsMap] = await Promise.all([
        this.metadataClient.getAllTags(this.activeWorkspaceId),
        this.metadataClient.getTagCounts(this.activeWorkspaceId),
      ]);
      // Convert Map to plain object for serialization
      const counts: Record<string, number> = {};
      countsMap.forEach((value, key) => {
        counts[key] = value;
      });
      return { tags, counts };
    } catch {
      return { tags: [], counts: {} };
    }
  }

  private async getContexts(): Promise<string[]> {
    if (!this.activeWorkspaceId) {
      return [];
    }
    try {
      // Use new Projects system
      const projects = await this.knowledgeClient.listProjects(this.activeWorkspaceId);
      return projects.map(p => p.name);
    } catch {
      return [];
    }
  }

  private async getLLMProviders(): Promise<any[]> {
    try {
      return await this.llmClient.listProviders();
    } catch {
      return [];
    }
  }

  private handleMessage(message: WebviewMessage | null | undefined): void {
    if (!message || typeof message.type !== "string") {
      return;
    }

    switch (message.type) {
      case "refresh":
        void this.refresh();
        break;

      case "reindex_workspace":
        this.handleReindexWorkspace(message);
        break;

      case "add_tag":
        this.handleAddTag(message);
        break;

      case "remove_tag":
        this.handleRemoveTag(message);
        break;

      case "add_context":
        this.handleAddContext(message);
        break;

      case "remove_context":
        this.handleRemoveContext(message);
        break;

      case "process_file":
        this.handleProcessFile(message);
        break;

      case "get_file_metadata":
        this.handleGetFileMetadata(message);
        break;

      case "list_files_by_tag":
        this.handleListFilesByTag(message);
        break;

      case "list_files_by_context":
        this.handleListFilesByContext(message);
        break;

      case "suggest_tags":
        this.handleSuggestTags(message);
        break;

      case "generate_summary":
        this.handleGenerateSummary(message);
        break;

      case "llm_completion":
        this.handleLLMCompletion(message);
        break;

      case "set_active_workspace":
        this.activeWorkspaceId = message.workspaceId as string;
        void this.refresh();
        break;

      case "load_config":
        this.handleLoadConfig();
        break;

      case "save_config":
        this.handleSaveConfig(message);
        break;

      case "save_prompts":
        this.handleSavePrompts(message);
        break;

      case "load_config_versions":
        this.handleLoadConfigVersions();
        break;

      case "restore_config_version":
        this.handleRestoreConfigVersion(message);
        break;
    }
  }

  private async handleLoadConfig(): Promise<void> {
    try {
      const config = await this.adminClient.getConfig();
      this.panel.webview.postMessage({
        type: "config_data",
        data: config,
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleSaveConfig(message: WebviewMessage): Promise<void> {
    try {
      const config = message.config as any;
      await this.adminClient.updateConfig(config, true);
      this.panel.webview.postMessage({
        type: "success",
        message: "Configuration saved successfully",
      });
      await this.handleLoadConfig();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleSavePrompts(message: WebviewMessage): Promise<void> {
    try {
      const prompts = message.prompts as any;
      const createVersion = (message.createVersion as boolean) ?? true;
      const description = (message.description as string) || "";
      await this.adminClient.updatePrompts(prompts, createVersion, description);
      this.panel.webview.postMessage({
        type: "success",
        message: "Prompts saved successfully",
      });
      await this.handleLoadConfig();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleLoadConfigVersions(): Promise<void> {
    try {
      const result = await this.adminClient.getConfigVersions(50, 0);
      this.panel.webview.postMessage({
        type: "config_versions",
        data: result,
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleRestoreConfigVersion(message: WebviewMessage): Promise<void> {
    try {
      const versionId = message.versionId as string;
      const createBackup = (message.createBackup as boolean) ?? true;
      const description = (message.description as string) || "";
      await this.adminClient.restoreConfigVersion(versionId, createBackup, description);
      this.panel.webview.postMessage({
        type: "success",
        message: "Configuration version restored successfully",
      });
      await this.handleLoadConfig();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleReindexWorkspace(message: WebviewMessage): Promise<void> {
    const workspaceId = (message.workspaceId as string) || this.activeWorkspaceId;
    const path = message.path as string;
    if (!workspaceId || !path) {
      return;
    }
    try {
      await this.adminClient.scanWorkspace(workspaceId, path, true);
      this.panel.webview.postMessage({ type: "success", message: "Reindex started" });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleAddTag(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    const tag = message.tag as string;
    if (!relativePath || !tag) return;

    try {
      await this.metadataClient.addTag(this.activeWorkspaceId, relativePath, tag);
      this.panel.webview.postMessage({ type: "success", message: `Tag "${tag}" added` });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleRemoveTag(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    const tag = message.tag as string;
    if (!relativePath || !tag) return;

    try {
      await this.metadataClient.removeTag(this.activeWorkspaceId, relativePath, tag);
      this.panel.webview.postMessage({ type: "success", message: `Tag "${tag}" removed` });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleAddContext(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    const projectName = message.context as string;
    if (!relativePath || !projectName) return;

    try {
      // Get or create project
      let project = await this.knowledgeClient.getProjectByName(this.activeWorkspaceId, projectName);
      if (!project) {
        project = await this.knowledgeClient.createProject(this.activeWorkspaceId, projectName);
      }

      // Get document ID
      const crypto = require('crypto');
      const normalized = relativePath.replace(/\\/g, '/');
      const documentId = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');

      // Associate document with project
      await this.knowledgeClient.addDocumentToProject(
        this.activeWorkspaceId,
        project.id,
        documentId
      );
      
      this.panel.webview.postMessage({
        type: "success",
        message: `Project "${projectName}" added to "${relativePath}"`,
      });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleRemoveContext(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    const projectName = message.context as string;
    if (!relativePath || !projectName) return;

    try {
      // Get project
      const project = await this.knowledgeClient.getProjectByName(this.activeWorkspaceId, projectName);
      if (!project) {
        this.panel.webview.postMessage({
          type: "error",
          message: `Project "${projectName}" not found`,
        });
        return;
      }

      // Get document ID
      const crypto = require('crypto');
      const normalized = relativePath.replace(/\\/g, '/');
      const documentId = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');

      // Remove document from project
      await this.knowledgeClient.removeDocumentFromProject(
        this.activeWorkspaceId,
        project.id,
        documentId
      );
      
      this.panel.webview.postMessage({
        type: "success",
        message: `Document removed from project "${projectName}"`,
      });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleProcessFile(message: WebviewMessage): Promise<void> {
    const workspaceId = (message.workspaceId as string) || this.activeWorkspaceId;
    const relativePath = message.relativePath as string;
    if (!workspaceId || !relativePath) return;

    try {
      await this.adminClient.processFile(workspaceId, relativePath);
      this.panel.webview.postMessage({ type: "success", message: "File processing started" });
      await this.refresh();
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleGetFileMetadata(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    if (!relativePath) return;

    try {
      const metadata = await this.metadataClient.getMetadataByPath(
        this.activeWorkspaceId,
        relativePath
      );
      this.panel.webview.postMessage({
        type: "file_metadata",
        data: metadata,
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleListFilesByTag(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const tag = message.tag as string;
    if (!tag) return;

    try {
      const files = await this.metadataClient.listByTag(this.activeWorkspaceId, tag);
      this.panel.webview.postMessage({
        type: "files_list",
        data: files,
        filter: { type: "tag", value: tag },
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleListFilesByContext(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const context = message.context as string;
    if (!context) return;

    try {
      const files = await this.metadataClient.listByContext(this.activeWorkspaceId, context);
      this.panel.webview.postMessage({
        type: "files_list",
        data: files,
        filter: { type: "context", value: context },
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleSuggestTags(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    if (!relativePath) return;

    try {
      // This would need to be implemented in GrpcLLMClient
      this.panel.webview.postMessage({
        type: "error",
        message: "Tag suggestions not yet implemented in LLM client",
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleGenerateSummary(message: WebviewMessage): Promise<void> {
    if (!this.activeWorkspaceId) return;
    const relativePath = message.relativePath as string;
    if (!relativePath) return;

    try {
      // This would need to be implemented in GrpcLLMClient
      this.panel.webview.postMessage({
        type: "error",
        message: "Summary generation not yet implemented in LLM client",
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private async handleLLMCompletion(message: WebviewMessage): Promise<void> {
    const prompt = message.prompt as string;
    if (!prompt) return;

    try {
      const result = await this.llmClient.generateCompletion({
        prompt,
        maxTokens: (message.maxTokens as number) || 500,
        temperature: (message.temperature as number) || 0.3,
      });
      this.panel.webview.postMessage({
        type: "llm_completion_result",
        data: result,
      });
    } catch (error: unknown) {
      this.panel.webview.postMessage({
        type: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private startPipelineStream(): void {
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
    }
    this.stopPipelineStream = this.adminClient.subscribePipelineEvents((event) => {
      this.panel.webview.postMessage({ type: "pipeline_event", data: event });
    });
  }

  private getHtml(): string {
    // This will be a comprehensive HTML UI - see next file
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Cortex Backend Frontend</title>
  <style>
    ${this.getStyles()}
  </style>
</head>
<body>
  ${this.getBody()}
  <script>
    ${this.getScript()}
  </script>
</body>
</html>`;
  }

  private getStyles(): string {
    return `
      :root {
        color-scheme: light dark;
        --bg: #0e1217;
        --panel: #151b22;
        --surface: #1c2128;
        --text: #e6edf3;
        --text-muted: #9da7b3;
        --accent: #3fb950;
        --accent-hover: #2ea043;
        --danger: #ff6b6b;
        --warning: #f1c40f;
        --border: #2d3640;
        --chip: #223041;
        --code: #11151b;
        --shadow: rgba(0, 0, 0, 0.3);
      }

      * {
        box-sizing: border-box;
        margin: 0;
        padding: 0;
      }

      body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Roboto", "Oxygen", "Ubuntu", "Cantarell", sans-serif;
        background: var(--bg);
        color: var(--text);
        padding: 0;
        margin: 0;
        overflow: hidden;
      }

      .header {
        background: var(--panel);
        border-bottom: 1px solid var(--border);
        padding: 16px 24px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        position: sticky;
        top: 0;
        z-index: 100;
      }

      .header h1 {
        font-size: 18px;
        font-weight: 600;
        margin: 0;
      }

      .header-actions {
        display: flex;
        gap: 8px;
      }

      .tabs {
        display: flex;
        background: var(--panel);
        border-bottom: 1px solid var(--border);
        padding: 0 24px;
        gap: 4px;
        overflow-x: auto;
      }

      .tab {
        padding: 12px 16px;
        background: transparent;
        border: none;
        border-bottom: 2px solid transparent;
        color: var(--text-muted);
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: all 0.2s;
        white-space: nowrap;
      }

      .tab:hover {
        color: var(--text);
        background: var(--surface);
      }

      .tab.active {
        color: var(--accent);
        border-bottom-color: var(--accent);
      }

      .content {
        padding: 24px;
        overflow-y: auto;
        height: calc(100vh - 120px);
      }

      .tab-content {
        display: none;
      }

      .tab-content.active {
        display: block;
      }

      .card {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 16px;
        margin-bottom: 16px;
      }

      .card-title {
        font-size: 14px;
        font-weight: 600;
        margin-bottom: 12px;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .stat-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
        gap: 16px;
        margin-bottom: 24px;
      }

      .stat-card {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 16px;
      }

      .stat-label {
        font-size: 12px;
        color: var(--text-muted);
        margin-bottom: 8px;
      }

      .stat-value {
        font-size: 24px;
        font-weight: 700;
        color: var(--text);
      }

      .btn {
        padding: 8px 16px;
        border: none;
        border-radius: 6px;
        font-size: 14px;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.2s;
      }

      .btn-primary {
        background: var(--accent);
        color: #041108;
      }

      .btn-primary:hover {
        background: var(--accent-hover);
      }

      .btn-secondary {
        background: var(--surface);
        color: var(--text);
        border: 1px solid var(--border);
      }

      .btn-secondary:hover {
        background: var(--chip);
      }

      .btn-danger {
        background: var(--danger);
        color: white;
      }

      .btn-danger:hover {
        opacity: 0.9;
      }

      .btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      .input-group {
        margin-bottom: 16px;
      }

      .input-label {
        display: block;
        font-size: 12px;
        color: var(--text-muted);
        margin-bottom: 6px;
      }

      .input {
        width: 100%;
        padding: 8px 12px;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 6px;
        color: var(--text);
        font-size: 14px;
      }

      .input:focus {
        outline: none;
        border-color: var(--accent);
      }

      .list {
        list-style: none;
      }

      .list-item {
        padding: 12px;
        border-bottom: 1px solid var(--border);
        display: flex;
        align-items: center;
        justify-content: space-between;
      }

      .list-item:last-child {
        border-bottom: none;
      }

      .list-item:hover {
        background: var(--surface);
      }

      .badge {
        padding: 4px 8px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 600;
        background: var(--chip);
        color: var(--text-muted);
      }

      .badge-success {
        background: rgba(63, 185, 80, 0.2);
        color: var(--accent);
      }

      .badge-danger {
        background: rgba(255, 107, 107, 0.2);
        color: var(--danger);
      }

      .alert {
        padding: 12px 16px;
        border-radius: 6px;
        margin-bottom: 16px;
      }

      .alert-error {
        background: rgba(255, 107, 107, 0.1);
        border: 1px solid var(--danger);
        color: var(--danger);
      }

      .alert-success {
        background: rgba(63, 185, 80, 0.1);
        border: 1px solid var(--accent);
        color: var(--accent);
      }

      .loading {
        text-align: center;
        padding: 40px;
        color: var(--text-muted);
      }

      .code {
        font-family: "SF Mono", "Monaco", "Inconsolata", "Fira Code", monospace;
        font-size: 12px;
        background: var(--code);
        padding: 12px;
        border-radius: 6px;
        overflow-x: auto;
        white-space: pre-wrap;
        word-break: break-all;
      }

      .grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: 16px;
      }

      .file-item {
        padding: 12px;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.2s;
      }

      .file-item:hover {
        border-color: var(--accent);
        background: var(--chip);
      }

      .file-path {
        font-size: 13px;
        color: var(--text);
        margin-bottom: 4px;
      }

      .file-meta {
        font-size: 11px;
        color: var(--text-muted);
      }

      .tag-chip {
        display: inline-block;
        padding: 4px 10px;
        margin: 4px 4px 4px 0;
        background: var(--chip);
        border-radius: 12px;
        font-size: 12px;
        cursor: pointer;
      }

      .tag-chip:hover {
        background: var(--accent);
        color: #041108;
      }

      .workspace-selector {
        margin-bottom: 16px;
      }

      .workspace-select {
        width: 100%;
        padding: 8px 12px;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 6px;
        color: var(--text);
        font-size: 14px;
      }
    `;
  }

  private getBody(): string {
    return `
      <div class="header">
        <h1>🧠 Cortex Backend</h1>
        <div class="header-actions">
          <button class="btn btn-secondary" id="refreshBtn">Refresh</button>
        </div>
      </div>

      <div class="tabs">
        <button class="tab active" data-tab="overview">Overview</button>
        <button class="tab" data-tab="workspaces">Workspaces</button>
        <button class="tab" data-tab="files">Files</button>
        <button class="tab" data-tab="metadata">Metadata</button>
        <button class="tab" data-tab="llm">LLM</button>
        <button class="tab" data-tab="config">Configuration</button>
        <button class="tab" data-tab="tasks">Tasks</button>
        <button class="tab" data-tab="logs">Logs</button>
      </div>

      <div class="content">
        <div id="alert-container"></div>

        <!-- Overview Tab -->
        <div class="tab-content active" id="overview">
          <div class="stat-grid">
            <div class="stat-card">
              <div class="stat-label">Health</div>
              <div class="stat-value" id="health-status">-</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Version</div>
              <div class="stat-value" id="version">-</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Uptime</div>
              <div class="stat-value" id="uptime">-</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Workspaces</div>
              <div class="stat-value" id="workspace-count">-</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Indexed Files</div>
              <div class="stat-value" id="file-count">-</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Memory</div>
              <div class="stat-value" id="memory">-</div>
            </div>
          </div>

          <div class="card">
            <div class="card-title">Queue Status</div>
            <div class="grid">
              <div>Pending: <span id="queue-pending">-</span></div>
              <div>Running: <span id="queue-running">-</span></div>
              <div>Completed: <span id="queue-completed">-</span></div>
              <div>Failed: <span id="queue-failed">-</span></div>
            </div>
          </div>

          <div class="card">
            <div class="card-title">Pipeline Events</div>
            <div class="code" id="pipeline-events">No events yet</div>
          </div>
        </div>

        <!-- Workspaces Tab -->
        <div class="tab-content" id="workspaces">
          <div class="workspace-selector">
            <label class="input-label">Active Workspace</label>
            <select class="workspace-select" id="workspace-select">
              <option>Loading...</option>
            </select>
          </div>

          <div class="card">
            <div class="card-title">Workspace Actions</div>
            <button class="btn btn-primary" id="reindex-btn">Reindex Workspace</button>
          </div>

          <div class="card">
            <div class="card-title">Workspace Details</div>
            <div class="code" id="workspace-details">No workspace selected</div>
          </div>
        </div>

        <!-- Files Tab -->
        <div class="tab-content" id="files">
          <div class="card">
            <div class="card-title">File Browser</div>
            <div class="input-group">
              <label class="input-label">Search Files</label>
              <input type="text" class="input" id="file-search" placeholder="Search by path, extension, or type...">
            </div>
            <div id="files-list" class="grid"></div>
          </div>
        </div>

        <!-- Metadata Tab -->
        <div class="tab-content" id="metadata">
          <div class="card">
            <div class="card-title">Tags</div>
            <div id="tags-list"></div>
            <div class="input-group">
              <label class="input-label">Add Tag to File</label>
              <input type="text" class="input" id="tag-file-path" placeholder="File path">
              <input type="text" class="input" style="margin-top: 8px;" id="tag-name" placeholder="Tag name">
              <button class="btn btn-primary" style="margin-top: 8px;" id="add-tag-btn">Add Tag</button>
            </div>
          </div>

          <div class="card">
            <div class="card-title">Projects</div>
            <div id="contexts-list"></div>
            <div class="input-group">
              <label class="input-label">Add Project to File</label>
              <input type="text" class="input" id="context-file-path" placeholder="File path">
              <input type="text" class="input" style="margin-top: 8px;" id="context-name" placeholder="Project name">
              <button class="btn btn-primary" style="margin-top: 8px;" id="add-context-btn">Add Project</button>
            </div>
          </div>

          <div class="card">
            <div class="card-title">File Metadata</div>
            <div class="input-group">
              <label class="input-label">Get Metadata for File</label>
              <input type="text" class="input" id="metadata-file-path" placeholder="File path">
              <button class="btn btn-secondary" style="margin-top: 8px;" id="get-metadata-btn">Get Metadata</button>
            </div>
            <div class="code" id="metadata-display" style="margin-top: 16px; display: none;"></div>
          </div>
        </div>

        <!-- LLM Tab -->
        <div class="tab-content" id="llm">
          <div class="card">
            <div class="card-title">LLM Providers</div>
            <div id="providers-list"></div>
          </div>

          <div class="card">
            <div class="card-title">Completion</div>
            <div class="input-group">
              <label class="input-label">Prompt</label>
              <textarea class="input" id="llm-prompt" rows="4" placeholder="Enter your prompt..."></textarea>
            </div>
            <button class="btn btn-primary" id="llm-complete-btn">Generate</button>
            <div class="code" id="llm-result" style="margin-top: 16px; display: none;"></div>
          </div>
        </div>

        <!-- Configuration Tab -->
        <div class="tab-content" id="config">
          <div class="card">
            <div class="card-title">Configuration Settings</div>
            <div id="config-settings"></div>
            <div style="margin-top: 16px;">
              <button class="btn btn-primary" id="save-config-btn">Save Configuration</button>
              <button class="btn btn-secondary" style="margin-left: 8px;" id="reload-config-btn">Reload from Server</button>
            </div>
          </div>

          <div class="card">
            <div class="card-title">AI Prompts</div>
            <div class="input-group">
              <label class="input-label">Tag Suggestion Prompt</label>
              <textarea class="input" id="prompt-suggest-tags" rows="6" placeholder="Prompt template for tag suggestions..."></textarea>
            </div>
            <div class="input-group">
              <label class="input-label">Project Suggestion Prompt</label>
              <textarea class="input" id="prompt-suggest-project" rows="6" placeholder="Prompt template for project suggestions..."></textarea>
            </div>
            <div class="input-group">
              <label class="input-label">Summary Generation Prompt</label>
              <textarea class="input" id="prompt-generate-summary" rows="6" placeholder="Prompt template for summary generation..."></textarea>
            </div>
            <div class="input-group">
              <label class="input-label">Key Terms Extraction Prompt</label>
              <textarea class="input" id="prompt-extract-key-terms" rows="6" placeholder="Prompt template for key terms extraction..."></textarea>
            </div>
            <div class="input-group">
              <label class="input-label">RAG Answer Prompt</label>
              <textarea class="input" id="prompt-rag-answer" rows="6" placeholder="Prompt template for RAG answers..."></textarea>
            </div>
            <div style="margin-top: 16px;">
              <button class="btn btn-primary" id="save-prompts-btn">Save Prompts</button>
              <label style="margin-left: 16px; color: var(--text-muted);">
                <input type="checkbox" id="create-version-checkbox" checked> Create version snapshot
              </label>
            </div>
            <div class="input-group" style="margin-top: 8px;">
              <label class="input-label">Version Description (optional)</label>
              <input type="text" class="input" id="version-description" placeholder="Describe this configuration change...">
            </div>
          </div>

          <div class="card">
            <div class="card-title">Configuration Versions</div>
            <div id="config-versions-list"></div>
            <div style="margin-top: 16px;">
              <button class="btn btn-secondary" id="load-versions-btn">Load Versions</button>
            </div>
          </div>
        </div>

        <!-- Tasks Tab -->
        <div class="tab-content" id="tasks">
          <div class="card">
            <div class="card-title">Task Queue</div>
            <div class="code" id="tasks-list">No tasks available</div>
          </div>
        </div>

        <!-- Logs Tab -->
        <div class="tab-content" id="logs">
          <div class="card">
            <div class="card-title">Daemon Logs</div>
            <div class="code" id="logs-content">No logs available</div>
          </div>
        </div>
      </div>
    `;
  }

  private getScript(): string {
    return `
      const vscode = acquireVsCodeApi();
      let currentData = null;
      let activeWorkspaceId = null;

      // Tab switching
      document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
          const tabName = tab.dataset.tab;
          document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
          document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
          tab.classList.add('active');
          document.getElementById(tabName).classList.add('active');
        });
      });

      // Refresh button
      document.getElementById('refreshBtn').addEventListener('click', () => {
        vscode.postMessage({ type: 'refresh' });
      });

      // Workspace selector
      document.getElementById('workspace-select').addEventListener('change', (e) => {
        activeWorkspaceId = e.target.value;
        vscode.postMessage({ type: 'set_active_workspace', workspaceId: e.target.value });
      });

      // Reindex button
      document.getElementById('reindex-btn').addEventListener('click', () => {
        if (!activeWorkspaceId || !currentData?.snapshot?.workspaces) return;
        const workspace = currentData.snapshot.workspaces.find(w => w.id === activeWorkspaceId);
        if (!workspace) return;
        vscode.postMessage({
          type: 'reindex_workspace',
          workspaceId: activeWorkspaceId,
          path: workspace.path
        });
      });

      // Metadata operations
      document.getElementById('add-tag-btn').addEventListener('click', () => {
        const path = document.getElementById('tag-file-path').value;
        const tag = document.getElementById('tag-name').value;
        if (!path || !tag) return;
        vscode.postMessage({ type: 'add_tag', relativePath: path, tag });
        document.getElementById('tag-file-path').value = '';
        document.getElementById('tag-name').value = '';
      });

      document.getElementById('add-context-btn').addEventListener('click', () => {
        const path = document.getElementById('context-file-path').value;
        const context = document.getElementById('context-name').value;
        if (!path || !context) return;
        vscode.postMessage({ type: 'add_context', relativePath: path, context });
        document.getElementById('context-file-path').value = '';
        document.getElementById('context-name').value = '';
      });

      document.getElementById('get-metadata-btn').addEventListener('click', () => {
        const path = document.getElementById('metadata-file-path').value;
        if (!path) return;
        vscode.postMessage({ type: 'get_file_metadata', relativePath: path });
      });

      // LLM completion
      document.getElementById('llm-complete-btn').addEventListener('click', () => {
        const prompt = document.getElementById('llm-prompt').value;
        if (!prompt) return;
        vscode.postMessage({ type: 'llm_completion', prompt });
      });

      // Configuration handlers
      document.getElementById('reload-config-btn').addEventListener('click', () => {
        vscode.postMessage({ type: 'load_config' });
      });

      document.getElementById('save-config-btn').addEventListener('click', () => {
        // TODO: Collect config from UI and send
        vscode.postMessage({ type: 'save_config', config: {} });
      });

      document.getElementById('save-prompts-btn').addEventListener('click', () => {
        const prompts = {
          suggest_tags: document.getElementById('prompt-suggest-tags').value,
          suggest_project: document.getElementById('prompt-suggest-project').value,
          generate_summary: document.getElementById('prompt-generate-summary').value,
          extract_key_terms: document.getElementById('prompt-extract-key-terms').value,
          rag_answer: document.getElementById('prompt-rag-answer').value,
        };
        const createVersion = document.getElementById('create-version-checkbox').checked;
        const description = document.getElementById('version-description').value;
        vscode.postMessage({
          type: 'save_prompts',
          prompts,
          createVersion,
          description,
        });
      });

      document.getElementById('load-versions-btn').addEventListener('click', () => {
        vscode.postMessage({ type: 'load_config_versions' });
      });

      // Message handler
      window.addEventListener('message', (event) => {
        const message = event.data;
        if (!message) return;

        if (message.type === 'loading') {
          document.getElementById('refreshBtn').disabled = message.value === true;
          return;
        }

        if (message.type === 'error') {
          showAlert(message.message || 'Unknown error', 'error');
          return;
        }

        if (message.type === 'success') {
          showAlert(message.message || 'Success', 'success');
          return;
        }

        if (message.type === 'data') {
          currentData = message.data;
          renderData(message.data);
          return;
        }

        if (message.type === 'activeWorkspace') {
          activeWorkspaceId = message.workspaceId;
          return;
        }

        if (message.type === 'file_metadata') {
          const display = document.getElementById('metadata-display');
          display.textContent = JSON.stringify(message.data, null, 2);
          display.style.display = 'block';
          return;
        }

        if (message.type === 'llm_completion_result') {
          const result = document.getElementById('llm-result');
          result.textContent = message.data;
          result.style.display = 'block';
          return;
        }

        if (message.type === 'pipeline_event') {
          addPipelineEvent(message.data);
          return;
        }

        if (message.type === 'config_data') {
          renderConfig(message.data);
          return;
        }

        if (message.type === 'config_versions') {
          renderConfigVersions(message.data);
          return;
        }
      });

      function showAlert(message, type) {
        const container = document.getElementById('alert-container');
        const alert = document.createElement('div');
        alert.className = \`alert alert-\${type}\`;
        alert.textContent = message;
        container.appendChild(alert);
        setTimeout(() => alert.remove(), 5000);
      }

      function formatBytes(bytes) {
        if (!bytes && bytes !== 0) return '-';
        const units = ['B', 'KB', 'MB', 'GB'];
        let idx = 0;
        let value = bytes;
        while (value >= 1024 && idx < units.length - 1) {
          value /= 1024;
          idx++;
        }
        return \`\${value.toFixed(1)} \${units[idx]}\`;
      }

      function formatSeconds(seconds) {
        if (!seconds || seconds < 0) return '-';
        const hrs = Math.floor(seconds / 3600);
        const mins = Math.floor((seconds % 3600) / 60);
        return hrs > 0 ? \`\${hrs}h \${mins}m\` : \`\${mins}m\`;
      }

      function renderData(data) {
        const { snapshot, tags, contexts, providers } = data;

        // Overview
        const status = snapshot?.status || {};
        const health = snapshot?.health || {};
        const metrics = snapshot?.metrics || {};
        const resources = status.resources || metrics.resources || {};

        document.getElementById('health-status').textContent = health.status || '-';
        document.getElementById('version').textContent = status.version || '-';
        document.getElementById('uptime').textContent = formatSeconds(status.uptime_seconds || metrics.counters?.uptime_seconds);
        document.getElementById('workspace-count').textContent = status.workspace_count ?? '-';
        document.getElementById('file-count').textContent = status.indexed_files ?? '-';
        document.getElementById('memory').textContent = formatBytes(resources.memory_bytes || metrics.gauges?.heap_alloc);

        document.getElementById('queue-pending').textContent = status.queue_stats?.pending ?? metrics.counters?.tasks_pending ?? '-';
        document.getElementById('queue-running').textContent = status.queue_stats?.running ?? '-';
        document.getElementById('queue-completed').textContent = status.queue_stats?.completed ?? metrics.counters?.tasks_processed ?? '-';
        document.getElementById('queue-failed').textContent = status.queue_stats?.failed ?? metrics.counters?.tasks_failed ?? '-';

        // Workspaces
        const workspaces = snapshot?.workspaces || [];
        const workspaceSelect = document.getElementById('workspace-select');
        workspaceSelect.innerHTML = '';
        if (workspaces.length === 0) {
          workspaceSelect.innerHTML = '<option>No workspaces</option>';
        } else {
          workspaces.forEach(ws => {
            const option = document.createElement('option');
            option.value = ws.id;
            option.textContent = ws.name || ws.path || ws.id;
            if (ws.id === activeWorkspaceId) {
              option.selected = true;
            }
            workspaceSelect.appendChild(option);
          });
          if (!activeWorkspaceId && workspaces[0]) {
            activeWorkspaceId = workspaces[0].id;
          }
        }

        const workspaceDetails = document.getElementById('workspace-details');
        const activeWorkspace = workspaces.find(w => w.id === activeWorkspaceId);
        if (activeWorkspace) {
          workspaceDetails.textContent = JSON.stringify(activeWorkspace, null, 2);
        }

        // Files
        const files = snapshot?.files || [];
        const filesList = document.getElementById('files-list');
        filesList.innerHTML = '';
        files.slice(0, 50).forEach(file => {
          const entry = file.entry || {};
          const item = document.createElement('div');
          item.className = 'file-item';
          item.innerHTML = \`
            <div class="file-path">\${entry.relative_path || entry.absolute_path || '-'}</div>
            <div class="file-meta">\${entry.extension || '-'} • \${formatBytes(entry.file_size || 0)}</div>
          \`;
          item.addEventListener('click', () => {
            vscode.postMessage({ type: 'get_file_metadata', relativePath: entry.relative_path });
            document.querySelectorAll('.tab').forEach(t => {
              if (t.dataset.tab === 'metadata') {
                t.click();
              }
            });
          });
          filesList.appendChild(item);
        });

        // Tags
        const tagsList = document.getElementById('tags-list');
        tagsList.innerHTML = '';
        if (tags?.tags && tags.tags.length > 0) {
          tags.tags.forEach(tag => {
            const count = tags.counts?.[tag] || 0;
            const chip = document.createElement('span');
            chip.className = 'tag-chip';
            chip.textContent = \`\${tag} (\${count})\`;
            chip.addEventListener('click', () => {
              vscode.postMessage({ type: 'list_files_by_tag', tag });
              document.querySelectorAll('.tab').forEach(t => {
                if (t.dataset.tab === 'files') {
                  t.click();
                }
              });
            });
            tagsList.appendChild(chip);
          });
        } else {
          tagsList.textContent = 'No tags yet';
        }

        // Contexts
        const contextsList = document.getElementById('contexts-list');
        contextsList.innerHTML = '';
        if (contexts && contexts.length > 0) {
          contexts.forEach(context => {
            const chip = document.createElement('span');
            chip.className = 'tag-chip';
            chip.textContent = context;
            chip.addEventListener('click', () => {
              vscode.postMessage({ type: 'list_files_by_context', context });
              document.querySelectorAll('.tab').forEach(t => {
                if (t.dataset.tab === 'files') {
                  t.click();
                }
              });
            });
            contextsList.appendChild(chip);
          });
        } else {
          contextsList.textContent = 'No contexts yet';
        }

        // Providers
        const providersList = document.getElementById('providers-list');
        providersList.innerHTML = '';
        if (providers && providers.length > 0) {
          providers.forEach(provider => {
            const item = document.createElement('div');
            item.className = 'list-item';
            item.innerHTML = \`
              <div>
                <strong>\${provider.name || provider.id}</strong>
                <div style="font-size: 12px; color: var(--text-muted);">\${provider.type} • \${provider.endpoint}</div>
              </div>
              <span class="badge \${provider.available ? 'badge-success' : 'badge-danger'}">
                \${provider.available ? 'Available' : 'Unavailable'}
              </span>
            \`;
            providersList.appendChild(item);
          });
        } else {
          providersList.textContent = 'No providers configured';
        }

        // Logs
        const logs = snapshot?.logs || {};
        const logLines = Array.isArray(logs.lines) ? logs.lines : [];
        document.getElementById('logs-content').textContent = logLines.length > 0
          ? logLines.join('\\n')
          : 'No logs available';
      }

      const pipelineEvents = [];
      function addPipelineEvent(event) {
        if (!event) return;
        pipelineEvents.push(event);
        if (pipelineEvents.length > 100) {
          pipelineEvents.shift();
        }
        const lines = pipelineEvents.slice().reverse().map(evt => {
          const ts = evt.timestamp_unix ? new Date(Number(evt.timestamp_unix) * 1000) : null;
          const time = ts ? ts.toLocaleTimeString() : '--:--:--';
          const stage = evt.stage || '-';
          const type = evt.type || '-';
          const path = evt.file_path || '-';
          const suffix = evt.error ? \` | error=\${evt.error}\` : '';
          return \`\${time} | \${type} | \${stage} | \${path}\${suffix}\`;
        });
        document.getElementById('pipeline-events').textContent = lines.length > 0
          ? lines.join('\\n')
          : 'No pipeline events yet';
      }

      function renderConfig(config) {
        if (!config || !config.llm) return;
        
        const prompts = config.llm.prompts || {};
        document.getElementById('prompt-suggest-tags').value = prompts.suggest_tags || '';
        document.getElementById('prompt-suggest-project').value = prompts.suggest_project || '';
        document.getElementById('prompt-generate-summary').value = prompts.generate_summary || '';
        document.getElementById('prompt-extract-key-terms').value = prompts.extract_key_terms || '';
        document.getElementById('prompt-rag-answer').value = prompts.rag_answer || '';

        // Render config settings
        const settingsDiv = document.getElementById('config-settings');
        if (settingsDiv && config) {
          settingsDiv.innerHTML = \`
            <div style="font-size: 12px; color: var(--text-muted);">
              <div>gRPC Address: <strong>\${config.grpc_address || '-'}</strong></div>
              <div>HTTP Address: <strong>\${config.http_address || '-'}</strong></div>
              <div>Data Dir: <strong>\${config.data_dir || '-'}</strong></div>
              <div>Worker Count: <strong>\${config.worker_count || '-'}</strong></div>
              <div>Max Concurrent Tasks: <strong>\${config.max_concurrent_tasks || '-'}</strong></div>
              <div>Log Level: <strong>\${config.log_level || '-'}</strong></div>
              <div style="margin-top: 8px;">
                <strong>LLM:</strong>
                <div style="margin-left: 16px;">
                  Enabled: <strong>\${config.llm?.enabled ? 'Yes' : 'No'}</strong><br>
                  Default Provider: <strong>\${config.llm?.default_provider || '-'}</strong><br>
                  Default Model: <strong>\${config.llm?.default_model || '-'}</strong><br>
                  Max Context Tokens: <strong>\${config.llm?.max_context_tokens || '-'}</strong>
                </div>
              </div>
            </div>
          \`;
        }
      }

      function renderConfigVersions(data) {
        const versionsDiv = document.getElementById('config-versions-list');
        if (!versionsDiv) return;

        if (!data || !data.versions || data.versions.length === 0) {
          versionsDiv.innerHTML = '<div style="color: var(--text-muted);">No configuration versions found</div>';
          return;
        }

        versionsDiv.innerHTML = '';
        data.versions.forEach(version => {
          const item = document.createElement('div');
          item.className = 'list-item';
          const date = version.created_at ? new Date(Number(version.created_at) * 1000).toLocaleString() : '-';
          item.innerHTML = \`
            <div>
              <strong>\${version.version_id?.substring(0, 8) || '-'}</strong>
              <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
                \${date} • \${version.created_by || 'System'} • \${version.description || 'No description'}
              </div>
            </div>
            <button class="btn btn-secondary" onclick="restoreVersion('\${version.version_id}')">Restore</button>
          \`;
          versionsDiv.appendChild(item);
        });
      }

      function restoreVersion(versionId) {
        if (!confirm('Are you sure you want to restore this configuration version? This will overwrite the current configuration.')) {
          return;
        }
        vscode.postMessage({
          type: 'restore_config_version',
          versionId,
          createBackup: true,
          description: 'Restored from admin UI',
        });
      }

      // Make restoreVersion available globally
      window.restoreVersion = restoreVersion;

      // Initial load
      vscode.postMessage({ type: 'refresh' });
      vscode.postMessage({ type: 'load_config' });
    `;
  }
}

