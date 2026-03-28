/**
 * TaxonomyTreeProvider - Dynamic taxonomy hierarchy view
 *
 * Displays the workspace taxonomy as a tree with:
 * - Hierarchical category structure (root > categories > subcategories)
 * - File counts per node
 * - Lazy loading of children
 * - Context menu for CRUD operations
 * - AI-powered taxonomy suggestions
 */

import * as vscode from "vscode";
import {
  GrpcTaxonomyClient,
  TaxonomyNode,
  TaxonomyInductionResult,
} from "../core/GrpcTaxonomyClient";

export class TaxonomyTreeItem extends vscode.TreeItem {
  constructor(
    public readonly node: TaxonomyNode,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState
  ) {
    super(node.name, collapsibleState);

    this.id = node.id;
    this.tooltip = this.buildTooltip();
    this.description = this.buildDescription();
    this.contextValue = this.getContextValue();
    this.iconPath = this.getIcon();

    // Command to show files when clicking on a node
    if (node.docCount > 0) {
      this.command = {
        command: "cortex.taxonomy.showFiles",
        title: "Show Files",
        arguments: [node.id],
      };
    }
  }

  private buildTooltip(): string {
    const lines = [this.node.name];
    if (this.node.description) {
      lines.push(this.node.description);
    }
    lines.push(`Path: ${this.node.path}`);
    lines.push(`Documents: ${this.node.docCount}`);
    if (this.node.childCount > 0) {
      lines.push(`Subcategories: ${this.node.childCount}`);
    }
    lines.push(`Source: ${this.node.source}`);
    lines.push(`Confidence: ${Math.round(this.node.confidence * 100)}%`);
    return lines.join("\n");
  }

  private buildDescription(): string {
    const parts: string[] = [];
    if (this.node.docCount > 0) {
      parts.push(`${this.node.docCount} files`);
    }
    if (this.node.source === "AI") {
      parts.push("AI");
    }
    return parts.join(" | ");
  }

  private getContextValue(): string {
    // Context value determines which context menu items appear
    const parts = ["taxonomyNode"];
    if (this.node.source === "USER") {
      parts.push("userCreated");
    }
    if (this.node.childCount === 0 && this.node.docCount === 0) {
      parts.push("empty");
    }
    return parts.join(".");
  }

  private getIcon(): vscode.ThemeIcon {
    // Different icons based on level and source
    if (this.node.level === 0) {
      return new vscode.ThemeIcon("folder-library");
    }
    if (this.node.source === "AI") {
      return new vscode.ThemeIcon("sparkle");
    }
    if (this.node.childCount > 0) {
      return new vscode.ThemeIcon("folder");
    }
    return new vscode.ThemeIcon("tag");
  }
}

