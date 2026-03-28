/**
 * Shared test helpers and utilities
 * 
 * Centralizes common test utilities to avoid duplication across test files
 */

import * as vscode from 'vscode';
import { FileCacheService } from '../../core/FileCacheService';
import { IMetadataStore } from '../../core/IMetadataStore';

/**
 * Common FileEntry type used across all tree provider tests
 */
export type FileEntry = {
  relative_path?: string;
  filename?: string;
  extension?: string;
  file_size?: number;
  last_modified?: number;
  created_at?: number;
  enhanced?: {
    stats?: {
      modified?: number;
      accessed?: number;
      created?: number;
      size?: number;
    };
    language?: string;
    mime_type?: {
      category?: string;
      mime_type?: string;
      encoding?: string;
    };
    code_metrics?: {
      lines_of_code?: number;
      comment_percentage?: number;
      function_count?: number;
      class_count?: number;
      complexity?: number;
    };
    document_metrics?: {
      page_count?: number;
      word_count?: number;
      character_count?: number;
      author?: string;
      title?: string;
    };
    content_encoding?: string;
    language_confidence?: number;
    os_metadata?: {
      file_system?: {
        mount_point?: string;
        file_system_type?: string;
      };
    };
    audio_metadata?: {
      id3_artist?: string;
      bitrate?: number;
      sample_rate?: number;
      channels?: number;
      codec?: string;
      format?: string;
      id3_genre?: string;
      vorbis_genre?: string;
      id3_album?: string;
      vorbis_album?: string;
      id3_year?: number;
      vorbis_date?: string;
      has_album_art?: boolean;
    };
    video_metadata?: {
      width?: number;
      height?: number;
      duration?: number;
      bitrate?: number;
      frame_rate?: number;
      codec?: string;
      video_codec?: string;
      audio_codec?: string;
      container?: string;
      video_aspect_ratio?: string;
      has_subtitles?: boolean;
      subtitle_tracks?: string[];
      has_chapters?: boolean;
      is_3d?: boolean;
      is_hd?: boolean;
      is_4k?: boolean;
    };
    os_context_taxonomy?: {
      security?: {
        permission_level?: string;
        security_category?: string[];
        security_attributes?: string[];
        has_acls?: boolean;
        acl_complexity?: string;
      };
      ownership?: {
        owner_type?: string;
        group_category?: string;
        access_relations?: string[];
        ownership_pattern?: string;
      };
      temporal?: {
        access_frequency?: string;
        time_category?: string[];
      };
      system?: {
        system_file_type?: string;
        file_system_category?: string;
        system_attributes?: string[];
        system_features?: string[];
      };
    };
    error?: {
      code?: string;
      message?: string;
    };
  };
};

/**
 * Creates a minimal mock ExtensionContext for testing
 */
export function createMockContext(): vscode.ExtensionContext {
  return {
    workspaceState: {
      get: () => undefined,
      update: async () => undefined,
    },
    globalState: {
      get: () => undefined,
      update: async () => undefined,
    },
    extensionPath: '/test',
  } as unknown as vscode.ExtensionContext;
}

/**
 * Mocks FileCacheService.getFiles() to return provided files
 * Automatically restores original implementation after test
 */
export async function withMockedFileCache<T>(
  files: FileEntry[],
  fn: () => Promise<T>
): Promise<T> {
  const original = (FileCacheService as unknown as { getInstance: unknown }).getInstance;
  (FileCacheService as unknown as { getInstance: unknown }).getInstance = () => ({
    setWorkspaceId: () => undefined,
    getFiles: async () => files,
  });
  try {
    return await fn();
  } finally {
    (FileCacheService as unknown as { getInstance: unknown }).getInstance = original;
  }
}

/**
 * Helper to get children from a tree provider
 */
export async function getChildrenItems(
  provider: unknown,
  element?: unknown
): Promise<vscode.TreeItem[]> {
  const typed = provider as {
    getChildren: (arg?: unknown) => Promise<vscode.TreeItem[]>;
  };
  return await typed.getChildren(element);
}

/**
 * Creates a minimal mock IMetadataStore for testing
 */
