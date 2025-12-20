/**
 * TagTreeProvider - Provides virtual tree view grouped by tags
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

/**
 * Tree item for tag view
 */
class TagTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly tagName?: string
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
      this.iconPath = new vscode.ThemeIcon('tag');
      this.contextValue = 'cortex-tag';
    }
  }
}

/**
 * TreeDataProvider for Tag view
 */
export class TagTreeProvider
  implements vscode.TreeDataProvider<TagTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    TagTreeItem | undefined | null | void
  > = new vscode.EventEmitter<TagTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    TagTreeItem | undefined | null | void
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

  handleDidExpand(element: TagTreeItem): void {
    this.accordionState.handleDidExpand(element.tagName);
  }

  handleDidCollapse(element: TagTreeItem): void {
    this.accordionState.handleDidCollapse(element.tagName);
  }

  getTreeItem(element: TagTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: TagTreeItem): Promise<TagTreeItem[]> {
    if (!element) {
      return this.getTagNodes();
    } else if (element.tagName) {
      return this.getFilesWithTag(element.tagName);
    } else {
      return [];
    }
  }

  private getTagNodes(): TagTreeItem[] {
    const tags = this.metadataStore.getAllTags();

    if (tags.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      const placeholder = new TagTreeItem(
        'No tags yet',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      placeholder.tooltip = 'Use "Cortex: Add tag to current file" to create tags';
      return [placeholder];
    }

    return tags.map((tag) => {
      const fileCount = this.metadataStore.getFilesByTag(tag).length;
      const label = `${tag} (${fileCount})`;
      const isExpanded = this.accordionState.isExpanded(tag);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new TagTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        tag
      );
    });
  }

  private getFilesWithTag(tag: string): TagTreeItem[] {
    const relativePaths = this.metadataStore.getFilesByTag(tag);

    return relativePaths.map((relativePath) => {
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);

      const item = new TagTreeItem(
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

  private getIndexingPlaceholder(): TagTreeItem {
    const placeholder = new TagTreeItem(
      formatIndexingMessage(this.indexingStatus as IndexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootKeys(): string[] {
    return this.metadataStore.getAllTags();
  }
}
