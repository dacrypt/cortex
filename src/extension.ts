/**
 * Cortex Extension - Main entry point
 * 
 * Frontend-only: No local indexing, classification, or detection.
 * All data comes from the backend via gRPC.
 */

import * as vscode from "vscode";
import * as path from "node:path";

// Backend clients
import { GrpcAdminClient } from "./core/GrpcAdminClient";
import { GrpcMetadataClient } from "./core/GrpcMetadataClient";
import { GrpcRAGClient } from "./core/GrpcRAGClient";
import { GrpcKnowledgeClient } from "./core/GrpcKnowledgeClient";
import { GrpcLLMClient } from "./core/GrpcLLMClient";
import { GrpcPreferencesClient } from "./core/GrpcPreferencesClient";
import { GrpcTaxonomyClient } from "./core/GrpcTaxonomyClient";
import { GrpcClusteringClient } from "./core/GrpcClusteringClient";
import { BackendMetadataStore } from "./core/BackendMetadataStore";
import { IMetadataStore } from "./core/IMetadataStore";
import { FileCacheService } from "./core/FileCacheService";
import { ProjectInferenceService } from "./services/ProjectInferenceService";

// View providers
import { FileInfoTreeProvider } from "./views/FileInfoTreeProvider";
import { FileInfoWebviewProvider } from "./frontend/FileInfoWebview";
import { UnifiedFacetTreeProvider } from "./views/UnifiedFacetTreeProvider";
import { CortexTreeProvider } from "./views/CortexTreeProvider";
import { FacetProviderContext } from "./views/contracts/IFacetProvider";
import { TaxonomyTreeProvider, registerTaxonomyCommands } from "./views/TaxonomyTreeProvider";
import { registerClusterGraphCommands } from "./frontend/ClusterGraphWebview";
import { registerSFSCommandInput } from "./frontend/SFSCommandInputWebview";

// Commands
import { addTagCommand } from "./commands/addTag";
import { openViewCommand } from "./commands/openView";
import { suggestTagsAI } from "./commands/suggestTagsAI";
import { suggestProjectAI } from "./commands/suggestProjectAI";
import { generateSummaryAI } from "./commands/generateSummaryAI";
import { askAICommand } from "./commands/askAI";
import { openBackendFrontendCommand } from "./commands/openBackendFrontend";
import { openPipelineProgressCommand } from "./commands/openPipelineProgress";
import { openMetricsDashboardCommand } from "./commands/openMetricsDashboard";
import { openWordEditorCommand } from "./commands/openWordEditor";
import { executeSemanticCommand, showCommandHistory, disposeSFSClient } from "./commands/executeSemanticCommand";
import { copyTreeItemTextCommand } from "./commands/copyTreeItemText";
import { OnlyOfficeBridge } from "./services/OnlyOfficeBridge";
import { BackendManager } from "./core/BackendManager";

/**
 * Extension state stored in context for command access
 */
interface ExtensionState {
  workspaceRoot?: string;
  backendWorkspaceId?: string;
  adminClient?: GrpcAdminClient;
  metadataClient?: GrpcMetadataClient;
  ragClient?: GrpcRAGClient;
  knowledgeClient?: GrpcKnowledgeClient;
  metadataStore?: IMetadataStore;
  fileCacheService?: FileCacheService;
  refreshAllViews?: () => void;
  projectInferenceService?: ProjectInferenceService;
  cortexTreeView?: vscode.TreeView<vscode.TreeItem>;
  cortexTreeProvider?: CortexTreeProvider;
  onlyOfficeBridge?: OnlyOfficeBridge;
  backendManager?: BackendManager;
}

/**
 * Module-level extension state (since ExtensionContext is not extensible)
 */
let globalExtensionState: ExtensionState | undefined;

/**
 * Recursively expand all nodes in a tree view
 */
async function expandAllNodes<T extends vscode.TreeItem>(
  treeView: vscode.TreeView<T>,
  provider: vscode.TreeDataProvider<T>
): Promise<void> {
  async function expandNode(item: T): Promise<void> {
    if (item.collapsibleState !== vscode.TreeItemCollapsibleState.None) {
      try {
        await treeView.reveal(item, { expand: true, focus: false, select: false });
        // Get children and expand them recursively
        const children = await provider.getChildren(item);
        if (children) {
          for (const child of children) {
            await expandNode(child);
          }
        }
      } catch (error) {
        // Ignore errors for individual nodes (they might not be loaded yet)
        console.debug(`[Cortex] Could not expand node: ${item.label}`, error);
      }
    }
  }

  // Start with root nodes
  const rootItems = await provider.getChildren();
  if (rootItems) {
    for (const rootItem of rootItems) {
      await expandNode(rootItem);
    }
  }
}

