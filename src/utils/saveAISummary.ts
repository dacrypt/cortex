/**
 * Utility to save AI summaries to the repository
 */

import * as fs from 'fs/promises';
import * as path from 'path';

/**
 * Save an AI summary to the repository in .cortex/summaries/
 * @param workspaceRoot - Root of the workspace
 * @param relativePath - Relative path of the file being summarized
 * @param summary - The AI-generated summary
 * @param contentHash - Hash of the content that was summarized
 * @param keyTerms - Optional key terms extracted from the content
 * @returns Path to the saved summary file, or null if failed
 */
export async function saveAISummaryToFile(
  workspaceRoot: string,
  relativePath: string,
  summary: string,
  contentHash: string,
  keyTerms?: string[]
): Promise<string | null> {
  try {
    // Create .cortex/summaries directory if it doesn't exist
    const summariesDir = path.join(workspaceRoot, '.cortex', 'summaries');
    await fs.mkdir(summariesDir, { recursive: true });

    // Create a safe filename from the relative path
    // Replace path separators and special characters with underscores
    const safeFilename = relativePath
      .replace(/\//g, '_')
      .replace(/\\/g, '_')
      .replace(/[^a-zA-Z0-9._-]/g, '_')
      .replace(/_{2,}/g, '_'); // Replace multiple underscores with one

    // Add timestamp to make it unique and track when it was generated
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const summaryFilename = `${safeFilename}_${timestamp}.md`;
    const summaryPath = path.join(summariesDir, summaryFilename);

    // Create markdown content with metadata
    const markdownContent = `# AI Summary

**File:** \`${relativePath}\`
**Generated:** ${new Date().toISOString()}
**Content Hash:** \`${contentHash}\`

${keyTerms && keyTerms.length > 0 ? `**Key Terms:** ${keyTerms.join(', ')}\n\n` : ''}## Summary

${summary}
`;

    // Write the summary to file
    await fs.writeFile(summaryPath, markdownContent, 'utf-8');

    console.log(`[Cortex] Saved AI summary to ${summaryPath}`);
    return summaryPath;
  } catch (error) {
    console.error(
      `[Cortex] Failed to save AI summary for ${relativePath}:`,
      error
    );
    return null;
  }
}

/**
 * Get all saved summaries for a file
 * @param workspaceRoot - Root of the workspace
 * @param relativePath - Relative path of the file
 * @returns Array of summary file paths, sorted by date (newest first)
 */
export async function getSavedSummariesForFile(
  workspaceRoot: string,
  relativePath: string
): Promise<string[]> {
  try {
    const summariesDir = path.join(workspaceRoot, '.cortex', 'summaries');
    
    // Check if directory exists
    try {
      await fs.access(summariesDir);
    } catch {
      return []; // Directory doesn't exist, no summaries
    }

    // Create a safe filename prefix to match
    const safeFilenamePrefix = relativePath
      .replace(/\//g, '_')
      .replace(/\\/g, '_')
      .replace(/[^a-zA-Z0-9._-]/g, '_')
      .replace(/_{2,}/g, '_');

    // Read all files in summaries directory
    const files = await fs.readdir(summariesDir);
    
    // Filter files that match this file's prefix
    const matchingFiles = files
      .filter((file) => file.startsWith(safeFilenamePrefix) && file.endsWith('.md'))
      .map((file) => path.join(summariesDir, file));

    // Sort by modification time (newest first)
    const filesWithStats = await Promise.all(
      matchingFiles.map(async (filePath) => {
        const stats = await fs.stat(filePath);
        return { path: filePath, mtime: stats.mtime };
      })
    );

    filesWithStats.sort((a, b) => b.mtime.getTime() - a.mtime.getTime());
    return filesWithStats.map((f) => f.path);
  } catch (error) {
    console.error(
      `[Cortex] Failed to get saved summaries for ${relativePath}:`,
      error
    );
    return [];
  }
}










