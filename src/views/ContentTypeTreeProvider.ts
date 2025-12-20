/**
 * ContentTypeTreeProvider - Groups files by actual MIME type
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { MetadataExtractor } from '../extractors/MetadataExtractor';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';

class ContentTypeTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly contentType?: string
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
      // Icon based on category
      const icons: Record<string, string> = {
        text: 'file-text',
        code: 'file-code',
        image: 'file-media',
        video: 'device-camera-video',
        audio: 'music',
        archive: 'file-zip',
        document: 'file-pdf',
        binary: 'file-binary',
      };
      const icon = icons[contentType || ''] || 'file';
      this.iconPath = new vscode.ThemeIcon(icon);
      this.contextValue = 'cortex-contenttype';
    }
  }
}

export class ContentTypeTreeProvider
  implements vscode.TreeDataProvider<ContentTypeTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    ContentTypeTreeItem | undefined | null | void
  > = new vscode.EventEmitter<ContentTypeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    ContentTypeTreeItem | undefined | null | void
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

  handleDidExpand(element: ContentTypeTreeItem): void {
    this.accordionState.handleDidExpand(element.contentType);
  }

  handleDidCollapse(element: ContentTypeTreeItem): void {
    this.accordionState.handleDidCollapse(element.contentType);
  }

  getTreeItem(element: ContentTypeTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(
    element?: ContentTypeTreeItem
  ): Promise<ContentTypeTreeItem[]> {
    if (!element) {
      return this.getCategories();
    } else if (element.contentType) {
      return this.getFilesInCategory(element.contentType);
    } else {
      return [];
    }
  }

  private getCategories(): ContentTypeTreeItem[] {
    const sortedCategories = this.getCategoryEntries().map(
      ({ category, count }) => {
        const label = `${this.formatCategory(category)} (${count})`;
        const isExpanded = this.accordionState.isExpanded(category);
        const collapsibleState = isExpanded
          ? vscode.TreeItemCollapsibleState.Expanded
          : vscode.TreeItemCollapsibleState.Collapsed;
        return new ContentTypeTreeItem(
          label,
          collapsibleState,
          undefined,
          false,
          category
        );
      }
    );

    if (sortedCategories.length === 0) {
      return [this.getIndexingPlaceholder()];
    }

    return sortedCategories;
  }

  private getFilesInCategory(category: string): ContentTypeTreeItem[] {
    const files = this.indexStore.getAllFiles();

    const matchingFiles = files.filter(
      (file) => (file.enhanced?.mimeType?.category || 'unknown') === category
    );

    return matchingFiles.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);

      const item = new ContentTypeTreeItem(
        file.filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      // Build tooltip and description with document metadata if available
      const { tooltip, description } = this.buildFileInfo(file);
      item.tooltip = tooltip;
      item.description = description;

      return item;
    });
  }

  private getIndexingPlaceholder(): ContentTypeTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new ContentTypeTreeItem(
        'No files indexed',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new ContentTypeTreeItem(
      formatIndexingMessage(this.indexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private buildFileInfo(file: any): { tooltip: string; description: string } {
    const mimeType = file.enhanced?.mimeType?.mimeType || 'unknown';
    const encoding = file.enhanced?.mimeType?.encoding;
    const ext = file.extension.toLowerCase();

    let tooltip = `${file.relativePath}\nMIME: ${mimeType}${
      encoding ? `\nEncoding: ${encoding}` : ''
    }`;
    let description = mimeType;

    // Enhance with document metadata
    const docMeta = file.enhanced?.documentMetadata;
    const designMeta = file.enhanced?.designMetadata;

    if (docMeta || designMeta) {
      if (ext === '.docx' || ext === '.doc') {
        // Word document
        if (docMeta) {
          if (docMeta.pageCount) {
            tooltip += `\nPages: ${docMeta.pageCount}`;
            description = `${docMeta.pageCount} pages`;
          }
          if (docMeta.wordCount) tooltip += `\nWords: ${docMeta.wordCount}`;
          if (docMeta.author) tooltip += `\nAuthor: ${docMeta.author}`;
        }
      } else if (ext === '.xlsx' || ext === '.xls') {
        // Excel spreadsheet
        if (docMeta && 'sheetCount' in docMeta) {
          if (docMeta.sheetCount) {
            tooltip += `\nSheets: ${docMeta.sheetCount}`;
            description = `${docMeta.sheetCount} sheets`;
          }
          if (docMeta.hasFormulas) tooltip += `\nFormulas: Yes`;
          if (docMeta.hasMacros) tooltip += `\nMacros: Yes`;
        }
      } else if (ext === '.pptx' || ext === '.ppt') {
        // PowerPoint presentation
        if (docMeta && 'slideCount' in docMeta) {
          if (docMeta.slideCount) {
            tooltip += `\nSlides: ${docMeta.slideCount}`;
            description = `${docMeta.slideCount} slides`;
          }
          if (docMeta.hasAnimations) tooltip += `\nAnimations: Yes`;
          if (docMeta.hasEmbeddedMedia) tooltip += `\nMedia: Yes`;
        }
      } else if (ext === '.pdf') {
        // PDF document
        if (docMeta && 'pdfVersion' in docMeta) {
          if (docMeta.pageCount) {
            tooltip += `\nPages: ${docMeta.pageCount}`;
            description = `${docMeta.pageCount} pages`;
          }
          if (docMeta.pdfVersion) tooltip += `\nPDF Version: ${docMeta.pdfVersion}`;
          if (docMeta.isEncrypted) tooltip += `\nEncrypted: Yes`;
        }
      } else if (ext === '.psd') {
        // Photoshop file
        if (designMeta) {
          if (designMeta.width && designMeta.height) {
            tooltip += `\nDimensions: ${designMeta.width}×${designMeta.height}`;
            description = `${designMeta.width}×${designMeta.height}`;
          }
          if (designMeta.colorMode) tooltip += `\nColor: ${designMeta.colorMode}`;
          if (designMeta.bitDepth) tooltip += `\nBit Depth: ${designMeta.bitDepth}-bit`;
        }
      }
    }

    return { tooltip, description };
  }

  private formatCategory(category: string): string {
    const formatted: Record<string, string> = {
      text: 'Text Files',
      code: 'Code Files',
      image: 'Images',
      video: 'Videos',
      audio: 'Audio',
      archive: 'Archives',
      document: 'Documents',
      binary: 'Binary Files',
      unknown: 'Unknown',
    };
    return formatted[category] || category;
  }

  private getCategoryEntries(): Array<{ category: string; count: number }> {
    const files = this.indexStore.getAllFiles();
    const categoryCounts: Record<string, number> = {};

    for (const file of files) {
      const category = file.enhanced?.mimeType?.category || 'unknown';
      categoryCounts[category] = (categoryCounts[category] || 0) + 1;
    }

    return Object.entries(categoryCounts)
      .sort(([, a], [, b]) => b - a)
      .map(([category, count]) => ({ category, count }));
  }

  private getRootKeys(): string[] {
    return this.getCategoryEntries().map((entry) => entry.category);
  }
}