async function checkLLMAvailability(
  context: vscode.ExtensionContext,
  adminClient: GrpcAdminClient
): Promise<void> {
  try {
    const llmClient = new GrpcLLMClient(context);
    const config = await adminClient.getConfig();
    const defaultProvider = config?.llm?.default_provider;

    if (defaultProvider) {
      const status = await llmClient.getProviderStatus(defaultProvider);
      const available = Boolean(status?.provider?.available);
      const connected = Boolean(status?.connected);
      if (!available || !connected) {
        const detail = status?.error ? ` (${status.error})` : "";
        vscode.window.showWarningMessage(
          `Cortex: LLM provider "${defaultProvider}" is unavailable. AI processing will stall until it recovers${detail}.`
        );
      }
      return;
    }

    const providers = await llmClient.listProviders();
    const anyAvailable = providers.some((provider) => provider?.available);
    if (!anyAvailable) {
      vscode.window.showWarningMessage(
        "Cortex: No LLM providers are available. AI processing will stall until a provider is reachable."
      );
    }
  } catch (error) {
    console.warn("[Cortex] Failed to check LLM availability:", error);
  }
}

/**
 * Register all commands - commands get state from extensionState in context
 */
function registerAllCommands(context: vscode.ExtensionContext, extensionState: ExtensionState): vscode.Disposable[] {
  // Store state in module-level variable for command access
  globalExtensionState = extensionState;
  
  const commands: vscode.Disposable[] = [
    vscode.commands.registerCommand("cortex.addTag", () => {
      const state = globalExtensionState;
      if (!state?.workspaceRoot || !state?.metadataStore || !state?.refreshAllViews) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      return addTagCommand(state.workspaceRoot, state.metadataStore, state.refreshAllViews);
    }),
    vscode.commands.registerCommand("cortex.assignContext", async (item?: vscode.Uri | vscode.TreeItem | { resourceUri?: vscode.Uri }) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot) {
        vscode.window.showErrorMessage('Cortex: Workspace not registered with backend');
        return;
      }
      const { assignProjectCommand } = await import('./commands/assignProject');
      await assignProjectCommand(
        state.workspaceRoot,
        state.knowledgeClient || new GrpcKnowledgeClient(context),
        state.adminClient || new GrpcAdminClient(context),
        state.backendWorkspaceId,
        state.refreshAllViews || (() => {}),
        context,
        item
      );
    }),
    vscode.commands.registerCommand("cortex.createProject", async () => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot) {
        vscode.window.showErrorMessage('Cortex: Workspace not registered with backend');
        return;
      }
      const { createProjectCommand } = await import('./commands/createProject');
      await createProjectCommand(
        state.workspaceRoot,
        state.knowledgeClient || new GrpcKnowledgeClient(context),
        state.backendWorkspaceId,
        state.refreshAllViews || (() => {}),
        context
      );
    }),
    vscode.commands.registerCommand("cortex.editProject", async (projectIdOrItem?: string | { projectId?: string }) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot) {
        vscode.window.showErrorMessage('Cortex: Workspace not registered with backend');
        return;
      }
      const { editProjectCommand } = await import('./commands/createProject');
      const knowledgeClient = state.knowledgeClient || new GrpcKnowledgeClient(context);
      
      // Handle both projectId string and tree item
      let selectedProjectId: string | undefined;
      if (typeof projectIdOrItem === 'string') {
        selectedProjectId = projectIdOrItem;
      } else if (projectIdOrItem && 'projectId' in projectIdOrItem) {
        selectedProjectId = projectIdOrItem.projectId;
      }
      
      // If projectId not provided, let user select
      if (!selectedProjectId) {
        const projects = await knowledgeClient.listProjects(state.backendWorkspaceId);
        if (projects.length === 0) {
          vscode.window.showInformationMessage('No projects found');
          return;
        }
        const selected = await vscode.window.showQuickPick(
          projects.map(p => ({ 
            label: p.name, 
            description: p.description || (p.nature && p.nature !== 'generic' ? `Type: ${p.nature}` : undefined), 
            id: p.id 
          })),
          { placeHolder: 'Select project to edit' }
        );
        if (!selected) return;
        selectedProjectId = selected.id;
      }
      
      await editProjectCommand(
        state.workspaceRoot,
        knowledgeClient,
        state.backendWorkspaceId,
        selectedProjectId,
        state.refreshAllViews || (() => {}),
        context
      );
    }),
    openViewCommand(),
    vscode.commands.registerCommand("cortex.rebuildIndex", async () => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.adminClient) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized. Please reload the window.');
        return;
      }

      // Confirm with user that this will re-process everything
      const confirm = await vscode.window.showWarningMessage(
        'This will re-index and re-convert all files in the workspace. This includes:\n' +
        '• Re-scanning all files\n' +
        '• Re-extracting document content (PDFs, Office files, etc.)\n' +
        '• Re-generating embeddings for RAG\n' +
        '• Re-processing metadata and metrics\n\n' +
        'This may take a while for large workspaces. Continue?',
        { modal: true },
        'Yes, Re-index Everything',
        'Cancel'
      );

      if (confirm !== 'Yes, Re-index Everything') {
        return;
      }

      await vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: 'Cortex: Full Re-index & Re-convert',
          cancellable: false,
        },
        async (progress) => {
          progress.report({ increment: 0, message: 'Starting full workspace scan...' });
          try {
            // Clear shared cache so all providers get fresh data
            if (state.fileCacheService) {
              state.fileCacheService.clear();
            }
            
            // Force full scan: re-processes all files, re-extracts content, re-generates embeddings
            if (!state.adminClient || !state.backendWorkspaceId || !state.workspaceRoot) {
              throw new Error('Extension not fully initialized');
            }
            await state.adminClient.scanWorkspace(state.backendWorkspaceId, state.workspaceRoot, true);
            progress.report({ increment: 50, message: 'Scan complete, processing files...' });
            
            // Wait a moment for processing to start
            await new Promise((resolve) => setTimeout(resolve, 1000));
            
            progress.report({ increment: 100, message: 'Full re-index started on backend' });
            if (state.refreshAllViews) {
              state.refreshAllViews();
            }
            
            vscode.window.showInformationMessage(
              'Cortex: Full re-index started. Files will be re-processed in the background. ' +
              'Check the Backend Frontend for progress.',
              'Open Backend Frontend'
            ).then((selection) => {
              if (selection === 'Open Backend Frontend') {
                vscode.commands.executeCommand('cortex.openBackendFrontend');
              }
            });
          } catch (error) {
            vscode.window.showErrorMessage(
              `Cortex: Failed to trigger re-index: ${error instanceof Error ? error.message : String(error)}`
            );
          }
        }
      );
    }),
    vscode.commands.registerCommand("cortex.suggestTagsAI", () => {
      const state = globalExtensionState;
      if (!state?.adminClient || !state?.workspaceRoot || !state?.backendWorkspaceId) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      return suggestTagsAI(state.adminClient, state.workspaceRoot, state.backendWorkspaceId);
    }),
    vscode.commands.registerCommand("cortex.suggestProjectAI", () => {
      const state = globalExtensionState;
      if (!state?.adminClient || !state?.workspaceRoot || !state?.backendWorkspaceId) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      return suggestProjectAI(state.adminClient, state.workspaceRoot, state.backendWorkspaceId);
    }),
    vscode.commands.registerCommand("cortex.generateSummaryAI", () => {
      const state = globalExtensionState;
      if (!state?.adminClient || !state?.workspaceRoot || !state?.backendWorkspaceId) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      return generateSummaryAI(state.adminClient, state.workspaceRoot, state.backendWorkspaceId);
    }),
    vscode.commands.registerCommand("cortex.acceptSuggestedTag", async (tag: string) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.metadataClient || !state?.adminClient) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      const { acceptSuggestedTagCommand } = await import('./commands/acceptSuggestion');
      await acceptSuggestedTagCommand(
        state.workspaceRoot,
        state.metadataClient,
        state.adminClient,
        state.backendWorkspaceId,
        tag,
        state.refreshAllViews || (() => {})
      );
    }),
    vscode.commands.registerCommand("cortex.acceptSuggestedProject", async (projectName: string) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.metadataClient || !state?.adminClient) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      const { acceptSuggestedProjectCommand } = await import('./commands/acceptSuggestion');
      await acceptSuggestedProjectCommand(
        state.workspaceRoot,
        state.metadataClient,
        state.adminClient,
        state.knowledgeClient || new GrpcKnowledgeClient(context),
        state.backendWorkspaceId,
        projectName,
        state.refreshAllViews || (() => {})
      );
    }),
    vscode.commands.registerCommand("cortex.rejectSuggestedTag", async (tag: string) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.metadataClient || !state?.adminClient) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      const { rejectSuggestedTagCommand } = await import('./commands/acceptSuggestion');
      await rejectSuggestedTagCommand(
        state.workspaceRoot,
        state.metadataClient,
        state.adminClient,
        state.backendWorkspaceId,
        tag,
        state.refreshAllViews || (() => {})
      );
    }),
    vscode.commands.registerCommand("cortex.rejectSuggestedProject", async (projectName: string) => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.metadataClient || !state?.adminClient) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      const { rejectSuggestedProjectCommand } = await import('./commands/acceptSuggestion');
      await rejectSuggestedProjectCommand(
        state.workspaceRoot,
        state.metadataClient,
        state.adminClient,
        state.backendWorkspaceId,
        projectName,
        state.refreshAllViews || (() => {})
      );
    }),
    vscode.commands.registerCommand("cortex.askAI", () => {
      const state = globalExtensionState;
      if (!state?.ragClient || !state?.backendWorkspaceId) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      return askAICommand(state.ragClient, state.backendWorkspaceId);
    }),
    openBackendFrontendCommand(context),
    openPipelineProgressCommand(context),
    vscode.commands.registerCommand("cortex.openMetricsDashboard", async () => {
      const state = globalExtensionState;
      if (!state?.adminClient || !state?.backendWorkspaceId) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      await openMetricsDashboardCommand(context, state.adminClient, state.backendWorkspaceId);
    }),
    vscode.commands.registerCommand("cortex.openWordEditor", async (uri?: vscode.Uri) => {
      const state = globalExtensionState;
      if (!state?.workspaceRoot) {
        vscode.window.showErrorMessage("Cortex: Workspace not registered");
        return;
      }
      await openWordEditorCommand(context, { workspaceRoot: state.workspaceRoot, onlyOfficeBridge: state.onlyOfficeBridge }, uri);
    }),
    vscode.commands.registerCommand("cortex.refreshViews", () => {
      const state = globalExtensionState;
      if (state?.refreshAllViews) {
        state.refreshAllViews();
      }
    }),
    vscode.commands.registerCommand("cortex.verifyViews", async () => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot) {
        vscode.window.showErrorMessage('Cortex: Workspace not registered with backend');
        return;
      }
      // Dynamic import to avoid TypeScript rootDir issues
      // Using Function constructor to bypass TypeScript's rootDir restriction for scripts outside src/
      const importPath = '../scripts/verify-views';
      const verifyViewsModule = await new Function('path', 'return import(path)')(importPath);
      const { verifyViews } = verifyViewsModule;
      await verifyViews(context, state.backendWorkspaceId, state.workspaceRoot);
    }),
    vscode.commands.registerCommand("cortex.executeSemanticCommand", async () => {
      await executeSemanticCommand();
    }),
    vscode.commands.registerCommand("cortex.showCommandHistory", async () => {
      await showCommandHistory();
    }),
    vscode.commands.registerCommand("cortex.syncBackend", async () => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.metadataStore || !state?.fileCacheService || !state?.refreshAllViews) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      try {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: 'Cortex: Syncing with backend...',
            cancellable: false,
          },
          async (progress) => {
            progress.report({ increment: 0, message: 'Refreshing metadata...' });
            // Clear file cache to force refresh
            if (state.fileCacheService) {
              state.fileCacheService.clear();
            }
            // Refresh metadata store if it supports it
            interface RefreshableMetadataStore {
              refreshBase?: () => Promise<void>;
            }
            const refreshableStore = state.metadataStore as RefreshableMetadataStore;
            if (refreshableStore.refreshBase) {
              await refreshableStore.refreshBase();
            }
            progress.report({ increment: 50, message: 'Refreshing views...' });
            if (state.refreshAllViews) {
              state.refreshAllViews();
            }
            progress.report({ increment: 100, message: 'Sync complete' });
          }
        );
        vscode.window.showInformationMessage('Cortex: Backend sync complete');
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to sync with backend: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }),
    vscode.commands.registerCommand("cortex.autoIndexAI", async () => {
      const state = globalExtensionState;
      if (!state?.backendWorkspaceId || !state?.workspaceRoot || !state?.adminClient || !state?.refreshAllViews) {
        vscode.window.showErrorMessage('Cortex: Extension not fully initialized');
        return;
      }
      try {
        await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: 'Cortex: Auto Indexing with AI...',
            cancellable: false,
          },
          async (progress) => {
            progress.report({ increment: 0, message: 'Triggering AI auto-indexing...' });
            // Trigger backend to auto-index with AI
            if (!state.adminClient || !state.backendWorkspaceId || !state.workspaceRoot) {
              throw new Error('Extension not fully initialized');
            }
            await state.adminClient.scanWorkspace(state.backendWorkspaceId, state.workspaceRoot, false);
            progress.report({ increment: 50, message: 'AI indexing in progress...' });
            // Wait a bit for indexing to start
            await new Promise((resolve) => setTimeout(resolve, 1000));
            progress.report({ increment: 100, message: 'AI indexing started' });
            if (state.refreshAllViews) {
              state.refreshAllViews();
            }
          }
        );
        vscode.window.showInformationMessage('Cortex: AI auto-indexing started on backend');
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to start AI auto-indexing: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }),
    vscode.commands.registerCommand("cortex.expandAll", async () => {
      const state = globalExtensionState;
      if (!state?.cortexTreeView || !state?.cortexTreeProvider) {
        vscode.window.showErrorMessage('Cortex: Tree view not available');
        return;
      }
      try {
        await expandAllNodes(state.cortexTreeView, state.cortexTreeProvider);
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to expand all: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }),
    vscode.commands.registerCommand("cortex.collapseAll", async () => {
      const state = globalExtensionState;
      if (!state?.cortexTreeProvider) {
        vscode.window.showErrorMessage('Cortex: Tree provider not available');
        return;
      }
      try {
        // Refresh the tree to collapse all nodes
        state.cortexTreeProvider.refresh();
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to collapse all: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }),
    vscode.commands.registerCommand("cortex.copyTreeItemText", async (item?: vscode.TreeItem | { label?: string | vscode.TreeItemLabel }) => {
      return copyTreeItemTextCommand(item);
    }),
  ];

  return commands;
}

