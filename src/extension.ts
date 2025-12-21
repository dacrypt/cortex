/**
 * Cortex Extension - Main entry point
 */

import * as vscode from "vscode";
import * as path from "path";
import * as os from "os";
import * as fs from "fs/promises";

// Core components
import { FileScanner } from "./core/FileScanner";
import { IndexStore } from "./core/IndexStore";
import { MetadataStore as SQLiteMetadataStore } from "./core/MetadataStore";
import { MetadataStoreJSON } from "./core/MetadataStoreJSON";
import { IMetadataStore } from "./core/IMetadataStore";
import { IndexingStatus } from "./core/IndexingStatus";
import { IndexCache } from "./core/IndexCache";
import { FileIndexEntry } from "./models/types";
import { FileMetadata, MirrorMetadata } from "./models/types";
import { BlacklistStore } from "./core/BlacklistStore";
import { WorkerPool } from "./core/WorkerPool";
import { ProjectAutoAssigner } from "./core/ProjectAutoAssigner";
import { MirrorStore } from "./core/MirrorStore";
import { isOsTaggingSupported } from "./utils/osTags";
import { syncStoreTagsFromOs } from "./utils/tagSync";
import {
  extractKeyTerms,
  isLikelyTextExtension,
} from "./utils/aiSummary";
import { resolveAIContent } from "./utils/aiContent";
import { LLMService } from "./services/LLMService";
import { saveAISummaryToFile } from "./utils/saveAISummary";

// View providers
import { ContextTreeProvider } from "./views/ContextTreeProvider";
import { TagTreeProvider } from "./views/TagTreeProvider";
import { TypeTreeProvider } from "./views/TypeTreeProvider";
import { DateTreeProvider } from "./views/DateTreeProvider";
import { SizeTreeProvider } from "./views/SizeTreeProvider";
import { FolderTreeProvider } from "./views/FolderTreeProvider";
import { ContentTypeTreeProvider } from "./views/ContentTypeTreeProvider";
import { CodeMetricsTreeProvider } from "./views/CodeMetricsTreeProvider";
import { DocumentMetricsTreeProvider } from "./views/DocumentMetricsTreeProvider";
import { IssuesTreeProvider } from "./views/IssuesTreeProvider";
import { FileInfoTreeProvider } from "./views/FileInfoTreeProvider";
import { CategoryTreeProvider } from "./views/CategoryTreeProvider";

// Metadata extraction
import { MetadataExtractor } from "./extractors/MetadataExtractor";
import { EnhancedMetadata } from "./extractors/MetadataExtractor";

// Commands
import { addTagCommand } from "./commands/addTag";
import { assignContextCommand } from "./commands/assignContext";
import { openViewCommand } from "./commands/openView";
import { rebuildIndexCommand } from "./commands/rebuildIndex";
import { suggestTagsAI } from "./commands/suggestTagsAI";
import { suggestProjectAI } from "./commands/suggestProjectAI";
import { generateSummaryAI } from "./commands/generateSummaryAI";

type IndexerTask = {
  type: "basic" | "mime" | "code" | "document";
  workspaceRoot: string;
  file: {
    absolutePath: string;
    relativePath: string;
    extension: string;
    enhanced: EnhancedMetadata;
  };
};

type AccordionTreeProviderAny = {
  setAccordionEnabled(enabled: boolean): void;
  expandAll(): void;
  collapseAll(): void;
  handleDidExpand(element: unknown): void;
  handleDidCollapse(element: unknown): void;
};

async function syncOsTagsForFiles(
  files: FileIndexEntry[],
  metadataStore: IMetadataStore,
  onMetadataChanged: () => void
): Promise<void> {
  if (!isOsTaggingSupported()) {
    return;
  }

  const batchSize = 50;
  for (let i = 0; i < files.length; i += batchSize) {
    const batch = files.slice(i, i + batchSize);
    const results = await Promise.all(
      batch.map(async (file) => {
        try {
          return await syncStoreTagsFromOs(
            metadataStore,
            file.relativePath,
            file.absolutePath
          );
        } catch (error) {
          console.warn(
            `[Cortex] Failed to sync OS tags for ${file.relativePath}:`,
            error
          );
          return false;
        }
      })
    );

    if (results.some(Boolean)) {
      onMetadataChanged();
    }
  }
}

/**
 * Background indexing for basic metadata (stats, folder, language)
 */
