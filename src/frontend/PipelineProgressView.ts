import * as vscode from "vscode";
import { GrpcAdminClient, PipelineEvent } from "../core/GrpcAdminClient";

export class PipelineProgressView {
  private static currentPanel: PipelineProgressView | undefined;

  private readonly panel: vscode.WebviewPanel;
  private readonly disposables: vscode.Disposable[] = [];
  private readonly adminClient: GrpcAdminClient;
  private stopPipelineStream?: () => void;
  private workspaceId?: string;
  private workspaceRoot?: string;
  private fileStatuses = new Map<string, FileStatus>();

  static show(
    context: vscode.ExtensionContext,
    workspaceId: string,
    workspaceRoot: string
  ): void {
    if (PipelineProgressView.currentPanel) {
      PipelineProgressView.currentPanel.panel.reveal(vscode.ViewColumn.Two);
      PipelineProgressView.currentPanel.workspaceId = workspaceId;
      PipelineProgressView.currentPanel.workspaceRoot = workspaceRoot;
      PipelineProgressView.currentPanel.startPipelineStream();
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      "cortexPipelineProgress",
      "Pipeline Progress",
      vscode.ViewColumn.Two,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
      }
    );

    PipelineProgressView.currentPanel = new PipelineProgressView(
      panel,
      context,
      workspaceId,
      workspaceRoot
    );
    void PipelineProgressView.currentPanel.initialize();
  }

  private constructor(
    panel: vscode.WebviewPanel,
    private context: vscode.ExtensionContext,
    workspaceId: string,
    workspaceRoot: string
  ) {
    this.panel = panel;
    this.adminClient = new GrpcAdminClient(context);
    this.workspaceId = workspaceId;
    this.workspaceRoot = workspaceRoot;

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
  }

  private dispose(): void {
    PipelineProgressView.currentPanel = undefined;
    while (this.disposables.length) {
      const item = this.disposables.pop();
      item?.dispose();
    }
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
      this.stopPipelineStream = undefined;
    }
  }

  private startPipelineStream(): void {
    if (this.stopPipelineStream) {
      this.stopPipelineStream();
    }

    if (!this.workspaceId) {
      return;
    }

    this.stopPipelineStream = this.adminClient.subscribePipelineEvents(
      (event: PipelineEvent) => {
        this.handlePipelineEvent(event);
      },
      this.workspaceId
    );
  }

  private handlePipelineEvent(event: PipelineEvent): void {
    const filePath = event.file_path || "";
    if (!filePath) {
      return;
    }

    let status = this.fileStatuses.get(filePath);
    if (!status) {
      status = {
        filePath,
        stages: new Map(),
        currentStage: "",
        status: "pending",
        error: undefined,
        startedAt: event.timestamp_unix
          ? new Date(Number(event.timestamp_unix) * 1000)
          : new Date(),
      };
      this.fileStatuses.set(filePath, status);
    }

    const eventType = event.type || "";
    const stage = event.stage || "";

    if (eventType === "pipeline.started") {
      status.status = "processing";
      status.currentStage = "start";
    } else if (eventType === "pipeline.progress") {
      status.currentStage = stage;
      status.stages.set(stage, {
        status: "completed",
        completedAt: event.timestamp_unix
          ? new Date(Number(event.timestamp_unix) * 1000)
          : new Date(),
      });
    } else if (eventType === "pipeline.completed") {
      status.status = "completed";
      status.currentStage = "complete";
      status.stages.set("complete", {
        status: "completed",
        completedAt: event.timestamp_unix
          ? new Date(Number(event.timestamp_unix) * 1000)
          : new Date(),
      });
    } else if (eventType === "pipeline.failed") {
      status.status = "failed";
      status.error = event.error || "Unknown error";
      status.stages.set(stage, {
        status: "failed",
        error: event.error,
        completedAt: event.timestamp_unix
          ? new Date(Number(event.timestamp_unix) * 1000)
          : new Date(),
      });
    }

    this.updateView();
  }

  private updateView(): void {
    const files = Array.from(this.fileStatuses.values()).map((file) => {
      const stagesMap: Record<string, any> = {};
      file.stages.forEach((stageStatus, stageName) => {
        stagesMap[stageName] = {
          status: stageStatus.status,
          error: stageStatus.error,
          completedAt: stageStatus.completedAt?.toISOString(),
          duration: stageStatus.completedAt && stageStatus.completedAt.getTime() > stageStatus.completedAt.getTime() - (stageStatus.completedAt.getTime() - file.startedAt.getTime())
            ? stageStatus.completedAt.getTime() - file.startedAt.getTime()
            : undefined,
        };
      });
      return {
        filePath: file.filePath,
        stages: stagesMap,
        currentStage: file.currentStage,
        status: file.status,
        error: file.error,
        startedAt: file.startedAt.toISOString(),
        duration: file.startedAt ? Date.now() - file.startedAt.getTime() : 0,
      };
    });
    const pipelineStages = [
      "start",
      "basic",
      "mime",
      "mirror",
      "code",
      "document",
      "relationship",
      "state",
      "ai",
      "complete",
    ];

    this.panel.webview.postMessage({
      type: "update",
      data: {
        files,
        stages: pipelineStages,
        totalFiles: files.length,
        completedFiles: files.filter((f) => f.status === "completed").length,
        failedFiles: files.filter((f) => f.status === "failed").length,
        processingFiles: files.filter((f) => f.status === "processing").length,
        pendingFiles: files.filter((f) => f.status === "pending").length,
      },
    });
  }

  private handleMessage(message: any): void {
    switch (message.type) {
      case "rebuild":
        void this.rebuildEverything();
        break;
      case "clear":
        this.fileStatuses.clear();
        this.updateView();
        break;
    }
  }

  private async rebuildEverything(): Promise<void> {
    if (!this.workspaceId || !this.workspaceRoot) {
      vscode.window.showErrorMessage(
        "Cortex: Workspace not registered with backend"
      );
      return;
    }

    // Clear existing statuses
    this.fileStatuses.clear();
    this.updateView();

    try {
      await this.adminClient.scanWorkspace(
        this.workspaceId,
        this.workspaceRoot,
        true
      );
      vscode.window.showInformationMessage(
        "Cortex: Full rebuild started. Watch progress in this panel."
      );
    } catch (error) {
      vscode.window.showErrorMessage(
        `Cortex: Failed to trigger rebuild: ${
          error instanceof Error ? error.message : String(error)
        }`
      );
    }
  }

  private getHtml(): string {
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pipeline Progress</title>
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
            background: var(--vscode-editor-background);
            padding: 20px;
        }
        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
            padding-bottom: 15px;
            border-bottom: 1px solid var(--vscode-panel-border);
        }
        .header h1 {
            font-size: 18px;
            font-weight: 600;
        }
        .header-controls {
            display: flex;
            gap: 10px;
            align-items: center;
        }
        .search-box {
            padding: 6px 10px;
            background: var(--vscode-input-background);
            color: var(--vscode-input-foreground);
            border: 1px solid var(--vscode-input-border);
            border-radius: 4px;
            font-size: 12px;
            width: 250px;
        }
        .stats {
            display: flex;
            gap: 20px;
            margin-bottom: 20px;
            flex-wrap: wrap;
        }
        .stat {
            padding: 10px 15px;
            background: var(--vscode-editor-background);
            border: 1px solid var(--vscode-panel-border);
            border-radius: 4px;
            min-width: 120px;
        }
        .stat-label {
            font-size: 11px;
            color: var(--vscode-descriptionForeground);
            margin-bottom: 5px;
        }
        .stat-value {
            font-size: 20px;
            font-weight: 600;
        }
        .stat-value.completed { color: var(--vscode-testing-iconPassed); }
        .stat-value.failed { color: var(--vscode-testing-iconFailed); }
        .stat-value.processing { color: var(--vscode-testing-iconQueued); }
        .controls {
            display: flex;
            gap: 10px;
            margin-bottom: 20px;
        }
        button {
            padding: 8px 16px;
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
        }
        button:hover {
            background: var(--vscode-button-hoverBackground);
        }
        button.secondary {
            background: var(--vscode-button-secondaryBackground);
            color: var(--vscode-button-secondaryForeground);
        }
        button.secondary:hover {
            background: var(--vscode-button-secondaryHoverBackground);
        }
        .table-container {
            overflow-x: auto;
            border: 1px solid var(--vscode-panel-border);
            border-radius: 4px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            min-width: 800px;
        }
        thead {
            background: var(--vscode-editor-background);
            position: sticky;
            top: 0;
            z-index: 10;
        }
        th {
            padding: 10px;
            text-align: left;
            font-weight: 600;
            font-size: 12px;
            color: var(--vscode-descriptionForeground);
            border-bottom: 2px solid var(--vscode-panel-border);
            white-space: nowrap;
            cursor: pointer;
            user-select: none;
        }
        th:hover {
            background: var(--vscode-list-hoverBackground);
        }
        th.sortable::after {
            content: ' ↕';
            opacity: 0.5;
            font-size: 10px;
        }
        th.sort-asc::after {
            content: ' ↑';
            opacity: 1;
        }
        th.sort-desc::after {
            content: ' ↓';
            opacity: 1;
        }
        th:first-child {
            position: sticky;
            left: 0;
            background: var(--vscode-editor-background);
            z-index: 11;
            min-width: 300px;
        }
        th:nth-child(2) {
            position: sticky;
            left: 300px;
            background: var(--vscode-editor-background);
            z-index: 11;
            min-width: 120px;
        }
        td {
            padding: 8px 10px;
            border-bottom: 1px solid var(--vscode-panel-border);
            font-size: 12px;
        }
        td:first-child {
            position: sticky;
            left: 0;
            background: var(--vscode-editor-background);
            z-index: 9;
            font-family: var(--vscode-editor-font-family);
            font-size: 11px;
            max-width: 300px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        td:nth-child(2) {
            position: sticky;
            left: 300px;
            background: var(--vscode-editor-background);
            z-index: 9;
            min-width: 120px;
        }
        tbody tr:hover {
            background: var(--vscode-list-hoverBackground);
        }
        .stage-status {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 20px;
            height: 20px;
            font-size: 16px;
            margin-right: 5px;
            vertical-align: middle;
        }
        .stage-cell {
            text-align: center;
            min-width: 80px;
        }
        .stage-cell-content {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 2px;
        }
        .stage-duration {
            font-size: 9px;
            color: var(--vscode-descriptionForeground);
            opacity: 0.7;
        }
        .stage-status.pending {
            color: var(--vscode-descriptionForeground);
            opacity: 0.4;
        }
        .stage-status.processing {
            color: var(--vscode-testing-iconQueued);
            animation: spin 1.5s linear infinite;
        }
        .stage-status.completed {
            color: var(--vscode-testing-iconPassed);
        }
        .stage-status.failed {
            color: var(--vscode-testing-iconFailed);
        }
        .stage-status.skipped {
            color: var(--vscode-descriptionForeground);
            opacity: 0.5;
        }
        .stage-status.warning {
            color: var(--vscode-testing-iconQueued);
        }
        @keyframes spin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }
        /* VS Code Codicons - automatically available in webview */
        .codicon {
            display: inline-block;
            vertical-align: middle;
            font-size: 16px;
        }
        .codicon-modifier-spin {
            animation: spin 1.5s linear infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 2px 8px;
            border-radius: 10px;
            font-size: 10px;
            font-weight: 600;
            text-transform: uppercase;
        }
        .status-icon {
            font-size: 14px;
            display: inline-flex;
            align-items: center;
        }
        .status-badge.pending {
            background: var(--vscode-descriptionForeground);
            opacity: 0.2;
            color: var(--vscode-foreground);
        }
        .status-badge.pending .status-icon {
            color: var(--vscode-descriptionForeground);
            opacity: 0.6;
        }
        .status-badge.processing {
            background: var(--vscode-testing-iconQueued);
            color: white;
        }
        .status-badge.processing .status-icon {
            animation: spin 1.5s linear infinite;
        }
        .status-badge.completed {
            background: var(--vscode-testing-iconPassed);
            color: white;
        }
        .status-badge.failed {
            background: var(--vscode-testing-iconFailed);
            color: white;
        }
        .status-badge.skipped {
            background: var(--vscode-descriptionForeground);
            opacity: 0.2;
            color: var(--vscode-foreground);
        }
        .status-badge.warning {
            background: var(--vscode-testing-iconQueued);
            color: white;
        }
        .error-text {
            color: var(--vscode-testing-iconFailed);
            font-size: 11px;
            margin-top: 4px;
        }
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: var(--vscode-descriptionForeground);
        }
        .empty-state h2 {
            margin-bottom: 10px;
            color: var(--vscode-foreground);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Pipeline Progress Monitor</h1>
        <div class="header-controls">
            <input type="text" class="search-box" id="searchBox" placeholder="Filter files..." oninput="filterFiles()">
        </div>
    </div>
    
    <div class="stats">
        <div class="stat">
            <div class="stat-label">Total Files</div>
            <div class="stat-value" id="totalFiles">0</div>
        </div>
        <div class="stat">
            <div class="stat-label">Processing</div>
            <div class="stat-value processing" id="processingFiles">0</div>
        </div>
        <div class="stat">
            <div class="stat-label">Completed</div>
            <div class="stat-value completed" id="completedFiles">0</div>
        </div>
        <div class="stat">
            <div class="stat-label">Failed</div>
            <div class="stat-value failed" id="failedFiles">0</div>
        </div>
    </div>

    <div class="controls">
        <button onclick="rebuildEverything()">🔄 Rebuild Everything</button>
        <button class="secondary" onclick="clearView()">Clear View</button>
    </div>

    <div class="table-container">
        <table id="progressTable">
            <thead>
                <tr>
                    <th class="sortable" onclick="sortTable('file')">File</th>
                    <th class="sortable" onclick="sortTable('status')">Status</th>
                    <th class="sortable" onclick="sortTable('duration')">Duration</th>
                    <th id="stageHeaders"></th>
                </tr>
            </thead>
            <tbody id="tableBody">
                <tr>
                    <td colspan="100%" class="empty-state">
                        <h2>No files being processed</h2>
                        <p>Click "Rebuild Everything" to start indexing</p>
                    </td>
                </tr>
            </tbody>
        </table>
    </div>

    <script>
        const vscode = acquireVsCodeApi();
        let stages = [];
        let files = [];
        let allFiles = [];
        let sortColumn = null;
        let sortDirection = 'asc';
        let filterText = '';

        function rebuildEverything() {
            vscode.postMessage({ type: 'rebuild' });
        }

        function clearView() {
            vscode.postMessage({ type: 'clear' });
        }

        function filterFiles() {
            filterText = document.getElementById('searchBox').value.toLowerCase();
            applyFilters();
        }

        function sortTable(column) {
            if (sortColumn === column) {
                sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                sortColumn = column;
                sortDirection = 'asc';
            }
            applyFilters();
            updateTable();
        }

        function applyFilters() {
            let filtered = [...allFiles];
            
            // Apply text filter
            if (filterText) {
                filtered = filtered.filter(f => 
                    f.filePath.toLowerCase().includes(filterText) ||
                    f.status.toLowerCase().includes(filterText)
                );
            }
            
            // Apply sorting
            if (sortColumn) {
                filtered.sort((a, b) => {
                    let aVal, bVal;
                    switch(sortColumn) {
                        case 'file':
                            aVal = a.filePath.toLowerCase();
                            bVal = b.filePath.toLowerCase();
                            break;
                        case 'status':
                            aVal = a.status;
                            bVal = b.status;
                            break;
                        case 'duration':
                            aVal = a.duration || 0;
                            bVal = b.duration || 0;
                            break;
                        default:
                            return 0;
                    }
                    if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
                    if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
                    return 0;
                });
            }
            
            files = filtered;
        }

        function getStageStatus(file, stage) {
            const stageData = file.stages[stage];
            if (!stageData) {
                if (file.status === 'pending') return { status: 'pending', duration: null };
                if (file.status === 'processing' && file.currentStage === stage) {
                    return { status: 'processing', duration: null };
                }
                const currentIndex = stages.indexOf(file.currentStage);
                const stageIndex = stages.indexOf(stage);
                if (file.status === 'processing' && currentIndex > stageIndex) {
                    return { status: 'completed', duration: null };
                }
                return { status: 'pending', duration: null };
            }
            return {
                status: stageData.status || 'completed',
                duration: stageData.duration || null,
                error: stageData.error || null
            };
        }

        function getStatusIcon(status) {
            switch(status) {
                case 'pending':
                    return '⚪'; // Circle outline
                case 'processing':
                    return '⟳'; // Circular arrow (spinning)
                case 'completed':
                    return '✓'; // Checkmark
                case 'failed':
                    return '✗'; // X mark
                case 'skipped':
                    return '⊘'; // Slashed circle
                case 'warning':
                    return '⚠'; // Warning triangle
                default:
                    return '○'; // Default circle
            }
        }

        function getStatusIconCodicon(status) {
            // Using VS Code codicons (available in webview context)
            switch(status) {
                case 'pending':
                    return '<span class="codicon codicon-circle-outline"></span>';
                case 'processing':
                    return '<span class="codicon codicon-sync codicon-modifier-spin"></span>';
                case 'completed':
                    return '<span class="codicon codicon-check"></span>';
                case 'failed':
                    return '<span class="codicon codicon-error"></span>';
                case 'skipped':
                    return '<span class="codicon codicon-circle-slash"></span>';
                case 'warning':
                    return '<span class="codicon codicon-warning"></span>';
                default:
                    return '<span class="codicon codicon-circle"></span>';
            }
        }

        function getFileStatusIcon(status) {
            switch(status) {
                case 'pending':
                    return '<span class="codicon codicon-circle-outline status-icon"></span>';
                case 'processing':
                    return '<span class="codicon codicon-sync codicon-modifier-spin status-icon"></span>';
                case 'completed':
                    return '<span class="codicon codicon-check status-icon"></span>';
                case 'failed':
                    return '<span class="codicon codicon-error status-icon"></span>';
                case 'skipped':
                    return '<span class="codicon codicon-circle-slash status-icon"></span>';
                case 'warning':
                    return '<span class="codicon codicon-warning status-icon"></span>';
                default:
                    return '<span class="codicon codicon-circle status-icon"></span>';
            }
        }

        function formatDuration(ms) {
            if (!ms) return '';
            if (ms < 1000) return ms + 'ms';
            if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
            return (ms / 60000).toFixed(1) + 'm';
        }

        function updateTable() {
            const tbody = document.getElementById('tableBody');
            const stageHeaders = document.getElementById('stageHeaders');
            
            // Update stage headers
            stageHeaders.innerHTML = stages.map(s => 
                \`<th>\${s.charAt(0).toUpperCase() + s.slice(1)}</th>\`
            ).join('');

            if (files.length === 0) {
                tbody.innerHTML = \`
                    <tr>
                        <td colspan="\${stages.length + 2}" class="empty-state">
                            <h2>No files being processed</h2>
                            <p>Click "Rebuild Everything" to start indexing</p>
                        </td>
                    </tr>
                \`;
                return;
            }

            tbody.innerHTML = files.map(file => {
                const stageCells = stages.map(stage => {
                    const stageInfo = getStageStatus(file, stage);
                    const durationText = stageInfo.duration ? \`<div class="stage-duration">\${formatDuration(stageInfo.duration)}</div>\` : '';
                    const icon = getStatusIconCodicon(stageInfo.status);
                    const errorTooltip = stageInfo.error ? \` title="\${stageInfo.error}"\` : '';
                    return \`
                        <td class="stage-cell"\${errorTooltip}>
                            <div class="stage-cell-content">
                                <span class="stage-status \${stageInfo.status}">\${icon}</span>
                                \${durationText}
                            </div>
                        </td>
                    \`;
                }).join('');

                const statusIcon = getFileStatusIcon(file.status);
                const statusBadge = \`<span class="status-badge \${file.status}">\${statusIcon}\${file.status}</span>\`;
                const errorText = file.error ? \`<div class="error-text" title="\${file.error}">\${file.error}</div>\` : '';
                const totalDuration = formatDuration(file.duration);

                return \`
                    <tr>
                        <td title="\${file.filePath}">\${file.filePath}</td>
                        <td>
                            \${statusBadge}
                            \${errorText}
                        </td>
                        <td>\${totalDuration}</td>
                        \${stageCells}
                    </tr>
                \`;
            }).join('');
            
            // Update sort indicators
            document.querySelectorAll('th.sortable').forEach(th => {
                th.classList.remove('sort-asc', 'sort-desc');
                const col = th.getAttribute('onclick')?.match(/'([^']+)'/)?.[1];
                if (col === sortColumn) {
                    th.classList.add(sortDirection === 'asc' ? 'sort-asc' : 'sort-desc');
                }
            });
        }

        function updateStats(data) {
            document.getElementById('totalFiles').textContent = data.totalFiles || 0;
            document.getElementById('processingFiles').textContent = data.processingFiles || 0;
            document.getElementById('completedFiles').textContent = data.completedFiles || 0;
            document.getElementById('failedFiles').textContent = data.failedFiles || 0;
        }

        window.addEventListener('message', event => {
            const message = event.data;
            if (message.type === 'update') {
                stages = message.data.stages || [];
                allFiles = message.data.files || [];
                applyFilters();
                updateStats(message.data);
                updateTable();
            }
        });
    </script>
</body>
</html>`;
  }
}

interface FileStatus {
  filePath: string;
  stages: Map<string, StageStatus>;
  currentStage: string;
  status: "pending" | "processing" | "completed" | "failed" | "skipped" | "warning";
  error?: string;
  startedAt: Date;
}

interface StageStatus {
  status: "pending" | "processing" | "completed" | "failed" | "skipped" | "warning";
  error?: string;
  completedAt: Date;
}

