/**
 * Command: Add tag to current file
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { IndexStore } from '../core/IndexStore';

export async function addTagCommand(
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

  // Get or create metadata to ensure file exists in metadata store
  metadataStore.getOrCreateMetadata(relativePath, fileEntry.extension);

  // Get existing tags for suggestions
  const existingTags = metadataStore.getAllTags();

  // Show input box with suggestions
  const tag = await vscode.window.showInputBox({
    prompt: 'Enter tag name',
    placeHolder: 'e.g., important, review, bug-fix',
    validateInput: (value) => {
      if (!value || value.trim().length === 0) {
        return 'Tag name cannot be empty';
      }
      if (value.includes(',')) {
        return 'Tag name cannot contain commas';
      }
      return null;
    },
  });

  if (!tag) {
    return; // User cancelled
  }

  const normalizedTag = tag.trim().toLowerCase();

  // Add tag
  metadataStore.addTag(relativePath, normalizedTag);

  // Refresh views
  onMetadataChanged();

  vscode.window.showInformationMessage(
    `Added tag "${normalizedTag}" to ${fileEntry.filename}`
  );
}
