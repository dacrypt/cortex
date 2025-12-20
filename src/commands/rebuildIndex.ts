/**
 * Command: Rebuild Index
 */

import * as vscode from 'vscode';
import { FileScanner } from '../core/FileScanner';
import { IndexStore } from '../core/IndexStore';
import { IMetadataStore } from '../core/IMetadataStore';

export async function rebuildIndexCommand(
  workspaceRoot: string,
  fileScanner: FileScanner,
  indexStore: IndexStore,
  metadataStore: IMetadataStore,
  onIndexChanged: () => void
): Promise<void> {
  // Show progress indicator
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Cortex: Rebuilding index...',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ increment: 0 });

      // Scan workspace
      progress.report({ increment: 30, message: 'Scanning workspace...' });
      const files = await fileScanner.scanWorkspace();

      // Rebuild in-memory index
      progress.report({ increment: 30, message: 'Building index...' });
      indexStore.buildIndex(files);

      // Ensure metadata exists for all indexed files
      progress.report({ increment: 30, message: 'Updating metadata...' });
      for (const file of files) {
        metadataStore.getOrCreateMetadata(file.relativePath, file.extension);
      }

      progress.report({ increment: 10, message: 'Refreshing views...' });
      onIndexChanged();

      return Promise.resolve();
    }
  );

  const stats = indexStore.getStats();
  vscode.window.showInformationMessage(
    `Cortex: Index rebuilt (${stats.totalFiles} files)`
  );
}
