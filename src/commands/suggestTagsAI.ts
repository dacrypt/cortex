import * as vscode from 'vscode';
import * as path from 'path';
import { LLMService } from '../services/LLMService';
import { IMetadataStore } from '../core/IMetadataStore';
import { MirrorStore } from '../core/MirrorStore';
import { FileMetadata } from '../models/types';
import { extractKeyTerms } from '../utils/aiSummary';
import { resolveAIContent } from '../utils/aiContent';

/**
 * Command to suggest tags for the current file using AI
 * Analyzes file content with a local LLM to recommend relevant tags
 */
export async function suggestTagsAI(
	llmService: LLMService,
	metadataStore: IMetadataStore,
	mirrorStore: MirrorStore,
	workspaceRoot: string
): Promise<void> {
	const editor = vscode.window.activeTextEditor;
	if (!editor) {
		vscode.window.showErrorMessage('No active file open');
		return;
	}

	console.log('[Cortex][AI] suggestTagsAI invoked');

	// Check if LLM service is available
	const available = await llmService.isAvailable();
	if (!available) {
		await llmService.showNotAvailableMessage();
		return;
	}

	const document = editor.document;
	const absolutePath = document.uri.fsPath;
	const relativePath = absolutePath.replace(workspaceRoot + '/', '');
	const extension = path.extname(absolutePath);
	const aiContent = await resolveAIContent(
		{
			absolutePath,
			relativePath,
			extension,
			editorText: document.getText()
		},
		mirrorStore
	);
	if (!aiContent) {
		vscode.window.showErrorMessage('No readable content available for AI analysis');
		return;
	}
	console.log(`[Cortex][AI] suggestTagsAI content ready for ${relativePath}`);
	const content = aiContent.content;
	const contentHash = aiContent.contentHash;
	const metadata = metadataStore.getMetadataByPath(relativePath) as FileMetadata | null;
	const summary =
		metadata?.aiSummary && metadata?.aiSummaryHash === contentHash
			? metadata.aiSummary
			: undefined;
	const keyTerms =
		metadata?.aiKeyTerms && metadata.aiKeyTerms.length > 0
			? metadata.aiKeyTerms
			: extractKeyTerms(content);

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
				const suggestedTags = await llmService.suggestTags(relativePath, content, {
					summary,
					keyTerms,
					snippet: content.slice(0, 500)
				});

				if (suggestedTags.length === 0) {
					console.log(`[Cortex][AI] No tag suggestions for ${relativePath}`);
					vscode.window.showInformationMessage('No tag suggestions generated');
					return;
				}

				// Get existing tags for this file
				const existingTags = metadata?.tags || [];

				// Filter out tags that already exist
				const newTags = suggestedTags.filter(tag => !existingTags.includes(tag));

				if (newTags.length === 0) {
					console.log(`[Cortex][AI] Tag suggestions already applied for ${relativePath}`);
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
				console.log(
					`[Cortex][AI] Applied ${selectedTags.length} tag(s) to ${relativePath}`
				);

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
