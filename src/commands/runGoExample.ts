/**
 * Example command showing how to execute Go code from the extension
 */

import * as vscode from 'vscode';
import * as path from 'path';
import {
  executeGoBinary,
  runGoFile,
  buildAndRunGoFile,
  executeGoCommand,
  isGoAvailable,
} from '../utils/goExecutor';

/**
 * Example: Execute a Go program from the current file
 */
export async function runGoExampleCommand(): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showWarningMessage('No active editor');
    return;
  }

  const filePath = editor.document.uri.fsPath;
  const fileExtension = path.extname(filePath);

  // Check if it's a Go file
  if (fileExtension !== '.go') {
    vscode.window.showWarningMessage('Current file is not a Go file');
    return;
  }

  // Check if Go is available
  const goAvailable = await isGoAvailable();
  if (!goAvailable) {
    vscode.window.showErrorMessage(
      'Go is not installed or not in PATH. Please install Go to use this feature.'
    );
    return;
  }

  // Show progress
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Running Go program...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: 'Executing Go file...' });

      // Option 1: Run directly with `go run`
      const result = await runGoFile(filePath);

      if (result.exitCode === 0) {
        // Show output in a new document
        const outputDoc = await vscode.workspace.openTextDocument({
          content: result.stdout || '(no output)',
          language: 'plaintext',
        });
        await vscode.window.showTextDocument(outputDoc);
        vscode.window.showInformationMessage('Go program executed successfully');
      } else {
        // Show error
        const errorDoc = await vscode.workspace.openTextDocument({
          content: result.stderr || 'Unknown error',
          language: 'plaintext',
        });
        await vscode.window.showTextDocument(errorDoc);
        vscode.window.showErrorMessage(
          `Go program failed with exit code ${result.exitCode}`
        );
      }
    }
  );
}

/**
 * Example: Execute a compiled Go binary
 */
export async function runGoBinaryCommand(): Promise<void> {
  const binaryPath = await vscode.window.showInputBox({
    prompt: 'Enter path to Go binary',
    placeHolder: '/path/to/binary',
  });

  if (!binaryPath) {
    return;
  }

  const argsInput = await vscode.window.showInputBox({
    prompt: 'Enter arguments (space-separated)',
    placeHolder: 'arg1 arg2 arg3',
  });

  const args = argsInput ? argsInput.split(' ') : [];

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Running Go binary...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: 'Executing binary...' });

      const result = await executeGoBinary(binaryPath, args);

      if (result.exitCode === 0) {
        const outputDoc = await vscode.workspace.openTextDocument({
          content: result.stdout || '(no output)',
          language: 'plaintext',
        });
        await vscode.window.showTextDocument(outputDoc);
        vscode.window.showInformationMessage('Go binary executed successfully');
      } else {
        const errorDoc = await vscode.workspace.openTextDocument({
          content: result.stderr || 'Unknown error',
          language: 'plaintext',
        });
        await vscode.window.showTextDocument(errorDoc);
        vscode.window.showErrorMessage(
          `Go binary failed with exit code ${result.exitCode}`
        );
      }
    }
  );
}

/**
 * Example: Run Go tests
 */
export async function runGoTestsCommand(): Promise<void> {
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    vscode.window.showWarningMessage('No workspace folder open');
    return;
  }

  const workspaceRoot = workspaceFolders[0].uri.fsPath;

  const goAvailable = await isGoAvailable();
  if (!goAvailable) {
    vscode.window.showErrorMessage('Go is not installed or not in PATH');
    return;
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Running Go tests...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: 'Executing go test...' });

      const result = await executeGoCommand('test', workspaceRoot, ['-v']);

      const outputDoc = await vscode.workspace.openTextDocument({
        content: result.stdout + (result.stderr ? '\n\nErrors:\n' + result.stderr : ''),
        language: 'plaintext',
      });
      await vscode.window.showTextDocument(outputDoc);

      if (result.exitCode === 0) {
        vscode.window.showInformationMessage('Go tests passed');
      } else {
        vscode.window.showErrorMessage('Go tests failed');
      }
    }
  );
}


