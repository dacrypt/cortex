/**
 * CategoryFacetTreeProvider - Displays project category facets
 *
 * Groups projects by category (writing, collection, development, management, hierarchical)
 * and shows files associated with each project.
 */

import * as vscode from 'vscode';
import { BaseFacetTreeProvider } from './base/BaseFacetTreeProvider';
import { FacetItemPayload, IFacetTreeItem } from './contracts/IFacetProvider';
import { ViewNodeKind } from './contracts/viewNodes';
import type { Project } from '../core/GrpcKnowledgeClient';

type CategoryMatchRule = {
  tokens: string[];
};

const CATEGORY_RULES: Record<string, CategoryMatchRule> = {
  writing_category: { tokens: ['writing'] },
  collection_category: { tokens: ['collection'] },
  development_category: { tokens: ['development', 'dev'] },
  management_category: { tokens: ['management', 'manage'] },
  hierarchical_category: { tokens: ['hierarchical', 'hierarchy'] },
};

class CategoryFacetTreeItem extends vscode.TreeItem implements IFacetTreeItem {
  id: string;
  kind: ViewNodeKind;
  facet?: string;
  value?: string;
  count?: number;
  payload?: FacetItemPayload;
  isFile?: boolean;
  projectId?: string;
  isUnknownGroup?: boolean;

  constructor(params: {
    id: string;
    kind: ViewNodeKind;
    label: string;
    collapsibleState: vscode.TreeItemCollapsibleState;
    resourceUri?: vscode.Uri;
    isFile?: boolean;
    facet?: string;
    value?: string;
    count?: number;
    payload?: FacetItemPayload;
    icon?: vscode.ThemeIcon;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    contextValue?: string;
    projectId?: string;
    isUnknownGroup?: boolean;
  }) {
    super(params.label, params.collapsibleState);

    this.id = params.id;
    this.kind = params.kind;
    this.facet = params.facet;
    this.value = params.value;
    this.count = params.count;
    this.payload = params.payload;
    this.isFile = params.isFile;
    this.projectId = params.projectId;
    this.isUnknownGroup = params.isUnknownGroup;

    if (params.description) this.description = params.description;
    if (params.tooltip) this.tooltip = params.tooltip;
    if (params.icon) this.iconPath = params.icon;
    if (params.contextValue) this.contextValue = params.contextValue;

    if (params.isFile && params.resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [params.resourceUri],
      };
      if (!this.iconPath) this.iconPath = vscode.ThemeIcon.File;
      if (!this.contextValue) this.contextValue = 'cortex-file';
    } else if (params.kind === 'facet' && !this.iconPath) {
      this.iconPath = new vscode.ThemeIcon('folder');
      if (!this.contextValue) this.contextValue = 'cortex-facet-term';
    }
  }
}

export class CategoryFacetTreeProvider extends BaseFacetTreeProvider<CategoryFacetTreeItem> {
  async getChildren(element?: CategoryFacetTreeItem): Promise<CategoryFacetTreeItem[]> {
    if (!element) {
      return await this.getCategoryProjects();
    }
    if (element.isUnknownGroup) {
      return await this.getUncategorizedProjects();
    }
    if (element.projectId) {
      return await this.getFilesForProject(element.projectId);
    }
    return [];
  }

  protected createTreeItem(params: {
    id: string;
    kind: ViewNodeKind;
    label: string;
    collapsibleState: vscode.TreeItemCollapsibleState;
    resourceUri?: vscode.Uri;
    isFile?: boolean;
    facet?: string;
    value?: string;
    count?: number;
    payload?: FacetItemPayload;
    description?: string;
    tooltip?: string | vscode.MarkdownString;
    icon?: vscode.ThemeIcon;
    projectId?: string;
    isUnknownGroup?: boolean;
  }): CategoryFacetTreeItem {
    return new CategoryFacetTreeItem(params);
  }

