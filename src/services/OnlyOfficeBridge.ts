import * as fs from "node:fs";
import * as http from "node:http";
import * as https from "node:https";
import * as path from "node:path";
import { URL } from "node:url";

interface OnlyOfficeCallbackPayload {
  status?: number;
  url?: string;
}

export class OnlyOfficeBridge {
  private server?: http.Server;
  private readonly workspaceRoot: string;
  private readonly port: number;

  constructor(workspaceRoot: string, port: number) {
    this.workspaceRoot = workspaceRoot;
    this.port = port;
  }

  get baseUrl(): string {
    return `http://127.0.0.1:${this.port}`;
  }

  get runningPort(): number {
    return this.port;
  }

  async start(): Promise<void> {
    if (this.server) {
      return;
    }

    this.server = http.createServer((req, res) => {
      const requestUrl = new URL(req.url || "/", this.baseUrl);
      const pathname = requestUrl.pathname;

      if (pathname === "/health") {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("ok");
        return;
      }

      if (pathname === "/doc" && req.method === "GET") {
        const filePath = requestUrl.searchParams.get("path");
        if (!filePath) {
          res.writeHead(400, { "Content-Type": "text/plain" });
          res.end("Missing path");
          return;
        }
        this.handleDocumentRequest(filePath, res);
        return;
      }

      if (pathname === "/callback" && req.method === "POST") {
        const filePath = requestUrl.searchParams.get("path");
        if (!filePath) {
          res.writeHead(400, { "Content-Type": "text/plain" });
          res.end("Missing path");
          return;
        }
        this.handleCallbackRequest(req, res, filePath);
        return;
      }

      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("Not found");
    });

    await new Promise<void>((resolve, reject) => {
      this.server?.listen(this.port, "127.0.0.1", () => resolve());
      this.server?.on("error", (error) => reject(error));
    });
  }

  async stop(): Promise<void> {
    if (!this.server) {
      return;
    }

    await new Promise<void>((resolve, reject) => {
      this.server?.close((error) => {
        if (error) {
          reject(error);
        } else {
          resolve();
        }
      });
    });

    this.server = undefined;
  }

  private handleDocumentRequest(filePath: string, res: http.ServerResponse): void {
    const absolutePath = this.resolveWorkspacePath(filePath);
    if (!absolutePath) {
      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("File not found");
      return;
    }

    const stat = fs.statSync(absolutePath);
    res.writeHead(200, {
      "Content-Type":
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "Content-Length": stat.size,
      "Access-Control-Allow-Origin": "*"
    });

    const stream = fs.createReadStream(absolutePath);
    stream.pipe(res);
    stream.on("error", () => {
      res.writeHead(500, { "Content-Type": "text/plain" });
      res.end("Failed to stream document");
    });
  }

  private async handleCallbackRequest(
    req: http.IncomingMessage,
    res: http.ServerResponse,
    filePath: string
  ): Promise<void> {
    const chunks: Buffer[] = [];
    req.on("data", (chunk) => {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
    });

    req.on("end", async () => {
      let payload: OnlyOfficeCallbackPayload = {};
      try {
        const body = Buffer.concat(chunks).toString("utf8").trim();
        payload = body ? JSON.parse(body) : {};
      } catch (error) {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: 1 }));
        return;
      }

      const status = payload.status ?? 0;
      if (status === 2 || status === 6) {
        const absolutePath = this.resolveWorkspacePath(filePath);
        if (!absolutePath) {
          res.writeHead(404, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: 1 }));
          return;
        }

        if (!payload.url) {
          res.writeHead(400, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: 1 }));
          return;
        }

        try {
          await this.downloadToFile(payload.url, absolutePath);
        } catch (error) {
          res.writeHead(500, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: 1 }));
          return;
        }
      }

      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: 0 }));
    });
  }

  private resolveWorkspacePath(requestedPath: string): string | undefined {
    const normalized = requestedPath.replace(/\\/g, "/");
    const absolutePath = path.resolve(this.workspaceRoot, normalized);
    const workspacePath = path.resolve(this.workspaceRoot);

    if (!absolutePath.startsWith(workspacePath + path.sep)) {
      return undefined;
    }

    if (!fs.existsSync(absolutePath)) {
      return undefined;
    }

    return absolutePath;
  }

  private async downloadToFile(fileUrl: string, destination: string): Promise<void> {
    const client = fileUrl.startsWith("https") ? https : http;
    const tempPath = `${destination}.onlyoffice.tmp`;

    await new Promise<void>((resolve, reject) => {
      const request = client.get(fileUrl, (response) => {
        if (response.statusCode && response.statusCode >= 400) {
          reject(new Error(`Download failed with status ${response.statusCode}`));
          return;
        }

        const fileStream = fs.createWriteStream(tempPath);
        response.pipe(fileStream);

        fileStream.on("finish", () => {
          fileStream.close((err) => {
            if (err) {
              reject(err);
              return;
            }
            fs.rename(tempPath, destination, (renameErr) => {
              if (renameErr) {
                reject(renameErr);
              } else {
                resolve();
              }
            });
          });
        });

        fileStream.on("error", (error) => reject(error));
      });

      request.on("error", (error) => reject(error));
    });
  }
}
