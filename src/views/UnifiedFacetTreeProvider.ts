/**
 * UnifiedFacetTreeProvider - Generates facet tree dynamically from FacetRegistry
 * 
 * This is the main provider that replaces all individual facet providers.
 * It generates the entire tree structure: Categories → Facets → Values → Files
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import * as crypto from 'node:crypto';
import { BaseFacetTreeProvider } from './base/BaseFacetTreeProvider';
import {
  IFacetTreeItem,
  FacetType,
  FacetCategory,
  FacetProviderContext,
  FacetItemPayload,
  IFacetProvider,
} from './contracts/IFacetProvider';
import { ViewNodeKind } from './contracts/viewNodes';
import { getFacetRegistry } from './contracts/FacetRegistry';
import { FacetProviderFactory } from './base/FacetProviderFactory';
import { GrpcEntityClient } from '../core/GrpcEntityClient';
import { Entity } from '../models/entity';
import { t } from './i18n';

/**
 * Unified facet tree item
 */
class UnifiedFacetTreeItem extends vscode.TreeItem implements IFacetTreeItem {
  id: string;
  kind: ViewNodeKind;
  facet?: string;
  value?: string;
  count?: number;
  payload?: FacetItemPayload;
  isFile?: boolean;

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

    if (options.description) this.description = options.description;
    if (options.tooltip) this.tooltip = options.tooltip;
    if (options.icon) this.iconPath = options.icon;
    if (options.contextValue) this.contextValue = options.contextValue;

    if (options.isFile && options.resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [options.resourceUri],
      };
      if (!this.iconPath) this.iconPath = vscode.ThemeIcon.File;
      if (!this.contextValue) this.contextValue = 'cortex-file';
    } else if (kind === 'facet' && !this.iconPath) {
      this.iconPath = new vscode.ThemeIcon('tag');
      if (!this.contextValue) this.contextValue = 'cortex-facet-term';
    } else if (kind === 'group' && !this.iconPath) {
      this.iconPath = new vscode.ThemeIcon('library');
      if (!this.contextValue) this.contextValue = 'cortex-facet-group';
    }
  }
}

/**
 * Unified Facet Tree Provider
 * 
 * Generates the entire facet tree dynamically from FacetRegistry.
 * Structure:
 * - Root: Categories
 * - Category: Facets in that category
 * - Facet: Values for that facet
 * - Value: Files matching that value
 */
// Provider with optional getFilesForTerm method
interface ProviderWithGetFilesForTerm extends IFacetProvider {
  getFilesForTerm?: (term: string, field: string) => Promise<IFacetTreeItem[]>;
}

export class UnifiedFacetTreeProvider extends BaseFacetTreeProvider<UnifiedFacetTreeItem> {
  private readonly registry = getFacetRegistry();
  private readonly facetProviders = new Map<string, IFacetProvider>(); // Cache of facet providers
  private currentFacet?: string; // Track current facet for context
  private readonly entityClient: GrpcEntityClient | null = null;
  private readonly extensionContext?: vscode.ExtensionContext;
  private readonly categoryLabels: Record<FacetCategory, string> = {
    [FacetCategory.Core]: 'Core',
    [FacetCategory.Organization]: 'Organization',
    [FacetCategory.Temporal]: 'Temporal',
    [FacetCategory.Content]: 'Content',
    [FacetCategory.System]: 'System',
    [FacetCategory.Specialized]: 'Specialized',
  };

  constructor(ctx: FacetProviderContext) {
    // Use a dummy config - we'll use the registry instead
    super(
      {
        field: '',
        label: 'Facetas',
        type: FacetType.Terms,
        category: FacetCategory.Core,
      },
      ctx
    );

    this.extensionContext = ctx.context;

    // Initialize entity client if context is available
    if (ctx.context) {
      try {
        this.entityClient = new GrpcEntityClient(ctx.context);
      } catch (error) {
        console.warn('[UnifiedFacetTree] Failed to initialize EntityClient:', error);
      }
    }
  }

  protected createTreeItem(params: {
    id: string;
    kind: ViewNodeKind;
    label: string;
    collapsibleState: vscode.TreeItemCollapsibleState;
    resourceUri?: vscode.Uri;
    isFile?: boolean;
    facet?: string;
    value?: string;
    count?: number;
    payload?: FacetItemPayload;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    icon?: vscode.ThemeIcon;
  }): UnifiedFacetTreeItem {
    return new UnifiedFacetTreeItem(
      params.id,
      params.kind,
      params.label,
      params.collapsibleState,
      {
        facet: params.facet,
        value: params.value,
        count: params.count,
        payload: params.payload,
        isFile: params.isFile,
        resourceUri: params.resourceUri,
        icon: params.icon,
        description: params.description,
        tooltip: params.tooltip,
      }
    );
  }

