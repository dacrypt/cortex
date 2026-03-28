/**
 * Integration tests for Cluster Facet
 * 
 * These tests validate that the cluster facet:
 * 1. Returns clusters correctly from the backend
 * 2. Can retrieve cluster members (files) when clicking on a cluster
 * 3. Works end-to-end through ClusterFacetTreeProvider
 * 4. Handles missing cluster data gracefully
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';
import * as path from 'node:path';
import { ClusterFacetTreeProvider } from '../../views/ClusterFacetTreeProvider';
import { GrpcClusteringClient } from '../../core/GrpcClusteringClient';
import {
  createMockContext,
  withMockedFileCache,
  getChildrenItems,
  comprehensiveTestData,
} from '../helpers/testHelpers';

describe('Cluster Facet Integration Tests', () => {
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
      const clusteringClient = new GrpcClusteringClient(mockContext);
      await clusteringClient.getClusters(workspaceId);
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
      
      // Check if clustering service is not implemented
      if (errorMessage.includes('UNIMPLEMENTED') || errorMessage.includes('unknown service')) {
        throw new Error(
          `Clustering service is not implemented in the backend. ` +
          `This feature requires backend support. ` +
          `Error: ${errorMessage}`
        );
      }
      
      throw error;
    }
  }

  /**
   * Creates a ClusterFacetTreeProvider
   */
  function createProvider(): ClusterFacetTreeProvider {
    return new ClusterFacetTreeProvider(
      workspaceRoot,
      mockContext,
      workspaceId
    );
  }

  describe('Backend Clustering API', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should retrieve clusters from backend', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const clusteringClient = new GrpcClusteringClient(mockContext);
      
      const clusters = await clusteringClient.getClusters(workspaceId);
      
      assert.ok(Array.isArray(clusters), 'Should return array of clusters');
      console.log(`[ClusterFacetTest] Found ${clusters.length} clusters from backend`);
      
      // If clusters exist, verify structure
      if (clusters.length > 0) {
        clusters.forEach((cluster, index) => {
          assert.ok(cluster !== null && cluster !== undefined,
            `Cluster at index ${index} should not be null/undefined`);
          assert.ok(typeof cluster.id === 'string' && cluster.id.length > 0,
            `Cluster at index ${index} should have a non-empty id`);
          assert.ok(typeof cluster.memberCount === 'number' && cluster.memberCount >= 0,
            `Cluster at index ${index} should have a non-negative memberCount`);
          assert.ok(typeof cluster.confidence === 'number' && cluster.confidence >= 0 && cluster.confidence <= 1,
            `Cluster at index ${index} should have confidence between 0 and 1`);
          
          // Log cluster details
          const clusterName = cluster.name?.trim() || `Cluster ${cluster.id.substring(0, 8)}`;
          console.log(`[ClusterFacetTest] Cluster ${index + 1}: ${clusterName} (${cluster.memberCount} members, ${(cluster.confidence * 100).toFixed(0)}% confidence)`);
        });
      } else {
        console.log('[ClusterFacetTest] No clusters found in workspace (this is acceptable if clustering has not been run)');
      }
    });

    it('should retrieve cluster members for a specific cluster', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const clusteringClient = new GrpcClusteringClient(mockContext);
      
      // First, get available clusters
      const clusters = await clusteringClient.getClusters(workspaceId);
      
      if (clusters.length === 0) {
        console.log('[ClusterFacetTest] No clusters available, skipping cluster members test');
        return;
      }
      
      // Get first cluster
      const firstCluster = clusters[0];
      const clusterId = firstCluster.id;
      const clusterName = firstCluster.name?.trim() || `Cluster ${clusterId.substring(0, 8)}`;
      
      console.log(`[ClusterFacetTest] Testing cluster members for: ${clusterName} (${clusterId})`);
      
      // Get cluster members
      const members = await clusteringClient.getClusterMembers(workspaceId, clusterId);
      
      assert.ok(Array.isArray(members), 'Should return array of cluster members');
      console.log(`[ClusterFacetTest] Found ${members.length} members for cluster ${clusterName}`);
      
      // Verify members structure
      if (members.length > 0) {
        members.forEach((member, index) => {
          assert.ok(member !== null && member !== undefined,
            `Member at index ${index} should not be null/undefined`);
          assert.ok(typeof member.documentId === 'string' && member.documentId.length > 0,
            `Member at index ${index} should have a non-empty documentId`);
          assert.ok(typeof member.relativePath === 'string' && member.relativePath.length > 0,
            `Member at index ${index} should have a non-empty relativePath`);
          assert.ok(typeof member.membershipScore === 'number' && member.membershipScore >= 0 && member.membershipScore <= 1,
            `Member at index ${index} should have membershipScore between 0 and 1`);
          assert.ok(typeof member.isCentral === 'boolean',
            `Member at index ${index} should have a boolean isCentral`);
          
          // Log member details
          console.log(`[ClusterFacetTest]   Member ${index + 1}: ${member.filename || path.basename(member.relativePath)} (${(member.membershipScore * 100).toFixed(0)}%${member.isCentral ? ', central' : ''})`);
        });
      } else {
        console.warn(`[ClusterFacetTest] Cluster ${clusterName} has no members (this may indicate an issue)`);
      }
      
      // Verify member count matches
      assert.strictEqual(members.length, firstCluster.memberCount,
        `Member count should match cluster.memberCount (${firstCluster.memberCount})`);
    });

    it('should return empty array for non-existent cluster', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      const clusteringClient = new GrpcClusteringClient(mockContext);
      
      const members = await clusteringClient.getClusterMembers(
        workspaceId,
        'nonexistent-cluster-id-12345'
      );
      
      assert.ok(Array.isArray(members), 'Should return array even for non-existent cluster');
      assert.strictEqual(members.length, 0,
        'Should return empty array for non-existent cluster');
    });
  });

  describe('ClusterFacetTreeProvider Integration', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should load clusters in ClusterFacetTreeProvider', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Get root items (should be clusters)
        const rootItems = await getChildrenItems(provider);
        
        assert.ok(Array.isArray(rootItems), 'Should return array of clusters');
        console.log(`[ClusterFacetTest] ClusterFacetTreeProvider returned ${rootItems.length} clusters`);
        
        // If clusters exist, verify structure
        if (rootItems.length > 0) {
          // Check if we have actual cluster data or placeholder
          const hasPlaceholder = rootItems.some((item) => {
            const label = String(item.label);
            return label.includes('No clusters found') || label.includes('not available');
          });
          
          if (hasPlaceholder) {
            // If we have placeholder, it means no cluster data is available
            console.log('[ClusterFacetTest] No cluster data available in workspace (this is acceptable)');
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
            
            // Should be collapsible (has children)
            assert.ok(item.collapsibleState !== vscode.TreeItemCollapsibleState.None,
              `Item at index ${index} should be collapsible (has children)`);
          });
        } else {
          // Empty results are acceptable if no cluster data exists
          console.log('[ClusterFacetTest] No clusters returned (workspace may have no cluster data)');
        }
      });
    });

    it('should retrieve files when clicking on a cluster', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Get clusters
        const clusters = await getChildrenItems(provider);
        
        if (clusters.length === 0) {
          console.log('[ClusterFacetTest] No clusters available, skipping file retrieval test');
          return;
        }
        
        // Get first cluster
        const firstCluster = clusters[0];
        const clusterLabel = String(firstCluster.label);
        console.log(`[ClusterFacetTest] Testing file retrieval for cluster: "${clusterLabel}"`);
        
        // Check if this is a placeholder item (not a real cluster)
        const clusterItem = firstCluster as any;
        if (clusterLabel.includes('No clusters found') || clusterLabel.includes('not available')) {
          console.log('[ClusterFacetTest] Placeholder item found, skipping test (no clusters available)');
          return;
        }
        
        // Verify cluster has a value (cluster ID)
        assert.ok(clusterItem.value, `Cluster should have a value (cluster ID), but got: ${JSON.stringify(clusterItem)}`);
        console.log(`[ClusterFacetTest] Cluster ID: ${clusterItem.value}`);
        console.log(`[ClusterFacetTest] Cluster item details:`, {
          id: clusterItem.id,
          kind: clusterItem.kind,
          facet: clusterItem.facet,
          value: clusterItem.value,
          term: clusterItem.term,
          label: clusterLabel
        });
        
        // First, verify backend has members for this cluster
        const clusteringClient = new GrpcClusteringClient(mockContext);
        const backendMembers = await clusteringClient.getClusterMembers(workspaceId, clusterItem.value);
        console.log(`[ClusterFacetTest] Backend has ${backendMembers.length} members for cluster ${clusterItem.value}`);
        
        if (backendMembers.length === 0) {
          console.log('[ClusterFacetTest] Cluster has no members in backend, skipping file retrieval test');
          return;
        }
        
        // Now get files for this cluster (simulating click/expansion)
        console.log(`[ClusterFacetTest] Calling getChildrenItems(provider, firstCluster) to get files...`);
        const files = await getChildrenItems(provider, firstCluster);
        
        assert.ok(Array.isArray(files), 'Should return array of files');
        console.log(`[ClusterFacetTest] Found ${files.length} files for cluster "${clusterLabel}"`);
        
        // This is the critical test: files should not be empty if backend has members
        if (files.length === 0) {
          console.error(`[ClusterFacetTest] ❌ FAILURE: Cluster "${clusterLabel}" returned 0 files!`);
          console.error(`[ClusterFacetTest] Cluster ID: ${clusterItem.value}`);
          console.error(`[ClusterFacetTest] Cluster kind: ${clusterItem.kind}`);
          console.error(`[ClusterFacetTest] Cluster facet: ${clusterItem.facet}`);
          console.error(`[ClusterFacetTest] Backend returned ${backendMembers.length} members directly`);
          console.error(`[ClusterFacetTest] This indicates a bug in ClusterFacetTreeProvider.getChildren() or getClusterMembers()`);
          
          throw new Error(
            `Cluster "${clusterLabel}" has ${backendMembers.length} members in backend but provider returned 0 files. ` +
            `This indicates a bug in ClusterFacetTreeProvider. ` +
            `Cluster ID: ${clusterItem.value}, Kind: ${clusterItem.kind}, Facet: ${clusterItem.facet}`
          );
        }
        
        // Verify files structure
        assert.ok(files.length > 0, `Should have at least one file for cluster with ${backendMembers.length} members`);
        
        files.forEach((file, index) => {
          assert.ok(file !== null && file !== undefined,
            `File at index ${index} should not be null/undefined`);
          
          const fileLabel = String(file.label);
          assert.ok(fileLabel.length > 0,
            `File at index ${index} should have a non-empty label`);
          
          // Should not be error items
          assert.ok(!fileLabel.includes('Error') && !fileLabel.includes('timeout'),
            `File at index ${index} should not be an error item: "${fileLabel}"`);
          
          // Should not be placeholder items (unless cluster is actually empty)
          if (fileLabel.includes('No files')) {
            throw new Error(`Cluster "${clusterLabel}" returned placeholder "No files" but backend has ${backendMembers.length} members`);
          }
          
          // Verify file has resourceUri or relativePath
          const fileItem = file as any;
          assert.ok(
            fileItem.resourceUri || fileItem.payload?.metadata?.relativePath,
            `File at index ${index} should have resourceUri or relativePath in payload`
          );
          
          console.log(`[ClusterFacetTest]   File ${index + 1}: ${fileLabel}`);
        });
        
        console.log(`[ClusterFacetTest] ✓ Successfully retrieved ${files.length} files for cluster "${clusterLabel}"`);
      });
    });

    it('should handle getFilesForTerm correctly', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        
        // Get clusters
        const clusters = await getChildrenItems(provider);
        
        if (clusters.length === 0) {
          console.log('[ClusterFacetTest] No clusters available, skipping getFilesForTerm test');
          return;
        }
        
        // Get first cluster
        const firstCluster = clusters[0] as any;
        const clusterLabel = String(firstCluster.label);
        
        // Check if this is a placeholder item (not a real cluster)
        if (clusterLabel.includes('No clusters found') || clusterLabel.includes('not available')) {
          console.log('[ClusterFacetTest] Placeholder item found, skipping test (no clusters available)');
          return;
        }
        
        const clusterId = firstCluster.value;
        const clusterName = firstCluster.term || clusterLabel.split(' (')[0];
        
        // Verify we have a valid cluster ID
        assert.ok(clusterId, `Cluster should have a value (cluster ID)`);
        assert.ok(clusterName, `Cluster should have a name or term`);
        
        console.log(`[ClusterFacetTest] Testing getFilesForTerm with cluster ID: ${clusterId}, name: ${clusterName}`);
        
        // Test getFilesForTerm with cluster ID
        const filesById = await provider.getFilesForTerm(clusterId, 'cluster');
        console.log(`[ClusterFacetTest] getFilesForTerm with ID returned ${filesById.length} files`);
        
        // Test getFilesForTerm with cluster name
        const filesByName = await provider.getFilesForTerm(clusterName, 'cluster');
        console.log(`[ClusterFacetTest] getFilesForTerm with name returned ${filesByName.length} files`);
        
        // Both should return the same files (or at least one should work)
        assert.ok(filesById.length > 0 || filesByName.length > 0,
          `At least one of getFilesForTerm(ID) or getFilesForTerm(name) should return files`);
        
        // If both work, they should return the same number of files
        if (filesById.length > 0 && filesByName.length > 0) {
          assert.strictEqual(filesById.length, filesByName.length,
            `getFilesForTerm should return same number of files whether searching by ID or name`);
        }
      });
    });

    it('should handle empty cluster data gracefully', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      await withMockedFileCache([], async () => {
        const provider = createProvider();
        
        const clusters = await getChildrenItems(provider);
        
        // Should return array (may be empty or have placeholder)
        assert.ok(Array.isArray(clusters),
          'Should return array even with no cluster data');
      });
    });
  });

  describe('End-to-End Cluster Facet Flow', () => {
    before(async function() {
      // Fail if backend is not available
      await requireBackend();
    });

    it('should complete full flow: clusters -> click cluster -> files', async () => {
      // Ensure backend is available - test will fail if not
      await requireBackend();
      
      // Step 1: Get clusters from backend
      const clusteringClient = new GrpcClusteringClient(mockContext);
      const backendClusters = await clusteringClient.getClusters(workspaceId);
      
      if (backendClusters.length === 0) {
        console.log('[ClusterFacetTest] No clusters available for end-to-end test');
        return;
      }
      
      console.log(`[ClusterFacetTest] Step 1: Found ${backendClusters.length} clusters from backend`);
      
      // Step 2: Get cluster members from backend
      const firstCluster = backendClusters[0];
      const backendMembers = await clusteringClient.getClusterMembers(workspaceId, firstCluster.id);
      console.log(`[ClusterFacetTest] Step 2: Found ${backendMembers.length} members for cluster "${firstCluster.name || firstCluster.id}"`);
      
      // Step 3: Verify through ClusterFacetTreeProvider
      await withMockedFileCache(comprehensiveTestData, async () => {
        const provider = createProvider();
        const providerClusters = await getChildrenItems(provider);
        
        console.log(`[ClusterFacetTest] Step 3: ClusterFacetTreeProvider returned ${providerClusters.length} clusters`);
        assert.ok(Array.isArray(providerClusters), 'Provider should return array of clusters');
        
        // If we have clusters, verify we can get files
        if (providerClusters.length > 0) {
          const firstProviderCluster = providerClusters[0] as any;
          const files = await getChildrenItems(provider, firstProviderCluster);
          
          console.log(`[ClusterFacetTest] Step 4: Retrieved ${files.length} files from provider`);
          assert.ok(Array.isArray(files), 'Should return array of files from provider');
          
          // Verify consistency: Backend and Provider should return similar results
          if (backendMembers.length > 0 && files.length > 0) {
            console.log('[ClusterFacetTest] ✓ End-to-end flow completed successfully');
            console.log(`  - Backend clusters: ${backendClusters.length} clusters`);
            console.log(`  - Backend members: ${backendMembers.length} files`);
            console.log(`  - ClusterFacetTreeProvider: ${files.length} files`);
            
            // Verify counts match (or are close - may differ due to filtering)
            assert.ok(files.length > 0, 'Provider should return at least one file');
          } else if (backendMembers.length > 0 && files.length === 0) {
            throw new Error(
              `Backend has ${backendMembers.length} members but provider returned 0 files. ` +
              `This indicates a bug in ClusterFacetTreeProvider.`
            );
          }
        }
      });
    });
  });
});

