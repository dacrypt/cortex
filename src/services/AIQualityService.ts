/**
 * AI Quality Service - Ensures all operations use AI/LLM for quality guarantee
 * This service validates that LLM is available and enforces AI usage for all operations
 */

import * as vscode from 'vscode';
import { GrpcLLMClient } from '../core/GrpcLLMClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { PROJECT_NATURE_OPTIONS, ProjectNatureOption } from '../utils/projectNature';

export interface AIValidationResult {
  available: boolean;
  error?: string;
  providerId?: string;
  model?: string;
}

export interface ProjectNatureSuggestion {
  nature: string;
  confidence: number;
  reason: string;
  suggestedAttributes?: {
    status?: string;
    priority?: string;
    temporality?: string;
    collaboration?: string;
  };
}

export interface ProjectDescriptionSuggestion {
  description: string;
  confidence: number;
}

export interface ProjectNameValidation {
  valid: boolean;
  suggestions?: string[];
  issues?: string[];
  reason?: string;
}

/**
 * AI Quality Service - Mandatory AI validation and quality assurance
 */
export class AIQualityService {
  private llmClient: GrpcLLMClient;
  private adminClient: GrpcAdminClient;
  private context: vscode.ExtensionContext;

  constructor(context: vscode.ExtensionContext) {
    this.context = context;
    this.llmClient = new GrpcLLMClient(context);
    this.adminClient = new GrpcAdminClient(context);
  }