  async getChildren(element?: UnifiedFacetTreeItem): Promise<UnifiedFacetTreeItem[]> {
    try {
      // Root level: show Clusters at top, then categories
      if (!element) {
        return await this.getRootNodes();
      }

      // Clusters root node: show cluster values directly (sorted by activity)
      if (element.kind === 'group' && element.facet === 'clusters-root') {
        return await this.getClusterValues();
      }

      // Category level: show facets in this category
      if (element.kind === 'group' && element.facet === 'category') {
        const category = element.value as FacetCategory;
        return this.getFacetsInCategory(category);
      }

      // Facet level: show values for this facet
      if (element.kind === 'facet' && element.facet && !element.value) {
        return await this.getFacetValues(element.facet);
      }

      // Value level: show files matching this facet value
      if (element.kind === 'facet' && element.facet && element.value) {
        const facetConfig = this.registry.get(element.facet);
        console.log(`[UnifiedFacetTree] getChildren: clicked on facet value - field="${element.facet}", value="${element.value}", label="${element.label}"`);

        // For Structure (folder) facets, if clicking on a folder name with count like "Libros (8)", extract the folder name
        if (facetConfig?.type === FacetType.Structure && element.value.includes('(')) {
          // Extract folder name from "Libros (8)" -> "Libros"
          const folderName = element.value.split(' (')[0];
          console.log(`[UnifiedFacetTree] Extracting folder name from "${element.value}" -> "${folderName}"`);
          return await this.getFilesForFacetValue(element.facet, folderName);
        }

        // For cluster facet, ensure we're using the cluster ID (value), not the name
        if (element.facet === 'cluster') {
          console.log(`[UnifiedFacetTree] Cluster facet clicked - using value (cluster ID): "${element.value}"`);
        }

        return await this.getFilesForFacetValue(element.facet, element.value);
      }

      return [];
    } catch (error) {
      console.error('[UnifiedFacetTree] Error in getChildren:', error);
      return [this.createErrorItem(error as Error, 'getChildren')];
    }
  }

  /**
   * Get root nodes: Clusters at top (always expanded), then categories
   */
  private async getRootNodes(): Promise<UnifiedFacetTreeItem[]> {
    const nodes: UnifiedFacetTreeItem[] = [];

    // 1. Add Clusters root node at the top (always expanded)
    const clustersNode = this.createGroupItem(
      'clusters-root',
      'Clusters',
      vscode.TreeItemCollapsibleState.Expanded // Always expanded
    );
    clustersNode.facet = 'clusters-root';
    clustersNode.iconPath = new vscode.ThemeIcon('group-by-ref-type');
    clustersNode.contextValue = 'cortex-clusters-root';
    nodes.push(clustersNode);

    // 2. Add category nodes
    nodes.push(...this.getCategories());

    return nodes;
  }

  /**
   * Get cluster values directly, sorted by activity date (most recent first)
   */
  private async getClusterValues(): Promise<UnifiedFacetTreeItem[]> {
    try {
      // Get or create cluster provider
      let provider = this.facetProviders.get('cluster');
      if (!provider) {
        const facetConfig = this.registry.get('cluster');
        if (!facetConfig) {
          return [this.createEmptyPlaceholder('Cluster facet not configured', 'cluster')];
        }
        provider = FacetProviderFactory.create(facetConfig, {
          workspaceRoot: this.workspaceRoot,
          workspaceId: this.workspaceId,
          context: (this.extensionContext || {}) as vscode.ExtensionContext,
          fileCacheService: this.fileCacheService,
          knowledgeClient: this.knowledgeClient,
          adminClient: this.adminClient,
          metadataStore: this.metadataStore,
        });
        this.facetProviders.set('cluster', provider);
      }

      // Get clusters from provider
      const clusters = await provider.getChildren();
      if (!clusters || clusters.length === 0) {
        return [this.createEmptyPlaceholder(
          'No clusters found. Use "Cortex: Run Clustering" to analyze.',
          'cluster'
        )];
      }

      // Sort by updatedAt (most recent first) - clusters have updatedAt in payload
      interface ClusterItem extends IFacetTreeItem {
        term?: string;
        payload?: FacetItemPayload & {
          metadata?: {
            cluster?: {
              updatedAt?: number;
            };
          };
        };
      }

      const sortedClusters = [...clusters].sort((a: ClusterItem, b: ClusterItem) => {
        const aUpdated = a.payload?.metadata?.cluster?.updatedAt || 0;
        const bUpdated = b.payload?.metadata?.cluster?.updatedAt || 0;
        return bUpdated - aUpdated; // Most recent first
      });

      // Convert to tree items
      return sortedClusters.map((child: ClusterItem) => {
        const value = child.value || '';
        const count = child.count || 0;
        const term = child.term || value;

        this.currentFacet = 'cluster';
        const item = this.createFacetValueItem(
          value,
          count,
          vscode.TreeItemCollapsibleState.Collapsed
        );

        // Use cluster name for display
        item.label = `${term} (${count})`;
        item.iconPath = new vscode.ThemeIcon('group-by-ref-type');

        // Add activity date to description if available
        const updatedAt = (child as ClusterItem).payload?.metadata?.cluster?.updatedAt;
        if (updatedAt) {
          const date = new Date(updatedAt);
          const now = new Date();
          const diffMs = now.getTime() - date.getTime();
          const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
          if (diffDays === 0) {
            item.description = 'today';
          } else if (diffDays === 1) {
            item.description = 'yesterday';
          } else if (diffDays < 7) {
            item.description = `${diffDays}d ago`;
          } else {
            item.description = date.toLocaleDateString();
          }
        }

        this.currentFacet = undefined;
        return item;
      });
    } catch (error) {
      console.error('[UnifiedFacetTree] Error getting cluster values:', error);
      return [this.createErrorItem(error as Error, 'cluster')];
    }
  }

  /**
   * Get all categories
   */
  private getCategories(): UnifiedFacetTreeItem[] {
    const categories = [
      FacetCategory.Core,
      FacetCategory.Organization,
      FacetCategory.Temporal,
      FacetCategory.Content,
      FacetCategory.System,
      FacetCategory.Specialized,
    ];

    return categories.map((category) => {
      const facets = this.registry.getByCategory(category);
      const count = facets.length;
      const label = `${this.categoryLabels[category]} (${count})`;

      const item = this.createGroupItem(
        `category:${category}`,
        label,
        vscode.TreeItemCollapsibleState.Collapsed
      );
      // Store category in facet field for identification
      item.facet = 'category';
      item.value = category;
      return item;
    });
  }

