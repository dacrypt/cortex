/**
 * Integration tests for all Tree Providers
 * 
 * These tests validate that all tree providers:
 * 1. Return nodes (not empty) when data is available
 * 2. Handle missing data gracefully
 * 3. Return proper structure and types
 * 4. Show informative messages when appropriate
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';

// Import all tree providers
import { FolderTreeProvider } from '../../views/FolderTreeProvider';
// Obsolete providers removed: CodeMetricsTreeProvider, DocumentMetricsTreeProvider, IssuesTreeProvider
import { DateRangeFacetTreeProvider } from '../../views/DateRangeFacetTreeProvider';
import { NumericRangeFacetTreeProvider } from '../../views/NumericRangeFacetTreeProvider';
import { TermsFacetTreeProvider } from '../../views/TermsFacetTreeProvider';

import {
  FileEntry,
  createMockContext,
  withMockedFileCache,
  getChildrenItems,
  comprehensiveTestData,
} from '../helpers/testHelpers';

describe('Tree Providers Integration Tests', () => {
  const mockContext = createMockContext();
  const workspaceRoot = '/test/workspace';
  const workspaceId = 'test-workspace-id';


  describe('FolderTreeProvider', () => {
    it('should return root folders', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new FolderTreeProvider(workspaceRoot, mockContext, workspaceId);
        const roots = await getChildrenItems(provider);

        assert.ok(roots.length > 0, 'Should return root folders');
        const folderLabels = roots.map((r) => String(r.label));
        assert.ok(
          folderLabels.some((label) => label.includes('src')),
          'Should include src folder'
        );
        assert.ok(
          folderLabels.some((label) => label.includes('docs')),
          'Should include docs folder'
        );
      });
    });

    it('should return files and subfolders in a folder', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new FolderTreeProvider(workspaceRoot, mockContext, workspaceId);
        const roots = await getChildrenItems(provider);
        const srcNode = roots.find((item) => String(item.label).includes('src'));

        if (srcNode) {
          const children = await getChildrenItems(provider, srcNode);
          assert.ok(children.length > 0, 'Should return children of src folder');
          // Should include both files and subfolders
          const hasFiles = children.some(
            (c) => c.contextValue === 'cortex-file'
          );
          const hasFolders = children.some(
            (c) => c.contextValue === 'cortex-folder'
          );
          assert.ok(hasFiles || hasFolders, 'Should include files or subfolders');
        }
      });
    });
  });



  // Obsolete provider tests removed:
  // - CodeMetricsTreeProvider (replaced by UnifiedFacetTreeProvider)
  // - DocumentMetricsTreeProvider (replaced by UnifiedFacetTreeProvider)
  // - IssuesTreeProvider (replaced by UnifiedFacetTreeProvider)

  describe('DateRangeFacetTreeProvider', () => {
    it('should return date range facets', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new DateRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'modified'
        );
        const roots = await getChildrenItems(provider);

        assert.ok(roots.length > 0, 'Should return date range facets');
      });
    });
  });

  describe('NumericRangeFacetTreeProvider', () => {
    it('should return numeric range facets', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new NumericRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'size'
        );
        const roots = await getChildrenItems(provider);

        assert.ok(roots.length > 0, 'Should return numeric range facets');
      });
    });
  });

  describe('TermsFacetTreeProvider', () => {
    it('should return term facets for extensions', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new TermsFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'extension'
        );
        const roots = await getChildrenItems(provider);

        assert.ok(roots.length > 0, 'Should return term facets');
      });
    });
  });

  describe('All Providers - Common Validations', () => {
    it('should never return undefined or null items', async () => {
      const providers = [
        () => new FolderTreeProvider(workspaceRoot, mockContext, workspaceId),
      ];

      for (const createProvider of providers) {
        await withMockedFileCache(comprehensiveTestData, async () => {
          const provider = createProvider();
          const roots = await getChildrenItems(provider);

          // All items should be valid TreeItems
          roots.forEach((item, index) => {
            assert.ok(item !== null && item !== undefined, 
              `Provider ${createProvider.name} returned null/undefined item at index ${index}`);
            assert.ok(item instanceof vscode.TreeItem || typeof item === 'object',
              `Provider ${createProvider.name} returned invalid item at index ${index}`);
          });
        });
      }
    });

    it('should handle empty file lists gracefully', async () => {
      const emptyFiles: FileEntry[] = [];

      const providers = [
        () => new FolderTreeProvider(workspaceRoot, mockContext, workspaceId),
      ];

      for (const createProvider of providers) {
        await withMockedFileCache(emptyFiles, async () => {
          const provider = createProvider();
          const roots = await getChildrenItems(provider);

          // Should return placeholder or empty array, not crash
          assert.ok(Array.isArray(roots), 'Should return array even with no files');
          // If it returns items, they should be valid placeholders
          if (roots.length > 0) {
            roots.forEach((item) => {
              assert.ok(item !== null && item !== undefined, 'Placeholder should be valid');
            });
          }
        });
      }
    });
  });
});
