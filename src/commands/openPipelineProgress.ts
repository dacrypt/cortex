import * as vscode from "vscode";
import { PipelineProgressView } from "../frontend/PipelineProgressView";

export function openPipelineProgressCommand(
  context: vscode.ExtensionContext
): vscode.Disposable {
  return vscode.commands.registerCommand(
    "cortex.openPipelineProgress",
    async () => {
      // Get workspace info
      const workspaceFolders = vscode.workspace.workspaceFolders;
      if (!workspaceFolders || workspaceFolders.length === 0) {
        vscode.window.showErrorMessage(
          "Cortex: No workspace folder open"
        );
        return;
      }

      const workspaceRoot = workspaceFolders[0].uri.fsPath;

      // Get workspace ID from backend
      const { GrpcAdminClient } = await import("../core/GrpcAdminClient");
      const adminClient = new GrpcAdminClient(context);

      try {
        const workspaces = await adminClient.listWorkspaces();
        const workspace = workspaces.find((ws) => ws.path === workspaceRoot);

        if (!workspace || !workspace.id) {
          vscode.window.showErrorMessage(
            "Cortex: Workspace not registered with backend. Please open the Backend Frontend first."
          );
          return;
        }

        PipelineProgressView.show(context, workspace.id, workspaceRoot);
      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Failed to open pipeline progress: ${
            error instanceof Error ? error.message : String(error)
          }`
        );
      }
    }
  );
}

