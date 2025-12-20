/**
 * ContextTreeProvider - Provides virtual tree view grouped by contexts
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

/**
 * Tree item for context view
 */
class ContextTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly contextName?: string
  ) {
    super(label, collapsibleState);

    if (isFile && resourceUri) {
      // File item - make it clickable
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [resourceUri],
      };

      // Set icon based on file type
      this.iconPath = vscode.ThemeIcon.File;
      this.contextValue = 'cortex-file';
    } else {
      // Context item - folder icon
      this.iconPath = vscode.ThemeIcon.Folder;
      this.contextValue = 'cortex-context';
    }
  }
}

/**
 * TreeDataProvider for Context view
 */
export class ContextTreeProvider
  implements vscode.TreeDataProvider<ContextTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    ContextTreeItem | undefined | null | void
  > = new vscode.EventEmitter<ContextTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    ContextTreeItem | undefined | null | void
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

  /**
   * Refresh the tree view
   */
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

  handleDidExpand(element: ContextTreeItem): void {
    this.accordionState.handleDidExpand(element.contextName);
  }

  handleDidCollapse(element: ContextTreeItem): void {
    this.accordionState.handleDidCollapse(element.contextName);
  }

  /**
   * Get tree item
   */
  getTreeItem(element: ContextTreeItem): vscode.TreeItem {
    return element;
  }

  /**
   * Get children for a tree item
   */
  async getChildren(element?: ContextTreeItem): Promise<ContextTreeItem[]> {
    if (!element) {
      // Root level - show all contexts
      return this.getContextNodes();
    } else if (element.contextName) {
      // Context level - show files in this context
      return this.getFilesInContext(element.contextName);
    } else {
      return [];
    }
  }

  /**
   * Get all context nodes
   */
  private getContextNodes(): ContextTreeItem[] {
    const contexts = this.metadataStore.getAllContexts();

    if (contexts.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      // Show placeholder when no contexts exist
      const placeholder = new ContextTreeItem(
        'No contexts yet',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      placeholder.tooltip = 'Use "Cortex: Assign context to current file" to create contexts';
      return [placeholder];
    }

    return contexts.map((context) => {
      const fileCount = this.metadataStore.getFilesByContext(context).length;
      const label = `${context} (${fileCount})`;
      const isExpanded = this.accordionState.isExpanded(context);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new ContextTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        context
      );
    });
  }

  /**
   * Get files in a specific context
   */
  private getFilesInContext(context: string): ContextTreeItem[] {
    const relativePaths = this.metadataStore.getFilesByContext(context);

    return relativePaths.map((relativePath) => {
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);

      const item = new ContextTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      // Set tooltip with full path
      item.tooltip = relativePath;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private getIndexingPlaceholder(): ContextTreeItem {
    const placeholder = new ContextTreeItem(
      formatIndexingMessage(this.indexingStatus as IndexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootKeys(): string[] {
    return this.metadataStore.getAllContexts();
  }
}
