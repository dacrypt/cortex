/**
 * Command: Open Cortex View
 */

import * as vscode from 'vscode';

export function openViewCommand(): vscode.Disposable {
  return vscode.commands.registerCommand("cortex.openView", async () => {
    // Focus the Cortex view in the Activity Bar
    await vscode.commands.executeCommand('cortex-mainView.focus');
  });
}
