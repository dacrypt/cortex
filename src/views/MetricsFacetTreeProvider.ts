/**
 * MetricsFacetTreeProvider - Displays aggregated metric ranges for code/document metrics.
 *
 * Generates range buckets per metric field and returns matching files.
 */

import * as vscode from 'vscode';
import { BaseFacetTreeProvider } from './base/BaseFacetTreeProvider';
import { IFacetTreeItem } from './contracts/IFacetProvider';
import { ViewNodeKind } from './contracts/viewNodes';

type MetricDefinition = {
  field: string;
  label: string;
};

type MetricRange = {
  label: string;
  min: number;
  max: number;
  count: number;
  missing?: boolean;
};

const METRIC_DEFINITIONS: Record<string, MetricDefinition[]> = {
  code_metrics: [
    { field: 'complexity', label: 'Complexity' },
    { field: 'lines_of_code', label: 'Lines of Code' },
    { field: 'comment_percentage', label: 'Comment Percentage' },
    { field: 'function_count', label: 'Function Count' },
  ],
  document_metrics: [
    { field: 'page_count', label: 'Page Count' },
    { field: 'word_count', label: 'Word Count' },
    { field: 'character_count', label: 'Character Count' },
  ],
};

class MetricsFacetTreeItem extends vscode.TreeItem implements IFacetTreeItem {
  id: string;
  kind: ViewNodeKind;
  facet?: string;
  value?: string;
  count?: number;
  payload?: any;
  isFile?: boolean;
  metricField?: string;
  minValue?: number;
  maxValue?: number;
  isMissing?: boolean;

  constructor(params: {
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
    icon?: vscode.ThemeIcon;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    contextValue?: string;
    metricField?: string;
    minValue?: number;
    maxValue?: number;
    isMissing?: boolean;
  }) {
    super(params.label, params.collapsibleState);

    this.id = params.id;
    this.kind = params.kind;
    this.facet = params.facet;
    this.value = params.value;
    this.count = params.count;
    this.payload = params.payload;
    this.isFile = params.isFile;
    this.metricField = params.metricField;
    this.minValue = params.minValue;
    this.maxValue = params.maxValue;
    this.isMissing = params.isMissing;

    if (params.description) this.description = params.description;
    if (params.tooltip) this.tooltip = params.tooltip;
    if (params.icon) this.iconPath = params.icon;
    if (params.contextValue) this.contextValue = params.contextValue;

    if (params.isFile && params.resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [params.resourceUri],
      };
      if (!this.iconPath) this.iconPath = vscode.ThemeIcon.File;
      if (!this.contextValue) this.contextValue = 'cortex-file';
    } else if (params.kind === 'facet' && !this.iconPath) {
      this.iconPath = new vscode.ThemeIcon('graph');
      if (!this.contextValue) this.contextValue = 'cortex-facet-range';
    }
  }
}

