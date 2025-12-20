export type IndexingPhase =
  | 'idle'
  | 'scanning'
  | 'basic'
  | 'contentTypes'
  | 'documents'
  | 'code'
  | 'done'
  | 'error';

export interface IndexingStatus {
  phase: IndexingPhase;
  message: string;
  processed: number;
  total: number;
  isIndexing: boolean;
}

export function formatIndexingMessage(status: IndexingStatus): string {
  const base = status.message || 'Indexing';
  const count =
    status.total > 0
      ? `${status.processed}/${status.total}`
      : status.processed > 0
      ? `${status.processed}`
      : '';
  const suffix = count ? ` (${count})` : '';
  return `Indexando: ${base}${suffix}`;
}
