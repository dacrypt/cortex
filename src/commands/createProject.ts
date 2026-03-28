/**
 * Command: Create a new project with AI-powered nature selection
 * MANDATORY: All project creation requires AI validation for quality assurance
 */

import * as vscode from 'vscode';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import {
  PROJECT_NATURE_CATEGORIES,
  createNatureQuickPickItems,
  DEFAULT_NATURE,
  getNatureLabel,
} from '../utils/projectNature';
import { AIQualityService } from '../services/AIQualityService';

export async function createProjectCommand(
  workspaceRoot: string,
  knowledgeClient: GrpcKnowledgeClient,
  workspaceId: string,
  onMetadataChanged: () => void,
  context?: vscode.ExtensionContext
): Promise<void> {
  // CRITICAL: Initialize AI Quality Service - mandatory for quality assurance
  if (!context) {
    vscode.window.showErrorMessage('Extension context required for AI validation');
    return;
  }

  const aiService = new AIQualityService(context);

  try {
    // MANDATORY: Validate LLM is available before proceeding
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'Validating AI availability...',
        cancellable: false,
      },
      async (progress) => {
        progress.report({ message: 'Checking LLM availability...' });
        await aiService.requireLLM();
      }
    );

    // Step 1: Get project name
    const projectName = await vscode.window.showInputBox({
      prompt: 'Enter project name',
      placeHolder: 'e.g., Mi Libro, Compra Vehículo, ERP Empresa',
      validateInput: (value) => {
        if (!value || value.trim().length === 0) {
          return 'Project name cannot be empty';
        }
        if (value.includes(',')) {
          return 'Project name cannot contain commas';
        }
        return null;
      },
    });

    if (!projectName) {
      return; // User cancelled
    }

    const normalizedName = projectName.trim();

    // Check if project already exists
    const existing = await knowledgeClient.getProjectByName(workspaceId, normalizedName);
    if (existing) {
      const overwrite = await vscode.window.showWarningMessage(
        `Project "${normalizedName}" already exists. Do you want to edit it instead?`,
        'Edit Project',
        'Cancel'
      );
      if (overwrite === 'Edit Project') {
        await editProjectCommand(workspaceRoot, knowledgeClient, workspaceId, existing.id, onMetadataChanged, context);
      }
      return;
    }

    // Step 2: AI-powered nature suggestion
    const existingProjects = await knowledgeClient.listProjects(workspaceId);
    const existingProjectNames = existingProjects.map(p => p.name);

    // MANDATORY: Get AI suggestion for nature
    const natureSuggestion = await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'AI analyzing project...',
        cancellable: false,
      },
      async (progress) => {
        progress.report({ message: 'AI suggesting project nature...' });
        return await aiService.suggestProjectNature(
          normalizedName,
          undefined,
          existingProjectNames
        );
      }
    );

    // Show AI suggestion with option to override
    const natureItems = createNatureQuickPickItems();
    const aiSuggestedItem = natureItems.find(item => item.detail === natureSuggestion.nature);
    
    const quickPick = vscode.window.createQuickPick<vscode.QuickPickItem>();
    quickPick.items = natureItems;
    quickPick.placeholder = `AI suggests: ${getNatureLabel(natureSuggestion.nature)} (${(natureSuggestion.confidence * 100).toFixed(0)}% confidence) - ${natureSuggestion.reason}`;
    quickPick.matchOnDescription = true;
    quickPick.matchOnDetail = true;
    if (aiSuggestedItem) {
      quickPick.selectedItems = [aiSuggestedItem];
    }

    const naturePromise = new Promise<vscode.QuickPickItem | undefined>((resolve) => {
      quickPick.onDidAccept(() => {
        resolve(quickPick.selectedItems[0]);
        quickPick.dispose();
      });
      quickPick.onDidHide(() => {
        resolve(undefined);
        quickPick.dispose();
      });
    });

    quickPick.show();
    const selectedNature = await naturePromise;

    if (!selectedNature) {
      return; // User cancelled
    }

    const nature = (selectedNature as any).detail || natureSuggestion.nature;

    // Step 3: AI-generated description (with option to override)
    let description: string | undefined;

    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: 'AI generating description...',
        cancellable: false,
      },
      async (progress) => {
        progress.report({ message: 'AI creating project description...' });
        const descSuggestion = await aiService.generateProjectDescription(normalizedName, nature);
        
        // Show AI-generated description with option to edit
        const userDescription = await vscode.window.showInputBox({
          prompt: 'Project description (AI-generated, you can edit)',
          value: descSuggestion.description,
          placeHolder: 'Brief description of the project',
        });
        
        description = userDescription?.trim() || descSuggestion.description;
      }
    );

    // Step 4: Create project with AI-validated nature and attributes
    const attributes = natureSuggestion.suggestedAttributes 
      ? JSON.stringify(natureSuggestion.suggestedAttributes)
      : undefined;

    const project = await knowledgeClient.createProjectWithNature(
      workspaceId,
      normalizedName,
      description,
      undefined, // no parent
      nature,
      attributes
    );

    vscode.window.showInformationMessage(
      `✓ Created project "${normalizedName}" (${getNatureLabel(nature)}) with AI validation`
    );

    // If project has a parent, auto-infer parent characteristics from new subproject
    if (project.parent_id && context) {
      const { ProjectInferenceService } = await import('../services/ProjectInferenceService');
      const { GrpcRAGClient } = await import('../core/GrpcRAGClient');
      const ragClient = new GrpcRAGClient(context);
      const inferenceService = new ProjectInferenceService(context, knowledgeClient, ragClient);
      
      // Update parent project characteristics based on new subproject
      inferenceService.autoInferOnMemberChange(workspaceId, project.parent_id, () => {
        onMetadataChanged();
      }).catch((error) => {
        console.error('[createProject] Parent auto-inference failed:', error);
      });
    }

    // Refresh views
    onMetadataChanged();
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    
    // Check if it's an LLM availability error
    if (errorMessage.includes('LLM is not available') || errorMessage.includes('CRITICAL')) {
      vscode.window.showErrorMessage(
        `❌ ${errorMessage}\n\n` +
        `Cortex requires AI/LLM to be available for quality assurance.\n` +
        `Please ensure your LLM service is running and configured.`
      );
    } else {
      vscode.window.showErrorMessage(`Failed to create project: ${errorMessage}`);
    }
    console.error('[Cortex] createProjectCommand error:', error);
  }
}

