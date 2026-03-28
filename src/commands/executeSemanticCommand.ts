import * as vscode from 'vscode';
import { GrpcSFSClient, SFSCommandResult, SFSPreviewResult, CommandSuggestion } from '../core/GrpcSFSClient';

let sfsClient: GrpcSFSClient | null = null;

function getClient(): GrpcSFSClient {
  if (!sfsClient) {
    const config = vscode.workspace.getConfiguration('cortex');
    const endpoint = config.get<string>('backend.endpoint', 'localhost:50051');
    sfsClient = new GrpcSFSClient(endpoint);
  }
  return sfsClient;
}

// Get workspace ID from current workspace
function getWorkspaceId(): string {
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    throw new Error('No workspace folder open');
  }
  // Use first workspace folder path as workspace ID
  return workspaceFolders[0].uri.fsPath;
}

// Get currently selected file IDs from explorer
function getSelectedFileIds(): string[] {
  // This would need to be integrated with the tree view selection
  // For now, return empty array
  return [];
}

// Main command: Execute semantic command
export async function executeSemanticCommand(contextFileIds?: string[]): Promise<void> {
  try {
    const client = getClient();
    const workspaceId = getWorkspaceId();
    const fileIds = contextFileIds || getSelectedFileIds();

    // Get suggestions for quick picks
    let suggestions: CommandSuggestion[] = [];
    try {
      suggestions = await client.suggestCommands(workspaceId, '', fileIds, 5);
    } catch (e) {
      // Ignore suggestion errors
    }

    // Build quick pick items from suggestions
    const suggestionItems: vscode.QuickPickItem[] = suggestions.map(s => ({
      label: s.command,
      description: s.description,
      detail: `${s.category} • Relevance: ${Math.round(s.relevance * 100)}%`,
    }));

    // Create quick pick with suggestions and input box
    const quickPick = vscode.window.createQuickPick();
    quickPick.placeholder = 'Enter a natural language command (e.g., "group all PDFs by author")';
    quickPick.items = suggestionItems;
    quickPick.matchOnDescription = true;
    quickPick.matchOnDetail = true;

    // Update suggestions as user types
    quickPick.onDidChangeValue(async (value) => {
      if (value.length >= 2) {
        try {
          const newSuggestions = await client.suggestCommands(workspaceId, value, fileIds, 5);
          quickPick.items = newSuggestions.map(s => ({
            label: s.command,
            description: s.description,
            detail: `${s.category} • Relevance: ${Math.round(s.relevance * 100)}%`,
          }));
        } catch (e) {
          // Keep current suggestions on error
        }
      }
    });

    // Handle selection
    const selectedCommand = await new Promise<string | undefined>((resolve) => {
      quickPick.onDidAccept(() => {
        const selected = quickPick.selectedItems[0];
        if (selected) {
          resolve(selected.label);
        } else if (quickPick.value) {
          resolve(quickPick.value);
        } else {
          resolve(undefined);
        }
        quickPick.hide();
      });
      quickPick.onDidHide(() => {
        resolve(undefined);
        quickPick.dispose();
      });
      quickPick.show();
    });

    if (!selectedCommand) {
      return;
    }

    // Preview the command first
    const preview = await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'Previewing command...',
        cancellable: false,
      },
      async () => {
        return client.previewCommand(workspaceId, selectedCommand, fileIds);
      }
    );

    // Show preview and ask for confirmation
    const confirmResult = await showPreviewAndConfirm(preview);
    if (!confirmResult) {
      return;
    }

    // Execute the command
    const result = await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'Executing command...',
        cancellable: false,
      },
      async () => {
        return client.executeCommand(workspaceId, selectedCommand, fileIds);
      }
    );

    // Show result
    showCommandResult(result);

  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    vscode.window.showErrorMessage(`Failed to execute semantic command: ${message}`);
  }
}