export class TaxonomyTreeProvider
  implements vscode.TreeDataProvider<TaxonomyTreeItem>
{
  private _onDidChangeTreeData = new vscode.EventEmitter<
    TaxonomyTreeItem | undefined | null | void
  >();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private cachedNodes = new Map<string, TaxonomyNode[]>();

  constructor(
    private readonly taxonomyClient: GrpcTaxonomyClient,
    private workspaceId: string
  ) {}

  setWorkspaceId(workspaceId: string): void {
    this.workspaceId = workspaceId;
    this.refresh();
  }

  refresh(): void {
    this.cachedNodes.clear();
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TaxonomyTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: TaxonomyTreeItem): Promise<TaxonomyTreeItem[]> {
    if (!this.workspaceId) {
      return [];
    }

    try {
      let nodes: TaxonomyNode[];

      if (!element) {
        // Root level - get root nodes
        try {
          nodes = await this.taxonomyClient.getRootNodes(this.workspaceId);
        } catch (error: any) {
          if (this.isUnimplementedError(error)) {
            return [this.createStatusItem('Taxonomy service is not available', 'This feature requires backend support.')];
          }
          if (this.isTimeoutError(error)) {
            return [this.createStatusItem('Taxonomy request timed out', 'Backend did not respond in time.')];
          }
          throw error;
        }
      } else {
        // Get children of this node
        const cached = this.cachedNodes.get(element.node.id);
        if (cached) {
          nodes = cached;
        } else {
          try {
            nodes = await this.taxonomyClient.getChildren(
              this.workspaceId,
              element.node.id
            );
          } catch (error: any) {
            if (this.isUnimplementedError(error)) {
              return [this.createStatusItem('Taxonomy service is not available', 'This feature requires backend support.')];
            }
            if (this.isTimeoutError(error)) {
              return [this.createStatusItem('Taxonomy request timed out', 'Backend did not respond in time.')];
            }
            throw error;
          }
          this.cachedNodes.set(element.node.id, nodes);
        }
      }

      return nodes.map(
        (node) =>
          new TaxonomyTreeItem(
            node,
            node.childCount > 0
              ? vscode.TreeItemCollapsibleState.Collapsed
              : vscode.TreeItemCollapsibleState.None
          )
      );
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error loading nodes:", error);
      const message = error instanceof Error ? error.message : String(error);
      return [this.createStatusItem('Taxonomy failed to load', message)];
    }
  }

  getParent(element: TaxonomyTreeItem): vscode.ProviderResult<TaxonomyTreeItem> {
    // We don't track parent references, so return undefined
    // This means "Reveal in Tree" won't work, but that's acceptable
    return undefined;
  }

  private createStatusItem(message: string, description: string): TaxonomyTreeItem {
    const node: TaxonomyNode = {
      id: `taxonomy:status:${message}`,
      workspaceId: this.workspaceId,
      name: message,
      description,
      path: '/',
      level: 0,
      docCount: 0,
      childCount: 0,
      totalDocCount: 0,
      source: 'SYSTEM',
      confidence: 0,
      parentId: '',
      keywords: [],
      createdAt: Date.now(),
      updatedAt: Date.now(),
    };
    return new TaxonomyTreeItem(node, vscode.TreeItemCollapsibleState.None);
  }

  private isUnimplementedError(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error);
    return (error as { code?: number })?.code === 12 || message.includes('UNIMPLEMENTED');
  }

  private isTimeoutError(error: unknown): boolean {
    const message = error instanceof Error ? error.message.toLowerCase() : String(error).toLowerCase();
    return (error as { code?: number })?.code === 4 || message.includes('timed out');
  }

  // ============================================================
  // CRUD Operations
  // ============================================================

  async createNode(
    name: string,
    parentId?: string,
    description?: string
  ): Promise<TaxonomyNode | null> {
    try {
      const node = await this.taxonomyClient.createNode(
        this.workspaceId,
        name,
        parentId,
        description
      );
      this.refresh();
      return node;
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error creating node:", error);
      vscode.window.showErrorMessage(
        `Failed to create category: ${error instanceof Error ? error.message : String(error)}`
      );
      return null;
    }
  }

  async updateNode(
    nodeId: string,
    name?: string,
    description?: string
  ): Promise<boolean> {
    try {
      await this.taxonomyClient.updateNode(
        this.workspaceId,
        nodeId,
        name,
        description
      );
      this.refresh();
      return true;
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error updating node:", error);
      vscode.window.showErrorMessage(
        `Failed to update category: ${error instanceof Error ? error.message : String(error)}`
      );
      return false;
    }
  }

  async deleteNode(nodeId: string): Promise<boolean> {
    try {
      await this.taxonomyClient.deleteNode(this.workspaceId, nodeId);
      this.refresh();
      return true;
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error deleting node:", error);
      vscode.window.showErrorMessage(
        `Failed to delete category: ${error instanceof Error ? error.message : String(error)}`
      );
      return false;
    }
  }

  async addFileToNode(nodeId: string, fileId: string): Promise<boolean> {
    try {
      await this.taxonomyClient.addFileToNode(
        this.workspaceId,
        nodeId,
        fileId
      );
      this.refresh();
      return true;
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error adding file to node:", error);
      vscode.window.showErrorMessage(
        `Failed to assign file to category: ${error instanceof Error ? error.message : String(error)}`
      );
      return false;
    }
  }

  async removeFileFromNode(nodeId: string, fileId: string): Promise<boolean> {
    try {
      await this.taxonomyClient.removeFileFromNode(
        this.workspaceId,
        nodeId,
        fileId
      );
      this.refresh();
      return true;
    } catch (error) {
      console.error(
        "[TaxonomyTreeProvider] Error removing file from node:",
        error
      );
      vscode.window.showErrorMessage(
        `Failed to remove file from category: ${error instanceof Error ? error.message : String(error)}`
      );
      return false;
    }
  }

  async induceTaxonomy(): Promise<TaxonomyInductionResult | null> {
    try {
      const result = await this.taxonomyClient.induceTaxonomy(this.workspaceId);
      this.refresh();
      return result;
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error inducing taxonomy:", error);
      vscode.window.showErrorMessage(
        `Failed to generate taxonomy: ${error instanceof Error ? error.message : String(error)}`
      );
      return null;
    }
  }

  async getFileTaxonomies(fileId: string): Promise<TaxonomyNode[]> {
    try {
      return await this.taxonomyClient.getFileTaxonomies(
        this.workspaceId,
        fileId
      );
    } catch (error) {
      console.error("[TaxonomyTreeProvider] Error getting file taxonomies:", error);
      return [];
    }
  }
}

