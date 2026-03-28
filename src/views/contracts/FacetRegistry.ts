/**
 * Frontend Facet Registry
 * 
 * Maintains a registry of all available facets and their configurations.
 * This mirrors the backend FacetRegistry but is frontend-specific.
 */

import * as vscode from 'vscode';
import { FacetConfig, FacetType, FacetCategory } from './IFacetProvider';

/**
 * Frontend facet registry
 */
export class FacetRegistry {
  private readonly facets = new Map<string, FacetConfig>();
  private readonly facetsByCategory = new Map<FacetCategory, FacetConfig[]>();
  private readonly facetsByType = new Map<FacetType, FacetConfig[]>();

  constructor() {
    this.registerAll();
  }

  /**
   * Register all available facets
   */
  private registerAll(): void {
    // Core facets
    this.register({
      field: 'extension',
      label: 'By Extension',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by file extension',
      icon: new vscode.ThemeIcon('file-code'),
      aliases: ['ext'],
    });

    this.register({
      field: 'type',
      label: 'By Type',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by inferred file type',
      icon: new vscode.ThemeIcon('symbol-class'),
      aliases: ['file_type'],
    });

    this.register({
      field: 'document_type',
      label: 'By Document Type',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by AI-suggested document/content type (semantic classification)',
      icon: new vscode.ThemeIcon('symbol-class'),
    });

    this.register({
      field: 'mime_type',
      label: 'By MIME Type',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by MIME content type (e.g., image/jpeg, application/pdf)',
      icon: new vscode.ThemeIcon('file-submodule'),
    });

    this.register({
      field: 'mime_category',
      label: 'By MIME Category',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by MIME category (e.g., image, document, text, code)',
      icon: new vscode.ThemeIcon('symbol-constant'),
    });

    this.register({
      field: 'indexing_status',
      label: 'By Indexing Status',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by indexing completion status',
      icon: new vscode.ThemeIcon('sync'),
      aliases: ['index_status'],
    });

    // Organization facets
    this.register({
      field: 'tag',
      label: 'By Tag',
      type: FacetType.Terms,
      category: FacetCategory.Organization,
      description: 'Group files by user-assigned tags',
      icon: new vscode.ThemeIcon('tag'),
    });

    this.register({
      field: 'project',
      label: 'By Project',
      type: FacetType.Terms,
      category: FacetCategory.Organization,
      description: 'Group files by assigned project',
      icon: new vscode.ThemeIcon('folder'),
      aliases: ['context'],
    });

    this.register({
      field: 'owner',
      label: 'By Owner',
      type: FacetType.Terms,
      category: FacetCategory.Organization,
      description: 'Group files by file owner',
      icon: new vscode.ThemeIcon('account'),
      aliases: ['owners'],
    });

    this.register({
      field: 'cluster',
      label: 'By Cluster',
      type: FacetType.Terms,
      category: FacetCategory.Organization,
      description: 'Group files by semantic cluster (AI-detected document communities)',
      icon: new vscode.ThemeIcon('group-by-ref-type'),
      aliases: ['document_cluster'],
    });

    // Temporal facets
    this.register({
      field: 'modified',
      label: 'By Modified Date',
      type: FacetType.DateRange,
      category: FacetCategory.Temporal,
      description: 'Group files by modification date',
      icon: new vscode.ThemeIcon('calendar'),
      aliases: ['last_modified', 'mtime'],
    });

    this.register({
      field: 'created',
      label: 'By Created Date',
      type: FacetType.DateRange,
      category: FacetCategory.Temporal,
      description: 'Group files by creation date',
      icon: new vscode.ThemeIcon('calendar'),
      aliases: ['created_at'],
    });

    this.register({
      field: 'accessed',
      label: 'By Accessed Date',
      type: FacetType.DateRange,
      category: FacetCategory.Temporal,
      description: 'Group files by last access date',
      icon: new vscode.ThemeIcon('calendar'),
      aliases: ['accessed_at'],
    });

    this.register({
      field: 'changed',
      label: 'By Changed Date',
      type: FacetType.DateRange,
      category: FacetCategory.Temporal,
      description: 'Group files by metadata change date',
      icon: new vscode.ThemeIcon('calendar'),
      aliases: ['changed_at'],
    });

    this.register({
      field: 'temporal_pattern',
      label: 'By Temporal Pattern',
      type: FacetType.Terms,
      category: FacetCategory.Temporal,
      description: 'Group files by access patterns',
      icon: new vscode.ThemeIcon('pulse'),
    });

    // Content facets
    this.register({
      field: 'language',
      label: 'By Language',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by detected language',
      icon: new vscode.ThemeIcon('globe'),
      aliases: ['detected_language'],
    });

    this.register({
      field: 'category',
      label: 'By Category',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by AI-assigned category',
      icon: new vscode.ThemeIcon('sparkle'),
      aliases: ['ai_category'],
    });

    this.register({
      field: 'author',
      label: 'By Author',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by extracted author',
      icon: new vscode.ThemeIcon('person'),
      aliases: ['authors'],
    });

    this.register({
      field: 'publication_year',
      label: 'By Publication Year',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by publication year',
      icon: new vscode.ThemeIcon('calendar'),
      aliases: ['year'],
    });

    this.register({
      field: 'readability_level',
      label: 'By Readability Level',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by readability level',
      icon: new vscode.ThemeIcon('text-size'),
    });

    this.register({
      field: 'purpose',
      label: 'By Purpose',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by AI-suggested purpose',
      icon: new vscode.ThemeIcon('target'),
    });

    this.register({
      field: 'audience',
      label: 'By Audience',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by AI-suggested audience',
      icon: new vscode.ThemeIcon('people'),
    });

    this.register({
      field: 'domain',
      label: 'By Domain',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by taxonomy domain',
      icon: new vscode.ThemeIcon('layers'),
    });

    this.register({
      field: 'subdomain',
      label: 'By Subdomain',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by taxonomy subdomain',
      icon: new vscode.ThemeIcon('layers'),
    });

    this.register({
      field: 'topic',
      label: 'By Topic',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by taxonomy topic',
      icon: new vscode.ThemeIcon('lightbulb'),
    });

    this.register({
      field: 'location',
      label: 'By Location',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by extracted location',
      icon: new vscode.ThemeIcon('location'),
    });

    this.register({
      field: 'organization',
      label: 'By Organization',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by mentioned organization',
      icon: new vscode.ThemeIcon('organization'),
    });

    this.register({
      field: 'sentiment',
      label: 'By Sentiment',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by sentiment analysis',
      icon: new vscode.ThemeIcon('smiley'),
    });

    this.register({
      field: 'duplicate_type',
      label: 'By Duplicate Type',
      type: FacetType.Terms,
      category: FacetCategory.Content,
      description: 'Group files by duplicate relationship type',
      icon: new vscode.ThemeIcon('copy'),
    });

    // Numeric range facets
    this.register({
      field: 'size',
      label: 'By Size',
      type: FacetType.NumericRange,
      category: FacetCategory.Core,
      description: 'Group files by size ranges',
      icon: new vscode.ThemeIcon('file-binary'),
      aliases: ['file_size'],
    });

    this.register({
      field: 'complexity',
      label: 'By Complexity',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by code complexity',
      icon: new vscode.ThemeIcon('warning'),
    });

    this.register({
      field: 'project_score',
      label: 'By Project Score',
      type: FacetType.NumericRange,
      category: FacetCategory.Organization,
      description: 'Group files by project assignment score',
      icon: new vscode.ThemeIcon('graph'),
    });

    this.register({
      field: 'function_count',
      label: 'By Function Count',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by number of functions',
      icon: new vscode.ThemeIcon('symbol-method'),
    });

    this.register({
      field: 'lines_of_code',
      label: 'By Lines of Code',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by lines of code',
      icon: new vscode.ThemeIcon('list-ordered'),
    });

    this.register({
      field: 'comment_percentage',
      label: 'By Comment Percentage',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by comment percentage',
      icon: new vscode.ThemeIcon('comment'),
    });

    this.register({
      field: 'content_quality',
      label: 'By Content Quality',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by content quality score',
      icon: new vscode.ThemeIcon('star'),
    });

    this.register({
      field: 'image_dimensions',
      label: 'By Image Dimensions',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by image dimensions',
      icon: new vscode.ThemeIcon('image'),
    });

    this.register({
      field: 'image_color_depth',
      label: 'By Image Color Depth',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by image color depth',
      icon: new vscode.ThemeIcon('image'),
    });

    this.register({
      field: 'image_iso',
      label: 'By Image ISO',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by image ISO setting',
      icon: new vscode.ThemeIcon('camera'),
    });

    this.register({
      field: 'image_aperture',
      label: 'By Image Aperture',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by image aperture',
      icon: new vscode.ThemeIcon('camera'),
    });

    this.register({
      field: 'image_focal_length',
      label: 'By Image Focal Length',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by image focal length',
      icon: new vscode.ThemeIcon('camera'),
    });

    this.register({
      field: 'audio_duration',
      label: 'By Audio Duration',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by audio duration',
      icon: new vscode.ThemeIcon('unmute'),
    });

    this.register({
      field: 'audio_bitrate',
      label: 'By Audio Bitrate',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by audio bitrate',
      icon: new vscode.ThemeIcon('unmute'),
    });

    this.register({
      field: 'audio_sample_rate',
      label: 'By Audio Sample Rate',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by audio sample rate',
      icon: new vscode.ThemeIcon('unmute'),
    });

    this.register({
      field: 'video_duration',
      label: 'By Video Duration',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by video duration',
      icon: new vscode.ThemeIcon('play'),
    });

    this.register({
      field: 'video_bitrate',
      label: 'By Video Bitrate',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by video bitrate',
      icon: new vscode.ThemeIcon('play'),
    });

    this.register({
      field: 'video_frame_rate',
      label: 'By Video Frame Rate',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by video frame rate',
      icon: new vscode.ThemeIcon('play'),
    });

    this.register({
      field: 'language_confidence',
      label: 'By Language Confidence',
      type: FacetType.NumericRange,
      category: FacetCategory.Specialized,
      description: 'Group files by language detection confidence',
      icon: new vscode.ThemeIcon('globe'),
    });

    // Media term facets
    this.register({
      field: 'image_format',
      label: 'By Image Format',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by image format',
      icon: new vscode.ThemeIcon('image'),
    });

    this.register({
      field: 'camera_make',
      label: 'By Camera Make',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by camera manufacturer',
      icon: new vscode.ThemeIcon('camera'),
    });

    this.register({
      field: 'audio_genre',
      label: 'By Audio Genre',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by audio genre',
      icon: new vscode.ThemeIcon('unmute'),
    });

    this.register({
      field: 'audio_artist',
      label: 'By Audio Artist',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by audio artist',
      icon: new vscode.ThemeIcon('unmute'),
    });

    this.register({
      field: 'video_resolution',
      label: 'By Video Resolution',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by video resolution',
      icon: new vscode.ThemeIcon('play'),
    });

    this.register({
      field: 'video_codec',
      label: 'By Video Codec',
      type: FacetType.Terms,
      category: FacetCategory.Specialized,
      description: 'Group files by video codec',
      icon: new vscode.ThemeIcon('play'),
    });

    // System facets
    this.register({
      field: 'permission_level',
      label: 'By Permission Level',
      type: FacetType.Terms,
      category: FacetCategory.System,
      description: 'Group files by permission level',
      icon: new vscode.ThemeIcon('shield'),
    });

    // Structure facets
    // Folder facet: groups files by folder name (e.g., "compras", "libros")
    // Uses SQL query to extract folder name from relative_path
    this.register({
      field: 'folder',
      label: 'By Folder',
      type: FacetType.Terms,
      category: FacetCategory.Core,
      description: 'Group files by folder name',
      icon: new vscode.ThemeIcon('folder'),
    });

    // Category facets
    this.register({
      field: 'writing_category',
      label: 'Writing',
      type: FacetType.Category,
      category: FacetCategory.Organization,
      description: 'Group files by writing category projects',
      icon: new vscode.ThemeIcon('edit'),
    });

    this.register({
      field: 'collection_category',
      label: 'Collections',
      type: FacetType.Category,
      category: FacetCategory.Organization,
      description: 'Group files by collection category projects',
      icon: new vscode.ThemeIcon('library'),
    });

    this.register({
      field: 'development_category',
      label: 'Development',
      type: FacetType.Category,
      category: FacetCategory.Organization,
      description: 'Group files by development category projects',
      icon: new vscode.ThemeIcon('code'),
    });

    this.register({
      field: 'management_category',
      label: 'Management',
      type: FacetType.Category,
      category: FacetCategory.Organization,
      description: 'Group files by management category projects',
      icon: new vscode.ThemeIcon('checklist'),
    });

    this.register({
      field: 'hierarchical_category',
      label: 'Hierarchical',
      type: FacetType.Category,
      category: FacetCategory.Organization,
      description: 'Group files by hierarchical category projects',
      icon: new vscode.ThemeIcon('list-tree'),
    });

    // Metrics facets
    this.register({
      field: 'code_metrics',
      label: 'Code Metrics',
      type: FacetType.Metrics,
      category: FacetCategory.Specialized,
      description: 'Group files by code metrics',
      icon: new vscode.ThemeIcon('graph'),
    });

    this.register({
      field: 'document_metrics',
      label: 'Document Metrics',
      type: FacetType.Metrics,
      category: FacetCategory.Specialized,
      description: 'Group files by document metrics',
      icon: new vscode.ThemeIcon('book'),
    });

    // Issues facets
    // TODO: Implement in backend before enabling
    // this.register({
    //   field: 'issue_type',
    //   label: 'Issues and TODOs',
    //   type: FacetType.Issues,
    //   category: FacetCategory.Specialized,
    //   description: 'Group files by issues and TODOs',
    //   icon: new vscode.ThemeIcon('issues'),
    // });

    // Metadata facets
    // TODO: Implement in backend before enabling
    // this.register({
    //   field: 'metadata_type',
    //   label: 'Metadata Classification',
    //   type: FacetType.Metadata,
    //   category: FacetCategory.Specialized,
    //   description: 'Group files by metadata type',
    //   icon: new vscode.ThemeIcon('symbol-property'),
    // });
  }