  private async getCategoryProjects(): Promise<CategoryFacetTreeItem[]> {
    if (!this.knowledgeClient) {
      return [this.createEmptyPlaceholder('Projects unavailable')];
    }

    try {
      return await this.getCached(`projects:${this.config.field}`, async () => {
        const projects: Project[] = await this.knowledgeClient.listProjects(this.workspaceId);
        const filtered = this.filterProjectsByCategory(projects);
        const filteredIds = new Set(filtered.map((project) => project.id));

        if (filtered.length === 0) {
          return [this.createEmptyPlaceholder(`No projects found for ${this.config.label}`)];
        }

        const includeSubprojects = this.config.field === 'hierarchical_category';
        const counts = await Promise.all(
          projects.map(async (project: Project) => {
            try {
              const docs = await this.knowledgeClient.queryDocuments(
                this.workspaceId,
                project.id,
                includeSubprojects
              );
              return { project, count: docs.length };
            } catch {
              return { project, count: 0 };
            }
          })
        );

        const filteredCounts = counts.filter(({ project }) => filteredIds.has(project.id));
        const missingCounts = counts.filter(({ project }) => !filteredIds.has(project.id));
        const missingFileCount = missingCounts.reduce((acc, entry) => acc + entry.count, 0);

        filteredCounts.sort((a, b) => a.project.name.localeCompare(b.project.name));
        const items: CategoryFacetTreeItem[] = [];

        if (missingCounts.length > 0) {
          const label = this.formatLabelWithCount('?', missingFileCount);
          items.push(
            this.createTreeItem({
              id: this.generateFacetId('?'),
              kind: 'facet',
              label,
              collapsibleState: vscode.TreeItemCollapsibleState.Collapsed,
              facet: this.config.field,
              value: '?',
              count: missingFileCount,
              icon: new vscode.ThemeIcon('question'),
              isUnknownGroup: true,
              tooltip: `Proyectos sin categoría\n${missingFileCount} files`,
            })
          );
        }

        for (const { project, count } of filteredCounts) {
          const value = project.name;
          const label = this.formatLabelWithCount(value, count);
          items.push(this.createTreeItem({
            id: this.generateFacetId(value),
            kind: 'facet',
            label,
            collapsibleState: vscode.TreeItemCollapsibleState.Collapsed,
            facet: this.config.field,
            value,
            count,
            icon: new vscode.ThemeIcon('folder'),
            payload: {
              facet: this.config.field,
              value,
              metadata: { projectId: project.id },
            },
            projectId: project.id,
            tooltip: `${value}\n${count} files`,
          }));
        }

        return items;
      });
    } catch (error) {
      return [this.createErrorItem(error as Error, this.config.field)];
    }
  }

  private async getUncategorizedProjects(): Promise<CategoryFacetTreeItem[]> {
    if (!this.knowledgeClient) {
      return [this.createEmptyPlaceholder('Projects unavailable')];
    }
    try {
      const projects: Project[] = await this.knowledgeClient.listProjects(this.workspaceId);
      const filtered = this.filterProjectsByCategory(projects);
      const filteredIds = new Set(filtered.map((project) => project.id));
      const includeSubprojects = this.config.field === 'hierarchical_category';

      const missingProjects = projects.filter((project: Project) => !filteredIds.has(project.id));
      if (missingProjects.length === 0) {
        return [this.createEmptyPlaceholder('No files found')];
      }

      const counts = await Promise.all(
        missingProjects.map(async (project: Project) => {
          try {
            const docs = await this.knowledgeClient.queryDocuments(
              this.workspaceId,
              project.id,
              includeSubprojects
            );
            return { project, count: docs.length };
          } catch {
            return { project, count: 0 };
          }
        })
      );

      counts.sort((a, b) => a.project.name.localeCompare(b.project.name));

      return counts.map(({ project, count }) => {
        const value = project.name;
        const label = this.formatLabelWithCount(value, count);
        return this.createTreeItem({
          id: this.generateFacetId(value),
          kind: 'facet',
          label,
          collapsibleState: vscode.TreeItemCollapsibleState.Collapsed,
          facet: this.config.field,
          value,
          count,
          icon: new vscode.ThemeIcon('folder'),
          payload: {
            facet: this.config.field,
            value,
            metadata: { projectId: project.id },
          },
          projectId: project.id,
          tooltip: `${value}\n${count} files`,
        });
      });
    } catch (error) {
      return [this.createErrorItem(error as Error, this.config.field)];
    }
  }

