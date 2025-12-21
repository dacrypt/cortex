import * as assert from 'assert';
import * as path from 'path';
import * as vscode from 'vscode';
import { addTagCommand } from '../../commands/addTag';
import { assignContextCommand } from '../../commands/assignContext';
import { openViewCommand } from '../../commands/openView';
import { rebuildIndexCommand } from '../../commands/rebuildIndex';
import { IndexStore } from '../../core/IndexStore';
import { MetadataStoreJSON } from '../../core/MetadataStoreJSON';
import { FileScanner } from '../../core/FileScanner';

describe('commands', () => {
  const fixtureRelative = path.join('src', 'sample.ts');

  async function openFixture(workspaceRoot: string): Promise<string> {
    const absolutePath = path.join(workspaceRoot, fixtureRelative);
    const doc = await vscode.workspace.openTextDocument(absolutePath);
    await vscode.window.showTextDocument(doc);
    return absolutePath;
  }

  it('addTagCommand adds a tag to the active file', async () => {
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
    assert.ok(workspaceRoot);

    process.env.CORTEX_DISABLE_OS_TAGS = '1';

    const metadataStore = new MetadataStoreJSON(workspaceRoot);
    await metadataStore.initialize();
    const indexStore = new IndexStore();

    const absolutePath = await openFixture(workspaceRoot);
    const relativePath = path.relative(workspaceRoot, absolutePath);
    indexStore.buildIndex([
      {
        absolutePath,
        relativePath,
        filename: path.basename(relativePath),
        extension: path.extname(relativePath),
        lastModified: Date.now(),
        fileSize: 10,
      },
    ]);

    const originalInputBox = vscode.window.showInputBox;
    const originalInfo = vscode.window.showInformationMessage;
    const originalWarn = vscode.window.showWarningMessage;

    (vscode.window as any).showInputBox = async () => 'Important';
    (vscode.window as any).showInformationMessage = async () => undefined;
    (vscode.window as any).showWarningMessage = async () => undefined;

    await addTagCommand(
      workspaceRoot,
      metadataStore,
      indexStore,
      () => undefined
    );

    const tags = metadataStore.getFilesByTag('important');
    assert.ok(tags.includes(relativePath));

    (vscode.window as any).showInputBox = originalInputBox;
    (vscode.window as any).showInformationMessage = originalInfo;
    (vscode.window as any).showWarningMessage = originalWarn;
    delete process.env.CORTEX_DISABLE_OS_TAGS;
  });

  it('assignContextCommand assigns a context', async () => {
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
    assert.ok(workspaceRoot);

    const metadataStore = new MetadataStoreJSON(workspaceRoot);
    await metadataStore.initialize();
    const indexStore = new IndexStore();

    const absolutePath = await openFixture(workspaceRoot);
    const relativePath = path.relative(workspaceRoot, absolutePath);
    indexStore.buildIndex([
      {
        absolutePath,
        relativePath,
        filename: path.basename(relativePath),
        extension: path.extname(relativePath),
        lastModified: Date.now(),
        fileSize: 10,
      },
    ]);

    const originalInputBox = vscode.window.showInputBox;
    const originalInfo = vscode.window.showInformationMessage;
    (vscode.window as any).showInputBox = async () => 'Project';
    (vscode.window as any).showInformationMessage = async () => undefined;

    await assignContextCommand(
      workspaceRoot,
      metadataStore,
      indexStore,
      () => undefined
    );

    const contexts = metadataStore.getFilesByContext('project');
    assert.ok(contexts.includes(relativePath));

    (vscode.window as any).showInputBox = originalInputBox;
    (vscode.window as any).showInformationMessage = originalInfo;
  });

  it('openViewCommand triggers focus command', async () => {
    const originalExecute = vscode.commands.executeCommand;
    let called = '';
    (vscode.commands as any).executeCommand = async (command: string) => {
      called = command;
      return undefined;
    };

    await openViewCommand();
    assert.strictEqual(called, 'cortex-contextView.focus');

    (vscode.commands as any).executeCommand = originalExecute;
  });

  it('rebuildIndexCommand scans workspace and populates metadata', async () => {
    const workspaceRoot =
      vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
    assert.ok(workspaceRoot);

    const metadataStore = new MetadataStoreJSON(workspaceRoot);
    await metadataStore.initialize();
    const indexStore = new IndexStore();
    const fileScanner = new FileScanner(workspaceRoot);

    const originalWithProgress = vscode.window.withProgress;
    const originalInfo = vscode.window.showInformationMessage;
    (vscode.window as any).withProgress = async (
      _options: any,
      task: any
    ) => {
      return task({ report: () => undefined });
    };
    (vscode.window as any).showInformationMessage = async () => undefined;

    await rebuildIndexCommand(
      workspaceRoot,
      fileScanner,
      indexStore,
      metadataStore,
      () => undefined
    );

    assert.ok(indexStore.getAllFiles().length > 0);
    const sampleMeta = metadataStore.getMetadataByPath(fixtureRelative);
    assert.ok(sampleMeta);

    (vscode.window as any).withProgress = originalWithProgress;
    (vscode.window as any).showInformationMessage = originalInfo;
  });
});