/**
 * Command: Edit an existing project
 */
export async function editProjectCommand(
  workspaceRoot: string,
  knowledgeClient: GrpcKnowledgeClient,
  workspaceId: string,
  projectId: string,
  onMetadataChanged: () => void,
  context?: vscode.ExtensionContext
): Promise<void> {
  // CRITICAL: AI validation required for edits
  if (!context) {
    vscode.window.showErrorMessage('Extension context required for AI validation');
    return;
  }

  const aiService = new AIQualityService(context);
  try {
    // MANDATORY: Validate LLM is available
    await aiService.requireLLM();

    // Get existing project
    const project = await knowledgeClient.getProject(workspaceId, projectId);
    if (!project) {
      vscode.window.showErrorMessage('Project not found');
      return;
    }

    // Show edit options
    const editOptions = [
      'Edit Name',
      'Edit Description',
      'Change Nature',
      'Edit Attributes',
      'Cancel',
    ];

    const selected = await vscode.window.showQuickPick(editOptions, {
      placeHolder: `Edit project: ${project.name}`,
    });

    if (!selected || selected === 'Cancel') {
      return;
    }

    switch (selected) {
      case 'Edit Name':
        const newName = await vscode.window.showInputBox({
          prompt: 'Enter new project name',
          value: project.name,
          validateInput: (value) => {
            if (!value || value.trim().length === 0) {
              return 'Project name cannot be empty';
            }
            return null;
          },
        });
        if (newName) {
          await knowledgeClient.updateProject(workspaceId, projectId, {
            name: newName.trim(),
          });
          vscode.window.showInformationMessage(`Project name updated to "${newName}"`);
          onMetadataChanged();
        }
        break;

      case 'Edit Description':
        const newDescription = await vscode.window.showInputBox({
          prompt: 'Enter new description',
          value: project.description || '',
        });
        if (newDescription !== undefined) {
          await knowledgeClient.updateProject(workspaceId, projectId, {
            description: newDescription || undefined,
          });
          vscode.window.showInformationMessage('Project description updated');
          onMetadataChanged();
        }
        break;

      case 'Change Nature':
        // MANDATORY: Use AI to suggest new nature
        await aiService.requireLLM();
        
        const existingProjectsForEdit = await knowledgeClient.listProjects(workspaceId);
        const existingProjectNamesForEdit = existingProjectsForEdit.map(p => p.name);
        
        const natureSuggestionEdit = await vscode.window.withProgress(
          {
            location: vscode.ProgressLocation.Notification,
            title: 'AI analyzing project...',
            cancellable: false,
          },
          async (progress) => {
            progress.report({ message: 'AI suggesting new nature...' });
            return await aiService.suggestProjectNature(
              project.name,
              project.description || undefined,
              existingProjectNamesForEdit
            );
          }
        );
        
        const natureItems = createNatureQuickPickItems();
        const currentNatureItem = natureItems.find(
          (item) => item.detail === project.nature
        );
        const aiSuggestedItemEdit = natureItems.find(
          (item) => item.detail === natureSuggestionEdit.nature
        );
        
        const quickPickEdit = vscode.window.createQuickPick<vscode.QuickPickItem>();
        quickPickEdit.items = natureItems;
        quickPickEdit.placeholder = `AI suggests: ${getNatureLabel(natureSuggestionEdit.nature)} (${(natureSuggestionEdit.confidence * 100).toFixed(0)}% confidence)`;
        quickPickEdit.matchOnDescription = true;
        quickPickEdit.matchOnDetail = true;
        if (aiSuggestedItemEdit) {
          quickPickEdit.selectedItems = [aiSuggestedItemEdit];
        } else if (currentNatureItem) {
          quickPickEdit.selectedItems = [currentNatureItem];
        }
        
        const naturePromiseEdit = new Promise<vscode.QuickPickItem | undefined>((resolve) => {
          quickPickEdit.onDidAccept(() => {
            resolve(quickPickEdit.selectedItems[0]);
            quickPickEdit.dispose();
          });
          quickPickEdit.onDidHide(() => {
            resolve(undefined);
            quickPickEdit.dispose();
          });
        });
        
        quickPickEdit.show();
        const selectedNature = await naturePromiseEdit;
        
        if (selectedNature) {
          const nature = (selectedNature as any).detail || natureSuggestionEdit.nature;
          
          // When nature changes, automatically update attributes based on AI suggestion
          let updatedAttributes = project.attributes ? JSON.parse(project.attributes) : {};
          
          // Merge AI-suggested attributes if available
          if (natureSuggestionEdit.suggestedAttributes) {
            updatedAttributes = {
              ...updatedAttributes,
              ...natureSuggestionEdit.suggestedAttributes,
            };
          }
          
          await knowledgeClient.updateProject(workspaceId, projectId, {
            nature,
            attributes: JSON.stringify(updatedAttributes),
          });
          vscode.window.showInformationMessage(
            `✓ Project nature changed to ${getNatureLabel(nature)}. Attributes automatically updated based on nature.`
          );
          onMetadataChanged();
        }
        break;

      case 'Edit Attributes':
        await editProjectAttributes(workspaceRoot, knowledgeClient, workspaceId, projectId, onMetadataChanged);
        break;
    }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    vscode.window.showErrorMessage(`Failed to edit project: ${errorMessage}`);
    console.error('[Cortex] editProjectCommand error:', error);
  }
}

