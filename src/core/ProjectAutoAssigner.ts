/**
 * Auto-assign projects (contexts) based on time + folder clusters.
 */

import * as path from 'path';
import { IndexStore } from './IndexStore';
import { IMetadataStore } from './IMetadataStore';
import { FileIndexEntry } from '../models/types';

export interface ProjectAutoAssignOptions {
  windowMs?: number;
  minClusterSize?: number;
  dominanceThreshold?: number;
  suggestionThreshold?: number;
}

export interface ProjectAutoAssignResult {
  clustersEvaluated: number;
  assignedCount: number;
  filesUpdated: number;
}

const DEFAULT_OPTIONS: Required<ProjectAutoAssignOptions> = {
  windowMs: 6 * 60 * 60 * 1000,
  minClusterSize: 2,
  dominanceThreshold: 0.6,
  suggestionThreshold: 0.3,
};

export class ProjectAutoAssigner {
  private options: Required<ProjectAutoAssignOptions>;

  constructor(
    private metadataStore: IMetadataStore,
    private indexStore: IndexStore,
    options?: ProjectAutoAssignOptions
  ) {
    this.options = { ...DEFAULT_OPTIONS, ...options };
    if (this.options.suggestionThreshold > this.options.dominanceThreshold) {
      this.options.suggestionThreshold = this.options.dominanceThreshold;
    }
  }

  assignProjects(
    files?: FileIndexEntry[],
    shouldSkip?: (file: FileIndexEntry) => boolean
  ): ProjectAutoAssignResult {
    const candidates = (files ?? this.indexStore.getAllFiles()).filter(
      (file) => !shouldSkip?.(file)
    );

    const clusters = this.buildClusters(candidates);
    let assignedCount = 0;
    const updatedFiles = new Set<string>();

    for (const clusterFiles of clusters.values()) {
      if (clusterFiles.length < this.options.minClusterSize) {
        continue;
      }

      const contextsByFile = new Map<string, string[]>();
      const filesWithContexts = new Set<string>();
      const contextToFiles = new Map<string, Set<string>>();

      for (const file of clusterFiles) {
        const metadata =
          this.metadataStore.getMetadataByPath(file.relativePath) ??
          this.metadataStore.getOrCreateMetadata(
            file.relativePath,
            file.extension
          );
        const contexts = metadata.contexts ?? [];
        contextsByFile.set(file.relativePath, contexts);
        if (contexts.length > 0) {
          filesWithContexts.add(file.relativePath);
        }
        for (const context of contexts) {
          let set = contextToFiles.get(context);
          if (!set) {
            set = new Set<string>();
            contextToFiles.set(context, set);
          }
          set.add(file.relativePath);
        }
      }

      if (contextToFiles.size === 0) {
        continue;
      }

      const [dominantContext, dominantCount] =
        this.getDominantContext(contextToFiles);
      if (!dominantContext) {
        continue;
      }

      const dominance =
        filesWithContexts.size === 0
          ? 0
          : dominantCount / filesWithContexts.size;
      for (const file of clusterFiles) {
        if (shouldSkip?.(file)) {
          continue;
        }
        const existing = contextsByFile.get(file.relativePath) ?? [];
        if (existing.length > 0) {
          continue;
        }
        if (dominance >= this.options.dominanceThreshold) {
          this.metadataStore.addContext(file.relativePath, dominantContext);
          this.metadataStore.clearSuggestedContexts(file.relativePath);
          assignedCount += 1;
          updatedFiles.add(file.relativePath);
          continue;
        }
        if (dominance >= this.options.suggestionThreshold) {
          this.metadataStore.addSuggestedContext(
            file.relativePath,
            dominantContext
          );
          updatedFiles.add(file.relativePath);
        }
      }
    }

    return {
      clustersEvaluated: clusters.size,
      assignedCount,
      filesUpdated: updatedFiles.size,
    };
  }

  private buildClusters(files: FileIndexEntry[]): Map<string, FileIndexEntry[]> {
    const clusters = new Map<string, FileIndexEntry[]>();

    for (const file of files) {
      const activityTime = this.getActivityTime(file);
      if (!activityTime) {
        continue;
      }
      const bucket = Math.floor(activityTime / this.options.windowMs);
      const folder = file.enhanced?.folder ?? path.dirname(file.relativePath);
      const key = `${bucket}|${folder}`;
      const group = clusters.get(key);
      if (group) {
        group.push(file);
      } else {
        clusters.set(key, [file]);
      }
    }

    return clusters;
  }

  private getActivityTime(file: FileIndexEntry): number {
    const stats = file.enhanced?.stats;
    const created = stats?.created ?? 0;
    const modified = stats?.modified ?? 0;
    const indexedModified = file.lastModified ?? 0;
    return Math.max(created, modified, indexedModified);
  }

  private getDominantContext(
    contextToFiles: Map<string, Set<string>>
  ): [string | null, number] {
    let bestContext: string | null = null;
    let bestCount = 0;
    for (const [context, files] of contextToFiles.entries()) {
      if (files.size > bestCount) {
        bestContext = context;
        bestCount = files.size;
      }
    }
    return [bestContext, bestCount];
  }
}
