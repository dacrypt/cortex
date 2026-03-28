/**
 * ClusterFacetTreeProvider - Provides clustering facet for UnifiedFacetTreeProvider
 *
 * Uses GrpcClusteringClient to fetch document clusters and display them as facet terms.
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { IFacetProvider, IFacetTreeItem, FacetItemPayload, FacetConfig, FacetType, FacetCategory } from './contracts/IFacetProvider';
import { ViewNodeKind } from './contracts/viewNodes';
import { GrpcClusteringClient, DocumentCluster } from '../core/GrpcClusteringClient';

/**
 * Tree item for cluster facet
 */
class ClusterFacetTreeItem extends vscode.TreeItem implements IFacetTreeItem {
  id: string;
  kind: ViewNodeKind;
  facet?: string;
  value?: string;
  count?: number;
  payload?: FacetItemPayload;
  isFile?: boolean;
  term?: string;

  constructor(
    id: string,
    kind: ViewNodeKind,
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState,
    options: {
      facet?: string;
      value?: string;
      count?: number;
      payload?: FacetItemPayload;
      isFile?: boolean;
      term?: string;
      resourceUri?: vscode.Uri;
      icon?: vscode.ThemeIcon;
      description?: string;
      tooltip?: string | vscode.MarkdownString;
      contextValue?: string;
    } = {}
  ) {
    super(label, collapsibleState);

    this.id = id;
    this.kind = kind;
    this.facet = options.facet;
    this.value = options.value;
    this.count = options.count;
    this.payload = options.payload;
    this.isFile = options.isFile;
    this.term = options.term;

    if (options.description) this.description = options.description;
    if (options.tooltip) this.tooltip = options.tooltip;
    if (options.icon) this.iconPath = options.icon;
    if (options.contextValue) this.contextValue = options.contextValue;

    if (options.isFile && options.resourceUri) {
      this.resourceUri = options.resourceUri;
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [options.resourceUri],
      };
      if (!this.iconPath) this.iconPath = vscode.ThemeIcon.File;
      if (!this.contextValue) this.contextValue = 'cortex-file';
    }
  }
}

/**
 * Cluster Facet Tree Provider
 *
 * Displays document clusters as facet terms, allowing drill-down to see cluster members.
 */
export class ClusterFacetTreeProvider implements IFacetProvider {
  readonly config: FacetConfig;
  readonly workspaceRoot: string;
  readonly workspaceId: string;
  private readonly clusteringClient: GrpcClusteringClient;
  private cachedClusters: DocumentCluster[] | null = null;

  constructor(
    workspaceRoot: string,
    context: vscode.ExtensionContext,
    workspaceId: string
  ) {
    this.workspaceRoot = workspaceRoot;
    this.workspaceId = workspaceId;
    this.clusteringClient = new GrpcClusteringClient(context);
    this.config = {
      field: 'cluster',
      label: 'By Cluster',
      type: FacetType.Terms,
      category: FacetCategory.Organization,
      description: 'Group files by semantic cluster (AI-detected document communities)',
      icon: new vscode.ThemeIcon('group-by-ref-type'),
    };
  }

  /**
   * Get tree item representation
   */
  getTreeItem(element: IFacetTreeItem): vscode.TreeItem {
    return element as vscode.TreeItem;
  }

  /**
   * Get children for tree view
   */
  async getChildren(element?: IFacetTreeItem): Promise<IFacetTreeItem[]> {
    console.log('[ClusterFacet] getChildren called', {
      hasElement: !!element,
      elementKind: element?.kind,
      elementId: element?.id,
      workspaceId: this.workspaceId
    });
    try {
      // Root level: show all clusters
      if (!element) {
        console.log('[ClusterFacet] Getting root clusters');
        const result = await this.getClusters();
        console.log('[ClusterFacet] getClusters returned', result.length, 'items');
        return result;
      }

      // Cluster level: show members
      if (element.kind === 'facet' && element.value) {
        console.log(`[ClusterFacet] Getting members for cluster: ${element.value} (kind: ${element.kind}, id: ${element.id})`);
        const members = await this.getClusterMembers(element.value);
        console.log(`[ClusterFacet] Found ${members.length} members for cluster ${element.value}`);
        return members;
      }

      console.warn('[ClusterFacet] Unexpected element:', { 
        kind: element?.kind, 
        value: element?.value, 
        id: element?.id 
      });
      return [];
    } catch (error) {
      console.error('[ClusterFacet] Error getting children:', error);
      
      // Check if service is not implemented
      const errorMessage = error instanceof Error ? error.message : String(error);
      if (errorMessage.includes('UNIMPLEMENTED') || errorMessage.includes('unknown service')) {
        return [this.createEmptyItem('Clustering service is not available. This feature requires backend support.')];
      }
      
      return [this.createErrorItem(error as Error)];
    }
  }

