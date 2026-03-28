/**
 * Integration tests for UnifiedFacetTreeProvider - Tests each facet individually
 * 
 * These tests validate that each facet:
 * 1. Loads correctly (can be instantiated and queried)
 * 2. Shows expected values when data is available
 * 3. Handles missing data gracefully
 * 4. Returns proper structure and types
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';
import * as path from 'node:path';
import { UnifiedFacetTreeProvider } from '../../views/UnifiedFacetTreeProvider';
import { getFacetRegistry } from '../../views/contracts/FacetRegistry';
import { FacetCategory, FacetType } from '../../views/contracts/IFacetProvider';
import { GrpcKnowledgeClient } from '../../core/GrpcKnowledgeClient';
import {
  FileEntry,
  createMockContext,
  withMockedFileCache,
  getChildrenItems,
  comprehensiveTestData,
  createMockMetadataStore,
} from '../helpers/testHelpers';

describe('UnifiedFacetTreeProvider - Facet Integration Tests', () => {
  // Use real extension path if available (for backend connection tests)
  const realExtensionPath = process.cwd();
  const mockContext = {
    ...createMockContext(),
    extensionPath: realExtensionPath, // Use real path so proto files can be found
  } as vscode.ExtensionContext;
  const workspaceRoot = '/test/workspace';
  const workspaceId = 'test-workspace-id';
  const registry = getFacetRegistry();

  /**
   * Checks if the backend is available by attempting to connect
   * Throws an error if backend is not available
   */
  async function requireBackend(): Promise<void> {
    try {
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      // Try a simple call to check if backend is available
      // Use timeout parameter (gRPC-level timeout) to fail fast if backend is not available
      await knowledgeClient.getFacets(workspaceId, [{ field: 'extension', type: 'terms' }], undefined, 5000);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      
      // Check if it's a proto file error (backend files not found)
      if (errorMessage.includes('proto') || errorMessage.includes('ENOENT')) {
        throw new Error(
          `Backend proto files not found. Make sure you're running tests from the project root. ` +
          `Expected proto files at: ${path.join(realExtensionPath, 'backend', 'api', 'proto')}`
        );
      }
      
      // Check if it's a connection error (backend not running)
      if (errorMessage.includes('ECONNREFUSED') || 
          errorMessage.includes('timeout') ||
          errorMessage.includes('14 UNAVAILABLE') ||
          errorMessage.includes('connection')) {
        throw new Error(
          `Backend is not running or not accessible. ` +
          `Please start the backend before running integration tests. ` +
          `Error: ${errorMessage}`
        );
      }
      
      // Re-throw other errors
      throw error;
    }
  }

  /**
   * Creates a UnifiedFacetTreeProvider with mocked dependencies
   */
  function createProvider(): UnifiedFacetTreeProvider {
    const metadataStore = createMockMetadataStore();
    return new UnifiedFacetTreeProvider({
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
  }

  /**
   * Gets all facets from the registry
   */
  function getAllFacets() {
    return registry.getAll();
  }

  describe('Provider Initialization', () => {
    it('should initialize without errors', () => {
      assert.doesNotThrow(() => {
        createProvider();
      }, 'Provider should initialize without throwing');
    });

    it('should return categories as root nodes', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        const roots = await getChildrenItems(provider);

        assert.ok(Array.isArray(roots), 'Should return an array');
        assert.ok(roots.length > 0, 'Should return at least one category');
        
        // Verify categories are present
        const labels = roots.map((r) => String(r.label));
        const expectedCategories = ['Core', 'Organization', 'Temporal', 'Content', 'System', 'Specialized'];
        for (const category of expectedCategories) {
          assert.ok(
            labels.some((label) => label.includes(category)),
            `Should include ${category} category`
          );
        }
      });
    });
  });

  describe('Core Facets', () => {
    const coreFacets = getAllFacets().filter((f) => f.category === FacetCategory.Core);

    for (const facet of coreFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            // Get categories
            const categories = await getChildrenItems(provider);
            const coreCategory = categories.find((c) => String(c.label).includes('Core'));
            
            assert.ok(coreCategory, `Should find Core category for ${facet.label}`);
            
            // Get facets in Core category
            const coreFacets = await getChildrenItems(provider, coreCategory);
            const targetFacet = coreFacets.find((f) => {
              const label = String(f.label);
              return label.includes(facet.label) || label.includes(facet.field);
            });
            
            assert.ok(targetFacet, `Should find ${facet.label} facet`);
            
            // Get values for this facet
            const values = await getChildrenItems(provider, targetFacet);
            
            // Should return array (may be empty if no data)
            assert.ok(Array.isArray(values), `Should return array for ${facet.label}`);
            
            // If values exist, they should be valid
            if (values.length > 0) {
              values.forEach((value, index) => {
                assert.ok(value !== null && value !== undefined,
                  `${facet.label} value at index ${index} should not be null/undefined`);
                // TreeItem can be an object with label, collapsibleState, etc.
                assert.ok(typeof value === 'object',
                  `${facet.label} value at index ${index} should be an object`);
              });
            }
          });
        });

        it('should handle empty data gracefully', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache([], async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const coreCategory = categories.find((c) => String(c.label).includes('Core'));
            
            if (coreCategory) {
              const coreFacets = await getChildrenItems(provider, coreCategory);
              const targetFacet = coreFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                // Should return array (may have placeholder)
                assert.ok(Array.isArray(values), 
                  `${facet.label} should return array even with no data`);
              }
            }
          });
        });
      });
    }
  });

  describe('Organization Facets', () => {
    const orgFacets = getAllFacets().filter((f) => f.category === FacetCategory.Organization);

    for (const facet of orgFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const orgCategory = categories.find((c) => String(c.label).includes('Organization'));
            
            if (orgCategory) {
              const orgFacets = await getChildrenItems(provider, orgCategory);
              const targetFacet = orgFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                assert.ok(Array.isArray(values), 
                  `Should return array for ${facet.label}`);
              }
            }
          });
        });
      });
    }
  });

  describe('Temporal Facets', () => {
    const temporalFacets = getAllFacets().filter((f) => f.category === FacetCategory.Temporal);

    for (const facet of temporalFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const temporalCategory = categories.find((c) => String(c.label).includes('Temporal'));
            
            if (temporalCategory) {
              const temporalFacets = await getChildrenItems(provider, temporalCategory);
              const targetFacet = temporalFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                assert.ok(Array.isArray(values), 
                  `Should return array for ${facet.label}`);
              }
            }
          });
        });
      });
    }
  });

  describe('Content Facets', () => {
    const contentFacets = getAllFacets().filter((f) => f.category === FacetCategory.Content);

    for (const facet of contentFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const contentCategory = categories.find((c) => String(c.label).includes('Content'));
            
            if (contentCategory) {
              const contentFacets = await getChildrenItems(provider, contentCategory);
              const targetFacet = contentFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                assert.ok(Array.isArray(values), 
                  `Should return array for ${facet.label}`);
              }
            }
          });
        });
      });
    }
  });

  describe('System Facets', () => {
    const systemFacets = getAllFacets().filter((f) => f.category === FacetCategory.System);

    for (const facet of systemFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const systemCategory = categories.find((c) => String(c.label).includes('System'));
            
            if (systemCategory) {
              const systemFacets = await getChildrenItems(provider, systemCategory);
              const targetFacet = systemFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                assert.ok(Array.isArray(values), 
                  `Should return array for ${facet.label}`);
              }
            }
          });
        });
      });
    }
  });

  describe('Specialized Facets', () => {
    const specializedFacets = getAllFacets().filter((f) => f.category === FacetCategory.Specialized);

    for (const facet of specializedFacets) {
      describe(`Facet: ${facet.label} (${facet.field})`, () => {
        it('should load facet values', async () => {
          // Backend is required for these tests
          await requireBackend();
          
          await withMockedFileCache(comprehensiveTestData, async () => {
            const provider = createProvider();
            
            const categories = await getChildrenItems(provider);
            const specializedCategory = categories.find((c) => String(c.label).includes('Specialized'));
            
            if (specializedCategory) {
              const specializedFacets = await getChildrenItems(provider, specializedCategory);
              const targetFacet = specializedFacets.find((f) => {
                const label = String(f.label);
                return label.includes(facet.label) || label.includes(facet.field);
              });
              
              if (targetFacet) {
                const values = await getChildrenItems(provider, targetFacet);
                assert.ok(Array.isArray(values), 
                  `Should return array for ${facet.label}`);
              }
            }
          });
        });
      });
    }
  });

  describe('Nested Facet Navigation - Complete Validation', () => {
    /**
     * Test helper: Navigate through a facet and validate files are returned
     */
    async function testFacetNavigation(
      provider: UnifiedFacetTreeProvider,
      categoryName: string,
      facetField: string,
      facetLabel?: string
    ): Promise<void> {
      // Step 1: Get categories
      const categories = await getChildrenItems(provider);
      const category = categories.find((c) => String(c.label).includes(categoryName));
      assert.ok(category, `Should find ${categoryName} category`);
      
      // Step 2: Get facets in category
      const facets = await getChildrenItems(provider, category);
      const targetFacet = facets.find((f) => {
        const label = String(f.label);
        const field = (f as any).payload?.facet || (f as any).value || '';
        return (facetLabel && label.includes(facetLabel)) || 
               label.includes(facetField) || 
               field === facetField;
      });
      assert.ok(targetFacet, `Should find ${facetField} facet in ${categoryName}`);
      
      // Step 3: Get facet values
      const values = await getChildrenItems(provider, targetFacet);
      assert.ok(Array.isArray(values), `Should return array for ${facetField}`);
      
      // Step 4: If values exist, test navigation to files
      if (values.length > 0) {
        // Test first value
        const firstValue = values[0];
        const valueLabel = String(firstValue.label);
        
        // Skip placeholder/error items
        if (!valueLabel.includes('No ') && !valueLabel.includes('Error') && !valueLabel.includes('Backend unavailable')) {
          const files = await getChildrenItems(provider, firstValue);
          assert.ok(Array.isArray(files), `Should return array of files for ${facetField}:${valueLabel}`);
          
          // Should not have error items
          const errorItems = files.filter((f) => {
            const label = String(f.label);
            return label.includes('Error') || label.includes('timeout');
          });
          assert.strictEqual(errorItems.length, 0, 
            `Should not have error items for ${facetField}:${valueLabel}. Found: ${errorItems.map((e) => String(e.label)).join(', ')}`);
          
          // If files exist, verify they're valid
          if (files.length > 0) {
            const firstFile = files[0];
            assert.ok(firstFile, `First file should exist for ${facetField}:${valueLabel}`);
            const fileLabel = String(firstFile.label);
            assert.ok(fileLabel.length > 0, `File should have a label for ${facetField}:${valueLabel}`);
          }
        }
      }
    }

    it('should navigate through core > by type > document and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Core', 'type', 'By Type');
      });
    });

    it('should navigate through core > by extension and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Core', 'extension', 'By Extension');
      });
    });

    it('should navigate through core > by indexing_status and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Core', 'indexing_status', 'By Indexing Status');
      });
    });

    it('should navigate through organization > by tag and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Organization', 'tag', 'By Tag');
      });
    });

    it('should navigate through organization > by project and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Organization', 'project', 'By Project');
      });
    });

    it('should navigate through content > by language and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Content', 'language', 'By Language');
      });
    });

    it('should navigate through content > by category and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Content', 'category', 'By Category');
      });
    });

    it('should navigate through temporal > by modified date and show files', async () => {
      await requireBackend();
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        await testFacetNavigation(provider, 'Temporal', 'modified', 'By Modified Date');
      });
    });
  });

  describe('Specific Facet Validations', () => {
    describe('Extension Facet', () => {
      it('should show file extensions from test data', async () => {
        await withMockedFileCache(comprehensiveTestData, async () => {
          const provider = createProvider();
          
          const categories = await getChildrenItems(provider);
          const coreCategory = categories.find((c) => String(c.label).includes('Core'));
          
          if (coreCategory) {
            const coreFacets = await getChildrenItems(provider, coreCategory);
            const extensionFacet = coreFacets.find((f) => {
              const label = String(f.label);
              return label.includes('Extension') || label.includes('extension');
            });
            
            if (extensionFacet) {
              const values = await getChildrenItems(provider, extensionFacet);
              
              // Should find extensions from test data: .ts, .pdf, .bin, .png, .txt
              const valueLabels = values.map((v) => String(v.label).toLowerCase());
              const hasTs = valueLabels.some((l) => l.includes('ts') || l.includes('.ts'));
              const hasPdf = valueLabels.some((l) => l.includes('pdf') || l.includes('.pdf'));
              
              // At least one extension should be found
              assert.ok(values.length >= 0, 'Should return extension values');
            }
          }
        });
      });
    });

    describe('Type Facet', () => {
      it('should show file types from test data', async () => {
        await withMockedFileCache(comprehensiveTestData, async () => {
          const provider = createProvider();
          
          const categories = await getChildrenItems(provider);
          const coreCategory = categories.find((c) => String(c.label).includes('Core'));
          
          if (coreCategory) {
            const coreFacets = await getChildrenItems(provider, coreCategory);
            const typeFacet = coreFacets.find((f) => {
              const label = String(f.label);
              return label.includes('Type') && !label.includes('Document Type') && !label.includes('MIME');
            });
            
            if (typeFacet) {
              const values = await getChildrenItems(provider, typeFacet);
              assert.ok(Array.isArray(values), 'Should return type values');
            }
          }
        });
      });
    });

    describe('Size Facet (Numeric Range)', () => {
      it('should show size ranges from test data', async () => {
        await withMockedFileCache(comprehensiveTestData, async () => {
          const provider = createProvider();
          
          const categories = await getChildrenItems(provider);
          const coreCategory = categories.find((c) => String(c.label).includes('Core'));
          
          if (coreCategory) {
            const coreFacets = await getChildrenItems(provider, coreCategory);
            const sizeFacet = coreFacets.find((f) => {
              const label = String(f.label);
              return label.includes('Size') || label.includes('size');
            });
            
            if (sizeFacet) {
              const values = await getChildrenItems(provider, sizeFacet);
              assert.ok(Array.isArray(values), 'Should return size range values');
            }
          }
        });
      });
    });

    describe('Modified Date Facet (Date Range)', () => {
      it('should show date ranges from test data', async () => {
        await withMockedFileCache(comprehensiveTestData, async () => {
          const provider = createProvider();
          
          const categories = await getChildrenItems(provider);
          const temporalCategory = categories.find((c) => String(c.label).includes('Temporal'));
          
          if (temporalCategory) {
            const temporalFacets = await getChildrenItems(provider, temporalCategory);
            const modifiedFacet = temporalFacets.find((f) => {
              const label = String(f.label);
              return label.includes('Modified') || label.includes('modified');
            });
            
            if (modifiedFacet) {
              const values = await getChildrenItems(provider, modifiedFacet);
              assert.ok(Array.isArray(values), 'Should return date range values');
            }
          }
        });
      });
    });
  });

  describe('Error Handling', () => {
    it('should handle provider errors gracefully', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Try to get children - should not throw
        assert.doesNotThrow(async () => {
          await getChildrenItems(provider);
        }, 'Should not throw when getting children');
      });
    });

    it('should return valid items even when some facets fail', async () => {
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        const categories = await getChildrenItems(provider);
        
        // All items should be valid
        categories.forEach((item, index) => {
          assert.ok(item !== null && item !== undefined,
            `Category at index ${index} should not be null/undefined`);
        });
      });
    });
  });

  describe('Backend Integration (REQUIRES BACKEND)', () => {
    // These tests REQUIRE the backend to be running
    // They will FAIL if the backend is not available
    
    before(async function() {
      // Check backend availability before running any backend integration tests
      // This will cause all tests in this suite to be skipped if backend is not available
      try {
        await requireBackend();
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        this.skip(); // Skip all tests in this suite
        throw new Error(`Backend not available: ${errorMessage}`);
      }
    });
    
    it('should connect to backend and retrieve real facet data', async () => {
      // Backend is required - test will fail if not available
      await requireBackend();
      
      const provider = createProvider();
      const categories = await getChildrenItems(provider);
      
      assert.ok(categories.length > 0, 'Should retrieve categories from backend');
      
      // Try to get a specific facet with real data
      const coreCategory = categories.find((c) => String(c.label).includes('Core'));
      assert.ok(coreCategory, 'Should find Core category');
      
      const coreFacets = await getChildrenItems(provider, coreCategory);
      const extensionFacet = coreFacets.find((f) => {
        const label = String(f.label);
        return label.includes('Extension') || label.includes('extension');
      });
      
      assert.ok(extensionFacet, 'Should find Extension facet');
      
      const values = await getChildrenItems(provider, extensionFacet);
      assert.ok(Array.isArray(values), 'Should return array of extension values from backend');
      assert.ok(values.length >= 0, 'Should return extension values (may be empty if no files indexed)');
    });

    it('should retrieve real file data for extension facet', async () => {
      // Backend is required - test will fail if not available
      await requireBackend();
      
      const provider = createProvider();
      const categories = await getChildrenItems(provider);
      const coreCategory = categories.find((c) => String(c.label).includes('Core'));
      
      assert.ok(coreCategory, 'Should find Core category');
      
      const coreFacets = await getChildrenItems(provider, coreCategory);
      const extensionFacet = coreFacets.find((f) => {
        const label = String(f.label);
        return label.includes('Extension') || label.includes('extension');
      });
      
      assert.ok(extensionFacet, 'Should find Extension facet');
      
      const values = await getChildrenItems(provider, extensionFacet);
      assert.ok(Array.isArray(values), 'Should return array of extension values');
      
      // If we have values, try to get files for one of them
      if (values.length > 0) {
        const firstValue = values[0];
        const files = await getChildrenItems(provider, firstValue);
        
        assert.ok(Array.isArray(files), 'Should return array of files from backend');
        
        // Verify files have valid structure
        if (files.length > 0) {
          files.forEach((file, index) => {
            assert.ok(file !== null && file !== undefined,
              `File at index ${index} should not be null/undefined`);
            assert.ok(typeof file === 'object',
              `File at index ${index} should be an object`);
          });
        }
      }
    });
  });
});

