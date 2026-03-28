import * as vscode from "vscode";
import { BackendFrontend } from "../frontend/BackendFrontend";

export function openBackendFrontendCommand(
  context: vscode.ExtensionContext
): vscode.Disposable {
  return vscode.commands.registerCommand("cortex.openBackendFrontend", () => {
    BackendFrontend.show(context);
  });
}


