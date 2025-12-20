/**
 * Command: Assign context to current file
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';

export async function assignContextCommand(
  workspaceRoot: string,
  metadataStore: IMetadataStore,
  indexStore: IndexStore,
  onMetadataChanged: () => void
): Promise<void> {
  // Get active editor
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No file is currently open');
    return;
  }

  const absolutePath = editor.document.uri.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  // Verify file is in workspace
  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  // Verify file is in index
  const fileEntry = indexStore.getFile(relativePath);
  if (!fileEntry) {
    vscode.window.showErrorMessage('File is not indexed');
    return;
  }

  // Get or create metadata
  metadataStore.getOrCreateMetadata(relativePath, fileEntry.extension);

  // Get existing contexts for suggestions
  const existingContexts = metadataStore.getAllContexts();

  // Show input box
  const context = await vscode.window.showInputBox({
    prompt: 'Enter context name',
    placeHolder: 'e.g., project-alpha, client-acme, case-2024-01',
    validateInput: (value) => {
      if (!value || value.trim().length === 0) {
        return 'Context name cannot be empty';
      }
      if (value.includes(',')) {
        return 'Context name cannot contain commas';
      }
      return null;
    },
  });

  if (!context) {
    return; // User cancelled
  }

  const normalizedContext = context.trim().toLowerCase();

  // Add context
  metadataStore.addContext(relativePath, normalizedContext);

  // Refresh views
  onMetadataChanged();

  vscode.window.showInformationMessage(
    `Added context "${normalizedContext}" to ${fileEntry.filename}`
  );
}
