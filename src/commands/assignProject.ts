/**
 * Command: Assign project to current file using AI
 * MANDATORY: All project assignments require AI validation for quality assurance
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import * as fs from 'node:fs';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { AIQualityService } from '../services/AIQualityService';

export async function assignProjectCommand(
  workspaceRoot: string,
  knowledgeClient: GrpcKnowledgeClient,
  adminClient: GrpcAdminClient,
  workspaceId: string,
  onMetadataChanged: () => void,
  context?: vscode.ExtensionContext,
  item?: vscode.Uri | vscode.TreeItem | { resourceUri?: vscode.Uri }
): Promise<void> {
  // CRITICAL: AI validation required
  if (!context) {
    vscode.window.showErrorMessage('Extension context required for AI validation');
    return;
  }

  const aiService = new AIQualityService(context);

  // Get file URI from parameter, tree item, or active editor
  let fileUriToUse: vscode.Uri | undefined;
  
  if (item instanceof vscode.Uri) {
    // Direct URI passed
    fileUriToUse = item;
  } else if (item) {
    // Tree item or object - try multiple ways to get the URI
    interface TreeItemWithPayload {
      resourceUri?: vscode.Uri;
      payload?: {
        metadata?: { relativePath?: string };
        value?: string;
      };
    }
    
    // Method 1: TreeItem instance with resourceUri
    if (item instanceof vscode.TreeItem && item.resourceUri) {
      fileUriToUse = item.resourceUri;
    }
    // Method 2: Object with resourceUri property
    else if ('resourceUri' in item && item.resourceUri instanceof vscode.Uri) {
      fileUriToUse = item.resourceUri;
    }
    // Method 3: Extract from payload (contains relativePath)
    else {
      const treeItem = item as TreeItemWithPayload;
      if (treeItem.payload?.metadata?.relativePath) {
        const relativePath = treeItem.payload.metadata.relativePath;
        const absolutePath = path.join(workspaceRoot, relativePath);
        fileUriToUse = vscode.Uri.file(absolutePath);
      }
      // Method 4: Extract from payload.value (relativePath stored there)
      else if (treeItem.payload?.value && typeof treeItem.payload.value === 'string') {
        const relativePath = treeItem.payload.value;
        const absolutePath = path.join(workspaceRoot, relativePath);
        fileUriToUse = vscode.Uri.file(absolutePath);
      }
    }
  }
  
  // Fallback to active editor if no URI found
  if (!fileUriToUse) {
    const editor = vscode.window.activeTextEditor;
    if (editor) {
      fileUriToUse = editor.document.uri;
    }
  }

  if (!fileUriToUse) {
    vscode.window.showErrorMessage('No file selected. Please select a file from the tree or open it in the editor.');
    return;
  }

  const absolutePath = fileUriToUse.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  // Verify file is in workspace
  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  try {
    // MANDATORY: Validate LLM is available
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'Validating AI availability...',
        cancellable: false,
      },
      async (progress) => {
        progress.report({ message: 'Checking LLM availability...' });
        await aiService.requireLLM();
      }
    );

    // Get file content for AI analysis
    let fileContent = '';
    try {
      fileContent = fs.readFileSync(absolutePath, 'utf-8');
    } catch (error) {
      console.warn('[assignProject] Failed to read file content:', error);
      // Continue with empty content - AI can still work with file metadata
    }
    const existingProjects = await knowledgeClient.listProjects(workspaceId);
    const existingProjectNames = existingProjects.map(p => p.name);

    // AI-powered project suggestion
    let aiSuggestion: Awaited<ReturnType<typeof aiService.suggestProjectForFile>>;
    
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'AI analyzing file...',
        cancellable: false,
      },
      async (progress) => {
        progress.report({ message: 'AI suggesting project based on file content...' });
        aiSuggestion = await aiService.suggestProjectForFile(
          workspaceId,
          relativePath,
          fileContent,
          existingProjectNames
        );
      }
    );

    // Show AI suggestion with option to override
    const projectName = await vscode.window.showInputBox({
      prompt: `AI suggests project: "${aiSuggestion!.project}" (${(aiSuggestion!.confidence * 100).toFixed(0)}% confidence)\n${aiSuggestion!.reason}\n\nEnter project name (or accept AI suggestion):`,
      value: aiSuggestion!.project,
      placeHolder: 'e.g., book-draft, car-purchase, client-acme',
      validateInput: (value) => {
        if (!value || value.trim().length === 0) {
          return 'Project name cannot be empty';
        }
        if (value.includes(',')) {
          return 'Project name cannot contain commas';
        }
        return null;
      },
    });

    if (!projectName) {
      return; // User cancelled
    }

    const normalizedName = projectName.trim();

    // Get or create project (if new, it will be created with AI validation via createProject)
    let project = await knowledgeClient.getProjectByName(workspaceId, normalizedName);
    if (!project) {
      // Create new project with generic nature - user can edit later
      project = await knowledgeClient.createProjectWithNature(
        workspaceId,
        normalizedName,
        undefined,
        undefined,
        'generic'
      );
      vscode.window.showInformationMessage(
        `Created project "${normalizedName}" (use "Edit Project" to set nature with AI)`
      );
    }

    // Get document ID from relative path
    const documentId = await knowledgeClient.getDocumentIdByPath(
      workspaceId,
      relativePath,
      adminClient
    );

    if (!documentId) {
      vscode.window.showWarningMessage(
        `File "${relativePath}" is not yet indexed. Please wait for indexing to complete, then try again.`
      );
      return;
    }

    // Associate document with project
    await knowledgeClient.addDocumentToProject(
      workspaceId,
      project.id,
      documentId
    );
    
    vscode.window.showInformationMessage(
      `✓ Project "${normalizedName}" assigned to ${path.basename(relativePath)} (AI-validated)`
    );

    // Auto-infer project characteristics from members (including this new file)
    if (context) {
      const { ProjectInferenceService } = await import('../services/ProjectInferenceService');
      const { GrpcRAGClient } = await import('../core/GrpcRAGClient');
      const ragClient = new GrpcRAGClient(context);
      const inferenceService = new ProjectInferenceService(context, knowledgeClient, ragClient);
      
      // Run inference in background (don't block UI)
      inferenceService.autoInferOnMemberChange(workspaceId, project.id, () => {
        onMetadataChanged();
        vscode.window.showInformationMessage(
          `✓ Project "${normalizedName}" characteristics updated based on members`
        );
      }).catch((error) => {
        console.error('[assignProject] Auto-inference failed:', error);
        // Don't show error to user - inference is optional enhancement
      });
    }

    // Refresh views
    onMetadataChanged();
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    
    // Check if it's an LLM availability error
    if (errorMessage.includes('LLM is not available') || errorMessage.includes('CRITICAL')) {
      vscode.window.showErrorMessage(
        `❌ ${errorMessage}\n\n` +
        `Cortex requires AI/LLM to be available for quality assurance.\n` +
        `Please ensure your LLM service is running and configured.`
      );
    } else {
      vscode.window.showErrorMessage(`Failed to assign project: ${errorMessage}`);
    }
    console.error('[Cortex] assignProjectCommand error:', error);
  }
}

