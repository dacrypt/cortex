/**
 * Project Inference Service
 * Automatically infers project characteristics from its members (files and subprojects)
 * using LLM analysis
 */

import * as vscode from 'vscode';
import { GrpcKnowledgeClient, Project } from '../core/GrpcKnowledgeClient';
import { GrpcRAGClient } from '../core/GrpcRAGClient';
import { AIQualityService } from './AIQualityService';
import { parseProjectAttributes, ProjectAttributes } from '../utils/projectVisualization';

export interface ProjectMember {
  type: 'file' | 'subproject';
  path?: string;
  name: string;
  nature?: string;
  attributes?: string;
  description?: string;
  tags?: string[];
  metadata?: Record<string, any>;
}

export interface ProjectInferenceResult {
  nature?: string;
  attributes?: ProjectAttributes;
  tags?: string[];
  description?: string;
  confidence: number;
  reasoning: string;
}

/**
 * Service to automatically infer project characteristics from members
 */
export class ProjectInferenceService {
  private aiService: AIQualityService;
  private knowledgeClient: GrpcKnowledgeClient;
  private ragClient: GrpcRAGClient;

  constructor(
    context: vscode.ExtensionContext,
    knowledgeClient: GrpcKnowledgeClient,
    ragClient: GrpcRAGClient
  ) {
    this.aiService = new AIQualityService(context);
    this.knowledgeClient = knowledgeClient;
    this.ragClient = ragClient;
  }

  /**
   * Infer project characteristics from its members
   */
  async inferProjectCharacteristics(
    workspaceId: string,
    projectId: string
  ): Promise<ProjectInferenceResult | null> {
    try {
      // Ensure LLM is available
      await this.aiService.requireLLM();

      // Get project
      const project = await this.knowledgeClient.getProject(workspaceId, projectId);
      if (!project) {
        return null;
      }

      // Get all members (files and subprojects)
      const members = await this.getProjectMembers(workspaceId, projectId);

      if (members.length === 0) {
        return null;
      }

      // Analyze members with LLM
      return await this.analyzeMembersWithLLM(project, members);
    } catch (error) {
      console.error('[ProjectInferenceService] Error inferring characteristics:', error);
      return null;
    }
  }

  /**
   * Get all members of a project (files and subprojects)
   */
  private async getProjectMembers(
    workspaceId: string,
    projectId: string
  ): Promise<ProjectMember[]> {
    const members: ProjectMember[] = [];

    try {
      // Get files in project
      const files = await this.knowledgeClient.queryDocuments(
        workspaceId,
        projectId,
        false // includeSubprojects
      );

      for (const file of files) {
        members.push({
          type: 'file',
          path: file.path,
          name: file.title || file.path.split('/').pop() || file.path,
        });
      }

      // Get subprojects
      const allProjects = await this.knowledgeClient.listProjects(workspaceId);
      const subprojects = allProjects.filter((p) => p.parent_id === projectId);

      for (const subproject of subprojects) {
        members.push({
          type: 'subproject',
          name: subproject.name,
          nature: subproject.nature,
          attributes: subproject.attributes,
          description: subproject.description,
        });
      }
    } catch (error) {
      console.error('[ProjectInferenceService] Error getting members:', error);
    }

    return members;
  }

