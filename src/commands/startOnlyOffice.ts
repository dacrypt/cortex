import * as vscode from "vscode";
import * as path from "node:path";
import * as fs from "node:fs";
import * as http from "node:http";
import * as https from "node:https";
import { spawn } from "node:child_process";

const HEALTH_TIMEOUT_MS = 12000;

export async function ensureOnlyOfficeRunning(
  workspaceRoot: string,
  extensionPath: string,
  documentServerUrl: string
): Promise<void> {
  const config = vscode.workspace.getConfiguration("cortex");
  const composeFile = config.get<string>(
    "onlyoffice.composeFile",
    "docker-compose.onlyoffice.yml"
  );
  const composePath = resolveComposePath(composeFile, workspaceRoot, extensionPath);
  if (!composePath) {
    vscode.window.showErrorMessage(
      "ONLYOFFICE compose file not found. Set cortex.onlyoffice.composeFile to a valid path."
    );
    return;
  }

  const healthy = await checkHealth(documentServerUrl);
  if (healthy) {
    return;
  }

  const result = await runDockerCompose(composePath, workspaceRoot);
  if (result.code !== 0) {
    vscode.window.showErrorMessage(
      `Failed to start ONLYOFFICE DocumentServer. ${result.stderr || ""}`.trim()
    );
    return;
  }

  const ready = await waitForHealth(documentServerUrl);
  if (!ready) {
    vscode.window.showErrorMessage(
      "ONLYOFFICE DocumentServer did not become ready. Check Docker logs."
    );
  }
}

function resolveComposePath(
  composeFile: string,
  workspaceRoot: string,
  extensionPath: string
): string | undefined {
  if (path.isAbsolute(composeFile)) {
    return fs.existsSync(composeFile) ? composeFile : undefined;
  }

  const workspaceCandidate = path.resolve(workspaceRoot, composeFile);
  if (fs.existsSync(workspaceCandidate)) {
    return workspaceCandidate;
  }

  const extensionCandidate = path.resolve(extensionPath, composeFile);
  if (fs.existsSync(extensionCandidate)) {
    return extensionCandidate;
  }

  return undefined;
}

function runDockerCompose(
  composePath: string,
  workspaceRoot: string
): Promise<{ code: number | null; stderr: string }> {
  return new Promise((resolve) => {
    const child = spawn(
      "docker",
      ["compose", "-f", composePath, "up", "-d"],
      { cwd: workspaceRoot }
    );

    let stderr = "";
    child.stderr.on("data", (data) => {
      stderr += data.toString();
    });

    child.on("close", (code) => {
      resolve({ code, stderr: stderr.trim() });
    });
  });
}

async function waitForHealth(documentServerUrl: string): Promise<boolean> {
  const start = Date.now();
  while (Date.now() - start < HEALTH_TIMEOUT_MS) {
    if (await checkHealth(documentServerUrl)) {
      return true;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  return false;
}

async function checkHealth(documentServerUrl: string): Promise<boolean> {
  const url = new URL("/healthcheck", documentServerUrl);
  const client = url.protocol === "https:" ? https : http;

  return new Promise((resolve) => {
    const request = client.get(url, (response) => {
      if (response.statusCode !== 200) {
        response.resume();
        resolve(false);
        return;
      }

      let body = "";
      response.setEncoding("utf8");
      response.on("data", (chunk) => {
        body += chunk;
      });
      response.on("end", () => {
        resolve(body.trim() === "true");
      });
    });

    request.on("error", () => resolve(false));
    request.setTimeout(3000, () => {
      request.destroy();
      resolve(false);
    });
  });
}
