import * as fs from 'fs/promises';
import { computeContentHash, isLikelyTextExtension } from './aiSummary';
import { MirrorStore } from '../core/MirrorStore';

export type AIContentSource = 'file' | 'mirror';

export interface AIContentResult {
  content: string;
  contentHash: string;
  source: AIContentSource;
}

export async function resolveAIContent(
  params: {
    absolutePath: string;
    relativePath: string;
    extension: string;
    editorText?: string;
  },
  mirrorStore: MirrorStore,
  options: { allowMirrorGeneration?: boolean } = {}
): Promise<AIContentResult | null> {
  const extension = params.extension.toLowerCase();

  if (isLikelyTextExtension(extension)) {
    const content =
      params.editorText ?? (await fs.readFile(params.absolutePath, 'utf8'));
    if (content.includes('\u0000')) {
      return null;
    }
    return {
      content,
      contentHash: computeContentHash(content),
      source: 'file',
    };
  }

  if (!mirrorStore.isMirrorableExtension(extension)) {
    return null;
  }

  const allowMirrorGeneration = options.allowMirrorGeneration !== false;
  const content = allowMirrorGeneration
    ? await mirrorStore.ensureMirrorContent(
        params.absolutePath,
        params.relativePath,
        extension
      )
    : await mirrorStore.readMirrorContent(params.relativePath, extension);
  if (content == null || content.includes('\u0000')) {
    return null;
  }
  return {
    content,
    contentHash: computeContentHash(content),
    source: 'mirror',
  };
}
