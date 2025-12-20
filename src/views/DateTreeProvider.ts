/**
 * DateTreeProvider - Groups files by modification date
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { MetadataExtractor } from '../extractors/MetadataExtractor';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

class DateTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly dateCategory?: string
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
      this.iconPath = new vscode.ThemeIcon('calendar');
      this.contextValue = 'cortex-date';
    }
  }
}

export class DateTreeProvider
  implements vscode.TreeDataProvider<DateTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    DateTreeItem | undefined | null | void
  > = new vscode.EventEmitter<DateTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    DateTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;

  private extractor: MetadataExtractor;

  constructor(
    private workspaceRoot: string,
    private indexStore: IndexStore,
    private indexingStatus?: IndexingStatus
  ) {
    this.extractor = new MetadataExtractor(workspaceRoot);
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

  handleDidExpand(element: DateTreeItem): void {
    this.accordionState.handleDidExpand(element.dateCategory);
  }

  handleDidCollapse(element: DateTreeItem): void {
    this.accordionState.handleDidCollapse(element.dateCategory);
  }

  getTreeItem(element: DateTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: DateTreeItem): Promise<DateTreeItem[]> {
    if (!element) {
      return this.getDateCategories();
    } else if (element.dateCategory) {
      return this.getFilesInCategory(element.dateCategory);
    } else {
      return [];
    }
  }

  private getDateCategories(): DateTreeItem[] {
    const categories = this.getDateCategoryEntries().map(({ category, count }) => {
      const label = `${category} (${count})`;
      const isExpanded = this.accordionState.isExpanded(category);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new DateTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        category
      );
    });
    if (categories.length === 0) {
      return [this.getIndexingPlaceholder()];
    }
    return categories;
  }

  private getFilesInCategory(category: string): DateTreeItem[] {
    const files = this.indexStore.getAllFiles();

    const matchingFiles = files.filter(
      (file) => this.extractor.categorizeDate(file.lastModified) === category
    );

    // Sort by modification time (most recent first)
    matchingFiles.sort((a, b) => b.lastModified - a.lastModified);

    return matchingFiles.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = file.filename;

      const item = new DateTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      item.tooltip = `${file.relativePath}\nModified: ${this.extractor.formatDate(file.lastModified)}`;
      item.description = this.extractor.formatDate(file.lastModified);

      return item;
    });
  }

  private getIndexingPlaceholder(): DateTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new DateTreeItem(
        'No files indexed',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new DateTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getDateCategoryEntries(): Array<{ category: string; count: number }> {
    const files = this.indexStore.getAllFiles();
    const categoryCounts: Record<string, number> = {};

    for (const file of files) {
      const category = this.extractor.categorizeDate(file.lastModified);
      categoryCounts[category] = (categoryCounts[category] || 0) + 1;
    }

    const categoryOrder = [
      'Last Hour',
      'Today',
      'This Week',
      'This Month',
      'Last 3 Months',
      'Last 6 Months',
      'This Year',
      'Older',
    ];

    return categoryOrder
      .filter((category) => categoryCounts[category] > 0)
      .map((category) => ({
        category,
        count: categoryCounts[category],
      }));
  }

  private getRootKeys(): string[] {
    return this.getDateCategoryEntries().map((entry) => entry.category);
  }
}
