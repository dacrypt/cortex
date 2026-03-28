import * as vscode from "vscode";
import { GrpcAdminClient } from "../core/GrpcAdminClient";

/**
 * Command to generate an AI summary for the current file using backend AI.
 */
export async function generateSummaryAI(
  adminClient: GrpcAdminClient,
  workspaceRoot: string,
  workspaceId?: string
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage("No active file open");
    return;
  }
  if (!workspaceId) {
    vscode.window.showErrorMessage("Backend workspace not available");
    return;
  }

  console.log("[Cortex][AI] generateSummaryAI invoked");

  const document = editor.document;
  const absolutePath = document.uri.fsPath;
  const relativePath = absolutePath.replace(workspaceRoot + "/", "");

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Running backend AI indexing...",
      cancellable: false,
    },
    async (progress) => {
      try {
        progress.report({ message: "Processing file on backend..." });
        await adminClient.processFile(workspaceId, relativePath);
        vscode.window.showInformationMessage("✓ Backend AI indexing complete");
        vscode.commands.executeCommand("cortex.refreshViews");
      } catch (error) {
        console.error("Error in generateSummaryAI:", error);
        vscode.window.showErrorMessage(
          `Failed to generate summary: ${error instanceof Error ? error.message : "Unknown error"}`
        );
      }
    }
  );
}
