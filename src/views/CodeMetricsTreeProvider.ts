/**
 * CodeMetricsTreeProvider - Groups code files by metrics
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { MetadataExtractor } from '../extractors/MetadataExtractor';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

class CodeMetricsTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly metricCategory?: string
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
    } else {
      this.iconPath = new vscode.ThemeIcon('graph');
      this.contextValue = 'cortex-metric';
    }
  }
}

export class CodeMetricsTreeProvider
  implements vscode.TreeDataProvider<CodeMetricsTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    CodeMetricsTreeItem | undefined | null | void
  > = new vscode.EventEmitter<CodeMetricsTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    CodeMetricsTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;

  constructor(
    private workspaceRoot: string,
    private indexStore: IndexStore,
    private metadataExtractor: MetadataExtractor,
    private indexingStatus?: IndexingStatus
  ) {
    this.accordionState = new TreeAccordionState(
      () => this.getRootKeys(),
      () => this.refresh()
    );
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  setAccordionEnabled(enabled: boolean): void {
    this.accordionState.setAccordionEnabled(enabled);
  }

  expandAll(): void {
    this.accordionState.expandAll();
  }

  collapseAll(): void {
    this.accordionState.collapseAll();
  }

  handleDidExpand(element: CodeMetricsTreeItem): void {
    this.accordionState.handleDidExpand(element.metricCategory);
  }

  handleDidCollapse(element: CodeMetricsTreeItem): void {
    this.accordionState.handleDidCollapse(element.metricCategory);
  }

  getTreeItem(element: CodeMetricsTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(
    element?: CodeMetricsTreeItem
  ): Promise<CodeMetricsTreeItem[]> {
    if (!element) {
      return this.getMetricCategories();
    } else if (element.metricCategory) {
      return this.getFilesInCategory(element.metricCategory);
    } else {
      return [];
    }
  }

  private getMetricCategories(): CodeMetricsTreeItem[] {
    const files = this.indexStore.getAllFiles();
    const codeFilesWithMetrics = files.filter((f) => f.enhanced?.codeMetadata);

    if (codeFilesWithMetrics.length === 0 && this.indexingStatus?.isIndexing) {
      return [this.getIndexingPlaceholder()];
    }

    // Debug logging
    console.log(`[CodeMetrics] === TREE VIEW DEBUG ===`);
    console.log(`[CodeMetrics] Total files in index: ${files.length}`);
    console.log(`[CodeMetrics] Files with enhanced: ${files.filter(f => f.enhanced).length}`);
    console.log(`[CodeMetrics] Files with language: ${files.filter(f => f.enhanced?.language).length}`);
    console.log(`[CodeMetrics] Files with codeMetadata: ${files.filter(f => f.enhanced?.codeMetadata).length}`);

    // Show sample data
    const sampleFiles = files.slice(0, 3).map(f => ({
      file: f.filename,
      hasEnhanced: !!f.enhanced,
      language: f.enhanced?.language,
      hasCodeMetadata: !!f.enhanced?.codeMetadata,
      loc: f.enhanced?.codeMetadata?.linesOfCode
    }));
    console.log(`[CodeMetrics] Sample files:`, sampleFiles);

    // Only code files with metrics
    const codeFiles = codeFilesWithMetrics;

    if (codeFiles.length === 0) {
      console.warn(`[CodeMetrics] No code files with metrics found!`);
      const placeholder = new CodeMetricsTreeItem(
        'No code files with metrics',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return [placeholder];
    }

    console.log(`[CodeMetrics] Found ${codeFiles.length} files with code metrics`);

    // Calculate statistics
    const totalLOC = codeFiles.reduce(
      (sum, f) => sum + (f.enhanced?.codeMetadata?.linesOfCode || 0),
      0
    );
    const avgCommentPercentage =
      codeFiles.reduce(
        (sum, f) => sum + (f.enhanced?.codeMetadata?.commentPercentage || 0),
        0
      ) / codeFiles.length;

    const categories = [
      {
        id: 'by-size',
        label: `By Size (${totalLOC.toLocaleString()} total LOC)`,
        description: 'Files grouped by lines of code',
      },
      {
        id: 'by-comments',
        label: `By Comments (${avgCommentPercentage.toFixed(1)}% avg)`,
        description: 'Files grouped by comment percentage',
      },
      {
        id: 'by-complexity',
        label: 'By Complexity',
        description: 'Files by function/class count',
      },
      {
        id: 'top-files',
        label: `Largest Files (Top 10)`,
        description: 'Files with most lines of code',
      },
      {
        id: 'well-commented',
        label: 'Well Commented (>20%)',
        description: 'Files with good documentation',
      },
      {
        id: 'poorly-commented',
        label: 'Poorly Commented (<5%)',
        description: 'Files needing documentation',
      },
    ];

    return categories.map((cat) => {
      const isExpanded = this.accordionState.isExpanded(cat.id);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new CodeMetricsTreeItem(
        cat.label,
        collapsibleState,
        undefined,
        false,
        cat.id
      );
    });
  }

  private getFilesInCategory(category: string): CodeMetricsTreeItem[] {
    const files = this.indexStore.getAllFiles();
    const codeFiles = files.filter((f) => f.enhanced?.codeMetadata);

    let filtered: typeof codeFiles = [];

    switch (category) {
      case 'by-size':
        // Group by LOC ranges
        const ranges = [
          { min: 0, max: 50, label: 'Tiny (< 50 LOC)' },
          { min: 50, max: 200, label: 'Small (50-200 LOC)' },
          { min: 200, max: 500, label: 'Medium (200-500 LOC)' },
          { min: 500, max: 1000, label: 'Large (500-1000 LOC)' },
          { min: 1000, max: Infinity, label: 'Huge (> 1000 LOC)' },
        ];

        // Return size categories as items
        return ranges
          .map((range) => {
            const count = codeFiles.filter((f) => {
              const loc = f.enhanced?.codeMetadata?.linesOfCode || 0;
              return loc >= range.min && loc < range.max;
            }).length;

            if (count === 0) return null;

            const item = new CodeMetricsTreeItem(
              `${range.label} (${count})`,
              vscode.TreeItemCollapsibleState.None
            );
            item.tooltip = `${count} files in this range`;
            return item;
          })
          .filter((item): item is CodeMetricsTreeItem => item !== null);

      case 'top-files':
        filtered = codeFiles
          .sort(
            (a, b) =>
              (b.enhanced?.codeMetadata?.linesOfCode || 0) -
              (a.enhanced?.codeMetadata?.linesOfCode || 0)
          )
          .slice(0, 10);
        break;

      case 'well-commented':
        filtered = codeFiles.filter(
          (f) => (f.enhanced?.codeMetadata?.commentPercentage || 0) > 20
        );
        break;

      case 'poorly-commented':
        filtered = codeFiles.filter(
          (f) => (f.enhanced?.codeMetadata?.commentPercentage || 0) < 5
        );
        break;

      default:
        filtered = codeFiles;
    }

    return filtered.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);

      const metrics = file.enhanced?.codeMetadata!;
      const item = new CodeMetricsTreeItem(
        file.filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      item.tooltip = `${file.relativePath}
LOC: ${metrics.linesOfCode}
Comments: ${metrics.commentPercentage.toFixed(1)}%
Functions: ${metrics.functions}
Classes: ${metrics.classes}`;

      item.description = `${metrics.linesOfCode} LOC, ${metrics.commentPercentage.toFixed(0)}% comments`;

      return item;
    });
  }

  private getIndexingPlaceholder(): CodeMetricsTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new CodeMetricsTreeItem(
        'No code files with metrics',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new CodeMetricsTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootKeys(): string[] {
    const files = this.indexStore.getAllFiles();
    const codeFiles = files.filter((f) => f.enhanced?.codeMetadata);
    if (codeFiles.length === 0) {
      return [];
    }
    return [
      'by-size',
      'by-comments',
      'by-complexity',
      'top-files',
      'well-commented',
      'poorly-commented',
    ];
  }
}
