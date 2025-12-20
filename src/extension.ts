/**
 * Cortex Extension - Main entry point
 */

import * as vscode from "vscode";
import * as path from "path";
import * as os from "os";

// Core components
import { FileScanner } from "./core/FileScanner";
import { IndexStore } from "./core/IndexStore";
import { MetadataStoreJSON as MetadataStore } from "./core/MetadataStoreJSON";
import { IndexingStatus } from "./core/IndexingStatus";
import { IndexCache } from "./core/IndexCache";
import { FileIndexEntry } from "./models/types";
import { BlacklistStore } from "./core/BlacklistStore";
import { WorkerPool } from "./core/WorkerPool";

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

// Metadata extraction
import { MetadataExtractor } from "./extractors/MetadataExtractor";
import { EnhancedMetadata } from "./extractors/MetadataExtractor";

// Commands
import { addTagCommand } from "./commands/addTag";
import { assignContextCommand } from "./commands/assignContext";
import { openViewCommand } from "./commands/openView";
import { rebuildIndexCommand } from "./commands/rebuildIndex";

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
  const metadataStore = new MetadataStore(workspaceRoot);
  const metadataExtractor = new MetadataExtractor(workspaceRoot);
  const indexCache = new IndexCache(workspaceRoot);
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

  // Initialize metadata store
  await metadataStore.initialize();

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
    basicStatus,
    blacklistStore
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
    { dispose: () => basicPool.dispose() },
    { dispose: () => contentTypePool.dispose() },
    { dispose: () => codePool.dispose() },
    { dispose: () => documentPool.dispose() },
    ...commands
  );

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
      metadataStore.getOrCreateMetadata(
        fileEntry.relativePath,
        fileEntry.extension
      );
      refreshAllViews();
      scheduleCacheSave();
    }
  });

  fileWatcher.onDidDelete((uri) => {
    const relativePath = path.relative(workspaceRoot, uri.fsPath);
    indexStore.removeFile(relativePath);
    refreshAllViews();
    scheduleCacheSave();
  });

  context.subscriptions.push(fileWatcher);

  // Run initial scan and background indexing without blocking activation.
  (async () => {
    try {
      // Phase 1: Instant file scan (no metadata extraction - super fast!)
      let files: FileIndexEntry[] = [];
      updateIndexingStatus({
        phase: "scanning",
        message: "Escaneando",
        processed: 0,
        total: 0,
        isIndexing: true,
      });
      await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Cortex: Scanning workspace...",
          cancellable: false,
        },
        async (progress) => {
          console.log("[Cortex] Starting workspace scan...");
          files = await fileScanner.scanWorkspace((processed) => {
            updateIndexingStatus({
              phase: "scanning",
              message: "Escaneando",
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

          progress.report({ message: `Found ${files.length} files` });
        }
      );

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
            documentMetricsTreeProvider,
            (patch) => updateIndexingStatus(patch),
            filesNeedingDocs,
            documentPool,
            scheduleIndexingSave
          );
        })(),
      ]);

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
        "Cortex is ready! Start organizing your files with tags and contexts.",
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
