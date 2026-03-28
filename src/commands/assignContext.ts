/**
 * Command: Assign project to current file
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { IMetadataStore } from '../core/IMetadataStore';

export async function assignContextCommand(
  workspaceRoot: string,
  metadataStore: IMetadataStore,
  onMetadataChanged: () => void,
  item?: vscode.Uri | vscode.TreeItem | { resourceUri?: vscode.Uri }
): Promise<void> {
  // Get file URI from parameter, tree item, or active editor
  let fileUriToUse: vscode.Uri | undefined;
  
  // Debug: Log what we received
  console.log('[assignContext] Item received:', JSON.stringify(item, null, 2));
  console.log('[assignContext] Item type:', typeof item);
  console.log('[assignContext] Is URI:', item instanceof vscode.Uri);
  console.log('[assignContext] Is TreeItem:', item instanceof vscode.TreeItem);
  
  if (item instanceof vscode.Uri) {
    // Direct URI passed
    fileUriToUse = item;
    console.log('[assignContext] Using direct URI');
  } else if (item) {
    // Tree item or object - try multiple ways to get the URI
    const treeItem = item as any;
    console.log('[assignContext] TreeItem keys:', Object.keys(treeItem));
    console.log('[assignContext] TreeItem.resourceUri:', treeItem.resourceUri);
    console.log('[assignContext] TreeItem.payload:', treeItem.payload);
    
    // Method 1: Direct resourceUri property
    if (treeItem.resourceUri instanceof vscode.Uri) {
      fileUriToUse = treeItem.resourceUri;
      console.log('[assignContext] Using resourceUri (Method 1)');
    }
    // Method 2: TreeItem instance with resourceUri
    else if (treeItem instanceof vscode.TreeItem && treeItem.resourceUri) {
      fileUriToUse = treeItem.resourceUri;
      console.log('[assignContext] Using resourceUri from TreeItem (Method 2)');
    }
    // Method 3: Extract from payload (contains relativePath)
    else if (treeItem.payload && treeItem.payload.metadata && treeItem.payload.metadata.relativePath) {
      const relativePath = treeItem.payload.metadata.relativePath;
      const absolutePath = path.join(workspaceRoot, relativePath);
      fileUriToUse = vscode.Uri.file(absolutePath);
      console.log('[assignContext] Using payload.metadata.relativePath (Method 3):', relativePath);
    }
    // Method 4: Extract from payload.value (relativePath stored there)
    else if (treeItem.payload && treeItem.payload.value && typeof treeItem.payload.value === 'string') {
      const relativePath = treeItem.payload.value;
      const absolutePath = path.join(workspaceRoot, relativePath);
      fileUriToUse = vscode.Uri.file(absolutePath);
      console.log('[assignContext] Using payload.value (Method 4):', relativePath);
    } else {
      console.log('[assignContext] No URI found in tree item');
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

  // Get file extension
  const extension = path.extname(relativePath);

  // Get or create metadata
  metadataStore.getOrCreateMetadata(relativePath, extension);

  // Get existing projects for suggestions
  const existingContexts = metadataStore.getAllContexts();

  // Show input box
  const context = await vscode.window.showInputBox({
    prompt: 'Enter project name',
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

  if (!context) {
    return; // User cancelled
  }

  const normalizedContext = context.trim().toLowerCase();

  // Add context
  metadataStore.addContext(relativePath, normalizedContext);

  // Refresh views
  onMetadataChanged();

  const filename = path.basename(relativePath);
  vscode.window.showInformationMessage(
    `Added project "${normalizedContext}" to ${filename}`
  );
}
