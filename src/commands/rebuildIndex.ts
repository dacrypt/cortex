/**
 * Command: Rebuild Index
 * 
 * Triggers backend to reindex the workspace
 */

import * as vscode from 'vscode';
import { GrpcAdminClient } from '../core/GrpcAdminClient';

export async function rebuildIndexCommand(
  workspaceRoot: string,
  onIndexChanged: () => void
): Promise<void> {
  // Get extension context
  const extension = vscode.extensions.getExtension('your-publisher-name.cortex');
  if (!extension) {
    vscode.window.showErrorMessage('Cortex: Extension not found');
    return;
  }
  
  // Create admin client with extension context
  const adminClient = new GrpcAdminClient(extension.exports as vscode.ExtensionContext);

  // Get workspace ID
  let workspaceId: string | undefined;
  try {
    const workspaces = await adminClient.listWorkspaces();
    const workspace = workspaces.find((ws) => ws.path === workspaceRoot);
    if (!workspace) {
      vscode.window.showErrorMessage('Cortex: Workspace not registered with backend');
      return;
    }
    workspaceId = workspace.id;
  } catch (error) {
    vscode.window.showErrorMessage(
      `Cortex: Failed to connect to backend: ${error instanceof Error ? error.message : String(error)}`
    );
    return;
  }

  // Trigger backend reindex
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Cortex: Rebuilding index (backend)...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ increment: 0, message: 'Triggering backend reindex...' });
      
      try {
        await adminClient.scanWorkspace(workspaceId!, workspaceRoot, true);
        progress.report({ increment: 100, message: 'Reindex started' });
        onIndexChanged();
        vscode.window.showInformationMessage('Cortex: Reindex started on backend');
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to trigger reindex: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }
  );
}
