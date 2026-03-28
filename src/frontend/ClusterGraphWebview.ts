/**
 * ClusterGraphWebview - Interactive visualization of document clusters using D3.js
 *
 * Features:
 * - Force-directed graph layout
 * - Color-coded clusters
 * - Edge types (semantic, temporal, entity, structural)
 * - Zoom/pan interaction
 * - Click to open files
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { GrpcClusteringClient, DocumentGraphData, ClusterNode, ClusterEdge } from '../core/GrpcClusteringClient';

/**
 * Provider for the Cluster Graph Webview Panel
 */
export class ClusterGraphWebviewProvider {
  public static readonly viewType = 'cortex.clusterGraph';
  private panel: vscode.WebviewPanel | undefined;
  private readonly clusteringClient: GrpcClusteringClient;
  private readonly extensionUri: vscode.Uri;
  private readonly workspaceId: string;
  private readonly workspaceRoot: string;

  constructor(
    private readonly context: vscode.ExtensionContext,
    workspaceId: string,
    workspaceRoot: string
  ) {
    this.extensionUri = context.extensionUri;
    this.workspaceId = workspaceId;
    this.workspaceRoot = workspaceRoot;
    this.clusteringClient = new GrpcClusteringClient(context);
  }

  /**
   * Show the cluster graph panel
   */
  async show(): Promise<void> {
    if (this.panel) {
      this.panel.reveal(vscode.ViewColumn.Beside);
      await this.refresh();
      return;
    }

    this.panel = vscode.window.createWebviewPanel(
      ClusterGraphWebviewProvider.viewType,
      'Cluster Graph',
      vscode.ViewColumn.Beside,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
        localResourceRoots: [this.extensionUri],
      }
    );

    this.panel.onDidDispose(() => {
      this.panel = undefined;
    });

    this.panel.webview.onDidReceiveMessage(
      async (message) => {
        await this.handleMessage(message);
      },
      undefined,
      this.context.subscriptions
    );