  /**
   * Get all clusters
   */
  private async getClusters(): Promise<ClusterFacetTreeItem[]> {
    try {
      console.log(`[ClusterFacet] Fetching clusters for workspaceId: ${this.workspaceId}`);
      // Always fetch fresh data from backend (don't use cache)
      // The cache is only used for getClusterMembers to avoid repeated calls
      const clusters = await this.clusteringClient.getClusters(this.workspaceId);
      console.log(`[ClusterFacet] Received ${clusters.length} clusters from backend`);

      if (clusters.length === 0) {
        console.log('[ClusterFacet] No clusters found, showing empty item');
        return [this.createEmptyItem(
          'No clusters found. Use command "Cortex: Run Clustering Analysis" to discover document clusters.',
          'cortex.runClustering'
        )];
      }

      console.log('[ClusterFacet] Clusters received:', clusters.map(c => ({
        id: c.id,
        name: c.name,
        status: c.status,
        memberCount: c.memberCount,
        confidence: c.confidence
      })));

      // Update cache for getClusterMembers
      this.cachedClusters = clusters;

      return clusters.map((cluster, index) => {
        const memberCount = cluster.memberCount;
        // Generate a name if cluster doesn't have one
        // Use cluster ID for uniqueness instead of index to avoid duplicates
        const clusterName = cluster.name?.trim() || `Cluster ${cluster.id.substring(0, 8)}`;
        const label = `${clusterName} (${memberCount})`;
        
        // Ensure term is always the generated name, not the ID
        const clusterTerm = clusterName;

        // Build tooltip with cluster details
        const tooltipParts: string[] = [
          `**${clusterName}**`,
          '',
          cluster.summary || 'No summary available',
          '',
          `Confidence: ${(cluster.confidence * 100).toFixed(0)}%`,
          `Members: ${memberCount} files`,
        ];

        if (cluster.topEntities.length > 0) {
          tooltipParts.push(`Top Entities: ${cluster.topEntities.slice(0, 5).join(', ')}`);
        }
        if (cluster.topKeywords.length > 0) {
          tooltipParts.push(`Keywords: ${cluster.topKeywords.slice(0, 5).join(', ')}`);
        }

        return new ClusterFacetTreeItem(
          `cluster:${cluster.id}`,
          'facet',
          label,
          vscode.TreeItemCollapsibleState.Collapsed,
          {
            facet: 'cluster',
            value: cluster.id,
            count: memberCount,
            term: clusterTerm, // Always use the generated/actual name, never the ID
            icon: new vscode.ThemeIcon('group-by-ref-type'),
            tooltip: new vscode.MarkdownString(tooltipParts.join('\n')),
            description: `${(cluster.confidence * 100).toFixed(0)}% confidence`,
            contextValue: 'cortex-cluster',
            payload: {
              facet: 'cluster',
              value: cluster.id,
              metadata: { cluster },
            },
          }
        );
      });
    } catch (error) {
      console.error('[ClusterFacet] Error getting clusters:', error);
      
      // Check if service is not implemented
      const errorMessage = error instanceof Error ? error.message : String(error);
      if (errorMessage.includes('UNIMPLEMENTED') || errorMessage.includes('unknown service')) {
        return [this.createEmptyItem('Clustering service is not available. This feature requires backend support.')];
      }
      
      return [this.createErrorItem(error as Error)];
    }
  }

