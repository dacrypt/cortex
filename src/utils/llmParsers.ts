/**
 * LLM Response Parsers - Robust parsing for LLM responses
 * Inspired by langchaingo patterns, provides consistent parsing between frontend and backend
 */

export interface ParseOptions {
  maxRetries?: number;
  cleanAggressively?: boolean;
}

/**
 * Robust JSON parser with automatic retry and cleanup
 */
export class JSONParser {
  private maxRetries: number;

  constructor(options: ParseOptions = {}) {
    this.maxRetries = options.maxRetries || 3;
  }

  /**
   * Parse JSON from LLM response with automatic cleanup and retry
   */
  async parseJSON<T>(response: string, options: ParseOptions = {}): Promise<T> {
    const maxRetries = options.maxRetries || this.maxRetries;
    let cleaned = this.cleanResponse(response);
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      if (attempt > 0) {
        // Try more aggressive cleaning on retry
        cleaned = this.cleanResponseAggressive(response);
        console.debug(`[JSONParser] Retry attempt ${attempt + 1} with aggressive cleaning`);
      }

      try {
        const result = JSON.parse(cleaned) as T;
        if (attempt > 0) {
          console.info(`[JSONParser] Parse succeeded after ${attempt + 1} attempts`);
        }
        return result;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
      }
    }

    throw new Error(
      `Failed to parse JSON after ${maxRetries} attempts: ${lastError?.message} (preview: ${cleaned.substring(0, 200)})`
    );
  }

  /**
   * Standard cleaning of LLM response
   */
  private cleanResponse(response: string): string {
    let cleaned = response.trim();

    // Remove markdown code blocks
    cleaned = cleaned.replace(/^```json\s*/i, '');
    cleaned = cleaned.replace(/^```\s*/g, '');
    cleaned = cleaned.replace(/\s*```$/g, '');
    cleaned = cleaned.trim();

    // Extract JSON object if wrapped in text
    const firstBrace = cleaned.indexOf('{');
    if (firstBrace > 0) {
      cleaned = cleaned.substring(firstBrace);
    }
    const lastBrace = cleaned.lastIndexOf('}');
    if (lastBrace > 0 && lastBrace < cleaned.length - 1) {
      cleaned = cleaned.substring(0, lastBrace + 1);
    }

    return cleaned;
  }

  /**
   * Aggressive cleaning for retry attempts
   */
  private cleanResponseAggressive(response: string): string {
    let cleaned = this.cleanResponse(response);

    // Remove common LLM artifacts
    cleaned = cleaned.replace(/^(Here is the JSON:|JSON:|Response:)\s*/i, '');
    cleaned = cleaned.trim();

    // Fix common JSON issues - remove trailing commas
    cleaned = cleaned.replace(/,(\s*[}\]])/g, '$1');

    // Remove comments (not valid in JSON but LLMs sometimes add them)
    const lines = cleaned.split('\n');
    const filteredLines = lines.filter((line) => {
      const trimmed = line.trim();
      return !trimmed.startsWith('//') && !trimmed.startsWith('#');
    });
    cleaned = filteredLines.join('\n');

    return cleaned;
  }
}

/**
 * String parser for simple text responses
 */
export class StringParser {
  /**
   * Parse string response with automatic cleanup
   */
  parseString(response: string): string {
    let cleaned = response.trim();

    // Remove markdown code blocks
    cleaned = cleaned.replace(/^```\s*/g, '');
    cleaned = cleaned.replace(/\s*```$/g, '');
    cleaned = cleaned.trim();

    // Remove quotes if the entire response is quoted
    if (cleaned.length >= 2) {
      if (
        (cleaned[0] === '"' && cleaned[cleaned.length - 1] === '"') ||
        (cleaned[0] === "'" && cleaned[cleaned.length - 1] === "'")
      ) {
        cleaned = cleaned.substring(1, cleaned.length - 1);
      }
    }

    // Remove trailing punctuation (iteratively to handle multiple)
    while (true) {
      const original = cleaned;
      cleaned = cleaned.replace(/[.,;:]$/, '');
      if (cleaned === original) {
        break; // No more punctuation to remove
      }
    }
    cleaned = cleaned.trim();

    return cleaned;
  }
}

/**
 * Array parser for string arrays (tags, lists, etc.)
 */
export class ArrayParser {
  private jsonParser: JSONParser;

  constructor() {
    this.jsonParser = new JSONParser();
  }

  /**
   * Parse string array from LLM response
   * Supports JSON arrays and comma-separated lists
   */
  async parseArray(response: string): Promise<string[]> {
    // Try parsing as JSON array first
    try {
      const result = await this.jsonParser.parseJSON<string[]>(response);
      // Clean and validate each element
      const cleaned = result
        .map((item) => {
          item = item.trim();
          item = item.replace(/^["']|["']$/g, ''); // Remove surrounding quotes
          return item;
        })
        .filter((item) => item !== '' && item.length <= 100);
      return cleaned;
    } catch (err) {
      // Fallback: try parsing as comma-separated list
      let cleaned = response.trim();
      cleaned = cleaned.replace(/^```\s*/g, '');
      cleaned = cleaned.replace(/\s*```$/g, '');
      cleaned = cleaned.trim();

      // Remove brackets if present
      cleaned = cleaned.replace(/^\[|\]$/g, '');
      cleaned = cleaned.trim();

      const items = cleaned.split(',');
      return items
        .map((item) => {
          item = item.trim();
          item = item.replace(/^["']|["']$/g, '');
          return item;
        })
        .filter((item) => item !== '');
    }
  }
}

/**
 * Convenience functions for common use cases
 */
export const jsonParser = new JSONParser();
export const stringParser = new StringParser();
export const arrayParser = new ArrayParser();

/**
 * Parse JSON response (convenience function)
 */
export async function parseJSON<T>(response: string): Promise<T> {
  return jsonParser.parseJSON<T>(response);
}

/**
 * Parse string response (convenience function)
 */
export function parseString(response: string): string {
  return stringParser.parseString(response);
}

/**
 * Parse array response (convenience function)
 */
export async function parseArray(response: string): Promise<string[]> {
  return arrayParser.parseArray(response);
}