    await this.refresh();
  }

  /**
   * Refresh the graph data
   */
  async refresh(): Promise<void> {
    if (!this.panel) return;

    try {
      const graphData = await this.clusteringClient.getDocumentGraph(this.workspaceId, 0.1);
      const clusters = await this.clusteringClient.getClusters(this.workspaceId);

      // Build cluster color map
      const clusterColors = new Map<string, string>();
      const colorPalette = [
        '#4285f4', '#ea4335', '#34a853', '#fbbc05', '#ff6d01',
        '#46bdc6', '#7b1fa2', '#c2185b', '#00897b', '#5e35b1',
      ];
      clusters.forEach((cluster, index) => {
        clusterColors.set(cluster.id, colorPalette[index % colorPalette.length]);
      });

      this.panel.webview.html = this.getHtml(graphData, clusterColors, clusters);
    } catch (error) {
      console.error('[ClusterGraph] Error loading graph:', error);
      this.panel.webview.html = this.getErrorHtml(error as Error);
    }
  }

  /**
   * Handle messages from the webview
   */
  private async handleMessage(message: { command: string; [key: string]: any }): Promise<void> {
    switch (message.command) {
      case 'openFile':
        if (message.relativePath) {
          const fullPath = path.join(this.workspaceRoot, message.relativePath);
          const uri = vscode.Uri.file(fullPath);
          await vscode.commands.executeCommand('vscode.open', uri);
        }
        break;

      case 'refresh':
        await this.refresh();
        break;

      case 'runClustering':
        try {
          await vscode.window.withProgress(
            {
              location: vscode.ProgressLocation.Notification,
              title: 'Running clustering analysis...',
              cancellable: false,
            },
            async () => {
              await this.clusteringClient.runClustering(this.workspaceId);
              await this.refresh();
            }
          );
          vscode.window.showInformationMessage('Clustering analysis completed');
        } catch (error) {
          vscode.window.showErrorMessage(`Clustering failed: ${(error as Error).message}`);
        }
        break;

      case 'filterEdgeType':
        // Handled in JS
        break;
    }
  }

  /**
   * Generate HTML for the webview
   */
  private getHtml(
    graphData: DocumentGraphData,
    clusterColors: Map<string, string>,
    clusters: { id: string; name: string; memberCount: number }[]
  ): string {
    // Convert cluster colors to JSON
    const colorsJson = JSON.stringify(Object.fromEntries(clusterColors));
    const graphDataJson = JSON.stringify(graphData);
    const clustersJson = JSON.stringify(clusters);

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' https://d3js.org; style-src 'unsafe-inline';">
  <title>Cluster Graph</title>
  <style>
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }

    body {
      font-family: var(--vscode-font-family, 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif);
      background: var(--vscode-editor-background, #1e1e1e);
      color: var(--vscode-editor-foreground, #d4d4d4);
      overflow: hidden;
    }

    .toolbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 8px 12px;
      background: var(--vscode-sideBar-background, #252526);
      border-bottom: 1px solid var(--vscode-panel-border, #454545);
    }

    .toolbar button {
      padding: 4px 10px;
      background: var(--vscode-button-background, #0e639c);
      color: var(--vscode-button-foreground, #ffffff);
      border: none;
      border-radius: 3px;
      cursor: pointer;
      font-size: 12px;
    }

    .toolbar button:hover {
      background: var(--vscode-button-hoverBackground, #1177bb);
    }

    .toolbar select {
      padding: 4px 8px;
      background: var(--vscode-input-background, #3c3c3c);
      color: var(--vscode-input-foreground, #cccccc);
      border: 1px solid var(--vscode-input-border, #3c3c3c);
      border-radius: 3px;
      font-size: 12px;
    }

    .toolbar .stats {
      margin-left: auto;
      font-size: 11px;
      color: var(--vscode-descriptionForeground, #808080);
    }

    #graph-container {
      width: 100%;
      height: calc(100vh - 45px);
    }

    svg {
      width: 100%;
      height: 100%;
    }

    .node {
      cursor: pointer;
    }

    .node circle {
      stroke: var(--vscode-editor-foreground, #d4d4d4);
      stroke-width: 1.5px;
      transition: stroke-width 0.2s, r 0.2s;
    }

    .node:hover circle {
      stroke-width: 3px;
    }

    .node.central circle {
      stroke-width: 2.5px;
      stroke: gold;
    }

    .node text {
      font-size: 10px;
      fill: var(--vscode-editor-foreground, #d4d4d4);
      pointer-events: none;
      text-anchor: middle;
      dominant-baseline: middle;
    }

    .link {
      stroke-opacity: 0.6;
      transition: stroke-opacity 0.2s, stroke-width 0.2s;
    }

    .link:hover {
      stroke-opacity: 1;
      stroke-width: 3px;
    }

    .link.semantic { stroke: #4285f4; }
    .link.temporal { stroke: #fbbc05; }
    .link.entity { stroke: #34a853; }
    .link.structural { stroke: #ea4335; }

    .legend {
      position: absolute;
      bottom: 20px;
      left: 20px;
      background: var(--vscode-sideBar-background, #252526);
      border: 1px solid var(--vscode-panel-border, #454545);
      border-radius: 4px;
      padding: 10px;
      font-size: 11px;
    }

    .legend-title {
      font-weight: bold;
      margin-bottom: 6px;
    }

    .legend-item {
      display: flex;
      align-items: center;
      gap: 6px;
      margin-bottom: 4px;
    }

    .legend-color {
      width: 16px;
      height: 3px;
      border-radius: 1px;
    }

    .tooltip {
      position: absolute;
      padding: 8px 12px;
      background: var(--vscode-editorWidget-background, #2d2d30);
      border: 1px solid var(--vscode-editorWidget-border, #454545);
      border-radius: 4px;
      font-size: 12px;
      pointer-events: none;
      opacity: 0;
      transition: opacity 0.15s;
      z-index: 1000;
      max-width: 300px;
    }

    .tooltip.visible {
      opacity: 1;
    }

    .tooltip-title {
      font-weight: bold;
      margin-bottom: 4px;
    }

    .tooltip-info {
      color: var(--vscode-descriptionForeground, #808080);
      font-size: 11px;
    }

    .empty-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: calc(100vh - 45px);
      color: var(--vscode-descriptionForeground, #808080);
    }

    .empty-state h2 {
      margin-bottom: 10px;
    }

    .empty-state button {
      margin-top: 15px;
      padding: 8px 16px;
      background: var(--vscode-button-background, #0e639c);
      color: var(--vscode-button-foreground, #ffffff);
      border: none;
      border-radius: 4px;
      cursor: pointer;
      font-size: 13px;
    }
  </style>
</head>
<body>
  <div class="toolbar">
    <button onclick="refresh()">↻ Refresh</button>
    <button onclick="runClustering()">⚡ Run Clustering</button>
    <select id="edge-filter" onchange="filterEdges(this.value)">
      <option value="all">All Edges</option>
      <option value="semantic">Semantic Only</option>
      <option value="temporal">Temporal Only</option>
      <option value="entity">Entity Only</option>
      <option value="structural">Structural Only</option>
    </select>
    <span class="stats">Nodes: ${graphData.totalNodes} | Edges: ${graphData.totalEdges}</span>
  </div>

  <div id="graph-container"></div>

  <div class="legend">
    <div class="legend-title">Edge Types</div>
    <div class="legend-item"><div class="legend-color" style="background: #4285f4;"></div> Semantic</div>
    <div class="legend-item"><div class="legend-color" style="background: #fbbc05;"></div> Temporal</div>
    <div class="legend-item"><div class="legend-color" style="background: #34a853;"></div> Entity</div>
    <div class="legend-item"><div class="legend-color" style="background: #ea4335;"></div> Structural</div>
  </div>

  <div id="tooltip" class="tooltip">
    <div class="tooltip-title"></div>
    <div class="tooltip-info"></div>
  </div>

  <script src="https://d3js.org/d3.v7.min.js"></script>
  <script>
    const vscode = acquireVsCodeApi();
    const graphData = ${graphDataJson};
    const clusterColors = ${colorsJson};
    const clusters = ${clustersJson};

    let currentFilter = 'all';
    let simulation;

    function init() {
      if (graphData.nodes.length === 0) {
        showEmptyState();
        return;
      }

      const container = document.getElementById('graph-container');
      const width = container.clientWidth;
      const height = container.clientHeight;

      const svg = d3.select('#graph-container')
        .append('svg')
        .attr('width', width)
        .attr('height', height);

      // Add zoom behavior
      const g = svg.append('g');
      const zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => {
          g.attr('transform', event.transform);
        });
      svg.call(zoom);

      // Create edges
      const links = g.append('g')
        .selectAll('line')
        .data(graphData.edges)
        .enter()
        .append('line')
        .attr('class', d => 'link ' + d.edgeType.toLowerCase())
        .attr('stroke-width', d => Math.max(1, d.weight * 3))
        .attr('data-type', d => d.edgeType.toLowerCase());

      // Create nodes
      const nodes = g.append('g')
        .selectAll('.node')
        .data(graphData.nodes)
        .enter()
        .append('g')
        .attr('class', d => {
          const centralNodes = clusters.find(c => c.id === d.clusterId)?.centralNodes || [];
          return 'node' + (centralNodes.includes(d.id) ? ' central' : '');
        })
        .call(d3.drag()
          .on('start', dragstarted)
          .on('drag', dragged)
          .on('end', dragended))
        .on('click', (event, d) => {
          if (d.nodeType === 'document') {
            vscode.postMessage({ command: 'openFile', relativePath: d.label });
          }
        })
        .on('mouseover', showTooltip)
        .on('mouseout', hideTooltip);

      nodes.append('circle')
        .attr('r', d => d.nodeType === 'cluster' ? 12 : 6)
        .attr('fill', d => clusterColors[d.clusterId] || '#808080');

      nodes.append('text')
        .attr('dy', d => d.nodeType === 'cluster' ? 20 : 15)
        .text(d => {
          const label = d.label || '';
          return label.length > 20 ? label.substring(0, 17) + '...' : label;
        });

      // Create force simulation
      simulation = d3.forceSimulation(graphData.nodes)
        .force('link', d3.forceLink(graphData.edges)
          .id(d => d.id)
          .distance(d => 50 + (1 - d.weight) * 100))
        .force('charge', d3.forceManyBody().strength(-100))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(20));

      simulation.on('tick', () => {
        links
          .attr('x1', d => d.source.x)
          .attr('y1', d => d.source.y)
          .attr('x2', d => d.target.x)
          .attr('y2', d => d.target.y);

        nodes.attr('transform', d => 'translate(' + d.x + ',' + d.y + ')');
      });

      // Zoom to fit initially
      setTimeout(() => {
        const bounds = g.node().getBBox();
        const dx = bounds.width;
        const dy = bounds.height;
        const x = bounds.x + dx / 2;
        const y = bounds.y + dy / 2;
        const scale = Math.min(0.9 * width / dx, 0.9 * height / dy, 2);
        const translate = [width / 2 - scale * x, height / 2 - scale * y];
        svg.transition()
          .duration(750)
          .call(zoom.transform, d3.zoomIdentity.translate(translate[0], translate[1]).scale(scale));
      }, 500);
    }

    function showEmptyState() {
      document.getElementById('graph-container').innerHTML =
        '<div class="empty-state">' +
        '<h2>No Clusters Found</h2>' +
        '<p>Run clustering analysis to discover document communities.</p>' +
        '<button onclick="runClustering()">⚡ Run Clustering</button>' +
        '</div>';
    }

    function dragstarted(event, d) {
      if (!event.active) simulation.alphaTarget(0.3).restart();
      d.fx = d.x;
      d.fy = d.y;
    }

    function dragged(event, d) {
      d.fx = event.x;
      d.fy = event.y;
    }

    function dragended(event, d) {
      if (!event.active) simulation.alphaTarget(0);
      d.fx = null;
      d.fy = null;
    }

    function showTooltip(event, d) {
      const tooltip = document.getElementById('tooltip');
      const clusterName = clusters.find(c => c.id === d.clusterId)?.name || 'Unknown';

      tooltip.querySelector('.tooltip-title').textContent = d.label;
      tooltip.querySelector('.tooltip-info').innerHTML =
        'Type: ' + d.nodeType + '<br>' +
        'Cluster: ' + clusterName;

      tooltip.style.left = (event.pageX + 10) + 'px';
      tooltip.style.top = (event.pageY - 20) + 'px';
      tooltip.classList.add('visible');
    }

    function hideTooltip() {
      document.getElementById('tooltip').classList.remove('visible');
    }

    function filterEdges(type) {
      currentFilter = type;
      d3.selectAll('.link').style('display', function() {
        if (type === 'all') return 'block';
        return this.getAttribute('data-type') === type ? 'block' : 'none';
      });
    }

    function refresh() {
      vscode.postMessage({ command: 'refresh' });
    }

    function runClustering() {
      vscode.postMessage({ command: 'runClustering' });
    }

    // Initialize
    init();
  </script>
</body>
</html>`;
  }

  /**
   * Generate error HTML
   */
  private getErrorHtml(error: Error): string {
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cluster Graph - Error</title>
  <style>
    body {
      font-family: var(--vscode-font-family);
      background: var(--vscode-editor-background);
      color: var(--vscode-editor-foreground);
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100vh;
      margin: 0;
    }
    .error-icon { font-size: 48px; margin-bottom: 20px; }
    .error-title { font-size: 18px; font-weight: bold; margin-bottom: 10px; }
    .error-message { color: var(--vscode-errorForeground); max-width: 400px; text-align: center; }
    button {
      margin-top: 20px;
      padding: 8px 16px;
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
      border: none;
      border-radius: 4px;
      cursor: pointer;
    }
  </style>
</head>
<body>
  <div class="error-icon">⚠️</div>
  <div class="error-title">Failed to load cluster graph</div>
  <div class="error-message">${error.message}</div>
  <button onclick="location.reload()">Retry</button>
</body>
</html>`;
  }

  /**
   * Dispose resources
   */
  dispose(): void {
    this.panel?.dispose();
  }
}

/**
 * Register cluster graph commands
 */
export function registerClusterGraphCommands(
  context: vscode.ExtensionContext,
  workspaceId: string,
  workspaceRoot: string
): void {
  let provider: ClusterGraphWebviewProvider | undefined;

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex.openClusterGraph', async () => {
      if (!provider) {
        provider = new ClusterGraphWebviewProvider(context, workspaceId, workspaceRoot);
      }
      await provider.show();
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex.runClustering', async () => {
      const clusteringClient = new GrpcClusteringClient(context);
      try {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: 'Running clustering analysis...',
            cancellable: false,
          },
          async () => {
            const result = await clusteringClient.runClustering(workspaceId);
            if (result.success) {
              vscode.window.showInformationMessage(
                `Clustering complete: ${result.clustersCreated} clusters created, ${result.documentsAssigned} documents assigned`
              );
            } else {
              vscode.window.showWarningMessage(`Clustering: ${result.message}`);
            }
          }
        );
      } catch (error) {
        vscode.window.showErrorMessage(`Clustering failed: ${(error as Error).message}`);
      }
    })
  );
}