  /**
   * Get cluster members (files)
   */
  private async getClusterMembers(clusterId: string): Promise<ClusterFacetTreeItem[]> {
    try {
      console.log(`[ClusterFacet] Calling getClusterMembers for clusterId: ${clusterId}, workspaceId: ${this.workspaceId}`);
      const members = await this.clusteringClient.getClusterMembers(
        this.workspaceId,
        clusterId
      );
      console.log(`[ClusterFacet] Received ${members.length} members from backend`);

      if (members.length === 0) {
        console.warn(`[ClusterFacet] No members returned for cluster ${clusterId}`);
        return [this.createEmptyItem('No files in this cluster')];
      }

      return members.map((member) => {
        const fullPath = path.join(this.workspaceRoot, member.relativePath);
        const uri = vscode.Uri.file(fullPath);
        const filename = member.filename || path.basename(member.relativePath);

        // Build tooltip
        const tooltipParts: string[] = [
          `**${filename}**`,
          '',
          `Path: ${member.relativePath}`,
          `Score: ${(member.membershipScore * 100).toFixed(0)}%`,
        ];

        if (member.isCentral) {
          tooltipParts.push('⭐ Central node (representative of this cluster)');
        }

        return new ClusterFacetTreeItem(
          `cluster:${clusterId}:file:${member.documentId}`,
          'file',
          filename,
          vscode.TreeItemCollapsibleState.None,
          {
            facet: 'cluster',
            value: clusterId,
            isFile: true,
            resourceUri: uri,
            icon: member.isCentral
              ? new vscode.ThemeIcon('star-full')
              : vscode.ThemeIcon.File,
            tooltip: new vscode.MarkdownString(tooltipParts.join('\n')),
            description: member.isCentral
              ? `⭐ ${(member.membershipScore * 100).toFixed(0)}%`
              : `${(member.membershipScore * 100).toFixed(0)}%`,
            contextValue: 'cortex-file',
            payload: {
              facet: 'cluster',
              value: member.relativePath,
              metadata: { member, clusterId },
            },
          }
        );
      });
    } catch (error) {
      console.error('[ClusterFacet] Error getting cluster members:', error);
      
      // Check if service is not implemented
      const errorMessage = error instanceof Error ? error.message : String(error);
      if (errorMessage.includes('UNIMPLEMENTED') || errorMessage.includes('unknown service')) {
        return [this.createEmptyItem('Clustering service is not available.')];
      }
      
      return [this.createErrorItem(error as Error)];
    }
  }

  /**
   * Get files for a specific term (cluster name/id)
   * This is used by UnifiedFacetTreeProvider for direct term lookup
   */
  async getFilesForTerm(term: string, _field: string): Promise<IFacetTreeItem[]> {
    console.log(`[ClusterFacet] getFilesForTerm called with term: "${term}"`);
    // Find cluster by name or id - always fetch fresh data
    const clusters = await this.clusteringClient.getClusters(this.workspaceId);
    this.cachedClusters = clusters; // Update cache for getClusterMembers

    console.log(`[ClusterFacet] Searching through ${clusters.length} clusters`);
    
    // Try to find by ID first (most reliable)
    let cluster = clusters.find((c) => c.id === term);
    
    // If not found by ID, try by name
    if (!cluster) {
      cluster = clusters.find((c) => {
        const clusterName = c.name?.trim() || `Cluster ${c.id.substring(0, 8)}`;
        return clusterName === term || c.name === term;
      });
    }
    
    // Also try matching against the label format "Cluster Name (count)"
    if (!cluster) {
      const termWithoutCount = term.split(' (')[0].trim();
      cluster = clusters.find((c) => {
        const clusterName = c.name?.trim() || `Cluster ${c.id.substring(0, 8)}`;
        return clusterName === termWithoutCount || c.name === termWithoutCount;
      });
    }

    if (!cluster) {
      console.warn(`[ClusterFacet] Cluster not found for term: "${term}"`);
      console.log(`[ClusterFacet] Available clusters:`, clusters.map(c => ({
        id: c.id,
        name: c.name || `Cluster ${c.id.substring(0, 8)}`
      })));
      return [];
    }

    console.log(`[ClusterFacet] Found cluster: ${cluster.id} (${cluster.name || `Cluster ${cluster.id.substring(0, 8)}`})`);
    return this.getClusterMembers(cluster.id);
  }

  /**
   * Create empty placeholder item
   */
  private createEmptyItem(message: string, command?: string): ClusterFacetTreeItem {
    const item = new ClusterFacetTreeItem(
      'cluster:empty',
      'group',
      message,
      vscode.TreeItemCollapsibleState.None,
      {
        icon: new vscode.ThemeIcon('info'),
        contextValue: command ? 'cortex-empty-actionable' : 'cortex-empty',
      }
    );
    
    // Make it clickable if a command is provided
    if (command) {
      item.command = {
        command,
        title: 'Run Clustering Analysis',
        arguments: [],
      };
    }
    
    return item;
  }

  /**
   * Create error item
   */
  private createErrorItem(error: Error): ClusterFacetTreeItem {
    return new ClusterFacetTreeItem(
      'cluster:error',
      'group',
      `Error: ${error.message}`,
      vscode.TreeItemCollapsibleState.None,
      {
        icon: new vscode.ThemeIcon('error'),
        tooltip: error.stack,
        contextValue: 'cortex-error',
      }
    );
  }

  /**
   * Refresh cached data
   */
  refresh(): void {
    this.cachedClusters = null;
  }

  /**
   * Dispose resources
   */
  dispose(): void {
    // Client is managed internally, no explicit close needed
  }
}
