/**
 * DateRangeFacetTreeProvider - Displays date range facets (modified, created, accessed, changed)
 *
 * Shows aggregated counts within date ranges
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { FileCacheService } from '../core/FileCacheService';
import { getFileActivityTimestamp, sortFilesByActivity, type ActivityFileEntry } from '../utils/fileActivity';
import { t } from './i18n';

type TreeChangeEvent = DateRangeFacetTreeItem | undefined | null | void;

interface DateRange {
  label: string;
  count?: number;
  start_unix?: number;
  end_unix?: number;
}

interface DateRangeFacetResult {
  date_range?: {
    ranges?: DateRange[];
  };
}

interface FacetResponse {
  results?: DateRangeFacetResult[];
}

class DateRangeFacetTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly rangeLabel?: string,
    public readonly field?: string,
    public readonly startUnix?: number,
    public readonly endUnix?: number
  ) {
    super(label, collapsibleState);

    if (isFile && resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [resourceUri],
      };
      this.iconPath = vscode.ThemeIcon.File;
      this.contextValue = 'cortex-file';
    } else if (rangeLabel) {
      this.iconPath = new vscode.ThemeIcon('calendar');
      this.contextValue = 'cortex-facet-range';
    } else {
      this.iconPath = new vscode.ThemeIcon('clock');
      this.contextValue = 'cortex-facet-field';
    }
  }
}

export class DateRangeFacetTreeProvider
  implements vscode.TreeDataProvider<DateRangeFacetTreeItem>
{
  private readonly _onDidChangeTreeData: vscode.EventEmitter<TreeChangeEvent> =
    new vscode.EventEmitter<TreeChangeEvent>();
  readonly onDidChangeTreeData: vscode.Event<TreeChangeEvent> =
    this._onDidChangeTreeData.event;
  private readonly knowledgeClient: GrpcKnowledgeClient;
  private readonly workspaceId: string;
  private readonly workspaceRoot: string;
  private readonly fileCacheService: FileCacheService;
  private facetField: string = 'modified'; // Default field
  private readonly facetCache: Map<string, { ranges: Array<{ label: string; count: number; start: number; end: number }>; timestamp: number }> = new Map();
  private readonly CACHE_TTL = 30000; // 30 seconds

  constructor(
    workspaceRoot: string,
    context: vscode.ExtensionContext,
    workspaceId: string,
    field: string = 'modified'
  ) {
    this.workspaceRoot = workspaceRoot;
    this.workspaceId = workspaceId;
    this.facetField = field;
    this.knowledgeClient = new GrpcKnowledgeClient(context);
    const adminClient = new GrpcAdminClient(context);
    this.fileCacheService = FileCacheService.getInstance(adminClient);
    this.fileCacheService.setWorkspaceId(workspaceId);
  }

  setField(field: string): void {
    this.facetField = field;
    this.facetCache.clear();
    this.refresh();
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: DateRangeFacetTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: DateRangeFacetTreeItem): Promise<DateRangeFacetTreeItem[]> {
    if (!element) {
      return await this.getRanges();
    } else if (element.rangeLabel && element.field) {
      return await this.getFilesInRange(element);
    } else {
      return [];
    }
  }

  private async getRanges(): Promise<DateRangeFacetTreeItem[]> {
    try {
      const cached = this.facetCache.get(this.facetField);
      if (cached && Date.now() - cached.timestamp < this.CACHE_TTL) {
        return this.buildRangeItems(cached.ranges);
      }

      const response = await this.knowledgeClient.getFacets(
        this.workspaceId,
        [{ field: this.facetField, type: 'date_range' }]
      ) as FacetResponse;

      if (!response?.results || response.results.length === 0) {
        return this.getEmptyPlaceholder();
      }

      const facetResult = response.results[0];
      if (!facetResult.date_range?.ranges) {
        return this.getEmptyPlaceholder();
      }

      const ranges = facetResult.date_range.ranges.map((r: DateRange) => ({
        label: r.label,
        count: r.count || 0,
        start: r.start_unix || 0,
        end: r.end_unix || 0
      }));

      // Sort by start time descending (most recent first)
      ranges.sort((a, b) => b.start - a.start);

      this.facetCache.set(this.facetField, {
        ranges,
        timestamp: Date.now()
      });

      return this.buildRangeItems(ranges);
    } catch (error) {
      console.error(`[DateRangeFacetTree] Error fetching facets for ${this.facetField}:`, error);
      const errorItem = new DateRangeFacetTreeItem(
        t('errorLoadingFacetField', { field: this.facetField }),
        vscode.TreeItemCollapsibleState.None
      );
      errorItem.iconPath = new vscode.ThemeIcon('error');
      errorItem.tooltip = t('errorLoadingFacets');
      errorItem.id = `error:${this.facetField}`;
      return [errorItem];
    }
  }

  private buildRangeItems(
    ranges: Array<{ label: string; count: number; start: number; end: number }>
  ): DateRangeFacetTreeItem[] {
    if (ranges.length === 0) {
      return this.getEmptyPlaceholder();
    }

    return ranges.map(({ label, count, start, end }) => {
      const displayLabel = `${label} (${count})`;

      const item = new DateRangeFacetTreeItem(
        displayLabel,
        vscode.TreeItemCollapsibleState.Collapsed,
        undefined,
        false,
        label,
        this.facetField,
        start,
        end
      );
      item.id = `range:${this.facetField}:${start}:${end}`;

      const startDate = new Date(start);
      const endDate = end > 0 ? new Date(end) : null;
      const rangeDesc = endDate
        ? `${startDate.toLocaleDateString()} - ${endDate.toLocaleDateString()}`
        : `Since ${startDate.toLocaleDateString()}`;
      item.tooltip = `${this.facetField}: ${rangeDesc}\nCount: ${count} files`;
      item.description = `${count} files`;

      return item;
    });
  }

  private async getFilesInRange(element: DateRangeFacetTreeItem): Promise<DateRangeFacetTreeItem[]> {
    const filesCache = await this.fileCacheService.getFiles();
    const start = this.normalizeTimestamp(element.startUnix || 0);
    const end = this.normalizeTimestamp(element.endUnix || 0);
    const field = element.field || this.facetField;

    const matchingFiles = filesCache.filter((file: ActivityFileEntry) => {
      const value = this.getDateFieldValue(file, field);
      if (value === 0) {
        return false;
      }
      if (end === 0) {
        return value >= start;
      }
      return value >= start && value <= end;
    });

    sortFilesByActivity(matchingFiles);

    return matchingFiles.map((file) => {
      const relativePath = file.relative_path || '';
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);
      const activityTime = getFileActivityTimestamp(file);

      const item = new DateRangeFacetTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );
      item.id = `file:date-range:${this.facetField}:${relativePath}`;

      item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private getDateFieldValue(file: ActivityFileEntry, field: string): number {
    const normalized = this.normalizeDateField(field);
    if (normalized === 'created') {
      return this.normalizeTimestamp(
        Number(file.created_at || file.enhanced?.stats?.created || 0)
      );
    }
    if (normalized === 'accessed') {
      return this.normalizeTimestamp(
        Number(file.accessed_at || file.enhanced?.stats?.accessed || 0)
      );
    }
    if (normalized === 'changed') {
      return this.normalizeTimestamp(
        Number(file.changed_at || file.enhanced?.stats?.changed || 0)
      );
    }
    return this.normalizeTimestamp(
      Number(file.last_modified || file.enhanced?.stats?.modified || 0)
    );
  }

  private normalizeDateField(field: string): string {
    const normalized = field.trim().toLowerCase();
    if (normalized === 'created_at' || normalized === 'created') {
      return 'created';
    }
    if (normalized === 'accessed_at' || normalized === 'accessed') {
      return 'accessed';
    }
    if (normalized === 'changed_at' || normalized === 'changed' || normalized === 'ctime') {
      return 'changed';
    }
    if (normalized === 'last_modified' || normalized === 'modified' || normalized === 'mtime') {
      return 'modified';
    }
    return normalized;
  }

  private normalizeTimestamp(value: number): number {
    if (!value) {
      return 0;
    }
    // Heuristic: treat values in seconds as Unix seconds and convert to ms.
    return value < 1e12 ? value * 1000 : value;
  }

  private getEmptyPlaceholder(): DateRangeFacetTreeItem[] {
    const placeholder = new DateRangeFacetTreeItem(
      t('noFacetData', { field: this.facetField }),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('info');
    placeholder.tooltip = t('noFacetDataTooltip', { field: this.facetField });
    placeholder.id = `placeholder:empty:${this.facetField}`;
    return [placeholder];
  }
}
