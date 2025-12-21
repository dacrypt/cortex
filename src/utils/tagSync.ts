import { IMetadataStore } from '../core/IMetadataStore';
import {
  isOsTaggingSupported,
  normalizeTags,
  readOsTags,
  writeOsTags,
} from './osTags';

export function updateStoreTags(
  metadataStore: IMetadataStore,
  relativePath: string,
  desiredTags: string[]
): boolean {
  const metadata = metadataStore.getMetadataByPath(relativePath);
  if (!metadata) {
    return false;
  }

  const normalizedDesired = new Set(normalizeTags(desiredTags));
  const existing = new Set(metadata.tags);

  let changed = false;
  for (const tag of normalizedDesired) {
    if (!existing.has(tag)) {
      metadataStore.addTag(relativePath, tag);
      changed = true;
    }
  }
  for (const tag of existing) {
    if (!normalizedDesired.has(tag)) {
      metadataStore.removeTag(relativePath, tag);
      changed = true;
    }
  }

  return changed;
}

export async function syncStoreTagsFromOs(
  metadataStore: IMetadataStore,
  relativePath: string,
  absolutePath: string
): Promise<boolean> {
  if (!isOsTaggingSupported()) {
    return false;
  }

  const osTags = await readOsTags(absolutePath);
  if (!osTags) {
    return false;
  }

  return updateStoreTags(metadataStore, relativePath, osTags);
}

export async function addTagWithOsSync(
  metadataStore: IMetadataStore,
  relativePath: string,
  absolutePath: string,
  normalizedTag: string
): Promise<Error | null> {
  if (!isOsTaggingSupported()) {
    metadataStore.addTag(relativePath, normalizedTag);
    return null;
  }

  try {
    const osTags = await readOsTags(absolutePath);
    const merged = new Set(normalizeTags(osTags || []));
    merged.add(normalizedTag);
    const targetTags = Array.from(merged);
    await writeOsTags(absolutePath, targetTags);
    updateStoreTags(metadataStore, relativePath, targetTags);
    return null;
  } catch (error) {
    metadataStore.addTag(relativePath, normalizedTag);
    return error instanceof Error
      ? error
      : new Error('Failed to sync OS tags');
  }
}