  /**
   * Get facets in a category
   */
  private getFacetsInCategory(category: FacetCategory): UnifiedFacetTreeItem[] {
    const facets = this.registry.getByCategory(category);

    return facets.map((facet) => {
      return this.createTreeItem({
        id: `facet:${facet.field}`,
        kind: 'facet',
        label: facet.label,
        collapsibleState: vscode.TreeItemCollapsibleState.Collapsed,
        facet: facet.field,
        icon: facet.icon,
        description: facet.description,
      });
    });
  }

  /**
   * Get values for a facet
   */
  private async getFacetValues(field: string): Promise<UnifiedFacetTreeItem[]> {
    const facetConfig = this.registry.get(field);
    if (!facetConfig) {
      return [this.createEmptyPlaceholder(`Facet ${field} not found`, field)];
    }

    // Check if this facet type is supported
    if (!FacetProviderFactory.isSupported(facetConfig.type)) {
      return [this.createEmptyPlaceholder(`Facet type ${facetConfig.type} not yet supported`, field)];
    }

    try {
      // Get or create provider for this facet
      let provider = this.facetProviders.get(field);
      if (!provider) {
        provider = FacetProviderFactory.create(facetConfig, {
          workspaceRoot: this.workspaceRoot,
          workspaceId: this.workspaceId,
          context: (this.extensionContext || {}) as vscode.ExtensionContext,
          fileCacheService: this.fileCacheService,
          knowledgeClient: this.knowledgeClient,
          adminClient: this.adminClient,
          metadataStore: this.metadataStore,
        });
        this.facetProviders.set(field, provider);
      }

      // Get children from provider (should be facet values)
      const children = await provider.getChildren();
      
      if (!children || children.length === 0) {
        return [this.createEmptyPlaceholder(`No values found for ${facetConfig.label}`, field)];
      }

      // Convert to UnifiedFacetTreeItem
      interface ExtendedFacetItem extends IFacetTreeItem {
        term?: string;
        rangeLabel?: string;
        folderPath?: string;
        isFile?: boolean;
      }
      // Filter out files for Structure (folder) facets - only show folders
      const filteredChildren = facetConfig.type === FacetType.Structure
        ? children.filter((child: ExtendedFacetItem) => {
            // Only include items that are not files (i.e., folders)
            const isFile = child.isFile === true;
            console.log(`[UnifiedFacetTree] Filtering child: label="${typeof child.label === 'string' ? child.label : child.label?.label || ''}", isFile=${isFile}, folderPath=${(child as any).folderPath || child.folderPath || 'none'}`);
            return !isFile;
          })
        : children;
      
      if (facetConfig.type === FacetType.Structure && filteredChildren.length === 0) {
        return [this.createEmptyPlaceholder(`No folders found for ${facetConfig.label}`, field)];
      }
      
      return filteredChildren.map((child: ExtendedFacetItem) => {
        // Skip placeholder items (they have IDs starting with "placeholder:")
        if (child.id && child.id.startsWith('placeholder:')) {
          // Return placeholder as-is, but mark it so it won't be processed as a facet value
          return this.createEmptyPlaceholder(
            typeof child.label === 'string' ? child.label : child.label?.label || 'No data',
            field
          );
        }

        // Extract info from child item - handle different provider types
        let value = '';
        let count = 0;

        // For cluster facet, always use value (cluster ID) not term (cluster name) to ensure unique IDs
        if (field === 'cluster' && child.value) {
          // Cluster facet: use cluster ID as value for unique identification
          value = child.value;
          count = child.count || 0;
        } else if (child.term) {
          // TermsFacetTreeProvider
          value = child.term;
          // TermsFacetTreeItem doesn't expose count property, parse from label
          // Label format: "term (count)" or description format: "count files"
          if (child.count !== undefined) {
            count = child.count;
          } else if (child.label) {
            // Parse count from label like "term (5)"
            const labelText = typeof child.label === 'string' ? child.label : child.label?.label || '';
            const regex = /\((\d+)\)/;
            const match = regex.exec(labelText);
            if (match) {
              count = Number.parseInt(match[1], 10);
            }
          } else if (child.description) {
            // Parse count from description like "5 files"
            const descText = typeof child.description === 'string' ? child.description : String(child.description);
            const regex = /(\d+)\s+files?/;
            const match = regex.exec(descText);
            if (match) {
              count = Number.parseInt(match[1], 10);
            }
          }
        } else if (child.rangeLabel) {
          // NumericRangeFacetTreeProvider or DateRangeFacetTreeProvider
          value = child.rangeLabel;
          count = child.count || 0;
        } else if (child.folderPath) {
          // FolderTreeProvider - use folder name from label (e.g., "Libros (8)" -> "Libros")
          const labelText = typeof child.label === 'string' ? child.label : child.label?.label || '';
          value = labelText.split(' (')[0]; // Extract folder name before count
          // Parse count from label
          const regex = /\((\d+)\)/;
          const match = regex.exec(labelText);
          if (match) {
            count = Number.parseInt(match[1], 10);
          }
        } else if (child.value) {
          // Generic - use value for unique IDs
          value = child.value;
          count = child.count || 0;
        } else {
          // Fallback - use label
          value = typeof child.label === 'string' ? child.label : child.label?.label || '';
        }

        // Store current facet field for createFacetValueItem
        this.currentFacet = field;
        
        // For cluster facet, use the cluster name (term) for display, but keep ID as value
        let displayValue = value;
        if (field === 'cluster' && child.term) {
          // Use cluster name for display label, but keep cluster ID as value for unique identification
          displayValue = child.term;
        }
        
        const item = this.createFacetValueItem(
          value, // Always use the ID/value for internal identification
          count,
          vscode.TreeItemCollapsibleState.Collapsed
        );
        
        // Override label for cluster facet to show name instead of ID
        if (field === 'cluster' && child.term) {
          item.label = `${displayValue} (${count})`;
        }
        
        // Normalize extension display: remove leading dot for better UX
        // Keep original value for queries, but display without dot
        if (field === 'extension' && value.startsWith('.')) {
          const displayValue = value.slice(1);
          item.label = `${displayValue} (${count})`;
        }
        this.currentFacet = undefined;
        return item;
      });
    } catch (error) {
      console.error(`[UnifiedFacetTree] Error getting values for facet ${field}:`, error);
      return [this.createErrorItem(error as Error, field)];
    }
  }