export function createMockMetadataStore(overrides?: Partial<IMetadataStore>): IMetadataStore {
  const tags = new Set<string>();
  const filesByTag = new Map<string, string[]>();
  const contexts = new Set<string>();
  const filesByContext = new Map<string, string[]>();
  const metadata = new Map<string, any>();

  return {
    async initialize() {},
    getOrCreateMetadata(relativePath: string, extension: string) {
      return {
        file_id: relativePath,
        relativePath,
        tags: [],
        contexts: [],
        type: extension.slice(1) || 'unknown',
        created_at: Date.now(),
        updated_at: Date.now(),
      };
    },
    getMetadata: () => null,
    getMetadataByPath: (path: string) => metadata.get(path) || null,
    addTag: (path: string, tag: string) => {
      tags.add(tag);
      const files = filesByTag.get(tag) || [];
      if (!files.includes(path)) files.push(path);
      filesByTag.set(tag, files);
    },
    removeTag: () => {},
    addContext: (path: string, context: string) => {
      contexts.add(context);
      const files = filesByContext.get(context) || [];
      if (!files.includes(path)) files.push(path);
      filesByContext.set(context, files);
    },
    removeContext: () => {},
    addSuggestedContext: () => {},
    clearSuggestedContexts: () => {},
    getSuggestedContexts: () => [],
    getFilesBySuggestedContext: () => [],
    updateNotes: () => {},
    updateAISummary: () => {},
    ensureMetadataForFiles: () => 0,
    updateMirrorMetadata: () => {},
    clearMirrorMetadata: () => {},
    removeFile: () => {},
    getFilesByTag: (tag: string) => filesByTag.get(tag.toLowerCase()) || [],
    getFilesByContext: (context: string) => filesByContext.get(context.toLowerCase()) || [],
    getFilesByType: () => [],
    getAllTags: () => Array.from(tags),
    getTagCounts: () => {
      const counts = new Map<string, number>();
      for (const [tag, files] of filesByTag.entries()) {
        counts.set(tag, files.length);
      }
      return counts;
    },
    getAllContexts: () => Array.from(contexts),
    getAllSuggestedContexts: () => [],
    getAllTypes: () => [],
    close: () => {},
    ...overrides,
  } as IMetadataStore;
}

/**
 * Comprehensive test data covering various file scenarios
 */
export const comprehensiveTestData: FileEntry[] = [
  // Recent TypeScript file with full metadata
  {
    relative_path: 'src/recent.ts',
    filename: 'recent.ts',
    extension: '.ts',
    file_size: 1024,
    last_modified: Date.now() - 1000,
    created_at: Date.now() - 86400000,
    enhanced: {
      stats: {
        modified: Date.now() - 1000,
        accessed: Date.now() - 500,
        created: Date.now() - 86400000,
        size: 1024,
      },
      mime_type: {
        category: 'code',
        mime_type: 'text/typescript',
        encoding: 'utf-8',
      },
      code_metrics: {
        lines_of_code: 50,
        comment_percentage: 10,
        function_count: 3,
        class_count: 1,
      },
    },
  },
  // Old PDF document
  {
    relative_path: 'docs/old.pdf',
    filename: 'old.pdf',
    extension: '.pdf',
    file_size: 1024 * 1024 * 2,
    last_modified: Date.now() - 365 * 24 * 60 * 60 * 1000 * 2,
    enhanced: {
      stats: {
        modified: Date.now() - 365 * 24 * 60 * 60 * 1000 * 2,
        size: 1024 * 1024 * 2,
      },
      mime_type: {
        category: 'document',
        mime_type: 'application/pdf',
      },
      document_metrics: {
        page_count: 10,
        word_count: 500,
        author: 'Test Author',
        title: 'Test Document',
      },
    },
  },
  // File with missing metadata (tests fallback logic)
  {
    relative_path: 'data/unknown.bin',
    filename: 'unknown.bin',
    extension: '.bin',
    file_size: 512,
    last_modified: Date.now() - 86400000,
  },
  // Image file
  {
    relative_path: 'images/photo.png',
    filename: 'photo.png',
    extension: '.png',
    file_size: 1024 * 500,
    last_modified: Date.now() - 7 * 24 * 60 * 60 * 1000,
    enhanced: {
      stats: {
        modified: Date.now() - 7 * 24 * 60 * 60 * 1000,
        size: 1024 * 500,
      },
      mime_type: {
        category: 'image',
        mime_type: 'image/png',
      },
    },
  },
  // File with error
  {
    relative_path: 'bad/error.txt',
    filename: 'error.txt',
    extension: '.txt',
    file_size: 0,
    last_modified: Date.now() - 3600000,
    enhanced: {
      stats: {
        modified: Date.now() - 3600000,
      },
      error: {
        code: 'EACCES',
        message: 'Permission denied',
      },
    },
  },
  // File in subfolder
  {
    relative_path: 'src/utils/helper.ts',
    filename: 'helper.ts',
    extension: '.ts',
    file_size: 2048,
    last_modified: Date.now() - 1800000,
    enhanced: {
      stats: {
        modified: Date.now() - 1800000,
        size: 2048,
      },
      mime_type: {
        category: 'code',
        mime_type: 'text/typescript',
      },
      code_metrics: {
        lines_of_code: 100,
        comment_percentage: 15,
        function_count: 5,
        class_count: 0,
      },
    },
  },
];