export class MetricsFacetTreeProvider extends BaseFacetTreeProvider<MetricsFacetTreeItem> {
  async getChildren(element?: MetricsFacetTreeItem): Promise<MetricsFacetTreeItem[]> {
    if (!element) {
      return await this.getMetricRanges();
    }
    if (element.metricField && element.minValue !== undefined && element.maxValue !== undefined) {
      return await this.getFilesInRange(element);
    }
    return [];
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
    payload?: any;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    icon?: vscode.ThemeIcon;
    metricField?: string;
    minValue?: number;
    maxValue?: number;
    isMissing?: boolean;
  }): MetricsFacetTreeItem {
    return new MetricsFacetTreeItem(params);
  }

  private async getMetricRanges(): Promise<MetricsFacetTreeItem[]> {
    const metrics = METRIC_DEFINITIONS[this.config.field] || [];
    if (metrics.length === 0) {
      return [this.createEmptyPlaceholder(`No metrics defined for ${this.config.label}`)];
    }

    const allRanges: MetricsFacetTreeItem[] = [];

    for (const metric of metrics) {
      const ranges = await this.getRangesForMetric(metric.field);
      const missingCount = await this.countMissingMetric(metric.field);
      if (missingCount > 0) {
        ranges.push({ label: '?', min: 0, max: 0, count: missingCount, missing: true });
      }
      for (const range of ranges) {
        const value = `${metric.label}: ${range.label}`;
        const label = this.formatLabelWithCount(value, range.count);
        allRanges.push(
          this.createTreeItem({
            id: this.generateFacetId(`${metric.field}:${range.label}`),
            kind: 'facet',
            label,
            collapsibleState: vscode.TreeItemCollapsibleState.Collapsed,
            facet: this.config.field,
            value,
            count: range.count,
            metricField: metric.field,
            minValue: range.min,
            maxValue: range.max,
            isMissing: range.missing === true,
            payload: {
              facet: this.config.field,
              value,
              metadata: {
                metricField: metric.field,
                min: range.min,
                max: range.max,
                missing: range.missing === true,
              },
            },
            tooltip: `${metric.label} ${range.label}\n${range.count} files`,
          })
        );
      }
    }

    if (allRanges.length === 0) {
      return [this.createEmptyPlaceholder(`No metric data found for ${this.config.label}`)];
    }

    return allRanges;
  }

  private async getRangesForMetric(field: string): Promise<MetricRange[]> {
    return await this.getCached(`ranges:${this.config.field}:${field}`, async () => {
      const backend = await this.queryBackendFacet(field, 'numeric_range');
      const backendRanges = backend?.numeric_range?.ranges;
      if (Array.isArray(backendRanges) && backendRanges.length > 0) {
        return backendRanges
          .map((range: { label?: string; count?: number; min?: number; max?: number }) => ({
            label: range.label || `${range.min ?? 0}-${range.max ?? 0}`,
            min: range.min ?? 0,
            max: range.max ?? 0,
            count: range.count ?? 0,
          }))
          .filter((range) => Number.isFinite(range.min) && Number.isFinite(range.max));
      }

      const files = await this.getFiles();
      const values = files.map((file) => this.getMetricValue(file, field)).filter((value) => Number.isFinite(value));
      return this.buildRangesFromValues(field, values as number[]);
    });
  }

  private async getFilesInRange(element: MetricsFacetTreeItem): Promise<MetricsFacetTreeItem[]> {
    const files = await this.getFiles();
    const metricField = element.metricField || '';
    const minValue = element.minValue ?? 0;
    const maxValue = element.maxValue ?? 0;

    if (element.isMissing) {
      const missing = files.filter((file) => !Number.isFinite(this.getMetricValue(file, metricField)));
      if (missing.length === 0) {
        return [this.createEmptyPlaceholder('No files found')];
      }
      this.sortFilesByActivity(missing);
      return missing.map((file) => {
        const relativePath = file.relative_path || '';
        return this.createTreeItem({
          id: this.generateFileId(relativePath, metricField),
          kind: 'file',
          label: this.getFilename(relativePath),
          collapsibleState: vscode.TreeItemCollapsibleState.None,
          resourceUri: this.getFileUri(relativePath),
          isFile: true,
          facet: this.config.field,
          payload: {
            facet: this.config.field,
            value: relativePath,
            metadata: { metricField, missing: true },
          },
          description: this.getDirname(relativePath),
          tooltip: this.createFileTooltip(relativePath, file),
        });
      });
    }

    const matching = files.filter((file) => {
      const value = this.getMetricValue(file, metricField);
      if (!Number.isFinite(value)) {
        return false;
      }
      if (maxValue === minValue) {
        return value === minValue;
      }
      return value >= minValue && value <= maxValue;
    });

    if (matching.length === 0) {
      return [this.createEmptyPlaceholder('No files found')];
    }

    this.sortFilesByActivity(matching);

    return matching.map((file) => {
      const relativePath = file.relative_path || '';
      return this.createTreeItem({
        id: this.generateFileId(relativePath, metricField),
        kind: 'file',
        label: this.getFilename(relativePath),
        collapsibleState: vscode.TreeItemCollapsibleState.None,
        resourceUri: this.getFileUri(relativePath),
        isFile: true,
        facet: this.config.field,
        payload: {
          facet: this.config.field,
          value: relativePath,
          metadata: { metricField, min: minValue, max: maxValue },
        },
        description: this.getDirname(relativePath),
        tooltip: this.createFileTooltip(relativePath, file),
      });
    });
  }

  private async countMissingMetric(field: string): Promise<number> {
    const files = await this.getFiles();
    let missingCount = 0;
    for (const file of files) {
      const value = this.getMetricValue(file, field);
      if (!Number.isFinite(value)) {
        missingCount += 1;
      }
    }
    return missingCount;
  }

  private buildRangesFromValues(field: string, values: number[]): MetricRange[] {
    if (!values || values.length === 0) {
      return [];
    }

    const min = Math.min(...values);
    const max = Math.max(...values);

    if (min === max) {
      return [
        {
          label: this.formatRangeLabel(field, min, max),
          min,
          max,
          count: values.length,
        },
      ];
    }

    const bucketCount = 5;
    const step = (max - min) / bucketCount;
    const ranges: MetricRange[] = [];

    for (let i = 0; i < bucketCount; i += 1) {
      const rangeMin = min + step * i;
      const rangeMax = i === bucketCount - 1 ? max : min + step * (i + 1);
      const count = values.filter((value) => {
        if (i === bucketCount - 1) {
          return value >= rangeMin && value <= rangeMax;
        }
        return value >= rangeMin && value < rangeMax;
      }).length;

      if (count === 0) {
        continue;
      }

      ranges.push({
        label: this.formatRangeLabel(field, rangeMin, rangeMax),
        min: rangeMin,
        max: rangeMax,
        count,
      });
    }

    return ranges;
  }

  private formatRangeLabel(field: string, min: number, max: number): string {
    if (min === max) {
      return this.formatMetricValue(field, min);
    }
    return `${this.formatMetricValue(field, min)} - ${this.formatMetricValue(field, max)}`;
  }

  private formatMetricValue(field: string, value: number): string {
    const normalized = field.trim().toLowerCase();
    if (normalized === 'comment_percentage') {
      return `${Math.round(value)}%`;
    }
    if (normalized === 'page_count') {
      return `${Math.round(value)}`;
    }
    if (normalized === 'word_count') {
      return `${Math.round(value)}`;
    }
    if (normalized === 'character_count') {
      return `${Math.round(value)}`;
    }
    if (normalized === 'lines_of_code') {
      return `${Math.round(value)}`;
    }
    if (normalized === 'function_count') {
      return `${Math.round(value)}`;
    }
    return value.toFixed(2).replace(/\.00$/, '');
  }

  private getMetricValue(file: any, field: string): number {
    const normalized = field.trim().toLowerCase();
    if (normalized === 'complexity') {
      return Number(file.enhanced?.code_metrics?.complexity ?? 0);
    }
    if (normalized === 'lines_of_code') {
      return Number(file.enhanced?.code_metrics?.lines_of_code ?? 0);
    }
    if (normalized === 'comment_percentage') {
      return Number(file.enhanced?.code_metrics?.comment_percentage ?? 0);
    }
    if (normalized === 'function_count') {
      return Number(file.enhanced?.code_metrics?.function_count ?? 0);
    }
    if (normalized === 'page_count') {
      return Number(file.enhanced?.document_metrics?.page_count ?? 0);
    }
    if (normalized === 'word_count') {
      return Number(file.enhanced?.document_metrics?.word_count ?? 0);
    }
    if (normalized === 'character_count') {
      return Number(file.enhanced?.document_metrics?.character_count ?? 0);
    }
    return 0;
  }
}
