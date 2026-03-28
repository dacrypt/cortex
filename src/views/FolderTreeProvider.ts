/**
 * FolderTreeProvider - Semantic folder structure view
 * 
 * Queries backend for file data
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { FileCacheService } from '../core/FileCacheService';
import { getFileActivityTimestamp, sortFilesByActivity, type ActivityFileEntry } from '../utils/fileActivity';
import { IFacetTreeItem } from './contracts/IFacetProvider';
import { ViewNodeKind } from './contracts/viewNodes';
import { t } from './i18n';

type FileEntry = {
  relative_path?: string;
  filename?: string;
  last_modified?: number;
  created_at?: number;
  enhanced?: {
    stats?: {
      modified?: number;
      accessed?: number;
      created?: number;
    };
  };
};

type TreeChangeEvent = FolderTreeItem | undefined | null | void;

class FolderTreeItem extends vscode.TreeItem implements IFacetTreeItem {
  id: string;
  kind: ViewNodeKind;
  facet?: string;
  value?: string;
  count?: number;
  payload?: any;
  isFile: boolean;
  folderPath?: string;

  constructor(
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState,
    resourceUri?: vscode.Uri,
    isFile: boolean = false,
    folderPath?: string
  ) {
    super(label, collapsibleState);

    this.isFile = isFile;
    this.folderPath = folderPath;
    this.kind = isFile ? 'file' : 'facet';
    this.id = isFile ? `file:folder:${folderPath || 'root'}:${resourceUri?.fsPath || ''}` : `folder:${folderPath || 'root'}`;
    
    // Explicitly check isFile flag - if true, it's a file regardless of other conditions
    if (isFile) {
      // Files must have a resourceUri to be opened
      if (resourceUri) {
        this.command = {
          command: 'vscode.open',
          title: 'Open File',
          arguments: [resourceUri],
        };
      }
      this.iconPath = vscode.ThemeIcon.File;
      this.contextValue = 'cortex-file';
    } else {
      // Not a file, so it's a folder
      this.iconPath = vscode.ThemeIcon.Folder;
      this.contextValue = 'cortex-folder';
    }
  }
}

export class FolderTreeProvider
  implements vscode.TreeDataProvider<FolderTreeItem>
{
  private readonly _onDidChangeTreeData: vscode.EventEmitter<TreeChangeEvent> =
    new vscode.EventEmitter<TreeChangeEvent>();
  readonly onDidChangeTreeData: vscode.Event<TreeChangeEvent> =
    this._onDidChangeTreeData.event;
  private readonly fileCacheService: FileCacheService;

  constructor(
    private readonly workspaceRoot: string,
    context: vscode.ExtensionContext,
    workspaceId?: string
  ) {
    const adminClient = new GrpcAdminClient(context);
    this.fileCacheService = FileCacheService.getInstance(adminClient);
    if (workspaceId) {
      this.fileCacheService.setWorkspaceId(workspaceId);
    }
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: FolderTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: FolderTreeItem): Promise<FolderTreeItem[]> {
    try {
      if (!element) {
        return await this.getRootFolders();
      } else if (element.folderPath) {
        return await this.getItemsInFolder(element.folderPath);
      } else {
        return [];
      }
    } catch (error) {
      console.error('[FolderTreeProvider] Error in getChildren:', error);
      const errorItem = new FolderTreeItem(
        `Error: ${error instanceof Error ? error.message : String(error)}`,
        vscode.TreeItemCollapsibleState.None
      );
      errorItem.iconPath = new vscode.ThemeIcon('error');
      errorItem.id = 'error:getChildren';
      return [errorItem];
    }
  }

  private async getRootFolders(): Promise<FolderTreeItem[]> {
    try {
      const filesCache = await this.fileCacheService.getFiles();
    
    const folders = this.getRootFolderPaths(filesCache);

    // Also count root-level files
    const rootFiles = filesCache.filter(
      (file: FileEntry) => {
        const relPath = file.relative_path || '';
        return !relPath.includes(path.sep);
      }
    );

    const items: FolderTreeItem[] = [];

    // Add folders
    folders.forEach((folder) => {
      const folderFiles = filesCache.filter((file: FileEntry) => {
        const relPath = file.relative_path || '';
        return relPath.startsWith(folder + path.sep);
      });
      const label = `${folder} (${folderFiles.length})`;

      items.push(
        Object.assign(new FolderTreeItem(
          label,
          vscode.TreeItemCollapsibleState.Collapsed,
          undefined,
          false,
          folder
        ), { id: `folder:${folder}` })
      );
    });

    // Add root files
    if (rootFiles.length > 0) {
      sortFilesByActivity(rootFiles as ActivityFileEntry[]);
      rootFiles.forEach((file: FileEntry) => {
        const relativePath = file.relative_path || '';
        const absolutePath = path.join(this.workspaceRoot, relativePath);
        const uri = vscode.Uri.file(absolutePath);
        const filename = file.filename || path.basename(relativePath);
        const activityTime = getFileActivityTimestamp(file as ActivityFileEntry);

        const item = new FolderTreeItem(
          filename,
          vscode.TreeItemCollapsibleState.None,
          uri,
          true
        );
        item.id = `file:folder:root:${relativePath}`;
        item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
        items.push(item);
      });
    }

    if (items.length === 0) {
      const placeholder = new FolderTreeItem(
        t('noFilesIndexed'),
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      placeholder.id = 'placeholder:empty:folders';
      return [placeholder];
    }

    return items;
    } catch (error) {
      console.error('[FolderTreeProvider] Error getting root folders:', error);
      const errorItem = new FolderTreeItem(
        `Error loading folders: ${error instanceof Error ? error.message : String(error)}`,
        vscode.TreeItemCollapsibleState.None
      );
      errorItem.iconPath = new vscode.ThemeIcon('error');
      errorItem.id = 'error:root:folders';
      return [errorItem];
    }
  }

  private async getItemsInFolder(folderPath: string): Promise<FolderTreeItem[]> {
    try {
      const filesCache = await this.fileCacheService.getFiles();
    
    const items: FolderTreeItem[] = [];
    const subfolders = new Set<string>();
    const fileEntries: FileEntry[] = [];

    // Find immediate children (files and subfolders)
    for (const file of filesCache) {
      const relativePath = file.relative_path || '';
      if (!relativePath.startsWith(folderPath + path.sep)) {
        continue;
      }

      const relativePart = relativePath.substring(
        folderPath.length + path.sep.length
      );
      const parts = relativePart.split(path.sep);

      if (parts.length === 1) {
        // Direct file in this folder - verify it's actually a file, not a folder
        // A file should have an extension or be explicitly identified as a file
        const filename = file.filename || path.basename(relativePath);
        const hasExtension = path.extname(filename).length > 0;
        
        // Check if this path represents a file (has extension) or if it's actually a folder
        // We can't have folders in the file cache, so if it's in the cache, it's a file
        // But we need to ensure we're not treating it as a folder
        fileEntries.push(file);
      } else if (parts.length > 1) {
        // File in subfolder - the first part is the subfolder name
        subfolders.add(parts[0]);
      }
    }

    sortFilesByActivity(fileEntries as ActivityFileEntry[]);

    // Add subfolders
    Array.from(subfolders)
      .sort((a, b) => a.localeCompare(b))
      .forEach((subfolder) => {
        const fullPath = path.join(folderPath, subfolder);
        const subfolderFiles = filesCache.filter((file: FileEntry) => {
          const relPath = file.relative_path || '';
          return relPath.startsWith(fullPath + path.sep);
        });
        const label = `${subfolder} (${subfolderFiles.length})`;

        items.unshift(
          Object.assign(new FolderTreeItem(
            label,
            vscode.TreeItemCollapsibleState.Collapsed,
            undefined,
            false, // isFile = false for folders
            fullPath
          ), { id: `folder:${fullPath}` })
        );
      });

    // Add files - ensure they are marked as files with isFile=true
    const fileItems = fileEntries.map((file) => {
      const relativePath = file.relative_path || '';
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = file.filename || path.basename(relativePath);
      const activityTime = getFileActivityTimestamp(file as ActivityFileEntry);

      // Explicitly create as file with isFile=true and resourceUri
      const item = new FolderTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None, // Files are not collapsible
        uri, // resourceUri is required for files
        true // isFile = true for files
      );
      item.id = `file:folder:${folderPath}:${relativePath}`;
      item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
      return item;
    });

    return items.concat(fileItems);
    } catch (error) {
      console.error(`[FolderTreeProvider] Error getting items in folder ${folderPath}:`, error);
      const errorItem = new FolderTreeItem(
        `Error loading folder: ${error instanceof Error ? error.message : String(error)}`,
        vscode.TreeItemCollapsibleState.None
      );
      errorItem.iconPath = new vscode.ThemeIcon('error');
      errorItem.id = `error:folder:${folderPath}`;
      return [errorItem];
    }
  }

  private getRootFolderPaths(filesCache: FileEntry[]): string[] {
    const folders = new Set<string>();

    for (const file of filesCache) {
      const relativePath = file.relative_path || '';
      const parts = relativePath.split(path.sep);
      if (parts.length > 1) {
        folders.add(parts[0]);
      }
    }

    return Array.from(folders).sort((a, b) => a.localeCompare(b));
  }
}
