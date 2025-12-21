import * as crypto from 'crypto';

const STOPWORDS = new Set([
  'the', 'and', 'for', 'with', 'this', 'that', 'from', 'into', 'than', 'then',
  'else', 'when', 'while', 'where', 'what', 'which', 'who', 'whom', 'whose',
  'also', 'been', 'are', 'was', 'were', 'will', 'would', 'could', 'should',
  'have', 'has', 'had', 'not', 'but', 'can', 'may', 'might', 'our', 'your',
  'their', 'them', 'they', 'you', 'we', 'its', 'it\'s', 'true', 'false', 'null',
  'return', 'const', 'let', 'var', 'function', 'class', 'interface', 'type',
  'import', 'export', 'default', 'public', 'private', 'protected', 'static',
  'async', 'await', 'new', 'try', 'catch', 'throw', 'case', 'break', 'if',
  'else', 'switch', 'for', 'while', 'do', 'in', 'of', 'to', 'as', 'is'
]);

const TEXT_EXTENSIONS = new Set([
  '.ts', '.tsx', '.js', '.jsx', '.py', '.java', '.go', '.rs', '.rb', '.php',
  '.cs', '.swift', '.kt', '.c', '.cpp', '.h', '.hpp', '.m', '.mm',
  '.md', '.txt', '.rst', '.adoc',
  '.json', '.yml', '.yaml', '.toml', '.xml', '.ini', '.env',
  '.html', '.css', '.scss', '.less',
  '.sql', '.sh', '.bash', '.zsh'
]);

export function computeContentHash(content: string): string {
  return crypto.createHash('sha256').update(content).digest('hex');
}

export function extractKeyTerms(content: string, maxTerms = 12): string[] {
  const counts = new Map<string, number>();
  const tokens = content.match(/[A-Za-z_][A-Za-z0-9_]{2,}/g) || [];

  for (const raw of tokens) {
    const token = raw.toLowerCase();
    if (STOPWORDS.has(token)) {
      continue;
    }
    counts.set(token, (counts.get(token) || 0) + 1);
  }

  return Array.from(counts.entries())
    .sort((a, b) => {
      if (b[1] !== a[1]) return b[1] - a[1];
      return a[0].localeCompare(b[0]);
    })
    .slice(0, maxTerms)
    .map(([term]) => term);
}

export function isLikelyTextExtension(extension: string): boolean {
  return TEXT_EXTENSIONS.has(extension.toLowerCase());
}