  /**
   * Get entities (files, folders, projects) for a facet value
   */
  private async getFilesForFacetValue(
    field: string,
    value: string
  ): Promise<UnifiedFacetTreeItem[]> {
    console.log(`[UnifiedFacetTree] getFilesForFacetValue called: field="${field}", value="${value}"`);
    const facetConfig = this.registry.get(field);
    if (!facetConfig) {
      return [];
    }

    // For NumericRange, DateRange, and Structure facets, always use provider directly (can't be queried via getEntitiesByFacet)
    if (facetConfig.type === FacetType.NumericRange || facetConfig.type === FacetType.DateRange || facetConfig.type === FacetType.Structure) {
      // Use provider directly for range facets
      try {
        let provider = this.facetProviders.get(field);
        if (!provider) {
          provider = FacetProviderFactory.create(facetConfig, {
            workspaceRoot: this.workspaceRoot,
            workspaceId: this.workspaceId,
            context: (this.extensionContext || {}) as vscode.ExtensionContext,
            fileCacheService: this.fileCacheService,
            knowledgeClient: this.knowledgeClient,
            adminClient: this.adminClient,
            metadataStore: this.metadataStore,
          });
          this.facetProviders.set(field, provider);
        }

        // Get all items from provider (ranges for NumericRange/DateRange, folders for Structure)
        const facetValues = await provider.getChildren();
        
        // Find the matching item
        interface ExtendedFacetItem extends IFacetTreeItem {
          rangeLabel?: string;
          minValue?: number;
          maxValue?: number;
          folderPath?: string;
        }
        const matchingItem = facetValues.find((item: ExtendedFacetItem) => {
          const itemLabel = typeof item.label === 'string' ? item.label : item.label?.label || '';
          // For folder facets, extract folder name from label like "Libros (8)" -> "Libros"
          if (facetConfig.type === FacetType.Structure) {
            const folderName = itemLabel.split(' (')[0]; // Extract name before count
            const valueName = value.split(' (')[0]; // Extract name from value
            // Check if folder name matches, or if folderPath matches
            const itemFolderPath = (item as any).folderPath || item.folderPath;
            return folderName === valueName || 
                   itemLabel === value || 
                   itemFolderPath === value ||
                   itemFolderPath === valueName ||
                   folderName === value;
          }
          // For range facets
          return item.rangeLabel === value || itemLabel.includes(value) || item.value === value;
        });

        if (!matchingItem) {
          return [this.createEmptyPlaceholder(`${facetConfig.type === FacetType.Structure ? 'Folder' : 'Range'} ${value} not found in facet ${field}`, `${field}:${value}`)];
        }

        // Get files/folders for this item
        const files = await provider.getChildren(matchingItem);
        if (!files || files.length === 0) {
          return [this.createEmptyPlaceholder(`No files found for ${value}`, `${field}:${value}`)];
        }

        // Convert to UnifiedFacetTreeItem
        interface FileItemWithPath extends IFacetTreeItem {
          relativePath?: string;
          isFile?: boolean;
          folderPath?: string;
        }
        return files.map((file: FileItemWithPath) => {
          // For Structure (folder) facets, handle both files and folders
          if (facetConfig.type === FacetType.Structure) {
            // Check if this is a file or folder
            const isFile = file.isFile === true;
            let relativePath = '';
            if (file.resourceUri) {
              relativePath = this.getRelativePath(file.resourceUri.fsPath);
            } else if (file.folderPath) {
              // It's a folder, use folderPath
              relativePath = file.folderPath;
            } else if (file.payload?.metadata && 'relativePath' in file.payload.metadata) {
              relativePath = String(file.payload.metadata.relativePath);
            }
            
            if (!relativePath) {
              return null;
            }
            
            if (isFile) {
              // It's a file
              const item = this.createFileItem(relativePath, undefined, field, value);
              if (file.tooltip) item.tooltip = file.tooltip;
              if (file.description) item.description = file.description;
              return item;
            } else {
              // It's a folder - create a folder item that can be expanded
              const folderName = file.folderPath ? path.basename(file.folderPath) : path.basename(relativePath);
              const item = this.createFacetValueItem(
                folderName,
                0, // Count will be shown in label
                vscode.TreeItemCollapsibleState.Collapsed
              );
              item.facet = field;
              item.value = file.folderPath || relativePath;
              item.id = `folder:${field}:${file.folderPath || relativePath}`;
              if (file.tooltip) item.tooltip = file.tooltip;
              if (file.description) item.description = file.description;
              item.iconPath = new vscode.ThemeIcon('folder');
              item.contextValue = 'cortex-folder';
              return item;
            }
          }
          
          // For range facets, treat all as files
          let relativePath = '';
          if (file.resourceUri) {
            relativePath = this.getRelativePath(file.resourceUri.fsPath);
          } else if (file.payload?.metadata && 'relativePath' in file.payload.metadata) {
            relativePath = String(file.payload.metadata.relativePath);
          }
          if (!relativePath) {
            return null;
          }
          const item = this.createFileItem(relativePath, undefined, field, value);
          if (file.tooltip) item.tooltip = file.tooltip;
          if (file.description) item.description = file.description;
          return item;
        }).filter((item: UnifiedFacetTreeItem | null): item is UnifiedFacetTreeItem => item !== null);
      } catch (error) {
        console.error(`[UnifiedFacetTree] Provider failed for ${field}=${value}:`, error);
        return [this.createErrorItem(error as Error, `${field}:${value}`)];
      }
    }

    // For extension, type, document_type, mime_type, mime_category, project, context, and cluster facets, skip EntityClient and use provider directly (faster and more reliable)
    // Store field as string to avoid TypeScript type narrowing issues
    const fieldAsString: string = field;
    const directProviderFields: readonly string[] = ['extension', 'type', 'document_type', 'mime_type', 'mime_category', 'project', 'context', 'cluster'];
    const isDirectProviderField = directProviderFields.includes(fieldAsString);
    if (!isDirectProviderField) {
      if (!this.entityClient) {
        return [this.createErrorItem(new Error('Entity client unavailable'), `${field}:${value}`)];
      }
      // Try to use EntityClient if available (unified entities) for other facets
      try {
        // Use timeout parameter in getEntitiesByFacet (gRPC-level timeout)
        // This ensures the gRPC call itself times out, not just the Promise
        // Increase timeout for indexing_status as it may need to query many files
        const timeoutMs = (field === 'indexing_status' || field === 'indexing_error' || field === 'index_error' || field === 'indexing_errors') ? 12000 : 8000;
        const entities = await this.entityClient.getEntitiesByFacet(
          this.workspaceId,
          field,
          value,
          ['file', 'folder', 'project'],
          timeoutMs
        );

        if (entities.length === 0) {
          return [this.createEmptyPlaceholder(`No entities found for ${value}`, `${field}:${value}`)];
        }

        // Convert entities to tree items
        return entities.map((entity) => this.entityToTreeItem(entity, field, value));
      } catch (error) {
        console.error(`[UnifiedFacetTree] EntityClient failed for ${field}=${value}:`, error);
        return [this.createErrorItem(error as Error, `${field}:${value}`)];
      }
    }

    // Provider-based approach (extensions/types and local facet providers)
    try {
      // Get provider for this facet
      let provider = this.facetProviders.get(field);
      if (!provider) {
        console.warn(`[UnifiedFacetTree] Provider not found for ${field}, creating new one`);
        // Try to create provider if not cached
        const facetConfig = this.registry.get(field);
        if (facetConfig && FacetProviderFactory.isSupported(facetConfig.type)) {
          provider = FacetProviderFactory.create(facetConfig, {
            workspaceRoot: this.workspaceRoot,
            workspaceId: this.workspaceId,
            context: (this.extensionContext || {}) as vscode.ExtensionContext,
            fileCacheService: this.fileCacheService,
            knowledgeClient: this.knowledgeClient,
            adminClient: this.adminClient,
            metadataStore: this.metadataStore,
          });
          this.facetProviders.set(field, provider);
        } else {
          return [this.createEmptyPlaceholder(`Provider not found for ${field}`, field)];
        }
      }

      // For extension, type, document_type, mime_type, and mime_category facets, use direct method if available
      const providerWithMethod = provider as ProviderWithGetFilesForTerm;
      if (providerWithMethod.getFilesForTerm) {
        // Normalize extension value for TermsFacetTreeProvider (only for extension, not for others)
        // document_type, mime_type, mime_category values should be passed as-is
        // Use fieldAsString to avoid TypeScript type narrowing issues
        const normalizedValue = (fieldAsString === 'extension' && value.startsWith('.')) ? value.slice(1) : value;
        console.log(`[UnifiedFacetTree] Normalized value for ${field}: "${value}" -> "${normalizedValue}"`);
        
          // Add timeout to prevent hanging (longer for document_type as it queries AI metadata)
          // Use indexOf to avoid TypeScript type narrowing issues
          // document_type is at index 2 in directProviderFields array
          const fieldIndex = directProviderFields.indexOf(fieldAsString);
          const timeoutMs = fieldIndex === 2 ? 15000 : 8000;
          const timeoutPromise = new Promise<never>((_, reject) => {
            setTimeout(() => reject(new Error(`getFilesForTerm timeout after ${timeoutMs/1000} seconds`)), timeoutMs);
          });
          
          try {
            console.log(`[UnifiedFacetTree] Getting files for ${field}: ${normalizedValue} (original value: ${value})`);
            const startTime = Date.now();
            const files = await Promise.race([
              providerWithMethod.getFilesForTerm(normalizedValue, field),
              timeoutPromise
            ]);
            const duration = Date.now() - startTime;
            console.log(`[UnifiedFacetTree] getFilesForTerm completed in ${duration}ms for ${field}: ${normalizedValue}`);
            console.log(`[UnifiedFacetTree] Files returned:`, {
              isArray: Array.isArray(files),
              length: files?.length,
              firstFileType: files?.[0] ? typeof files[0] : 'none',
              firstFileHasResourceUri: files?.[0]?.resourceUri ? true : false,
              firstFileHasPayload: files?.[0]?.payload ? true : false
            });
          
          console.log(`[UnifiedFacetTree] Found ${files?.length || 0} files for ${field}: ${normalizedValue}`);
          console.log(`[UnifiedFacetTree] Files type check:`, {
            isArray: Array.isArray(files),
            filesType: typeof files,
            filesLength: files?.length,
            firstFileType: files?.[0] ? typeof files[0] : 'none'
          });
          
          if (!files || files.length === 0) {
            console.log(`[UnifiedFacetTree] No files to convert, returning placeholder`);
            return [this.createEmptyPlaceholder(`No files found for ${value}`, `${field}:${value}`)];
          }
          
          // Log file structure for debugging
          console.log(`[UnifiedFacetTree] Converting ${files.length} files to UnifiedFacetTreeItem`);
          files.forEach((file: IFacetTreeItem, index: number) => {
            console.log(`[UnifiedFacetTree] File ${index + 1}:`, {
              hasResourceUri: !!file.resourceUri,
              resourceUri: file.resourceUri?.fsPath,
              hasPayload: !!file.payload,
              payloadMetadata: file.payload?.metadata,
              label: file.label,
              kind: file.kind,
              isFile: file.isFile
            });
          });
          
          // Convert to UnifiedFacetTreeItem
          const convertedItems = files.map((file: IFacetTreeItem) => {
            let relativePath = '';
            if (file.resourceUri) {
              relativePath = this.getRelativePath(file.resourceUri.fsPath);
              console.log(`[UnifiedFacetTree] Extracted relativePath from resourceUri: ${relativePath}`);
            } else if (file.payload?.metadata && 'relativePath' in file.payload.metadata) {
              relativePath = String(file.payload.metadata.relativePath);
              console.log(`[UnifiedFacetTree] Extracted relativePath from payload.metadata: ${relativePath}`);
            } else if (file.payload?.metadata && 'member' in file.payload.metadata) {
              // For cluster members, try to get relativePath from member object
              const member = (file.payload.metadata as any).member;
              if (member && member.relativePath) {
                relativePath = member.relativePath;
                console.log(`[UnifiedFacetTree] Extracted relativePath from payload.metadata.member: ${relativePath}`);
              }
            } else {
              console.warn('[UnifiedFacetTree] Unknown file format:', {
                hasResourceUri: !!file.resourceUri,
                hasPayload: !!file.payload,
                payloadKeys: file.payload ? Object.keys(file.payload) : [],
                metadataKeys: file.payload?.metadata ? Object.keys(file.payload.metadata) : []
              });
              return null;
            }
            if (!relativePath) {
              console.error('[UnifiedFacetTree] Could not extract relativePath from file:', file);
              return null;
            }
            const item = this.createFileItem(relativePath, undefined, field, value);
            if (file.tooltip) item.tooltip = file.tooltip;
            if (file.description) item.description = file.description;
            console.log(`[UnifiedFacetTree] Created UnifiedFacetTreeItem for: ${relativePath}`);
            return item;
          }).filter((item: UnifiedFacetTreeItem | null): item is UnifiedFacetTreeItem => item !== null);
          
          console.log(`[UnifiedFacetTree] Successfully converted ${convertedItems.length} files out of ${files.length}`);
          return convertedItems;
        } catch (error) {
          console.error(`[UnifiedFacetTree] getFilesForTerm failed for ${field}=${value}:`, error);
          return [this.createErrorItem(error as Error, `${field}:${value}`)];
        }
      }

      // Find the value item in provider's children
      const facetValues = await provider.getChildren();
      console.log(`[UnifiedFacetTree] Searching for value "${value}" in ${facetValues.length} facet values for field "${field}"`);
      
      // For extension facet, normalize both the search value and item values for comparison
      const normalizeForComparison = (val: string): string => {
        if (field === 'extension') {
          // Normalize extension: remove leading dot and lowercase
          return val.replace(/^\./, '').toLowerCase();
        }
        return val;
      };
      
      const normalizedValue = normalizeForComparison(value);
      interface ExtendedFacetItem extends IFacetTreeItem {
        term?: string;
        rangeLabel?: string;
      }
      
      // Log available values for debugging
      if (field === 'cluster') {
        console.log(`[UnifiedFacetTree] Available cluster values:`, facetValues.map((v: ExtendedFacetItem) => {
          const labelText = typeof v.label === 'string' ? v.label : v.label?.label || '';
          return {
            value: v.value,
            term: v.term,
            label: labelText,
            kind: v.kind,
            id: v.id
          };
        }).slice(0, 5));
      }
      
      const valueItem = facetValues.find((item: ExtendedFacetItem) => {
        const itemValue = item.value || item.term || item.rangeLabel || '';
        const normalizedItemValue = normalizeForComparison(itemValue);
        const labelText = typeof item.label === 'string' ? item.label : item.label?.label || '';
        
        // For cluster facet, prioritize exact value match (cluster ID)
        if (field === 'cluster') {
          // Match by value (cluster ID) first
          if (item.value === value) {
            console.log(`[UnifiedFacetTree] Found cluster by value match: ${item.value}`);
            return true;
          }
          // Also try matching by term (cluster name) if value doesn't match
          if (item.term && (item.term === value || item.term === normalizedValue)) {
            console.log(`[UnifiedFacetTree] Found cluster by term match: ${item.term}`);
            return true;
          }
          // Try matching label (which includes count like "Cluster Name (5)")
          if (labelText) {
            const labelWithoutCount = labelText.split(' (')[0].trim();
            if (labelWithoutCount === value || labelWithoutCount === normalizedValue) {
              console.log(`[UnifiedFacetTree] Found cluster by label match: ${labelWithoutCount}`);
              return true;
            }
          }
          return false;
        }
        
        return (
          normalizedItemValue === normalizedValue ||
          item.value === value ||
          item.term === value ||
          item.rangeLabel === value ||
          (labelText && normalizeForComparison(labelText.split(' ')[0]) === normalizedValue)
        );
      });

      if (!valueItem) {
        console.warn(`[UnifiedFacetTree] Value ${value} not found in facet ${field}. Available values:`, 
          facetValues.map((v: ExtendedFacetItem) => {
            const labelText = typeof v.label === 'string' ? v.label : v.label?.label || '';
            return {
              value: v.value,
              term: v.term,
              label: labelText
            };
          }).slice(0, 5));
        return [this.createEmptyPlaceholder(`Value ${value} not found in facet ${field}`, `${field}:${value}`)];
      }

      console.log(`[UnifiedFacetTree] Found valueItem for ${field}:`, {
        value: valueItem.value,
        term: (valueItem as ExtendedFacetItem).term,
        kind: valueItem.kind,
        id: valueItem.id
      });

      // Get files for this value
      console.log(`[UnifiedFacetTree] Calling provider.getChildren(valueItem) for ${field}:${value}`);
      const files = await provider.getChildren(valueItem);
      console.log(`[UnifiedFacetTree] provider.getChildren returned ${files?.length || 0} files for ${field}:${value}`);

      if (!files || files.length === 0) {
        return [this.createEmptyPlaceholder(`No files found for ${value}`, `${field}:${value}`)];
      }

      // Convert to UnifiedFacetTreeItem
      interface FileItemWithPath extends IFacetTreeItem {
        relativePath?: string;
        term?: string;
      }
      return files.map((file: FileItemWithPath | string) => {
        let relativePath = '';

        if (typeof file === 'string') {
          relativePath = file;
        } else if (file.resourceUri) {
          relativePath = this.getRelativePath(file.resourceUri.fsPath);
        } else if (file.relativePath) {
          relativePath = file.relativePath;
        } else {
          console.warn('[UnifiedFacetTree] Unknown file format:', file);
          return null;
        }

        if (!relativePath) {
          return null;
        }

        const item = this.createFileItem(relativePath, undefined, field, value);
        // Add tooltip if available
        if (typeof file === 'string') {
          item.tooltip = this.createFileTooltip(relativePath, file);
          item.description = this.createFileDescription(relativePath);
        } else {
          item.tooltip = file.tooltip || this.createFileTooltip(relativePath, file);
          item.description = file.description || this.createFileDescription(relativePath);
        }
        return item;
      }).filter((item: UnifiedFacetTreeItem | null): item is UnifiedFacetTreeItem => item !== null);
    } catch (error) {
      console.error(`[UnifiedFacetTree] Error getting files for ${field}=${value}:`, error);
      return [this.createErrorItem(error as Error, `${field}:${value}`)];
    }
  }

