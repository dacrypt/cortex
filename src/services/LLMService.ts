import * as vscode from 'vscode';

/**
 * Service for interacting with local LLM providers (Ollama, LM Studio, etc.)
 * Provides AI-powered features like tag suggestions and project classification
 */
export class LLMService {
	/**
	 * Get LLM configuration from VS Code settings
	 */
	private get config() {
		return vscode.workspace.getConfiguration('cortex.llm');
	}

	/**
	 * Check if LLM features are enabled
	 */
	private get enabled(): boolean {
		return this.config.get('enabled', false);
	}

	/**
	 * Get the configured LLM endpoint URL
	 */
	private get endpoint(): string {
		return this.config.get('endpoint', 'http://localhost:11434');
	}

	/**
	 * Get the configured model name
	 */
	private get model(): string {
		return this.config.get('model', 'llama3.2');
	}

	/**
	 * Get the maximum number of tokens for context
	 */
	private get maxContextTokens(): number {
		return this.config.get('maxContextTokens', 2000);
	}

	/**
	 * Check if the LLM feature is enabled in settings
	 */
	isEnabled(): boolean {
		return this.enabled;
	}

	/**
	 * Check if the LLM service is available and responding
	 */
	async isAvailable(): Promise<boolean> {
		if (!this.enabled) {
			return false;
		}

		try {
			const controller = new AbortController();
			const timeout = setTimeout(() => controller.abort(), 5000);

			const response = await fetch(`${this.endpoint}/api/tags`, {
				signal: controller.signal
			});

			clearTimeout(timeout);
			return response.ok;
		} catch (error) {
			console.error('LLM service not available:', error);
			return false;
		}
	}

	/**
	 * Get list of available models from the LLM service
	 */
	async getAvailableModels(): Promise<string[]> {
		try {
			const response = await fetch(`${this.endpoint}/api/tags`);
			if (!response.ok) {
				return [];
			}

			const data = await response.json() as { models?: Array<{ name: string }> };
			return data.models?.map((m) => m.name) || [];
		} catch (error) {
			console.error('Failed to get available models:', error);
			return [];
		}
	}

	/**
	 * Suggest tags based on file content analysis
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file to analyze
	 * @returns Array of suggested tags
	 */
	async suggestTags(filePath: string, fileContent: string): Promise<string[]> {
		const prompt = `You are analyzing a file to suggest relevant organizational tags.

File: ${filePath}

Content (first ${this.maxContextTokens} chars):
${fileContent.slice(0, this.maxContextTokens)}

Based on the file path and content, suggest 3-5 relevant tags that would help organize this file.
Consider:
- The file's purpose (e.g., "config", "test", "documentation")
- The technology used (e.g., "typescript", "react", "api")
- The domain or feature (e.g., "auth", "database", "ui")
- The status or priority if apparent (e.g., "wip", "deprecated", "critical")

Return ONLY a JSON array of lowercase tag strings, nothing else.
Example: ["typescript", "api", "authentication", "backend"]`;

		try {
			const response = await this.generateCompletion(prompt);
			const tags = this.parseJsonResponse(response);

			if (Array.isArray(tags)) {
				return tags
					.filter(tag => typeof tag === 'string')
					.map(tag => tag.toLowerCase().trim())
					.filter(tag => tag.length > 0 && tag.length < 50)
					.slice(0, 5); // Limit to 5 tags
			}

			return [];
		} catch (error) {
			console.error('Failed to suggest tags:', error);
			return [];
		}
	}

	/**
	 * Suggest a project name based on file context and related files
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file
	 * @param recentFiles - Array of recently worked on file paths
	 * @returns Suggested project name or null
	 */
	async suggestProject(
		filePath: string,
		fileContent: string,
		recentFiles: string[]
	): Promise<string | null> {
		const recentFilesList = recentFiles.slice(0, 10).join('\n  - ');

		const prompt = `You are analyzing files to suggest a project name that groups related work.

Current file: ${filePath}

Content preview (first 500 chars):
${fileContent.slice(0, 500)}

Recently worked on files:
  - ${recentFilesList}

Based on the file paths and content, suggest a descriptive project name (2-4 words) that captures what these files are working on together.
The project name should be:
- Concise and descriptive
- Lowercase with hyphens (e.g., "user-authentication", "payment-api")
- Focused on the feature, component, or domain

If the files don't seem related to a common project, return exactly: null

Return ONLY the project name or null, nothing else.`;

		try {
			const response = await this.generateCompletion(prompt);
			const projectName = response.trim().toLowerCase();

			// Check if response is "null" or empty
			if (projectName === 'null' || projectName === '' || projectName === 'none') {
				return null;
			}

			// Validate project name format
			if (projectName.length > 0 && projectName.length < 100) {
				return projectName;
			}

			return null;
		} catch (error) {
			console.error('Failed to suggest project:', error);
			return null;
		}
	}

	/**
	 * Generate a summary or description for a file
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file
	 * @returns A concise summary of the file's purpose
	 */
	async generateFileSummary(filePath: string, fileContent: string): Promise<string | null> {
		const prompt = `Analyze this file and provide a concise one-sentence summary of its purpose.

File: ${filePath}

Content (first ${this.maxContextTokens} chars):
${fileContent.slice(0, this.maxContextTokens)}

Provide a brief, clear summary (max 100 characters) that describes what this file does.
Return ONLY the summary text, nothing else.`;

		try {
			const response = await this.generateCompletion(prompt);
			const summary = response.trim();

			if (summary.length > 0 && summary.length < 200) {
				return summary;
			}

			return null;
		} catch (error) {
			console.error('Failed to generate summary:', error);
			return null;
		}
	}

