/**
 * TypeTreeProvider - Provides virtual tree view grouped by file types
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

/**
 * Tree item for type view
 */
class TypeTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly typeName?: string
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
      this.iconPath = new vscode.ThemeIcon('symbol-file');
      this.contextValue = 'cortex-type';
    }
  }
}

/**
 * TreeDataProvider for Type view
 */
export class TypeTreeProvider
  implements vscode.TreeDataProvider<TypeTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    TypeTreeItem | undefined | null | void
  > = new vscode.EventEmitter<TypeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    TypeTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;

  constructor(
    private workspaceRoot: string,
    private metadataStore: IMetadataStore,
    private indexStore: IndexStore,
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

  handleDidExpand(element: TypeTreeItem): void {
    this.accordionState.handleDidExpand(element.typeName);
  }

  handleDidCollapse(element: TypeTreeItem): void {
    this.accordionState.handleDidCollapse(element.typeName);
  }

  getTreeItem(element: TypeTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: TypeTreeItem): Promise<TypeTreeItem[]> {
    if (!element) {
      return this.getTypeNodes();
    } else if (element.typeName) {
      return this.getFilesOfType(element.typeName);
    } else {
      return [];
    }
  }

  private getTypeNodes(): TypeTreeItem[] {
    const types = this.metadataStore.getAllTypes();

    if (types.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      const placeholder = new TypeTreeItem(
        'No files indexed',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return [placeholder];
    }

    return types.map((type) => {
      const fileCount = this.metadataStore.getFilesByType(type).length;
      const label = `${type} (${fileCount})`;
      const isExpanded = this.accordionState.isExpanded(type);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new TypeTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        type
      );
    });
  }

  private getFilesOfType(type: string): TypeTreeItem[] {
    const relativePaths = this.metadataStore.getFilesByType(type);

    return relativePaths.map((relativePath) => {
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);

      const item = new TypeTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      item.tooltip = relativePath;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private getIndexingPlaceholder(): TypeTreeItem {
    const placeholder = new TypeTreeItem(
      formatIndexingMessage(this.indexingStatus as IndexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootKeys(): string[] {
    return this.metadataStore.getAllTypes();
  }
}
