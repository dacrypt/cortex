/**
 * Command: Add tag to current file
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';
import { normalizeTag, TAG_MAX_LENGTH, TAG_MAX_WORDS } from '../utils/osTags';
import { addTagWithOsSync } from '../utils/tagSync';

export async function addTagCommand(
  workspaceRoot: string,
  metadataStore: IMetadataStore,
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

  // Get file extension
  const extension = path.extname(relativePath);

  // Get or create metadata to ensure file exists in metadata store
  metadataStore.getOrCreateMetadata(relativePath, extension);

  // Get existing tags for suggestions
  const existingTags = metadataStore.getAllTags();

  // Show input box with suggestions
  const tag = await vscode.window.showInputBox({
    prompt: 'Enter tag name (slug-style)',
    placeHolder: 'e.g., revision-legal, notas-reunion, facturas-2024',
    validateInput: (value) => {
      if (!value || value.trim().length === 0) {
        return 'Tag name cannot be empty';
      }
      if (value.includes(',')) {
        return 'Tag name cannot contain commas';
      }
      const normalized = normalizeTag(value);
      if (!normalized) {
        return `Use a slug tag (letters/numbers, hyphens, up to ${TAG_MAX_WORDS} words and ${TAG_MAX_LENGTH} characters)`;
      }
      return null;
    },
  });

  if (!tag) {
    return; // User cancelled
  }

  const normalizedTag = normalizeTag(tag);
  if (!normalizedTag) {
    return;
  }

  // Add tag
  const osError = await addTagWithOsSync(
    metadataStore,
    relativePath,
    absolutePath,
    normalizedTag
  );

  // Refresh views
  onMetadataChanged();

  const filename = path.basename(relativePath);
  if (osError) {
    vscode.window.showWarningMessage(
      `Added tag "${normalizedTag}" to ${filename}, but failed to sync OS tags: ${osError.message}`
    );
  } else {
    vscode.window.showInformationMessage(
      `Added tag "${normalizedTag}" to ${filename}`
    );
  }
}
