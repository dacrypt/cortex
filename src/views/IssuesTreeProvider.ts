/**
 * IssuesTreeProvider - Shows files with errors during indexing
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { BlacklistStore, BlacklistEntry } from '../core/BlacklistStore';
import { TreeAccordionState } from './TreeAccordionState';

class IssuesTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly errorCode?: string
  ) {
    super(label, collapsibleState);

    if (isFile && resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [resourceUri],
      };
      this.iconPath = new vscode.ThemeIcon('error', new vscode.ThemeColor('errorForeground'));
      this.contextValue = 'cortex-error-file';
    } else {
      this.iconPath = new vscode.ThemeIcon('warning');
      this.contextValue = 'cortex-error-category';
    }
  }
}

export class IssuesTreeProvider
  implements vscode.TreeDataProvider<IssuesTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    IssuesTreeItem | undefined | null | void
  > = new vscode.EventEmitter<IssuesTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    IssuesTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;

  constructor(
    private workspaceRoot: string,
    private indexStore: IndexStore,
    private indexingStatus?: IndexingStatus,
    private blacklistStore?: BlacklistStore
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

  handleDidExpand(element: IssuesTreeItem): void {
    this.accordionState.handleDidExpand(element.errorCode);
  }

  handleDidCollapse(element: IssuesTreeItem): void {
    this.accordionState.handleDidCollapse(element.errorCode);
  }

  getTreeItem(element: IssuesTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(
    element?: IssuesTreeItem
  ): Promise<IssuesTreeItem[]> {
    if (!element) {
      return this.getErrorCategories();
    } else if (element.errorCode) {
      return this.getFilesInCategory(element.errorCode);
    } else {
      return [];
    }
  }

  private getErrorCategories(): IssuesTreeItem[] {
    const files = this.indexStore.getAllFiles();
    const filesWithErrors = files.filter(f => f.enhanced?.error);
    const blacklistEntries = this.blacklistStore?.getEntries() || [];

    if (filesWithErrors.length === 0 && blacklistEntries.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      const placeholder = new IssuesTreeItem(
        'No issues found',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('check', new vscode.ThemeColor('testing.iconPassed'));
      return [placeholder];
    }

    // Group by error code
    const errorGroups = new Map<string, typeof filesWithErrors>();
    filesWithErrors.forEach(file => {
      const code = file.enhanced?.error?.code || 'UNKNOWN';
      if (!errorGroups.has(code)) {
        errorGroups.set(code, []);
      }
      errorGroups.get(code)!.push(file);
    });

    // Create category items
    const categories: IssuesTreeItem[] = [];

    errorGroups.forEach((files, code) => {
      let label = '';
      let description = '';

      switch (code) {
        case 'ENAMETOOLONG':
          label = `Path Too Long (${files.length})`;
          description = 'File paths exceed system limits';
          break;
        case 'ENOENT':
          label = `File Not Found (${files.length})`;
          description = 'Files were deleted or moved';
          break;
        case 'EACCES':
          label = `Permission Denied (${files.length})`;
          description = 'Cannot access these files';
          break;
        default:
          label = `${code} (${files.length})`;
          description = 'Other errors';
      }

      const item = new IssuesTreeItem(
        label,
        this.getCategoryState(code),
        undefined,
        false,
        code
      );
      item.description = description;
      item.tooltip = `${files.length} files with error code: ${code}`;
      categories.push(item);
    });

    if (blacklistEntries.length > 0) {
      const item = new IssuesTreeItem(
        `Blacklisted (${blacklistEntries.length})`,
        this.getCategoryState('BLACKLIST'),
        undefined,
        false,
        'BLACKLIST'
      );
      item.description = 'Skipped in future scans';
      item.tooltip = `${blacklistEntries.length} entries in blacklist`;
      categories.push(item);
    }

    return categories.sort((a, b) => a.label.localeCompare(b.label));
  }

  private getFilesInCategory(errorCode: string): IssuesTreeItem[] {
    const files = this.indexStore.getAllFiles();
    if (errorCode === 'BLACKLIST') {
      const entries = this.blacklistStore?.getEntries() || [];
      return this.getBlacklistItems(entries);
    }
    const filtered = files.filter(f => f.enhanced?.error?.code === errorCode);

    return filtered.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);

      const item = new IssuesTreeItem(
        file.filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      // Build detailed tooltip
      const error = file.enhanced?.error;
      let tooltip = `${file.relativePath}\n\n`;
      tooltip += `Error Code: ${error?.code}\n`;
      tooltip += `Message: ${error?.message}\n`;
      tooltip += `Operation: ${error?.operation}\n`;
      tooltip += `\nPath Length: ${file.absolutePath.length} characters`;

      if (error?.code === 'ENAMETOOLONG') {
        tooltip += `\n\nTip: macOS has a path limit of ~1024 characters.\nConsider shortening folder names.`;
      }

      item.tooltip = tooltip;
      item.description = `${error?.code}: ${error?.message.substring(0, 50)}...`;

      return item;
    });
  }

  private getBlacklistItems(entries: BlacklistEntry[]): IssuesTreeItem[] {
    return entries.map((entry) => {
      const absolutePath = path.join(this.workspaceRoot, entry.path);
      const uri = entry.type === 'file' ? vscode.Uri.file(absolutePath) : undefined;
      const label = path.basename(entry.path) || entry.path;

      const item = new IssuesTreeItem(
        label,
        vscode.TreeItemCollapsibleState.None,
        uri,
        entry.type === 'file'
      );
      item.tooltip = `Path: ${entry.path}\nCode: ${entry.code}\nMessage: ${entry.message}\nCount: ${entry.count}`;
      item.description = `${entry.code}: ${entry.message.substring(0, 50)}...`;
      return item;
    });
  }

  private getIndexingPlaceholder(): IssuesTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new IssuesTreeItem(
        'No issues found',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new IssuesTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getCategoryState(key: string): vscode.TreeItemCollapsibleState {
    return this.accordionState.isExpanded(key)
      ? vscode.TreeItemCollapsibleState.Expanded
      : vscode.TreeItemCollapsibleState.Collapsed;
  }

  private getRootKeys(): string[] {
    const files = this.indexStore.getAllFiles();
    const filesWithErrors = files.filter((f) => f.enhanced?.error);
    const errorCodes = new Set<string>();

    filesWithErrors.forEach((file) => {
      const code = file.enhanced?.error?.code || 'UNKNOWN';
      errorCodes.add(code);
    });

    const keys = Array.from(errorCodes).sort();
    const blacklistEntries = this.blacklistStore?.getEntries() || [];
    if (blacklistEntries.length > 0) {
      keys.push('BLACKLIST');
    }

    return keys;
  }
}
