/**
 * Commands for accepting/rejecting AI suggestions
 */

import * as vscode from 'vscode';
import * as path from 'path';
import { GrpcMetadataClient } from '../core/GrpcMetadataClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';

/**
 * Accept a suggested tag
 */
export async function acceptSuggestedTagCommand(
  workspaceRoot: string,
  metadataClient: GrpcMetadataClient,
  adminClient: GrpcAdminClient,
  workspaceId: string,
  tag: string,
  onMetadataChanged: () => void
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No file is currently open');
    return;
  }

  const absolutePath = editor.document.uri.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  try {
    // Get file ID
    const file = await adminClient.getFile(workspaceId, relativePath);
    if (!file?.file_id) {
      vscode.window.showErrorMessage('File not found in index');
      return;
    }

    // Accept the suggestion
    await metadataClient.acceptSuggestedTag(workspaceId, file.file_id, tag);

    vscode.window.showInformationMessage(`✓ Tag "${tag}" aceptado`);
    onMetadataChanged();
  } catch (error) {
    vscode.window.showErrorMessage(
      `Error al aceptar tag: ${error instanceof Error ? error.message : String(error)}`
    );
  }
}

/**
 * Accept a suggested project
 */
export async function acceptSuggestedProjectCommand(
  workspaceRoot: string,
  metadataClient: GrpcMetadataClient,
  adminClient: GrpcAdminClient,
  knowledgeClient: any, // GrpcKnowledgeClient
  workspaceId: string,
  projectName: string,
  onMetadataChanged: () => void
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No file is currently open');
    return;
  }

  const absolutePath = editor.document.uri.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  try {
    // Get file ID
    const file = await adminClient.getFile(workspaceId, relativePath);
    if (!file?.file_id) {
      vscode.window.showErrorMessage('File not found in index');
      return;
    }

    // Accept the suggestion
    await metadataClient.acceptSuggestedProject(workspaceId, file.file_id, projectName);

    // If it's a new project, create it
    let project = await knowledgeClient.getProjectByName(workspaceId, projectName);
    if (!project) {
      project = await knowledgeClient.createProjectWithNature(
        workspaceId,
        projectName,
        undefined,
        undefined,
        'generic'
      );
    }

    // Associate document with project
    const documentId = await knowledgeClient.getDocumentIdByPath(workspaceId, relativePath, adminClient);
    if (documentId) {
      await knowledgeClient.addDocumentToProject(workspaceId, project.id, documentId);
    }

    vscode.window.showInformationMessage(`✓ Proyecto "${projectName}" aceptado y asignado`);
    onMetadataChanged();
  } catch (error) {
    vscode.window.showErrorMessage(
      `Error al aceptar proyecto: ${error instanceof Error ? error.message : String(error)}`
    );
  }
}

/**
 * Reject a suggested tag
 */
export async function rejectSuggestedTagCommand(
  workspaceRoot: string,
  metadataClient: GrpcMetadataClient,
  adminClient: GrpcAdminClient,
  workspaceId: string,
  tag: string,
  onMetadataChanged: () => void
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No file is currently open');
    return;
  }

  const absolutePath = editor.document.uri.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  try {
    const file = await adminClient.getFile(workspaceId, relativePath);
    if (!file?.file_id) {
      vscode.window.showErrorMessage('File not found in index');
      return;
    }

    await metadataClient.rejectSuggestedTag(workspaceId, file.file_id, tag);
    vscode.window.showInformationMessage(`Tag "${tag}" rechazado`);
    onMetadataChanged();
  } catch (error) {
    vscode.window.showErrorMessage(
      `Error al rechazar tag: ${error instanceof Error ? error.message : String(error)}`
    );
  }
}

/**
 * Reject a suggested project
 */
export async function rejectSuggestedProjectCommand(
  workspaceRoot: string,
  metadataClient: GrpcMetadataClient,
  adminClient: GrpcAdminClient,
  workspaceId: string,
  projectName: string,
  onMetadataChanged: () => void
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No file is currently open');
    return;
  }

  const absolutePath = editor.document.uri.fsPath;
  const relativePath = path.relative(workspaceRoot, absolutePath);

  if (relativePath.startsWith('..')) {
    vscode.window.showErrorMessage('File is outside workspace');
    return;
  }

  try {
    const file = await adminClient.getFile(workspaceId, relativePath);
    if (!file?.file_id) {
      vscode.window.showErrorMessage('File not found in index');
      return;
    }

    await metadataClient.rejectSuggestedProject(workspaceId, file.file_id, projectName);
    vscode.window.showInformationMessage(`Proyecto "${projectName}" rechazado`);
    onMetadataChanged();
  } catch (error) {
    vscode.window.showErrorMessage(
      `Error al rechazar proyecto: ${error instanceof Error ? error.message : String(error)}`
    );
  }
}