/**
 * Edit project attributes
 */
async function editProjectAttributes(
  workspaceRoot: string,
  knowledgeClient: GrpcKnowledgeClient,
  workspaceId: string,
  projectId: string,
  onMetadataChanged: () => void
): Promise<void> {
  try {
    const project = await knowledgeClient.getProject(workspaceId, projectId);
    if (!project) {
      return;
    }

    const attributes = project.attributes ? JSON.parse(project.attributes) : {};

    // Select attribute category first, then specific attribute
    const categoryOptions = [
      {
        label: '$(circle-outline) PKM - Estado del Proyecto',
        description: 'Estado: planning, active, on-hold, completed, archived',
        category: 'pkm-status',
      },
      {
        label: '$(circle-outline) PMI - Complejidad',
        description: 'Complejidad: simple, moderate, complex',
        category: 'pmi-complexity',
      },
      {
        label: '$(circle-outline) PMI - Duración',
        description: 'Duración: short-term, medium-term, long-term',
        category: 'pmi-duration',
      },
      {
        label: '$(circle-outline) PMI - Alcance',
        description: 'Alcance: small, medium, large, enterprise',
        category: 'pmi-scope',
      },
      {
        label: '$(circle-outline) GTD - Tipo de Resultado',
        description: 'Tipo: unique project, continuous process, reference',
        category: 'gtd-result',
      },
      {
        label: '$(circle-outline) Ontologías - Estructura',
        description: 'Estructura: linear, hierarchical, network',
        category: 'ontology-structure',
      },
      {
        label: '$(circle-outline) General - Prioridad',
        description: 'Prioridad: low, medium, high, critical',
        category: 'general-priority',
      },
      {
        label: '$(circle-outline) General - Temporalidad',
        description: 'Temporalidad: temporary, ongoing',
        category: 'general-temporality',
      },
      {
        label: '$(circle-outline) General - Colaboración',
        description: 'Colaboración: individual, team, organization',
        category: 'general-collaboration',
      },
      {
        label: '$(circle-outline) General - Visibilidad',
        description: 'Visibilidad: private, shared, public',
        category: 'general-visibility',
      },
      {
        label: '$(circle-slash) Cancel',
        category: 'cancel',
      },
    ];

    const selectedCategory = await vscode.window.showQuickPick(categoryOptions, {
      placeHolder: 'Select attribute category to edit',
    });

    if (!selectedCategory || selectedCategory.category === 'cancel') {
      return;
    }

    let newValue: string | undefined;
    const category = selectedCategory.category;

    // Handle each category
    if (category === 'pkm-status') {
      const statusOptions = ['planning', 'active', 'on-hold', 'completed', 'archived'];
      const currentStatus = attributes.status || 'active';
      const selectedStatus = await vscode.window.showQuickPick(
        statusOptions.map((s) => ({
          label: s,
          picked: s === currentStatus,
        })),
        {
          placeHolder: 'Select status (PKM)',
        }
      );
      newValue = selectedStatus?.label;
      if (newValue) {
        attributes.status = newValue;
      }
    } else if (category === 'pmi-complexity') {
      const complexityOptions = ['simple', 'moderate', 'complex'];
      const currentComplexity = attributes.complexity || 'moderate';
      const selectedComplexity = await vscode.window.showQuickPick(
        complexityOptions.map((c) => ({
          label: c,
          picked: c === currentComplexity,
        })),
        {
          placeHolder: 'Select complexity (PMI)',
        }
      );
      newValue = selectedComplexity?.label;
      if (newValue) {
        attributes.complexity = newValue;
      }
    } else if (category === 'pmi-duration') {
      const durationOptions = ['short-term', 'medium-term', 'long-term'];
      const currentDuration = attributes.duration || 'medium-term';
      const selectedDuration = await vscode.window.showQuickPick(
        durationOptions.map((d) => ({
          label: d,
          picked: d === currentDuration,
        })),
        {
          placeHolder: 'Select duration (PMI)',
        }
      );
      newValue = selectedDuration?.label;
      if (newValue) {
        attributes.duration = newValue;
      }
    } else if (category === 'pmi-scope') {
      const scopeOptions = ['small', 'medium', 'large', 'enterprise'];
      const currentScope = attributes.scope || 'medium';
      const selectedScope = await vscode.window.showQuickPick(
        scopeOptions.map((s) => ({
          label: s,
          picked: s === currentScope,
        })),
        {
          placeHolder: 'Select scope (PMI)',
        }
      );
      newValue = selectedScope?.label;
      if (newValue) {
        attributes.scope = newValue;
      }
    } else if (category === 'gtd-result') {
      const resultTypeOptions = ['unique project', 'continuous process', 'reference'];
      const currentResultType = attributes.resultType || 'unique project';
      const selectedResultType = await vscode.window.showQuickPick(
        resultTypeOptions.map((r) => ({
          label: r,
          picked: r === currentResultType,
        })),
        {
          placeHolder: 'Select result type (GTD)',
        }
      );
      newValue = selectedResultType?.label;
      if (newValue) {
        attributes.resultType = newValue;
      }
    } else if (category === 'ontology-structure') {
      const structureOptions = ['linear', 'hierarchical', 'network'];
      const currentStructure = attributes.structure || 'linear';
      const selectedStructure = await vscode.window.showQuickPick(
        structureOptions.map((s) => ({
          label: s,
          picked: s === currentStructure,
        })),
        {
          placeHolder: 'Select structure (Ontologies)',
        }
      );
      newValue = selectedStructure?.label;
      if (newValue) {
        attributes.structure = newValue;
      }
    } else if (category === 'general-priority') {
      const priorityOptions = ['low', 'medium', 'high', 'critical'];
      const currentPriority = attributes.priority || 'medium';
      const selectedPriority = await vscode.window.showQuickPick(
        priorityOptions.map((p) => ({
          label: p,
          picked: p === currentPriority,
        })),
        {
          placeHolder: 'Select priority',
        }
      );
      newValue = selectedPriority?.label;
      if (newValue) {
        attributes.priority = newValue;
      }
    } else if (category === 'general-temporality') {
      const tempOptions = ['temporary', 'ongoing'];
      const currentTemp = attributes.temporality || 'temporary';
      const selectedTemp = await vscode.window.showQuickPick(
        tempOptions.map((t) => ({
          label: t,
          picked: t === currentTemp,
        })),
        {
          placeHolder: 'Select temporality',
        }
      );
      newValue = selectedTemp?.label;
      if (newValue) {
        attributes.temporality = newValue;
      }
    } else if (category === 'general-collaboration') {
      const collabOptions = ['individual', 'team', 'organization'];
      const currentCollab = attributes.collaboration || 'individual';
      const selectedCollab = await vscode.window.showQuickPick(
        collabOptions.map((c) => ({
          label: c,
          picked: c === currentCollab,
        })),
        {
          placeHolder: 'Select collaboration type',
        }
      );
      newValue = selectedCollab?.label;
      if (newValue) {
        attributes.collaboration = newValue;
      }
    } else if (category === 'general-visibility') {
      const visOptions = ['private', 'shared', 'public'];
      const currentVis = attributes.visibility || 'private';
      const selectedVis = await vscode.window.showQuickPick(
        visOptions.map((v) => ({
          label: v,
          picked: v === currentVis,
        })),
        {
          placeHolder: 'Select visibility',
        }
      );
      newValue = selectedVis?.label;
      if (newValue) {
        attributes.visibility = newValue;
      }
    }

    if (newValue) {
      await knowledgeClient.updateProject(workspaceId, projectId, {
        attributes: JSON.stringify(attributes),
      });
      vscode.window.showInformationMessage('Project attributes updated');
      onMetadataChanged();
    }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    vscode.window.showErrorMessage(`Failed to edit attributes: ${errorMessage}`);
    console.error('[Cortex] editProjectAttributes error:', error);
  }
}

