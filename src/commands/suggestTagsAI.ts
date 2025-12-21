import * as vscode from 'vscode';
import { LLMService } from '../services/LLMService';
import { IMetadataStore } from '../core/IMetadataStore';

/**
 * Command to suggest tags for the current file using AI
 * Analyzes file content with a local LLM to recommend relevant tags
 */
export async function suggestTagsAI(
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
			title: 'Analyzing file with AI...',
			cancellable: false
		},
		async (progress) => {
			try {
				progress.report({ message: 'Generating tag suggestions...' });

				// Get AI suggestions
				const suggestedTags = await llmService.suggestTags(relativePath, content);

				if (suggestedTags.length === 0) {
					vscode.window.showInformationMessage('No tag suggestions generated');
					return;
				}

				// Get existing tags for this file
				const metadata = metadataStore.getMetadataByPath(relativePath);
				const existingTags = metadata?.tags || [];

				// Filter out tags that already exist
				const newTags = suggestedTags.filter(tag => !existingTags.includes(tag));

				if (newTags.length === 0) {
					vscode.window.showInformationMessage('All suggested tags are already applied');
					return;
				}

				// Let user select which tags to apply
				const selectedTags = await vscode.window.showQuickPick(
					newTags.map(tag => ({
						label: tag,
						picked: true, // Pre-select all by default
						description: existingTags.includes(tag) ? '(already applied)' : undefined
					})),
					{
						canPickMany: true,
						placeHolder: 'Select tags to apply (AI suggestions)',
						title: 'AI Tag Suggestions'
					}
				);

				if (!selectedTags || selectedTags.length === 0) {
					return;
				}

				// Apply selected tags
				progress.report({ message: 'Applying tags...' });

				for (const item of selectedTags) {
					await metadataStore.addTag(relativePath, item.label);
				}

				vscode.window.showInformationMessage(
					`✓ Added ${selectedTags.length} AI-suggested tag(s): ${selectedTags.map(t => t.label).join(', ')}`
				);

				// Refresh views to show updated tags
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
