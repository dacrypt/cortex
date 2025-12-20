/**
 * Command: Open Cortex View
 */

import * as vscode from 'vscode';

export async function openViewCommand(): Promise<void> {
  // Focus the Cortex view in the Activity Bar
  await vscode.commands.executeCommand('cortex-contextView.focus');
}
