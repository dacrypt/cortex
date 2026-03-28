/**
 * Integration tests for all view providers
 * 
 * These tests validate that each view provider:
 * 1. Initializes correctly
 * 2. Loads root items
 * 3. Handles navigation through tree structure
 * 4. Returns valid data without errors or timeouts
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';
import * as path from 'node:path';
import { UnifiedFacetTreeProvider } from '../../views/UnifiedFacetTreeProvider';
import { CortexTreeProvider } from '../../views/CortexTreeProvider';
import { FileInfoTreeProvider } from '../../views/FileInfoTreeProvider';
import { CategoryFacetTreeProvider } from '../../views/CategoryFacetTreeProvider';
import { MetricsFacetTreeProvider } from '../../views/MetricsFacetTreeProvider';
import { FolderTreeProvider } from '../../views/FolderTreeProvider';
import { TermsFacetTreeProvider } from '../../views/TermsFacetTreeProvider';
import { NumericRangeFacetTreeProvider } from '../../views/NumericRangeFacetTreeProvider';
import { DateRangeFacetTreeProvider } from '../../views/DateRangeFacetTreeProvider';
import { GrpcKnowledgeClient } from '../../core/GrpcKnowledgeClient';
import { GrpcAdminClient } from '../../core/GrpcAdminClient';
import { BackendMetadataStore } from '../../core/BackendMetadataStore';
import { GrpcMetadataClient } from '../../core/GrpcMetadataClient';
import {
  FileEntry,
  createMockContext,
  withMockedFileCache,
  getChildrenItems,
  comprehensiveTestData,
  createMockMetadataStore,
} from '../helpers/testHelpers';

describe('View Providers - Integration Tests', () => {
  const realExtensionPath = process.cwd();
  const mockContext = {
    ...createMockContext(),
    extensionPath: realExtensionPath,
  } as vscode.ExtensionContext;
  const workspaceRoot = '/test/workspace';
  const workspaceId = 'test-workspace-id';

  /**
   * Checks if the backend is available
   */
  async function requireBackend(): Promise<void> {
    try {
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      await knowledgeClient.getFacets(workspaceId, [{ field: 'extension', type: 'terms' }], undefined, 5000);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      if (errorMessage.includes('proto') || errorMessage.includes('ENOENT')) {
        throw new Error(
          `Backend proto files not found. Make sure you're running tests from the project root.`
        );
      }
      if (errorMessage.includes('ECONNREFUSED') || 
          errorMessage.includes('timeout') ||
          errorMessage.includes('14 UNAVAILABLE')) {
        throw new Error(
          `Backend is not running. Please start the backend before running integration tests.`
        );
      }
      throw error;
    }
  }

  describe('UnifiedFacetTreeProvider', () => {
    it('should initialize and load categories', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new UnifiedFacetTreeProvider({
          workspaceRoot,
          workspaceId,
          context: mockContext,
          fileCacheService: {
            setWorkspaceId: () => undefined,
            getFiles: async () => comprehensiveTestData,
          },
          knowledgeClient: undefined,
          adminClient: undefined,
          metadataStore,
        });

        const categories = await getChildrenItems(provider);
        assert.ok(Array.isArray(categories), 'Should return array of categories');
        assert.ok(categories.length > 0, 'Should have at least one category');
        
        const labels = categories.map((c) => String(c.label));
        const expectedCategories = ['Core', 'Organization', 'Temporal', 'Content', 'System', 'Specialized'];
        for (const expected of expectedCategories) {
          assert.ok(
            labels.some((label) => label.includes(expected)),
            `Should include ${expected} category`
          );
        }
      });
    });

    it('should navigate through facets and show files', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new UnifiedFacetTreeProvider({
          workspaceRoot,
          workspaceId,
          context: mockContext,
          fileCacheService: {
            setWorkspaceId: () => undefined,
            getFiles: async () => comprehensiveTestData,
          },
          knowledgeClient: undefined,
          adminClient: undefined,
          metadataStore,
        });

        // Navigate: Core > By Extension
        const categories = await getChildrenItems(provider);
        const coreCategory = categories.find((c) => String(c.label).includes('Core'));
        assert.ok(coreCategory, 'Should find Core category');

        const coreFacets = await getChildrenItems(provider, coreCategory);
        const extensionFacet = coreFacets.find((f) => {
          const label = String(f.label);
          return label.includes('Extension');
        });
        assert.ok(extensionFacet, 'Should find By Extension facet');

        const extensions = await getChildrenItems(provider, extensionFacet);
        assert.ok(Array.isArray(extensions), 'Should return array of extensions');
        
        // If extensions exist, test navigation to files
        if (extensions.length > 0) {
          const firstExt = extensions[0];
          const extLabel = String(firstExt.label);
          
          if (!extLabel.includes('No ') && !extLabel.includes('Error')) {
            const files = await getChildrenItems(provider, firstExt);
            assert.ok(Array.isArray(files), 'Should return array of files');
            
            // Should not have error items
            const errorItems = files.filter((f) => {
              const label = String(f.label);
              return label.includes('Error') || label.includes('timeout');
            });
            assert.strictEqual(errorItems.length, 0, 
              `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          }
        }
      });
    });
  });

  describe('CortexTreeProvider', () => {
    it('should initialize with sections', async () => {
      await requireBackend();
      
      const unifiedFacetProvider = new UnifiedFacetTreeProvider({
        workspaceRoot,
        workspaceId,
        context: mockContext,
        fileCacheService: {
          setWorkspaceId: () => undefined,
          getFiles: async () => comprehensiveTestData,
        },
        knowledgeClient: undefined,
        adminClient: undefined,
        metadataStore: createMockMetadataStore(),
      });

      const sections = [
        {
          id: 'facets',
          label: 'Facetas',
          icon: new vscode.ThemeIcon('list-filter'),
          initialState: vscode.TreeItemCollapsibleState.Expanded,
          provider: unifiedFacetProvider,
        },
      ];

      const provider = new CortexTreeProvider(sections);
      const rootItems = await getChildrenItems(provider);
      
      assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      assert.ok(rootItems.length > 0, 'Should have at least one root item');
      
      // Should have the facets section
      const facetsSection = rootItems.find((item) => {
        const label = String(item.label);
        return label.includes('Facetas') || label.includes('Facets');
      });
      assert.ok(facetsSection, 'Should have facets section');
    });

    it('should navigate through sections', async () => {
      await requireBackend();
      
      const unifiedFacetProvider = new UnifiedFacetTreeProvider({
        workspaceRoot,
        workspaceId,
        context: mockContext,
        fileCacheService: {
          setWorkspaceId: () => undefined,
          getFiles: async () => comprehensiveTestData,
        },
        knowledgeClient: undefined,
        adminClient: undefined,
        metadataStore: createMockMetadataStore(),
      });

      const sections = [
        {
          id: 'facets',
          label: 'Facetas',
          icon: new vscode.ThemeIcon('list-filter'),
          initialState: vscode.TreeItemCollapsibleState.Expanded,
          provider: unifiedFacetProvider,
        },
      ];

      const provider = new CortexTreeProvider(sections);
      const rootItems = await getChildrenItems(provider);
      
      if (rootItems.length > 0) {
        const firstSection = rootItems[0];
        const children = await getChildrenItems(provider, firstSection);
        assert.ok(Array.isArray(children), 'Should return array of children');
      }
    });
  });

  describe('FileInfoTreeProvider', () => {
    it('should initialize without errors', async () => {
      await requireBackend();
      
      const adminClient = new GrpcAdminClient(mockContext);
      const metadataClient = new GrpcMetadataClient(mockContext);
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      const metadataStore = new BackendMetadataStore(metadataClient, workspaceId);

      const provider = new FileInfoTreeProvider(
        workspaceRoot,
        metadataStore,
        metadataClient,
        adminClient,
        knowledgeClient,
        workspaceId
      );

      // FileInfoTreeProvider requires a file to be set via updateCurrentFile
      // Test that it initializes without errors
      const rootItems = await getChildrenItems(provider);
      assert.ok(Array.isArray(rootItems), 'Should return array of root items');
    });

    it('should handle file selection', async () => {
      await requireBackend();
      
      const adminClient = new GrpcAdminClient(mockContext);
      const metadataClient = new GrpcMetadataClient(mockContext);
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      const metadataStore = new BackendMetadataStore(metadataClient, workspaceId);

      const provider = new FileInfoTreeProvider(
        workspaceRoot,
        metadataStore,
        metadataClient,
        adminClient,
        knowledgeClient,
        workspaceId
      );

      // Set a test file using updateCurrentFile (expects relative path as string)
      const testFileRelativePath = 'test.ts';
      await provider.updateCurrentFile(testFileRelativePath);

      const rootItems = await getChildrenItems(provider);
      assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      
      // Should have file information sections
      if (rootItems.length > 0) {
        const labels = rootItems.map((item) => String(item.label));
        // Should have some file information
        assert.ok(labels.length > 0, 'Should have file information items');
      }
    });
  });

  describe('CategoryFacetTreeProvider', () => {
    it('should initialize and load category projects', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const knowledgeClient = new GrpcKnowledgeClient(mockContext);
        const metadataStore = createMockMetadataStore();

        const provider = new CategoryFacetTreeProvider(
          {
            field: 'writing_category',
            label: 'By Writing Category',
            type: 'terms' as any,
            category: 'organization' as any,
            description: 'Group projects by writing category',
          },
          {
            workspaceRoot,
            workspaceId,
            context: mockContext,
            fileCacheService: {
              setWorkspaceId: () => undefined,
              getFiles: async () => comprehensiveTestData,
            },
            knowledgeClient,
            adminClient: undefined,
            metadataStore: createMockMetadataStore(),
          }
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        
        // May be empty if no projects match the category
        // But should not throw errors
        if (rootItems.length > 0) {
          const firstItem = rootItems[0];
          const children = await getChildrenItems(provider, firstItem);
          assert.ok(Array.isArray(children), 'Should return array of children');
        }
      });
    });
  });

  describe('MetricsFacetTreeProvider', () => {
    it('should initialize and load metric categories', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();

        const provider = new MetricsFacetTreeProvider(
          {
            field: 'code_metrics',
            label: 'By Code Metrics',
            type: 'numeric_range' as any,
            category: 'content' as any,
            description: 'Group files by code metrics',
          },
          {
            workspaceRoot,
            workspaceId,
            context: mockContext,
            fileCacheService: {
              setWorkspaceId: () => undefined,
              getFiles: async () => comprehensiveTestData,
            },
            knowledgeClient: undefined,
            adminClient: undefined,
            metadataStore: createMockMetadataStore(),
          }
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        
        // Should have metric categories (complexity, lines_of_code, etc.)
        if (rootItems.length > 0) {
          const firstMetric = rootItems[0];
          const ranges = await getChildrenItems(provider, firstMetric);
          assert.ok(Array.isArray(ranges), 'Should return array of metric ranges');
          
          // If ranges exist, test navigation to files
          if (ranges.length > 0) {
            const firstRange = ranges[0];
            const files = await getChildrenItems(provider, firstRange);
            assert.ok(Array.isArray(files), 'Should return array of files');
            
            // Should not have error items
            const errorItems = files.filter((f) => {
              const label = String(f.label);
              return label.includes('Error') || label.includes('timeout');
            });
            assert.strictEqual(errorItems.length, 0, 
              `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          }
        }
      });
    });

    it('should handle document metrics', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();

        const provider = new MetricsFacetTreeProvider(
          {
            field: 'document_metrics',
            label: 'By Document Metrics',
            type: 'numeric_range' as any,
            category: 'content' as any,
            description: 'Group files by document metrics',
          },
          {
            workspaceRoot,
            workspaceId,
            context: mockContext,
            fileCacheService: {
              setWorkspaceId: () => undefined,
              getFiles: async () => comprehensiveTestData,
            },
            knowledgeClient: undefined,
            adminClient: undefined,
            metadataStore: createMockMetadataStore(),
          }
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      });
    });
  });

  describe('FolderTreeProvider', () => {
    it('should initialize and load root folders', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new FolderTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        
        // Should have folders or files
        if (rootItems.length > 0) {
          const firstItem = rootItems[0];
          const label = String(firstItem.label);
          assert.ok(label.length > 0, 'Item should have a label');
          
          // If it's a folder, test navigation
          if (firstItem.collapsibleState !== vscode.TreeItemCollapsibleState.None) {
            const children = await getChildrenItems(provider, firstItem);
            assert.ok(Array.isArray(children), 'Should return array of children');
          }
        }
      });
    });

    it('should navigate through folder hierarchy', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new FolderTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId
        );

        const rootItems = await getChildrenItems(provider);
        
        // Find a folder (not a file)
        const folder = rootItems.find((item) => 
          item.collapsibleState !== vscode.TreeItemCollapsibleState.None
        );
        
        if (folder) {
          const children = await getChildrenItems(provider, folder);
          assert.ok(Array.isArray(children), 'Should return array of children');
          
          // Should not have error items
          const errorItems = children.filter((item) => {
            const label = String(item.label);
            return label.includes('Error') || label.includes('timeout');
          });
          assert.strictEqual(errorItems.length, 0, 
            `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
        }
      });
    });
  });

  describe('TermsFacetTreeProvider', () => {
    it('should initialize and load extension facets', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new TermsFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'extension',
          metadataStore
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        assert.ok(rootItems.length > 0, 'Should have at least one extension');
        
        // Test navigation to files
        if (rootItems.length > 0) {
          const firstExt = rootItems[0];
          const extLabel = String(firstExt.label);
          
          if (!extLabel.includes('No ') && !extLabel.includes('Error')) {
            const files = await getChildrenItems(provider, firstExt);
            assert.ok(Array.isArray(files), 'Should return array of files');
            
            // Should not have error items
            const errorItems = files.filter((f) => {
              const label = String(f.label);
              return label.includes('Error') || label.includes('timeout');
            });
            assert.strictEqual(errorItems.length, 0, 
              `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          }
        }
      });
    });

    it('should handle type facets', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new TermsFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'type',
          metadataStore
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      });
    });

    it('should handle tag facets', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new TermsFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'tag',
          metadataStore
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      });
    });
  });

  describe('NumericRangeFacetTreeProvider', () => {
    it('should initialize and load size ranges', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new NumericRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'size'
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        assert.ok(rootItems.length > 0, 'Should have at least one size range');
        
        // Test navigation to files
        if (rootItems.length > 0) {
          const firstRange = rootItems[0];
          const rangeLabel = String(firstRange.label);
          
          if (!rangeLabel.includes('No ') && !rangeLabel.includes('Error')) {
            const files = await getChildrenItems(provider, firstRange);
            assert.ok(Array.isArray(files), 'Should return array of files');
            
            // Should not have error items
            const errorItems = files.filter((f) => {
              const label = String(f.label);
              return label.includes('Error') || label.includes('timeout');
            });
            assert.strictEqual(errorItems.length, 0, 
              `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          }
        }
      });
    });

    it('should handle complexity ranges', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new NumericRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'complexity'
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      });
    });
  });

  describe('DateRangeFacetTreeProvider', () => {
    it('should initialize and load date ranges', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new DateRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'modified'
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
        assert.ok(rootItems.length > 0, 'Should have at least one date range');
        
        // Test navigation to files
        if (rootItems.length > 0) {
          const firstRange = rootItems[0];
          const rangeLabel = String(firstRange.label);
          
          if (!rangeLabel.includes('No ') && !rangeLabel.includes('Error')) {
            const files = await getChildrenItems(provider, firstRange);
            assert.ok(Array.isArray(files), 'Should return array of files');
            
            // Should not have error items
            const errorItems = files.filter((f) => {
              const label = String(f.label);
              return label.includes('Error') || label.includes('timeout');
            });
            assert.strictEqual(errorItems.length, 0, 
              `Should not have error items. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          }
        }
      });
    });

    it('should handle created date ranges', async () => {
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = new DateRangeFacetTreeProvider(
          workspaceRoot,
          mockContext,
          workspaceId,
          'created'
        );

        const rootItems = await getChildrenItems(provider);
        assert.ok(Array.isArray(rootItems), 'Should return array of root items');
      });
    });
  });

  describe('View Provider Error Handling', () => {
    it('should handle backend unavailability gracefully', async () => {
      // Test without backend requirement
      await withMockedFileCache(comprehensiveTestData, async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new UnifiedFacetTreeProvider({
          workspaceRoot,
          workspaceId,
          context: mockContext,
          fileCacheService: {
            setWorkspaceId: () => undefined,
            getFiles: async () => comprehensiveTestData,
          },
          knowledgeClient: undefined,
          adminClient: undefined,
          metadataStore,
        });

        // Should not throw even if backend is unavailable
        assert.doesNotThrow(async () => {
          const categories = await getChildrenItems(provider);
          assert.ok(Array.isArray(categories), 'Should return array even without backend');
        });
      });
    });

    it('should handle empty data gracefully', async () => {
      await requireBackend();
      
      await withMockedFileCache([], async () => {
        const metadataStore = createMockMetadataStore();
        const provider = new UnifiedFacetTreeProvider({
          workspaceRoot,
          workspaceId,
          context: mockContext,
          fileCacheService: {
            setWorkspaceId: () => undefined,
            getFiles: async () => [],
          },
          knowledgeClient: undefined,
          adminClient: undefined,
          metadataStore,
        });

        const categories = await getChildrenItems(provider);
        assert.ok(Array.isArray(categories), 'Should return array even with no data');
      });
    });
  });
});