	/**
	 * Find files related to the current file based on semantic analysis
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file
	 * @param allFiles - List of all file paths in the workspace
	 * @returns Array of related file paths
	 */
	async findRelatedFiles(
		filePath: string,
		fileContent: string,
		allFiles: string[]
	): Promise<string[]> {
		const filesList = allFiles.slice(0, 100).join('\n  - ');

		const prompt = `You are analyzing a file to find related files in the workspace.

Current file: ${filePath}

Content preview (first 1000 chars):
${fileContent.slice(0, 1000)}

Available files in workspace:
  - ${filesList}

Based on the current file's content, identify files that are likely related or work together with it.
Consider:
- Import/export relationships
- Similar functionality or domain
- Test files for implementation files
- Configuration files
- Documentation

Return ONLY a JSON array of file paths (relative paths from the list above), nothing else.
Example: ["src/auth/login.ts", "src/auth/types.ts", "test/auth.test.ts"]
Maximum 10 files.`;

		try {
			const response = await this.generateCompletion(prompt);
			const relatedFiles = this.parseJsonResponse(response);

			if (Array.isArray(relatedFiles)) {
				return relatedFiles
					.filter(file => typeof file === 'string')
					.filter(file => allFiles.includes(file))
					.filter(file => file !== filePath)
					.slice(0, 10);
			}

			return [];
		} catch (error) {
			console.error('Failed to find related files:', error);
			return [];
		}
	}

	/**
	 * Generate a completion from the LLM
	 * @param prompt - The prompt to send to the LLM
	 * @returns The generated text response
	 */
	private async generateCompletion(prompt: string): Promise<string> {
		const controller = new AbortController();
		const timeout = setTimeout(() => controller.abort(), 30000); // 30 second timeout

		try {
			const response = await fetch(`${this.endpoint}/api/generate`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					model: this.model,
					prompt: prompt,
					stream: false,
					options: {
						temperature: 0.3, // Lower temperature for more deterministic outputs
						num_predict: 500, // Limit response length
					}
				}),
				signal: controller.signal
			});

			clearTimeout(timeout);

			if (!response.ok) {
				throw new Error(`LLM request failed: ${response.status} ${response.statusText}`);
			}

			const data = await response.json() as { response?: string };
			return data.response || '';
		} catch (error) {
			clearTimeout(timeout);
			if (error instanceof Error && error.name === 'AbortError') {
				throw new Error('LLM request timed out');
			}
			throw error;
		}
	}

	/**
	 * Parse JSON from LLM response, handling various response formats
	 * @param response - The raw response text from the LLM
	 * @returns Parsed JSON object or array
	 */
	private parseJsonResponse(response: string): any {
		// Try to extract JSON from markdown code blocks
		const jsonMatch = response.match(/```(?:json)?\s*(\[[\s\S]*?\]|\{[\s\S]*?\})\s*```/);
		if (jsonMatch) {
			try {
				return JSON.parse(jsonMatch[1]);
			} catch {
				// Continue to try parsing the full response
			}
		}

		// Try to find JSON array or object in the response
		const arrayMatch = response.match(/\[[\s\S]*?\]/);
		if (arrayMatch) {
			try {
				return JSON.parse(arrayMatch[0]);
			} catch {
				// Continue
			}
		}

		const objectMatch = response.match(/\{[\s\S]*?\}/);
		if (objectMatch) {
			try {
				return JSON.parse(objectMatch[0]);
			} catch {
				// Continue
			}
		}

		// Try parsing the entire response
		try {
			return JSON.parse(response);
		} catch {
			throw new Error(`Failed to parse JSON from response: ${response}`);
		}
	}

	/**
	 * Show an error message if LLM is not available
	 */
	async showNotAvailableMessage(): Promise<void> {
		// Check if the feature is disabled vs. service unavailable
		if (!this.enabled) {
			const choice = await vscode.window.showWarningMessage(
				'AI features are disabled. Enable "cortex.llm.enabled" in settings to use AI-powered commands.',
				'Enable Now',
				'Open Settings'
			);

			if (choice === 'Enable Now') {
				// Enable the setting directly
				await this.config.update('enabled', true, vscode.ConfigurationTarget.Workspace);
				vscode.window.showInformationMessage('AI features enabled! Make sure Ollama is running.');
			} else if (choice === 'Open Settings') {
				vscode.commands.executeCommand('workbench.action.openSettings', 'cortex.llm');
			}
		} else {
			const choice = await vscode.window.showErrorMessage(
				'Local LLM service not responding. Make sure Ollama is installed and running.',
				'Learn More',
				'Open Settings'
			);

			if (choice === 'Learn More') {
				vscode.env.openExternal(vscode.Uri.parse('https://ollama.ai/'));
			} else if (choice === 'Open Settings') {
				vscode.commands.executeCommand('workbench.action.openSettings', 'cortex.llm');
			}
		}
	}
}
