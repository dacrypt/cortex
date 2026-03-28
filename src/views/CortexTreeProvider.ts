/**
 * CortexTreeProvider - Hierarchical tree view with grouped sections for better UX.
 *
 * Supports:
 * - Nested groups (sections containing other sections)
 * - Sections with providers (leaf sections that delegate to a TreeDataProvider)
 * - Icons for visual hierarchy
 * - Conditional visibility based on workspace content
 */

import * as vscode from 'vscode';

type TreeItemProvider = vscode.TreeDataProvider<vscode.TreeItem>;

/**
 * Section definition for the Cortex tree.
 * Can be either a group (with children) or a leaf (with provider).
 */
export interface CortexSectionDefinition {
  id: string;
  label: string;
  icon?: vscode.ThemeIcon;
  description?: string;
  /** Provider for leaf sections */
  provider?: TreeItemProvider;
  /** Child sections for group sections */
  children?: CortexSectionDefinition[];
  /** Initial collapsed state. Defaults to Collapsed */
  initialState?: vscode.TreeItemCollapsibleState;
}

/**
 * Tree item representing a section header (group or leaf).
 */
class CortexSectionItem extends vscode.TreeItem {
  constructor(
    public readonly sectionId: string,
    label: string,
    icon?: vscode.ThemeIcon,
    description?: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.Collapsed
  ) {
    super(label, collapsibleState);
    this.contextValue = 'cortex-section';
    this.description = description;
    if (icon) {
      this.iconPath = icon;
    }
  }
}

/**
 * Tree item representing a group header (contains other sections, not a provider).
 */
class CortexGroupItem extends vscode.TreeItem {
  constructor(
    public readonly groupId: string,
    label: string,
    icon?: vscode.ThemeIcon,
    description?: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.Collapsed
  ) {
    super(label, collapsibleState);
    this.contextValue = 'cortex-group';
    this.description = description;
    if (icon) {
      this.iconPath = icon;
    }
  }
}

type CortexTreeElement = CortexSectionItem | CortexGroupItem | vscode.TreeItem;

export class CortexTreeProvider implements vscode.TreeDataProvider<CortexTreeElement> {
  private readonly _onDidChangeTreeData = new vscode.EventEmitter<CortexTreeElement | void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private readonly rootItems: (CortexSectionItem | CortexGroupItem)[];
  private readonly sectionById = new Map<string, CortexSectionDefinition>();
  private readonly itemProviderMap = new WeakMap<vscode.TreeItem, TreeItemProvider>();
  private readonly parentMap = new WeakMap<vscode.TreeItem, vscode.TreeItem | undefined>();
  private readonly subscriptions: vscode.Disposable[] = [];

  constructor(sections: CortexSectionDefinition[]) {
    this.rootItems = [];
    this.registerSections(sections, null);
  }

  /**
   * Recursively register sections and build the tree structure.
   */
  private registerSections(
    sections: CortexSectionDefinition[],
    parent: CortexGroupItem | CortexSectionItem | null
  ): (CortexSectionItem | CortexGroupItem)[] {
    const items: (CortexSectionItem | CortexGroupItem)[] = [];

    for (const section of sections) {
      this.sectionById.set(section.id, section);

      const collapsibleState = section.initialState ?? vscode.TreeItemCollapsibleState.Collapsed;

      if (section.children && section.children.length > 0) {
        // Group section - contains other sections
        const groupItem = new CortexGroupItem(
          section.id,
          section.label,
          section.icon,
          section.description,
          collapsibleState
        );
        items.push(groupItem);

        if (parent) {
          this.parentMap.set(groupItem, parent);
        }

        // Register children recursively
        const childItems = this.registerSections(section.children, groupItem);
        for (const child of childItems) {
          this.parentMap.set(child, groupItem);
        }
      } else if (section.provider) {
        // Leaf section - has a provider
        const sectionItem = new CortexSectionItem(
          section.id,
          section.label,
          section.icon,
          section.description,
          collapsibleState
        );
        items.push(sectionItem);

        if (parent) {
          this.parentMap.set(sectionItem, parent);
        }

        // Subscribe to provider changes
        if (section.provider.onDidChangeTreeData) {
          this.subscriptions.push(
            section.provider.onDidChangeTreeData(() => {
              this._onDidChangeTreeData.fire();
            })
          );
        }
      }
    }

    if (!parent) {
      this.rootItems.push(...items);
    }

    return items;
  }

  dispose(): void {
    vscode.Disposable.from(...this.subscriptions).dispose();
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: CortexTreeElement): vscode.TreeItem {
    return element;
  }

  getParent(element: CortexTreeElement): vscode.ProviderResult<CortexTreeElement> {
    if (element instanceof CortexSectionItem || element instanceof CortexGroupItem) {
      return this.parentMap.get(element);
    }
    return this.parentMap.get(element);
  }

  async getChildren(element?: CortexTreeElement): Promise<CortexTreeElement[]> {
    // Root level - return top-level sections/groups
    if (!element) {
      return this.rootItems;
    }

    // Group item - return child sections
    if (element instanceof CortexGroupItem) {
      return this.getGroupChildren(element);
    }

    // Section item - return provider children
    if (element instanceof CortexSectionItem) {
      return this.getSectionChildren(element);
    }

    // Regular tree item from a provider - delegate to provider
    const provider = this.itemProviderMap.get(element);
    if (!provider) {
      return [];
    }

    const children = await this.getProviderChildren(provider, element);
    for (const child of children) {
      this.parentMap.set(child, element);
    }
    return children;
  }

  private getGroupChildren(element: CortexGroupItem): (CortexSectionItem | CortexGroupItem)[] {
    const section = this.sectionById.get(element.groupId);
    if (!section?.children) {
      return [];
    }

    const childItems: (CortexSectionItem | CortexGroupItem)[] = [];
    for (const childDef of section.children) {
      const childSection = this.sectionById.get(childDef.id);
      if (!childSection) continue;

      const collapsibleState =
        childSection.initialState ?? vscode.TreeItemCollapsibleState.Collapsed;

      const item = this.createSectionOrGroupItem(childSection, collapsibleState);
      if (item) {
        this.parentMap.set(item, element);
        childItems.push(item);
      }
    }

    return childItems;
  }

  private createSectionOrGroupItem(
    section: CortexSectionDefinition,
    collapsibleState: vscode.TreeItemCollapsibleState
  ): CortexSectionItem | CortexGroupItem | null {
    if (section.children && section.children.length > 0) {
      return new CortexGroupItem(
        section.id,
        section.label,
        section.icon,
        section.description,
        collapsibleState
      );
    } else if (section.provider) {
      return new CortexSectionItem(
        section.id,
        section.label,
        section.icon,
        section.description,
        collapsibleState
      );
    }
    return null;
  }

  private async getSectionChildren(element: CortexSectionItem): Promise<vscode.TreeItem[]> {
    const section = this.sectionById.get(element.sectionId);
    if (!section?.provider) {
      return [];
    }
    const children = await this.getProviderChildren(section.provider);
    for (const child of children) {
      this.parentMap.set(child, element);
    }
    return children;
  }

  private async getProviderChildren(
    provider: TreeItemProvider,
    element?: vscode.TreeItem
  ): Promise<vscode.TreeItem[]> {
    const children = (await provider.getChildren(element as never)) ?? [];
    for (const child of children) {
      this.itemProviderMap.set(child, provider);
    }
    return children;
  }
}
