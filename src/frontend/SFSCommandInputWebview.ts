/**
 * SFSCommandInputWebview - Sidebar webview for natural language commands
 *
 * Features:
 * - Persistent input box in sidebar
 * - Real-time autocomplete suggestions
 * - Command preview before execution
 * - Command history dropdown
 */

import * as vscode from 'vscode';
import { GrpcSFSClient, CommandSuggestion, SFSCommandHistoryEntry, SFSPreviewResult } from '../core/GrpcSFSClient';

/**
 * SFS Command Input Webview Provider
 */
export class SFSCommandInputWebviewProvider implements vscode.WebviewViewProvider {
  public static readonly viewType = 'cortex-sfsInput';

  private view?: vscode.WebviewView;
  private sfsClient: GrpcSFSClient;
  private cachedSuggestions: CommandSuggestion[] = [];
  private cachedHistory: SFSCommandHistoryEntry[] = [];

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly workspaceId: string
  ) {
    const config = vscode.workspace.getConfiguration('cortex');
    const endpoint = config.get<string>('grpc.address', 'localhost:50051');
    this.sfsClient = new GrpcSFSClient(endpoint);
  }

  resolveWebviewView(
    webviewView: vscode.WebviewView,
    _context: vscode.WebviewViewResolveContext,
    _token: vscode.CancellationToken
  ): void | Thenable<void> {
    this.view = webviewView;

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [this.extensionUri],
    };

    webviewView.webview.html = this.getHtml();

    webviewView.webview.onDidReceiveMessage(async (message) => {
      await this.handleMessage(message);
    });

    // Load initial suggestions and history
    this.loadInitialData();
  }

  /**
   * Load initial suggestions and history
   */
  private async loadInitialData(): Promise<void> {
    try {
      await this.sfsClient.connect();

      const [suggestions, history] = await Promise.all([
        this.sfsClient.suggestCommands(this.workspaceId, '', [], 10).catch((error: any) => {
          // Handle unimplemented service gracefully
          if (error?.code === 12 || error?.message?.includes('UNIMPLEMENTED')) {
            return [];
          }
          throw error;
        }),
        this.sfsClient.getCommandHistory(this.workspaceId, 10).catch((error: any) => {
          // Handle unimplemented service gracefully
          if (error?.code === 12 || error?.message?.includes('UNIMPLEMENTED')) {
            return [];
          }
          throw error;
        }),
      ]);

      this.cachedSuggestions = suggestions;
      this.cachedHistory = history;

      this.view?.webview.postMessage({
        type: 'initialData',
        suggestions: this.cachedSuggestions,
        history: this.cachedHistory,
      });
    } catch (error) {
      console.error('[SFSInput] Failed to load initial data:', error);
    }
  }

  /**
   * Handle messages from the webview
   */
  private async handleMessage(message: { type: string; [key: string]: any }): Promise<void> {
    switch (message.type) {
      case 'getSuggestions':
        await this.getSuggestions(message.query);
        break;

      case 'preview':
        await this.previewCommand(message.command);
        break;

      case 'execute':
        await this.executeCommand(message.command);
        break;

      case 'cancelPreview':
        // Just close the preview panel
        break;
    }
  }

  /**
   * Get suggestions for partial command
   */
  private async getSuggestions(query: string): Promise<void> {
    try {
      const suggestions = await this.sfsClient.suggestCommands(
        this.workspaceId,
        query,
        [],
        10
      );

      this.view?.webview.postMessage({
        type: 'suggestions',
        suggestions,
      });
    } catch (error) {
      console.error('[SFSInput] Failed to get suggestions:', error);
    }
  }

  /**
   * Preview a command
   */
  private async previewCommand(command: string): Promise<void> {
    try {
      const preview = await this.sfsClient.previewCommand(
        this.workspaceId,
        command,
        []
      );

      this.view?.webview.postMessage({
        type: 'preview',
        preview,
      });
    } catch (error) {
      console.error('[SFSInput] Failed to preview command:', error);
      this.view?.webview.postMessage({
        type: 'previewError',
        error: (error as Error).message,
      });
    }
  }

  /**
   * Execute a command
   */
  private async executeCommand(command: string): Promise<void> {
    try {
      const result = await this.sfsClient.executeCommand(
        this.workspaceId,
        command,
        []
      );

      this.view?.webview.postMessage({
        type: 'result',
        result,
      });

      if (result.success) {
        vscode.window.showInformationMessage(
          `${result.explanation} (${result.files_affected} files)`
        );

        // Refresh history
        this.loadInitialData();
      } else {
        vscode.window.showErrorMessage(`Command failed: ${result.error_message}`);
      }
    } catch (error) {
      console.error('[SFSInput] Failed to execute command:', error);
      vscode.window.showErrorMessage(`Failed to execute command: ${(error as Error).message}`);
    }
  }

  /**
   * Generate HTML for the webview
   */
  private getHtml(): string {
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
  <title>SFS Commands</title>
  <style>
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }

    body {
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      color: var(--vscode-foreground);
      background: var(--vscode-sideBar-background);
      padding: 8px;
    }

    .input-container {
      position: relative;
      margin-bottom: 8px;
    }

    .command-input {
      width: 100%;
      padding: 8px 32px 8px 8px;
      background: var(--vscode-input-background);
      color: var(--vscode-input-foreground);
      border: 1px solid var(--vscode-input-border);
      border-radius: 4px;
      font-size: 12px;
      outline: none;
    }

    .command-input:focus {
      border-color: var(--vscode-focusBorder);
    }

    .command-input::placeholder {
      color: var(--vscode-input-placeholderForeground);
    }

    .submit-btn {
      position: absolute;
      right: 4px;
      top: 50%;
      transform: translateY(-50%);
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
      border: none;
      border-radius: 3px;
      padding: 4px 8px;
      cursor: pointer;
      font-size: 11px;
    }

    .submit-btn:hover {
      background: var(--vscode-button-hoverBackground);
    }

    .suggestions-dropdown {
      position: absolute;
      top: 100%;
      left: 0;
      right: 0;
      background: var(--vscode-dropdown-background);
      border: 1px solid var(--vscode-dropdown-border);
      border-radius: 4px;
      max-height: 200px;
      overflow-y: auto;
      z-index: 100;
      display: none;
    }

    .suggestions-dropdown.visible {
      display: block;
    }

    .suggestion-item {
      padding: 6px 8px;
      cursor: pointer;
      border-bottom: 1px solid var(--vscode-panel-border);
    }

    .suggestion-item:last-child {
      border-bottom: none;
    }

    .suggestion-item:hover,
    .suggestion-item.selected {
      background: var(--vscode-list-hoverBackground);
    }

    .suggestion-command {
      font-weight: 500;
      margin-bottom: 2px;
    }

    .suggestion-desc {
      font-size: 11px;
      color: var(--vscode-descriptionForeground);
    }

    .suggestion-meta {
      font-size: 10px;
      color: var(--vscode-descriptionForeground);
      margin-top: 2px;
    }

    .section-title {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      color: var(--vscode-sideBarSectionHeader-foreground);
      margin: 12px 0 6px 0;
      padding-bottom: 4px;
      border-bottom: 1px solid var(--vscode-panel-border);
    }

    .history-list {
      max-height: 150px;
      overflow-y: auto;
    }

    .history-item {
      padding: 6px 8px;
      cursor: pointer;
      border-radius: 3px;
      margin-bottom: 2px;
    }

    .history-item:hover {
      background: var(--vscode-list-hoverBackground);
    }

    .history-command {
      font-size: 12px;
      margin-bottom: 2px;
    }

    .history-meta {
      font-size: 10px;
      color: var(--vscode-descriptionForeground);
      display: flex;
      gap: 8px;
    }

    .history-success { color: var(--vscode-testing-iconPassed); }
    .history-failed { color: var(--vscode-testing-iconFailed); }

    .preview-panel {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: var(--vscode-editor-background);
      z-index: 200;
      display: none;
      flex-direction: column;
      padding: 12px;
    }

    .preview-panel.visible {
      display: flex;
    }

    .preview-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 12px;
    }

    .preview-title {
      font-weight: 600;
    }

    .preview-close {
      background: none;
      border: none;
      color: var(--vscode-foreground);
      cursor: pointer;
      font-size: 16px;
    }

    .preview-content {
      flex: 1;
      overflow-y: auto;
    }

    .preview-explanation {
      margin-bottom: 12px;
      padding: 8px;
      background: var(--vscode-textBlockQuote-background);
      border-radius: 4px;
    }

    .preview-stats {
      display: flex;
      gap: 12px;
      margin-bottom: 12px;
      font-size: 12px;
    }

    .preview-stat {
      padding: 4px 8px;
      background: var(--vscode-badge-background);
      color: var(--vscode-badge-foreground);
      border-radius: 3px;
    }

    .preview-warnings {
      background: var(--vscode-inputValidation-warningBackground);
      border: 1px solid var(--vscode-inputValidation-warningBorder);
      padding: 8px;
      border-radius: 4px;
      margin-bottom: 12px;
      font-size: 12px;
    }

    .preview-changes {
      font-size: 12px;
    }

    .preview-change {
      padding: 4px 0;
      border-bottom: 1px solid var(--vscode-panel-border);
    }

    .preview-actions {
      display: flex;
      gap: 8px;
      margin-top: 12px;
      padding-top: 12px;
      border-top: 1px solid var(--vscode-panel-border);
    }

    .btn {
      padding: 6px 14px;
      border: none;
      border-radius: 3px;
      cursor: pointer;
      font-size: 12px;
    }

    .btn-primary {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }

    .btn-primary:hover {
      background: var(--vscode-button-hoverBackground);
    }

    .btn-secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }

    .btn-secondary:hover {
      background: var(--vscode-button-secondaryHoverBackground);
    }

    .empty-state {
      text-align: center;
      padding: 20px;
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
    }

    .loading {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px;
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
    }

    .spinner {
      width: 14px;
      height: 14px;
      border: 2px solid var(--vscode-progressBar-background);
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  </style>
</head>
<body>
  <div class="input-container">
    <input
      type="text"
      class="command-input"
      id="commandInput"
      placeholder="Enter a command (e.g., 'group PDFs by author')"
      autocomplete="off"
    />
    <button class="submit-btn" id="submitBtn">▶</button>
    <div class="suggestions-dropdown" id="suggestions"></div>
  </div>

  <div class="section-title">Suggestions</div>
  <div id="defaultSuggestions"></div>

  <div class="section-title">Recent Commands</div>
  <div class="history-list" id="historyList"></div>

  <div class="preview-panel" id="previewPanel">
    <div class="preview-header">
      <span class="preview-title">Command Preview</span>
      <button class="preview-close" id="closePreview">×</button>
    </div>
    <div class="preview-content" id="previewContent"></div>
    <div class="preview-actions">
      <button class="btn btn-primary" id="executeBtn">Execute</button>
      <button class="btn btn-secondary" id="cancelBtn">Cancel</button>
    </div>
  </div>

  <script>
    const vscode = acquireVsCodeApi();

    let currentSuggestions = [];
    let selectedIndex = -1;
    let pendingCommand = '';
    let debounceTimer = null;

    const input = document.getElementById('commandInput');
    const submitBtn = document.getElementById('submitBtn');
    const suggestionsEl = document.getElementById('suggestions');
    const defaultSuggestionsEl = document.getElementById('defaultSuggestions');
    const historyListEl = document.getElementById('historyList');
    const previewPanel = document.getElementById('previewPanel');
    const previewContent = document.getElementById('previewContent');
    const closePreview = document.getElementById('closePreview');
    const executeBtn = document.getElementById('executeBtn');
    const cancelBtn = document.getElementById('cancelBtn');

    // Input handlers
    input.addEventListener('input', (e) => {
      const query = e.target.value.trim();

      if (debounceTimer) clearTimeout(debounceTimer);

      if (query.length >= 2) {
        debounceTimer = setTimeout(() => {
          vscode.postMessage({ type: 'getSuggestions', query });
        }, 300);
      } else {
        suggestionsEl.classList.remove('visible');
      }
    });

    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        if (selectedIndex >= 0 && currentSuggestions[selectedIndex]) {
          input.value = currentSuggestions[selectedIndex].command;
        }
        if (input.value.trim()) {
          submitCommand(input.value.trim());
        }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectSuggestion(selectedIndex + 1);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectSuggestion(selectedIndex - 1);
      } else if (e.key === 'Escape') {
        suggestionsEl.classList.remove('visible');
        selectedIndex = -1;
      }
    });

    input.addEventListener('focus', () => {
      if (currentSuggestions.length > 0) {
        suggestionsEl.classList.add('visible');
      }
    });

    input.addEventListener('blur', () => {
      // Delay hiding to allow click on suggestions
      setTimeout(() => {
        suggestionsEl.classList.remove('visible');
      }, 150);
    });

    submitBtn.addEventListener('click', () => {
      if (input.value.trim()) {
        submitCommand(input.value.trim());
      }
    });

    closePreview.addEventListener('click', () => {
      previewPanel.classList.remove('visible');
    });

    cancelBtn.addEventListener('click', () => {
      previewPanel.classList.remove('visible');
    });

    executeBtn.addEventListener('click', () => {
      if (pendingCommand) {
        vscode.postMessage({ type: 'execute', command: pendingCommand });
        previewPanel.classList.remove('visible');
        input.value = '';
      }
    });

    function submitCommand(command) {
      pendingCommand = command;
      vscode.postMessage({ type: 'preview', command });
    }

    function selectSuggestion(index) {
      if (currentSuggestions.length === 0) return;

      selectedIndex = Math.max(-1, Math.min(index, currentSuggestions.length - 1));

      document.querySelectorAll('.suggestion-item').forEach((el, i) => {
        el.classList.toggle('selected', i === selectedIndex);
      });
    }

    function renderSuggestions(suggestions, container) {
      if (!suggestions || suggestions.length === 0) {
        container.innerHTML = '<div class="empty-state">No suggestions available</div>';
        return;
      }

      container.innerHTML = suggestions.map((s, i) =>
        '<div class="suggestion-item" data-index="' + i + '" data-command="' + escapeHtml(s.command) + '">' +
        '<div class="suggestion-command">' + escapeHtml(s.command) + '</div>' +
        '<div class="suggestion-desc">' + escapeHtml(s.description) + '</div>' +
        '<div class="suggestion-meta">' + s.category + ' • ' + Math.round(s.relevance * 100) + '%</div>' +
        '</div>'
      ).join('');

      container.querySelectorAll('.suggestion-item').forEach(el => {
        el.addEventListener('click', () => {
          input.value = el.dataset.command;
          submitCommand(el.dataset.command);
        });
      });
    }

    function renderHistory(history) {
      if (!history || history.length === 0) {
        historyListEl.innerHTML = '<div class="empty-state">No command history</div>';
        return;
      }

      historyListEl.innerHTML = history.map(h => {
        const date = new Date(h.executed_at).toLocaleString();
        const statusClass = h.success ? 'history-success' : 'history-failed';
        const statusText = h.success ? '✓' : '✗';

        return '<div class="history-item" data-command="' + escapeHtml(h.command) + '">' +
          '<div class="history-command">' + escapeHtml(h.command) + '</div>' +
          '<div class="history-meta">' +
          '<span class="' + statusClass + '">' + statusText + '</span>' +
          '<span>' + h.files_affected + ' files</span>' +
          '<span>' + date + '</span>' +
          '</div>' +
          '</div>';
      }).join('');

      historyListEl.querySelectorAll('.history-item').forEach(el => {
        el.addEventListener('click', () => {
          input.value = el.dataset.command;
          input.focus();
        });
      });
    }

    function renderPreview(preview) {
      let html = '<div class="preview-explanation">' + escapeHtml(preview.explanation) + '</div>';

      html += '<div class="preview-stats">' +
        '<span class="preview-stat">' + preview.files_affected + ' files</span>' +
        '<span class="preview-stat">' + Math.round(preview.confidence * 100) + '% confidence</span>' +
        '<span class="preview-stat">' + preview.operation + '</span>' +
        '</div>';

      if (preview.warnings && preview.warnings.length > 0) {
        html += '<div class="preview-warnings">' +
          '<strong>⚠️ Warnings:</strong><br>' +
          preview.warnings.map(w => '• ' + escapeHtml(w)).join('<br>') +
          '</div>';
      }

      if (preview.planned_changes && preview.planned_changes.length > 0) {
        html += '<div class="preview-changes"><strong>Changes:</strong>';
        html += preview.planned_changes.slice(0, 10).map(c =>
          '<div class="preview-change">' +
          '<code>' + escapeHtml(c.relative_path) + '</code> → ' + c.operation +
          (c.after_value ? ' (' + escapeHtml(c.after_value) + ')' : '') +
          '</div>'
        ).join('');
        if (preview.planned_changes.length > 10) {
          html += '<div class="preview-change">... and ' + (preview.planned_changes.length - 10) + ' more</div>';
        }
        html += '</div>';
      }

      previewContent.innerHTML = html;
      previewPanel.classList.add('visible');
    }

    function escapeHtml(str) {
      if (!str) return '';
      return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
    }

    // Message handler
    window.addEventListener('message', event => {
      const message = event.data;

      switch (message.type) {
        case 'initialData':
          renderSuggestions(message.suggestions, defaultSuggestionsEl);
          renderHistory(message.history);
          break;

        case 'suggestions':
          currentSuggestions = message.suggestions;
          selectedIndex = -1;
          renderSuggestions(message.suggestions, suggestionsEl);
          suggestionsEl.classList.add('visible');
          break;

        case 'preview':
          renderPreview(message.preview);
          break;

        case 'previewError':
          previewContent.innerHTML = '<div class="preview-warnings">Error: ' + escapeHtml(message.error) + '</div>';
          previewPanel.classList.add('visible');
          break;

        case 'result':
          // Clear input on success
          if (message.result.success) {
            input.value = '';
          }
          break;
      }
    });
  </script>
</body>
</html>`;
  }

  /**
   * Dispose resources
   */
  dispose(): void {
    this.sfsClient.disconnect();
  }
}

/**
 * Register SFS Command Input in extension
 */
export function registerSFSCommandInput(
  context: vscode.ExtensionContext,
  workspaceId: string
): void {
  const provider = new SFSCommandInputWebviewProvider(context.extensionUri, workspaceId);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      SFSCommandInputWebviewProvider.viewType,
      provider
    )
  );
}
