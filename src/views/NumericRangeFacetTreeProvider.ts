/**
 * NumericRangeFacetTreeProvider - Displays numeric range facets (size, complexity, etc.)
 *
 * Shows aggregated counts within numeric ranges
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { FileCacheService } from '../core/FileCacheService';
import { formatSize } from '../utils/sizeUtils';
import { getFileActivityTimestamp, sortFilesByActivity, type ActivityFileEntry } from '../utils/fileActivity';
import { t } from './i18n';

type TreeChangeEvent = NumericRangeFacetTreeItem | undefined | null | void;

interface NumericRange {
  label: string;
  count?: number;
  min?: number;
  max?: number;
}

interface NumericRangeFacetResult {
  numeric_range?: {
    ranges?: NumericRange[];
  };
}

interface FacetResponse {
  results?: NumericRangeFacetResult[];
}

class NumericRangeFacetTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly rangeLabel?: string,
    public readonly field?: string,
    public readonly minValue?: number,
    public readonly maxValue?: number
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
      this.iconPath = new vscode.ThemeIcon('database');
      this.contextValue = 'cortex-facet-range';
    } else {
      this.iconPath = new vscode.ThemeIcon('graph');
      this.contextValue = 'cortex-facet-field';
    }
  }
}

export class NumericRangeFacetTreeProvider
  implements vscode.TreeDataProvider<NumericRangeFacetTreeItem>
{
  private readonly _onDidChangeTreeData: vscode.EventEmitter<TreeChangeEvent> =
    new vscode.EventEmitter<TreeChangeEvent>();
  readonly onDidChangeTreeData: vscode.Event<TreeChangeEvent> =
    this._onDidChangeTreeData.event;
  private readonly knowledgeClient: GrpcKnowledgeClient;
  private readonly workspaceId: string;
  private readonly workspaceRoot: string;
  private readonly fileCacheService: FileCacheService;
  private facetField: string = 'size'; // Default field
  private readonly facetCache: Map<string, { ranges: Array<{ label: string; count: number; min: number; max: number }>; timestamp: number }> = new Map();
  private readonly CACHE_TTL = 30000; // 30 seconds

  constructor(
    workspaceRoot: string,
    context: vscode.ExtensionContext,
    workspaceId: string,
    field: string = 'size'
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

  getTreeItem(element: NumericRangeFacetTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: NumericRangeFacetTreeItem): Promise<NumericRangeFacetTreeItem[]> {
    if (!element) {
      return await this.getRanges();
    } else if (element.rangeLabel && element.field) {
      return await this.getFilesInRange(element);
    } else {
      return [];
    }
  }

  private async getRanges(): Promise<NumericRangeFacetTreeItem[]> {
    try {
      const cached = this.facetCache.get(this.facetField);
      if (cached && Date.now() - cached.timestamp < this.CACHE_TTL) {
        return this.buildRangeItems(cached.ranges);
      }

      const response = await this.knowledgeClient.getFacets(
        this.workspaceId,
        [{ field: this.facetField, type: 'numeric_range' }]
      ) as FacetResponse;

      if (!response?.results || response.results.length === 0) {
        return this.getEmptyPlaceholder();
      }

      const facetResult = response.results[0];
      if (!facetResult.numeric_range?.ranges) {
        return this.getEmptyPlaceholder();
      }

      const ranges = facetResult.numeric_range.ranges.map((r: NumericRange) => ({
        label: r.label,
        count: r.count || 0,
        min: r.min || 0,
        max: r.max || 0
      }));

      // Sort by min value ascending
      ranges.sort((a, b) => a.min - b.min);

      this.facetCache.set(this.facetField, {
        ranges,
        timestamp: Date.now()
      });

      return this.buildRangeItems(ranges);
    } catch (error) {
      console.error(`[NumericRangeFacetTree] Error fetching facets for ${this.facetField}:`, error);
      const errorItem = new NumericRangeFacetTreeItem(
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
    ranges: Array<{ label: string; count: number; min: number; max: number }>
  ): NumericRangeFacetTreeItem[] {
    if (ranges.length === 0) {
      return this.getEmptyPlaceholder();
    }

    return ranges.map(({ label, count, min, max }) => {
      const displayLabel = `${label} (${count})`;

      const item = new NumericRangeFacetTreeItem(
        displayLabel,
        vscode.TreeItemCollapsibleState.Collapsed,
        undefined,
        false,
        label,
        this.facetField,
        min,
        max
      );
      item.id = `range:${this.facetField}:${min}:${max}`;

      const rangeDesc = this.getRangeDescription(this.facetField, min, max);
      item.tooltip = `${this.facetField}: ${rangeDesc}\nCount: ${count} files`;
      item.description = `${count} files`;

      return item;
    });
  }

  private async getFilesInRange(element: NumericRangeFacetTreeItem): Promise<NumericRangeFacetTreeItem[]> {
    const filesCache = await this.fileCacheService.getFiles();
    const minValue = element.minValue ?? 0;
    const maxValue = element.maxValue ?? 0;

    const matchingFiles = filesCache.filter((file: ActivityFileEntry) => {
      const value = this.getNumericFieldValue(file, this.facetField);
      if (maxValue === 0) {
        return value >= minValue;
      }
      return value >= minValue && value <= maxValue;
    });

    sortFilesByActivity(matchingFiles);

    return matchingFiles.map((file) => {
      const relativePath = file.relative_path || '';
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);
      const activityTime = getFileActivityTimestamp(file);

      const item = new NumericRangeFacetTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );
      item.id = `file:numeric-range:${this.facetField}:${relativePath}`;

      item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private getEmptyPlaceholder(): NumericRangeFacetTreeItem[] {
    const placeholder = new NumericRangeFacetTreeItem(
      t('noFacetData', { field: this.facetField }),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('info');
    placeholder.tooltip = t('noFacetDataTooltip', { field: this.facetField });
    placeholder.id = `placeholder:empty:${this.facetField}`;
    return [placeholder];
  }

  private getNumericFieldValue(file: ActivityFileEntry, field: string): number {
    const normalized = this.normalizeNumericField(field);

    if (normalized === 'size') {
      return Number(file.file_size || file.enhanced?.stats?.size || 0);
    }
    if (normalized === 'complexity') {
      return Number(file.enhanced?.code_metrics?.complexity || 0);
    }
    if (normalized === 'function_count') {
      return Number(file.enhanced?.code_metrics?.function_count || 0);
    }
    if (normalized === 'lines_of_code') {
      return Number(file.enhanced?.code_metrics?.lines_of_code || 0);
    }
    if (normalized === 'comment_percentage') {
      return Number(file.enhanced?.code_metrics?.comment_percentage || 0);
    }
    if (normalized === 'project_score') {
      const entry = file as { project_score?: number; assignment_score?: number };
      return Number(entry.project_score || entry.assignment_score || 0);
    }
    if (normalized === 'content_quality') {
      return Number(file.enhanced?.content_quality?.quality_score || 0);
    }
    if (normalized === 'image_dimensions') {
      const width = Number(file.enhanced?.image_metadata?.width || 0);
      const height = Number(file.enhanced?.image_metadata?.height || 0);
      if (width <= 0 || height <= 0) {
        return 0;
      }
      return width * height;
    }
    if (normalized === 'image_color_depth') {
      return Number(file.enhanced?.image_metadata?.color_depth || 0);
    }
    if (normalized === 'image_iso') {
      return Number(file.enhanced?.image_metadata?.iso || file.enhanced?.image_metadata?.exif_iso || 0);
    }
    if (normalized === 'image_aperture') {
      return Number(file.enhanced?.image_metadata?.f_number || file.enhanced?.image_metadata?.exif_fnumber || 0);
    }
    if (normalized === 'image_focal_length') {
      return Number(file.enhanced?.image_metadata?.focal_length || file.enhanced?.image_metadata?.exif_focal_length || 0);
    }
    if (normalized === 'audio_duration') {
      return Number(file.enhanced?.audio_metadata?.duration || 0);
    }
    if (normalized === 'audio_bitrate') {
      return Number(file.enhanced?.audio_metadata?.bitrate || 0);
    }
    if (normalized === 'audio_sample_rate') {
      return Number(file.enhanced?.audio_metadata?.sample_rate || 0);
    }
    if (normalized === 'video_duration') {
      return Number(file.enhanced?.video_metadata?.duration || 0);
    }
    if (normalized === 'video_bitrate') {
      return Number(file.enhanced?.video_metadata?.bitrate || 0);
    }
    if (normalized === 'video_frame_rate') {
      return Number(file.enhanced?.video_metadata?.frame_rate || 0);
    }
    if (normalized === 'language_confidence') {
      return Number(file.enhanced?.language_confidence || 0);
    }
    return 0;
  }

  private normalizeNumericField(field: string): string {
    const normalized = field.trim().toLowerCase();
    if (normalized === 'file_size' || normalized === 'size') {
      return 'size';
    }
    if (normalized === 'comment_pct' || normalized === 'comment_percentage') {
      return 'comment_percentage';
    }
    if (normalized === 'loc' || normalized === 'lines_of_code') {
      return 'lines_of_code';
    }
    if (normalized === 'assignment_score' || normalized === 'project_score') {
      return 'project_score';
    }
    if (normalized === 'content_quality' || normalized === 'quality_score') {
      return 'content_quality';
    }
    if (normalized === 'image_dimensions' || normalized === 'image_size') {
      return 'image_dimensions';
    }
    if (normalized === 'image_color_depth') {
      return 'image_color_depth';
    }
    if (normalized === 'image_iso' || normalized === 'iso') {
      return 'image_iso';
    }
    if (normalized === 'image_aperture' || normalized === 'aperture') {
      return 'image_aperture';
    }
    if (normalized === 'image_focal_length' || normalized === 'focal_length') {
      return 'image_focal_length';
    }
    if (normalized === 'audio_duration' || normalized === 'duration') {
      return 'audio_duration';
    }
    if (normalized === 'audio_bitrate') {
      return 'audio_bitrate';
    }
    if (normalized === 'audio_sample_rate') {
      return 'audio_sample_rate';
    }
    if (normalized === 'video_duration') {
      return 'video_duration';
    }
    if (normalized === 'video_bitrate') {
      return 'video_bitrate';
    }
    if (normalized === 'video_frame_rate' || normalized === 'frame_rate' || normalized === 'fps') {
      return 'video_frame_rate';
    }
    if (normalized === 'language_confidence') {
      return 'language_confidence';
    }
    return normalized;
  }

  private getRangeDescription(field: string, min: number, max: number): string {
    const normalized = this.normalizeNumericField(field);
    const formatValue = (value: number) => {
      if (normalized === 'size') {
        return formatSize(value);
      }
      if (normalized === 'comment_percentage') {
        return `${value}%`;
      }
      if (normalized === 'project_score') {
        return value.toFixed(2);
      }
      if (normalized === 'content_quality') {
        return `${Math.round(value * 100)}%`;
      }
      if (normalized === 'image_dimensions') {
        return `${(value / 1e6).toFixed(1)} MP`;
      }
      if (normalized === 'image_color_depth') {
        return `${value} bpp`;
      }
      if (normalized === 'image_iso') {
        return `ISO ${Math.round(value)}`;
      }
      if (normalized === 'image_aperture') {
        return `f/${value.toFixed(1)}`;
      }
      if (normalized === 'image_focal_length') {
        return `${value} mm`;
      }
      if (normalized === 'audio_duration') {
        return this.formatDurationSeconds(value);
      }
      if (normalized === 'audio_bitrate') {
        return `${Math.round(value)} kbps`;
      }
      if (normalized === 'audio_sample_rate') {
        return `${(value / 1000).toFixed(1)} kHz`;
      }
      if (normalized === 'video_duration') {
        return this.formatDurationSeconds(value);
      }
      if (normalized === 'video_bitrate') {
        return `${Math.round(value)} kbps`;
      }
      if (normalized === 'video_frame_rate') {
        return `${value.toFixed(1)} fps`;
      }
      if (normalized === 'language_confidence') {
        return `${Math.round(value * 100)}%`;
      }
      return String(value);
    };

    if (max === 0) {
      return `>= ${formatValue(min)}`;
    }
    return `${formatValue(min)} - ${formatValue(max)}`;
  }

  private formatDurationSeconds(value: number): string {
    if (!Number.isFinite(value) || value <= 0) {
      return '0s';
    }
    if (value < 60) {
      return `${Math.round(value)}s`;
    }
    if (value < 3600) {
      const minutes = Math.floor(value / 60);
      const seconds = Math.round(value % 60);
      return `${minutes}m ${seconds}s`;
    }
    const hours = Math.floor(value / 3600);
    const minutes = Math.floor((value % 3600) / 60);
    return `${hours}h ${minutes}m`;
  }
}
