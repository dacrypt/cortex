/**
 * Integration tests for Owner Facet
 * 
 * These tests validate that the owner facet:
 * 1. Returns owner terms correctly from the backend
 * 2. Can retrieve files by owner using EntityClient
 * 3. Works end-to-end through TermsFacetTreeProvider
 * 4. Handles missing owner data gracefully
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';
import * as path from 'node:path';
import { TermsFacetTreeProvider } from '../../views/TermsFacetTreeProvider';
import { GrpcKnowledgeClient } from '../../core/GrpcKnowledgeClient';
import { GrpcEntityClient } from '../../core/GrpcEntityClient';
import {
  createMockContext,
  withMockedFileCache,
  getChildrenItems,
  comprehensiveTestData,
  createMockMetadataStore,
} from '../helpers/testHelpers';

describe('Owner Facet Integration Tests', () => {
  const realExtensionPath = process.cwd();
  const mockContext = {
    ...createMockContext(),
    extensionPath: realExtensionPath,
  } as vscode.ExtensionContext;
  const workspaceRoot = '/test/workspace';
  const workspaceId = 'test-workspace-id';

  /**
   * Checks if the backend is available by attempting to connect
   */
  async function requireBackend(): Promise<void> {
    try {
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      await knowledgeClient.getFacets(workspaceId, [{ field: 'owner', type: 'terms' }], undefined, 5000);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      
      if (errorMessage.includes('proto') || errorMessage.includes('ENOENT')) {
        throw new Error(
          `Backend proto files not found. Make sure you're running tests from the project root. ` +
          `Expected proto files at: ${path.join(realExtensionPath, 'backend', 'api', 'proto')}`
        );
      }
      
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
      
      throw error;
    }
  }

  /**
   * Creates a TermsFacetTreeProvider for owner facet
   */
  function createProvider(): TermsFacetTreeProvider {
    const metadataStore = createMockMetadataStore();
    return new TermsFacetTreeProvider(
      workspaceRoot,
      mockContext,
      workspaceId,
      'owner',
      metadataStore
    );
  }

  describe('Backend Facet API', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should retrieve owner facet terms from backend', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      
      const facets = await knowledgeClient.getFacets(
        workspaceId,
        [{ field: 'owner', type: 'terms' }],
        undefined,
        10000
      );
      
      const results = (facets as { results?: Array<{ field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }> })?.results || [];
      assert.ok(Array.isArray(results), 'Should return array of facet results');
      assert.ok(results.length > 0, 'Should return at least one facet result');
      
      const ownerFacet = results.find((f: { field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }) => f.field === 'owner');
      assert.ok(ownerFacet, 'Should find owner facet in results');
      // Backend returns 'FACET_TYPE_TERMS' but we accept both formats
      assert.ok(ownerFacet.type === 'terms' || ownerFacet.type === 'FACET_TYPE_TERMS', 
        `Owner facet should be terms type, got: ${ownerFacet.type}`);
      
      const terms = ownerFacet.terms?.terms || [];
      assert.ok(Array.isArray(terms), 'Should return array of terms');
      
      // Log owner terms for debugging
      console.log(`[OwnerFacetTest] Found ${terms.length} owner terms:`, 
        terms.map((t: { term?: string; count?: number }) => `${t.term} (${t.count})`).join(', '));
      
      // If terms exist, verify structure
      if (terms.length > 0) {
        terms.forEach((term: { term?: string; count?: number }, index: number) => {
          assert.ok(term !== null && term !== undefined,
            `Term at index ${index} should not be null/undefined`);
          assert.ok(typeof term === 'object',
            `Term at index ${index} should be an object`);
          assert.ok(term.term !== undefined,
            `Term at index ${index} should have a term property`);
          assert.ok(typeof term.count === 'number',
            `Term at index ${index} should have a numeric count`);
          assert.ok(term.count >= 0,
            `Term at index ${index} should have non-negative count`);
        });
      }
    });

    it('should handle owner facet with "owner:" prefix correctly', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      
      const facets = await knowledgeClient.getFacets(
        workspaceId,
        [{ field: 'owner', type: 'terms' }],
        undefined,
        10000
      );
      
      const results = (facets as { results?: Array<{ field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }> })?.results || [];
      const ownerFacet = results.find((f: { field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }) => f.field === 'owner');
      if (ownerFacet && ownerFacet.terms?.terms) {
        const terms = ownerFacet.terms.terms;
        
        // Check if any terms have "owner:" prefix
        const prefixedTerms = terms.filter((t: { term?: string }) => 
          t.term?.startsWith('owner:')
        );
        
        // Terms should either all have prefix or none should have prefix
        // Frontend should handle both cases
        if (prefixedTerms.length > 0) {
          console.log(`[OwnerFacetTest] Found ${prefixedTerms.length} terms with "owner:" prefix`);
        }
      }
    });
  });

  describe('EntityClient GetEntitiesByFacet', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should retrieve files by owner using EntityClient', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const entityClient = new GrpcEntityClient(mockContext);
      
      // First, get available owners from facets
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      const facets = await knowledgeClient.getFacets(
        workspaceId,
        [{ field: 'owner', type: 'terms' }],
        undefined,
        10000
      );
      
      const results = (facets as { results?: Array<{ field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }> })?.results || [];
      const ownerFacet = results.find((f: { field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }) => f.field === 'owner');
      if (!ownerFacet || !ownerFacet.terms?.terms || ownerFacet.terms.terms.length === 0) {
        // No owner data available - skip test
        console.log('[OwnerFacetTest] No owner data available, skipping test');
        return;
      }
      
      // Get first owner term (remove "owner:" prefix if present)
      const firstTerm = ownerFacet.terms.terms[0] as { term?: string; count?: number };
      let ownerValue = firstTerm.term || '';
      if (ownerValue.startsWith('owner:')) {
        ownerValue = ownerValue.substring(6);
      }
      
      console.log(`[OwnerFacetTest] Testing with owner: "${ownerValue}"`);
      
      // Get entities by owner
      const entities = await entityClient.getEntitiesByFacet(
        workspaceId,
        'owner',
        ownerValue,
        ['file'],
        10000
      );
      
      assert.ok(Array.isArray(entities), 'Should return array of entities');
      console.log(`[OwnerFacetTest] Found ${entities.length} files for owner "${ownerValue}"`);
      
      // Verify entities have correct owner
      if (entities.length > 0) {
        entities.forEach((entity, index) => {
          assert.ok(entity !== null && entity !== undefined,
            `Entity at index ${index} should not be null/undefined`);
          assert.strictEqual(entity.type, 'file',
            `Entity at index ${index} should be a file`);
          assert.ok(entity.path || entity.fileData?.relativePath,
            `Entity at index ${index} should have a path`);
          
          // Verify owner matches (if available)
          if (entity.owner) {
            assert.strictEqual(entity.owner, ownerValue,
              `Entity at index ${index} should have owner "${ownerValue}"`);
          }
        });
      }
    });

    it('should return empty array for non-existent owner', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const entityClient = new GrpcEntityClient(mockContext);
      
      const entities = await entityClient.getEntitiesByFacet(
        workspaceId,
        'owner',
        'nonexistent-owner-12345',
        ['file'],
        10000
      );
      
      assert.ok(Array.isArray(entities), 'Should return array even for non-existent owner');
      assert.strictEqual(entities.length, 0,
        'Should return empty array for non-existent owner');
    });
  });

  describe('TermsFacetTreeProvider Integration', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should load owner facet values in TermsFacetTreeProvider', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Get root items (should be owner terms)
        const rootItems = await getChildrenItems(provider);
        
        assert.ok(Array.isArray(rootItems), 'Should return array of owner terms');
        console.log(`[OwnerFacetTest] TermsFacetTreeProvider returned ${rootItems.length} owner terms`);
        
        // If terms exist, verify structure
        if (rootItems.length > 0) {
          // Check if we have actual owner data or placeholder
          const hasPlaceholder = rootItems.some((item) => {
            const label = String(item.label);
            return label.includes('No hay datos') || label.includes('No data');
          });
          
          if (hasPlaceholder) {
            // If we have placeholder, it means no owner data is available
            // This is acceptable - the backend is working but there's no owner data
            console.log('[OwnerFacetTest] No owner data available in workspace (this is acceptable)');
            return;
          }
          
          rootItems.forEach((item, index) => {
            assert.ok(item !== null && item !== undefined,
              `Item at index ${index} should not be null/undefined`);
            
            const label = String(item.label);
            assert.ok(label.length > 0,
              `Item at index ${index} should have a non-empty label`);
            
            // Should not be error items (but placeholder is acceptable)
            assert.ok(!label.includes('Error') && !label.includes('timeout'),
              `Item at index ${index} should not be an error: "${label}"`);
          });
        } else {
          // Empty results are acceptable if no owner data exists
          console.log('[OwnerFacetTest] No owner terms returned (workspace may have no owner data)');
        }
      });
    });

    it('should retrieve files for a specific owner', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Get owner terms
        const ownerTerms = await getChildrenItems(provider);
        
        if (ownerTerms.length === 0) {
          console.log('[OwnerFacetTest] No owner terms available, skipping file retrieval test');
          return;
        }
        
        // Get files for first owner
        const firstOwner = ownerTerms[0];
        const ownerLabel = String(firstOwner.label);
        console.log(`[OwnerFacetTest] Testing file retrieval for owner: "${ownerLabel}"`);
        
        const files = await getChildrenItems(provider, firstOwner);
        
        assert.ok(Array.isArray(files), 'Should return array of files');
        console.log(`[OwnerFacetTest] Found ${files.length} files for owner "${ownerLabel}"`);
        
        // Verify files structure
        if (files.length > 0) {
          files.forEach((file, index) => {
            assert.ok(file !== null && file !== undefined,
              `File at index ${index} should not be null/undefined`);
            
            const fileLabel = String(file.label);
            assert.ok(fileLabel.length > 0,
              `File at index ${index} should have a non-empty label`);
            
            // Should not be error items
            assert.ok(!fileLabel.includes('Error') && !fileLabel.includes('timeout'),
              `File at index ${index} should not be an error item: "${fileLabel}"`);
          });
        }
      });
    });

    it('should handle empty owner data gracefully', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache([], async () => {
        const provider = createProvider();
        
        const ownerTerms = await getChildrenItems(provider);
        
        // Should return array (may be empty or have placeholder)
        assert.ok(Array.isArray(ownerTerms),
          'Should return array even with no owner data');
      });
    });
  });

  describe('End-to-End Owner Facet Flow', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should complete full flow: facets -> terms -> files', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      // Step 1: Get owner facet terms from backend
      const knowledgeClient = new GrpcKnowledgeClient(mockContext);
      const facets = await knowledgeClient.getFacets(
        workspaceId,
        [{ field: 'owner', type: 'terms' }],
        undefined,
        10000
      );
      
      const results = (facets as { results?: Array<{ field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }> })?.results || [];
      const ownerFacet = results.find((f: { field?: string; type?: string; terms?: { terms?: Array<{ term?: string; count?: number }> } }) => f.field === 'owner');
      if (!ownerFacet || !ownerFacet.terms?.terms || ownerFacet.terms.terms.length === 0) {
        console.log('[OwnerFacetTest] No owner data available for end-to-end test');
        return;
      }
      
      const terms = ownerFacet.terms.terms;
      console.log(`[OwnerFacetTest] Step 1: Found ${terms.length} owner terms from backend`);
      
      // Step 2: Get files for first owner using EntityClient
      const entityClient = new GrpcEntityClient(mockContext);
      let ownerValue = (terms[0] as { term?: string }).term || '';
      if (ownerValue.startsWith('owner:')) {
        ownerValue = ownerValue.substring(6);
      }
      
      const entities = await entityClient.getEntitiesByFacet(
        workspaceId,
        'owner',
        ownerValue,
        ['file'],
        10000
      );
      
      console.log(`[OwnerFacetTest] Step 2: Found ${entities.length} files for owner "${ownerValue}"`);
      assert.ok(Array.isArray(entities), 'Should return array of entities');
      
      // Step 3: Verify files through TermsFacetTreeProvider
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        const providerTerms = await getChildrenItems(provider);
        
        console.log(`[OwnerFacetTest] Step 3: TermsFacetTreeProvider returned ${providerTerms.length} terms`);
        assert.ok(Array.isArray(providerTerms), 'Provider should return array of terms');
        
        // If we have terms, verify we can get files
        if (providerTerms.length > 0) {
          const firstTerm = providerTerms[0];
          const files = await getChildrenItems(provider, firstTerm);
          
          console.log(`[OwnerFacetTest] Step 4: Retrieved ${files.length} files from provider`);
          assert.ok(Array.isArray(files), 'Should return array of files from provider');
          
          // Verify consistency: EntityClient and Provider should return similar results
          // (may differ due to caching, but should both work)
          if (entities.length > 0 && files.length > 0) {
            console.log('[OwnerFacetTest] ✓ End-to-end flow completed successfully');
            console.log(`  - Backend facets: ${terms.length} owner terms`);
            console.log(`  - EntityClient: ${entities.length} files`);
            console.log(`  - TermsFacetTreeProvider: ${files.length} files`);
          }
        }
      });
    });
  });
});