  /**
   * Register a facet configuration
   */
  private register(config: FacetConfig): void {
    this.facets.set(config.field, config);

    // Index by category
    if (!this.facetsByCategory.has(config.category)) {
      this.facetsByCategory.set(config.category, []);
    }
    this.facetsByCategory.get(config.category)!.push(config);

    // Index by type
    if (!this.facetsByType.has(config.type)) {
      this.facetsByType.set(config.type, []);
    }
    this.facetsByType.get(config.type)!.push(config);

    // Index aliases
    if (config.aliases) {
      for (const alias of config.aliases) {
        this.facets.set(alias, config);
      }
    }
  }

  /**
   * Get facet configuration by field name
   */
  get(field: string): FacetConfig | undefined {
    return this.facets.get(field);
  }

  /**
   * Get all facets
   */
  getAll(): FacetConfig[] {
    return Array.from(new Set(this.facets.values()));
  }

  /**
   * Get facets by category
   */
  getByCategory(category: FacetCategory): FacetConfig[] {
    return this.facetsByCategory.get(category) || [];
  }

  /**
   * Get facets by type
   */
  getByType(type: FacetType): FacetConfig[] {
    return this.facetsByType.get(type) || [];
  }

  /**
   * Resolve field name to canonical name
   */
  resolve(field: string): FacetConfig | undefined {
    return this.get(field);
  }
}

/**
 * Singleton instance
 */
let registryInstance: FacetRegistry | undefined;

/**
 * Get the global facet registry instance
 */
export function getFacetRegistry(): FacetRegistry {
  if (!registryInstance) {
    registryInstance = new FacetRegistry();
  }
  return registryInstance;
}