async function startBasicIndexing(
  indexStore: IndexStore,
  metadataExtractor: MetadataExtractor,
  updateIndexingStatus: (patch: Partial<IndexingStatus>) => void,
  filesToProcess?: FileIndexEntry[],
  onBatchComplete?: () => void,
  workerPool?: WorkerPool<IndexerTask>,
  workspaceRoot?: string
): Promise<void> {
  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Extracting file metadata",
      cancellable: false,
    },
    async (progress) => {
      const files = filesToProcess || indexStore.getAllFiles();
      const targetFiles = files.filter(
        (file) => !file.enhanced?.indexed?.basic
      );
      console.log(
        `[Cortex] Starting basic metadata extraction for ${targetFiles.length} files...`
      );

      updateIndexingStatus({
        phase: "basic",
        message: "Metadata basica",
        processed: 0,
        total: targetFiles.length,
        isIndexing: true,
      });

      if (targetFiles.length === 0) {
        updateIndexingStatus({
          phase: "done",
          message: "Metadata basica lista",
          processed: 0,
          total: 0,
          isIndexing: false,
        });
        return;
      }

      const batchSize = 100;
      let processed = 0;

      for (let i = 0; i < targetFiles.length; i += batchSize) {
        const batch = targetFiles.slice(i, i + batchSize);
        if (batch.length === 0) {
          continue;
        }

        if (workerPool && workspaceRoot) {
          await Promise.all(
            batch.map(async (file) => {
              try {
                const result = await workerPool.runTask({
                  type: "basic",
                  workspaceRoot,
                  file: {
                    absolutePath: file.absolutePath,
                    relativePath: file.relativePath,
                    extension: file.extension,
                    enhanced: file.enhanced || {
                      stats: {
                        size: 0,
                        created: 0,
                        modified: 0,
                        accessed: 0,
                        isReadOnly: false,
                        isHidden: false,
                      },
                      folder: ".",
                      depth: 0,
                    },
                  },
                });
                if (
                  result &&
                  typeof result === "object" &&
                  "enhanced" in result
                ) {
                  file.enhanced = (
                    result as { enhanced: EnhancedMetadata }
                  ).enhanced;
                }
              } catch (error) {
                console.error(
                  `[Cortex] Basic metadata failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        } else {
          await Promise.all(
            batch.map(async (file) => {
              try {
                const enhanced = await metadataExtractor.extractBasic(
                  file.absolutePath,
                  file.relativePath,
                  file.extension
                );
                file.enhanced = enhanced;
              } catch (error) {
                console.error(
                  `[Cortex] Basic metadata failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        }

        processed += batch.length;
        progress.report({
          message: `${processed}/${targetFiles.length} files`,
          increment: (batch.length / targetFiles.length) * 100,
        });

        updateIndexingStatus({
          phase: "basic",
          message: "Metadata basica",
          processed,
          total: targetFiles.length,
          isIndexing: true,
        });

        onBatchComplete?.();
      }

      onBatchComplete?.();
      console.log(`[Cortex] ✓ Basic metadata complete`);
      updateIndexingStatus({
        phase: "done",
        message: "Metadata basica lista",
        processed: targetFiles.length,
        total: targetFiles.length,
        isIndexing: false,
      });
    }
  );
}

/**
 * Background indexing for ContentType view
 */
async function startContentTypeIndexing(
  workspaceRoot: string,
  indexStore: IndexStore,
  metadataExtractor: MetadataExtractor,
  provider: ContentTypeTreeProvider,
  updateIndexingStatus: (patch: Partial<IndexingStatus>) => void,
  filesToProcess?: FileIndexEntry[],
  workerPool?: WorkerPool<IndexerTask>,
  onBatchComplete?: () => void
): Promise<void> {
  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Analyzing content types",
      cancellable: false,
    },
    async (progress) => {
      const files = filesToProcess || indexStore.getAllFiles();
      const targetFiles = files.filter((file) => !file.enhanced?.indexed?.mime);
      console.log(
        `[Cortex] Starting MIME type extraction for ${targetFiles.length} files...`
      );

      updateIndexingStatus({
        phase: "contentTypes",
        message: "Tipos de contenido",
        processed: 0,
        total: targetFiles.length,
        isIndexing: true,
      });

      if (targetFiles.length === 0) {
        updateIndexingStatus({
          phase: "done",
          message: "Tipos de contenido listos",
          processed: 0,
          total: 0,
          isIndexing: false,
        });
        return;
      }

      const batchSize = 200;
      const reportEvery = 500;
      let processed = 0;

      for (let i = 0; i < targetFiles.length; i += batchSize) {
        const batch = targetFiles.slice(i, i + batchSize);
        if (batch.length === 0) {
          continue;
        }

        if (workerPool) {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;
              try {
                const result = await workerPool.runTask({
                  type: "mime",
                  workspaceRoot,
                  file: {
                    absolutePath: file.absolutePath,
                    relativePath: file.relativePath,
                    extension: file.extension,
                    enhanced: file.enhanced,
                  },
                });
                if (
                  result &&
                  typeof result === "object" &&
                  "enhanced" in result
                ) {
                  file.enhanced = (
                    result as { enhanced: EnhancedMetadata }
                  ).enhanced;
                }
              } catch (error) {
                console.error(
                  `[Cortex] Content type extraction failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        } else {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;
              await metadataExtractor.extractMimeTypeMetadata(
                file.absolutePath,
                file.enhanced
              );
            })
          );
        }

        processed += batch.length;
        const shouldReport =
          processed === targetFiles.length || processed % reportEvery === 0;

        if (shouldReport) {
          progress.report({
            message: `${processed}/${targetFiles.length} files`,
            increment: (batch.length / targetFiles.length) * 100,
          });
          updateIndexingStatus({
            phase: "contentTypes",
            message: "Tipos de contenido",
            processed,
            total: targetFiles.length,
            isIndexing: true,
          });
        }

        if (processed % 500 === 0) {
          provider.refresh();
        }

        onBatchComplete?.();
      }

      onBatchComplete?.();
      provider.refresh();
      console.log(`[Cortex] ✓ Content type analysis complete`);
      updateIndexingStatus({
        phase: "done",
        message: "Tipos de contenido listos",
        processed: targetFiles.length,
        total: targetFiles.length,
        isIndexing: false,
      });
    }
  );
}

/**
 * Background indexing for Documents view
 */
async function startDocumentIndexing(
  workspaceRoot: string,
  indexStore: IndexStore,
  metadataExtractor: MetadataExtractor,
  mirrorStore: MirrorStore,
  updateMirrorTracking: (
    file: FileIndexEntry,
    mirrorPath: string,
    sourceMtime: number
  ) => void,
  provider: DocumentMetricsTreeProvider,
  updateIndexingStatus: (patch: Partial<IndexingStatus>) => void,
  filesToProcess?: FileIndexEntry[],
  workerPool?: WorkerPool<IndexerTask>,
  onBatchComplete?: () => void
): Promise<void> {
  const files = filesToProcess || indexStore.getAllFiles();

  // Filter Office and design files
  const documentExts = [
    ".docx",
    ".doc",
    ".xlsx",
    ".xls",
    ".pptx",
    ".ppt",
    ".pdf",
    ".psd",
  ];
  const documentFiles = files.filter(
    (f) =>
      documentExts.includes(f.extension.toLowerCase()) &&
      !f.enhanced?.indexed?.document
  );

  if (documentFiles.length === 0) {
    console.log(`[Cortex] No document files found`);
    updateIndexingStatus({
      phase: "done",
      message: "Sin documentos",
      processed: 0,
      total: 0,
      isIndexing: false,
    });
    return;
  }

  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: `Cortex: Analyzing ${documentFiles.length} documents`,
      cancellable: false,
    },
    async (progress) => {
      console.log(
        `[Cortex] Extracting document metadata for ${documentFiles.length} files...`
      );

      updateIndexingStatus({
        phase: "documents",
        message: "Documentos",
        processed: 0,
        total: documentFiles.length,
        isIndexing: true,
      });

      const batchSize = 10;
      const reportEvery = 50;
      let processed = 0;
      let successful = 0;

      for (let i = 0; i < documentFiles.length; i += batchSize) {
        const batch = documentFiles
          .slice(i, i + batchSize)
          .filter((file) => !file.enhanced?.indexed?.document);
        if (batch.length === 0) {
          continue;
        }

        if (workerPool) {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;
              try {
                const result = await workerPool.runTask({
                  type: "document",
                  workspaceRoot,
                  file: {
                    absolutePath: file.absolutePath,
                    relativePath: file.relativePath,
                    extension: file.extension,
                    enhanced: file.enhanced,
                  },
                });
                if (
                  result &&
                  typeof result === "object" &&
                  "enhanced" in result
                ) {
                  file.enhanced = (
                    result as { enhanced: EnhancedMetadata }
                  ).enhanced;
                  if (
                    file.enhanced.documentMetadata ||
                    file.enhanced.designMetadata
                  ) {
                    successful++;
                  }
                }
                if (mirrorStore.isMirrorableExtension(file.extension)) {
                  const mirrorPath = mirrorStore.getMirrorPath(
                    file.relativePath,
                    file.extension
                  );
                  const content = await mirrorStore.ensureMirrorContent(
                    file.absolutePath,
                    file.relativePath,
                    file.extension
                  );
                  if (content != null && mirrorPath) {
                    updateMirrorTracking(
                      file,
                      mirrorPath,
                      file.lastModified
                    );
                  }
                }
              } catch (error) {
                console.error(
                  `[Cortex] Document extraction failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        } else {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;

              try {
                await metadataExtractor.extractDocumentMetadata(
                  file.absolutePath,
                  file.enhanced,
                  file.extension
                );

                if (
                  file.enhanced.documentMetadata ||
                  file.enhanced.designMetadata
                ) {
                  successful++;
                }
                if (mirrorStore.isMirrorableExtension(file.extension)) {
                  const mirrorPath = mirrorStore.getMirrorPath(
                    file.relativePath,
                    file.extension
                  );
                  const content = await mirrorStore.ensureMirrorContent(
                    file.absolutePath,
                    file.relativePath,
                    file.extension
                  );
                  if (content != null && mirrorPath) {
                    updateMirrorTracking(
                      file,
                      mirrorPath,
                      file.lastModified
                    );
                  }
                }
              } catch (error) {
                console.error(
                  `[Cortex] Document extraction failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        }

        processed += batch.length;
        const shouldReport =
          processed === documentFiles.length || processed % reportEvery === 0;

        if (shouldReport) {
          progress.report({
            message: `${processed}/${documentFiles.length} files (${successful} extracted)`,
            increment: (batch.length / documentFiles.length) * 100,
          });
          updateIndexingStatus({
            phase: "documents",
            message: "Documentos",
            processed,
            total: documentFiles.length,
            isIndexing: true,
          });
        }

        if (processed % 20 === 0) {
          provider.refresh();
        }

        onBatchComplete?.();
      }

      onBatchComplete?.();
      provider.refresh();
      console.log(
        `[Cortex] ✓ Document analysis complete (${successful}/${documentFiles.length} successful)`
      );
      updateIndexingStatus({
        phase: "done",
        message: "Documentos listos",
        processed: documentFiles.length,
        total: documentFiles.length,
        isIndexing: false,
      });
    }
  );
}

/**
 * Background indexing for CodeMetrics view
 */
async function startCodeMetricsIndexing(
  workspaceRoot: string,
  indexStore: IndexStore,
  metadataExtractor: MetadataExtractor,
  provider: CodeMetricsTreeProvider,
  updateIndexingStatus: (patch: Partial<IndexingStatus>) => void,
  filesToProcess?: FileIndexEntry[],
  workerPool?: WorkerPool<IndexerTask>,
  onBatchComplete?: () => void
): Promise<void> {
  const files = filesToProcess || indexStore.getAllFiles();
  const codeFiles = files.filter(
    (f) => f.enhanced?.language && !f.enhanced?.indexed?.code
  );

  if (codeFiles.length === 0) {
    console.log(`[Cortex] No code files found`);
    updateIndexingStatus({
      phase: "done",
      message: "Sin codigo",
      processed: 0,
      total: 0,
      isIndexing: false,
    });
    return;
  }

  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: `Cortex: Analyzing ${codeFiles.length} code files`,
      cancellable: false,
    },
    async (progress) => {
      console.log(
        `[Cortex] Extracting code metrics for ${codeFiles.length} files...`
      );

      updateIndexingStatus({
        phase: "code",
        message: "Metricas de codigo",
        processed: 0,
        total: codeFiles.length,
        isIndexing: true,
      });

      const batchSize = 25;
      const reportEvery = 250;
      let processed = 0;
      let successful = 0;

      for (let i = 0; i < codeFiles.length; i += batchSize) {
        const batch = codeFiles
          .slice(i, i + batchSize)
          .filter((file) => !file.enhanced?.indexed?.code);
        if (batch.length === 0) {
          continue;
        }

        if (workerPool) {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;

              try {
                const result = await workerPool.runTask({
                  type: "code",
                  workspaceRoot,
                  file: {
                    absolutePath: file.absolutePath,
                    relativePath: file.relativePath,
                    extension: file.extension,
                    enhanced: file.enhanced,
                  },
                });
                if (
                  result &&
                  typeof result === "object" &&
                  "enhanced" in result
                ) {
                  file.enhanced = (
                    result as { enhanced: EnhancedMetadata }
                  ).enhanced;
                  if (file.enhanced.codeMetadata) {
                    successful++;
                  }
                }
              } catch (error) {
                console.error(
                  `[Cortex] Code metrics failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        } else {
          await Promise.all(
            batch.map(async (file) => {
              if (!file.enhanced) return;

              try {
                await metadataExtractor.extractCodeMetadata(
                  file.absolutePath,
                  file.enhanced
                );

                if (file.enhanced.codeMetadata) {
                  successful++;
                }
              } catch (error) {
                console.error(
                  `[Cortex] Code metrics failed for ${file.filename}:`,
                  error
                );
              }
            })
          );
        }

        processed += batch.length;
        const shouldReport =
          processed === codeFiles.length || processed % reportEvery === 0;

        if (shouldReport) {
          progress.report({
            message: `${processed}/${codeFiles.length} files (${successful} analyzed)`,
            increment: (batch.length / codeFiles.length) * 100,
          });
          updateIndexingStatus({
            phase: "code",
            message: "Metricas de codigo",
            processed,
            total: codeFiles.length,
            isIndexing: true,
          });
        }

        if (processed % 50 === 0) {
          provider.refresh();
        }

        onBatchComplete?.();
      }

      onBatchComplete?.();
      provider.refresh();
      console.log(
        `[Cortex] ✓ Code analysis complete (${successful}/${codeFiles.length} successful)`
      );
      updateIndexingStatus({
        phase: "done",
        message: "Metricas de codigo listas",
        processed: codeFiles.length,
        total: codeFiles.length,
        isIndexing: false,
      });
    }
  );
}

/**
 * Background indexing for document mirrors (MD/CSV twins)
 */
async function startMirrorIndexing(
  mirrorStore: MirrorStore,
  updateMirrorTracking: (
    file: FileIndexEntry,
    mirrorPath: string,
    sourceMtime: number
  ) => void,
  filesToProcess?: FileIndexEntry[],
  onBatchComplete?: () => void,
  showProgress = true
): Promise<void> {
  const config = vscode.workspace.getConfiguration("cortex.mirror");
  const maxFileSizeMb = config.get<number>("maxFileSizeMB", 25);
  const maxFileSizeBytes = Math.max(1, maxFileSizeMb) * 1024 * 1024;
  const maxConcurrency = Math.max(
    1,
    Math.min(config.get<number>("maxConcurrency", 1), 4)
  );

  const files = filesToProcess ?? [];
  const targetFiles = files.filter(
    (file) =>
      mirrorStore.isMirrorableExtension(file.extension) &&
      file.fileSize > 0 &&
      file.fileSize <= maxFileSizeBytes
  );

  if (targetFiles.length === 0) {
    console.log("[Cortex] No mirrorable documents found");
    return;
  }

  const runIndexing = async (
    progress?: vscode.Progress<{ message?: string; increment?: number }>
  ) => {
    console.log(
      `[Cortex] Building mirrors for ${targetFiles.length} document files...`
    );

    const queue = [...targetFiles];
    let processed = 0;
    let updated = 0;
    let skipped = 0;
    let failed = 0;
    let lastReported = 0;

    const updateProgress = () => {
      if (!progress) {
        return;
      }
      if (processed % 10 === 0 || processed === targetFiles.length) {
        const delta = processed - lastReported;
        if (delta <= 0) {
          return;
        }
        lastReported = processed;
        progress.report({
          message: `${processed}/${targetFiles.length} files`,
          increment: (delta / targetFiles.length) * 100,
        });
      }
    };

    const worker = async () => {
      while (queue.length > 0) {
        const file = queue.shift();
        if (!file) {
          return;
        }

        try {
          const mirrorPath = mirrorStore.getMirrorPath(
            file.relativePath,
            file.extension
          );
          if (
            mirrorPath &&
            file.enhanced?.mirror?.sourceMtime === file.lastModified &&
            file.enhanced.indexed?.mirror &&
            (await mirrorStore.mirrorExists(
              file.relativePath,
              file.extension
            ))
          ) {
            skipped++;
            continue;
          }
          const content = await mirrorStore.ensureMirrorContent(
            file.absolutePath,
            file.relativePath,
            file.extension
          );
          if (content == null) {
            failed++;
          } else if (content.length === 0) {
            skipped++;
          } else {
            updated++;
            if (mirrorPath) {
              updateMirrorTracking(file, mirrorPath, file.lastModified);
            }
          }
        } catch (error) {
          failed++;
          console.warn(
            `[Cortex] Mirror generation failed for ${file.relativePath}:`,
            error
          );
        } finally {
          processed++;
          updateProgress();
          onBatchComplete?.();
        }
      }
    };

    const workerCount = Math.min(maxConcurrency, targetFiles.length);
    await Promise.all(Array.from({ length: workerCount }, () => worker()));

    console.log(
      `[Cortex] ✓ Mirror indexing complete (${updated} updated, ${skipped} unchanged, ${failed} failed)`
    );
  };

  if (!showProgress) {
    return runIndexing();
  }

  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Building document mirrors",
      cancellable: false,
    },
    async (progress) => {
      await runIndexing(progress);
    }
  );
}

/**
 * Background indexing for AI summaries
 */
async function startAISummaryIndexing(
  indexStore: IndexStore,
  metadataStore: IMetadataStore,
  llmService: LLMService,
  mirrorStore: MirrorStore,
  workspaceRoot: string,
  filesToProcess?: FileIndexEntry[],
  onBatchComplete?: () => void,
  showProgress = true
): Promise<void> {
  const config = vscode.workspace.getConfiguration("cortex.llm.autoSummary");
  const enabled = config.get<boolean>("enabled", false);
  if (!enabled || !llmService.isEnabled()) {
    return;
  }

  const available = await llmService.isAvailable();
  if (!available) {
    console.warn("[Cortex] AI summaries skipped: LLM not available");
    return;
  }

  const maxFileSize = config.get<number>("maxFileSize", 250000);
  const maxConcurrency = Math.max(
    1,
    Math.min(config.get<number>("maxConcurrency", 2), 8)
  );

  const files = filesToProcess || indexStore.getAllFiles();
  const targetFiles = files.filter(
    (file) =>
      (isLikelyTextExtension(file.extension) ||
        mirrorStore.isMirrorableExtension(file.extension)) &&
      file.fileSize > 0 &&
      file.fileSize <= maxFileSize
  );

  if (targetFiles.length === 0) {
    console.log("[Cortex] No files eligible for AI summaries");
    return;
  }

  const runIndexing = async (
    progress?: vscode.Progress<{ message?: string; increment?: number }>
  ) => {
      console.log(
        `[Cortex] Starting AI summary indexing for ${targetFiles.length} files...`
      );

      const queue = [...targetFiles];
      let processed = 0;
      let summarized = 0;
      let skipped = 0;
      let failed = 0;
      let lastReported = 0;

      const updateProgress = () => {
        if (!progress) {
          return;
        }
        if (processed % 10 === 0 || processed === targetFiles.length) {
          const delta = processed - lastReported;
          if (delta <= 0) {
            return;
          }
          lastReported = processed;
          progress.report({
            message: `${processed}/${targetFiles.length} files`,
            increment: (delta / targetFiles.length) * 100,
          });
        }
      };

      const worker = async () => {
        while (queue.length > 0) {
          const file = queue.shift();
          if (!file) {
            return;
          }

          try {
            const aiContent = await resolveAIContent(
              {
                absolutePath: file.absolutePath,
                relativePath: file.relativePath,
                extension: file.extension,
              },
              mirrorStore
            );
            if (!aiContent) {
              skipped++;
              continue;
            }

            const content = aiContent.content;
            const contentHash = aiContent.contentHash;
            const metadata = metadataStore.getMetadataByPath(
              file.relativePath
            ) as FileMetadata | null;

            if (
              metadata?.aiSummary &&
              metadata.aiSummaryHash === contentHash
            ) {
              if (!metadata.aiKeyTerms || metadata.aiKeyTerms.length === 0) {
                metadataStore.updateAISummary(
                  file.relativePath,
                  metadata.aiSummary,
                  contentHash,
                  extractKeyTerms(content)
                );
              }
              skipped++;
              continue;
            }

            const summary = await llmService.generateFileSummary(
              file.relativePath,
              content
            );

            if (summary) {
              const keyTerms = extractKeyTerms(content);
              metadataStore.updateAISummary(
                file.relativePath,
                summary,
                contentHash,
                keyTerms
              );
              
              // Save summary to repository
              await saveAISummaryToFile(
                workspaceRoot,
                file.relativePath,
                summary,
                contentHash,
                keyTerms
              );
              
              summarized++;
            } else {
              failed++;
            }
          } catch (error) {
            failed++;
            console.warn(
              `[Cortex] AI summary failed for ${file.relativePath}:`,
              error
            );
          } finally {
            processed++;
            updateProgress();
            onBatchComplete?.();
          }
        }
      };

      const workerCount = Math.min(maxConcurrency, targetFiles.length);
      await Promise.all(
        Array.from({ length: workerCount }, () => worker())
      );

      console.log(
        `[Cortex] ✓ AI summary indexing complete (${summarized} summarized, ${skipped} skipped, ${failed} failed)`
      );
  };

  if (!showProgress) {
    return runIndexing();
  }

  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Generating AI summaries",
      cancellable: false,
    },
    async (progress) => {
      await runIndexing(progress);
    }
  );
}

/**
 * Background indexing for AI tags and project suggestions
 */
async function startAITagProjectIndexing(
  indexStore: IndexStore,
  metadataStore: IMetadataStore,
  llmService: LLMService,
  mirrorStore: MirrorStore,
  workspaceRoot: string,
  filesToProcess?: FileIndexEntry[],
  onBatchComplete?: () => void,
  showProgress = true,
  onRefreshViews?: () => void
): Promise<void> {
  const config = vscode.workspace.getConfiguration("cortex.llm.autoIndex");
  const enabled = config.get<boolean>("enabled", false);
  if (!enabled || !llmService.isEnabled()) {
    console.log(
      `[Cortex][AI] Auto-index disabled (enabled=${enabled}, llmEnabled=${llmService.isEnabled()})`
    );
    return;
  }

  const available = await llmService.isAvailable();
  if (!available) {
    console.warn("[Cortex] AI tags/projects skipped: LLM not available");
    return;
  }

  const applyTags = config.get<boolean>("applyTags", false);
  const applyProjects = config.get<boolean>("applyProjects", false);
  const useSuggestedContexts = config.get<boolean>(
    "useSuggestedContexts",
    true
  );
  const maxFileSize = config.get<number>("maxFileSize", 250000);
  const maxConcurrency = Math.max(
    1,
    Math.min(config.get<number>("maxConcurrency", 2), 8)
  );

  if (!applyTags && !applyProjects) {
    console.log(
      "[Cortex][AI] Auto-index configured but both applyTags/applyProjects are false"
    );
    return;
  }

  console.log(
    `[Cortex][AI] Auto-index config applyTags=${applyTags} applyProjects=${applyProjects} useSuggestedContexts=${useSuggestedContexts} maxFileSize=${maxFileSize} maxConcurrency=${maxConcurrency}`
  );

  const files = filesToProcess || indexStore.getAllFiles();
  const targetFiles = files.filter(
    (file) =>
      (isLikelyTextExtension(file.extension) ||
        mirrorStore.isMirrorableExtension(file.extension)) &&
      file.fileSize > 0 &&
      file.fileSize <= maxFileSize
  );

  if (targetFiles.length === 0) {
    console.log("[Cortex] No files eligible for AI tags/projects");
    return;
  }

  const runIndexing = async (
    progress?: vscode.Progress<{ message?: string; increment?: number }>
  ) => {
      console.log(
        `[Cortex] Starting AI tag/project indexing for ${targetFiles.length} files...`
      );

      const queue = [...targetFiles];
      let processed = 0;
      let tagged = 0;
      let suggestedProjects = 0;
      let skipped = 0;
      let failed = 0;
      let lastReported = 0;

      const updateProgress = () => {
        if (!progress) {
          return;
        }
        if (processed % 10 === 0 || processed === targetFiles.length) {
          const delta = processed - lastReported;
          if (delta <= 0) {
            return;
          }
          lastReported = processed;
          progress.report({
            message: `${processed}/${targetFiles.length} files`,
            increment: (delta / targetFiles.length) * 100,
          });
        }
      };

      const worker = async () => {
        while (queue.length > 0) {
          const file = queue.shift();
          if (!file) {
            return;
          }

          try {
            console.log(`[Cortex][AI] Processing ${file.relativePath}`);
            const aiContent = await resolveAIContent(
              {
                absolutePath: file.absolutePath,
                relativePath: file.relativePath,
                extension: file.extension,
              },
              mirrorStore
            );
            if (!aiContent) {
              console.log(
                `[Cortex][AI] Skipped ${file.relativePath}: no readable content`
              );
              skipped++;
              continue;
            }

            const content = aiContent.content;
            const contentHash = aiContent.contentHash;
            const metadata = metadataStore.getMetadataByPath(
              file.relativePath
            ) as FileMetadata | null;
            let summary =
              metadata?.aiSummary && metadata?.aiSummaryHash === contentHash
                ? metadata.aiSummary
                : undefined;
            const keyTerms =
              metadata?.aiKeyTerms && metadata.aiKeyTerms.length > 0
                ? metadata.aiKeyTerms
                : extractKeyTerms(content);

            if (!summary) {
              const gen = await llmService.generateFileSummary(
                file.relativePath,
                content
              );
              if (gen != null) {
                summary = gen;
                metadataStore.updateAISummary(
                  file.relativePath,
                  summary,
                  contentHash,
                  keyTerms
                );
                
                // Save summary to repository
                await saveAISummaryToFile(
                  workspaceRoot,
                  file.relativePath,
                  summary,
                  contentHash,
                  keyTerms
                );
                
                console.log(
                  `[Cortex][AI] Summary generated for ${file.relativePath}`
                );
              }
            } else if (!metadata?.aiKeyTerms || metadata.aiKeyTerms.length === 0) {
              metadataStore.updateAISummary(
                file.relativePath,
                summary,
                contentHash,
                keyTerms
              );
              
              // Save summary to repository even if it was cached
              await saveAISummaryToFile(
                workspaceRoot,
                file.relativePath,
                summary,
                contentHash,
                keyTerms
              );
              
              console.log(
                `[Cortex][AI] Summary cache refreshed for ${file.relativePath}`
              );
            }

            const snippet = content.slice(0, 500);

            if (applyTags) {
              const suggestedTags = await llmService.suggestTags(
                file.relativePath,
                content,
                { summary, keyTerms, snippet }
              );

              if (suggestedTags.length > 0) {
                const existingTags = metadata?.tags || [];
                const newTags = suggestedTags.filter(
                  (tag) => !existingTags.includes(tag)
                );
                if (newTags.length > 0) {
                  newTags.forEach((tag) =>
                    metadataStore.addTag(file.relativePath, tag)
                  );
                  tagged += 1;
                  console.log(
                    `[Cortex][AI] Applied ${newTags.length} tag(s) to ${file.relativePath}`
                  );
                }
              } else {
                console.log(
                  `[Cortex][AI] No tag suggestions for ${file.relativePath}`
                );
              }
            }

            if (applyProjects) {
              const recentFiles: string[] = [];
              const allContexts = metadataStore.getAllContexts();
              const dirPath = file.relativePath.substring(
                0,
                file.relativePath.lastIndexOf("/")
              );

              for (const context of allContexts.slice(0, 3)) {
                const filesInContext = metadataStore.getFilesByContext(context);
                recentFiles.push(...filesInContext.slice(0, 3));
              }
              if (dirPath) {
                recentFiles.push(`${dirPath}/*`);
              }

              const suggestedProject = await llmService.suggestProject(
                file.relativePath,
                content,
                recentFiles,
                { summary, keyTerms, snippet }
              );

              if (suggestedProject) {
                const existingContexts = metadata?.contexts || [];
                if (!existingContexts.includes(suggestedProject)) {
                  if (useSuggestedContexts) {
                    metadataStore.addSuggestedContext(
                      file.relativePath,
                      suggestedProject
                    );
                    console.log(
                      `[Cortex][AI] Suggested project "${suggestedProject}" for ${file.relativePath}`
                    );
                  } else {
                    metadataStore.addContext(
                      file.relativePath,
                      suggestedProject
                    );
                    console.log(
                      `[Cortex][AI] Applied project "${suggestedProject}" to ${file.relativePath}`
                    );
                  }
                  suggestedProjects += 1;
                } else {
                  console.log(
                    `[Cortex][AI] Project "${suggestedProject}" already applied to ${file.relativePath}`
                  );
                }
            } else {
              console.log(
                `[Cortex][AI] No project suggestion for ${file.relativePath}`
              );
            }
          }
        } catch (error) {
          failed++;
          console.warn(
            `[Cortex] AI tag/project failed for ${file.relativePath}:`,
            error
          );
        } finally {
          processed++;
          updateProgress();
          
          // Refresh views periodically to show newly indexed tags
          if (processed % 20 === 0 && onRefreshViews) {
            onRefreshViews();
          }
          
          onBatchComplete?.();
        }
        }
      };

      const workerCount = Math.min(maxConcurrency, targetFiles.length);
      await Promise.all(
        Array.from({ length: workerCount }, () => worker())
      );

      console.log(
        `[Cortex] ✓ AI tag/project indexing complete (${tagged} tagged, ${suggestedProjects} projects, ${skipped} skipped, ${failed} failed)`
      );
  };

  if (!showProgress) {
    return runIndexing();
  }

  return vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Generating AI tags/projects",
      cancellable: false,
    },
    async (progress) => {
      await runIndexing(progress);
    }
  );
}

