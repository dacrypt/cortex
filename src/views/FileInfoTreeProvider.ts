/**
 * FileInfoTreeProvider - Muestra toda la información del archivo actual
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import * as fs from 'node:fs/promises';
import * as crypto from 'node:crypto';
import { IMetadataStore } from '../core/IMetadataStore';
import { GrpcMetadataClient } from '../core/GrpcMetadataClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcClusteringClient } from '../core/GrpcClusteringClient';
import { FileMetadata, SuggestedMetadata, SuggestedTaxonomy } from '../models/types';
import { getSavedSummariesForFile } from '../utils/saveAISummary';
import { t } from './i18n';

/**
 * Type alias for FileInfoTreeItem change events
 */
type FileInfoTreeItemChangeEvent = FileInfoTreeItem | undefined | null | void;

/**
 * Type for file entry from backend
 */
interface FileEntry {
  file_id?: string;
  filename?: string;
  extension?: string;
  fileSize?: number;
  lastModified?: number;
  enhanced?: {
    stats?: {
      size?: number;
      created?: number;
      modified?: number;
      accessed?: number;
      isReadOnly?: boolean;
      isHidden?: boolean;
    };
    folder?: string;
    language?: string;
    mimeType?: string;
    indexed?: {
      basic?: boolean;
      mime?: boolean;
      code?: boolean;
      document?: boolean;
      mirror?: boolean;
      os_metadata?: boolean;
      enrichment?: boolean;
    };
    indexing_errors?: Array<{
      stage?: string;
      operation?: string;
      error?: string;
      details?: string;
      requirement?: string;
      timestamp?: number | string;
    }>;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

/**
 * Type for LLM trace entry
 */
interface LLMTrace {
  created_at?: string | number;
  operation?: string;
  stage?: string;
  model?: string;
  prompt_path?: string;
  prompt_preview?: string;
  output_path?: string;
  output_preview?: string;
  [key: string]: unknown;
}

/**
 * Tree item para la vista de información del archivo
 */
class FileInfoTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly description?: string,
    public readonly tooltip?: string,
    public readonly icon?: vscode.ThemeIcon | vscode.Uri | { light: vscode.Uri; dark: vscode.Uri }
  ) {
    super(label, collapsibleState);
    this.description = description;
    this.tooltip = tooltip;
    if (icon) {
      this.iconPath = icon;
    }
  }
}

/**
 * TreeDataProvider para la vista de información del archivo
 */
