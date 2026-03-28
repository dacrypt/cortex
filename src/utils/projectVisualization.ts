/**
 * Project Visualization utilities
 * Provides visual indicators, badges, and styling for projects based on their characteristics
 */

import * as vscode from 'vscode';
import { getNatureLabel, getNatureIcon } from './projectNature';

export interface ProjectAttributes {
  status?: string;
  priority?: string;
  complexity?: string;
  duration?: string;
  scope?: string;
  resultType?: string;
  structure?: string;
  temporality?: string;
  collaboration?: string;
  visibility?: string;
}

/**
 * Get visual badge/description for project based on attributes
 */
export function getProjectBadges(attributes: ProjectAttributes | null): string {
  if (!attributes) {
    return '';
  }

  const badges: string[] = [];

  // Status badge (PKM)
  if (attributes.status) {
    const statusEmoji: Record<string, string> = {
      planning: '📋',
      active: '🟢',
      'on-hold': '⏸️',
      completed: '✅',
      archived: '📦',
    };
    badges.push(`${statusEmoji[attributes.status] || '📋'} ${attributes.status}`);
  }

  // Priority badge
  if (attributes.priority) {
    const priorityEmoji: Record<string, string> = {
      low: '🔵',
      medium: '🟡',
      high: '🟠',
      critical: '🔴',
    };
    badges.push(`${priorityEmoji[attributes.priority] || '⚪'} ${attributes.priority}`);
  }

  // Collaboration badge
  if (attributes.collaboration) {
    const collabEmoji: Record<string, string> = {
      individual: '👤',
      team: '👥',
      organization: '🏢',
    };
    badges.push(`${collabEmoji[attributes.collaboration] || '👤'} ${attributes.collaboration}`);
  }

  return badges.join(' • ');
}

/**
 * Get icon for project based on nature and attributes
 */
export function getProjectIcon(
  nature: string | undefined,
  attributes: ProjectAttributes | null
): vscode.ThemeIcon {
  if (!nature || nature === 'generic') {
    return vscode.ThemeIcon.Folder;
  }

  // Convert codicon to theme icon name
  const natureIconStr = getNatureIcon(nature);
  const themeIconName = natureIconStr.replace(/^\$\(|\)$/g, '');
  
  // Use nature icon, fallback to folder
  try {
    return new vscode.ThemeIcon(themeIconName);
  } catch {
    return vscode.ThemeIcon.Folder;
  }
}

/**
 * Get color/decoration for project based on status and priority
 */
export function getProjectDecoration(
  attributes: ProjectAttributes | null
): { color?: vscode.ThemeColor; badge?: string } {
  if (!attributes) {
    return {};
  }

  // Color based on status
  if (attributes.status === 'active') {
    return { color: new vscode.ThemeColor('charts.green') };
  } else if (attributes.status === 'completed') {
    return { color: new vscode.ThemeColor('charts.blue') };
  } else if (attributes.status === 'on-hold') {
    return { color: new vscode.ThemeColor('charts.yellow') };
  } else if (attributes.status === 'archived') {
    return { color: new vscode.ThemeColor('charts.grey') };
  }

  // Color based on priority
  if (attributes.priority === 'critical') {
    return { color: new vscode.ThemeColor('charts.red') };
  } else if (attributes.priority === 'high') {
    return { color: new vscode.ThemeColor('charts.orange') };
  }

  return {};
}

/**
 * Format project label with visual indicators
 */
export function formatProjectLabel(
  projectName: string,
  fileCount: number,
  nature: string | undefined,
  attributes: ProjectAttributes | null
): string {
  let label = projectName;
  
  // Add nature indicator if not generic
  if (nature && nature !== 'generic') {
    const natureLabel = getNatureLabel(nature);
    label = `${label} • ${natureLabel}`;
  }
  
  // Add file count
  label = `${label} (${fileCount})`;
  
  return label;
}

/**
 * Get comprehensive tooltip with all project characteristics
 */
export function getProjectTooltip(
  projectName: string,
  description: string | undefined,
  nature: string | undefined,
  attributes: ProjectAttributes | null
): vscode.MarkdownString {
  const tooltip = new vscode.MarkdownString();
  tooltip.appendMarkdown(`### ${projectName}\n\n`);

  if (description) {
    tooltip.appendMarkdown(`**Descripción**: ${description}\n\n`);
  }

  if (nature && nature !== 'generic') {
    tooltip.appendMarkdown(`**Naturaleza**: ${getNatureLabel(nature)}\n\n`);
  }

  if (attributes) {
    tooltip.appendMarkdown(`---\n\n`);
    tooltip.appendMarkdown(`### Características\n\n`);

    // PKM: Estado
    if (attributes.status) {
      tooltip.appendMarkdown(`**Estado (PKM)**: ${attributes.status}\n`);
    }

    // PMI
    if (attributes.complexity) {
      tooltip.appendMarkdown(`**Complejidad (PMI)**: ${attributes.complexity}\n`);
    }
    if (attributes.duration) {
      tooltip.appendMarkdown(`**Duración (PMI)**: ${attributes.duration}\n`);
    }
    if (attributes.scope) {
      tooltip.appendMarkdown(`**Alcance (PMI)**: ${attributes.scope}\n`);
    }

    // GTD
    if (attributes.resultType) {
      tooltip.appendMarkdown(`**Tipo de Resultado (GTD)**: ${attributes.resultType}\n`);
    }

    // Ontologías
    if (attributes.structure) {
      tooltip.appendMarkdown(`**Estructura**: ${attributes.structure}\n`);
    }

    // General
    if (attributes.priority) {
      tooltip.appendMarkdown(`**Prioridad**: ${attributes.priority}\n`);
    }
    if (attributes.temporality) {
      tooltip.appendMarkdown(`**Temporalidad**: ${attributes.temporality}\n`);
    }
    if (attributes.collaboration) {
      tooltip.appendMarkdown(`**Colaboración**: ${attributes.collaboration}\n`);
    }
    if (attributes.visibility) {
      tooltip.appendMarkdown(`**Visibilidad**: ${attributes.visibility}\n`);
    }
  }

  return tooltip;
}

/**
 * Parse project attributes from JSON string
 */
export function parseProjectAttributes(attributesStr: string | undefined): ProjectAttributes | null {
  if (!attributesStr) {
    return null;
  }

  try {
    return JSON.parse(attributesStr) as ProjectAttributes;
  } catch {
    return null;
  }
}







