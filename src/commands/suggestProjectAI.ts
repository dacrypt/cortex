import * as vscode from 'vscode';
import { LLMService } from '../services/LLMService';
import { IMetadataStore } from '../core/IMetadataStore';

/**
 * Command to suggest a project name for the current file using AI
 * Analyzes file content and recent work context to recommend a project assignment
 */
export async function suggestProjectAI(
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
			title: 'Analyzing context with AI...',
			cancellable: false
		},
		async (progress) => {
			try {
				progress.report({ message: 'Getting file context...' });

				// Get files from existing projects as context
				const allContexts = metadataStore.getAllContexts();
				const recentFiles: string[] = [];

				// Get files from the same directory
				const dirPath = relativePath.substring(0, relativePath.lastIndexOf('/'));

				// Add some context files from existing projects (max 10)
				for (const context of allContexts.slice(0, 3)) {
					const filesInContext = metadataStore.getFilesByContext(context);
					recentFiles.push(...filesInContext.slice(0, 3));
				}

				// Add the directory path as context
				if (dirPath) {
					recentFiles.push(`${dirPath}/*`);
				}

				progress.report({ message: 'Generating project suggestion...' });

				// Get AI suggestion
				const suggestedProject = await llmService.suggestProject(
					relativePath,
					content,
					recentFiles
				);

				if (!suggestedProject) {
					vscode.window.showInformationMessage(
						'AI could not suggest a project based on current context'
					);
					return;
				}

				// Get existing projects for this file
				const metadata = metadataStore.getMetadataByPath(relativePath);
				const existingProjects = metadata?.contexts || [];

				if (existingProjects.includes(suggestedProject)) {
					vscode.window.showInformationMessage(
						`File is already assigned to project: ${suggestedProject}`
					);
					return;
				}

				// Ask user to confirm the suggestion
				const action = await vscode.window.showInformationMessage(
					`AI suggests project: "${suggestedProject}"`,
					'Apply',
					'Edit & Apply',
					'Cancel'
				);

				if (action === 'Cancel' || !action) {
					return;
				}

				let finalProjectName = suggestedProject;

				if (action === 'Edit & Apply') {
					const edited = await vscode.window.showInputBox({
						prompt: 'Edit the project name',
						value: suggestedProject,
						placeHolder: 'project-name',
						validateInput: (value) => {
							if (!value || value.trim().length === 0) {
								return 'Project name cannot be empty';
							}
							if (value.length > 100) {
								return 'Project name too long (max 100 characters)';
							}
							return null;
						}
					});

					if (!edited) {
						return;
					}

					finalProjectName = edited.trim();
				}

				// Apply the project
				progress.report({ message: 'Assigning project...' });
				await metadataStore.addContext(relativePath, finalProjectName);

				vscode.window.showInformationMessage(
					`✓ Assigned file to project: ${finalProjectName}`
				);

				// Refresh views to show updated project
				vscode.commands.executeCommand('cortex.refreshViews');

			} catch (error) {
				console.error('Error in suggestProjectAI:', error);
				vscode.window.showErrorMessage(
					`Failed to suggest project: ${error instanceof Error ? error.message : 'Unknown error'}`
				);
			}
		}
	);
}