export class FileInfoTreeProvider
  implements vscode.TreeDataProvider<FileInfoTreeItem>
{
  private readonly _onDidChangeTreeData: vscode.EventEmitter<
    FileInfoTreeItemChangeEvent
  > = new vscode.EventEmitter<FileInfoTreeItemChangeEvent>();
  readonly onDidChangeTreeData: vscode.Event<
    FileInfoTreeItemChangeEvent
  > = this._onDidChangeTreeData.event;

  private currentFile: string | null = null;
  private readonly traceCache = new Map<string, unknown[]>();
  private treeView: vscode.TreeView<unknown> | null = null;
  private autoExpandEnabled = true;
  private suggestedMetadataCache: SuggestedMetadata | null = null;
  private disposed = false; // Track if provider has been disposed
  private readonly activeTimers = new Set<NodeJS.Timeout>(); // Track all active timers

  constructor(
    private readonly workspaceRoot: string,
    private readonly metadataStore: IMetadataStore,
    private readonly traceClient?: GrpcMetadataClient,
    private readonly adminClient?: GrpcAdminClient,
    private readonly knowledgeClient?: GrpcKnowledgeClient,
    private readonly clusteringClient?: GrpcClusteringClient,
    private backendWorkspaceId?: string
  ) {}

  /**
   * Establece la referencia a la TreeView para poder expandir elementos
   */
  setTreeView(
    treeView: vscode.TreeView<unknown>,
    options?: { autoExpand?: boolean }
  ): void {
    this.treeView = treeView;
    this.autoExpandEnabled = options?.autoExpand ?? true;
  }

  setBackendWorkspaceId(workspaceId: string): void {
    this.backendWorkspaceId = workspaceId;
    if (this.currentFile) {
      void this.loadTraces(this.currentFile);
    }
  }

  /**
   * Actualiza el archivo actual y refresca la vista
   */
  async updateCurrentFile(relativePath: string | null): Promise<void> {
    if (this.disposed) {
      return;
    }

    this.currentFile = relativePath;
    if (relativePath && this.traceClient && this.backendWorkspaceId) {
      void this.loadTraces(relativePath);
    }
    this.refresh();
    
    // Expandir automáticamente todos los elementos después de un breve delay
    // para permitir que el árbol se renderice primero
    if (this.treeView && this.autoExpandEnabled) {
      const timer = setTimeout(async () => {
        this.activeTimers.delete(timer);
        if (!this.disposed) {
          await this.expandAllSections();
        }
      }, 100);
      this.activeTimers.add(timer);
    }
  }

  /**
   * Expande todas las secciones del árbol al máximo
   */
  private async expandAllSections(): Promise<void> {
    if (this.disposed || !this.treeView || !this.currentFile) {
      return;
    }

    try {
      const rootItems = await this.getRootItems();
      await this.expandRootItems(rootItems);
    } catch (error) {
      this.safeLogWarn('[FileInfoTree] Error expanding sections:', error);
    }
  }

  /**
   * Expande los elementos raíz y sus hijos
   */
  private async expandRootItems(rootItems: FileInfoTreeItem[]): Promise<void> {
    if (!this.treeView) {
      return;
    }

    for (const rootItem of rootItems) {
      if (rootItem.collapsibleState !== vscode.TreeItemCollapsibleState.None) {
        await this.treeView.reveal(rootItem, { expand: true, focus: false, select: false });
        await this.expandChildren(rootItem);
      }
    }
  }

  /**
   * Expande los hijos de un elemento
   */
  private async expandChildren(parent: FileInfoTreeItem): Promise<void> {
    if (!this.treeView) {
      return;
    }

    const children = await this.getChildren(parent);
    for (const child of children) {
      if (child.collapsibleState !== vscode.TreeItemCollapsibleState.None) {
        await this.treeView.reveal(child, { expand: true, focus: false, select: false });
      }
    }
  }

  /**
   * Logs a warning message safely, ignoring errors during disposal
   */
  private safeLogWarn(message: string, error?: unknown): void {
    if (this.disposed) {
      return;
    }
    try {
      console.warn(message, error);
    } catch {
      // Ignore logging errors during disposal
    }
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: FileInfoTreeItem): vscode.TreeItem {
    return element;
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  getParent(_element: FileInfoTreeItem): FileInfoTreeItem | undefined {
    // VS Code requires getParent to be implemented when using reveal()
    // Since our tree structure is flat (root items -> child items), we return undefined
    // This means all items are considered root-level for parent purposes
    return undefined;
  }

  async getChildren(element?: FileInfoTreeItem): Promise<FileInfoTreeItem[]> {
    if (!element) {
      return await this.getRootItems();
    }

    // Si el elemento tiene hijos, los retornamos
    if (element.collapsibleState !== vscode.TreeItemCollapsibleState.None) {
      return await this.getChildItems(element.label);
    }

    return [];
  }

  private async getRootItems(): Promise<FileInfoTreeItem[]> {
    if (!this.currentFile) {
      return [
        new FileInfoTreeItem(
          t('noFileOpen'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('openFilePrompt'),
          new vscode.ThemeIcon('info')
        ),
      ];
    }

    const fileEntry = await this.getFileEntry();
    const metadata = this.metadataStore.getMetadataByPath(this.currentFile);
    const items: FileInfoTreeItem[] = [];

    this.addFileSection(items);
    await this.addPipelineStatusSection(items, fileEntry);
    await this.addOrganizationSections(items, fileEntry, metadata);
    await this.addSuggestionsSection(items);
    await this.addAISections(items, metadata);
    this.addNotesAndMetadataSections(items, metadata);

    return items;
  }

  private async getFileEntry(): Promise<FileEntry | null> {
    if (this.adminClient && this.backendWorkspaceId && this.currentFile) {
      try {
        return await this.adminClient.getFile(this.backendWorkspaceId, this.currentFile);
      } catch (error) {
        if (!this.disposed) {
          try {
            console.warn(`[Cortex] Failed to get file from backend: ${error}`);
          } catch {
            // Ignore logging errors during disposal
          }
        }
      }
    }
    return null;
  }

  private addFileSection(items: FileInfoTreeItem[]): void {
    items.push(
      new FileInfoTreeItem(
        t('fileSection'),
        vscode.TreeItemCollapsibleState.Expanded,
        undefined,
        t('fileSectionTooltip'),
        new vscode.ThemeIcon('file')
      )
    );
  }

  private async addPipelineStatusSection(
    items: FileInfoTreeItem[],
    fileEntry: FileEntry | null
  ): Promise<void> {
    if (!fileEntry?.enhanced) {
      return;
    }

    const indexed = fileEntry.enhanced.indexed;
    const errors = fileEntry.enhanced.indexing_errors || [];
    
    if (!indexed && errors.length === 0) {
      return; // No hay información del pipeline
    }

    const hasErrors = errors.length > 0;
    const errorCount = errors.length > 0 ? ` (${errors.length} error${errors.length > 1 ? 's' : ''})` : '';
    const statusLabel = hasErrors 
      ? `Estado del Pipeline${errorCount}`
      : 'Estado del Pipeline';

    items.push(
      new FileInfoTreeItem(
        statusLabel,
        vscode.TreeItemCollapsibleState.Expanded,
        undefined,
        hasErrors 
          ? `Pipeline con ${errors.length} error${errors.length > 1 ? 'es' : ''} de indexación`
          : 'Estado de procesamiento del archivo en el pipeline',
        new vscode.ThemeIcon(hasErrors ? 'warning' : 'check')
      )
    );
  }

  private getPipelineStatusChildItems(fileEntry: FileEntry): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const indexed = fileEntry.enhanced?.indexed;
    const errors = fileEntry.enhanced?.indexing_errors || [];

    // Mostrar estado de cada etapa del pipeline
    if (indexed) {
      const stages = [
        { key: 'basic', label: 'Basic', description: 'Información básica del archivo' },
        { key: 'mime', label: 'MIME', description: 'Tipo MIME detectado' },
        { key: 'code', label: 'Code', description: 'Análisis de código' },
        { key: 'document', label: 'Document', description: 'Procesamiento de documento' },
        { key: 'mirror', label: 'Mirror', description: 'Archivo espejo generado' },
        { key: 'os_metadata', label: 'OS Metadata', description: 'Metadatos del sistema operativo' },
        { key: 'enrichment', label: 'Enrichment', description: 'Enriquecimiento de datos' },
      ];

      for (const stage of stages) {
        const isComplete = indexed[stage.key as keyof typeof indexed] === true;
        items.push(
          new FileInfoTreeItem(
            stage.label,
            vscode.TreeItemCollapsibleState.None,
            isComplete ? '✓ Completado' : '○ Pendiente',
            `${stage.description}: ${isComplete ? 'Completado' : 'Pendiente'}`,
            new vscode.ThemeIcon(isComplete ? 'check' : 'circle-outline')
          )
        );
      }
    }

    // Mostrar errores de indexación si existen
    if (errors.length > 0) {
      items.push(
        new FileInfoTreeItem(
          '---',
          vscode.TreeItemCollapsibleState.None,
          undefined,
          undefined,
          undefined
        )
      );

      for (const error of errors) {
        const timestamp = error.timestamp 
          ? new Date(typeof error.timestamp === 'string' ? parseInt(error.timestamp) : error.timestamp * 1000).toLocaleString()
          : 'Desconocido';
        
        const errorLabel = `${error.stage || 'unknown'}/${error.operation || 'unknown'}`;
        const errorDescription = error.error || 'Error desconocido';
        
        items.push(
          new FileInfoTreeItem(
            errorLabel,
            vscode.TreeItemCollapsibleState.None,
            errorDescription,
            `Etapa: ${error.stage || 'unknown'}\nOperación: ${error.operation || 'unknown'}\nError: ${error.error || 'Desconocido'}\nDetalles: ${error.details || 'N/A'}\nRequisito: ${error.requirement || 'N/A'}\nTimestamp: ${timestamp}`,
            new vscode.ThemeIcon('error')
          )
        );
      }
    }

    return items;
  }

  private async addOrganizationSections(
    items: FileInfoTreeItem[],
    fileEntry: FileEntry | null,
    metadata: FileMetadata | null
  ): Promise<void> {
    const tags = metadata?.tags || [];
    const tagCount = tags.length > 0 ? ` (${tags.length})` : '';
    items.push(
      new FileInfoTreeItem(
        `${t('tagsSection')}${tagCount}`,
        vscode.TreeItemCollapsibleState.Expanded,
        tags.length === 0 ? t('tagsNoneAssigned') : undefined,
        this.formatTagsTooltip(tags),
        new vscode.ThemeIcon('tag')
      )
    );

    const projects = await this.loadProjects(fileEntry);
    const projectCount = projects.length > 0 ? ` (${projects.length})` : '';
    items.push(
      new FileInfoTreeItem(
        `${t('projectsSection')}${projectCount}`,
        vscode.TreeItemCollapsibleState.Expanded,
        projects.length === 0 ? t('projectsNoneAssigned') : undefined,
        this.formatProjectsTooltip(projects),
        new vscode.ThemeIcon('folder')
      )
    );

    const clusters = await this.loadClusters(fileEntry);
    const clusterCount = clusters.length > 0 ? ` (${clusters.length})` : '';
    items.push(
      new FileInfoTreeItem(
        `Clusters${clusterCount}`,
        vscode.TreeItemCollapsibleState.Expanded,
        clusters.length === 0 ? 'No clusters assigned' : undefined,
        this.formatClustersTooltip(clusters),
        new vscode.ThemeIcon('symbol-array')
      )
    );
  }

  private async loadProjects(fileEntry: FileEntry | null): Promise<Array<{ id: string; name: string }>> {
    const projects: Array<{ id: string; name: string }> = [];
    
    if (!this.knowledgeClient || !this.backendWorkspaceId || !fileEntry?.file_id || !this.currentFile) {
      return projects;
    }

    try {
      const normalized = this.currentFile.replaceAll('\\', '/');
      const documentId = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');
      const projectIds = await this.knowledgeClient.getProjectsForDocument(
        this.backendWorkspaceId,
        documentId
      );
      
      for (const projectId of projectIds) {
        const project = await this.knowledgeClient.getProject(this.backendWorkspaceId, projectId);
        if (project) {
          projects.push({ id: project.id, name: project.name });
        }
      }
    } catch (error) {
      if (!this.disposed) {
        try {
          console.warn(`[FileInfoTree] Failed to get projects from backend:`, error);
        } catch {
          // Ignore logging errors during disposal
        }
      }
    }

    return projects;
  }

  private async loadClusters(fileEntry: FileEntry | null): Promise<Array<{ id: string; name: string; score: number }>> {
    const clusters: Array<{ id: string; name: string; score: number }> = [];
    
    if (!this.clusteringClient || !this.backendWorkspaceId || !fileEntry?.file_id || !this.currentFile) {
      return clusters;
    }

    try {
      const normalized = this.currentFile.replaceAll('\\', '/');
      const documentId = crypto.createHash('sha256').update(`doc:${normalized}`).digest('hex');
      const clusterMemberships = await this.clusteringClient.getDocumentClusters(
        this.backendWorkspaceId,
        documentId
      );
      
      for (const membership of clusterMemberships) {
        if (membership.clusterName) {
          clusters.push({ 
            id: membership.clusterId, 
            name: membership.clusterName,
            score: membership.score || 0
          });
        }
      }
    } catch (error) {
      if (!this.disposed) {
        try {
          console.warn(`[FileInfoTree] Failed to get clusters from backend:`, error);
        } catch {
          // Ignore logging errors during disposal
        }
      }
    }

    return clusters;
  }

  private async addAISections(
    items: FileInfoTreeItem[],
    metadata: FileMetadata | null
  ): Promise<void> {
    this.addAISummarySection(items, metadata);
    await this.addLLMTraceSection(items);
    await this.addSavedSummariesSection(items);
    this.addKeyTermsSection(items, metadata);
  }

  private addAISummarySection(items: FileInfoTreeItem[], metadata: FileMetadata | null): void {
    if (metadata?.aiSummary) {
      items.push(
        new FileInfoTreeItem(
          t('aiSummaryLabel'),
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('aiSummaryTooltip'),
          new vscode.ThemeIcon('robot')
        )
      );
    }
  }

  private async addLLMTraceSection(items: FileInfoTreeItem[]): Promise<void> {
    if (!this.currentFile) return;
    
    const traces = this.traceCache.get(this.currentFile) || [];
    const traceCount = traces.length > 0 ? ` (${traces.length})` : '';
    const traceLabel = `${t('llmTraceLabel')}${traceCount}`;
    items.push(
      new FileInfoTreeItem(
        traceLabel,
        vscode.TreeItemCollapsibleState.Expanded,
        traces.length === 0 ? t('loading') : undefined,
        this.formatTracesTooltip(traces),
        new vscode.ThemeIcon('debug')
      )
    );
  }

  private async addSavedSummariesSection(items: FileInfoTreeItem[]): Promise<void> {
    if (!this.currentFile) return;
    
    const savedSummaries = await getSavedSummariesForFile(
      this.workspaceRoot,
      this.currentFile
    );
    if (savedSummaries.length > 0) {
      const summaryCount = ` (${savedSummaries.length})`;
      const plural = savedSummaries.length > 1 ? 'es' : '';
      items.push(
        new FileInfoTreeItem(
          `${t('savedSummariesLabel')}${summaryCount}`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('savedSummariesTooltip', { count: String(savedSummaries.length), plural }),
          new vscode.ThemeIcon('save')
        )
      );
    }
  }

  private addKeyTermsSection(items: FileInfoTreeItem[], metadata: FileMetadata | null): void {
    if (metadata?.aiKeyTerms && metadata.aiKeyTerms.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `${t('keyTermsLabel')} (${metadata.aiKeyTerms.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('keyTermsTooltip'),
          new vscode.ThemeIcon('key')
        )
      );
    }
  }

  private addNotesAndMetadataSections(
    items: FileInfoTreeItem[],
    metadata: FileMetadata | null
  ): void {
    if (metadata?.notes) {
      items.push(
        new FileInfoTreeItem(
          t('notesLabel'),
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('notesTooltip'),
          new vscode.ThemeIcon('note')
        )
      );
    }

    items.push(
      new FileInfoTreeItem(
        t('technicalMetadataLabel'),
        vscode.TreeItemCollapsibleState.Expanded,
        undefined,
        t('technicalMetadataTooltip'),
        new vscode.ThemeIcon('graph')
      )
    );

    if (metadata?.mirror) {
      items.push(
        new FileInfoTreeItem(
          t('mirrorLabel'),
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('mirrorTooltip'),
          new vscode.ThemeIcon('mirror')
        )
      );
    }
  }

  private async getChildItems(parentLabel: string): Promise<FileInfoTreeItem[]> {
    if (!this.currentFile) {
      return [];
    }

    const fileEntry = await this.getOrCreateFileEntry();
    if (!fileEntry) {
      return [];
    }

    const metadata = this.metadataStore.getMetadataByPath(this.currentFile);

    // Handle exact matches first
    const exactMatchHandler = this.getExactMatchHandler(parentLabel, fileEntry, metadata);
    if (exactMatchHandler) {
      return exactMatchHandler();
    }

    // Handle prefix matches
    return this.getPrefixMatchHandler(parentLabel, fileEntry, metadata);
  }

  private getExactMatchHandler(
    parentLabel: string,
    fileEntry: FileEntry,
    metadata: FileMetadata | null
  ): (() => Promise<FileInfoTreeItem[]> | FileInfoTreeItem[]) | null {
    const exactMatches: Record<string, () => Promise<FileInfoTreeItem[]> | FileInfoTreeItem[]> = {
      [t('fileSection')]: () => this.getFileChildItems(fileEntry, metadata),
      [t('aiSummaryLabel')]: () => this.getAISummaryChildItems(metadata),
      [t('notesLabel')]: () => this.getNotesChildItems(metadata),
      [t('technicalMetadataLabel')]: () => Promise.resolve(this.getTechnicalMetadataChildItems(fileEntry)),
      [t('mirrorLabel')]: () => this.getMirrorChildItems(metadata),
      [t('suggestedTaxonomyLabel')]: () => this.getSuggestedTaxonomyChildItems(),
    };

    // Handle pipeline status section (can have dynamic label with error count)
    if (parentLabel.startsWith('Estado del Pipeline')) {
      return () => this.getPipelineStatusChildItems(fileEntry);
    }

    return exactMatches[parentLabel] || null;
  }

  private getPrefixMatchHandler(
    parentLabel: string,
    fileEntry: FileEntry,
    metadata: FileMetadata | null
  ): Promise<FileInfoTreeItem[]> | FileInfoTreeItem[] {
    if (parentLabel.startsWith(t('tagsSection'))) {
      // Check if it's suggested tags first
      if (parentLabel.startsWith(t('suggestedTagsLabel'))) {
        return this.getSuggestedTagsChildItems();
      }
      return this.getTagsChildItems(metadata);
    }

    if (parentLabel.startsWith(t('projectsSection'))) {
      // Check if it's suggested projects first
      if (parentLabel.startsWith(t('suggestedProjectsLabel'))) {
        return this.getSuggestedProjectsChildItems();
      }
      return this.getProjectsChildItems();
    }

    if (parentLabel.startsWith('Clusters')) {
      return this.getClustersChildItems();
    }

    if (parentLabel.startsWith(t('keyTermsLabel'))) {
      return this.getKeyTermsChildItems(metadata);
    }

    if (parentLabel.startsWith(t('savedSummariesLabel'))) {
      return this.getSavedSummariesChildItems();
    }

    if (parentLabel.startsWith(t('llmTraceLabel'))) {
      return this.getLLMTraceChildItems();
    }

    if (parentLabel.startsWith(t('suggestionsLabel'))) {
      return this.getSuggestionsChildItems();
    }

    return [];
  }

  private async getOrCreateFileEntry(): Promise<FileEntry | null> {
    if (!this.adminClient || !this.backendWorkspaceId || !this.currentFile) {
      return null;
    }

    try {
      const fileEntry = await this.adminClient.getFile(this.backendWorkspaceId, this.currentFile);
      return fileEntry;
    } catch (error) {
      if (!this.disposed) {
        try {
          console.warn(`[Cortex] Failed to get file from backend: ${error}`);
        } catch {
          // Ignore logging errors during disposal
        }
      }
      return null;
    }
  }

  private getFileChildItems(fileEntry: FileEntry, metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    if (!this.currentFile) return items;
    
    // Información principal - más visible
    items.push(
      new FileInfoTreeItem(
        `📄 ${fileEntry.filename || path.basename(this.currentFile)}`,
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('fileNameLabel'),
          new vscode.ThemeIcon('file')
        )
      );
      
      const fileSize = fileEntry.fileSize || fileEntry.enhanced?.stats?.size || 0;
      const lastModified = fileEntry.lastModified || fileEntry.enhanced?.stats?.modified || 0;
      const extension = fileEntry.extension || path.extname(this.currentFile) || t('noExtension');
      const formattedFileSize = this.formatFileSize(fileSize);
      const lastModifiedDate = lastModified > 0 ? new Date(lastModified).toLocaleString() : t('unknown');
      
      const fileInfoItems: FileInfoTreeItem[] = [
        new FileInfoTreeItem(
          t('pathLabel'),
          vscode.TreeItemCollapsibleState.None,
          this.currentFile,
          this.currentFile,
          new vscode.ThemeIcon('folder-opened')
        ),
        new FileInfoTreeItem(
          t('fileSizeLabel'),
          vscode.TreeItemCollapsibleState.None,
          formattedFileSize,
          t('fileSizeTooltip', { size: formattedFileSize }),
          new vscode.ThemeIcon('database')
        ),
        new FileInfoTreeItem(
          t('lastModifiedLabel'),
          vscode.TreeItemCollapsibleState.None,
          lastModifiedDate,
          t('lastModifiedTooltip', { date: lastModifiedDate }),
          new vscode.ThemeIcon('clock')
        ),
        new FileInfoTreeItem(
          t('extensionLabel'),
          vscode.TreeItemCollapsibleState.None,
          extension,
          t('extensionTooltip'),
          new vscode.ThemeIcon('symbol-property')
        ),
      ];
      items.push(...fileInfoItems);
      
      const typeItem = new FileInfoTreeItem(
        t('fileTypeLabel'),
        vscode.TreeItemCollapsibleState.None,
        metadata?.type || fileEntry.extension || path.extname(this.currentFile) || t('unknown'),
        t('fileTypeTooltip'),
        new vscode.ThemeIcon('symbol-class')
      );
      items.push(typeItem);
      if (metadata) {
        const metadataItems: FileInfoTreeItem[] = [];
        if (metadata.created_at && metadata.created_at > 0) {
          metadataItems.push(
            new FileInfoTreeItem(
              t('indexAddedLabel'),
              vscode.TreeItemCollapsibleState.None,
              new Date(metadata.created_at * 1000).toLocaleString(),
              t('indexAddedTooltip'),
              new vscode.ThemeIcon('add')
            )
          );
        }
        if (metadata.updated_at && metadata.updated_at > 0) {
          metadataItems.push(
            new FileInfoTreeItem(
              t('lastUpdateLabel'),
              vscode.TreeItemCollapsibleState.None,
              new Date(metadata.updated_at * 1000).toLocaleString(),
              t('lastUpdateTooltip'),
              new vscode.ThemeIcon('refresh')
            )
          );
        }
        items.push(...metadataItems);
      }
    return items;
  }

  private getTagsChildItems(metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const tags = metadata?.tags || [];
    if (tags.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('tagsNoneAssigned'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('addTagHint'),
          new vscode.ThemeIcon('info')
        )
      );
    } else {
      tags.forEach((tag: string) => {
        items.push(
          new FileInfoTreeItem(
            tag,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            t('tagTooltip', { tag }),
            new vscode.ThemeIcon('tag')
          )
        );
      });
    }
    return items;
  }

  private async getProjectsChildItems(): Promise<FileInfoTreeItem[]> {
    const items: FileInfoTreeItem[] = [];
    const projects = await this.loadProjects(null);
    
    if (projects.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('projectsNoneAssigned'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('assignProjectHint'),
          new vscode.ThemeIcon('info')
        )
      );
    } else {
      projects.forEach((project) => {
        items.push(
          new FileInfoTreeItem(
            project.name,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            t('projectTooltip', { project: project.name }),
            new vscode.ThemeIcon('folder')
          )
        );
      });
    }
    return items;
  }

  private async getClustersChildItems(): Promise<FileInfoTreeItem[]> {
    const items: FileInfoTreeItem[] = [];
    const clusters = await this.loadClusters(null);
    
    if (clusters.length === 0) {
      items.push(
        new FileInfoTreeItem(
          'No clusters assigned',
          vscode.TreeItemCollapsibleState.None,
          undefined,
          'This file is not part of any document cluster',
          new vscode.ThemeIcon('info')
        )
      );
    } else {
      clusters.forEach((cluster) => {
        const scorePercent = cluster.score > 0 ? ` (${(cluster.score * 100).toFixed(0)}%)` : '';
        items.push(
          new FileInfoTreeItem(
            `${cluster.name}${scorePercent}`,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            `Cluster: ${cluster.name}${scorePercent ? ` - Confidence: ${(cluster.score * 100).toFixed(0)}%` : ''}`,
            new vscode.ThemeIcon('symbol-array')
          )
        );
      });
    }
    return items;
  }

  /**
   * Add suggestions section to the tree
   */
  private async addSuggestionsSection(items: FileInfoTreeItem[]): Promise<void> {
    if (!this.currentFile || !this.traceClient || !this.backendWorkspaceId) {
      return;
    }

    try {
      // Load suggested metadata from backend
      const suggested = await this.traceClient.getSuggestedMetadata(
        this.backendWorkspaceId,
        this.currentFile
      );
      
      this.suggestedMetadataCache = suggested;

      if (!suggested) {
        return; // No suggestions available
      }

      const tagCount = suggested.suggestedTags?.length || 0;
      const projectCount = suggested.suggestedProjects?.length || 0;
      const hasTaxonomy = !!suggested.suggestedTaxonomy;
      const fieldCount = suggested.suggestedFields?.length || 0;
      const totalCount = tagCount + projectCount + (hasTaxonomy ? 1 : 0) + fieldCount;

      if (totalCount === 0) {
        return; // No suggestions to show
      }

      items.push(
        new FileInfoTreeItem(
          `${t('suggestionsLabel')} (${totalCount})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('suggestionsTooltip', { confidence: (suggested.confidence * 100).toFixed(0) }),
          new vscode.ThemeIcon('lightbulb')
        )
      );
    } catch (error) {
      if (!this.disposed) {
        try {
          console.warn(`[FileInfoTree] Failed to load suggestions: ${error}`);
        } catch {
          // Ignore logging errors during disposal
        }
      }
    }
  }

  /**
   * Get child items for suggestions section
   */
  private async getSuggestionsChildItems(): Promise<FileInfoTreeItem[]> {
    if (!this.suggestedMetadataCache) {
      return [
        new FileInfoTreeItem(
          t('suggestionsLoading'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          undefined,
          new vscode.ThemeIcon('loading')
        ),
      ];
    }

    const items: FileInfoTreeItem[] = [];
    const suggested = this.suggestedMetadataCache;

    // Tags section
    if (suggested.suggestedTags && suggested.suggestedTags.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `${t('suggestedTagsLabel')} (${suggested.suggestedTags.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('suggestedTagsTooltip', { count: suggested.suggestedTags.length }),
          new vscode.ThemeIcon('tag')
        )
      );
    }

    // Projects section
    if (suggested.suggestedProjects && suggested.suggestedProjects.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `${t('suggestedProjectsLabel')} (${suggested.suggestedProjects.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('suggestedProjectsTooltip', { count: suggested.suggestedProjects.length }),
          new vscode.ThemeIcon('folder')
        )
      );
    }

    // Taxonomy section
    if (suggested.suggestedTaxonomy) {
      items.push(
        new FileInfoTreeItem(
          t('suggestedTaxonomyLabel'),
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('suggestedTaxonomyTooltip'),
          new vscode.ThemeIcon('symbol-class')
        )
      );
    }

    // Fields section
    if (suggested.suggestedFields && suggested.suggestedFields.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `${t('suggestedFieldsLabel')} (${suggested.suggestedFields.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          t('suggestedFieldsTooltip', { count: suggested.suggestedFields.length }),
          new vscode.ThemeIcon('symbol-field')
        )
      );
    }

    if (items.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('noSuggestions'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('noSuggestionsTooltip'),
          new vscode.ThemeIcon('info')
        )
      );
    }

    return items;
  }

  /**
   * Get child items for suggested tags
   */
  private getSuggestedTagsChildItems(): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const suggested = this.suggestedMetadataCache;

    if (!suggested?.suggestedTags || suggested.suggestedTags.length === 0) {
      return items;
    }

    for (const tag of suggested.suggestedTags) {
      const confidencePercent = (tag.confidence * 100).toFixed(0);
      const item = new FileInfoTreeItem(
        `${tag.tag} (${confidencePercent}%)`,
        vscode.TreeItemCollapsibleState.None,
        tag.reason || undefined,
        t('suggestedTagDetails', {
          source: tag.source,
          confidence: confidencePercent,
          reason: tag.reason || 'N/A',
        }),
        new vscode.ThemeIcon('tag')
      );
      item.contextValue = 'cortex-suggested-tag';
      item.command = {
        command: 'cortex.acceptSuggestedTag',
        title: t('acceptTagAction'),
        arguments: [tag.tag],
      };
      items.push(item);
    }

    return items;
  }

  /**
   * Get child items for suggested projects
   */
  private getSuggestedProjectsChildItems(): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const suggested = this.suggestedMetadataCache;

    if (!suggested?.suggestedProjects || suggested.suggestedProjects.length === 0) {
      return items;
    }

    for (const project of suggested.suggestedProjects) {
      const confidencePercent = (project.confidence * 100).toFixed(0);
      const isNewBadge = project.isNew ? ' [NUEVO]' : '';
      const item = new FileInfoTreeItem(
        `${project.projectName}${isNewBadge} (${confidencePercent}%)`,
        vscode.TreeItemCollapsibleState.None,
        project.reason || undefined,
        t('suggestedProjectDetails', {
          source: project.source,
          confidence: confidencePercent,
          reason: project.reason || 'N/A',
          newBadge: project.isNew ? t('suggestedProjectNew') : '',
        }),
        new vscode.ThemeIcon('folder')
      );
      item.contextValue = 'cortex-suggested-project';
      item.command = {
        command: 'cortex.acceptSuggestedProject',
        title: t('acceptProjectAction'),
        arguments: [project.projectName],
      };
      items.push(item);
    }

    return items;
  }

  /**
   * Get child items for suggested taxonomy
   */
  private getSuggestedTaxonomyChildItems(): FileInfoTreeItem[] {
    const taxonomy = this.suggestedMetadataCache?.suggestedTaxonomy;
    if (!taxonomy) {
      return [];
    }

    const items: FileInfoTreeItem[] = [];
    
    this.addCategoryItem(items, taxonomy);
    this.addDomainItem(items, taxonomy);
    this.addContentTypeItem(items, taxonomy);
    this.addTopicsItem(items, taxonomy);
    this.addReasoningItem(items, taxonomy);

    return items;
  }

  private addCategoryItem(items: FileInfoTreeItem[], taxonomy: SuggestedTaxonomy): void {
    if (!taxonomy.category) {
      return;
    }

    const confidence = this.formatConfidence(taxonomy.categoryConfidence);
    const categoryLabel = taxonomy.subcategory 
      ? `${taxonomy.category} > ${taxonomy.subcategory}`
      : taxonomy.category;
    
    items.push(
      new FileInfoTreeItem(
        t('categoryLabel', { category: taxonomy.category, confidence }),
        vscode.TreeItemCollapsibleState.None,
        taxonomy.subcategory || undefined,
        t('categorySuggested', { category: categoryLabel }),
        new vscode.ThemeIcon('symbol-class')
      )
    );
  }

  private addDomainItem(items: FileInfoTreeItem[], taxonomy: SuggestedTaxonomy): void {
    if (!taxonomy.domain) {
      return;
    }

    const confidence = this.formatConfidence(taxonomy.domainConfidence);
    const domainLabel = taxonomy.subdomain 
      ? `${taxonomy.domain} > ${taxonomy.subdomain}`
      : taxonomy.domain;
    
    items.push(
      new FileInfoTreeItem(
        t('domainLabel', { domain: taxonomy.domain, confidence }),
        vscode.TreeItemCollapsibleState.None,
        taxonomy.subdomain || undefined,
        t('domainSuggested', { domain: domainLabel }),
        new vscode.ThemeIcon('globe')
      )
    );
  }

  private addContentTypeItem(items: FileInfoTreeItem[], taxonomy: SuggestedTaxonomy): void {
    if (!taxonomy.contentType) {
      return;
    }

    const confidence = this.formatConfidence(taxonomy.contentTypeConfidence);
    const contentTypeLabel = taxonomy.purpose 
      ? `${taxonomy.contentType} (${taxonomy.purpose})`
      : taxonomy.contentType;
    
    items.push(
      new FileInfoTreeItem(
        t('contentTypeLabel', { type: taxonomy.contentType, confidence }),
        vscode.TreeItemCollapsibleState.None,
        taxonomy.purpose || undefined,
        t('contentTypeSuggested', { type: contentTypeLabel }),
        new vscode.ThemeIcon('symbol-property')
      )
    );
  }

  private addTopicsItem(items: FileInfoTreeItem[], taxonomy: SuggestedTaxonomy): void {
    if (!taxonomy.topic || taxonomy.topic.length === 0) {
      return;
    }

    const topicsText = taxonomy.topic.join(', ');
    items.push(
      new FileInfoTreeItem(
        t('topicsLabel', { topics: topicsText }),
        vscode.TreeItemCollapsibleState.None,
        undefined,
        t('topicsSuggested', { topics: topicsText }),
        new vscode.ThemeIcon('symbol-keyword')
      )
    );
  }

  private addReasoningItem(items: FileInfoTreeItem[], taxonomy: SuggestedTaxonomy): void {
    if (!taxonomy.reasoning) {
      return;
    }

    items.push(
      new FileInfoTreeItem(
        t('reasoningLabel'),
        vscode.TreeItemCollapsibleState.None,
        taxonomy.reasoning,
        taxonomy.reasoning,
        new vscode.ThemeIcon('comment')
      )
    );
  }

  private formatConfidence(confidence?: number): string {
    return confidence ? ` (${(confidence * 100).toFixed(0)}%)` : '';
  }

  private getAISummaryChildItems(metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    if (metadata?.aiSummary) {
      const summaryLines = metadata.aiSummary.split('\n');
        summaryLines.forEach((line: string) => {
        if (line.trim()) {
          items.push(
            new FileInfoTreeItem(
              line.trim(),
              vscode.TreeItemCollapsibleState.None,
              undefined,
              line.trim()
            )
          );
        }
      });
      if (metadata.aiSummaryHash) {
        items.push(
          new FileInfoTreeItem(
            `Hash: ${metadata.aiSummaryHash.substring(0, 16)}...`,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            `Hash completo: ${metadata.aiSummaryHash}`
          )
        );
      }
    }
    return items;
  }

  private getKeyTermsChildItems(metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const keyTerms = metadata?.aiKeyTerms || [];
      keyTerms.forEach((term: string) => {
      items.push(
        new FileInfoTreeItem(
          term,
          vscode.TreeItemCollapsibleState.None,
          undefined,
          `Término clave: ${term}`,
          new vscode.ThemeIcon('key')
        )
      );
    });
    return items;
  }

  private getNotesChildItems(metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    if (metadata?.notes) {
      const noteLines = metadata.notes.split('\n');
        noteLines.forEach((line: string) => {
        if (line.trim()) {
          items.push(
            new FileInfoTreeItem(
              line.trim(),
              vscode.TreeItemCollapsibleState.None,
              undefined,
              line.trim()
            )
          );
        }
      });
    }
    return items;
  }

  private getTechnicalMetadataChildItems(fileEntry: FileEntry): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    
    // Get stats from enhanced metadata (backend is source of truth)
    const stats = fileEntry.enhanced?.stats;
    
    if (stats) {
      const statsItems = this.buildStatsItems(stats);
      items.push(...statsItems);
    }
    
    // Add enhanced metadata items
    if (fileEntry.enhanced) {
      const enhancedItems = this.buildEnhancedItems(fileEntry.enhanced);
      items.push(...enhancedItems);
    }
    
    // If we still have no items, show a message
    if (items.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('noTechnicalMetadata'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('technicalMetadataHint'),
          new vscode.ThemeIcon('info')
        )
      );
    }
    
    return items;
  }

  private buildStatsItems(stats: { size?: number; created?: number; modified?: number; accessed?: number; isReadOnly?: boolean; isHidden?: boolean }): FileInfoTreeItem[] {
    const statsItems: FileInfoTreeItem[] = [];
    
    if (typeof stats.size === 'number') {
      statsItems.push(
        new FileInfoTreeItem(
          t('sizeLabel', { size: this.formatFileSize(stats.size) }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    if (typeof stats.created === 'number' && stats.created > 0) {
      statsItems.push(
        new FileInfoTreeItem(
          t('createdLabel', { date: new Date(stats.created).toLocaleString() }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    if (typeof stats.modified === 'number' && stats.modified > 0) {
      statsItems.push(
        new FileInfoTreeItem(
          t('modifiedLabel', { date: new Date(stats.modified).toLocaleString() }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    if (typeof stats.accessed === 'number' && stats.accessed > 0) {
      statsItems.push(
        new FileInfoTreeItem(
          t('accessedLabel', { date: new Date(stats.accessed).toLocaleString() }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    if (typeof stats.isReadOnly === 'boolean') {
      statsItems.push(
        new FileInfoTreeItem(
          t('readOnlyLabel', { value: stats.isReadOnly ? t('yes') : t('no') }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    if (typeof stats.isHidden === 'boolean') {
      statsItems.push(
        new FileInfoTreeItem(
          t('hiddenLabel', { value: stats.isHidden ? t('yes') : t('no') }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    
    return statsItems;
  }

  private buildEnhancedItems(enhanced: { folder?: unknown; depth?: unknown; language?: unknown; mimeType?: unknown; mime_type?: unknown; document_metrics?: unknown; documentMetrics?: unknown }): FileInfoTreeItem[] {
    const enhancedItems: FileInfoTreeItem[] = [];
    
    if (typeof enhanced.folder === 'string') {
      enhancedItems.push(
        new FileInfoTreeItem(
          t('folderLabel', { folder: enhanced.folder }),
          vscode.TreeItemCollapsibleState.None
        )
      );
      if (typeof enhanced.depth === 'number') {
        enhancedItems.push(
          new FileInfoTreeItem(
            t('depthLabel', { depth: String(enhanced.depth) }),
            vscode.TreeItemCollapsibleState.None
          )
        );
      }
    }
    if (typeof enhanced.language === 'string') {
      enhancedItems.push(
        new FileInfoTreeItem(
          t('languageLabel', { language: enhanced.language }),
          vscode.TreeItemCollapsibleState.None
        )
      );
    }
    
    // Handle mimeType (camelCase) or mime_type (snake_case)
    let mimeType: string | undefined;
    if (typeof enhanced.mimeType === 'string') {
      mimeType = enhanced.mimeType;
    } else if (typeof enhanced.mime_type === 'object' && enhanced.mime_type !== null && 'mime_type' in enhanced.mime_type) {
      mimeType = (enhanced.mime_type as { mime_type?: string }).mime_type;
    }
    
    if (mimeType) {
      enhancedItems.push(
        new FileInfoTreeItem(
          t('mimeTypeLabel', { mime: mimeType }),
          vscode.TreeItemCollapsibleState.None
        )
      );
      
      // Add MIME category if available
      if (typeof enhanced.mime_type === 'object' && enhanced.mime_type !== null && 'category' in enhanced.mime_type) {
        const category = (enhanced.mime_type as { category?: string }).category;
        if (category) {
          enhancedItems.push(
            new FileInfoTreeItem(
              t('mimeCategoryLabel', { category }),
              vscode.TreeItemCollapsibleState.None
            )
          );
        }
      }
    }
    
    // Handle DocumentMetrics (for PDFs and other documents)
    const docMetrics = enhanced.document_metrics || enhanced.documentMetrics;
    if (docMetrics && typeof docMetrics === 'object' && docMetrics !== null) {
      const metrics = docMetrics as Record<string, unknown>;
      
      // Basic metrics
      if (typeof metrics.page_count === 'number' && metrics.page_count > 0) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📄 ${t('pageCountLabel', { count: String(metrics.page_count) })}`,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            t('pageCountTooltip', { count: String(metrics.page_count) }),
            new vscode.ThemeIcon('file-pdf')
          )
        );
      }
      
      if (typeof metrics.word_count === 'number' && metrics.word_count > 0) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📝 ${t('wordCountLabel', { count: String(metrics.word_count) })}`,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            t('wordCountTooltip', { count: String(metrics.word_count) }),
            new vscode.ThemeIcon('text-size')
          )
        );
      }
      
      if (typeof metrics.character_count === 'number' && metrics.character_count > 0) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `🔤 ${t('characterCountLabel', { count: String(metrics.character_count) })}`,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            t('characterCountTooltip', { count: String(metrics.character_count) }),
            new vscode.ThemeIcon('symbol-text')
          )
        );
      }
      
      // PDF Info Dictionary metadata
      if (typeof metrics.title === 'string' && metrics.title.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📚 ${t('titleLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.title,
            metrics.title,
            new vscode.ThemeIcon('book')
          )
        );
      }
      
      if (typeof metrics.author === 'string' && metrics.author.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `✍️ ${t('authorLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.author,
            metrics.author,
            new vscode.ThemeIcon('person')
          )
        );
      }
      
      if (typeof metrics.subject === 'string' && metrics.subject.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📋 ${t('subjectLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.subject,
            metrics.subject,
            new vscode.ThemeIcon('tag')
          )
        );
      }
      
      if (Array.isArray(metrics.keywords) && metrics.keywords.length > 0) {
        const keywords = metrics.keywords.filter((k): k is string => typeof k === 'string' && k.trim() !== '');
        if (keywords.length > 0) {
          enhancedItems.push(
            new FileInfoTreeItem(
              `🏷️ ${t('keywordsLabel')}`,
              vscode.TreeItemCollapsibleState.None,
              keywords.join(', '),
              keywords.join(', '),
              new vscode.ThemeIcon('tag')
            )
          );
        }
      }
      
      if (typeof metrics.creator === 'string' && metrics.creator.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `🛠️ ${t('creatorLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.creator,
            metrics.creator,
            new vscode.ThemeIcon('tools')
          )
        );
      }
      
      if (typeof metrics.producer === 'string' && metrics.producer.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `⚙️ ${t('producerLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.producer,
            metrics.producer,
            new vscode.ThemeIcon('gear')
          )
        );
      }
      
      // PDF technical metadata
      if (typeof metrics.pdf_version === 'string' && metrics.pdf_version.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📄 ${t('pdfVersionLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.pdf_version,
            t('pdfVersionTooltip', { version: metrics.pdf_version }),
            new vscode.ThemeIcon('file-pdf')
          )
        );
      }
      
      if (typeof metrics.pdf_encrypted === 'boolean') {
        enhancedItems.push(
          new FileInfoTreeItem(
            `🔒 ${t('pdfEncryptedLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.pdf_encrypted ? t('yes') : t('no'),
            metrics.pdf_encrypted ? t('pdfEncryptedYes') : t('pdfEncryptedNo'),
            new vscode.ThemeIcon(metrics.pdf_encrypted ? 'lock' : 'unlock')
          )
        );
      }
      
      // Dates
      if (typeof metrics.created_date === 'number' && metrics.created_date > 0) {
        const date = new Date(metrics.created_date * 1000).toLocaleString();
        enhancedItems.push(
          new FileInfoTreeItem(
            `📅 ${t('createdDateLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            date,
            date,
            new vscode.ThemeIcon('calendar')
          )
        );
      }
      
      if (typeof metrics.modified_date === 'number' && metrics.modified_date > 0) {
        const date = new Date(metrics.modified_date * 1000).toLocaleString();
        enhancedItems.push(
          new FileInfoTreeItem(
            `🔄 ${t('modifiedDateLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            date,
            date,
            new vscode.ThemeIcon('refresh')
          )
        );
      }
      
      // XMP Metadata (if available)
      if (typeof metrics.xmp_title === 'string' && metrics.xmp_title.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📖 ${t('xmpTitleLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.xmp_title,
            metrics.xmp_title,
            new vscode.ThemeIcon('bookmark')
          )
        );
      }
      
      if (typeof metrics.xmp_description === 'string' && metrics.xmp_description.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📝 ${t('xmpDescriptionLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.xmp_description.length > 100 
              ? metrics.xmp_description.substring(0, 100) + '...' 
              : metrics.xmp_description,
            metrics.xmp_description,
            new vscode.ThemeIcon('note')
          )
        );
      }
      
      // Additional properties
      if (typeof metrics.company === 'string' && metrics.company.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `🏢 ${t('companyLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.company,
            metrics.company,
            new vscode.ThemeIcon('organization')
          )
        );
      }
      
      if (typeof metrics.category === 'string' && metrics.category.trim()) {
        enhancedItems.push(
          new FileInfoTreeItem(
            `📁 ${t('categoryLabel')}`,
            vscode.TreeItemCollapsibleState.None,
            metrics.category,
            metrics.category,
            new vscode.ThemeIcon('folder')
          )
        );
      }
      
      // Custom properties
      if (metrics.custom_properties && typeof metrics.custom_properties === 'object' && metrics.custom_properties !== null) {
        const customProps = metrics.custom_properties as Record<string, unknown>;
        const propKeys = Object.keys(customProps);
        if (propKeys.length > 0) {
          for (const key of propKeys.slice(0, 10)) { // Limit to first 10 custom properties
            const value = customProps[key];
            if (typeof value === 'string' && value.trim()) {
              enhancedItems.push(
                new FileInfoTreeItem(
                  `🔧 ${key}`,
                  vscode.TreeItemCollapsibleState.None,
                  value.length > 50 ? value.substring(0, 50) + '...' : value,
                  value,
                  new vscode.ThemeIcon('settings')
                )
              );
            }
          }
        }
      }
    }
    
    return enhancedItems;
  }

  private getMirrorChildItems(metadata: FileMetadata | null): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    if (metadata?.mirror) {
      const mirrorItems: FileInfoTreeItem[] = [
        new FileInfoTreeItem(
          t('mirrorFormatLabel', { format: metadata.mirror.format }),
          vscode.TreeItemCollapsibleState.None
        ),
        new FileInfoTreeItem(
          t('mirrorPathLabel', { path: metadata.mirror.path }),
          vscode.TreeItemCollapsibleState.None
        ),
        new FileInfoTreeItem(
          t('mirrorUpdatedLabel', { date: metadata.mirror.updatedAt > 0 ? new Date(metadata.mirror.updatedAt * 1000).toLocaleString() : t('unknown') }),
          vscode.TreeItemCollapsibleState.None
        ),
        new FileInfoTreeItem(
          t('mirrorSourceMtimeLabel', { date: metadata.mirror.sourceMtime > 0 ? new Date(metadata.mirror.sourceMtime * 1000).toLocaleString() : t('unknown') }),
          vscode.TreeItemCollapsibleState.None
        ),
      ];
      items.push(...mirrorItems);
    }
    return items;
  }

  private async getSavedSummariesChildItems(): Promise<FileInfoTreeItem[]> {
    const items: FileInfoTreeItem[] = [];
    if (!this.currentFile) return items;
    
    const savedSummaries = await getSavedSummariesForFile(
      this.workspaceRoot,
      this.currentFile
    );
    
    if (savedSummaries.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('noSummaries'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('summariesHint'),
          new vscode.ThemeIcon('info')
        )
      );
    } else {
      for (const summaryPath of savedSummaries) {
        const relativePath = path.relative(this.workspaceRoot, summaryPath);
        const filename = path.basename(summaryPath);
        const stats = await fs.stat(summaryPath);
        
        const item = new FileInfoTreeItem(
          filename,
          vscode.TreeItemCollapsibleState.None,
          new Date(stats.mtime).toLocaleString(),
          t('summarySavedTooltip', { path: relativePath, date: new Date(stats.mtime).toLocaleString() }),
          new vscode.ThemeIcon('file-text')
        );
        
        item.command = {
          command: 'vscode.open',
          title: t('openSummaryAction'),
          arguments: [vscode.Uri.file(summaryPath)],
        };
        
        items.push(item);
      }
    }
    return items;
  }

  private getLLMTraceChildItems(): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    if (!this.currentFile) return items;
    
    const traces = this.traceCache.get(this.currentFile) || [];
    if (traces.length === 0) {
      items.push(
        new FileInfoTreeItem(
          t('noTraces'),
          vscode.TreeItemCollapsibleState.None,
          undefined,
          t('tracesHint'),
          new vscode.ThemeIcon('info')
        )
      );
    } else {
      for (const trace of traces) {
        const traceItems = this.buildTraceItems(trace as LLMTrace);
        items.push(...traceItems);
      }
    }
    return items;
  }

  private buildTraceItems(traceData: LLMTrace): FileInfoTreeItem[] {
    const items: FileInfoTreeItem[] = [];
    const { label, description } = this.getTraceLabelAndDescription(traceData);

    if (traceData.prompt_path) {
      const promptItem = this.createTraceFileItem(
        `📝 Prompt (${label})`,
        traceData.prompt_path,
        traceData.prompt_preview,
        description,
        'Abrir Prompt',
        'edit'
      );
      items.push(promptItem);
    }

    if (traceData.output_path) {
      const outputItem = this.createTraceFileItem(
        `📤 Output (${label})`,
        traceData.output_path,
        traceData.output_preview,
        description,
        'Abrir Output',
        'file-text'
      );
      items.push(outputItem);
    }

    return items;
  }

  private getTraceLabelAndDescription(traceData: LLMTrace): { label: string; description: string } {
    const when = traceData.created_at
      ? new Date(Number(traceData.created_at) * 1000).toLocaleString()
      : '';
    const op = traceData.operation || 'llm';
    const stage = traceData.stage || 'ai';
    const model = traceData.model ? `model=${traceData.model}` : '';
    const label = `${stage}/${op}`;
    const description = [when, model].filter(Boolean).join(' · ');
    return { label, description };
  }

  private createTraceFileItem(
    label: string,
    filePath: string,
    preview: string | undefined,
    description: string,
    commandTitle: string,
    iconName: string
  ): FileInfoTreeItem {
    const absolutePath = path.isAbsolute(filePath)
      ? filePath
      : path.join(this.workspaceRoot, filePath);
    const item = new FileInfoTreeItem(
      label,
      vscode.TreeItemCollapsibleState.None,
      description,
      preview || absolutePath,
      new vscode.ThemeIcon(iconName)
    );
    item.command = {
      command: 'vscode.open',
      title: commandTitle,
      arguments: [vscode.Uri.file(absolutePath)],
    };
    return item;
  }

  private async loadTraces(relativePath: string): Promise<void> {
    if (!this.canLoadTraces(relativePath)) {
      this.logTraceLoadWarning(relativePath);
      return;
    }

    try {
      this.safeLog('[FileInfo] Loading traces for:', relativePath);
      const traces = await this.fetchTraces(relativePath);
      this.cacheTraces(relativePath, traces);
    } catch (error) {
      this.handleTraceLoadError(relativePath, error);
    }
  }

  private canLoadTraces(relativePath: string): boolean {
    return !this.disposed && 
           !this.traceCache.has(relativePath) && 
           !!this.traceClient && 
           !!this.backendWorkspaceId;
  }

  private logTraceLoadWarning(relativePath: string): void {
    if (this.disposed || (this.traceClient && this.backendWorkspaceId)) {
      return;
    }
    this.safeLogWarn('[FileInfo] Cannot load traces:', {
      hasTraceClient: !!this.traceClient,
      hasWorkspaceId: !!this.backendWorkspaceId,
      relativePath
    });
  }

  private async fetchTraces(relativePath: string): Promise<unknown[]> {
    if (!this.traceClient || !this.backendWorkspaceId) {
      return [];
    }
    return await this.traceClient.listProcessingTraces(
      this.backendWorkspaceId,
      relativePath,
      50
    );
  }

  private cacheTraces(relativePath: string, traces: unknown[]): void {
    if (this.disposed) {
      return;
    }
    this.safeLog('[FileInfo] Loaded traces:', traces.length);
    this.traceCache.set(relativePath, traces);
    this.refresh();
  }

  private handleTraceLoadError(relativePath: string, error: unknown): void {
    if (this.disposed) {
      return;
    }
    this.safeLogWarn('[FileInfo] Failed to load traces:', error);
    this.traceCache.set(relativePath, []);
  }

  /**
   * Logs a message safely, ignoring errors during disposal
   */
  private safeLog(message: string, ...args: unknown[]): void {
    if (this.disposed) {
      return;
    }
    try {
      console.log(message, ...args);
    } catch {
      // Ignore logging errors during disposal
    }
  }

  private formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${Number.parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  }

  private formatTagsTooltip(tags: string[]): string {
    if (tags.length === 0) {
      return t('tagsTooltipEmpty');
    }
    const plural = tags.length > 1 ? 's' : '';
    return t('tagsTooltipCount', { count: String(tags.length), plural });
  }

  private formatProjectsTooltip(projects: Array<{ id: string; name: string }>): string {
    if (projects.length === 0) {
      return t('projectsTooltipEmpty');
    }
    const plural = projects.length > 1 ? 's' : '';
    return t('projectsTooltipCount', { count: String(projects.length), plural });
  }

  private formatClustersTooltip(clusters: Array<{ id: string; name: string; score: number }>): string {
    if (clusters.length === 0) {
      return 'No clusters assigned to this file';
    }
    const plural = clusters.length > 1 ? 's' : '';
    return `${clusters.length} cluster${plural} assigned`;
  }

  private formatTracesTooltip(traces: unknown[]): string {
    if (traces.length === 0) {
      return t('tracesTooltipEmpty');
    }
    const plural = traces.length > 1 ? 's' : '';
    return t('tracesTooltipCount', { count: String(traces.length), plural });
  }

  /**
   * Dispose of the provider
   */
  dispose(): void {
    this.disposed = true;
    
    // Clear all active timers
    this.activeTimers.forEach(timer => clearTimeout(timer));
    this.activeTimers.clear();
    
    // Clear caches
    this.traceCache.clear();
    this.suggestedMetadataCache = null;
    this.currentFile = null;
    this.treeView = null;
  }
}