  /**
   * Convert Entity to UnifiedFacetTreeItem
   */
  private entityToTreeItem(entity: Entity, facet?: string, value?: string): UnifiedFacetTreeItem {
    const resourceUri = vscode.Uri.file(
      entity.fileData?.absolutePath ||
      entity.folderData?.absolutePath ||
      this.getAbsolutePath(entity.path)
    );

    // Determine icon based on entity type
    let icon: vscode.ThemeIcon;
    let contextValue: string;
    let command: vscode.Command | undefined;

    switch (entity.type) {
      case 'file':
        icon = vscode.ThemeIcon.File;
        contextValue = 'cortex-file';
        command = {
          command: 'vscode.open',
          title: 'Open File',
          arguments: [resourceUri],
        };
        break;
      case 'folder':
        icon = vscode.ThemeIcon.Folder;
        contextValue = 'cortex-folder';
        command = {
          command: 'vscode.openFolder',
          title: 'Open Folder',
          arguments: [resourceUri],
        };
        break;
      case 'project':
        icon = new vscode.ThemeIcon('project');
        contextValue = 'cortex-project';
        break;
      default:
        icon = vscode.ThemeIcon.File;
        contextValue = 'cortex-entity';
    }

    // Build label with type indicator
    // Normalize entity type to handle cases where it might be "ENTITY_TYPE_FILE" instead of "file"
    let normalizedType = 'file';
    if (entity.type) {
      const lowerType = entity.type.toLowerCase();
      if (lowerType.includes('folder')) {
        normalizedType = 'folder';
      } else if (lowerType.includes('project')) {
        normalizedType = 'project';
      } else if (!lowerType.includes('file')) {
        normalizedType = entity.type;
      }
      // else: keep 'file' as default
    }
    const typeLabel = normalizedType === 'file' ? '' : ` [${normalizedType}]`;
    const label = `${entity.name || entity.path || 'Unknown'}${typeLabel}`;

    // Build description
    const descriptionParts: string[] = [];
    if (entity.size) {
      descriptionParts.push(this.formatFileSize(entity.size));
    }
    if (entity.language) {
      descriptionParts.push(entity.language);
    }
    const description = descriptionParts.length > 0 ? descriptionParts.join(' • ') : undefined;

    // Build tooltip
    const tooltipParts: string[] = [
      `Type: ${entity.type}`,
      `Path: ${entity.path}`,
    ];
    if (entity.description) {
      tooltipParts.push(`Description: ${entity.description}`);
    }
    if (entity.tags && entity.tags.length > 0) {
      tooltipParts.push(`Tags: ${entity.tags.join(', ')}`);
    }
    if (entity.projects && entity.projects.length > 0) {
      tooltipParts.push(`Projects: ${entity.projects.join(', ')}`);
    }
    const tooltip = new vscode.MarkdownString(tooltipParts.join('\n\n'));

    // Generate unique ID - include facet and value context if provided
    let id: string;
    if (facet && value && entity.type === 'file') {
      // For files, include facet and value in ID to prevent duplicates
      const pathHash = crypto.createHash('sha256').update(entity.path).digest('hex').substring(0, 16);
      const sanitizedFacet = facet.replaceAll(/\W/g, '_');
      const sanitizedValue = value.replaceAll(/\W/g, '_').substring(0, 50);
      id = `file:${sanitizedFacet}:${sanitizedValue}:${pathHash}`;
    } else {
      // For folders/projects or when no facet context, use entity ID
      id = `${entity.type}:${entity.id.id}`;
    }

    const item = this.createTreeItem({
      id,
      kind: 'file',
      label,
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      resourceUri,
      isFile: entity.type === 'file',
      facet: facet || undefined,
      value: value || undefined,
      count: undefined,
      payload: {
        facet: facet || '',
        value: entity.path || '',
        metadata: { entity },
      },
      description,
      tooltip,
      icon,
    });

    item.contextValue = contextValue;
    if (command) {
      item.command = command;
    }

    return item;
  }

