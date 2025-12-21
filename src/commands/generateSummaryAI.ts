import * as vscode from 'vscode';
import { LLMService } from '../services/LLMService';
import { IMetadataStore } from '../core/IMetadataStore';

/**
 * Command to generate a summary for the current file using AI
 * Creates a concise description that can be saved as notes
 */
export async function generateSummaryAI(
	llmService: LLMService,
	metadataStore: IMetadataStore,
	workspaceRoot: string
): Promise<void> {
	const editor = vscode.window.activeTextEditor;
	if (!editor) {
		vscode.window.showErrorMessage('No active file open');
		return;
	}

	// Check if LLM service is available
	const available = await llmService.isAvailable();
	if (!available) {
		await llmService.showNotAvailableMessage();
		return;
	}

	const document = editor.document;
	const absolutePath = document.uri.fsPath;
	const relativePath = absolutePath.replace(workspaceRoot + '/', '');
	const content = document.getText();

	// Show progress while analyzing
	await vscode.window.withProgress(
		{
			location: vscode.ProgressLocation.Notification,
			title: 'Generating summary with AI...',
			cancellable: false
		},
		async (progress) => {
			try {
				progress.report({ message: 'Analyzing file content...' });

				// Get AI summary
				const summary = await llmService.generateFileSummary(relativePath, content);

				if (!summary) {
					vscode.window.showInformationMessage('Could not generate summary');
					return;
				}

				// Get existing notes
				const metadata = metadataStore.getMetadataByPath(relativePath);
				const existingNotes = metadata?.notes || '';

				// Show the summary and ask if user wants to save it
				const action = await vscode.window.showInformationMessage(
					`AI Summary: "${summary}"`,
					'Save as Notes',
					'Append to Notes',
					'Copy',
					'Cancel'
				);

				if (action === 'Cancel' || !action) {
					return;
				}

				if (action === 'Copy') {
					await vscode.env.clipboard.writeText(summary);
					vscode.window.showInformationMessage('Summary copied to clipboard');
					return;
				}

				if (action === 'Save as Notes') {
					await metadataStore.updateNotes(relativePath, summary);
					vscode.window.showInformationMessage('✓ Summary saved as notes');
					vscode.commands.executeCommand('cortex.refreshViews');
				} else if (action === 'Append to Notes') {
					const newNotes = existingNotes
						? `${existingNotes}\n\nAI Summary: ${summary}`
						: summary;
					await metadataStore.updateNotes(relativePath, newNotes);
					vscode.window.showInformationMessage('✓ Summary appended to notes');
					vscode.commands.executeCommand('cortex.refreshViews');
				}

			} catch (error) {
				console.error('Error in generateSummaryAI:', error);
				vscode.window.showErrorMessage(
					`Failed to generate summary: ${error instanceof Error ? error.message : 'Unknown error'}`
				);
			}
		}
	);
}
