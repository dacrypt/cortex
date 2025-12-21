/**
 * ContextTreeProvider - Provides virtual tree view grouped by projects
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

/**
 * Tree item for project view
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
      // Project item - folder icon
      this.iconPath = vscode.ThemeIcon.Folder;
      this.contextValue = 'cortex-context';
    }
  }
}

/**
 * TreeDataProvider for Project view
 */
export class ContextTreeProvider
  implements vscode.TreeDataProvider<ContextTreeItem>
{
  private static readonly suggestedPrefix = 'suggested:';
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
      // Root level - show all projects
      return this.getContextNodes();
    } else if (element.contextName) {
      // Project level - show files in this project
      return this.getFilesInContext(element.contextName);
    } else {
      return [];
    }
  }

  /**
   * Get all project nodes
   */
  private getContextNodes(): ContextTreeItem[] {
    const contexts = this.metadataStore.getAllContexts();
    const suggestedContexts = this.metadataStore.getAllSuggestedContexts();

    if (contexts.length === 0 && suggestedContexts.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      // Show placeholder when no projects exist
      const placeholder = new ContextTreeItem(
        'No projects yet',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      placeholder.tooltip =
        'Use "Cortex: Assign project to current file" to create projects';
      return [placeholder];
    }

    const items: ContextTreeItem[] = contexts.map((context) => {
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

    const existingContexts = new Set(contexts);
    suggestedContexts.forEach((context) => {
      if (existingContexts.has(context)) {
        return;
      }
      const fileCount =
        this.metadataStore.getFilesBySuggestedContext(context).length;
      const label = `${context} (${fileCount})`;
      const key = `${ContextTreeProvider.suggestedPrefix}${context}`;
      const isExpanded = this.accordionState.isExpanded(key);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;
      const item = new ContextTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        key
      );
      item.description = 'suggested';
      item.tooltip =
        'AI-suggested project. Use "Cortex: Suggest project for current file" to apply.';
      item.iconPath = new vscode.ThemeIcon('lightbulb');
      items.push(item);
    });

    return items;
  }

  /**
   * Get files in a specific project
   */
  private getFilesInContext(context: string): ContextTreeItem[] {
    const isSuggested = context.startsWith(
      ContextTreeProvider.suggestedPrefix
    );
    const resolvedContext = isSuggested
      ? context.slice(ContextTreeProvider.suggestedPrefix.length)
      : context;
    const relativePaths = isSuggested
      ? this.metadataStore.getFilesBySuggestedContext(resolvedContext)
      : this.metadataStore.getFilesByContext(resolvedContext);

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
    const contexts = this.metadataStore.getAllContexts();
    const suggestedContexts = this.metadataStore.getAllSuggestedContexts().map(
      (context) => `${ContextTreeProvider.suggestedPrefix}${context}`
    );
    return contexts.concat(suggestedContexts);
  }
}
