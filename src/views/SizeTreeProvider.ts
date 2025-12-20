/**
 * SizeTreeProvider - Groups files by size
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { MetadataExtractor } from '../extractors/MetadataExtractor';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

class SizeTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly sizeCategory?: string
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
      this.iconPath = new vscode.ThemeIcon('database');
      this.contextValue = 'cortex-size';
    }
  }
}

export class SizeTreeProvider
  implements vscode.TreeDataProvider<SizeTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    SizeTreeItem | undefined | null | void
  > = new vscode.EventEmitter<SizeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    SizeTreeItem | undefined | null | void
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

  handleDidExpand(element: SizeTreeItem): void {
    this.accordionState.handleDidExpand(element.sizeCategory);
  }

  handleDidCollapse(element: SizeTreeItem): void {
    this.accordionState.handleDidCollapse(element.sizeCategory);
  }

  getTreeItem(element: SizeTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: SizeTreeItem): Promise<SizeTreeItem[]> {
    if (!element) {
      return this.getSizeCategories();
    } else if (element.sizeCategory) {
      return this.getFilesInCategory(element.sizeCategory);
    } else {
      return [];
    }
  }

  private getSizeCategories(): SizeTreeItem[] {
    const categories = this.getSizeCategoryEntries().map(
      ({ category, count, totalSize }) => {
        const label = `${category} (${count} files, ${this.extractor.formatSize(totalSize)})`;
        const isExpanded = this.accordionState.isExpanded(category);
        const collapsibleState = isExpanded
          ? vscode.TreeItemCollapsibleState.Expanded
          : vscode.TreeItemCollapsibleState.Collapsed;

        return new SizeTreeItem(
          label,
          collapsibleState,
          undefined,
          false,
          category
        );
      }
    );
    if (categories.length === 0) {
      return [this.getIndexingPlaceholder()];
    }
    return categories;
  }

  private getFilesInCategory(category: string): SizeTreeItem[] {
    const files = this.indexStore.getAllFiles();

    const matchingFiles = files.filter(
      (file) => this.extractor.categorizeSize(file.fileSize) === category
    );

    // Sort by size (largest first)
    matchingFiles.sort((a, b) => b.fileSize - a.fileSize);

    return matchingFiles.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = file.filename;

      const item = new SizeTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      item.tooltip = `${file.relativePath}\nSize: ${this.extractor.formatSize(file.fileSize)}`;
      item.description = this.extractor.formatSize(file.fileSize);

      return item;
    });
  }

  private getIndexingPlaceholder(): SizeTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new SizeTreeItem(
        'No files indexed',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new SizeTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getSizeCategoryEntries(): Array<{
    category: string;
    count: number;
    totalSize: number;
  }> {
    const files = this.indexStore.getAllFiles();
    const categoryCounts: Record<string, number> = {};
    const categoryTotalSize: Record<string, number> = {};

    for (const file of files) {
      const category = this.extractor.categorizeSize(file.fileSize);
      categoryCounts[category] = (categoryCounts[category] || 0) + 1;
      categoryTotalSize[category] =
        (categoryTotalSize[category] || 0) + file.fileSize;
    }

    const categoryOrder = ['Huge', 'Large', 'Medium', 'Small', 'Tiny'];

    return categoryOrder
      .filter((category) => categoryCounts[category] > 0)
      .map((category) => ({
        category,
        count: categoryCounts[category],
        totalSize: categoryTotalSize[category],
      }));
  }

  private getRootKeys(): string[] {
    return this.getSizeCategoryEntries().map((entry) => entry.category);
  }
}