  private async getFilesForProject(projectId: string): Promise<CategoryFacetTreeItem[]> {
    if (!this.knowledgeClient) {
      return [this.createEmptyPlaceholder('Projects unavailable')];
    }
    try {
      const includeSubprojects = this.config.field === 'hierarchical_category';
      const documents = await this.knowledgeClient.queryDocuments(
        this.workspaceId,
        projectId,
        includeSubprojects
      );
      if (!documents || documents.length === 0) {
        return [this.createEmptyPlaceholder('No files found')];
      }

      return documents.map((doc: { path: string }) => {
        const relativePath = doc.path;
        return this.createTreeItem({
          id: this.generateFileId(relativePath, projectId),
          kind: 'file',
          label: this.getFilename(relativePath),
          collapsibleState: vscode.TreeItemCollapsibleState.None,
          resourceUri: this.getFileUri(relativePath),
          isFile: true,
          facet: this.config.field,
          payload: {
            facet: this.config.field,
            value: relativePath,
            metadata: { projectId },
          },
          description: this.getDirname(relativePath),
          tooltip: relativePath,
        });
      });
    } catch (error) {
      return [this.createErrorItem(error as Error, projectId)];
    }
  }

  private filterProjectsByCategory(projects: Project[]): Project[] {
    const rule = CATEGORY_RULES[this.config.field];
    if (!rule) {
      return projects;
    }

    if (this.config.field === 'hierarchical_category') {
      const parentIds = new Set(
        projects
          .map((project) => project.parent_id)
          .filter((id): id is string => Boolean(id))
      );
      return projects.filter((project) => {
        const tokens = this.getProjectTokens(project);
        return (
          Boolean(project.parent_id) ||
          parentIds.has(project.id) ||
          rule.tokens.some((token) => tokens.includes(token))
        );
      });
    }

    return projects.filter((project) => {
      const tokens = this.getProjectTokens(project);
      return rule.tokens.some((token) => tokens.includes(token));
    });
  }

  private getProjectTokens(project: Project): string[] {
    const tokens = new Set<string>();

    if (project.nature) {
      tokens.add(this.normalizeToken(project.nature));
    }

    if (project.attributes) {
      try {
        const parsed = JSON.parse(project.attributes);
        this.extractTokensFromAttributes(parsed).forEach((token) =>
          tokens.add(this.normalizeToken(token))
        );
      } catch {
        // Ignore malformed attributes
      }
    }

    return Array.from(tokens.values()).filter(Boolean);
  }

  private extractTokensFromAttributes(value: unknown): string[] {
    if (typeof value === 'string') {
      return [value];
    }
    if (Array.isArray(value)) {
      return value.flatMap((entry) => this.extractTokensFromAttributes(entry));
    }
    if (value && typeof value === 'object') {
      const record = value as Record<string, unknown>;
      const tokens: string[] = [];
      const keys = [
        'category',
        'categories',
        'nature',
        'type',
        'project_category',
        'project_nature',
        'project_type',
        'tags',
        'tag',
        'group',
      ];
      for (const key of keys) {
        if (record[key] !== undefined) {
          tokens.push(...this.extractTokensFromAttributes(record[key]));
        }
      }
      return tokens;
    }
    return [];
  }

  private normalizeToken(value: string): string {
    return value.trim().toLowerCase();
  }
}
