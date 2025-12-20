/**
 * DocumentMetricsTreeProvider - Groups documents by metadata and metrics
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IndexStore } from '../core/IndexStore';
import { MetadataExtractor, EnhancedMetadata } from '../extractors/MetadataExtractor';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';
import {
  DocumentMetadata,
  SpreadsheetMetadata,
  PresentationMetadata,
  PDFMetadata,
  DesignMetadata,
} from '../extractors/DocumentDetector';

class DocumentMetricsTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly metricCategory?: string
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
      this.iconPath = new vscode.ThemeIcon('folder');
      this.contextValue = 'cortex-document-metric';
    }
  }
}

export class DocumentMetricsTreeProvider
  implements vscode.TreeDataProvider<DocumentMetricsTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    DocumentMetricsTreeItem | undefined | null | void
  > = new vscode.EventEmitter<DocumentMetricsTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    DocumentMetricsTreeItem | undefined | null | void
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

  handleDidExpand(element: DocumentMetricsTreeItem): void {
    this.accordionState.handleDidExpand(element.metricCategory);
  }

  handleDidCollapse(element: DocumentMetricsTreeItem): void {
    this.accordionState.handleDidCollapse(element.metricCategory);
  }

  getTreeItem(element: DocumentMetricsTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(
    element?: DocumentMetricsTreeItem
  ): Promise<DocumentMetricsTreeItem[]> {
    if (!element) {
      return this.getMetricCategories();
    } else if (element.metricCategory) {
      return this.getFilesInCategory(element.metricCategory);
    } else {
      return [];
    }
  }

  private getMetricCategories(): DocumentMetricsTreeItem[] {
    const files = this.indexStore.getAllFiles();

    // Filter files with document or design metadata
    const docs = files.filter(f => f.enhanced?.documentMetadata || f.enhanced?.designMetadata);

    console.log(`[DocumentMetrics] === TREE VIEW DEBUG ===`);
    console.log(`[DocumentMetrics] Total files: ${files.length}`);
    console.log(`[DocumentMetrics] Files with doc metadata: ${docs.length}`);

    if (docs.length === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      const placeholder = new DocumentMetricsTreeItem(
        'No documents with metadata',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return [placeholder];
    }

    // Count by file type
    const wordDocs = docs.filter(f => ['.docx', '.doc'].includes(f.extension.toLowerCase()));
    const excelDocs = docs.filter(f => ['.xlsx', '.xls'].includes(f.extension.toLowerCase()));
    const pptDocs = docs.filter(f => ['.pptx', '.ppt'].includes(f.extension.toLowerCase()));
    const pdfDocs = docs.filter(f => f.extension.toLowerCase() === '.pdf');
    const designFiles = docs.filter(f => f.extension.toLowerCase() === '.psd');

    const categories: DocumentMetricsTreeItem[] = [];

    // Add category for each file type
    if (wordDocs.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `Word Documents (${wordDocs.length})`,
          this.getCategoryState('word'),
          undefined,
          false,
          'word'
        )
      );
    }

    if (excelDocs.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `Excel Spreadsheets (${excelDocs.length})`,
          this.getCategoryState('excel'),
          undefined,
          false,
          'excel'
        )
      );
    }

    if (pptDocs.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `PowerPoint Presentations (${pptDocs.length})`,
          this.getCategoryState('powerpoint'),
          undefined,
          false,
          'powerpoint'
        )
      );
    }

    if (pdfDocs.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `PDF Documents (${pdfDocs.length})`,
          this.getCategoryState('pdf'),
          undefined,
          false,
          'pdf'
        )
      );
    }

    if (designFiles.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `Photoshop Files (${designFiles.length})`,
          this.getCategoryState('photoshop'),
          undefined,
          false,
          'photoshop'
        )
      );
    }

    // Add "By Author" category if we have author info
    const docsWithAuthor = docs.filter(f => f.enhanced?.documentMetadata?.author);
    if (docsWithAuthor.length > 0) {
      categories.push(
        new DocumentMetricsTreeItem(
          `By Author (${docsWithAuthor.length})`,
          this.getCategoryState('by-author'),
          undefined,
          false,
          'by-author'
        )
      );
    }

    const groupers = this.getGroupDefinitions();
    groupers.forEach((grouper) => {
      const groups = this.buildGroups(docs, grouper);
      if (groups.size === 0) {
        return;
      }

      const filesWithGroup = Array.from(groups.values()).reduce(
        (acc, group) => acc + group.length,
        0
      );

      categories.push(
        new DocumentMetricsTreeItem(
          `${grouper.label} (${filesWithGroup})`,
          vscode.TreeItemCollapsibleState.Collapsed,
          undefined,
          false,
          `group:${grouper.id}`
        )
      );
    });

    return categories;
  }

  private getFilesInCategory(category: string): DocumentMetricsTreeItem[] {
    const files = this.indexStore.getAllFiles();
    let filtered = files.filter(f => f.enhanced?.documentMetadata || f.enhanced?.designMetadata);

    if (category.startsWith('group:')) {
      const remainder = category.substring('group:'.length);
      const [groupId, ...groupParts] = remainder.split(':');
      const groupKey = groupParts.join(':');
      const grouper = this.getGroupDefinitions().find((g) => g.id === groupId);

      if (!grouper) {
        return [];
      }

      if (!groupKey) {
        const groups = this.buildGroups(filtered, grouper);
        return Array.from(groups.entries()).map(([group, groupFiles]) =>
          new DocumentMetricsTreeItem(
            `${group} (${groupFiles.length})`,
            vscode.TreeItemCollapsibleState.Collapsed,
            undefined,
            false,
            `group:${groupId}:${group}`
          )
        );
      }

      const groups = this.buildGroups(filtered, grouper);
      filtered = groups.get(groupKey) ?? [];
    }

    switch (category) {
      case 'word':
        filtered = filtered.filter(f => ['.docx', '.doc'].includes(f.extension.toLowerCase()));
        break;

      case 'excel':
        filtered = filtered.filter(f => ['.xlsx', '.xls'].includes(f.extension.toLowerCase()));
        break;

      case 'powerpoint':
        filtered = filtered.filter(f => ['.pptx', '.ppt'].includes(f.extension.toLowerCase()));
        break;

      case 'pdf':
        filtered = filtered.filter(f => f.extension.toLowerCase() === '.pdf');
        break;

      case 'photoshop':
        filtered = filtered.filter(f => f.extension.toLowerCase() === '.psd');
        break;

      case 'by-author':
        // Group by author
        const authorGroups = new Map<string, typeof filtered>();
        filtered.forEach(file => {
          const author = file.enhanced?.documentMetadata?.author;
          if (author) {
            if (!authorGroups.has(author)) {
              authorGroups.set(author, []);
            }
            authorGroups.get(author)!.push(file);
          }
        });

        // Return author items
        return Array.from(authorGroups.entries()).map(([author, files]) =>
          new DocumentMetricsTreeItem(
            `${author} (${files.length} files)`,
            vscode.TreeItemCollapsibleState.Collapsed,
            undefined,
            false,
            `author:${author}`
          )
        );

      default:
        break;
    }

    if (category.startsWith('author:')) {
      const author = category.substring('author:'.length);
      filtered = filtered.filter(
        (file) => file.enhanced?.documentMetadata?.author === author
      );
    }

    return filtered.map((file) => {
      const absolutePath = path.join(this.workspaceRoot, file.relativePath);
      const uri = vscode.Uri.file(absolutePath);

      const item = new DocumentMetricsTreeItem(
        file.filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );

      // Build tooltip based on file type
      item.tooltip = this.buildTooltip(file);
      item.description = this.buildDescription(file);

      return item;
    });
  }

  private getIndexingPlaceholder(): DocumentMetricsTreeItem {
    if (!this.indexingStatus?.isIndexing) {
      const placeholder = new DocumentMetricsTreeItem(
        'No documents with metadata',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('info');
      return placeholder;
    }

    const placeholder = new DocumentMetricsTreeItem(
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
    const docs = files.filter(
      (f) => f.enhanced?.documentMetadata || f.enhanced?.designMetadata
    );

    if (docs.length === 0) {
      return [];
    }

    const categories: string[] = [];
    if (docs.some((f) => ['.docx', '.doc'].includes(f.extension.toLowerCase()))) {
      categories.push('word');
    }
    if (docs.some((f) => ['.xlsx', '.xls'].includes(f.extension.toLowerCase()))) {
      categories.push('excel');
    }
    if (docs.some((f) => ['.pptx', '.ppt'].includes(f.extension.toLowerCase()))) {
      categories.push('powerpoint');
    }
    if (docs.some((f) => f.extension.toLowerCase() === '.pdf')) {
      categories.push('pdf');
    }
    if (docs.some((f) => f.extension.toLowerCase() === '.psd')) {
      categories.push('photoshop');
    }
    if (docs.some((f) => f.enhanced?.documentMetadata?.author)) {
      categories.push('by-author');
    }

    return categories;
  }

  private buildTooltip(file: any): string {
    const docMeta = file.enhanced?.documentMetadata;
    const designMeta = file.enhanced?.designMetadata;
    const ext = file.extension.toLowerCase();

    let tooltip = `${file.relativePath}\n`;

    if (ext === '.docx' || ext === '.doc') {
      // Word document
      if (docMeta) {
        if (docMeta.title) tooltip += `Title: ${docMeta.title}\n`;
        if (docMeta.author) tooltip += `Author: ${docMeta.author}\n`;
        if (docMeta.pageCount) tooltip += `Pages: ${docMeta.pageCount}\n`;
        if (docMeta.wordCount) tooltip += `Words: ${docMeta.wordCount}\n`;
        if (docMeta.keywords) tooltip += `Keywords: ${docMeta.keywords.join(', ')}\n`;
      }
    } else if (ext === '.xlsx' || ext === '.xls') {
      // Excel spreadsheet
      const excelMeta = docMeta as SpreadsheetMetadata;
      if (excelMeta) {
        if (excelMeta.author) tooltip += `Author: ${excelMeta.author}\n`;
        if (excelMeta.sheetCount) tooltip += `Sheets: ${excelMeta.sheetCount}\n`;
        tooltip += `Has Formulas: ${excelMeta.hasFormulas ? 'Yes' : 'No'}\n`;
        tooltip += `Has Macros: ${excelMeta.hasMacros ? 'Yes' : 'No'}\n`;
      }
    } else if (ext === '.pptx' || ext === '.ppt') {
      // PowerPoint presentation
      const pptMeta = docMeta as PresentationMetadata;
      if (pptMeta) {
        if (pptMeta.author) tooltip += `Author: ${pptMeta.author}\n`;
        if (pptMeta.slideCount) tooltip += `Slides: ${pptMeta.slideCount}\n`;
        tooltip += `Animations: ${pptMeta.hasAnimations ? 'Yes' : 'No'}\n`;
        tooltip += `Embedded Media: ${pptMeta.hasEmbeddedMedia ? 'Yes' : 'No'}\n`;
      }
    } else if (ext === '.pdf') {
      // PDF document
      const pdfMeta = docMeta as PDFMetadata;
      if (pdfMeta) {
        if (pdfMeta.title) tooltip += `Title: ${pdfMeta.title}\n`;
        if (pdfMeta.author) tooltip += `Author: ${pdfMeta.author}\n`;
        if (pdfMeta.pageCount) tooltip += `Pages: ${pdfMeta.pageCount}\n`;
        if (pdfMeta.pdfVersion) tooltip += `PDF Version: ${pdfMeta.pdfVersion}\n`;
        tooltip += `Encrypted: ${pdfMeta.isEncrypted ? 'Yes' : 'No'}\n`;
      }
    } else if (ext === '.psd') {
      // Photoshop file
      if (designMeta) {
        tooltip += `Dimensions: ${designMeta.width}×${designMeta.height}\n`;
        if (designMeta.colorMode) tooltip += `Color Mode: ${designMeta.colorMode}\n`;
        if (designMeta.bitDepth) tooltip += `Bit Depth: ${designMeta.bitDepth}-bit\n`;
        tooltip += `Transparency: ${designMeta.hasTransparency ? 'Yes' : 'No'}\n`;
      }
    }

    return tooltip;
  }

  private buildDescription(file: any): string {
    const docMeta = file.enhanced?.documentMetadata;
    const designMeta = file.enhanced?.designMetadata;
    const ext = file.extension.toLowerCase();

    if (ext === '.docx' || ext === '.doc') {
      return docMeta?.pageCount ? `${docMeta.pageCount} pages` : '';
    } else if (ext === '.xlsx' || ext === '.xls') {
      const excelMeta = docMeta as SpreadsheetMetadata;
      return excelMeta?.sheetCount ? `${excelMeta.sheetCount} sheets` : '';
    } else if (ext === '.pptx' || ext === '.ppt') {
      const pptMeta = docMeta as PresentationMetadata;
      return pptMeta?.slideCount ? `${pptMeta.slideCount} slides` : '';
    } else if (ext === '.pdf') {
      const pdfMeta = docMeta as PDFMetadata;
      return pdfMeta?.pageCount ? `${pdfMeta.pageCount} pages` : '';
    } else if (ext === '.psd') {
      return designMeta?.width ? `${designMeta.width}×${designMeta.height}` : '';
    }

    return '';
  }

  private getGroupDefinitions(): Array<{
    id: string;
    label: string;
    groupFn: (file: any) => string | string[] | undefined;
  }> {
    return [
      {
        id: 'folder',
        label: 'By Folder',
        groupFn: (file) => file.enhanced?.folder ?? path.dirname(file.relativePath),
      },
      {
        id: 'top-folder',
        label: 'By Top Folder',
        groupFn: (file) => {
          const folder = file.enhanced?.folder ?? path.dirname(file.relativePath);
          if (!folder || folder === '.') {
            return 'Root';
          }
          return folder.split(path.sep)[0] || 'Root';
        },
      },
      {
        id: 'depth',
        label: 'By Folder Depth',
        groupFn: (file) => {
          const depth = file.enhanced?.depth;
          return typeof depth === 'number' ? `Depth ${depth}` : undefined;
        },
      },
      {
        id: 'size',
        label: 'By File Size',
        groupFn: (file) => this.bucketBySize(file.fileSize),
      },
      {
        id: 'modified',
        label: 'By Last Modified',
        groupFn: (file) => this.bucketByAge(file.enhanced?.stats?.modified),
      },
      {
        id: 'created',
        label: 'By Created',
        groupFn: (file) => this.bucketByAge(file.enhanced?.stats?.created),
      },
      {
        id: 'accessed',
        label: 'By Last Accessed',
        groupFn: (file) => this.bucketByAge(file.enhanced?.stats?.accessed),
      },
      {
        id: 'hidden',
        label: 'By Hidden Files',
        groupFn: (file) => this.bucketByYesNo(file.enhanced?.stats?.isHidden),
      },
      {
        id: 'read-only',
        label: 'By Read-Only',
        groupFn: (file) => this.bucketByYesNo(file.enhanced?.stats?.isReadOnly),
      },
      {
        id: 'git-author',
        label: 'By Git Last Author',
        groupFn: (file) => file.enhanced?.git?.lastAuthor,
      },
      {
        id: 'git-branch',
        label: 'By Git Branch',
        groupFn: (file) => file.enhanced?.git?.branch,
      },
      {
        id: 'git-commit-age',
        label: 'By Git Commit Age',
        groupFn: (file) => this.bucketByAge(file.enhanced?.git?.lastCommitDate),
      },
      {
        id: 'author',
        label: 'By Document Author',
        groupFn: (file) =>
          file.enhanced?.documentMetadata?.author ?? file.enhanced?.designMetadata?.author,
      },
      {
        id: 'creator',
        label: 'By Document Creator',
        groupFn: (file) => file.enhanced?.documentMetadata?.creator,
      },
      {
        id: 'title',
        label: 'By Title Initial',
        groupFn: (file) => this.bucketByFirstLetter(file.enhanced?.documentMetadata?.title),
      },
      {
        id: 'subject',
        label: 'By Subject Initial',
        groupFn: (file) => this.bucketByFirstLetter(file.enhanced?.documentMetadata?.subject),
      },
      {
        id: 'keywords',
        label: 'By Keywords',
        groupFn: (file) => file.enhanced?.documentMetadata?.keywords,
      },
      {
        id: 'language',
        label: 'By Language',
        groupFn: (file) =>
          file.enhanced?.documentMetadata?.language ?? file.enhanced?.language,
      },
      {
        id: 'template',
        label: 'By Template',
        groupFn: (file) => file.enhanced?.documentMetadata?.template,
      },
      {
        id: 'revision',
        label: 'By Revision',
        groupFn: (file) => file.enhanced?.documentMetadata?.revision,
      },
      {
        id: 'description',
        label: 'By Description Presence',
        groupFn: (file) =>
          this.bucketByPresence(file.enhanced?.documentMetadata?.description),
      },
      {
        id: 'encrypted',
        label: 'By Encryption',
        groupFn: (file) => this.bucketByYesNo(file.enhanced?.documentMetadata?.isEncrypted),
      },
      {
        id: 'password',
        label: 'By Password Protected',
        groupFn: (file) => this.bucketByYesNo(file.enhanced?.documentMetadata?.hasPassword),
      },
      {
        id: 'word-pages',
        label: 'By Word Page Count',
        groupFn: (file) =>
          this.isWord(file)
            ? this.bucketByCount(file.enhanced?.documentMetadata?.pageCount, this.pageBuckets, '250+ pages')
            : undefined,
      },
      {
        id: 'word-words',
        label: 'By Word Count',
        groupFn: (file) =>
          this.isWord(file)
            ? this.bucketByCount(file.enhanced?.documentMetadata?.wordCount, this.wordBuckets, '20000+ words')
            : undefined,
      },
      {
        id: 'word-characters',
        label: 'By Character Count',
        groupFn: (file) =>
          this.isWord(file)
            ? this.bucketByCount(
                file.enhanced?.documentMetadata?.characterCount,
                this.characterBuckets,
                '100000+ chars'
              )
            : undefined,
      },
      {
        id: 'sheet-count',
        label: 'By Sheet Count',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByCount(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.sheetCount,
                this.sheetBuckets,
                '50+ sheets'
              )
            : undefined,
      },
      {
        id: 'sheet-rows',
        label: 'By Total Rows',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByCount(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.totalRows,
                this.rowBuckets,
                '100000+ rows'
              )
            : undefined,
      },
      {
        id: 'sheet-columns',
        label: 'By Total Columns',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByCount(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.totalColumns,
                this.columnBuckets,
                '500+ columns'
              )
            : undefined,
      },
      {
        id: 'sheet-formulas',
        label: 'By Has Formulas',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.hasFormulas
              )
            : undefined,
      },
      {
        id: 'sheet-macros',
        label: 'By Has Macros',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.hasMacros
              )
            : undefined,
      },
      {
        id: 'sheet-charts',
        label: 'By Has Charts',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.hasCharts
              )
            : undefined,
      },
      {
        id: 'sheet-pivot',
        label: 'By Has Pivot Tables',
        groupFn: (file) =>
          this.isSpreadsheet(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as SpreadsheetMetadata | undefined)?.hasPivotTables
              )
            : undefined,
      },
      {
        id: 'slide-count',
        label: 'By Slide Count',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByCount(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.slideCount,
                this.slideBuckets,
                '200+ slides'
              )
            : undefined,
      },
      {
        id: 'has-animations',
        label: 'By Has Animations',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.hasAnimations
              )
            : undefined,
      },
      {
        id: 'has-transitions',
        label: 'By Has Transitions',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.hasTransitions
              )
            : undefined,
      },
      {
        id: 'has-embedded-media',
        label: 'By Has Embedded Media',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.hasEmbeddedMedia
              )
            : undefined,
      },
      {
        id: 'has-notes',
        label: 'By Has Notes',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.hasNotes
              )
            : undefined,
      },
      {
        id: 'master-slide-count',
        label: 'By Master Slide Count',
        groupFn: (file) =>
          this.isPresentation(file)
            ? this.bucketByCount(
                (file.enhanced?.documentMetadata as PresentationMetadata | undefined)?.masterSlideCount,
                this.masterSlideBuckets,
                '50+ masters'
              )
            : undefined,
      },
      {
        id: 'pdf-version',
        label: 'By PDF Version',
        groupFn: (file) =>
          this.isPdf(file)
            ? (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.pdfVersion
            : undefined,
      },
      {
        id: 'pdf-linearized',
        label: 'By PDF Linearized',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.isLinearized
              )
            : undefined,
      },
      {
        id: 'pdf-javascript',
        label: 'By PDF JavaScript',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.hasJavaScript
              )
            : undefined,
      },
      {
        id: 'pdf-attachments',
        label: 'By PDF Attachments',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.hasAttachments
              )
            : undefined,
      },
      {
        id: 'pdf-bookmarks',
        label: 'By PDF Bookmarks',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.hasBookmarks
              )
            : undefined,
      },
      {
        id: 'pdf-comments',
        label: 'By PDF Comments',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.hasComments
              )
            : undefined,
      },
      {
        id: 'pdf-print',
        label: 'By PDF Print Allowed',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.printAllowed
              )
            : undefined,
      },
      {
        id: 'pdf-copy',
        label: 'By PDF Copy Allowed',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.copyAllowed
              )
            : undefined,
      },
      {
        id: 'pdf-modify',
        label: 'By PDF Modify Allowed',
        groupFn: (file) =>
          this.isPdf(file)
            ? this.bucketByYesNo(
                (file.enhanced?.documentMetadata as PDFMetadata | undefined)?.modifyAllowed
              )
            : undefined,
      },
      {
        id: 'design-dimensions',
        label: 'By Design Dimensions',
        groupFn: (file) => {
          if (!this.isDesign(file)) {
            return undefined;
          }
          const width = file.enhanced?.designMetadata?.width;
          const height = file.enhanced?.designMetadata?.height;
          if (!width || !height) {
            return undefined;
          }
          const megapixels = (width * height) / 1000000;
          return this.bucketByCount(
            megapixels,
            this.megapixelBuckets,
            '20+ MP'
          );
        },
      },
      {
        id: 'design-resolution',
        label: 'By Design Resolution',
        groupFn: (file) =>
          this.isDesign(file)
            ? this.bucketByCount(
                file.enhanced?.designMetadata?.resolution,
                this.resolutionBuckets,
                '600+ DPI'
              )
            : undefined,
      },
      {
        id: 'design-color-mode',
        label: 'By Design Color Mode',
        groupFn: (file) =>
          this.isDesign(file) ? file.enhanced?.designMetadata?.colorMode : undefined,
      },
      {
        id: 'design-bit-depth',
        label: 'By Design Bit Depth',
        groupFn: (file) =>
          this.isDesign(file)
            ? this.bucketByCount(
                file.enhanced?.designMetadata?.bitDepth,
                this.bitDepthBuckets,
                '64+ bit'
              )
            : undefined,
      },
      {
        id: 'design-layer-count',
        label: 'By Design Layer Count',
        groupFn: (file) =>
          this.isDesign(file)
            ? this.bucketByCount(
                file.enhanced?.designMetadata?.layerCount,
                this.layerBuckets,
                '500+ layers'
              )
            : undefined,
      },
      {
        id: 'design-transparency',
        label: 'By Design Transparency',
        groupFn: (file) =>
          this.isDesign(file)
            ? this.bucketByYesNo(file.enhanced?.designMetadata?.hasTransparency)
            : undefined,
      },
      {
        id: 'design-software',
        label: 'By Design Software',
        groupFn: (file) =>
          this.isDesign(file) ? file.enhanced?.designMetadata?.software : undefined,
      },
      {
        id: 'design-software-version',
        label: 'By Design Software Version',
        groupFn: (file) =>
          this.isDesign(file) ? file.enhanced?.designMetadata?.softwareVersion : undefined,
      },
      {
        id: 'design-author',
        label: 'By Design Author',
        groupFn: (file) =>
          this.isDesign(file) ? file.enhanced?.designMetadata?.author : undefined,
      },
      {
        id: 'design-flattened',
        label: 'By Design Flattened',
        groupFn: (file) =>
          this.isDesign(file)
            ? this.bucketByYesNo(file.enhanced?.designMetadata?.isFlattened)
            : undefined,
      },
      {
        id: 'design-compression',
        label: 'By Design Compression',
        groupFn: (file) =>
          this.isDesign(file) ? file.enhanced?.designMetadata?.compressionMethod : undefined,
      },
    ];
  }

  private buildGroups(
    files: any[],
    grouper: {
      groupFn: (file: any) => string | string[] | undefined;
    }
  ): Map<string, any[]> {
    const groups = new Map<string, any[]>();

    files.forEach((file) => {
      const value = grouper.groupFn(file);
      if (!value) {
        return;
      }
      const values = Array.isArray(value) ? value : [value];
      values.forEach((entry) => {
        const label = String(entry).trim();
        if (!label) {
          return;
        }
        if (!groups.has(label)) {
          groups.set(label, []);
        }
        groups.get(label)!.push(file);
      });
    });

    return groups;
  }

  private bucketByFirstLetter(value?: string): string | undefined {
    if (!value) {
      return undefined;
    }
    const trimmed = value.trim();
    if (!trimmed) {
      return undefined;
    }
    const first = trimmed[0].toUpperCase();
    if (first >= 'A' && first <= 'Z') {
      return first;
    }
    if (first >= '0' && first <= '9') {
      return '0-9';
    }
    return '#';
  }

  private bucketByPresence(value?: string): string | undefined {
    if (value === undefined) {
      return undefined;
    }
    return value.trim() ? 'Present' : 'Missing';
  }

  private bucketByYesNo(value?: boolean): string | undefined {
    if (value === undefined) {
      return undefined;
    }
    return value ? 'Yes' : 'No';
  }

  private bucketByAge(timestamp?: number): string | undefined {
    if (!timestamp || Number.isNaN(timestamp)) {
      return undefined;
    }
    const now = Date.now();
    const ageMs = now - timestamp;
    const dayMs = 24 * 60 * 60 * 1000;
    const ageDays = ageMs / dayMs;

    if (ageDays < 1) return 'Last 24 hours';
    if (ageDays < 7) return '1-7 days';
    if (ageDays < 30) return '7-30 days';
    if (ageDays < 90) return '30-90 days';
    if (ageDays < 365) return '90-365 days';
    return '1+ years';
  }

  private bucketBySize(size?: number): string | undefined {
    if (size === undefined || Number.isNaN(size)) {
      return undefined;
    }
    const kb = 1024;
    const mb = 1024 * 1024;
    const gb = 1024 * 1024 * 1024;

    if (size < 100 * kb) return '<100 KB';
    if (size < mb) return '100 KB - 1 MB';
    if (size < 10 * mb) return '1 - 10 MB';
    if (size < 100 * mb) return '10 - 100 MB';
    if (size < gb) return '100 MB - 1 GB';
    return '1+ GB';
  }

  private bucketByCount(
    value: number | undefined,
    buckets: Array<{ max: number; label: string }>,
    overflowLabel: string
  ): string | undefined {
    if (value === undefined || Number.isNaN(value)) {
      return undefined;
    }
    for (const bucket of buckets) {
      if (value <= bucket.max) {
        return bucket.label;
      }
    }
    return overflowLabel;
  }

  private isWord(file: any): boolean {
    const ext = file.extension.toLowerCase();
    return ext === '.docx' || ext === '.doc';
  }

  private isSpreadsheet(file: any): boolean {
    const ext = file.extension.toLowerCase();
    return ext === '.xlsx' || ext === '.xls';
  }

  private isPresentation(file: any): boolean {
    const ext = file.extension.toLowerCase();
    return ext === '.pptx' || ext === '.ppt';
  }

  private isPdf(file: any): boolean {
    return file.extension.toLowerCase() === '.pdf';
  }

  private isDesign(file: any): boolean {
    return file.extension.toLowerCase() === '.psd';
  }

  private pageBuckets = [
    { max: 1, label: '1 page' },
    { max: 5, label: '2-5 pages' },
    { max: 10, label: '6-10 pages' },
    { max: 25, label: '11-25 pages' },
    { max: 50, label: '26-50 pages' },
    { max: 100, label: '51-100 pages' },
    { max: 250, label: '101-250 pages' },
  ];

  private wordBuckets = [
    { max: 100, label: '<100 words' },
    { max: 500, label: '100-500 words' },
    { max: 2000, label: '500-2k words' },
    { max: 5000, label: '2k-5k words' },
    { max: 10000, label: '5k-10k words' },
    { max: 20000, label: '10k-20k words' },
  ];

  private characterBuckets = [
    { max: 500, label: '<500 chars' },
    { max: 2000, label: '500-2k chars' },
    { max: 10000, label: '2k-10k chars' },
    { max: 25000, label: '10k-25k chars' },
    { max: 50000, label: '25k-50k chars' },
    { max: 100000, label: '50k-100k chars' },
  ];

  private sheetBuckets = [
    { max: 1, label: '1 sheet' },
    { max: 5, label: '2-5 sheets' },
    { max: 10, label: '6-10 sheets' },
    { max: 25, label: '11-25 sheets' },
    { max: 50, label: '26-50 sheets' },
  ];

  private rowBuckets = [
    { max: 100, label: '<100 rows' },
    { max: 1000, label: '100-1k rows' },
    { max: 10000, label: '1k-10k rows' },
    { max: 50000, label: '10k-50k rows' },
    { max: 100000, label: '50k-100k rows' },
  ];

  private columnBuckets = [
    { max: 10, label: '<10 columns' },
    { max: 50, label: '10-50 columns' },
    { max: 100, label: '50-100 columns' },
    { max: 250, label: '100-250 columns' },
    { max: 500, label: '250-500 columns' },
  ];

  private slideBuckets = [
    { max: 5, label: '1-5 slides' },
    { max: 10, label: '6-10 slides' },
    { max: 25, label: '11-25 slides' },
    { max: 50, label: '26-50 slides' },
    { max: 100, label: '51-100 slides' },
    { max: 200, label: '101-200 slides' },
  ];

  private masterSlideBuckets = [
    { max: 1, label: '1 master' },
    { max: 5, label: '2-5 masters' },
    { max: 10, label: '6-10 masters' },
    { max: 25, label: '11-25 masters' },
    { max: 50, label: '26-50 masters' },
  ];

  private megapixelBuckets = [
    { max: 1, label: '<1 MP' },
    { max: 5, label: '1-5 MP' },
    { max: 10, label: '5-10 MP' },
    { max: 20, label: '10-20 MP' },
  ];

  private resolutionBuckets = [
    { max: 72, label: '<=72 DPI' },
    { max: 150, label: '72-150 DPI' },
    { max: 300, label: '150-300 DPI' },
    { max: 600, label: '300-600 DPI' },
  ];

  private bitDepthBuckets = [
    { max: 8, label: '8-bit' },
    { max: 16, label: '16-bit' },
    { max: 32, label: '32-bit' },
    { max: 64, label: '64-bit' },
  ];

  private layerBuckets = [
    { max: 10, label: '<10 layers' },
    { max: 50, label: '10-50 layers' },
    { max: 100, label: '50-100 layers' },
    { max: 250, label: '100-250 layers' },
    { max: 500, label: '250-500 layers' },
  ];
}
