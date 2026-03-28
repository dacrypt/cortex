/**
 * BaseFacetTreeProvider - Abstract base class for all facet providers
 * 
 * Provides common functionality shared by all facet types:
 * - Event handling
 * - Caching
 * - File cache service integration
 * - Common tree item creation
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import {
  IFacetProvider,
  IFacetTreeItem,
  FacetConfig,
  FacetProviderContext,
} from '../contracts/IFacetProvider';
import { ViewNodeKind } from '../contracts/viewNodes';

/**
 * Base implementation for all facet tree providers
 */
export abstract class BaseFacetTreeProvider<T extends IFacetTreeItem = IFacetTreeItem>
  implements IFacetProvider<T>
{
  protected readonly _onDidChangeTreeData: vscode.EventEmitter<T | undefined | null | void> =
    new vscode.EventEmitter<T | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<T | undefined | null | void> =
    this._onDidChangeTreeData.event;

  public readonly config: FacetConfig;
  public readonly workspaceId: string;
  public readonly workspaceRoot: string;
  protected readonly fileCacheService: any; // FileCacheService
  protected readonly knowledgeClient?: any; // GrpcKnowledgeClient
  protected readonly adminClient?: any; // GrpcAdminClient
  protected readonly metadataStore?: any; // IMetadataStore

  // Cache for facet data
  protected readonly cache = new Map<string, { data: unknown; timestamp: number }>();
  protected readonly CACHE_TTL = 10000; // 10 seconds - reduced for more responsive real-time updates

  constructor(config: FacetConfig, ctx: FacetProviderContext) {
    this.config = config;
    this.workspaceId = ctx.workspaceId;
    this.workspaceRoot = ctx.workspaceRoot;
    this.fileCacheService = ctx.fileCacheService;
    this.knowledgeClient = ctx.knowledgeClient;
    this.adminClient = ctx.adminClient;
    this.metadataStore = ctx.metadataStore;

    // Set workspace ID if file cache service is available
    if (this.fileCacheService && this.workspaceId) {
      this.fileCacheService.setWorkspaceId(this.workspaceId);
    }
  }

  /**
   * Refresh the facet data
   */
  refresh(): void {
    this.clearCache();
    this._onDidChangeTreeData.fire();
  }

  /**
   * Clear the cache
   */
  protected clearCache(): void {
    this.cache.clear();
  }

  /**
   * Get cached data or compute it
   */
  protected getCached<T>(key: string, compute: () => Promise<T>): Promise<T> {
    const cached = this.cache.get(key);
    if (cached && Date.now() - cached.timestamp < this.CACHE_TTL) {
      return Promise.resolve(cached.data as T);
    }

    return compute().then((data) => {
      this.cache.set(key, { data, timestamp: Date.now() });
      return data;
    });
  }

  /**
   * Get tree item representation
   */
  getTreeItem(element: T): vscode.TreeItem {
    return element;
  }

  /**
   * Get children - must be implemented by subclasses
   */
  abstract getChildren(element?: T): Promise<T[]>;

  /**
   * Create a file tree item
   */
  protected createFileItem(
    relativePath: string,
    label?: string
  ): T {
    const fullPath = path.join(this.workspaceRoot, relativePath);
    const uri = vscode.Uri.file(fullPath);
    const filename = label || path.basename(relativePath);

    return this.createTreeItem({
      id: `file:${relativePath}`,
      kind: 'file',
      label: filename,
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      resourceUri: uri,
      isFile: true,
      facet: this.config.field,
      payload: {
        facet: this.config.field,
        value: relativePath,
        metadata: { relativePath },
      },
    }) as T;
  }

  /**
   * Create a facet value tree item
   */
  protected createFacetValueItem(
    value: string,
    count: number,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.Collapsed
  ): T {
    const label = `${value} (${count})`;

    return this.createTreeItem({
      id: `facet:${this.config.field}:${value}`,
      kind: 'facet',
      label,
      collapsibleState,
      facet: this.config.field,
      value,
      count,
      payload: {
        facet: this.config.field,
        value,
        metadata: { count },
      },
    }) as T;
  }

  /**
   * Create a group tree item
   */
  protected createGroupItem(
    groupKey: string,
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.Collapsed
  ): T {
    return this.createTreeItem({
      id: `group:${this.config.field}:${groupKey}`,
      kind: 'group',
      label,
      collapsibleState,
      facet: this.config.field,
      payload: {
        facet: this.config.field,
        value: groupKey,
        metadata: { groupKey },
      },
    }) as T;
  }

  /**
   * Create a tree item - must be implemented by subclasses
   * This allows each provider to use its own TreeItem class
   */
  protected abstract createTreeItem(params: {
    id: string;
    kind: ViewNodeKind;
    label: string;
    collapsibleState: vscode.TreeItemCollapsibleState;
    resourceUri?: vscode.Uri;
    isFile?: boolean;
    facet?: string;
    value?: string;
    count?: number;
    payload?: any;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    icon?: vscode.ThemeIcon;
  }): T;

  /**
   * Get files from cache
   */
  protected async getFiles(): Promise<any[]> {
    if (!this.fileCacheService) {
      return [];
    }
    return await this.fileCacheService.getFiles();
  }

  /**
   * Sort files by activity
   */
  protected sortFilesByActivity(files: any[]): any[] {
    // Import and use the utility function
    const { sortFilesByActivity } = require('../../utils/fileActivity');
    return sortFilesByActivity(files);
  }

  /**
   * Get file activity timestamp
   */
  protected getFileActivityTimestamp(file: any): number {
    const { getFileActivityTimestamp } = require('../../utils/fileActivity');
    return getFileActivityTimestamp(file);
  }

  /**
   * Create URI for a file
   */
  protected getFileUri(relativePath: string): vscode.Uri {
    return vscode.Uri.file(path.join(this.workspaceRoot, relativePath));
  }

  /**
   * Create empty placeholder item
   */
  protected createEmptyPlaceholder(message?: string): T {
    const { t } = require('../i18n');
    return this.createTreeItem({
      id: `placeholder:empty:${this.config.field}`,
      kind: 'group',
      label: message || t('noFilesFound'),
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      icon: new vscode.ThemeIcon('info'),
    });
  }

  /**
   * Create error item
   */
  protected createErrorItem(error: Error, context?: string): T {
    const { t } = require('../i18n');
    return this.createTreeItem({
      id: `error:${this.config.field}:${context || 'unknown'}`,
      kind: 'group',
      label: t('errorLoadingFacetField', { field: this.config.field }),
      collapsibleState: vscode.TreeItemCollapsibleState.None,
      icon: new vscode.ThemeIcon('error'),
      tooltip: `${t('errorLoadingFacets')}: ${error.message}`,
    });
  }

  /**
   * Query backend for facet data
   */
  protected async queryBackendFacet(
    field: string,
    type: 'terms' | 'numeric_range' | 'date_range'
  ): Promise<any> {
    if (!this.knowledgeClient) {
      return null;
    }
    try {
      const response = await this.knowledgeClient.getFacets(
        this.workspaceId,
        [{ field, type }]
      );
      return response?.results?.[0] || null;
    } catch (error) {
      console.warn(`[FacetTree] Backend facets unavailable for ${field}:`, error);
      return null;
    }
  }

  /**
   * Generate file ID
   */
  protected generateFileId(relativePath: string, context?: string): string {
    return `file:${this.config.field}${context ? `:${context}` : ''}:${relativePath}`;
  }

  /**
   * Generate facet value ID
   */
  protected generateFacetId(value: string): string {
    return `facet:${this.config.field}:${value}`;
  }

  /**
   * Generate range ID
   */
  protected generateRangeId(min: number | string, max: number | string): string {
    return `range:${this.config.field}:${min}:${max}`;
  }

  /**
   * Create file tooltip
   */
  protected createFileTooltip(relativePath: string, file: any): string {
    const activityTime = this.getFileActivityTimestamp(file);
    return `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
  }

  /**
   * Create facet tooltip
   */
  protected createFacetTooltip(value: string, count: number): string {
    return `${this.config.field}: ${value}\nCount: ${count} files`;
  }

  /**
   * Create file description (directory path)
   */
  protected createFileDescription(relativePath: string): string {
    return this.getDirname(relativePath);
  }

  /**
   * Filter files by exact value match
   */
  protected filterFilesByValue(
    files: any[],
    extractValue: (file: any) => string | string[],
    targetValue: string
  ): any[] {
    return files.filter((file) => {
      const values = extractValue(file);
      if (Array.isArray(values)) {
        return values.includes(targetValue);
      }
      return values === targetValue;
    });
  }

  /**
   * Filter files by numeric range
   */
  protected filterFilesByRange(
    files: any[],
    extractValue: (file: any) => number,
    min: number,
    max: number
  ): any[] {
    return files.filter((file) => {
      const value = extractValue(file);
      if (max === 0 || max === Infinity) {
        return value >= min;
      }
      return value >= min && value <= max;
    });
  }

  /**
   * Format label with count
   */
  protected formatLabelWithCount(label: string, count: number): string {
    return `${label} (${count})`;
  }

  /**
   * Get relative path from absolute path
   */
  protected getRelativePath(absolutePath: string): string {
    return path.relative(this.workspaceRoot, absolutePath);
  }

  /**
   * Get filename from relative path
   */
  protected getFilename(relativePath: string): string {
    return path.basename(relativePath);
  }

  /**
   * Get directory name from relative path
   */
  protected getDirname(relativePath: string): string {
    return path.dirname(relativePath);
  }

  /**
   * Dispose resources
   */
  dispose(): void {
    this._onDidChangeTreeData.dispose();
    this.clearCache();
  }
}

