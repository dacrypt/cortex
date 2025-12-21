/**
 * Ejemplo de integración del backend Go en la extensión
 * 
 * Este archivo muestra cómo usar GoBackend en comandos reales
 */

import * as vscode from 'vscode';
import { GoBackend } from '../core/GoBackend';

let backendInstance: GoBackend | null = null;

/**
 * Obtiene o crea la instancia del backend Go
 */
export function getBackend(context: vscode.ExtensionContext): GoBackend {
  if (!backendInstance) {
    backendInstance = new GoBackend(context, {
      binariesPath: 'bin',
      binaryName: 'cortex-backend',
      timeout: 30000,
    });

    // Manejar eventos del backend
    backendInstance.on('started', () => {
      console.log('[Cortex] Go backend started');
    });

    backendInstance.on('exited', (code, signal) => {
      console.log(`[Cortex] Go backend exited (code: ${code}, signal: ${signal})`);
    });

    backendInstance.on('stderr', (message) => {
      console.log(`[GoBackend] ${message}`);
    });
  }

  return backendInstance;
}

/**
 * Comando: Procesar archivo actual con backend Go
 */
export async function processFileWithGo(context: vscode.ExtensionContext): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showWarningMessage('No active editor');
    return;
  }

  const filePath = editor.document.uri.fsPath;
  const backend = getBackend(context);

  // Asegurar que el backend esté corriendo
  if (!backend.running) {
    try {
      await backend.start();
    } catch (error: any) {
      vscode.window.showErrorMessage(
        `Failed to start Go backend: ${error.message}\n\n` +
        `Make sure the backend binaries are compiled and included in the 'bin' directory.`
      );
      return;
    }
  }

  // Mostrar progreso
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Processing file with Go backend...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: 'Sending request to Go backend...' });

      try {
        const result = await backend.call('processFile', {
          filePath,
        });

        progress.report({ message: 'Processing complete' });

        // Mostrar resultado
        const resultText = JSON.stringify(result, null, 2);
        const doc = await vscode.workspace.openTextDocument({
          content: resultText,
          language: 'json',
        });
        await vscode.window.showTextDocument(doc);

        vscode.window.showInformationMessage(
          `File processed: ${result.filePath} (${result.size} bytes)`
        );
      } catch (error: any) {
        vscode.window.showErrorMessage(`Backend error: ${error.message}`);
      }
    }
  );
}

/**
 * Comando: Analizar código con backend Go
 */
export async function analyzeCodeWithGo(context: vscode.ExtensionContext): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showWarningMessage('No active editor');
    return;
  }

  const code = editor.document.getText();
  const backend = getBackend(context);

  if (!backend.running) {
    try {
      await backend.start();
    } catch (error: any) {
      vscode.window.showErrorMessage(`Failed to start Go backend: ${error.message}`);
      return;
    }
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Analyzing code with Go backend...',
      cancellable: false,
    },
    async (progress) => {
      try {
        const result = await backend.call('analyzeCode', {
          code,
        });

        const message = `Code analysis: ${result.lines} lines, ${result.characters} characters`;
        vscode.window.showInformationMessage(message);

        // Mostrar detalles en output channel
        const outputChannel = vscode.window.createOutputChannel('Go Backend Analysis');
        outputChannel.appendLine('Code Analysis Results:');
        outputChannel.appendLine(JSON.stringify(result, null, 2));
        outputChannel.show();
      } catch (error: any) {
        vscode.window.showErrorMessage(`Analysis error: ${error.message}`);
      }
    }
  );
}

/**
 * Inicializa el backend Go al activar la extensión
 */
export async function initializeGoBackend(
  context: vscode.ExtensionContext
): Promise<void> {
  const backend = getBackend(context);

  // Verificar si el binario existe
  const exists = await backend.checkBinaryExists();
  if (!exists) {
    console.warn(
      '[Cortex] Go backend binary not found. ' +
      'Run ./build-go.sh to compile binaries.'
    );
    return;
  }

  // Iniciar backend en background
  backend.start().catch((error) => {
    console.error('[Cortex] Failed to start Go backend:', error);
  });

  // Detener backend al desactivar
  context.subscriptions.push({
    dispose: async () => {
      if (backend.running) {
        await backend.stop();
      }
    },
  });
}


