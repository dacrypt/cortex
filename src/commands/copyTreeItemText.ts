/**
 * Copy Tree Item Text Command
 * 
 * Copies the label/text of a tree item to the clipboard.
 * Works with any tree item in Cortex views.
 */

import * as vscode from 'vscode';

/**
 * Copy the text/label of a tree item to clipboard
 */
export async function copyTreeItemTextCommand(
  item?: vscode.TreeItem | { label?: string | vscode.TreeItemLabel }
): Promise<void> {
  try {
    let textToCopy = '';

    if (!item) {
      vscode.window.showWarningMessage('No item selected to copy');
      return;
    }

    // Handle TreeItem
    if (item instanceof vscode.TreeItem) {
      if (typeof item.label === 'string') {
        textToCopy = item.label;
      } else if (item.label && typeof item.label === 'object' && 'label' in item.label) {
        textToCopy = item.label.label;
      } else {
        // Fallback: try to get text from description or tooltip
        if (item.description) {
          textToCopy = typeof item.description === 'string' 
            ? item.description 
            : String(item.description);
        } else if (item.tooltip) {
          textToCopy = typeof item.tooltip === 'string'
            ? item.tooltip
            : String(item.tooltip);
        } else {
          textToCopy = 'Unknown';
        }
      }
    } 
    // Handle object with label property
    else if (item && typeof item === 'object' && 'label' in item) {
      if (typeof item.label === 'string') {
        textToCopy = item.label;
      } else if (item.label && typeof item.label === 'object' && 'label' in item.label) {
        textToCopy = item.label.label;
      }
    }

    if (!textToCopy || textToCopy.trim() === '') {
      vscode.window.showWarningMessage('No text found to copy');
      return;
    }

    // Copy to clipboard
    await vscode.env.clipboard.writeText(textToCopy);
    vscode.window.showInformationMessage(`Copied: ${textToCopy}`);
  } catch (error) {
    console.error('[copyTreeItemText] Error:', error);
    vscode.window.showErrorMessage(`Failed to copy text: ${error instanceof Error ? error.message : String(error)}`);
  }
}


