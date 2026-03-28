import * as vscode from "vscode";
import { GrpcRAGClient } from "../core/GrpcRAGClient";

export async function askAICommand(
  ragClient: GrpcRAGClient,
  workspaceId: string | undefined
) {
  if (!workspaceId) {
    vscode.window.showErrorMessage("Cortex: Workspace not registered with backend");
    return;
  }

  const query = await vscode.window.showInputBox({
    prompt: "Hacer una pregunta sobre tus documentos (RAG)",
    placeHolder: "¿Qué dice el manual sobre la configuración de seguridad?",
  });

  if (!query) {
    return;
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: "Cortex: Consultando al cerebro local...",
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: "Buscando en documentos..." });
      try {
        const response = await ragClient.query(workspaceId, query);
        
        if (!response.answer) {
          vscode.window.showInformationMessage("Cortex: No se encontró información relevante.");
          return;
        }

        // Show answer in a virtual document or just messages for now
        // A better UX is a virtual markdown document
        const report = `## Consulta Cortex
**Pregunta:** ${query}

---

### Respuesta
${response.answer}

---

### Fuentes Utilizadas
${response.sources.map((s, i) => `[${i + 1}] **${s.relativePath}** (${s.headingPath})\n> ${s.snippet}`).join('\n\n')}
`;

        const doc = await vscode.workspace.openTextDocument({
          content: report,
          language: "markdown",
        });
        await vscode.window.showTextDocument(doc, {
          viewColumn: vscode.ViewColumn.Beside,
          preserveFocus: true,
        });

      } catch (error) {
        vscode.window.showErrorMessage(
          `Cortex: Error en la consulta RAG: ${error instanceof Error ? error.message : String(error)}`
        );
      }
    }
  );
}