/**
 * Extension activation
 */
export async function activate(context: vscode.ExtensionContext) {
  console.log("[Cortex] Activating extension (backend-only mode)...");

  // Create extension state object (stored in module-level variable)
  const extensionState: ExtensionState = {};
  globalExtensionState = extensionState;

  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    console.log("[Cortex] No workspace folders found");
    // Register commands anyway so they can show appropriate errors
    registerAllCommands(context, extensionState);
        return;
      }

  const workspaceRoot = workspaceFolders[0].uri.fsPath;
  extensionState.workspaceRoot = workspaceRoot;

  // Register commands early (before any early returns) so they're always available
  // Commands will get state from extensionState stored in context
  const commands = registerAllCommands(context, extensionState);
  context.subscriptions.push(...commands);

  // Initialize backend manager (for auto-starting backend)
  const backendManager = new BackendManager(context);
  extensionState.backendManager = backendManager;

  // Initialize backend clients
  const adminClient = new GrpcAdminClient(context);
  const metadataClient = new GrpcMetadataClient(context);
  const ragClient = new GrpcRAGClient(context);
  
  extensionState.adminClient = adminClient;
  extensionState.metadataClient = metadataClient;
  extensionState.ragClient = ragClient;
  
  // Initialize shared file cache service
  const fileCacheService = FileCacheService.getInstance(adminClient);
  extensionState.fileCacheService = fileCacheService;

  // Get or register workspace with backend first (needed for BackendMetadataStore)
  let backendWorkspaceId: string | undefined;
  try {
    const workspaces = await adminClient.listWorkspaces();
    const existingWorkspace = workspaces.find(
      (ws) => ws.path === workspaceRoot
    );
    if (existingWorkspace) {
      backendWorkspaceId = existingWorkspace.id;
      console.log(
        `[Cortex] Using existing workspace: ${backendWorkspaceId}`
          );
        } else {
      const newWorkspace = await adminClient.registerWorkspace(
                  workspaceRoot,
        path.basename(workspaceRoot)
      );
      backendWorkspaceId = newWorkspace.id;
      console.log(
        `[Cortex] Registered new workspace: ${backendWorkspaceId}`
      );
                }
              } catch (error) {
    console.error("[Cortex] Failed to connect to backend:", error);
    
    // Try to auto-start backend if enabled
    const config = vscode.workspace.getConfiguration("cortex");
    const autoStartBackend = config.get<boolean>("autoStartBackend", true);
    
    if (autoStartBackend) {
      try {
        console.log("[Cortex] Attempting to start backend automatically...");
        await backendManager.start();
        
        // Retry connection after starting
        const workspaces = await adminClient.listWorkspaces();
        const existingWorkspace = workspaces.find(
          (ws) => ws.path === workspaceRoot
        );
        if (existingWorkspace) {
          backendWorkspaceId = existingWorkspace.id;
          console.log(`[Cortex] Using existing workspace: ${backendWorkspaceId}`);
        } else {
          const newWorkspace = await adminClient.registerWorkspace(
            workspaceRoot,
            path.basename(workspaceRoot)
          );
          backendWorkspaceId = newWorkspace.id;
          console.log(`[Cortex] Registered new workspace: ${backendWorkspaceId}`);
        }
      } catch (startError) {
        console.error("[Cortex] Failed to start backend:", startError);
        vscode.window.showErrorMessage(
          "Cortex: Failed to start backend automatically. " +
          "Please start the Cortex daemon manually or check the logs."
        );
        // Commands already registered, no need to register again
        return;
      }
    } else {
      vscode.window.showErrorMessage(
        "Cortex: Failed to connect to backend. Make sure the Cortex daemon is running."
      );
      // Commands already registered at line 471, no need to register again
      return;
    }
  }

  if (!backendWorkspaceId) {
    console.error("[Cortex] No backend workspace ID found");
    // Commands already registered at line 471, no need to register again
        return;
      }

  extensionState.backendWorkspaceId = backendWorkspaceId;

  void checkLLMAvailability(context, adminClient);

  // Use backend metadata store (now that we have workspaceId)
  const metadataStore: IMetadataStore = new BackendMetadataStore(
    metadataClient,
    backendWorkspaceId
  );
  await metadataStore.initialize();
  extensionState.metadataStore = metadataStore;

  // Initialize knowledge client
  const knowledgeClient = new GrpcKnowledgeClient(context);
  extensionState.knowledgeClient = knowledgeClient;

  // Initialize clustering client
  const clusteringClient = new GrpcClusteringClient(context);

  // Initialize preferences client for feedback learning
  const preferencesClient = new GrpcPreferencesClient(context);

  // Initialize taxonomy client and provider
  const taxonomyClient = new GrpcTaxonomyClient(context);
  const taxonomyTreeProvider = new TaxonomyTreeProvider(taxonomyClient, backendWorkspaceId);
  registerTaxonomyCommands(context, taxonomyTreeProvider);

  // Register cluster graph commands
  registerClusterGraphCommands(context, backendWorkspaceId, workspaceRoot);

  // Register SFS Command Input webview
  registerSFSCommandInput(context, backendWorkspaceId);

  // Register taxonomy tree view
  const taxonomyTreeView = vscode.window.createTreeView("cortex-taxonomyView", {
    treeDataProvider: taxonomyTreeProvider,
    showCollapseAll: true,
  });
  context.subscriptions.push(taxonomyTreeView);

  // FileInfoTreeProvider - Not a facet, shows details of selected file (legacy)
  const fileInfoTreeProvider = new FileInfoTreeProvider(
    workspaceRoot,
    metadataStore,
    metadataClient,
    adminClient,
    knowledgeClient,
    clusteringClient,
    backendWorkspaceId
  );

  // FileInfoWebviewProvider - Modern webview with rich UI
  const fileInfoWebviewProvider = new FileInfoWebviewProvider(
    context.extensionUri,
    workspaceRoot,
    metadataStore,
    metadataClient,
    adminClient,
    knowledgeClient,
    preferencesClient,
    backendWorkspaceId
  );

  // Register webview provider
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      'cortex-fileInfoWebview',
      fileInfoWebviewProvider
    )
  );

  // ============================================================
  // UNIFIED FACET PROVIDER - All facets in one provider
  // ============================================================
  // This replaces ~40+ individual facet providers with a single
  // unified provider that generates everything dynamically from
  // the FacetRegistry
  // ============================================================
  
  const facetContext: FacetProviderContext = {
    workspaceRoot,
    workspaceId: backendWorkspaceId,
    context,
    fileCacheService,
    knowledgeClient,
    adminClient,
    metadataStore,
  };

  const unifiedFacetProvider = new UnifiedFacetTreeProvider(facetContext);
  // ==========================================================================
  // SIMPLIFIED NAVIGATION - All facets in one unified provider
  // ==========================================================================
  // The UnifiedFacetTreeProvider generates everything dynamically:
  // - Categories (Core, Organization, Temporal, Content, System, Specialized)
  // - Facets within each category
  // - Values for each facet
  // - Files matching each value
  // ==========================================================================
  const cortexTreeProvider = new CortexTreeProvider([
    {
      id: 'facets',
      label: 'Facetas',
      icon: new vscode.ThemeIcon('list-filter'),
      initialState: vscode.TreeItemCollapsibleState.Expanded,
      provider: unifiedFacetProvider,
    },
  ]);
  const cortexTreeView = vscode.window.createTreeView("cortex-mainView", {
    treeDataProvider: cortexTreeProvider,
  });
  const fileInfoTreeView = vscode.window.createTreeView("cortex-fileInfoView", {
    treeDataProvider: fileInfoTreeProvider,
    showCollapseAll: false,
  });

  // Store tree view and provider in extension state for command access
  extensionState.cortexTreeView = cortexTreeView;
  extensionState.cortexTreeProvider = cortexTreeProvider;

  // Conectar la TreeView al provider para permitir expansión automática
  fileInfoTreeProvider.setTreeView(fileInfoTreeView, { autoExpand: true });

  // Refresh function for views
  const refreshAllViews = () => {
    // Clear file cache to ensure fresh data from backend
    fileCacheService.clear();
    // Unified facet provider (replaces all individual facet providers)
    unifiedFacetProvider.refresh();
    // File info providers (tree legacy + webview)
    fileInfoTreeProvider.refresh();
    fileInfoWebviewProvider.refresh();
    // Main tree provider
    cortexTreeProvider.refresh();
  };
  extensionState.refreshAllViews = refreshAllViews;
  metadataStore.setRefreshHandler(refreshAllViews);

  // Initialize real-time update service
  const { RealtimeUpdateService } = await import('./services/RealtimeUpdateService');
  const realtimeUpdateService = new RealtimeUpdateService(context, backendWorkspaceId);
  realtimeUpdateService.registerRefreshCallback(refreshAllViews);
  
  // Initialize project inference service for auto-updating project characteristics
  const { ProjectInferenceService } = await import('./services/ProjectInferenceService');
  const projectInferenceService = new ProjectInferenceService(
    context,
    knowledgeClient,
    ragClient // Use existing ragClient from above
  );
  
  // Store inference service in global state for use in commands
  extensionState.projectInferenceService = projectInferenceService;
  
  // Start real-time updates if workspace is available
  if (backendWorkspaceId) {
    realtimeUpdateService.start();
    console.log('[Cortex] Real-time UI updates enabled');
    console.log('[Cortex] Project inference service initialized');
  }

  // Initialize indexing progress service (shows progress bar)
  const { IndexingProgressService } = await import('./services/IndexingProgressService');
  const indexingProgressService = new IndexingProgressService(context, backendWorkspaceId);
  
  // Start progress monitoring if workspace is available
  if (backendWorkspaceId) {
    indexingProgressService.start();
    console.log('[Cortex] Indexing progress monitoring enabled');
  }

  // Check embedding status and warn if not available
  try {
    const status = await adminClient.getStatus();
    if (status?.embeddingStatus?.enabled && !status?.embeddingStatus?.available) {
      // Embeddings are enabled but service is unavailable
      vscode.window.showWarningMessage(
        `Cortex: Embedding service (${status.embeddingStatus.model || 'unknown'}) is unavailable at ${status.embeddingStatus.endpoint || 'unknown endpoint'}. RAG search will not work until Ollama is running.`,
        'Dismiss'
      );
    } else if (status?.embeddingStatus?.enabled) {
      console.log(`[Cortex] Embeddings enabled: ${status.embeddingStatus.model} at ${status.embeddingStatus.endpoint}`);
    }
  } catch (error) {
    console.warn('[Cortex] Failed to check embedding status:', error);
  }

  // Register services for cleanup on deactivation
  context.subscriptions.push({
    dispose: async () => {
      realtimeUpdateService.dispose();
      indexingProgressService.dispose();
      // Stop backend if it was started by this extension
      if (backendManager.isRunning()) {
        await backendManager.stop();
      }
    }
  });



  // Register disposables
  const disposables = [
    fileInfoTreeProvider,
    fileInfoWebviewProvider,
    cortexTreeView,
    fileInfoTreeView,
    cortexTreeProvider,
    ...commands
  ];
  context.subscriptions.push(...disposables);

  console.log("[Cortex] Extension activated successfully (backend-only mode)");

  // Show welcome message
  const hasShownWelcome = context.globalState.get("cortex.hasShownWelcome");
  if (!hasShownWelcome) {
    vscode.window
      .showInformationMessage(
        "Cortex is ready! All indexing is handled by the backend daemon.",
        "Open Backend Frontend",
        "Dismiss"
      )
      .then((selection) => {
        if (selection === "Open Backend Frontend") {
          vscode.commands.executeCommand("cortex.openBackendFrontend");
        }
      });
    context.globalState.update("cortex.hasShownWelcome", true);
  }

  // Update file info view when active editor changes
  const updateFileInfoView = async () => {
    let uri: vscode.Uri | undefined;
    
    // First try to get from active text editor (for text files)
    const textEditor = vscode.window.activeTextEditor;
    if (textEditor) {
      uri = textEditor.document.uri;
    } else {
      // For non-text editors (like PDFs opened with custom editors),
      // try to get from active tab
      const tabGroups = vscode.window.tabGroups as typeof vscode.window.tabGroups | undefined;
      const activeTab = tabGroups?.activeTabGroup?.activeTab;
      
      if (activeTab?.input) {
        // Try different properties that custom editors might use
        const input = activeTab.input as any;
        
        // Debug logging (can be removed later)
        console.log('[Cortex] Active tab input:', {
          hasUri: !!input.uri,
          hasResource: !!input.resource,
          viewType: input.viewType,
          tabLabel: activeTab.label,
          inputKeys: Object.keys(input)
        });
        
        // Check for uri property (most common)
        if (input.uri && input.uri instanceof vscode.Uri) {
          uri = input.uri;
        }
        // Check for resource property (alternative)
        else if (input.resource && input.resource instanceof vscode.Uri) {
          uri = input.resource;
        }
        // Some custom editors might store the URI in a nested structure
        else if (input.viewType && typeof input.viewType === 'string') {
          // For custom editors, check if there's a primary property
          if (input.primary && input.primary instanceof vscode.Uri) {
            uri = input.primary;
          }
        }
      }
      
      // Also check visible text editors (some PDF extensions might register as text editors)
      if (!uri) {
        const visibleEditors = vscode.window.visibleTextEditors;
        for (const editor of visibleEditors) {
          if (editor.document.uri.scheme === 'file') {
            const ext = path.extname(editor.document.uri.fsPath).toLowerCase();
            if (ext === '.pdf') {
              uri = editor.document.uri;
              console.log('[Cortex] Found PDF in visible editors:', editor.document.uri.fsPath);
              break;
            }
          }
        }
      }
      
      // Last resort: check all tabs for PDF files
      if (!uri && tabGroups) {
        const allTabs = tabGroups.all
          .flatMap(group => group.tabs)
          .filter(tab => tab.label && tab.label.toLowerCase().endsWith('.pdf'));
        
        if (allTabs.length > 0) {
          const activeTabIndex = tabGroups.activeTabGroup.tabs.findIndex(
            t => t === activeTab
          );
          const pdfTab = activeTabIndex >= 0
            ? tabGroups.activeTabGroup.tabs[activeTabIndex]
            : allTabs[0];
          
          if (pdfTab?.input) {
            const input = pdfTab.input as any;
            if (input.uri && input.uri instanceof vscode.Uri) {
              uri = input.uri;
            } else if (input.resource && input.resource instanceof vscode.Uri) {
              uri = input.resource;
            }
          }
        }
      }
    }

    if (uri && uri.scheme === 'file') {
      const absolutePath = uri.fsPath;
      const relativePath = path.relative(workspaceRoot, absolutePath);
      if (relativePath.startsWith("..")) {
        // File is outside workspace, skip
        await fileInfoTreeProvider.updateCurrentFile(null);
        await fileInfoWebviewProvider.updateCurrentFile(null);
      } else {
        console.log('[Cortex] Updating File Info for:', relativePath);
        await fileInfoTreeProvider.updateCurrentFile(relativePath);
        await fileInfoWebviewProvider.updateCurrentFile(relativePath);
      }
    } else {
      console.log('[Cortex] No file URI found, clearing File Info');
      await fileInfoTreeProvider.updateCurrentFile(null);
      await fileInfoWebviewProvider.updateCurrentFile(null);
    }
  };

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(() => {
      updateFileInfoView();
    }),
    // Listen for visible editors changes (catches custom editors that register as text editors)
    vscode.window.onDidChangeVisibleTextEditors(() => {
      updateFileInfoView();
    }),
    // Also listen for tab changes to catch PDFs and other non-text editors
    ...(typeof vscode.window.tabGroups?.onDidChangeTabs === 'function'
      ? [vscode.window.tabGroups.onDidChangeTabs(() => updateFileInfoView())]
      : []),
    ...(typeof (vscode.window.tabGroups as unknown as { onDidChangeActiveTab?: unknown } | undefined)?.onDidChangeActiveTab === 'function'
      ? [(vscode.window.tabGroups as unknown as { onDidChangeActiveTab: (listener: () => void) => vscode.Disposable }).onDidChangeActiveTab(() => updateFileInfoView())]
      : [])
  );

  updateFileInfoView();
}

/**
 * Extension deactivation
 */
export function deactivate() {
  // Use try-catch to avoid errors if channel is already closed
  try {
    console.log("[Cortex] Deactivating extension...");
  } catch {
    // Ignore logging errors during deactivation
  }

  if (globalExtensionState?.onlyOfficeBridge) {
    void globalExtensionState.onlyOfficeBridge.stop();
  }

  // Cleanup SFS client
  disposeSFSClient();

  // Clear global extension state
  globalExtensionState = undefined;
}
