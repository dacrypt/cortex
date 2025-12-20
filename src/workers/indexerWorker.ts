import { parentPort } from 'worker_threads';
import { MetadataExtractor, EnhancedMetadata } from '../extractors/MetadataExtractor';

type TaskType = 'basic' | 'mime' | 'code' | 'document';

interface IndexerTask {
  type: TaskType;
  workspaceRoot: string;
  file: {
    absolutePath: string;
    relativePath: string;
    extension: string;
    enhanced: EnhancedMetadata;
  };
}

const extractorByRoot = new Map<string, MetadataExtractor>();

function getExtractor(workspaceRoot: string): MetadataExtractor {
  let extractor = extractorByRoot.get(workspaceRoot);
  if (!extractor) {
    extractor = new MetadataExtractor(workspaceRoot);
    extractorByRoot.set(workspaceRoot, extractor);
  }
  return extractor;
}

if (!parentPort) {
  throw new Error('Worker initialized without parent port');
}

parentPort.on('message', async (message: { id: number; payload: IndexerTask }) => {
  const { id, payload } = message;

  try {
    const extractor = getExtractor(payload.workspaceRoot);
    const { file } = payload;
    const enhanced = { ...file.enhanced };

    switch (payload.type) {
      case 'basic':
        Object.assign(
          enhanced,
          await extractor.extractBasic(
            file.absolutePath,
            file.relativePath,
            file.extension
          )
        );
        break;
      case 'mime':
        await extractor.extractMimeTypeMetadata(file.absolutePath, enhanced);
        break;
      case 'code':
        await extractor.extractCodeMetadata(file.absolutePath, enhanced);
        break;
      case 'document':
        await extractor.extractDocumentMetadata(
          file.absolutePath,
          enhanced,
          file.extension
        );
        break;
      default:
        throw new Error(`Unknown task type: ${payload.type}`);
    }

    parentPort?.postMessage({
      id,
      ok: true,
      result: { enhanced },
    });
  } catch (error) {
    const err = error as Error;
    parentPort?.postMessage({
      id,
      ok: false,
      error: err.message || 'Worker task failed',
    });
  }
});