/**
 * Extension activation
 */
export async function activate(context: vscode.ExtensionContext) {
  console.log("[Cortex] Activating extension...");

  // Verify workspace is open
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    vscode.window.showWarningMessage(
      "Cortex requires an open workspace to function"
    );
    return;
  }

  const workspaceRoot = workspaceFolders[0].uri.fsPath;
  console.log(`[Cortex] Workspace root: ${workspaceRoot}`);

  // Initialize core components
  const blacklistStore = new BlacklistStore(workspaceRoot);
  await blacklistStore.load();
  const fileScanner = new FileScanner(workspaceRoot, blacklistStore);
  const workerScript = path.join(
    context.extensionPath,
    "out",
    "workers",
    "indexerWorker.js"
  );
  const maxThreads = Math.max(1, os.cpus().length * 2);
  const poolCount = 4;
  const baseSize = Math.max(1, Math.floor(maxThreads / poolCount));
  const remainder = maxThreads - baseSize * poolCount;
  const poolSizes = [
    baseSize + (remainder > 0 ? 1 : 0),
    baseSize + (remainder > 1 ? 1 : 0),
    baseSize + (remainder > 2 ? 1 : 0),
    baseSize + (remainder > 3 ? 1 : 0),
  ];

  const basicPool = new WorkerPool<IndexerTask>(workerScript, "basic", {
    size: poolSizes[0],
  });
  const contentTypePool = new WorkerPool<IndexerTask>(
    workerScript,
    "content-type",
    {
      size: poolSizes[1],
    }
  );
  const codePool = new WorkerPool<IndexerTask>(workerScript, "code", {
    size: poolSizes[2],
  });
  const documentPool = new WorkerPool<IndexerTask>(workerScript, "documents", {
    size: poolSizes[3],
  });
  console.log(
    `[Cortex] Worker pools initialized (max ${maxThreads} threads): basic=${poolSizes[0]}, content-type=${poolSizes[1]}, code=${poolSizes[2]}, documents=${poolSizes[3]}`
  );
  const indexStore = new IndexStore();
  let metadataStore: IMetadataStore;
  try {
    const sqliteStore = new SQLiteMetadataStore(workspaceRoot);
    await sqliteStore.initialize();
    metadataStore = sqliteStore;
  } catch (error) {
    const sqliteMessage =
      error instanceof Error ? error.message : String(error);
    console.warn(
      "[Cortex] SQLite metadata store failed, falling back to JSON."
    );
    console.warn(
      "[Cortex] To enable SQLite, rebuild better-sqlite3 (e.g. `npm rebuild better-sqlite3`)."
    );
    console.warn("[Cortex] SQLite init error:", sqliteMessage);
    const jsonStore = new MetadataStoreJSON(workspaceRoot);
    await jsonStore.initialize();
    metadataStore = jsonStore;
  }
  const metadataExtractor = new MetadataExtractor(workspaceRoot);
  const mirrorStore = new MirrorStore(workspaceRoot);
  const indexCache = new IndexCache(workspaceRoot);
  const llmService = new LLMService();
  let cachedEntries: FileIndexEntry[] = [];
  let changedFiles: FileIndexEntry[] = [];
  const scanStatus: IndexingStatus = {
    phase: "scanning",
    message: "Escaneando",
    processed: 0,
    total: 0,
    isIndexing: true,
  };
  const basicStatus: IndexingStatus = {
    phase: "basic",
    message: "Metadata basica",
    processed: 0,
    total: 0,
    isIndexing: true,
  };
  const contentTypeStatus: IndexingStatus = {
    phase: "contentTypes",
    message: "Esperando tipos de contenido",
    processed: 0,
    total: 0,
    isIndexing: true,
  };
  const codeStatus: IndexingStatus = {
    phase: "code",
    message: "Esperando metricas de codigo",
    processed: 0,
    total: 0,
    isIndexing: true,
  };
  const documentStatus: IndexingStatus = {
    phase: "documents",
    message: "Esperando documentos",
    processed: 0,
    total: 0,
    isIndexing: true,
  };

  // Metadata store initialized during selection.

  // Initialize tree view providers
  const contextTreeProvider = new ContextTreeProvider(
    workspaceRoot,
    metadataStore,
    indexStore,
    scanStatus
  );

  const tagTreeProvider = new TagTreeProvider(
    workspaceRoot,
    metadataStore,
    indexStore,
    scanStatus
  );

  const typeTreeProvider = new TypeTreeProvider(
    workspaceRoot,
    metadataStore,
    indexStore,
    scanStatus
  );

  const dateTreeProvider = new DateTreeProvider(
    workspaceRoot,
    indexStore,
    scanStatus
  );

  const sizeTreeProvider = new SizeTreeProvider(
    workspaceRoot,
    indexStore,
    scanStatus
  );

  const folderTreeProvider = new FolderTreeProvider(
    workspaceRoot,
    indexStore,
    scanStatus
  );

  const contentTypeTreeProvider = new ContentTypeTreeProvider(
    workspaceRoot,
    indexStore,
    metadataExtractor,
    contentTypeStatus
  );

  const codeMetricsTreeProvider = new CodeMetricsTreeProvider(
    workspaceRoot,
    indexStore,
    metadataExtractor,
    codeStatus
  );

  const documentMetricsTreeProvider = new DocumentMetricsTreeProvider(
    workspaceRoot,
    indexStore,
    metadataExtractor,
    documentStatus
  );

  const issuesTreeProvider = new IssuesTreeProvider(
    workspaceRoot,
    indexStore,
    undefined,
    blacklistStore
  );

  const fileInfoTreeProvider = new FileInfoTreeProvider(
    workspaceRoot,
    metadataStore,
    indexStore
  );

  const categoryTreeProvider = new CategoryTreeProvider(
    workspaceRoot,
    metadataStore,
    indexStore,
    llmService,
    scanStatus
  );

  // Register tree views
  const contextTreeView = vscode.window.createTreeView("cortex-contextView", {
    treeDataProvider: contextTreeProvider,
    showCollapseAll: false,
  });

  const tagTreeView = vscode.window.createTreeView("cortex-tagView", {
    treeDataProvider: tagTreeProvider,
    showCollapseAll: false,
  });

  const typeTreeView = vscode.window.createTreeView("cortex-typeView", {
    treeDataProvider: typeTreeProvider,
    showCollapseAll: false,
  });

  const dateTreeView = vscode.window.createTreeView("cortex-dateView", {
    treeDataProvider: dateTreeProvider,
    showCollapseAll: false,
  });

  const sizeTreeView = vscode.window.createTreeView("cortex-sizeView", {
    treeDataProvider: sizeTreeProvider,
    showCollapseAll: false,
  });

  const folderTreeView = vscode.window.createTreeView("cortex-folderView", {
    treeDataProvider: folderTreeProvider,
    showCollapseAll: false,
  });

  const contentTypeTreeView = vscode.window.createTreeView(
    "cortex-contentTypeView",
    {
      treeDataProvider: contentTypeTreeProvider,
      showCollapseAll: false,
    }
  );

  const codeMetricsTreeView = vscode.window.createTreeView(
    "cortex-codeMetricsView",
    {
      treeDataProvider: codeMetricsTreeProvider,
      showCollapseAll: false,
    }
  );

  const documentMetricsTreeView = vscode.window.createTreeView(
    "cortex-documentMetricsView",
    {
      treeDataProvider: documentMetricsTreeProvider,
      showCollapseAll: false,
    }
  );

  const issuesTreeView = vscode.window.createTreeView("cortex-issuesView", {
    treeDataProvider: issuesTreeProvider,
    showCollapseAll: false,
  });

  const fileInfoTreeView = vscode.window.createTreeView("cortex-fileInfoView", {
    treeDataProvider: fileInfoTreeProvider,
    showCollapseAll: false,
  });

  const categoryTreeView = vscode.window.createTreeView("cortex-categoryView", {
    treeDataProvider: categoryTreeProvider,
    showCollapseAll: false,
  });

  const accordionViews: Array<{
    id: string;
    view: vscode.TreeView<unknown>;
    provider: AccordionTreeProviderAny;
  }> = [
    {
      id: "cortex-contextView",
      view: contextTreeView,
      provider: contextTreeProvider,
    },
    { id: "cortex-tagView", view: tagTreeView, provider: tagTreeProvider },
    { id: "cortex-typeView", view: typeTreeView, provider: typeTreeProvider },
    { id: "cortex-dateView", view: dateTreeView, provider: dateTreeProvider },
    { id: "cortex-sizeView", view: sizeTreeView, provider: sizeTreeProvider },
    {
      id: "cortex-folderView",
      view: folderTreeView,
      provider: folderTreeProvider,
    },
    {
      id: "cortex-contentTypeView",
      view: contentTypeTreeView,
      provider: contentTypeTreeProvider,
    },
    {
      id: "cortex-codeMetricsView",
      view: codeMetricsTreeView,
      provider: codeMetricsTreeProvider,
    },
    {
      id: "cortex-documentMetricsView",
      view: documentMetricsTreeView,
      provider: documentMetricsTreeProvider,
    },
    {
      id: "cortex-issuesView",
      view: issuesTreeView,
      provider: issuesTreeProvider,
    },
    {
      id: "cortex-categoryView",
      view: categoryTreeView,
      provider: categoryTreeProvider,
    },
  ];

  const accordionProvidersById = new Map<string, AccordionTreeProviderAny>(
    accordionViews.map((entry) => [entry.id, entry.provider])
  );

  let lastActiveViewId: string | undefined;
  accordionViews.forEach((entry) => {
    context.subscriptions.push(
      entry.view.onDidExpandElement((event) =>
        entry.provider.handleDidExpand(event.element)
      ),
      entry.view.onDidCollapseElement((event) =>
        entry.provider.handleDidCollapse(event.element)
      ),
      entry.view.onDidChangeSelection(() => {
        lastActiveViewId = entry.id;
      }),
      entry.view.onDidChangeVisibility((event) => {
        if (event.visible) {
          lastActiveViewId = entry.id;
        }
      })
    );
  });

  const viewIdMap = new Map<vscode.TreeView<unknown>, string>();
  accordionViews.forEach((entry) => {
    viewIdMap.set(entry.view, entry.id);
  });

  const getAccordionProvider = (
    view?: vscode.TreeView<unknown>
  ): AccordionTreeProviderAny | undefined => {
    if (view && viewIdMap.has(view)) {
      const viewId = viewIdMap.get(view)!;
      if (accordionProvidersById.has(viewId)) {
        return accordionProvidersById.get(viewId);
      }
    }
    if (lastActiveViewId && accordionProvidersById.has(lastActiveViewId)) {
      return accordionProvidersById.get(lastActiveViewId);
    }
    return undefined;
  };

  const setAccordionEnabled = (enabled: boolean): void => {
    vscode.commands.executeCommand(
      "setContext",
      "cortex.accordionEnabled",
      enabled
    );
    accordionViews.forEach((entry) =>
      entry.provider.setAccordionEnabled(enabled)
    );
  };

  setAccordionEnabled(false);

  const formatProgress = (status: IndexingStatus): string => {
    const width = 12;
    if (status.total > 0) {
      const ratio = Math.min(1, status.processed / status.total);
      const filled = Math.round(ratio * width);
      const bar = `${"=".repeat(filled)}${"-".repeat(width - filled)}`;
      const percent = Math.round(ratio * 100);
      return `Indexing: ${status.message} [${bar}] ${percent}% (${status.processed}/${status.total})`;
    }
    const pos = status.processed % width;
    let bar = "";
    for (let i = 0; i < width; i += 1) {
      bar += i === pos ? ">" : "-";
    }
    return `Indexing: ${status.message} [${bar}] (${status.processed})`;
  };

  const updateViewMessages = (key: string, status: IndexingStatus) => {
    const message = status.isIndexing ? formatProgress(status) : undefined;
    switch (key) {
      case "scan":
        contextTreeView.message = message;
        tagTreeView.message = message;
        typeTreeView.message = message;
        dateTreeView.message = message;
        sizeTreeView.message = message;
        folderTreeView.message = message;
        break;
      case "content-type":
        contentTypeTreeView.message = message;
        break;
      case "code":
        codeMetricsTreeView.message = message;
        break;
      case "documents":
        documentMetricsTreeView.message = message;
        break;
      case "basic":
        issuesTreeView.message = message;
        break;
      default:
        break;
    }
  };

  // Callback to refresh all views when metadata changes
  const refreshAllViews = () => {
    contextTreeProvider.refresh();
    tagTreeProvider.refresh();
    typeTreeProvider.refresh();
    dateTreeProvider.refresh();
    sizeTreeProvider.refresh();
    folderTreeProvider.refresh();
    contentTypeTreeProvider.refresh();
    codeMetricsTreeProvider.refresh();
    documentMetricsTreeProvider.refresh();
    issuesTreeProvider.refresh();
    categoryTreeProvider.refresh();
    fileInfoTreeProvider.refresh();
  };

  const updateMirrorTracking = (
    file: FileIndexEntry,
    mirrorPath: string,
    sourceMtime: number
  ) => {
    if (!file.enhanced) {
      file.enhanced = {
        stats: {
          size: 0,
          created: 0,
          modified: 0,
          accessed: 0,
          isReadOnly: false,
          isHidden: false,
        },
        folder: file.relativePath.includes("/")
          ? file.relativePath.substring(0, file.relativePath.lastIndexOf("/"))
          : ".",
        depth: file.relativePath.split("/").length - 1,
      };
    }
    if (!file.enhanced.indexed) {
      file.enhanced.indexed = {};
    }
    file.enhanced.indexed.mirror = true;
    const mirror = {
      format: mirrorStore.getMirrorFormat(file.extension) ?? "md",
      path: mirrorPath,
      sourceMtime,
      updatedAt: Date.now(),
    };
    file.enhanced.mirror = mirror;
    const storeWithMirror = metadataStore as {
      updateMirrorMetadata?: (relativePath: string, mirror: MirrorMetadata) => void;
    };
    storeWithMirror.updateMirrorMetadata?.(file.relativePath, mirror);
  };

  const removeMetadataEntry = (relativePath: string) => {
    const storeWithRemove = metadataStore as {
      removeFile?: (path: string) => void;
    };
    storeWithRemove.removeFile?.(relativePath);
  };

  type RecentDeleteEntry = {
    file: FileIndexEntry;
    metadata: ReturnType<typeof metadataStore.getMetadataByPath> | null;
    deletedAt: number;
  };

  const recentDeletes = new Map<string, RecentDeleteEntry>();
  let deleteCleanupTimer: NodeJS.Timeout | undefined;
  const RENAME_WINDOW_MS = 5000;

  const scheduleDeleteCleanup = () => {
    if (deleteCleanupTimer) {
      return;
    }
    deleteCleanupTimer = setTimeout(() => {
      deleteCleanupTimer = undefined;
      const now = Date.now();
      for (const [relativePath, entry] of recentDeletes.entries()) {
        if (now - entry.deletedAt < RENAME_WINDOW_MS) {
          continue;
        }
        removeMetadataEntry(relativePath);
        void mirrorStore.removeMirror(
          relativePath,
          path.extname(relativePath)
        );
        recentDeletes.delete(relativePath);
      }
    }, RENAME_WINDOW_MS);
  };

  const findRenameCandidate = (
    file: FileIndexEntry
  ): RecentDeleteEntry | null => {
    for (const entry of recentDeletes.values()) {
      if (Math.abs(entry.file.lastModified - file.lastModified) > 5) {
        continue;
      }
      if (entry.file.fileSize !== file.fileSize) {
        continue;
      }
      return entry;
    }
    return null;
  };

  const migrateMetadata = async (
    oldEntry: RecentDeleteEntry,
    newEntry: FileIndexEntry
  ) => {
    const oldPath = oldEntry.file.relativePath;
    const newPath = newEntry.relativePath;
    const existing =
      (oldEntry.metadata ??
        metadataStore.getMetadataByPath(oldPath)) as FileMetadata | null;
    if (!existing) {
      return;
    }

    metadataStore.getOrCreateMetadata(newPath, newEntry.extension);

    for (const tag of existing.tags ?? []) {
      metadataStore.addTag(newPath, tag);
    }
    for (const context of existing.contexts ?? []) {
      metadataStore.addContext(newPath, context);
    }
    for (const suggested of existing.suggestedContexts ?? []) {
      metadataStore.addSuggestedContext(newPath, suggested);
    }
    if (existing.notes) {
      metadataStore.updateNotes(newPath, existing.notes);
    }
    if (existing.aiSummary && existing.aiSummaryHash) {
      metadataStore.updateAISummary(
        newPath,
        existing.aiSummary,
        existing.aiSummaryHash,
        existing.aiKeyTerms
      );
    }

    if (existing.mirror) {
      const nextMirrorPath = mirrorStore.getMirrorPath(
        newPath,
        newEntry.extension
      );
      if (nextMirrorPath) {
        try {
          await fs.mkdir(path.dirname(nextMirrorPath), { recursive: true });
          await fs.rename(existing.mirror.path, nextMirrorPath);
          metadataStore.updateMirrorMetadata(newPath, {
            format: existing.mirror.format,
            path: nextMirrorPath,
            sourceMtime: newEntry.lastModified,
            updatedAt: Date.now(),
          });
        } catch {
          // Fall back to regeneration on next index pass.
        }
      }
    }

    removeMetadataEntry(oldPath);
  };

  const hydrateMirrorMetadata = (files: FileIndexEntry[]) => {
    let hydrated = 0;
    files.forEach((file) => {
      const metadata = metadataStore.getMetadataByPath(file.relativePath);
      if (!metadata?.mirror) {
        return;
      }
      if (!file.enhanced) {
        file.enhanced = {
          stats: {
            size: 0,
            created: 0,
            modified: 0,
            accessed: 0,
            isReadOnly: false,
            isHidden: false,
          },
          folder: file.relativePath.includes("/")
            ? file.relativePath.substring(0, file.relativePath.lastIndexOf("/"))
            : ".",
          depth: file.relativePath.split("/").length - 1,
        };
      }
      if (!file.enhanced.indexed) {
        file.enhanced.indexed = {};
      }
      file.enhanced.mirror = metadata.mirror;
      file.enhanced.indexed.mirror = true;
      hydrated += 1;
    });
    if (hydrated > 0) {
      console.log(`[Cortex] Hydrated mirror metadata for ${hydrated} files`);
    }
  };

  const verifyMirrorCache = async (files: FileIndexEntry[]) => {
    let missing = 0;
    let stale = 0;
    let restored = 0;

    for (const file of files) {
      if (!mirrorStore.isMirrorableExtension(file.extension)) {
        continue;
      }
      const mirrorPath = mirrorStore.getMirrorPath(
        file.relativePath,
        file.extension
      );
      if (!mirrorPath) {
        continue;
      }

      const exists = await mirrorStore.mirrorExists(
        file.relativePath,
        file.extension
      );
      if (!exists) {
        if (file.enhanced?.indexed) {
          file.enhanced.indexed.mirror = false;
        }
        const storeWithClear = metadataStore as {
          clearMirrorMetadata?: (relativePath: string) => void;
        };
        storeWithClear.clearMirrorMetadata?.(file.relativePath);
        missing += 1;
        continue;
      }

      const sourceMtime = file.lastModified;
      if (file.enhanced?.mirror?.sourceMtime !== sourceMtime) {
        if (file.enhanced?.indexed) {
          file.enhanced.indexed.mirror = false;
        }
        const storeWithClear = metadataStore as {
          clearMirrorMetadata?: (relativePath: string) => void;
        };
        storeWithClear.clearMirrorMetadata?.(file.relativePath);
        stale += 1;
        continue;
      }

      if (!file.enhanced?.mirror) {
        updateMirrorTracking(file, mirrorPath, sourceMtime);
        restored += 1;
      } else if (file.enhanced?.indexed) {
        file.enhanced.indexed.mirror = true;
      }
    }

    if (missing > 0 || stale > 0 || restored > 0) {
      console.log(
        `[Cortex] Mirror verification: ${missing} missing, ${stale} stale, ${restored} restored`
      );
    }
  };

  let cacheSaveTimer: NodeJS.Timeout | undefined;
  const scheduleCacheSave = (delayMs = 2000) => {
    if (cacheSaveTimer) {
      clearTimeout(cacheSaveTimer);
    }
    cacheSaveTimer = setTimeout(async () => {
      try {
        await indexCache.save(indexStore.getAllFiles());
        console.log("[Cortex] Index cache saved");
      } catch (error) {
        console.error("[Cortex] Failed to save index cache:", error);
      }
    }, delayMs);
  };

  let refreshTimer: NodeJS.Timeout | undefined;
  const scheduleRefreshAllViews = () => {
    if (refreshTimer) {
      return;
    }
    refreshTimer = setTimeout(() => {
      refreshTimer = undefined;
      refreshAllViews();
    }, 500);
  };

  const lastLogAtByKey = new Map<string, number>();
  const updateIndexingStatus = (patch: Partial<IndexingStatus>) => {
    Object.assign(scanStatus, patch);
    updateViewMessages("scan", scanStatus);
    const now = Date.now();
    const lastLogAt = lastLogAtByKey.get("scan") || 0;
    if (now - lastLogAt > 2000) {
      lastLogAtByKey.set("scan", now);
      console.log(
        `[Cortex][Indexing] ${scanStatus.phase} | ${scanStatus.message} | ${scanStatus.processed}/${scanStatus.total} | indexing=${scanStatus.isIndexing}`
      );
    }
    scheduleRefreshAllViews();
  };

  cachedEntries = await indexCache.load();
  if (cachedEntries.length > 0) {
    indexStore.buildIndex(cachedEntries);
    console.log(`[Cortex] Loaded ${cachedEntries.length} cached index entries`);
    updateIndexingStatus({
      phase: "done",
      message: "Listo",
      processed: cachedEntries.length,
      total: cachedEntries.length,
      isIndexing: false,
    });
    refreshAllViews();
  }

  // Register commands
  const commands = [
    vscode.commands.registerCommand("cortex.addTag", () =>
      addTagCommand(workspaceRoot, metadataStore, indexStore, refreshAllViews)
    ),

    vscode.commands.registerCommand("cortex.assignContext", () =>
      assignContextCommand(
        workspaceRoot,
        metadataStore,
        indexStore,
        refreshAllViews
      )
    ),

    vscode.commands.registerCommand("cortex.openView", openViewCommand),

    vscode.commands.registerCommand("cortex.rebuildIndex", () =>
      rebuildIndexCommand(
        workspaceRoot,
        fileScanner,
        indexStore,
        metadataStore,
        refreshAllViews
      )
    ),

    vscode.commands.registerCommand("cortex.enableAccordion", () =>
      setAccordionEnabled(true)
    ),

    vscode.commands.registerCommand("cortex.disableAccordion", () =>
      setAccordionEnabled(false)
    ),

    vscode.commands.registerCommand(
      "cortex.expandAll",
      (view?: vscode.TreeView<unknown>) => {
        const provider = getAccordionProvider(view);
        if (!provider) {
          return;
        }
        provider.expandAll();
      }
    ),

    vscode.commands.registerCommand(
      "cortex.collapseAll",
      (view?: vscode.TreeView<unknown>) => {
        const provider = getAccordionProvider(view);
        if (!provider) {
          return;
        }
        provider.collapseAll();
      }
    ),

    // AI-powered commands
    vscode.commands.registerCommand("cortex.suggestTagsAI", () =>
      suggestTagsAI(llmService, metadataStore, mirrorStore, workspaceRoot)
    ),

    vscode.commands.registerCommand("cortex.suggestProjectAI", () =>
      suggestProjectAI(llmService, metadataStore, mirrorStore, workspaceRoot)
    ),

    vscode.commands.registerCommand("cortex.generateSummaryAI", () =>
      generateSummaryAI(llmService, metadataStore, mirrorStore, workspaceRoot)
    ),

    // Refresh views command (used by AI commands)
    vscode.commands.registerCommand("cortex.refreshViews", refreshAllViews),
  ];

  // Register disposables
  context.subscriptions.push(
    contextTreeView,
    tagTreeView,
    typeTreeView,
    dateTreeView,
    sizeTreeView,
    folderTreeView,
    contentTypeTreeView,
    codeMetricsTreeView,
    documentMetricsTreeView,
    issuesTreeView,
    categoryTreeView,
    { dispose: () => basicPool.dispose() },
    { dispose: () => contentTypePool.dispose() },
    { dispose: () => codePool.dispose() },
    { dispose: () => documentPool.dispose() },
    ...commands
  );

  const documentExtensions = new Set([
    ".docx",
    ".doc",
    ".xlsx",
    ".xls",
    ".pptx",
    ".ppt",
    ".pdf",
    ".psd",
    ".odt",
    ".ods",
  ]);

  const pendingReindex = new Map<string, FileIndexEntry>();
  let reindexTimer: NodeJS.Timeout | undefined;
  let reindexRunning = false;

  const reindexFilesIncremental = async (files: FileIndexEntry[]) => {
    const eligible = files.filter(
      (file) => !blacklistStore.isBlacklisted(file.relativePath)
    );
    if (eligible.length === 0) {
      return;
    }

    const mirrorConfig = vscode.workspace.getConfiguration("cortex.mirror");
    const maxMirrorFileSizeMb = mirrorConfig.get<number>(
      "maxFileSizeMB",
      25
    );
    const maxMirrorFileSizeBytes = Math.max(1, maxMirrorFileSizeMb) * 1024 * 1024;
    const mirrorEligible = eligible.filter(
      (file) =>
        mirrorStore.isMirrorableExtension(file.extension) &&
        file.fileSize > 0 &&
        file.fileSize <= maxMirrorFileSizeBytes
    );

    await Promise.all(
      eligible.map(async (file) => {
        try {
          const enhanced = await metadataExtractor.extractBasic(
            file.absolutePath,
            file.relativePath,
            file.extension
          );
          await metadataExtractor.extractMimeTypeMetadata(
            file.absolutePath,
            enhanced
          );
          if (enhanced.language) {
            await metadataExtractor.extractCodeMetadata(
              file.absolutePath,
              enhanced
            );
          }
          if (documentExtensions.has(file.extension.toLowerCase())) {
            await metadataExtractor.extractDocumentMetadata(
              file.absolutePath,
              enhanced,
              file.extension
            );
          }
          file.enhanced = enhanced;
        } catch (error) {
          console.warn(
            `[Cortex] Incremental reindex failed for ${file.relativePath}:`,
            error
          );
        }
      })
    );

    await Promise.all(
      mirrorEligible.map(async (file) => {
        const mirrorPath = mirrorStore.getMirrorPath(
          file.relativePath,
          file.extension
        );
        if (
          mirrorPath &&
          file.enhanced?.mirror?.sourceMtime === file.lastModified &&
          file.enhanced.indexed?.mirror &&
          (await mirrorStore.mirrorExists(file.relativePath, file.extension))
        ) {
          return;
        }
        const content = await mirrorStore.ensureMirrorContent(
          file.absolutePath,
          file.relativePath,
          file.extension
        );
        if (content != null && mirrorPath) {
          updateMirrorTracking(file, mirrorPath, file.lastModified);
        }
      })
    );

    await startAISummaryIndexing(
      indexStore,
      metadataStore,
      llmService,
      mirrorStore,
      workspaceRoot,
      eligible,
      scheduleCacheSave,
      false
    );
    await startAITagProjectIndexing(
      indexStore,
      metadataStore,
      llmService,
      mirrorStore,
      workspaceRoot,
      eligible,
      scheduleCacheSave,
      false,
      refreshAllViews
    );

    scheduleRefreshAllViews();
    scheduleCacheSave();
  };

  const runReindexQueue = async () => {
    if (reindexRunning) {
      return;
    }
    reindexRunning = true;
    try {
      while (pendingReindex.size > 0) {
        const batch = Array.from(pendingReindex.values());
        pendingReindex.clear();
        await reindexFilesIncremental(batch);
      }
    } finally {
      reindexRunning = false;
    }
  };

  const scheduleReindex = (file: FileIndexEntry) => {
    pendingReindex.set(file.relativePath, file);
    if (reindexTimer) {
      return;
    }
    reindexTimer = setTimeout(() => {
      reindexTimer = undefined;
      void runReindexQueue();
    }, 1500);
  };

  // File watcher for incremental updates (optional enhancement)
  const fileWatcher = vscode.workspace.createFileSystemWatcher("**/*");

  fileWatcher.onDidCreate(async (uri) => {
    const absolutePath = uri.fsPath;
    const relativePath = path.relative(workspaceRoot, absolutePath);

    // Skip ignored directories
    if (shouldIgnorePath(relativePath)) {
      return;
    }
    if (blacklistStore.isBlacklisted(relativePath)) {
      return;
    }

    const fileEntry = await fileScanner.getFileEntry(absolutePath);
    if (fileEntry) {
      indexStore.upsertFile(fileEntry);
      const renameCandidate = findRenameCandidate(fileEntry);
      if (renameCandidate) {
        recentDeletes.delete(renameCandidate.file.relativePath);
        await migrateMetadata(renameCandidate, fileEntry);
      }
      metadataStore.getOrCreateMetadata(
        fileEntry.relativePath,
        fileEntry.extension
      );
      scheduleReindex(fileEntry);
      if (isOsTaggingSupported()) {
        void syncStoreTagsFromOs(
          metadataStore,
          fileEntry.relativePath,
          fileEntry.absolutePath
        )
          .then((changed) => {
            if (changed) {
              refreshAllViews();
            }
          })
          .catch((error) => {
            console.warn(
              `[Cortex] Failed to sync OS tags for ${fileEntry.relativePath}:`,
              error
            );
          });
      }
      refreshAllViews();
      scheduleCacheSave();
    }
  });

  fileWatcher.onDidChange(async (uri) => {
    const absolutePath = uri.fsPath;
    const relativePath = path.relative(workspaceRoot, absolutePath);

    if (shouldIgnorePath(relativePath)) {
      return;
    }
    if (blacklistStore.isBlacklisted(relativePath)) {
      return;
    }

    const fileEntry = await fileScanner.getFileEntry(absolutePath);
    if (fileEntry) {
      indexStore.upsertFile(fileEntry);
      metadataStore.getOrCreateMetadata(
        fileEntry.relativePath,
        fileEntry.extension
      );
      scheduleReindex(fileEntry);
      scheduleRefreshAllViews();
      scheduleCacheSave();
    }
  });

  fileWatcher.onDidDelete((uri) => {
    const relativePath = path.relative(workspaceRoot, uri.fsPath);
    const existing = indexStore.getFile(relativePath);
    indexStore.removeFile(relativePath);
    if (existing) {
      const metadata = metadataStore.getMetadataByPath(
        relativePath
      ) as FileMetadata | null;
      recentDeletes.set(relativePath, {
        file: existing,
        metadata,
        deletedAt: Date.now(),
      });
      scheduleDeleteCleanup();
    } else {
      removeMetadataEntry(relativePath);
      void mirrorStore.removeMirror(relativePath, path.extname(relativePath));
    }
    refreshAllViews();
    scheduleCacheSave();
  });

  context.subscriptions.push(fileWatcher);

  // Run initial scan and background indexing without blocking activation.
  (async () => {
    try {
      // Phase 1: Instant file scan (no metadata extraction - super fast!)
      let files: FileIndexEntry[] = [];
      const hasCache = cachedEntries.length > 0;
      const scanMessage = hasCache ? "Detectando cambios" : "Escaneando";
      const runWorkspaceScan = async (
        progress?: vscode.Progress<{ message?: string }>
      ) => {
        updateIndexingStatus({
          phase: "scanning",
          message: scanMessage,
          processed: 0,
          total: 0,
          isIndexing: true,
        });
        console.log("[Cortex] Starting workspace scan...");
        files = await fileScanner.scanWorkspace((processed) => {
          updateIndexingStatus({
            phase: "scanning",
            message: scanMessage,
            processed,
            total: 0,
            isIndexing: true,
          });
        });
        const cachedMap = new Map(
          cachedEntries.map((entry) => [entry.relativePath, entry])
        );
        const merged: FileIndexEntry[] = [];
        changedFiles = [];

        for (const file of files) {
          const cached = cachedMap.get(file.relativePath);
          if (
            cached &&
            cached.lastModified === file.lastModified &&
            cached.fileSize === file.fileSize
          ) {
            merged.push(cached);
          } else {
            merged.push(file);
            changedFiles.push(file);
          }
          cachedMap.delete(file.relativePath);
        }

        indexStore.buildIndex(merged);
        files = merged;
        console.log(
          `[Cortex] Scan merge complete: ${merged.length} total, ${changedFiles.length} changed/new, ${cachedMap.size} deleted`
        );
        console.log("[Cortex] Workspace scan finished, building index...");

        // Initialize empty enhanced metadata for all files
        let initializedDefaults = 0;
        files.forEach((file) => {
          if (!file.enhanced) {
            file.enhanced = {
              stats: {
                size: 0,
                created: 0,
                modified: 0,
                accessed: 0,
                isReadOnly: false,
                isHidden: false,
              },
              folder: file.relativePath.includes("/")
                ? file.relativePath.substring(
                    0,
                    file.relativePath.lastIndexOf("/")
                  )
                : ".",
              depth: file.relativePath.split("/").length - 1,
            };
            initializedDefaults += 1;
          }
        });
        if (initializedDefaults > 0) {
          console.log(
            `[Cortex] Initialized defaults for ${initializedDefaults} files`
          );
        }

        const createdMetadata = metadataStore.ensureMetadataForFiles(
          files.map((file) => ({
            relativePath: file.relativePath,
            extension: file.extension,
          }))
        );
        if (createdMetadata > 0) {
          console.log(`[Cortex] Created ${createdMetadata} metadata entries`);
        }

        hydrateMirrorMetadata(files);
        await verifyMirrorCache(files);

        progress?.report({ message: `Found ${files.length} files` });
      };

      if (hasCache) {
        await runWorkspaceScan();
      } else {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: "Cortex: Scanning workspace...",
            cancellable: false,
          },
          async (progress) => {
            await runWorkspaceScan(progress);
          }
        );
      }

      console.log(`[Cortex] Indexed ${indexStore.getStats().totalFiles} files`);
      updateIndexingStatus({
        phase: "done",
        message: "Escaneo completo",
        processed: files.length,
        total: files.length,
        isIndexing: false,
      });
      updateIndexingStatus({
        phase: "basic",
        message: "Metadata basica",
        processed: 0,
        total: files.length,
        isIndexing: true,
      });
      refreshAllViews();
      scheduleCacheSave();
      if (isOsTaggingSupported()) {
        void syncOsTagsForFiles(
          files.filter(
            (file) => !blacklistStore.isBlacklisted(file.relativePath)
          ),
          metadataStore,
          refreshAllViews
        );
      }

      // Phase 2: Independent background indexing - each runs separately with progress
      console.log("[Cortex] Starting independent background indexers...");
      updateIndexingStatus({
        phase: "contentTypes",
        message: "Preparando tipos de contenido",
        processed: 0,
        total: 0,
        isIndexing: true,
      });
      updateIndexingStatus({
        phase: "code",
        message: "Preparando metricas de codigo",
        processed: 0,
        total: 0,
        isIndexing: true,
      });
      updateIndexingStatus({
        phase: "documents",
        message: "Preparando documentos",
        processed: 0,
        total: 0,
        isIndexing: true,
      });

      // 1. Basic metadata first (stats, folder, language) - needed by most views
      console.log("[Cortex] Starting basic metadata indexing...");
      const currentFiles = indexStore.getAllFiles();
      const filesNeedingBasicMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter(
          (file) =>
            !file.enhanced?.indexed?.basic &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) => filesNeedingBasicMap.set(file.relativePath, file));
      changedFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) => filesNeedingBasicMap.set(file.relativePath, file));
      const filesNeedingBasic = Array.from(filesNeedingBasicMap.values());

      const filesNeedingContentTypeMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter(
          (file) =>
            !file.enhanced?.indexed?.mime &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) =>
          filesNeedingContentTypeMap.set(file.relativePath, file)
        );
      changedFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) =>
          filesNeedingContentTypeMap.set(file.relativePath, file)
        );
      const filesNeedingContentType = Array.from(
        filesNeedingContentTypeMap.values()
      );

      const filesNeedingCodeMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter(
          (file) =>
            file.enhanced?.language &&
            !file.enhanced?.indexed?.code &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) => filesNeedingCodeMap.set(file.relativePath, file));
      changedFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) => filesNeedingCodeMap.set(file.relativePath, file));
      const filesNeedingCode = Array.from(filesNeedingCodeMap.values());

      const filesNeedingDocsMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter(
          (file) =>
            file.enhanced &&
            !file.enhanced?.indexed?.document &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) => filesNeedingDocsMap.set(file.relativePath, file));
      changedFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) => filesNeedingDocsMap.set(file.relativePath, file));
      const filesNeedingDocs = Array.from(filesNeedingDocsMap.values());

      const filesNeedingSummaryMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) => filesNeedingSummaryMap.set(file.relativePath, file));
      changedFiles
        .filter((file) => !blacklistStore.isBlacklisted(file.relativePath))
        .forEach((file) => filesNeedingSummaryMap.set(file.relativePath, file));
      const filesNeedingSummary = Array.from(
        filesNeedingSummaryMap.values()
      );

      const filesNeedingMirrorsMap = new Map<string, FileIndexEntry>();
      currentFiles
        .filter(
          (file) =>
            mirrorStore.isMirrorableExtension(file.extension) &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) => filesNeedingMirrorsMap.set(file.relativePath, file));
      changedFiles
        .filter(
          (file) =>
            mirrorStore.isMirrorableExtension(file.extension) &&
            !blacklistStore.isBlacklisted(file.relativePath)
        )
        .forEach((file) => filesNeedingMirrorsMap.set(file.relativePath, file));
      const filesNeedingMirrors = Array.from(
        filesNeedingMirrorsMap.values()
      );

      const scheduleIndexingSave = () => scheduleCacheSave(15000);

      await startBasicIndexing(
        indexStore,
        metadataExtractor,
        (patch) => updateIndexingStatus(patch),
        filesNeedingBasic,
        scheduleIndexingSave,
        basicPool,
        workspaceRoot
      );

      const projectAutoAssignConfig = vscode.workspace.getConfiguration(
        "cortex.projectAutoAssign"
      );
      const autoAssignEnabled = projectAutoAssignConfig.get<boolean>(
        "enabled",
        true
      );
      if (autoAssignEnabled) {
        const windowHours = projectAutoAssignConfig.get<number>(
          "windowHours",
          6
        );
        const minClusterSize = projectAutoAssignConfig.get<number>(
          "minClusterSize",
          2
        );
        const dominanceThreshold = projectAutoAssignConfig.get<number>(
          "dominanceThreshold",
          0.6
        );
        const suggestionThreshold = projectAutoAssignConfig.get<number>(
          "suggestionThreshold",
          0.3
        );
        const windowMs = Math.max(1, windowHours) * 60 * 60 * 1000;
        const projectAutoAssigner = new ProjectAutoAssigner(
          metadataStore,
          indexStore,
          {
            windowMs,
            minClusterSize: Math.max(1, minClusterSize),
            dominanceThreshold: Math.min(Math.max(dominanceThreshold, 0), 1),
            suggestionThreshold: Math.min(
              Math.max(suggestionThreshold, 0),
              1
            ),
          }
        );
        const assignmentResult = projectAutoAssigner.assignProjects(
          indexStore.getAllFiles(),
          (file) => blacklistStore.isBlacklisted(file.relativePath)
        );
        if (assignmentResult.assignedCount > 0) {
          console.log(
            `[Cortex] Auto-assigned ${assignmentResult.assignedCount} project links across ${assignmentResult.filesUpdated} files`
          );
        }
      }

      // Refresh all views after basic metadata is ready
      refreshAllViews();

      // 2. Then run specialized indexers in parallel
      await Promise.all([
        (async () => {
          console.log("[Cortex] Starting content type indexing...");
          return startContentTypeIndexing(
            workspaceRoot,
            indexStore,
            metadataExtractor,
            contentTypeTreeProvider,
            (patch) => updateIndexingStatus(patch),
            filesNeedingContentType,
            contentTypePool,
            scheduleIndexingSave
          );
        })(),
        (async () => {
          console.log("[Cortex] Starting code metrics indexing...");
          return startCodeMetricsIndexing(
            workspaceRoot,
            indexStore,
            metadataExtractor,
            codeMetricsTreeProvider,
            (patch) => updateIndexingStatus(patch),
            filesNeedingCode,
            codePool,
            scheduleIndexingSave
          );
        })(),
        (async () => {
          console.log("[Cortex] Starting document indexing...");
          return startDocumentIndexing(
            workspaceRoot,
            indexStore,
            metadataExtractor,
            mirrorStore,
            updateMirrorTracking,
            documentMetricsTreeProvider,
            (patch) => updateIndexingStatus(patch),
            filesNeedingDocs,
            documentPool,
            scheduleIndexingSave
          );
        })(),
        (async () => {
          console.log("[Cortex] Starting mirror indexing...");
          return startMirrorIndexing(
            mirrorStore,
            updateMirrorTracking,
            filesNeedingMirrors,
            scheduleIndexingSave
          );
        })(),
      ]);

      console.log("[Cortex] Starting AI summary indexing...");
      await startAISummaryIndexing(
        indexStore,
        metadataStore,
        llmService,
        mirrorStore,
        workspaceRoot,
        filesNeedingSummary,
        scheduleIndexingSave
      );
      await startAITagProjectIndexing(
        indexStore,
        metadataStore,
        llmService,
        mirrorStore,
        workspaceRoot,
        filesNeedingSummary,
        scheduleIndexingSave,
        true,
        refreshAllViews
      );

      console.log("[Cortex] ✓ All background indexing complete");
      scheduleCacheSave(0);
      vscode.window.showInformationMessage(
        "Cortex: Workspace indexing complete!"
      );
    } catch (error) {
      console.error("[Cortex] Background indexing failed:", error);
      updateIndexingStatus({
        phase: "error",
        message: "Error de indexacion",
        processed: 0,
        total: 0,
        isIndexing: false,
      });
      updateIndexingStatus({
        phase: "error",
        message: "Error de indexacion",
        processed: 0,
        total: 0,
        isIndexing: false,
      });
      updateIndexingStatus({
        phase: "error",
        message: "Error de indexacion",
        processed: 0,
        total: 0,
        isIndexing: false,
      });
      updateIndexingStatus({
        phase: "error",
        message: "Error de indexacion",
        processed: 0,
        total: 0,
        isIndexing: false,
      });
      updateIndexingStatus({
        phase: "error",
        message: "Error de indexacion",
        processed: 0,
        total: 0,
        isIndexing: false,
      });
      vscode.window.showErrorMessage(
        "Cortex: Indexing failed. Check Debug Console for details."
      );
    }
  })();

  console.log("[Cortex] Extension activated successfully");

  // Show welcome message (first time only)
  const hasShownWelcome = context.globalState.get("cortex.hasShownWelcome");
  if (!hasShownWelcome) {
    vscode.window
      .showInformationMessage(
        "Cortex is ready! Start organizing your files with tags and projects.",
        "Open Cortex View",
        "Dismiss"
      )
      .then((selection) => {
        if (selection === "Open Cortex View") {
          openViewCommand();
        }
      });
    context.globalState.update("cortex.hasShownWelcome", true);
  }

  // Función helper para actualizar la vista de información del archivo
  const updateFileInfoView = () => {
    const editor = vscode.window.activeTextEditor;
    if (editor) {
      const absolutePath = editor.document.uri.fsPath;
      const relativePath = path.relative(workspaceRoot, absolutePath);
      
      // Verificar que el archivo esté en el workspace
      if (!relativePath.startsWith('..')) {
        fileInfoTreeProvider.updateCurrentFile(relativePath);
      } else {
        fileInfoTreeProvider.updateCurrentFile(null);
      }
    } else {
      fileInfoTreeProvider.updateCurrentFile(null);
    }
  };

  // Actualizar la vista cuando cambie el archivo activo
  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(() => {
      updateFileInfoView();
    })
  );

  // Inicializar con el archivo actual si hay uno abierto
  updateFileInfoView();
}

/**
 * Extension deactivation
 */
export function deactivate() {
  console.log("[Cortex] Deactivating extension...");
}

/**
 * Helper to determine if a path should be ignored
 */
function shouldIgnorePath(relativePath: string): boolean {
  const ignoredDirs = [
    ".git",
    "node_modules",
    ".vscode",
    ".cortex",
    "dist",
    "build",
  ];

  for (const dir of ignoredDirs) {
    if (relativePath.startsWith(dir + path.sep) || relativePath === dir) {
      return true;
    }
  }

  return false;
}
