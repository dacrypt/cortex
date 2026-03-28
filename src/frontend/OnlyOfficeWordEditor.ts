import * as vscode from "vscode";
import * as crypto from "node:crypto";
import * as path from "node:path";

interface OnlyOfficeWebviewOptions {
  documentServerUrl: string;
  bridgeUrl: string;
  workspaceRoot: string;
  fileUri: vscode.Uri;
}

export function createOnlyOfficeWebviewHtml(
  webview: vscode.Webview,
  options: OnlyOfficeWebviewOptions
): string {
  const nonce = crypto.randomBytes(16).toString("hex");
  const fileName = path.basename(options.fileUri.fsPath);
  const relativePath = path
    .relative(options.workspaceRoot, options.fileUri.fsPath)
    .replace(/\\/g, "/");

  const keySeed = `${options.fileUri.fsPath}:${Date.now()}`;
  const key = crypto.createHash("sha256").update(keySeed).digest("hex").slice(0, 32);

  const documentUrl = `${options.bridgeUrl}/doc?path=${encodeURIComponent(relativePath)}`;
  const callbackUrl = `${options.bridgeUrl}/callback?path=${encodeURIComponent(relativePath)}`;
  const docServer = options.documentServerUrl.replace(/\/+$/, "");
  const apiScript = `${docServer}/web-apps/apps/api/documents/api.js`;

  const csp = [
    "default-src 'none'",
    `img-src ${webview.cspSource} data: blob: ${docServer} ${options.bridgeUrl}`,
    `style-src ${webview.cspSource} 'unsafe-inline' ${docServer}`,
    `script-src ${webview.cspSource} 'nonce-${nonce}' ${docServer}`,
    `connect-src ${docServer} ${options.bridgeUrl} ws://localhost:* ws://127.0.0.1:* wss://localhost:* wss://127.0.0.1:*`,
    `frame-src ${docServer}`,
    `worker-src blob: ${docServer}`
  ].join("; ");

  const config = {
    document: {
      fileType: "docx",
      title: fileName,
      key,
      url: documentUrl
    },
    editorConfig: {
      callbackUrl,
      mode: "edit",
      user: {
        id: "local-user",
        name: "Local User"
      },
      customization: {
        compactHeader: false,
        chat: false,
        comments: true,
        help: true,
        toolbarNoTabs: false
      }
    },
    type: "desktop"
  };

  return `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="${csp}" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>${fileName}</title>
    <style>
      html, body, #editor-container {
        width: 100%;
        height: 100%;
        margin: 0;
        padding: 0;
        background: #ffffff;
      }
      #editor-container {
        display: flex;
        flex-direction: column;
      }
      #status-bar {
        height: 28px;
        display: flex;
        align-items: center;
        padding: 0 12px;
        font-family: var(--vscode-font-family);
        font-size: 12px;
        color: var(--vscode-foreground);
        background: var(--vscode-editor-background);
        border-bottom: 1px solid var(--vscode-panel-border);
      }
      #status-badge {
        padding: 2px 8px;
        border-radius: 12px;
        background: var(--vscode-badge-background);
        color: var(--vscode-badge-foreground);
      }
      #status-text {
        margin-left: 8px;
        color: var(--vscode-descriptionForeground);
      }
      #loading {
        margin: auto;
        font-family: var(--vscode-font-family);
        color: var(--vscode-foreground);
      }
      #onlyoffice-frame {
        flex: 1;
        min-height: 0;
        position: relative;
      }
      #loading-overlay {
        position: absolute;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        background: rgba(255, 255, 255, 0.7);
        font-family: var(--vscode-font-family);
        color: var(--vscode-foreground);
        font-size: 13px;
        z-index: 10;
      }
      #loading-spinner {
        width: 14px;
        height: 14px;
        border-radius: 50%;
        border: 2px solid var(--vscode-panel-border);
        border-top-color: var(--vscode-badge-foreground);
        margin-right: 8px;
        animation: spin 0.8s linear infinite;
      }
      #error-banner {
        position: absolute;
        top: 12px;
        left: 12px;
        right: 12px;
        padding: 10px 12px;
        border-radius: 6px;
        background: var(--vscode-inputValidation-errorBackground);
        color: var(--vscode-inputValidation-errorForeground);
        border: 1px solid var(--vscode-inputValidation-errorBorder);
        font-family: var(--vscode-font-family);
        font-size: 12px;
        display: none;
        z-index: 11;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
      }
      #error-banner button {
        background: var(--vscode-button-background);
        color: var(--vscode-button-foreground);
        border: none;
        border-radius: 4px;
        padding: 4px 10px;
        cursor: pointer;
        font-size: 12px;
      }
      #error-banner button:hover {
        background: var(--vscode-button-hoverBackground);
      }
      @keyframes spin {
        to { transform: rotate(360deg); }
      }
    </style>
  </head>
  <body>
    <div id="editor-container">
      <div id="status-bar">
        <span id="status-badge">Saved</span>
        <span id="status-text">Loading editor...</span>
      </div>
      <div id="onlyoffice-frame">
        <div id="error-banner"></div>
        <div id="loading-overlay">
          <div id="loading-spinner"></div>
          <div>Starting ONLYOFFICE…</div>
        </div>
        <div id="loading">Loading ONLYOFFICE editor...</div>
      </div>
    </div>
    <script nonce="${nonce}" src="${apiScript}"></script>
    <script nonce="${nonce}">
      const config = ${JSON.stringify(config)};
      const statusBadge = document.getElementById('status-badge');
      const statusText = document.getElementById('status-text');
      const loadingOverlay = document.getElementById('loading-overlay');
      const errorBanner = document.getElementById('error-banner');
      const healthUrl = ${JSON.stringify(`${docServer}/healthcheck`)};

      function setStatus(text, badgeText) {
        if (statusText) statusText.textContent = text;
        if (statusBadge && badgeText) statusBadge.textContent = badgeText;
      }

      function hideLoading() {
        if (loadingOverlay) loadingOverlay.style.display = 'none';
      }

      async function checkHealth() {
        try {
          const response = await fetch(healthUrl, { cache: 'no-store' });
          if (!response.ok) return false;
          const body = await response.text();
          return body.trim() === 'true';
        } catch (error) {
          return false;
        }
      }

      function showError(message) {
        if (!errorBanner) return;
        errorBanner.innerHTML = '';
        const messageSpan = document.createElement('span');
        messageSpan.textContent = message;
        const retryButton = document.createElement('button');
        retryButton.textContent = 'Retry';
        retryButton.addEventListener('click', async () => {
          retryButton.disabled = true;
          retryButton.textContent = 'Checking...';
          const healthy = await checkHealth();
          if (healthy) {
            location.reload();
            return;
          }
          retryButton.disabled = false;
          retryButton.textContent = 'Retry';
          messageSpan.textContent = 'DocumentServer is still starting. Please wait and retry.';
        });
        errorBanner.appendChild(messageSpan);
        errorBanner.appendChild(retryButton);
        errorBanner.style.display = 'flex';
      }

      setStatus('Loading editor...', 'Loading');
      config.events = {
        onAppReady: function() {
          setStatus('Editor ready', 'Ready');
        },
        onDocumentReady: function() {
          hideLoading();
          setStatus('Document ready', 'Saved');
        },
        onDocumentStateChange: function(event) {
          if (!statusBadge) return;
          statusBadge.textContent = event?.data ? 'Saving...' : 'Saved';
          if (statusText) {
            statusText.textContent = event?.data ? 'Saving changes...' : 'All changes saved';
          }
        },
        onError: function(event) {
          const message = event?.data?.message || 'Failed to load editor. Check DocumentServer.';
          showError(message);
          setStatus('Error', 'Error');
        }
      };
      const editor = new DocsAPI.DocEditor('onlyoffice-frame', config);
      window.onlyOfficeEditor = editor;
    </script>
  </body>
</html>`;
}
