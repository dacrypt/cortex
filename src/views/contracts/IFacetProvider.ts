/**
 * Common interfaces for all facet providers in Cortex
 * 
 * This module defines the unified contract that all facet providers must follow,
 * ensuring consistency across different facet types (terms, numeric ranges, dates, etc.)
 */

import * as vscode from 'vscode';
import { ViewNode, ViewNodeKind } from './viewNodes';

/**
 * Facet type enumeration
 */
export enum FacetType {
  Terms = 'terms',
  NumericRange = 'numeric_range',
  DateRange = 'date_range',
  Structure = 'structure', // For folder-based facets
  Category = 'category', // For category-based facets
  Metrics = 'metrics', // For metrics-based facets
  Issues = 'issues', // For issues-based facets
  Metadata = 'metadata', // For metadata classification facets
}

/**
 * Facet category for organization
 */
export enum FacetCategory {
  Core = 'core',
  Organization = 'organization',
  Temporal = 'temporal',
  Content = 'content',
  System = 'system',
  Specialized = 'specialized',
}

/**
 * Facet configuration - defines how a facet should behave
 */
export interface FacetConfig {
  /** Canonical field name (e.g., 'extension', 'size', 'modified') */
  field: string;
  /** Display label (e.g., 'By Extension', 'By Size') */
  label: string;
  /** Facet type */
  type: FacetType;
  /** Category for organization */
  category: FacetCategory;
  /** Description of what this facet does */
  description?: string;
  /** Icon to use in the tree */
  icon?: vscode.ThemeIcon;
  /** Whether this facet is enabled by default */
  enabled?: boolean;
  /** Aliases for the field name */
  aliases?: string[];
  /** Additional configuration specific to facet type */
  options?: Record<string, unknown>;
}

/**
 * Base payload for facet tree items
 */
export interface FacetItemPayload {
  /** The facet field name */
  facet: string;
  /** The facet value (term, range label, etc.) */
  value: string;
  /** Additional metadata */
  metadata?: Record<string, unknown>;
}

/**
 * Base interface for all facet tree items
 */
export interface IFacetTreeItem extends vscode.TreeItem {
  /** Unique identifier for this item */
  id: string;
  /** Kind of node (facet, file, group, etc.) */
  kind: ViewNodeKind;
  /** Facet field name */
  facet?: string;
  /** Facet value */
  value?: string;
  /** Number of files in this facet value */
  count?: number;
  /** Payload for context actions */
  payload?: FacetItemPayload;
  /** Whether this is a file item */
  isFile?: boolean;
}

/**
 * Base interface for all facet providers
 */
export interface IFacetProvider<T extends IFacetTreeItem = IFacetTreeItem>
  extends vscode.TreeDataProvider<T> {
  /** Facet configuration */
  readonly config: FacetConfig;
  /** Workspace ID */
  readonly workspaceId: string;
  /** Workspace root path */
  readonly workspaceRoot: string;
  /** Refresh the facet data */
  refresh(): void;
  /** Get children for a given item */
  getChildren(element?: T): Promise<T[]>;
  /** Get tree item representation */
  getTreeItem(element: T): vscode.TreeItem;
}

/**
 * Context passed to facet providers during initialization
 */
export interface FacetProviderContext {
  workspaceRoot: string;
  workspaceId: string;
  context: vscode.ExtensionContext;
  fileCacheService?: any; // FileCacheService
  knowledgeClient?: any; // GrpcKnowledgeClient
  adminClient?: any; // GrpcAdminClient
  metadataStore?: any; // IMetadataStore
}

/**
 * Factory function type for creating facet providers
 */
export type FacetProviderFactory<T extends IFacetProvider = IFacetProvider> = (
  config: FacetConfig,
  ctx: FacetProviderContext
) => T;

/**
 * Facet query result from backend
 */
export interface FacetQueryResult {
  field: string;
  type: FacetType;
  data: TermsFacetData | NumericRangeFacetData | DateRangeFacetData;
}

/**
 * Terms facet data
 */
export interface TermsFacetData {
  terms: Array<{ term: string; count: number }>;
}

/**
 * Numeric range facet data
 */
export interface NumericRangeFacetData {
  ranges: Array<{ label: string; count: number; min?: number; max?: number }>;
}

/**
 * Date range facet data
 */
export interface DateRangeFacetData {
  ranges: Array<{
    label: string;
    count: number;
    start_unix?: number;
    end_unix?: number;
  }>;
}


