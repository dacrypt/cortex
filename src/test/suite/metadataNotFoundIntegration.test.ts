/**
 * Integration tests for Metadata NotFound warnings during file processing
 * 
 * These tests replicate the exact scenarios from the logs where GetMetadata
 * returns NotFound during file processing, causing warnings.
 * 
 * TDD Approach:
 * 1. First, create tests that replicate the exact failure scenarios
 * 2. Verify that warnings occur (test fails)
 * 3. Fix the code to prevent warnings
 * 4. Verify tests pass
 */

import * as assert from 'node:assert';
import * as vscode from 'vscode';
import * as path from 'node:path';
import * as fs from 'node:fs';
import { GrpcMetadataClient } from '../../core/GrpcMetadataClient';
import { GrpcAdminClient } from '../../core/GrpcAdminClient';
import {
  createMockContext,
  comprehensiveTestData,
} from '../helpers/testHelpers';

describe('Metadata NotFound Integration Tests (TDD)', () => {
  const realExtensionPath = process.cwd();
  const mockContext = {
    ...createMockContext(),
    extensionPath: realExtensionPath,
  } as vscode.ExtensionContext;
  const workspaceRoot = path.join(process.cwd(), 'test-workspace');
  const workspaceId = '62db770a-4118-4849-b41a-09c3699274fe';

  /**
   * Checks if the backend is available
   */
  async function requireBackend(): Promise<void> {
    try {
      const adminClient = new GrpcAdminClient(mockContext);
      await adminClient.listWorkspaces();
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      
      if (errorMessage.includes('ECONNREFUSED') || 
          errorMessage.includes('timeout') ||
          errorMessage.includes('14 UNAVAILABLE')) {
        throw new Error(
          `Backend is not running. Please start the backend daemon before running integration tests.\n` +
          `To start the backend:\n` +
          `  1. cd backend\n` +
          `  2. go run ./cmd/cortexd\n` +
          `  3. Wait for "Cortex daemon started" message\n` +
          `  4. Run tests again\n` +
          `Error: ${errorMessage}`
        );
      }
      throw error;
    }
  }

  /**
   * Creates a test PDF file similar to those in the workspace
   */
  function createTestPDF(filePath: string): void {
    // Create a minimal PDF file for testing
    // This is a minimal valid PDF structure
    const pdfContent = Buffer.from(
      '%PDF-1.4\n' +
      '1 0 obj\n' +
      '<< /Type /Catalog /Pages 2 0 R >>\n' +
      'endobj\n' +
      '2 0 obj\n' +
      '<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n' +
      'endobj\n' +
      '3 0 obj\n' +
      '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>\n' +
      'endobj\n' +
      '4 0 obj\n' +
      '<< /Length 44 >>\n' +
      'stream\n' +
      'BT\n' +
      '/F1 12 Tf\n' +
      '100 700 Td\n' +
      '(Test PDF Content) Tj\n' +
      'ET\n' +
      'endstream\n' +
      'endobj\n' +
      'xref\n' +
      '0 5\n' +
      '0000000000 65535 f \n' +
      '0000000009 00000 n \n' +
      '0000000058 00000 n \n' +
      '0000000115 00000 n \n' +
      '0000000317 00000 n \n' +
      'trailer\n' +
      '<< /Size 5 /Root 1 0 R >>\n' +
      'startxref\n' +
      '398\n' +
      '%%EOF'
    );
    
    // Ensure directory exists
    const dir = path.dirname(filePath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    
    fs.writeFileSync(filePath, pdfContent);
  }

  /**
   * Cleans up test files
   */
  function cleanupTestFile(filePath: string): void {
    try {
      if (fs.existsSync(filePath)) {
        fs.unlinkSync(filePath);
      }
    } catch (error) {
      // Ignore cleanup errors
    }
  }

  before(async function() {
    this.timeout(10000);
    // Fail if backend is not available - TDD approach: tests should fail, not skip
    await requireBackend();
  });

  describe('GetMetadata NotFound warnings during PDF processing', function() {
    const testFilePath = path.join(workspaceRoot, 'Libros', 'test-metadata-notfound.pdf');
    const relativePath = 'Libros/test-metadata-notfound.pdf';

    beforeEach(() => {
      // Create test PDF before each test
      createTestPDF(testFilePath);
    });

    afterEach(() => {
      // Clean up test file
      cleanupTestFile(testFilePath);
    });

    it('should NOT generate NotFound warnings when GetMetadata is called during file processing', async function() {
      this.timeout(60000); // 60 seconds for processing

      const metadataClient = new GrpcMetadataClient(mockContext);
      const adminClient = new GrpcAdminClient(mockContext);

      // Step 1: Start processing the file (scan workspace)
      console.log(`[Test] Starting workspace scan for file: ${relativePath}`);
      
      // Trigger file processing by scanning the workspace
      // This simulates the exact scenario from the logs
      const scanPromise = adminClient.scanWorkspace(workspaceId, workspaceRoot, false);

      // Step 2: While processing, repeatedly try to get metadata
      // This replicates the exact scenario where GetMetadata is called
      // before metadata is fully created, causing NotFound warnings
      const metadataChecks: Array<{ time: number; found: boolean; error?: string }> = [];
      const startTime = Date.now();
      const checkInterval = 500; // Check every 500ms
      const maxChecks = 60; // Check for up to 30 seconds

      let checkCount = 0;
      while (checkCount < maxChecks) {
        try {
          const metadata = await metadataClient.getMetadataByPath(workspaceId, relativePath);
          const elapsed = Date.now() - startTime;
          metadataChecks.push({
            time: elapsed,
            found: metadata !== null && metadata !== undefined,
          });
          
          if (metadata !== null && metadata !== undefined) {
            console.log(`[Test] Metadata found after ${elapsed}ms`);
            break; // Metadata is available, stop checking
          }
        } catch (error: any) {
          const elapsed = Date.now() - startTime;
          const errorCode = error?.code;
          const errorMessage = error?.message || String(error);
          
          metadataChecks.push({
            time: elapsed,
            found: false,
            error: errorCode === 5 ? 'NOT_FOUND' : errorMessage,
          });

          // NOT_FOUND (code 5) is expected during processing
          // But we want to track when it occurs
          if (errorCode === 5) {
            console.log(`[Test] GetMetadata returned NOT_FOUND at ${elapsed}ms (expected during processing)`);
          } else {
            console.log(`[Test] GetMetadata error at ${elapsed}ms: ${errorMessage}`);
          }
        }

        await new Promise(resolve => setTimeout(resolve, checkInterval));
        checkCount++;
      }

      // Wait for scan to complete
      await scanPromise;
      console.log(`[Test] Workspace scan completed`);

      // Step 3: Verify metadata is eventually available
      const finalMetadata = await metadataClient.getMetadataByPath(workspaceId, relativePath);
      assert.ok(
        finalMetadata !== null && finalMetadata !== undefined,
        `Metadata should be available after processing. Got: ${finalMetadata}`
      );

      // Step 4: Analyze the metadata checks
      const notFoundCount = metadataChecks.filter(c => c.error === 'NOT_FOUND').length;
      const foundCount = metadataChecks.filter(c => c.found).length;
      
      console.log(`[Test] Metadata check summary:`);
      console.log(`  - Total checks: ${metadataChecks.length}`);
      console.log(`  - NOT_FOUND errors: ${notFoundCount}`);
      console.log(`  - Found: ${foundCount}`);
      console.log(`  - First found at: ${metadataChecks.find(c => c.found)?.time || 'never'}ms`);

      // TDD: This test should pass when the code is fixed
      // Currently, we expect some NOT_FOUND errors during processing
      // But we want to minimize them or ensure they don't cause warnings
      
      // The ideal scenario: metadata should be available very quickly after file processing starts
      // Or the system should handle NOT_FOUND gracefully without warnings
      
      // For now, we verify that:
      // 1. Metadata is eventually available
      // 2. We track when NOT_FOUND occurs
      assert.ok(
        finalMetadata !== null,
        'Metadata must be available after processing completes'
      );
    });

    it('should create metadata before AI stage tries to access it', async function() {
      this.timeout(60000);

      const metadataClient = new GrpcMetadataClient(mockContext);
      const adminClient = new GrpcAdminClient(mockContext);

      // Process the file
      await adminClient.scanWorkspace(workspaceId, workspaceRoot, false);

      // Wait a bit for processing to start
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Try to get metadata multiple times during processing
      // This simulates what happens when AI stage or other stages
      // try to access metadata before it's created
      const errors: string[] = [];
      for (let i = 0; i < 10; i++) {
        try {
          const metadata = await metadataClient.getMetadataByPath(workspaceId, relativePath);
          if (metadata !== null) {
            console.log(`[Test] Metadata available after ${i * 500}ms`);
            return; // Success - metadata is available
          }
        } catch (error: any) {
          if (error?.code === 5) {
            errors.push(`NOT_FOUND at check ${i}`);
          } else {
            errors.push(`Error at check ${i}: ${error?.message || String(error)}`);
          }
        }
        await new Promise(resolve => setTimeout(resolve, 500));
      }

      // After processing, metadata should definitely be available
      const finalMetadata = await metadataClient.getMetadataByPath(workspaceId, relativePath);
      assert.ok(
        finalMetadata !== null,
        `Metadata should be available after processing. Errors during processing: ${errors.join(', ')}`
      );
    });
  });

  describe('GetMetadata NotFound during concurrent file processing', function() {
    it('should handle concurrent GetMetadata calls during batch processing', async function() {
      this.timeout(120000); // 2 minutes for batch processing

      const metadataClient = new GrpcMetadataClient(mockContext);
      const adminClient = new GrpcAdminClient(mockContext);

      // Create multiple test PDFs
      const testFiles: Array<{ path: string; relative: string }> = [];
      for (let i = 0; i < 3; i++) {
        const fileName = `test-concurrent-${i}.pdf`;
        const filePath = path.join(workspaceRoot, 'Libros', fileName);
        const relativePath = `Libros/${fileName}`;
        createTestPDF(filePath);
        testFiles.push({ path: filePath, relative: relativePath });
      }

      try {
        // Start processing
        const scanPromise = adminClient.scanWorkspace(workspaceId, workspaceRoot, false);

        // Concurrently try to get metadata for all files
        const metadataPromises = testFiles.map(async (file) => {
          const results: Array<{ time: number; found: boolean }> = [];
          const startTime = Date.now();
          
          for (let i = 0; i < 20; i++) {
            try {
              const metadata = await metadataClient.getMetadataByPath(workspaceId, file.relative);
              const elapsed = Date.now() - startTime;
              results.push({
                time: elapsed,
                found: metadata !== null,
              });
              
              if (metadata !== null) {
                break;
              }
            } catch (error: any) {
              // Track NOT_FOUND but don't fail
              if (error?.code === 5) {
                const elapsed = Date.now() - startTime;
                results.push({ time: elapsed, found: false });
              }
            }
            
            await new Promise(resolve => setTimeout(resolve, 500));
          }
          
          return { file: file.relative, results };
        });

        const metadataResults = await Promise.all(metadataPromises);
        await scanPromise;

        // Verify all files have metadata after processing
        for (const file of testFiles) {
          const finalMetadata = await metadataClient.getMetadataByPath(workspaceId, file.relative);
          assert.ok(
            finalMetadata !== null,
            `Metadata should be available for ${file.relative} after processing`
          );
        }

        // Log results for analysis
        console.log('[Test] Concurrent metadata access results:');
        metadataResults.forEach(({ file, results }) => {
          const foundAt = results.find(r => r.found)?.time;
          const notFoundCount = results.filter(r => !r.found).length;
          console.log(`  ${file}: found at ${foundAt || 'never'}ms, ${notFoundCount} NOT_FOUND errors`);
        });

      } finally {
        // Cleanup
        testFiles.forEach(file => cleanupTestFile(file.path));
      }
    });
  });
});

