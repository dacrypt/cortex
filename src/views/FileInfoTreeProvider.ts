/**
 * FileInfoTreeProvider - Muestra toda la información del archivo actual
 */

import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs/promises';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';
import { FileMetadata } from '../models/types';
import { FileIndexEntry } from '../models/types';
import { getSavedSummariesForFile } from '../utils/saveAISummary';

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
  private _onDidChangeTreeData: vscode.EventEmitter<
    FileInfoTreeItem | undefined | null | void
  > = new vscode.EventEmitter<FileInfoTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData: vscode.Event<
    FileInfoTreeItem | undefined | null | void
  > = this._onDidChangeTreeData.event;

  private currentFile: string | null = null;

  constructor(
    private workspaceRoot: string,
    private metadataStore: IMetadataStore,
    private indexStore: IndexStore
  ) {}

  /**
   * Actualiza el archivo actual y refresca la vista
   */
  updateCurrentFile(relativePath: string | null): void {
    this.currentFile = relativePath;
    this.refresh();
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: FileInfoTreeItem): vscode.TreeItem {
    return element;
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
          'No hay archivo abierto',
          vscode.TreeItemCollapsibleState.None,
          undefined,
          'Abre un archivo para ver su información',
          new vscode.ThemeIcon('info')
        ),
      ];
    }

    const fileEntry = this.indexStore.getFile(this.currentFile);
    if (!fileEntry) {
      return [
        new FileInfoTreeItem(
          'Archivo no indexado',
          vscode.TreeItemCollapsibleState.None,
          this.currentFile,
          'Este archivo no está en el índice',
          new vscode.ThemeIcon('warning')
        ),
      ];
    }

    const metadata = this.metadataStore.getMetadataByPath(
      this.currentFile
    ) as FileMetadata | null;

    const items: FileInfoTreeItem[] = [];

    // Información básica del archivo
    items.push(
      new FileInfoTreeItem(
        '📄 Información Básica',
        vscode.TreeItemCollapsibleState.Expanded,
        undefined,
        undefined,
        new vscode.ThemeIcon('file')
      )
    );

    // Tags
    const tags = metadata?.tags || [];
    items.push(
      new FileInfoTreeItem(
        `🏷️ Tags (${tags.length})`,
        tags.length > 0
          ? vscode.TreeItemCollapsibleState.Expanded
          : vscode.TreeItemCollapsibleState.None,
        tags.length === 0 ? 'Sin tags' : undefined,
        undefined,
        new vscode.ThemeIcon('tag')
      )
    );

    // Proyectos/Contextos
    const contexts = metadata?.contexts || [];
    items.push(
      new FileInfoTreeItem(
        `📁 Proyectos (${contexts.length})`,
        contexts.length > 0
          ? vscode.TreeItemCollapsibleState.Expanded
          : vscode.TreeItemCollapsibleState.None,
        contexts.length === 0 ? 'Sin proyectos' : undefined,
        undefined,
        new vscode.ThemeIcon('folder')
      )
    );

    // Proyectos sugeridos
    const suggestedContexts = metadata?.suggestedContexts || [];
    if (suggestedContexts.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `💡 Proyectos Sugeridos (${suggestedContexts.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('lightbulb')
        )
      );
    }

    // Resumen AI
    if (metadata?.aiSummary) {
      items.push(
        new FileInfoTreeItem(
          '🤖 Resumen AI',
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('robot')
        )
      );
    }

    // Resúmenes guardados en el repositorio
    // Check for saved summaries asynchronously
    const savedSummaries = await getSavedSummariesForFile(
      this.workspaceRoot,
      this.currentFile
    );
    if (savedSummaries.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `💾 Resúmenes Guardados (${savedSummaries.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('save')
        )
      );
    }

    // Términos clave
    if (metadata?.aiKeyTerms && metadata.aiKeyTerms.length > 0) {
      items.push(
        new FileInfoTreeItem(
          `🔑 Términos Clave (${metadata.aiKeyTerms.length})`,
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('key')
        )
      );
    }

    // Notas
    if (metadata?.notes) {
      items.push(
        new FileInfoTreeItem(
          '📝 Notas',
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('note')
        )
      );
    }

    // Metadatos del archivo
    items.push(
      new FileInfoTreeItem(
        '📊 Metadatos',
        vscode.TreeItemCollapsibleState.Expanded,
        undefined,
        undefined,
        new vscode.ThemeIcon('graph')
      )
    );

    // Mirror metadata
    if (metadata?.mirror) {
      items.push(
        new FileInfoTreeItem(
          '🪞 Mirror',
          vscode.TreeItemCollapsibleState.Expanded,
          undefined,
          undefined,
          new vscode.ThemeIcon('mirror')
        )
      );
    }

    return items;
  }

  private async getChildItems(parentLabel: string): Promise<FileInfoTreeItem[]> {
    if (!this.currentFile) {
      return [];
    }

    const fileEntry = this.indexStore.getFile(this.currentFile);
    if (!fileEntry) {
      return [];
    }

    const metadata = this.metadataStore.getMetadataByPath(
      this.currentFile
    ) as FileMetadata | null;

    const items: FileInfoTreeItem[] = [];

    if (parentLabel === '📄 Información Básica') {
      items.push(
        new FileInfoTreeItem(
          `Ruta: ${this.currentFile}`,
          vscode.TreeItemCollapsibleState.None,
          undefined,
          this.currentFile
        )
      );
      items.push(
        new FileInfoTreeItem(
          `Nombre: ${fileEntry.filename}`,
          vscode.TreeItemCollapsibleState.None
        )
      );
      items.push(
        new FileInfoTreeItem(
          `Extensión: ${fileEntry.extension || 'sin extensión'}`,
          vscode.TreeItemCollapsibleState.None
        )
      );
      items.push(
        new FileInfoTreeItem(
          `Tipo: ${metadata?.type || fileEntry.extension || 'desconocido'}`,
          vscode.TreeItemCollapsibleState.None
        )
      );
      items.push(
        new FileInfoTreeItem(
          `Tamaño: ${this.formatFileSize(fileEntry.fileSize)}`,
          vscode.TreeItemCollapsibleState.None
        )
      );
      items.push(
        new FileInfoTreeItem(
          `Modificado: ${new Date(fileEntry.lastModified).toLocaleString()}`,
          vscode.TreeItemCollapsibleState.None
        )
      );
      if (metadata) {
        items.push(
          new FileInfoTreeItem(
            `Creado en índice: ${new Date(metadata.created_at * 1000).toLocaleString()}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
        items.push(
          new FileInfoTreeItem(
            `Actualizado: ${new Date(metadata.updated_at * 1000).toLocaleString()}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
      }
    } else if (parentLabel.startsWith('🏷️ Tags')) {
      const tags = metadata?.tags || [];
      if (tags.length === 0) {
        items.push(
          new FileInfoTreeItem(
            'No hay tags asignados',
            vscode.TreeItemCollapsibleState.None,
            undefined,
            'Usa "Cortex: Add tag to current file" para agregar tags',
            new vscode.ThemeIcon('info')
          )
        );
      } else {
        tags.forEach((tag) => {
          items.push(
            new FileInfoTreeItem(
              tag,
              vscode.TreeItemCollapsibleState.None,
              undefined,
              `Tag: ${tag}`,
              new vscode.ThemeIcon('tag')
            )
          );
        });
      }
    } else if (parentLabel.startsWith('📁 Proyectos')) {
      const contexts = metadata?.contexts || [];
      if (contexts.length === 0) {
        items.push(
          new FileInfoTreeItem(
            'No hay proyectos asignados',
            vscode.TreeItemCollapsibleState.None,
            undefined,
            'Usa "Cortex: Assign project to current file" para asignar proyectos',
            new vscode.ThemeIcon('info')
          )
        );
      } else {
        contexts.forEach((context) => {
          items.push(
            new FileInfoTreeItem(
              context,
              vscode.TreeItemCollapsibleState.None,
              undefined,
              `Proyecto: ${context}`,
              new vscode.ThemeIcon('folder')
            )
          );
        });
      }
    } else if (parentLabel.startsWith('💡 Proyectos Sugeridos')) {
      const suggestedContexts = metadata?.suggestedContexts || [];
      suggestedContexts.forEach((context) => {
        items.push(
          new FileInfoTreeItem(
            context,
            vscode.TreeItemCollapsibleState.None,
            undefined,
            `Proyecto sugerido: ${context}`,
            new vscode.ThemeIcon('lightbulb')
          )
        );
      });
    } else if (parentLabel === '🤖 Resumen AI') {
      if (metadata?.aiSummary) {
        // Dividir el resumen en líneas para mejor visualización
        const summaryLines = metadata.aiSummary.split('\n');
        summaryLines.forEach((line, index) => {
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
    } else if (parentLabel.startsWith('🔑 Términos Clave')) {
      const keyTerms = metadata?.aiKeyTerms || [];
      keyTerms.forEach((term) => {
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
    } else if (parentLabel === '📝 Notas') {
      if (metadata?.notes) {
        const noteLines = metadata.notes.split('\n');
        noteLines.forEach((line) => {
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
    } else if (parentLabel === '📊 Metadatos') {
      if (fileEntry.enhanced) {
        if (fileEntry.enhanced.stats) {
          items.push(
            new FileInfoTreeItem(
              `Tamaño: ${this.formatFileSize(fileEntry.enhanced.stats.size)}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Creado: ${new Date(fileEntry.enhanced.stats.created).toLocaleString()}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Modificado: ${new Date(fileEntry.enhanced.stats.modified).toLocaleString()}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Accedido: ${new Date(fileEntry.enhanced.stats.accessed).toLocaleString()}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Solo lectura: ${fileEntry.enhanced.stats.isReadOnly ? 'Sí' : 'No'}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Oculto: ${fileEntry.enhanced.stats.isHidden ? 'Sí' : 'No'}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
        }
        if (fileEntry.enhanced.folder) {
          items.push(
            new FileInfoTreeItem(
              `Carpeta: ${fileEntry.enhanced.folder}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
          items.push(
            new FileInfoTreeItem(
              `Profundidad: ${fileEntry.enhanced.depth}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
        }
        if (fileEntry.enhanced.language) {
          items.push(
            new FileInfoTreeItem(
              `Idioma: ${fileEntry.enhanced.language}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
        }
        if (fileEntry.enhanced.mimeType) {
          items.push(
            new FileInfoTreeItem(
              `Tipo MIME: ${fileEntry.enhanced.mimeType}`,
              vscode.TreeItemCollapsibleState.None
            )
          );
        }
      }
    } else if (parentLabel === '🪞 Mirror') {
      if (metadata?.mirror) {
        items.push(
          new FileInfoTreeItem(
            `Formato: ${metadata.mirror.format}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
        items.push(
          new FileInfoTreeItem(
            `Ruta: ${metadata.mirror.path}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
        items.push(
          new FileInfoTreeItem(
            `Última actualización: ${new Date(metadata.mirror.updatedAt * 1000).toLocaleString()}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
        items.push(
          new FileInfoTreeItem(
            `Mtime fuente: ${new Date(metadata.mirror.sourceMtime * 1000).toLocaleString()}`,
            vscode.TreeItemCollapsibleState.None
          )
        );
      }
    } else if (parentLabel.startsWith('💾 Resúmenes Guardados')) {
      const savedSummaries = await getSavedSummariesForFile(
        this.workspaceRoot,
        this.currentFile
      );
      
      if (savedSummaries.length === 0) {
        items.push(
          new FileInfoTreeItem(
            'No hay resúmenes guardados',
            vscode.TreeItemCollapsibleState.None,
            undefined,
            'Los resúmenes se guardan automáticamente cuando se generan',
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
            `Resumen guardado: ${relativePath}\nGenerado: ${new Date(stats.mtime).toLocaleString()}`,
            new vscode.ThemeIcon('file-text')
          );
          
          // Make it clickable to open the file
          item.command = {
            command: 'vscode.open',
            title: 'Abrir Resumen',
            arguments: [vscode.Uri.file(summaryPath)],
          };
          
          items.push(item);
        }
      }
    }

    return items;
  }

  private formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  }
}