  /**
   * Override to use facet field from context
   */
  protected createFacetValueItem(
    value: string,
    count: number,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.Collapsed
  ): UnifiedFacetTreeItem {
    const label = this.formatLabelWithCount(value, count);

    // Get facet from current context if available
    const facet = this.currentFacet || this.config.field || '';

    return this.createTreeItem({
      id: this.generateFacetId(value, facet),
      kind: 'facet',
      label,
      collapsibleState,
      facet,
      value,
      count,
      payload: {
        facet,
        value,
        metadata: { count },
      },
    });
  }

  /**
   * Override generateFacetId to include facet name for unique IDs
   * This ensures that the same value in different facets gets different IDs
   */
  protected generateFacetId(value: string, facet?: string): string {
    // Use provided facet, or get from current context, or use config field, or fallback to 'unknown'
    const facetField = facet || this.currentFacet || this.config.field;
    if (!facetField) {
      // This should not happen, but provide a fallback to prevent double colons
      console.warn('[UnifiedFacetTree] generateFacetId called without facet field, using "unknown"');
      return `facet:unknown:${value}`;
    }
    return `facet:${facetField}:${value}`;
  }

  /**
   * Override createFileItem to include facet and value context for unique IDs
   * This prevents duplicate ID errors when the same file appears in different facet contexts
   */
  protected createFileItem(
    relativePath: string,
    label?: string,
    facet?: string,
    value?: string
  ): UnifiedFacetTreeItem {
    // Generate unique ID based on facet and value context
    // This prevents duplicate ID errors when the same file appears in different facet contexts
    let id: string;
    if (facet && value) {
      // Include facet and value in ID to make it unique
      // Use a short hash of the relativePath to keep IDs manageable and avoid special characters
      const pathHash = crypto.createHash('sha256').update(relativePath).digest('hex').substring(0, 16);
      // Sanitize facet and value to avoid special characters in ID
      const sanitizedFacet = facet.replaceAll(/\W/g, '_');
      const sanitizedValue = value.replaceAll(/\W/g, '_').substring(0, 50); // Limit value length
      id = `file:${sanitizedFacet}:${sanitizedValue}:${pathHash}`;
    } else {
      // Fallback to simple ID if no context provided
      // Use hash to avoid issues with special characters in paths
      const pathHash = crypto.createHash('sha256').update(relativePath).digest('hex').substring(0, 16);
      id = `file:${pathHash}`;
    }

    const fullPath = path.join(this.workspaceRoot, relativePath);
    const uri = vscode.Uri.file(fullPath);
    const filename = label || path.basename(relativePath);

    return this.createTreeItem({
      id,
      kind: 'file',
      label: filename,
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      resourceUri: uri,
      isFile: true,
      facet: facet || this.config.field,
      payload: {
        facet: facet || this.config.field,
        value: relativePath,
        metadata: { relativePath },
      },
    });
  }