  /**
   * Analyze project members with LLM to infer characteristics
   */
  private async analyzeMembersWithLLM(
    project: Project,
    members: ProjectMember[]
  ): Promise<ProjectInferenceResult> {
    // Build context from members
    const memberContext = this.buildMemberContext(members);

    const prompt = `You are a project analysis expert. Analyze a project and its members to infer the project's characteristics.

Project Name: "${project.name}"
${project.description ? `Current Description: "${project.description}"` : ''}
${project.nature ? `Current Nature: "${project.nature}"` : ''}

Project Members (${members.length} total):
${memberContext}

Based on the project name, current characteristics, and all its members (files and subprojects), infer:

1. **Nature**: The most appropriate project nature/type from the taxonomy
2. **Attributes**: Project attributes (status, priority, complexity, duration, scope, etc.)
3. **Tags**: Relevant tags for categorization
4. **Description**: A comprehensive description if missing or needs update

Available natures (comprehensive taxonomy):
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
- generic (only if none fit)

Respond in JSON format:
{
  "nature": "exact_nature_value",
  "attributes": {
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
  },
  "tags": ["tag1", "tag2", "tag3"],
  "description": "comprehensive project description",
  "confidence": 0.0-1.0,
  "reasoning": "brief explanation of inference"
}

Only respond with valid JSON, no other text.`;

    try {
      // Access LLM client through AIQualityService
      // We need to use the internal llmClient, but it's private
      // Create a temporary method call or use the service's public interface
      const llmClient = (this.aiService as any).llmClient;
      if (!llmClient) {
        throw new Error('LLM client not available');
      }
      
      const response = await llmClient.generateCompletion({
        prompt,
        maxTokens: 500,
        temperature: 0.3,
      });

      // Parse JSON response using robust parser
      const { parseJSON } = await import('../utils/llmParsers');
      const result = await parseJSON<ProjectInferenceResult>(response);

      // Validate nature (dynamically from PROJECT_NATURE_OPTIONS)
      const { PROJECT_NATURE_OPTIONS } = await import('../utils/projectNature');
      const validNatures = PROJECT_NATURE_OPTIONS.map(opt => opt.value).concat(['generic']);

      if (result.nature && !validNatures.includes(result.nature)) {
        result.nature = 'generic';
        result.confidence = Math.min(result.confidence || 0.5, 0.5);
      }

      return result;
    } catch (error) {
      console.error('[ProjectInferenceService] LLM analysis failed:', error);
      return {
        confidence: 0.0,
        reasoning: `AI analysis failed: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Build context string from project members
   */
  private buildMemberContext(members: ProjectMember[]): string {
    const contexts: string[] = [];

    // Group by type for better organization
    const files = members.filter(m => m.type === 'file');
    const subprojects = members.filter(m => m.type === 'subproject');

    if (files.length > 0) {
      contexts.push(`Files (${files.length}):`);
      for (const file of files.slice(0, 20)) { // Limit to 20 files to avoid token limits
        const ext = file.metadata?.extension || '';
        const fileInfo = ext ? `${file.name} [${ext}]` : file.name;
        contexts.push(`  - ${fileInfo}${file.path ? ` (${file.path})` : ''}`);
      }
      if (files.length > 20) {
        contexts.push(`  ... and ${files.length - 20} more files`);
      }
      
      // Add file type summary
      const extensions = new Set(files.map(f => f.metadata?.extension).filter(Boolean));
      if (extensions.size > 0) {
        contexts.push(`  File types: ${Array.from(extensions).join(', ')}`);
      }
    }

    if (subprojects.length > 0) {
      contexts.push(`\nSubprojects (${subprojects.length}):`);
      for (const subproject of subprojects) {
        const attrs = subproject.attributes ? parseProjectAttributes(subproject.attributes) : null;
        const status = attrs?.status || 'unknown';
        contexts.push(
          `  - ${subproject.name} (nature: ${subproject.nature || 'generic'}, status: ${status})`
        );
        if (subproject.description) {
          contexts.push(`    Description: ${subproject.description}`);
        }
      }
    }

    return contexts.join('\n');
  }

  /**
   * Apply inferred characteristics to project (merge with existing)
   */
  async applyInferenceToProject(
    workspaceId: string,
    projectId: string,
    inference: ProjectInferenceResult,
    mergeStrategy: 'merge' | 'replace' = 'merge'
  ): Promise<void> {
    try {
      const project = await this.knowledgeClient.getProject(workspaceId, projectId);
      if (!project) {
        return;
      }

      const updates: {
        nature?: string;
        attributes?: string;
        description?: string;
      } = {};

      // Update nature if confidence is high enough
      if (inference.nature && inference.confidence >= 0.7) {
        if (mergeStrategy === 'replace' || !project.nature || project.nature === 'generic') {
          updates.nature = inference.nature;
        }
      }

      // Merge attributes
      if (inference.attributes) {
        const existingAttrs = parseProjectAttributes(project.attributes);
        const mergedAttrs = mergeStrategy === 'merge' && existingAttrs
          ? { ...existingAttrs, ...inference.attributes }
          : inference.attributes;
        updates.attributes = JSON.stringify(mergedAttrs);
      }

      // Update description if missing or needs improvement
      if (inference.description) {
        if (mergeStrategy === 'replace' || !project.description) {
          updates.description = inference.description;
        } else if (project.description && inference.description.length > project.description.length) {
          // Use longer/more comprehensive description
          updates.description = inference.description;
        }
      }

      // Apply updates if any
      if (Object.keys(updates).length > 0) {
        await this.knowledgeClient.updateProject(workspaceId, projectId, updates);
        console.log(
          `[ProjectInferenceService] Applied inference to project ${projectId}:`,
          updates
        );
      }
    } catch (error) {
      console.error('[ProjectInferenceService] Error applying inference:', error);
    }
  }

  /**
   * Auto-infer and apply characteristics when project members change
   */
  async autoInferOnMemberChange(
    workspaceId: string,
    projectId: string,
    onUpdated?: () => void
  ): Promise<void> {
    try {
      const inference = await this.inferProjectCharacteristics(workspaceId, projectId);
      if (inference && inference.confidence >= 0.6) {
        await this.applyInferenceToProject(workspaceId, projectId, inference, 'merge');
        if (onUpdated) {
          onUpdated();
        }
      }
    } catch (error) {
      console.error('[ProjectInferenceService] Auto-inference failed:', error);
    }
  }
}

