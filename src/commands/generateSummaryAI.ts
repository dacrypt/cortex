import * as vscode from "vscode";
import * as path from "path";
import { LLMService } from "../services/LLMService";
import { IMetadataStore } from "../core/IMetadataStore";
import { MirrorStore } from "../core/MirrorStore";
import { FileMetadata } from "../models/types";
import { extractKeyTerms } from "../utils/aiSummary";
import { resolveAIContent } from "../utils/aiContent";
import { saveAISummaryToFile } from "../utils/saveAISummary";

/**
 * Command to generate a summary for the current file using AI
 * Creates a concise description that can be saved as notes
 */
export async function generateSummaryAI(
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
  const content = aiContent.content;
  const contentHash = aiContent.contentHash;
  const metadata = metadataStore.getMetadataByPath(relativePath) as FileMetadata | null;
  const cachedSummary =
    metadata?.aiSummary && metadata?.aiSummaryHash === contentHash
      ? metadata.aiSummary
      : undefined;

  // Show progress while analyzing
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Generating summary with AI...",
      cancellable: false,
    },
    async (progress) => {
      try {
        progress.report({ message: "Analyzing file content..." });

        // Get AI summary
        let summary = cachedSummary;
        let keyTerms: string[] | undefined;
        if (!summary) {
          summary =
            (await llmService.generateFileSummary(relativePath, content)) ??
            undefined;
          if (summary) {
            keyTerms = extractKeyTerms(content);
            metadataStore.updateAISummary(
              relativePath,
              summary,
              contentHash,
              keyTerms
            );
            
            // Save summary to repository
            await saveAISummaryToFile(
              workspaceRoot,
              relativePath,
              summary,
              contentHash,
              keyTerms
            );
          }
        } else {
          keyTerms = metadata?.aiKeyTerms || extractKeyTerms(content);
          if (!metadata?.aiKeyTerms || metadata.aiKeyTerms.length === 0) {
            metadataStore.updateAISummary(
              relativePath,
              summary,
              contentHash,
              keyTerms
            );
          }
          
          // Save summary to repository even if it was cached
          await saveAISummaryToFile(
            workspaceRoot,
            relativePath,
            summary,
            contentHash,
            keyTerms
          );
        }

        if (!summary) {
          vscode.window.showInformationMessage("Could not generate summary");
          return;
        }

        const existingNotes = metadata?.notes || "";

        // Show the summary and ask if user wants to save it
        const action = await vscode.window.showInformationMessage(
          `AI Summary: "${summary}"`,
          "Save as Notes",
          "Append to Notes",
          "Copy",
          "Cancel"
        );

        if (action === "Cancel" || !action) {
          return;
        }

        if (action === "Copy") {
          await vscode.env.clipboard.writeText(summary);
          vscode.window.showInformationMessage("Summary copied to clipboard");
          return;
        }

        if (action === "Save as Notes") {
          await metadataStore.updateNotes(relativePath, summary);
          vscode.window.showInformationMessage("✓ Summary saved as notes");
          vscode.commands.executeCommand("cortex.refreshViews");
        } else if (action === "Append to Notes") {
          const newNotes = existingNotes
            ? `${existingNotes}\n\nAI Summary: ${summary}`
            : summary;
          await metadataStore.updateNotes(relativePath, newNotes);
          vscode.window.showInformationMessage("✓ Summary appended to notes");
          vscode.commands.executeCommand("cortex.refreshViews");
        }
      } catch (error) {
        console.error("Error in generateSummaryAI:", error);
        vscode.window.showErrorMessage(
          `Failed to generate summary: ${
            error instanceof Error ? error.message : "Unknown error"
          }`
        );
      }
    }
  );
}