// Show preview and ask for confirmation
async function showPreviewAndConfirm(preview: SFSPreviewResult): Promise<boolean> {
  // Build message
  let message = `${preview.explanation}\n\n`;
  message += `Files affected: ${preview.files_affected}\n`;
  message += `Confidence: ${Math.round(preview.confidence * 100)}%\n`;

  if (preview.warnings.length > 0) {
    message += `\n⚠️ Warnings:\n`;
    preview.warnings.forEach(w => {
      message += `  • ${w}\n`;
    });
  }

  if (preview.alternative_interpretations.length > 0) {
    message += `\nAlternative interpretations:\n`;
    preview.alternative_interpretations.forEach(a => {
      message += `  • ${a}\n`;
    });
  }

  // For low confidence, show more detailed confirmation
  if (preview.confidence < 0.7) {
    const selection = await vscode.window.showWarningMessage(
      `Low confidence command (${Math.round(preview.confidence * 100)}%): ${preview.explanation}`,
      { modal: true, detail: message },
      'Execute Anyway',
      'Cancel'
    );
    return selection === 'Execute Anyway';
  }

  // Normal confirmation
  const selection = await vscode.window.showInformationMessage(
    preview.explanation,
    { modal: false, detail: `${preview.files_affected} files will be affected.` },
    'Execute',
    'Cancel'
  );
  return selection === 'Execute';
}

// Show command result
function showCommandResult(result: SFSCommandResult): void {
  if (result.success) {
    let message = result.explanation;
    if (result.files_affected > 0) {
      message += ` (${result.files_affected} files)`;
    }

    // Show with undo option if available
    if (result.undo_command) {
      vscode.window.showInformationMessage(message, 'Undo').then(selection => {
        if (selection === 'Undo') {
          executeSemanticCommand([]);
          // Would need to execute the undo command
          vscode.window.showInputBox({
            prompt: 'Undo command',
            value: result.undo_command,
          });
        }
      });
    } else {
      vscode.window.showInformationMessage(message);
    }
  } else {
    vscode.window.showErrorMessage(`Command failed: ${result.error_message}`);
  }
}

// Command: Show command history
export async function showCommandHistory(): Promise<void> {
  try {
    const client = getClient();
    const workspaceId = getWorkspaceId();

    const history = await client.getCommandHistory(workspaceId, 20);

    if (history.length === 0) {
      vscode.window.showInformationMessage('No command history found.');
      return;
    }

    const items: vscode.QuickPickItem[] = history.map(entry => ({
      label: entry.command,
      description: entry.success ? '✓ Success' : '✗ Failed',
      detail: `${entry.result_summary} • ${new Date(entry.executed_at).toLocaleString()}`,
    }));

    const selected = await vscode.window.showQuickPick(items, {
      placeHolder: 'Select a command to re-run',
    });

    if (selected) {
      await executeSemanticCommand([]);
    }

  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    vscode.window.showErrorMessage(`Failed to get command history: ${message}`);
  }
}

// Command: Quick file operations via context menu
export async function tagSelectedFiles(fileIds: string[], tag?: string): Promise<void> {
  const tagName = tag || await vscode.window.showInputBox({
    prompt: 'Enter tag name',
    placeHolder: 'e.g., important, review-needed',
  });

  if (!tagName) {
    return;
  }

  const command = `tag these files as ${tagName}`;

  try {
    const client = getClient();
    const workspaceId = getWorkspaceId();

    const result = await client.executeCommand(workspaceId, command, fileIds);
    showCommandResult(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    vscode.window.showErrorMessage(`Failed to tag files: ${message}`);
  }
}

export async function assignSelectedFilesToProject(fileIds: string[], projectName?: string): Promise<void> {
  const name = projectName || await vscode.window.showInputBox({
    prompt: 'Enter project name',
    placeHolder: 'e.g., My Project',
  });

  if (!name) {
    return;
  }

  const command = `assign these files to project ${name}`;

  try {
    const client = getClient();
    const workspaceId = getWorkspaceId();

    const result = await client.executeCommand(workspaceId, command, fileIds);
    showCommandResult(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    vscode.window.showErrorMessage(`Failed to assign files: ${message}`);
  }
}

export async function groupSelectedFiles(fileIds: string[], groupBy?: string): Promise<void> {
  const criteria = groupBy || await vscode.window.showQuickPick(
    ['extension', 'folder', 'type', 'date'],
    { placeHolder: 'Group files by...' }
  );

  if (!criteria) {
    return;
  }

  const command = `group these files by ${criteria}`;

  try {
    const client = getClient();
    const workspaceId = getWorkspaceId();

    const result = await client.executeCommand(workspaceId, command, fileIds);
    showCommandResult(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Unknown error';
    vscode.window.showErrorMessage(`Failed to group files: ${message}`);
  }
}

// Cleanup
export function disposeSFSClient(): void {
  if (sfsClient) {
    sfsClient.disconnect();
    sfsClient = null;
  }
}
