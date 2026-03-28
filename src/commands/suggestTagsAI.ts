import * as vscode from 'vscode';
import { GrpcAdminClient } from '../core/GrpcAdminClient';

/**
 * Command to suggest tags for the current file using AI
 * Analyzes file content with a local LLM to recommend relevant tags
 */
export async function suggestTagsAI(
	adminClient: GrpcAdminClient,
	workspaceRoot: string,
	workspaceId?: string
): Promise<void> {
	const editor = vscode.window.activeTextEditor;
	if (!editor) {
		vscode.window.showErrorMessage('No active file open');
		return;
	}
	if (!workspaceId) {
		vscode.window.showErrorMessage('Backend workspace not available');
		return;
	}

	console.log('[Cortex][AI] suggestTagsAI invoked');

	const document = editor.document;
	const absolutePath = document.uri.fsPath;
	const relativePath = absolutePath.replace(workspaceRoot + '/', '');

	await vscode.window.withProgress(
		{
			location: vscode.ProgressLocation.Notification,
			title: 'Running backend AI indexing...',
			cancellable: false
		},
		async (progress) => {
			try {
				progress.report({ message: 'Processing file on backend...' });
				await adminClient.processFile(workspaceId, relativePath);
				vscode.window.showInformationMessage('✓ Backend AI indexing complete');
				vscode.commands.executeCommand('cortex.refreshViews');
			} catch (error) {
				console.error('Error in suggestTagsAI:', error);
				vscode.window.showErrorMessage(
					`Failed to suggest tags: ${error instanceof Error ? error.message : 'Unknown error'}`
				);
			}
		}
	);
}