  /**
   * CRITICAL: Validate that LLM is available before any operation
   * This is MANDATORY for quality assurance
   */
  async validateLLMAvailable(): Promise<AIValidationResult> {
    try {
      const providers = await this.llmClient.listProviders();
      
      if (!providers || providers.length === 0) {
        return {
          available: false,
          error: 'No LLM providers configured. AI features are required for quality assurance.',
        };
      }

      // Check if any provider is available
      const availableProvider = providers.find((p: any) => p.available === true);
      
      if (!availableProvider) {
        return {
          available: false,
          error: 'LLM providers are configured but not available. Please ensure your LLM service (Ollama, LM Studio, etc.) is running.',
        };
      }

      return {
        available: true,
        providerId: availableProvider.id,
        model: availableProvider.model,
      };
    } catch (error) {
      return {
        available: false,
        error: `Failed to validate LLM availability: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * MANDATORY: Ensure LLM is available, throw error if not
   * This blocks operations that cannot guarantee quality without AI
   */
  async requireLLM(): Promise<void> {
    const validation = await this.validateLLMAvailable();
    if (!validation.available) {
      throw new Error(
        `CRITICAL: LLM is not available. ${validation.error}\n\n` +
        `Cortex requires AI/LLM to be available for quality assurance.\n` +
        `Please:\n` +
        `1. Ensure Ollama or your LLM service is running\n` +
        `2. Check your Cortex backend configuration\n` +
        `3. Verify LLM providers are properly configured\n\n` +
        `Operations cannot proceed without AI validation.`
      );
    }
  }

  /**
   * Use AI to suggest project nature based on name and context
   */
  async suggestProjectNature(
    projectName: string,
    description?: string,
    existingProjects?: string[]
  ): Promise<ProjectNatureSuggestion> {
    await this.requireLLM();

    const prompt = `You are a project classification expert. Analyze the project name and suggest the most appropriate project nature/type.

Project Name: "${projectName}"
${description ? `Description: "${description}"` : ''}
${existingProjects && existingProjects.length > 0 ? `Existing Projects: ${existingProjects.join(', ')}` : ''}

Available project natures (comprehensive taxonomy):
- Writing: writing.book, writing.thesis, writing.article, writing.documentation, writing.blog, writing.poetry, writing.screenplay, writing.manual, writing.report, writing.newsletter, writing.presentation
- Collections: collection.library, collection.archive, collection.reference, collection.playlist, collection.gallery, collection.dataset
- Development: development.software, development.erp, development.website, development.api, development.mobile, development.game, development.data-science, development.devops, development.database, development.blockchain, development.embedded
- Management: management.business, management.personal, management.family
- Hierarchical: hierarchical.parent, hierarchical.child, hierarchical.portfolio
- Purchase: purchase.vehicle, purchase.property, purchase.equipment, purchase.service, purchase.insurance, purchase.investment, purchase.subscription
- Education: education.course, education.research, education.school, education.training, education.certification, education.workshop, education.online-course
- Events: event.wedding, event.travel, event.conference, event.meeting, event.party, event.exhibition, event.seminar
- Reference: reference.knowledge_base, reference.template, reference.archive
- Industry (PMI/GICS): industry.energy, industry.materials, industry.industrials, industry.consumer-discretionary, industry.consumer-staples, industry.healthcare, industry.financial, industry.it, industry.telecommunications, industry.utilities, industry.real-estate, industry.construction, industry.manufacturing, industry.retail, industry.hospitality, industry.agriculture, industry.transportation, industry.consulting
- Content (PKM): content.research, content.learning, content.creative, content.analytical, content.administrative
- Responsibility (GTD): responsibility.personal, responsibility.professional, responsibility.family
- Purpose (Ontologies): purpose.creation, purpose.research, purpose.management, purpose.learning
- generic (only if none of the above fit)

Respond in JSON format:
{
  "nature": "exact_nature_value",
  "confidence": 0.0-1.0,
  "reason": "brief explanation",
  "suggestedAttributes": {
    "status": "planning|active|on-hold|completed|archived",
    "priority": "low|medium|high|critical",
    "complexity": "simple|moderate|complex",
    "duration": "short-term|medium-term|long-term",
    "scope": "small|medium|large|enterprise",
    "resultType": "unique project|continuous process|reference",
    "structure": "linear|hierarchical|network",
    "temporality": "temporary|ongoing",
    "collaboration": "individual|team|organization",
    "visibility": "private|shared|public"
  }
}

Only respond with valid JSON, no other text.`;

    try {
      const response = await this.llmClient.generateCompletion({
        prompt,
        maxTokens: 300,
        temperature: 0.3,
      });

      // Parse JSON response using robust parser
      const { parseJSON } = await import('../utils/llmParsers');
      const result = await parseJSON<ProjectNatureSuggestion>(response);
      
      // Validate nature is in allowed list (comprehensive taxonomy - dynamically generated)
      const validNatures = PROJECT_NATURE_OPTIONS.map((opt: ProjectNatureOption) => opt.value).concat(['generic']);

      if (!validNatures.includes(result.nature)) {
        result.nature = 'generic';
        result.confidence = 0.5;
        result.reason = 'AI suggested invalid nature, defaulting to generic';
      }

      return result;
    } catch (error) {
      console.error('[AIQualityService] Failed to suggest nature:', error);
      // Fallback to generic if AI fails
      return {
        nature: 'generic',
        confidence: 0.0,
        reason: `AI suggestion failed: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Use AI to generate project description
   */
  async generateProjectDescription(
    projectName: string,
    nature: string
  ): Promise<ProjectDescriptionSuggestion> {
    await this.requireLLM();

    const prompt = `Generate a brief, professional description for a project.

Project Name: "${projectName}"
Project Type: "${nature}"

Requirements:
- 1-2 sentences maximum
- Professional and clear
- Describes the project's purpose
- In the same language as the project name

Respond in JSON format:
{
  "description": "the description text",
  "confidence": 0.0-1.0
}

Only respond with valid JSON, no other text.`;

    try {
      const response = await this.llmClient.generateCompletion({
        prompt,
        maxTokens: 150,
        temperature: 0.5,
      });

      // Parse JSON response using robust parser
      const { parseJSON } = await import('../utils/llmParsers');
      const result = await parseJSON<ProjectDescriptionSuggestion>(response);
      return result;
    } catch (error) {
      console.error('[AIQualityService] Failed to generate description:', error);
      return {
        description: `Project: ${projectName}`,
        confidence: 0.0,
      };
    }
  }

  /**
   * Use AI to validate and suggest improvements for project name
   */
  async validateProjectName(
    projectName: string,
    existingProjects?: string[]
  ): Promise<ProjectNameValidation> {
    await this.requireLLM();

    const prompt = `You are a project naming expert. Validate and provide feedback on a project name.

Project Name: "${projectName}"
${existingProjects && existingProjects.length > 0 ? `Existing Projects: ${existingProjects.join(', ')}` : ''}

Evaluate:
1. Is the name clear and descriptive?
2. Does it follow good naming conventions?
3. Is it too similar to existing projects?
4. Are there any issues (too long, unclear, etc.)?

Respond in JSON format:
{
  "valid": true|false,
  "suggestions": ["alternative name 1", "alternative name 2"],
  "issues": ["issue 1", "issue 2"],
  "reason": "brief explanation"
}

Only respond with valid JSON, no other text.`;

    try {
      const response = await this.llmClient.generateCompletion({
        prompt,
        maxTokens: 200,
        temperature: 0.3,
      });

      // Parse JSON response using robust parser
      const { parseJSON } = await import('../utils/llmParsers');
      try {
        const result = await parseJSON<ProjectNameValidation>(response);
        return result;
      } catch (parseError) {
        // If parsing fails, accept the name (graceful degradation)
        return {
          valid: true,
          reason: 'AI validation unavailable, accepting name',
        };
      }
    } catch (error) {
      console.error('[AIQualityService] Failed to validate name:', error);
      return {
        valid: true,
        reason: 'AI validation failed, accepting name',
      };
    }
  }

  /**
   * Use AI to suggest project for file content
   * Uses backend LLM service for better integration
   */
  async suggestProjectForFile(
    workspaceId: string,
    relativePath: string,
    content?: string,
    existingProjects?: string[]
  ): Promise<{ project: string; confidence: number; reason: string }> {
    await this.requireLLM();

    try {
      // Use backend LLM service SuggestProject method if available
      // Fallback to direct completion if not
      const response = await this.llmClient.suggestProject({
        workspaceId,
        relativePath,
        content: content?.substring(0, 2000), // Limit content size
        existingProjects,
      });

      return {
        project: response.project || relativePath.split('/')[0],
        confidence: response.confidence || 0.5,
        reason: response.reason || 'AI suggestion',
      };
    } catch (error) {
      // Fallback to direct completion
      const contentPreview = content ? content.substring(0, 2000) : '';
      const prompt = `Analyze this file and suggest the most appropriate project name.

File Path: "${relativePath}"
Content Preview: "${contentPreview}"
${existingProjects && existingProjects.length > 0 ? `Existing Projects: ${existingProjects.join(', ')}` : ''}

Suggest a project name that:
1. Accurately represents the file's purpose
2. Is consistent with existing projects if similar
3. Follows good naming conventions
4. Is concise and clear

Respond in JSON format:
{
  "project": "suggested project name",
  "confidence": 0.0-1.0,
  "reason": "brief explanation"
}

Only respond with valid JSON, no other text.`;

      const response = await this.llmClient.generateCompletion({
        prompt,
        maxTokens: 200,
        temperature: 0.4,
      });

      // Parse JSON response using robust parser
      const { parseJSON } = await import('../utils/llmParsers');
      const result = await parseJSON<{ project: string; confidence: number; reason: string }>(response);
      return {
        project: result.project || relativePath.split('/')[0],
        confidence: result.confidence || 0.5,
        reason: result.reason || 'AI suggestion',
      };
    }
  }
}