  /**
   * Override createEmptyPlaceholder to include facet context for unique IDs
   */
  protected createEmptyPlaceholder(message?: string, context?: string): UnifiedFacetTreeItem {
    // Use context (facet field) if provided, otherwise use a unique ID based on message
    const contextId = context || this.currentFacet || this.config.field || 'unknown';
    const messageHash = message ? message.replaceAll(/\s+/g, '_').substring(0, 20) : 'empty';
    return this.createTreeItem({
      id: `placeholder:empty:${contextId}:${messageHash}`,
      kind: 'group',
      label: message || t('noFilesFound'),
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      icon: new vscode.ThemeIcon('info'),
    });
  }

  /**
   * Override createErrorItem to include facet context for unique IDs
   */
  protected createErrorItem(error: Error, context?: string): UnifiedFacetTreeItem {
    const contextId = context || this.currentFacet || this.config.field || 'unknown';
    const errorHash = error.message ? error.message.replaceAll(/\s+/g, '_').substring(0, 20) : 'error';
    
    // Show more specific error message
    let errorMessage = error.message;
    if (errorMessage.includes('timeout')) {
      errorMessage = `Timeout: ${errorMessage}`;
    } else if (errorMessage.includes('unavailable') || errorMessage.includes('ECONNREFUSED')) {
      errorMessage = `Backend unavailable: ${errorMessage}`;
    } else if (errorMessage.length > 50) {
      // Truncate long error messages
      errorMessage = errorMessage.substring(0, 50) + '...';
    }
    
    return this.createTreeItem({
      id: `error:${contextId}:${errorHash}`,
      kind: 'group',
      label: `${t('errorLoadingFacetField', { field: contextId })}: ${errorMessage}`,
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      icon: new vscode.ThemeIcon('error'),
      tooltip: `${t('errorLoadingFacets')}: ${error.message}\nContext: ${contextId}`,
    });
  }

  /**
   * Clear cached providers
   */
  override refresh(): void {
    this.facetProviders.clear();
    super.refresh();
  }

  /**
   * Get absolute path from relative path
   */
  private getAbsolutePath(relativePath: string): string {
    return path.join(this.workspaceRoot, relativePath);
  }

  /**
   * Normalize extension (remove leading dot, lowercase)
   */
  private normalizeExtension(extension: string): string {
    const normalized = extension.trim().toLowerCase();
    return normalized.startsWith('.') ? normalized.slice(1) : normalized;
  }

  /**
   * Format file size
   */
  private formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
  }

  /**
   * Dispose resources
   */
  override dispose(): void {
    // Dispose all cached providers
    for (const provider of this.facetProviders.values()) {
      if ('dispose' in provider && typeof provider.dispose === 'function') {
        provider.dispose();
      }
    }
    this.facetProviders.clear();
    
    // Close entity client
    if (this.entityClient) {
      this.entityClient.close();
    }
    
    super.dispose();
  }
}
