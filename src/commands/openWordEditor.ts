import * as vscode from "vscode";
import * as path from "node:path";
import { OnlyOfficeBridge } from "../services/OnlyOfficeBridge";
import { createOnlyOfficeWebviewHtml } from "../frontend/OnlyOfficeWordEditor";
import { ensureOnlyOfficeRunning } from "./startOnlyOffice";

interface WordEditorState {
  workspaceRoot: string;
  onlyOfficeBridge?: OnlyOfficeBridge;
}

export async function openWordEditorCommand(
  context: vscode.ExtensionContext,
  state: WordEditorState,
  uri?: vscode.Uri
): Promise<void> {
  void context;
  const config = vscode.workspace.getConfiguration("cortex");
  const documentServerUrl = config.get<string>(
    "onlyoffice.documentServerUrl",
    "http://localhost:8080"
  );
  const bridgePort = config.get<number>("onlyoffice.bridgePort", 7090);

  const targetUri = await resolveTargetUri(uri);
  if (!targetUri) {
    return;
  }

  if (path.extname(targetUri.fsPath).toLowerCase() !== ".docx") {
    vscode.window.showErrorMessage("Only .docx files are supported right now.");
    return;
  }
  const workspaceRoot = path.resolve(state.workspaceRoot);
  const targetPath = path.resolve(targetUri.fsPath);
  if (!targetPath.startsWith(workspaceRoot + path.sep)) {
    vscode.window.showErrorMessage("The Word editor only supports files inside the workspace.");
    return;
  }

  await ensureOnlyOfficeRunning(state.workspaceRoot, context.extensionPath, documentServerUrl);

  if (!state.onlyOfficeBridge || state.onlyOfficeBridge.runningPort !== bridgePort) {
    if (state.onlyOfficeBridge) {
      await state.onlyOfficeBridge.stop();
    }
    state.onlyOfficeBridge = new OnlyOfficeBridge(state.workspaceRoot, bridgePort);
  }

  try {
    await state.onlyOfficeBridge.start();
  } catch (error) {
    vscode.window.showErrorMessage(
      "Failed to start the ONLYOFFICE bridge server. Check the bridge port setting."
    );
    return;
  }

  const panel = vscode.window.createWebviewPanel(
    "cortex-onlyoffice",
    `Word: ${path.basename(targetUri.fsPath)}`,
    vscode.ViewColumn.Active,
    {
      enableScripts: true,
      retainContextWhenHidden: true
    }
  );

  panel.webview.html = createOnlyOfficeWebviewHtml(panel.webview, {
    documentServerUrl,
    bridgeUrl: state.onlyOfficeBridge.baseUrl,
    workspaceRoot: state.workspaceRoot,
    fileUri: targetUri
  });
}

async function resolveTargetUri(uri?: vscode.Uri): Promise<vscode.Uri | undefined> {
  if (uri) {
    return uri;
  }

  const editor = vscode.window.activeTextEditor;
  if (editor?.document?.uri) {
    return editor.document.uri;
  }

  const selection = await vscode.window.showOpenDialog({
    canSelectMany: false,
    openLabel: "Open Word Document",
    filters: { "Word Documents": ["docx"] }
  });

  return selection?.[0];
}
