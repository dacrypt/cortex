import * as vscode from "vscode";
import * as path from "path";
import { LLMService } from "../services/LLMService";
import { IMetadataStore } from "../core/IMetadataStore";
import { MirrorStore } from "../core/MirrorStore";
import { FileMetadata } from "../models/types";
import { extractKeyTerms } from "../utils/aiSummary";
import { resolveAIContent } from "../utils/aiContent";

/**
 * Command to suggest a project name for the current file using AI
 * Analyzes file content and recent work context to recommend a project assignment
 */
export async function suggestProjectAI(
  llmService: LLMService,
  metadataStore: IMetadataStore,
  mirrorStore: MirrorStore,
  workspaceRoot: string
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage("No active file open");
    return;
  }

  console.log("[Cortex][AI] suggestProjectAI invoked");

  // Check if LLM service is available
  const available = await llmService.isAvailable();
  if (!available) {
    await llmService.showNotAvailableMessage();
    return;
  }

  const document = editor.document;
  const absolutePath = document.uri.fsPath;
  const relativePath = absolutePath.replace(workspaceRoot + "/", "");
  const extension = path.extname(absolutePath);
  const aiContent = await resolveAIContent(
    {
      absolutePath,
      relativePath,
      extension,
      editorText: document.getText(),
    },
    mirrorStore
  );
  if (!aiContent) {
    vscode.window.showErrorMessage(
      "No readable content available for AI analysis"
    );
    return;
  }
  console.log(`[Cortex][AI] suggestProjectAI content ready for ${relativePath}`);
  const content = aiContent.content;
  const contentHash = aiContent.contentHash;
  const metadata = metadataStore.getMetadataByPath(relativePath) as FileMetadata | null;
  const summary =
    metadata?.aiSummary && metadata?.aiSummaryHash === contentHash
      ? metadata.aiSummary
      : undefined;
  const keyTerms =
    metadata?.aiKeyTerms && metadata.aiKeyTerms.length > 0
      ? metadata.aiKeyTerms
      : extractKeyTerms(content);

  // Show progress while analyzing
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Analyzing context with AI...",
      cancellable: false,
    },
    async (progress) => {
      try {
        progress.report({ message: "Getting file context..." });

        // Get files from existing projects as context
        const allContexts = metadataStore.getAllContexts();
        const recentFiles: string[] = [];

        // Get files from the same directory
        const dirPath = relativePath.substring(
          0,
          relativePath.lastIndexOf("/")
        );

        // Add some context files from existing projects (max 10)
        for (const context of allContexts.slice(0, 3)) {
          const filesInContext = metadataStore.getFilesByContext(context);
          recentFiles.push(...filesInContext.slice(0, 3));
        }

        // Add the directory path as context
        if (dirPath) {
          recentFiles.push(`${dirPath}/*`);
        }

        progress.report({ message: "Generating project suggestion..." });

        // Get AI suggestion
        const suggestedProject = await llmService.suggestProject(
          relativePath,
          content,
          recentFiles,
          {
            summary,
            keyTerms,
            snippet: content.slice(0, 500),
          }
        );

        if (!suggestedProject) {
          console.log(
            `[Cortex][AI] No project suggestion for ${relativePath}`
          );
          vscode.window.showInformationMessage(
            "AI could not suggest a project based on current context"
          );
          return;
        }

        // Get existing projects for this file
        const existingProjects = metadata?.contexts || [];

        if (existingProjects.includes(suggestedProject)) {
          console.log(
            `[Cortex][AI] Project ${suggestedProject} already applied to ${relativePath}`
          );
          vscode.window.showInformationMessage(
            `File is already assigned to project: ${suggestedProject}`
          );
          return;
        }

        // Ask user to confirm the suggestion
        const action = await vscode.window.showInformationMessage(
          `AI suggests project: "${suggestedProject}"`,
          "Apply",
          "Edit & Apply",
          "Cancel"
        );

        if (action === "Cancel" || !action) {
          return;
        }

        let finalProjectName = suggestedProject;

        if (action === "Edit & Apply") {
          const edited = await vscode.window.showInputBox({
            prompt: "Edit the project name",
            value: suggestedProject,
            placeHolder: "project-name",
            validateInput: (value) => {
              if (!value || value.trim().length === 0) {
                return "Project name cannot be empty";
              }
              if (value.length > 100) {
                return "Project name too long (max 100 characters)";
              }
              return null;
            },
          });

          if (!edited) {
            return;
          }

          finalProjectName = edited.trim();
        }

        // Apply the project
        progress.report({ message: "Assigning project..." });
        await metadataStore.addContext(relativePath, finalProjectName);
        console.log(
          `[Cortex][AI] Applied project ${finalProjectName} to ${relativePath}`
        );

        vscode.window.showInformationMessage(
          `✓ Assigned file to project: ${finalProjectName}`
        );

        // Refresh views to show updated project
        vscode.commands.executeCommand("cortex.refreshViews");
      } catch (error) {
        console.error("Error in suggestProjectAI:", error);
        vscode.window.showErrorMessage(
          `Failed to suggest project: ${
            error instanceof Error ? error.message : "Unknown error"
          }`
        );
      }
    }
  );
}
