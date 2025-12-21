/**
 * CategoryTreeProvider - Provides virtual tree view grouped by document categories (library-style classification)
 * Uses LLM to classify documents based on summary and metadata
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { IndexingStatus, formatIndexingMessage } from '../core/IndexingStatus';
import { TreeAccordionState } from './TreeAccordionState';
import { LLMService } from '../services/LLMService';
import { FileIndexEntry, FileMetadata } from '../models/types';

/**
 * Tree item for category view
 */
class CategoryTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly categoryName?: string
  ) {
    super(label, collapsibleState);

    if (isFile && resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Abrir archivo',
        arguments: [resourceUri],
      };
      this.iconPath = vscode.ThemeIcon.File;
      this.contextValue = 'cortex-file';
    } else {
      this.iconPath = new vscode.ThemeIcon('library');
      this.contextValue = 'cortex-category';
    }
  }
}

/**
 * Document classification cache
 */
interface DocumentClassification {
  relativePath: string;
  category: string;
  timestamp: number;
}

/**
 * TreeDataProvider for Category view (Biblioteca)
 */
export class CategoryTreeProvider
  implements vscode.TreeDataProvider<CategoryTreeItem>
{
  private _onDidChangeTreeData: vscode.EventEmitter<
    CategoryTreeItem | undefined | null | void
  > = new vscode.EventEmitter<CategoryTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    CategoryTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;
  private accordionState: TreeAccordionState;
  private categoryCache: Map<string, DocumentClassification> = new Map();
  private isClassifying: boolean = false;
  private classificationPromise: Promise<void> | null = null;

  constructor(
    private workspaceRoot: string,
    private metadataStore: IMetadataStore,
    private indexStore: IndexStore,
    private llmService: LLMService,
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

  handleDidExpand(element: CategoryTreeItem): void {
    this.accordionState.handleDidExpand(element.categoryName);
  }

  handleDidCollapse(element: CategoryTreeItem): void {
    this.accordionState.handleDidCollapse(element.categoryName);
  }

  getTreeItem(element: CategoryTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: CategoryTreeItem): Promise<CategoryTreeItem[]> {
    if (!element) {
      // Root level - show all categories
      await this.ensureClassification();
      return this.getCategoryNodes();
    } else if (element.categoryName) {
      // Category level - show files in this category
      return this.getFilesInCategory(element.categoryName);
    } else {
      return [];
    }
  }

  /**
   * Ensure all documents are classified
   */
  private async ensureClassification(): Promise<void> {
    if (this.isClassifying && this.classificationPromise) {
      return this.classificationPromise;
    }

    if (!this.llmService.isEnabled()) {
      return;
    }

    const isAvailable = await this.llmService.isAvailable();
    if (!isAvailable) {
      return;
    }

    this.isClassifying = true;
    this.classificationPromise = this.performClassification();
    
    try {
      await this.classificationPromise;
    } finally {
      this.isClassifying = false;
      this.classificationPromise = null;
    }
  }

  /**
   * Classify all documents using LLM
   */
  private async performClassification(): Promise<void> {
    const files = this.indexStore.getAllFiles();
    const filesToClassify: Array<{
      file: FileIndexEntry;
      metadata: FileMetadata;
    }> = [];

    // Get all files with their metadata
    for (const file of files) {
      const metadata = this.metadataStore.getMetadataByPath(file.relativePath);
      if (!metadata) continue;

      // Skip if already classified recently (cache for 1 hour)
      const cached = this.categoryCache.get(file.relativePath);
      if (cached && Date.now() - cached.timestamp < 3600000) {
        continue;
      }

      // Only classify documents that have summaries or meaningful content
      if (metadata.aiSummary || metadata.tags.length > 0 || metadata.contexts.length > 0) {
        filesToClassify.push({ file, metadata });
      }
    }

    if (filesToClassify.length === 0) {
      return;
    }

    // Classify in batches to avoid overwhelming the LLM
    const batchSize = 10;
    for (let i = 0; i < filesToClassify.length; i += batchSize) {
      const batch = filesToClassify.slice(i, i + batchSize);
      await Promise.all(
        batch.map(({ file, metadata }) => this.classifyDocument(file, metadata))
      );
    }

    // Refresh the view after classification
    this.refresh();
  }

  /**
   * Classify a single document
   */
  private async classifyDocument(
    file: FileIndexEntry,
    metadata: FileMetadata
  ): Promise<void> {
    try {
      const category = await this.llmService.classifyDocumentCategory(
        file.relativePath,
        {
          summary: metadata.aiSummary,
          keyTerms: metadata.aiKeyTerms,
          tags: metadata.tags,
          contexts: metadata.contexts,
          type: metadata.type,
        }
      );

      if (category) {
        this.categoryCache.set(file.relativePath, {
          relativePath: file.relativePath,
          category,
          timestamp: Date.now(),
        });
      }
    } catch (error) {
      console.error(`Failed to classify ${file.relativePath}:`, error);
    }
  }

  /**
   * Get all category nodes
   */
  private getCategoryNodes(): CategoryTreeItem[] {
    // Group files by category
    const categoryMap = new Map<string, string[]>();
    
    const files = this.indexStore.getAllFiles();
    for (const file of files) {
      const cached = this.categoryCache.get(file.relativePath);
      if (cached) {
        const category = cached.category;
        if (!categoryMap.has(category)) {
          categoryMap.set(category, []);
        }
        categoryMap.get(category)!.push(file.relativePath);
      }
    }

    if (categoryMap.size === 0) {
      if (this.indexingStatus?.isIndexing) {
        return [this.getIndexingPlaceholder()];
      }
      
      if (!this.llmService.isEnabled()) {
        const placeholder = new CategoryTreeItem(
          'Clasificación por IA deshabilitada',
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('info');
        placeholder.tooltip = 'Habilita "cortex.llm.enabled" en la configuración para usar la clasificación por categorías';
        return [placeholder];
      }

      const placeholder = new CategoryTreeItem(
        'Clasificando documentos...',
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
      placeholder.tooltip = 'Esperando a que se complete la clasificación de documentos';
      return [placeholder];
    }

    // Sort categories alphabetically
    const sortedCategories = Array.from(categoryMap.entries()).sort((a, b) =>
      a[0].localeCompare(b[0], 'es')
    );

    return sortedCategories.map(([category, files]) => {
      const fileCount = files.length;
      const label = `${category} (${fileCount})`;
      const isExpanded = this.accordionState.isExpanded(category);
      const collapsibleState = isExpanded
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed;

      return new CategoryTreeItem(
        label,
        collapsibleState,
        undefined,
        false,
        category
      );
    });
  }

  /**
   * Get files in a specific category
   */
  private getFilesInCategory(category: string): CategoryTreeItem[] {
    const files: string[] = [];
    
    for (const [relativePath, classification] of this.categoryCache.entries()) {
      if (classification.category === category) {
        files.push(relativePath);
      }
    }

    return files.map((relativePath) => {
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);

      const item = new CategoryTreeItem(
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

  private getIndexingPlaceholder(): CategoryTreeItem {
    const placeholder = new CategoryTreeItem(
      formatIndexingMessage(this.indexingStatus as IndexingStatus),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('sync~spin');
    return placeholder;
  }

  private getRootKeys(): string[] {
    const categories = new Set<string>();
    for (const classification of this.categoryCache.values()) {
      categories.add(classification.category);
    }
    return Array.from(categories).sort((a, b) => a.localeCompare(b, 'es'));
  }
}

