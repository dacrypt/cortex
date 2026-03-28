/**
 * FacetProviderFactory - Creates facet providers dynamically based on facet configuration
 */

import * as vscode from 'vscode';
import {
  FacetConfig,
  FacetType,
  FacetProviderContext,
  IFacetProvider,
} from '../contracts/IFacetProvider';
import { TermsFacetTreeProvider } from '../TermsFacetTreeProvider';
import { NumericRangeFacetTreeProvider } from '../NumericRangeFacetTreeProvider';
import { DateRangeFacetTreeProvider } from '../DateRangeFacetTreeProvider';
import { FolderTreeProvider } from '../FolderTreeProvider';
import { CategoryFacetTreeProvider } from '../CategoryFacetTreeProvider';
import { MetricsFacetTreeProvider } from '../MetricsFacetTreeProvider';
import { ClusterFacetTreeProvider } from '../ClusterFacetTreeProvider';

/**
 * Factory for creating facet providers based on facet configuration
 */
export class FacetProviderFactory {
  /**
   * Create a facet provider for the given configuration
   */
  static create(
    config: FacetConfig,
    ctx: FacetProviderContext
  ): IFacetProvider {
    // Special case for cluster facet - uses dedicated provider
    if (config.field === 'cluster' || config.field === 'document_cluster') {
      return new ClusterFacetTreeProvider(
        ctx.workspaceRoot,
        ctx.context,
        ctx.workspaceId
      ) as any;
    }

    switch (config.type) {
      case FacetType.Terms:
        return new TermsFacetTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId,
          config.field,
          ctx.metadataStore
        ) as any;

      case FacetType.NumericRange:
        return new NumericRangeFacetTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId,
          config.field
        ) as any;

      case FacetType.DateRange:
        return new DateRangeFacetTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId,
          config.field
        ) as any;

      case FacetType.Structure:
        return new FolderTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId
        ) as any;

      case FacetType.Category:
        return new CategoryFacetTreeProvider(config, ctx);

      case FacetType.Metrics:
        return new MetricsFacetTreeProvider(config, ctx);

      case FacetType.Issues:
        return new TermsFacetTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId,
          'issue_type',
          ctx.metadataStore
        ) as any;

      case FacetType.Metadata:
        return new TermsFacetTreeProvider(
          ctx.workspaceRoot,
          ctx.context,
          ctx.workspaceId,
          config.field,
          ctx.metadataStore
        ) as any;

      default:
        throw new Error(`Unknown facet type: ${config.type} for field: ${config.field}`);
    }
  }

  /**
   * Check if a facet type is supported
   */
  static isSupported(type: FacetType): boolean {
    return [
      FacetType.Terms,
      FacetType.NumericRange,
      FacetType.DateRange,
      FacetType.Structure,
      FacetType.Category,
      FacetType.Metrics,
      FacetType.Issues,
      FacetType.Metadata,
    ].includes(type);
  }
}

