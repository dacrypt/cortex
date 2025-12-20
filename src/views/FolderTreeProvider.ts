/**
 * FolderTreeProvider - Semantic folder structure view
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

class FolderTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly folderPath?: string
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
      this.iconPath = vscode.ThemeIcon.Folder;
      this.contextValue = 'cortex-folder';
    }
  }
}

export class FolderTreeProvider
  implements vscode.TreeDataProvider<FolderTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    FolderTreeItem | undefined | null | void
  > = new vscode.EventEmitter<FolderTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    FolderTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;

  constructor(
    private workspaceRoot: string,
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

  handleDidExpand(element: FolderTreeItem): void {
    this.accordionState.handleDidExpand(element.folderPath);
  }

  handleDidCollapse(element: FolderTreeItem): void {
    this.accordionState.handleDidCollapse(element.folderPath);
  }

  getTreeItem(element: FolderTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: FolderTreeItem): Promise<FolderTreeItem[]> {
    if (!element) {
      return this.getRootFolders();
    } else if (element.folderPath !== undefined) {
      return this.getItemsInFolder(element.folderPath);
    } else {
      return [];
    }
  }

  private getRootFolders(): FolderTreeItem[] {
    const files = this.indexStore.getAllFiles();
    const folders = this.getRootFolderPaths();

    // Also count root-level files
    const rootFiles = files.filter(
      (file) => !file.relativePath.includes(path.sep)
    );

    const items: FolderTreeItem[] = [];

    // Add folders
    folders.forEach((folder) => {
      const folderFiles = files.filter((file) =>
        file.relativePath.startsWith(folder + path.sep)
      );
      const label = `${folder} (${folderFiles.length})`;
      const isExpanded = this.accordionState.isExpanded(folder);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      items.push(
        new FolderTreeItem(
          label,
          collapsibleState,
          undefined,
          false,
          folder
        )
      );
    });

    // Add root files
    if (rootFiles.length > 0) {
      rootFiles.forEach((file) => {
        const absolutePath = path.join(this.workspaceRoot, file.relativePath);
        const uri = vscode.Uri.file(absolutePath);

        const item = new FolderTreeItem(
          file.filename,
          vscode.TreeItemCollapsibleState.None,
          uri,
          true
        );
        item.tooltip = file.relativePath;
        items.push(item);
      });
    }

    if (items.length === 0) {
      return [this.getIndexingPlaceholder()];
    }

    return items;
  }

  private getItemsInFolder(folderPath: string): FolderTreeItem[] {
    const files = this.indexStore.getAllFiles();
    const items: FolderTreeItem[] = [];
    const subfolders = new Set<string>();

    // Find immediate children (files and subfolders)
    for (const file of files) {
      if (!file.relativePath.startsWith(folderPath + path.sep)) {
        continue;
      }

      const relativePart = file.relativePath.substring(
        folderPath.length + path.sep.length
      );
      const parts = relativePart.split(path.sep);

      if (parts.length === 1) {
        // Direct file in this folder
        const absolutePath = path.join(this.workspaceRoot, file.relativePath);
        const uri = vscode.Uri.file(absolutePath);

        const item = new FolderTreeItem(
          file.filename,
          vscode.TreeItemCollapsibleState.None,
          uri,
          true
        );
        item.tooltip = file.relativePath;
        items.push(item);
      } else {
        // File in subfolder
        subfolders.add(parts[0]);
      }
    }

    // Add subfolders
    Array.from(subfolders)
      .sort()
      .forEach((subfolder) => {
        const fullPath = path.join(folderPath, subfolder);
        const subfolderFiles = files.filter((file) =>
          file.relativePath.startsWith(fullPath + path.sep)
        );
        const label = `${subfolder} (${subfolderFiles.length})`;

        items.unshift(
          new FolderTreeItem(
            label,
            vscode.TreeItemCollapsibleState.Collapsed,
            undefined,
            false,
            fullPath
          )
        );
      });

    return items;
  }

  private getIndexingPlaceholder(): FolderTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new FolderTreeItem(
        'No files indexed',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new FolderTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootFolderPaths(): string[] {
    const files = this.indexStore.getAllFiles();
    const folders = new Set<string>();

    for (const file of files) {
      const parts = file.relativePath.split(path.sep);
      if (parts.length > 1) {
        folders.add(parts[0]);
      }
    }

    return Array.from(folders).sort();
  }

  private getRootKeys(): string[] {
    return this.getRootFolderPaths();
  }
}