/**
 * Register taxonomy commands
 */
export function registerTaxonomyCommands(
  context: vscode.ExtensionContext,
  provider: TaxonomyTreeProvider
): void {
  context.subscriptions.push(
    // Create new category
    vscode.commands.registerCommand(
      "cortex.taxonomy.createNode",
      async (parentItem?: TaxonomyTreeItem) => {
        const name = await vscode.window.showInputBox({
          prompt: "Enter category name",
          placeHolder: "e.g., Documentation, Reports, etc.",
        });
        if (!name) return;

        const description = await vscode.window.showInputBox({
          prompt: "Enter optional description",
          placeHolder: "Brief description of this category",
        });

        const parentId = parentItem?.node.id;
        const node = await provider.createNode(name, parentId, description);
        if (node) {
          vscode.window.showInformationMessage(`Category "${name}" created`);
        }
      }
    ),

    // Rename category
    vscode.commands.registerCommand(
      "cortex.taxonomy.renameNode",
      async (item: TaxonomyTreeItem) => {
        const newName = await vscode.window.showInputBox({
          prompt: "Enter new name",
          value: item.node.name,
        });
        if (!newName || newName === item.node.name) return;

        const success = await provider.updateNode(item.node.id, newName);
        if (success) {
          vscode.window.showInformationMessage(`Category renamed to "${newName}"`);
        }
      }
    ),

    // Edit description
    vscode.commands.registerCommand(
      "cortex.taxonomy.editDescription",
      async (item: TaxonomyTreeItem) => {
        const newDescription = await vscode.window.showInputBox({
          prompt: "Enter description",
          value: item.node.description || "",
        });
        if (newDescription === undefined) return;

        const success = await provider.updateNode(
          item.node.id,
          undefined,
          newDescription
        );
        if (success) {
          vscode.window.showInformationMessage("Description updated");
        }
      }
    ),

    // Delete category
    vscode.commands.registerCommand(
      "cortex.taxonomy.deleteNode",
      async (item: TaxonomyTreeItem) => {
        const confirm = await vscode.window.showWarningMessage(
          `Delete category "${item.node.name}"? This will also remove all subcategories.`,
          { modal: true },
          "Delete"
        );
        if (confirm !== "Delete") return;

        const success = await provider.deleteNode(item.node.id);
        if (success) {
          vscode.window.showInformationMessage(`Category "${item.node.name}" deleted`);
        }
      }
    ),

    // Generate taxonomy with AI
    vscode.commands.registerCommand("cortex.taxonomy.induce", async () => {
      const confirm = await vscode.window.showInformationMessage(
        "Generate taxonomy categories using AI? This will analyze your files and create a category hierarchy.",
        "Generate",
        "Cancel"
      );
      if (confirm !== "Generate") return;

      await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Generating taxonomy...",
          cancellable: false,
        },
        async () => {
          const result = await provider.induceTaxonomy();
          if (result) {
            vscode.window.showInformationMessage(
              `Taxonomy generated: ${result.nodesCreated} categories created, ${result.filesAssigned} files mapped`
            );
          }
        }
      );
    }),

    // Refresh taxonomy
    vscode.commands.registerCommand("cortex.taxonomy.refresh", () => {
      provider.refresh();
    }),

    // Show files in category (placeholder - would open a quick pick or view)
    vscode.commands.registerCommand(
      "cortex.taxonomy.showFiles",
      async (nodeId: string) => {
        vscode.window.showInformationMessage(
          `Show files in category: ${nodeId} (Not implemented yet)`
        );
      }
    )
  );
}
