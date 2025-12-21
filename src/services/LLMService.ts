import * as vscode from 'vscode';

/**
 * Service for interacting with local LLM providers (Ollama, LM Studio, etc.)
 * Provides AI-powered features like tag suggestions and project classification
 */
export class LLMService {
	private log(message: string): void {
		console.log(`[Cortex][LLM] ${message}`);
	}
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
			this.log('LLM disabled in settings');
			return false;
		}

		try {
			this.log(`Checking availability at ${this.endpoint}`);
			const started = Date.now();
			const controller = new AbortController();
			const timeout = setTimeout(() => controller.abort(), 5000);

			const response = await fetch(`${this.endpoint}/api/tags`, {
				signal: controller.signal
			});

			clearTimeout(timeout);
			this.log(
				`Availability check ${response.ok ? 'ok' : 'failed'} (${Date.now() - started}ms)`
			);
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
			this.log(`Fetching available models from ${this.endpoint}`);
			const started = Date.now();
			const response = await fetch(`${this.endpoint}/api/tags`);
			if (!response.ok) {
				this.log(
					`Model list request failed: ${response.status} ${response.statusText}`
				);
				return [];
			}

			const data = await response.json() as { models?: Array<{ name: string }> };
			const models = data.models?.map((m) => m.name) || [];
			this.log(`Model list received (${models.length} models, ${Date.now() - started}ms)`);
			return models;
		} catch (error) {
			console.error('Failed to get available models:', error);
			return [];
		}
	}

	/**
	 * Suggest tags based on file content analysis
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file to analyze
	 * @param options - Optional summary-driven inputs
	 * @returns Array of suggested tags
	 */
	async suggestTags(
		filePath: string,
		fileContent: string,
		options?: { summary?: string; keyTerms?: string[]; snippet?: string }
	): Promise<string[]> {
		const summary = options?.summary?.trim();
		const keyTerms = options?.keyTerms?.filter(Boolean) ?? [];
		const snippet = options?.snippet?.trim();
		const contextBlock = summary
			? `Summary:
${summary}
${keyTerms.length > 0 ? `Key terms: ${keyTerms.join(', ')}` : ''}
${snippet ? `Snippet:\n${snippet.slice(0, 500)}` : ''}`
			: `Content (first ${this.maxContextTokens} chars):
${fileContent.slice(0, this.maxContextTokens)}`;

		const prompt = `You are analyzing a file to suggest relevant organizational tags.

File: ${filePath}

${contextBlock}

Based on the file path and content, suggest 3-5 relevant tags that would help organize this file.
Consider:
- The file's purpose (e.g., "config", "test", "documentation")
- The technology used (e.g., "typescript", "react", "api")
- The domain or feature (e.g., "auth", "database", "ui")
- The status or priority if apparent (e.g., "wip", "deprecated", "critical")

Return ONLY a JSON array of lowercase tag strings, nothing else.
Example: ["typescript", "api", "authentication", "backend"]`;

		try {
			this.log(
				`[suggestTags] start file=${filePath} mode=${summary ? 'summary' : 'content'} contentChars=${fileContent.length} summaryChars=${summary?.length ?? 0} keyTerms=${keyTerms.length} snippetChars=${snippet?.length ?? 0}`
			);
			const response = await this.generateCompletion(prompt);
			this.log(`[suggestTags] Raw LLM response received, parsing JSON...`);
			const tags = this.parseJsonResponse(response);
			this.log(`[suggestTags] Parsed tags: ${JSON.stringify(tags)}`);

			if (Array.isArray(tags)) {
				const normalized = tags
					.filter(tag => typeof tag === 'string')
					.map(tag => tag.toLowerCase().trim())
					.filter(tag => tag.length > 0 && tag.length < 50)
					.slice(0, 5); // Limit to 5 tags
				this.log(`[suggestTags] done file=${filePath} tags=${normalized.length} result=${JSON.stringify(normalized)}`);
				return normalized;
			}

			this.log(`[suggestTags] done file=${filePath} tags=0 (not an array)`);
			return [];
		} catch (error) {
			this.log(`[suggestTags] ERROR file=${filePath}: ${error instanceof Error ? error.message : String(error)}`);
			console.error('Failed to suggest tags:', error);
			return [];
		}
	}

	/**
	 * Suggest a project name based on file context and related files
	 * @param filePath - The relative path of the file
	 * @param fileContent - The content of the file
	 * @param recentFiles - Array of recently worked on file paths
	 * @param options - Optional summary-driven inputs
	 * @returns Suggested project name or null
	 */
	async suggestProject(
		filePath: string,
		fileContent: string,
		recentFiles: string[],
		options?: { summary?: string; keyTerms?: string[]; snippet?: string }
	): Promise<string | null> {
		const recentFilesList = recentFiles.slice(0, 10).join('\n  - ');
		const summary = options?.summary?.trim();
		const keyTerms = options?.keyTerms?.filter(Boolean) ?? [];
		const snippet = options?.snippet?.trim();
		const contextBlock = summary
			? `Summary:
${summary}
${keyTerms.length > 0 ? `Key terms: ${keyTerms.join(', ')}` : ''}
${snippet ? `Snippet:\n${snippet.slice(0, 500)}` : ''}`
			: `Content preview (first 500 chars):
${fileContent.slice(0, 500)}`;

		const prompt = `You are analyzing files to suggest a project name that groups related work.

Current file: ${filePath}

${contextBlock}

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
			this.log(
				`[suggestProject] start file=${filePath} mode=${summary ? 'summary' : 'content'} contentChars=${fileContent.length} summaryChars=${summary?.length ?? 0} keyTerms=${keyTerms.length} snippetChars=${snippet?.length ?? 0} recentFiles=${recentFiles.length}`
			);
			const response = await this.generateCompletion(prompt);
			const projectName = response.trim().toLowerCase();
			this.log(`[suggestProject] Raw response after trim/lowercase: "${projectName}"`);

			// Check if response is "null" or empty
			if (projectName === 'null' || projectName === '' || projectName === 'none') {
				this.log(`[suggestProject] done file=${filePath} result=null (empty/null response)`);
				return null;
			}

			// Validate project name format
			if (projectName.length > 0 && projectName.length < 100) {
				this.log(`[suggestProject] done file=${filePath} result="${projectName}"`);
				return projectName;
			}

			this.log(`[suggestProject] done file=${filePath} result=null (invalid length: ${projectName.length})`);
			return null;
		} catch (error) {
			this.log(`[suggestProject] ERROR file=${filePath}: ${error instanceof Error ? error.message : String(error)}`);
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
			this.log(
				`[generateSummary] start file=${filePath} contentChars=${fileContent.length}`
			);
			const response = await this.generateCompletion(prompt);
			const summary = response.trim();
			this.log(`[generateSummary] Raw response after trim: "${summary}" (length: ${summary.length})`);

			if (summary.length > 0 && summary.length < 200) {
				this.log(`[generateSummary] done file=${filePath} summaryChars=${summary.length} result="${summary}"`);
				return summary;
			}

			this.log(`[generateSummary] done file=${filePath} summaryChars=0 (invalid length: ${summary.length})`);
			return null;
		} catch (error) {
			this.log(`[generateSummary] ERROR file=${filePath}: ${error instanceof Error ? error.message : String(error)}`);
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
			this.log(
				`[findRelatedFiles] start file=${filePath} contentChars=${fileContent.length} candidates=${allFiles.length}`
			);
			const response = await this.generateCompletion(prompt);
			this.log(`[findRelatedFiles] Raw LLM response received, parsing JSON...`);
			const relatedFiles = this.parseJsonResponse(response);
			this.log(`[findRelatedFiles] Parsed files: ${JSON.stringify(relatedFiles)}`);

			if (Array.isArray(relatedFiles)) {
				const filtered = relatedFiles
					.filter(file => typeof file === 'string')
					.filter(file => allFiles.includes(file))
					.filter(file => file !== filePath)
					.slice(0, 10);
				this.log(
					`[findRelatedFiles] done file=${filePath} related=${filtered.length} result=${JSON.stringify(filtered)}`
				);
				return filtered;
			}

			this.log(`[findRelatedFiles] done file=${filePath} related=0 (not an array)`);
			return [];
		} catch (error) {
			this.log(`[findRelatedFiles] ERROR file=${filePath}: ${error instanceof Error ? error.message : String(error)}`);
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
		const started = Date.now();
		
		const requestPayload = {
			model: this.model,
			prompt: prompt,
			stream: false,
			options: {
				temperature: 0.3, // Lower temperature for more deterministic outputs
				num_predict: 500, // Limit response length
			}
		};

		// Log input details
		this.log(`[INPUT] Request start model=${this.model} endpoint=${this.endpoint} promptChars=${prompt.length}`);
		this.log(`[INPUT] Full prompt:\n${'='.repeat(80)}\n${prompt}\n${'='.repeat(80)}`);
		this.log(`[INPUT] Request payload: ${JSON.stringify(requestPayload, null, 2)}`);

		try {
			const response = await fetch(`${this.endpoint}/api/generate`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify(requestPayload),
				signal: controller.signal
			});

			clearTimeout(timeout);

			if (!response.ok) {
				const errorText = await response.text();
				this.log(`[ERROR] Request failed: ${response.status} ${response.statusText}`);
				this.log(`[ERROR] Error response: ${errorText}`);
				throw new Error(`LLM request failed: ${response.status} ${response.statusText}`);
			}

			const data = await response.json() as { response?: string };
			const responseText = data.response || '';
			const duration = Date.now() - started;
			
			// Log output details
			this.log(`[OUTPUT] Request success model=${this.model} durationMs=${duration} responseChars=${responseText.length}`);
			this.log(`[OUTPUT] Full response:\n${'='.repeat(80)}\n${responseText}\n${'='.repeat(80)}`);
			this.log(`[OUTPUT] Raw response data: ${JSON.stringify(data, null, 2)}`);
			
			return responseText;
		} catch (error) {
			clearTimeout(timeout);
			const duration = Date.now() - started;
			this.log(`[ERROR] Request failed model=${this.model} durationMs=${duration}`);
			if (error instanceof Error) {
				this.log(`[ERROR] Error message: ${error.message}`);
				this.log(`[ERROR] Error stack: ${error.stack}`);
				if (error.name === 'AbortError') {
					throw new Error('LLM request timed out');
				}
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
		this.log(`[parseJsonResponse] Attempting to parse response (length: ${response.length})`);
		
		// Try to extract JSON from markdown code blocks
		const jsonMatch = response.match(/```(?:json)?\s*(\[[\s\S]*?\]|\{[\s\S]*?\})\s*```/);
		if (jsonMatch) {
			this.log(`[parseJsonResponse] Found JSON in markdown code block`);
			try {
				const parsed = JSON.parse(jsonMatch[1]);
				this.log(`[parseJsonResponse] Successfully parsed from markdown code block`);
				return parsed;
			} catch (error) {
				this.log(`[parseJsonResponse] Failed to parse from markdown code block: ${error instanceof Error ? error.message : String(error)}`);
				// Continue to try parsing the full response
			}
		}

		// Try to find JSON array or object in the response
		const arrayMatch = response.match(/\[[\s\S]*?\]/);
		if (arrayMatch) {
			this.log(`[parseJsonResponse] Found JSON array pattern`);
			try {
				const parsed = JSON.parse(arrayMatch[0]);
				this.log(`[parseJsonResponse] Successfully parsed array`);
				return parsed;
			} catch (error) {
				this.log(`[parseJsonResponse] Failed to parse array: ${error instanceof Error ? error.message : String(error)}`);
				// Continue
			}
		}

		const objectMatch = response.match(/\{[\s\S]*?\}/);
		if (objectMatch) {
			this.log(`[parseJsonResponse] Found JSON object pattern`);
			try {
				const parsed = JSON.parse(objectMatch[0]);
				this.log(`[parseJsonResponse] Successfully parsed object`);
				return parsed;
			} catch (error) {
				this.log(`[parseJsonResponse] Failed to parse object: ${error instanceof Error ? error.message : String(error)}`);
				// Continue
			}
		}

		// Try parsing the entire response
		this.log(`[parseJsonResponse] Attempting to parse entire response as JSON`);
		try {
			const parsed = JSON.parse(response);
			this.log(`[parseJsonResponse] Successfully parsed entire response`);
			return parsed;
		} catch (error) {
			this.log(`[parseJsonResponse] ERROR: Failed to parse JSON from response`);
			this.log(`[parseJsonResponse] Response content: ${response.substring(0, 500)}${response.length > 500 ? '...' : ''}`);
			throw new Error(`Failed to parse JSON from response: ${response.substring(0, 200)}${response.length > 200 ? '...' : ''}`);
		}
	}

	/**
	 * Classify a document into a library category based on its content and metadata
	 * @param filePath - The relative path of the file
	 * @param options - Document metadata including summary, tags, contexts, etc.
	 * @returns Category name in Spanish or null
	 */
	async classifyDocumentCategory(
		filePath: string,
		options: {
			summary?: string;
			keyTerms?: string[];
			tags?: string[];
			contexts?: string[];
			type?: string;
		}
	): Promise<string | null> {
		const summary = options?.summary?.trim();
		const keyTerms = options?.keyTerms?.filter(Boolean) ?? [];
		const tags = options?.tags?.filter(Boolean) ?? [];
		const contexts = options?.contexts?.filter(Boolean) ?? [];
		const type = options?.type || '';

		const contextBlock = summary
			? `Resumen del documento:
${summary}
${keyTerms.length > 0 ? `Términos clave: ${keyTerms.join(', ')}` : ''}
${tags.length > 0 ? `Etiquetas: ${tags.join(', ')}` : ''}
${contexts.length > 0 ? `Proyectos/Contextos: ${contexts.join(', ')}` : ''}
Tipo de archivo: ${type}`
			: `Tipo de archivo: ${type}
${tags.length > 0 ? `Etiquetas: ${tags.join(', ')}` : ''}
${contexts.length > 0 ? `Proyectos/Contextos: ${contexts.join(', ')}` : ''}
${keyTerms.length > 0 ? `Términos clave: ${keyTerms.join(', ')}` : ''}`;

		const prompt = `Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Archivo: ${filePath}

${contextBlock}

Basándote en el resumen, términos clave, etiquetas, proyectos y tipo de archivo, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- Ciencia y Tecnología
- Arte y Diseño
- Negocios y Finanzas
- Educación y Referencia
- Literatura y Escritura
- Documentación Técnica
- Recursos Humanos
- Marketing y Comunicación
- Legal y Regulatorio
- Salud y Medicina
- Ingeniería y Construcción
- Investigación y Análisis
- Configuración y Administración
- Pruebas y Calidad
- Sin Clasificar

Responde SOLO con el nombre de la categoría, nada más.
Si no puedes determinar la categoría, responde: "Sin Clasificar"`;

		try {
			this.log(
				`[classifyDocumentCategory] start file=${filePath} summaryChars=${summary?.length ?? 0} tags=${tags.length} contexts=${contexts.length}`
			);
			const response = await this.generateCompletion(prompt);
			const category = response.trim();
			this.log(`[classifyDocumentCategory] Raw response after trim: "${category}"`);

			// Validate category
			const validCategories = [
				'Ciencia y Tecnología',
				'Arte y Diseño',
				'Negocios y Finanzas',
				'Educación y Referencia',
				'Literatura y Escritura',
				'Documentación Técnica',
				'Recursos Humanos',
				'Marketing y Comunicación',
				'Legal y Regulatorio',
				'Salud y Medicina',
				'Ingeniería y Construcción',
				'Investigación y Análisis',
				'Configuración y Administración',
				'Pruebas y Calidad',
				'Sin Clasificar',
			];

			// Try to match the response to a valid category
			const matchedCategory = validCategories.find((cat) =>
				category.toLowerCase().includes(cat.toLowerCase())
			);

			if (matchedCategory) {
				this.log(`[classifyDocumentCategory] done file=${filePath} category="${matchedCategory}" (matched from "${category}")`);
				return matchedCategory;
			}

			// If no match, return "Sin Clasificar"
			this.log(`[classifyDocumentCategory] done file=${filePath} category="Sin Clasificar" (no match for "${category}")`);
			return 'Sin Clasificar';
		} catch (error) {
			this.log(`[classifyDocumentCategory] ERROR file=${filePath}: ${error instanceof Error ? error.message : String(error)}`);
			console.error('Failed to classify document category:', error);
			return 'Sin Clasificar';
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
