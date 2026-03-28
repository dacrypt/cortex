/**
 * TermsFacetTreeProvider - Displays term facets (extension, tag, project, type)
 *
 * Shows aggregated counts of distinct values
 */

import * as vscode from 'vscode';
import * as path from 'node:path';
import { GrpcKnowledgeClient } from '../core/GrpcKnowledgeClient';
import { GrpcAdminClient } from '../core/GrpcAdminClient';
import { GrpcEntityClient } from '../core/GrpcEntityClient';
import { Entity } from '../models/entity';
import { FileCacheService } from '../core/FileCacheService';
import { IMetadataStore } from '../core/IMetadataStore';
import { getFileActivityTimestamp, sortFilesByActivity, sortRelativePathsByActivity, type ActivityFileEntry } from '../utils/fileActivity';
// Note: Removed getContentCategoryForFile and normalizeType imports
// Frontend should not classify files - all classification is done by backend
// import { getContentCategoryForFile, normalizeType } from './contracts/facetSources';
import { t } from './i18n';

type TreeChangeEvent = TermsFacetTreeItem | undefined | null | void;

class TermsFacetTreeItem extends vscode.TreeItem {
  constructor(
    public readonly label: string,
    public readonly collapsibleState: vscode.TreeItemCollapsibleState,
    public readonly resourceUri?: vscode.Uri,
    public readonly isFile: boolean = false,
    public readonly term?: string,
    public readonly field?: string,
    public readonly groupKey?: string
  ) {
    super(label, collapsibleState);

    if (isFile && resourceUri) {
      this.command = {
        command: 'vscode.open',
        title: 'Open File',
        arguments: [resourceUri],
      };
      this.iconPath = vscode.ThemeIcon.File;
      this.contextValue = 'cortex-file';
    } else if (term) {
      this.iconPath = new vscode.ThemeIcon('tag');
      this.contextValue = 'cortex-facet-term';
    } else if (groupKey) {
      this.iconPath = new vscode.ThemeIcon('library');
      this.contextValue = 'cortex-facet-group';
    } else {
      this.iconPath = new vscode.ThemeIcon('list-unordered');
      this.contextValue = 'cortex-facet-field';
    }
  }
}

export class TermsFacetTreeProvider
  implements vscode.TreeDataProvider<TermsFacetTreeItem>
{
  private readonly _onDidChangeTreeData: vscode.EventEmitter<TreeChangeEvent> =
    new vscode.EventEmitter<TreeChangeEvent>();
  readonly onDidChangeTreeData: vscode.Event<TreeChangeEvent> =
    this._onDidChangeTreeData.event;
  private readonly knowledgeClient: GrpcKnowledgeClient;
  private readonly workspaceId: string;
  private readonly workspaceRoot: string;
  private readonly fileCacheService: FileCacheService;
  private readonly metadataStore?: IMetadataStore;
  private readonly extensionContext: vscode.ExtensionContext;
  private facetField: string = 'extension'; // Default field
  private readonly showFieldSelector: boolean;
  private readonly facetCache: Map<string, { terms: Array<{ term: string; count: number }>; timestamp: number }> = new Map();
  private readonly CACHE_TTL = 30000; // 30 seconds
  private readonly fieldGroups: Array<{ key: string; label: string; fields: Array<{ field: string; label: string }> }>;
  private readonly unknownLabel = '?';

  constructor(
    workspaceRoot: string,
    context: vscode.ExtensionContext,
    workspaceId: string,
    field?: string,
    metadataStore?: IMetadataStore
  ) {
    this.workspaceRoot = workspaceRoot;
    this.workspaceId = workspaceId;
    this.extensionContext = context;
    if (typeof field === 'string') {
      this.facetField = field;
      this.showFieldSelector = false;
    } else {
      this.showFieldSelector = true;
      if (field && !metadataStore) {
        metadataStore = field as unknown as IMetadataStore;
      }
    }
    this.knowledgeClient = new GrpcKnowledgeClient(context);
    const adminClient = new GrpcAdminClient(context);
    this.fileCacheService = FileCacheService.getInstance(adminClient);
    this.fileCacheService.setWorkspaceId(workspaceId);
    this.metadataStore = metadataStore;
    this.fieldGroups = [
      {
        key: 'core',
        label: 'Core File Attributes',
        fields: [
          { field: 'extension', label: 'By Extension' },
          { field: 'type', label: 'By Type' },
          { field: 'indexing_status', label: 'By Indexing Status' },
          { field: 'tag', label: 'By Tag' },
        ],
      },
      {
        key: 'temporal',
        label: 'Temporal',
        fields: [
          { field: 'temporal_pattern', label: 'By Temporal Pattern' },
        ],
      },
      {
        key: 'language_content',
        label: 'Language & Content',
        fields: [
          { field: 'language', label: 'By Language' },
          { field: 'content_encoding', label: 'By Content Encoding' },
          { field: 'document_type', label: 'By Document Type' },
          { field: 'mime_type', label: 'By MIME Type' },
          { field: 'mime_category', label: 'By MIME Category' },
          { field: 'purpose', label: 'By Purpose' },
          { field: 'audience', label: 'By Audience' },
          { field: 'readability_level', label: 'By Readability Level' },
        ],
      },
      {
        key: 'ai_taxonomy',
        label: 'AI & Taxonomy',
        fields: [
          { field: 'category', label: 'By AI Category' },
          { field: 'domain', label: 'By Taxonomy Domain' },
          { field: 'subdomain', label: 'By Taxonomy Subdomain' },
          { field: 'topic', label: 'By Taxonomy Topic' },
        ],
      },
      {
        key: 'people',
        label: 'People & Roles',
        fields: [
          { field: 'author', label: 'By Author' },
        ],
      },
      {
        key: 'geography',
        label: 'Geography',
        fields: [
          { field: 'location', label: 'By Location' },
        ],
      },
      {
        key: 'organizations_publication',
        label: 'Organizations & Publication',
        fields: [
          { field: 'organization', label: 'By Organization' },
          { field: 'publication_year', label: 'By Publication Year' },
        ],
      },
      {
        key: 'events_references',
        label: 'Events & References',
        fields: [
          { field: 'event', label: 'By Event' },
          { field: 'citation_type', label: 'By Citation Type' },
        ],
      },
      {
        key: 'audio',
        label: 'Audio',
        fields: [
          { field: 'audio_codec', label: 'By Audio Codec' },
          { field: 'audio_format', label: 'By Audio Format' },
          { field: 'audio_genre', label: 'By Audio Genre' },
          { field: 'audio_artist', label: 'By Audio Artist' },
          { field: 'audio_album', label: 'By Audio Album' },
          { field: 'audio_year', label: 'By Audio Year' },
          { field: 'audio_channels', label: 'By Audio Channels' },
          { field: 'audio_has_album_art', label: 'By Has Album Art' },
        ],
      },
      {
        key: 'video',
        label: 'Video',
        fields: [
          { field: 'video_resolution', label: 'By Video Resolution' },
          { field: 'video_codec', label: 'By Video Codec' },
          { field: 'video_audio_codec', label: 'By Video Audio Codec' },
          { field: 'video_container', label: 'By Video Container' },
          { field: 'video_aspect_ratio', label: 'By Video Aspect Ratio' },
          { field: 'video_has_subtitles', label: 'By Has Subtitles' },
          { field: 'video_subtitle_languages', label: 'By Subtitle Language' },
          { field: 'video_has_chapters', label: 'By Has Chapters' },
          { field: 'video_is_3d', label: 'By Is 3D' },
          { field: 'video_quality_tier', label: 'By Quality Tier' },
        ],
      },
      {
        key: 'images',
        label: 'Images',
        fields: [
          { field: 'image_format', label: 'By Image Format' },
          { field: 'image_color_space', label: 'By Image Color Space' },
          { field: 'camera_make', label: 'By Camera Make' },
          { field: 'camera_model', label: 'By Camera Model' },
          { field: 'image_gps_location', label: 'By Image GPS Location' },
          { field: 'image_orientation', label: 'By Image Orientation' },
          { field: 'image_has_transparency', label: 'By Has Transparency' },
          { field: 'image_is_animated', label: 'By Is Animated' },
        ],
      },
      {
        key: 'relationships',
        label: 'Relationships',
        fields: [
          { field: 'relationship_type', label: 'By Relationship Type' },
        ],
      },
      {
        key: 'enrichment',
        label: 'Enrichment & Duplicates',
        fields: [
          { field: 'sentiment', label: 'By Sentiment' },
          { field: 'duplicate_type', label: 'By Duplicate Type' },
        ],
      },
      {
        key: 'os_security',
        label: 'OS & Security',
        fields: [
          { field: 'owner', label: 'By Owner' },
          { field: 'permission_level', label: 'By Permission Level' },
          { field: 'owner_type', label: 'By Owner Type' },
          { field: 'group_category', label: 'By Group Category' },
          { field: 'security_category', label: 'By Security Category' },
          { field: 'security_attributes', label: 'By Security Attributes' },
          { field: 'has_acls', label: 'By Has ACLs' },
          { field: 'acl_complexity', label: 'By ACL Complexity' },
          { field: 'access_relation', label: 'By Access Relation' },
          { field: 'ownership_pattern', label: 'By Ownership Pattern' },
          { field: 'access_frequency', label: 'By Access Frequency' },
          { field: 'time_category', label: 'By Time Category' },
          { field: 'system_file_type', label: 'By System File Type' },
          { field: 'file_system_category', label: 'By File System Category' },
          { field: 'system_attributes', label: 'By System Attributes' },
          { field: 'system_features', label: 'By System Features' },
          { field: 'filesystem_type', label: 'By Filesystem Type' },
          { field: 'mount_point', label: 'By Mount Point' },
        ],
      },
      {
        key: 'projects',
        label: 'Projects',
        fields: [
          { field: 'project', label: 'By Project' },
        ],
      },
    ];
  }

  setField(field: string): void {
    this.facetField = field;
    this.facetCache.clear();
    this.refresh();
  }

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: TermsFacetTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: TermsFacetTreeItem): Promise<TermsFacetTreeItem[]> {
    if (!element) {
      if (this.showFieldSelector) {
        return this.getGroupItems();
      }
      return await this.getTermsForField(this.facetField);
    }
    if (element.term && element.field) {
      try {
        return await this.getFilesForTerm(element.term, String(element.field));
      } catch (error) {
        console.error(`[TermsFacetTree] Error fetching files for ${element.field}:${element.term}:`, error);
        const message = error instanceof Error ? error.message : String(error);
        const errorItem = new TermsFacetTreeItem(
          `Error loading ${element.field} "${element.term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        errorItem.iconPath = new vscode.ThemeIcon('error');
        errorItem.tooltip = `Failed to load files: ${message}`;
        errorItem.id = `placeholder:${element.field}:${element.term}:error`;
        return [errorItem];
      }
    }
    if (element.groupKey) {
      return this.getFieldItems(element.groupKey);
    }
    if (element.field) {
      return await this.getTermsForField(String(element.field));
    }
    return [];
  }

  private getGroupItems(): TermsFacetTreeItem[] {
    return this.fieldGroups.map((group) => {
      const item = new TermsFacetTreeItem(
        group.label,
        vscode.TreeItemCollapsibleState.Collapsed,
        undefined,
        false,
        undefined,
        undefined,
        group.key
      );
      item.id = `group:${group.key}`;
      return item;
    });
  }

  private getFieldItems(groupKey: string): TermsFacetTreeItem[] {
    const group = this.fieldGroups.find((entry) => entry.key === groupKey);
    if (!group) {
      return [];
    }
    return group.fields.map(({ field, label }) => {
      const item = new TermsFacetTreeItem(
        label,
        vscode.TreeItemCollapsibleState.Collapsed,
        undefined,
        false,
        undefined,
        field
      );
      item.id = `facet-field:${field}`;
      return item;
    });
  }

  private async getTermsForField(field: string): Promise<TermsFacetTreeItem[]> {
    const fieldKey = String(field);
    try {
      const cached = this.facetCache.get(fieldKey);
      if (cached && Date.now() - cached.timestamp < this.CACHE_TTL) {
        return this.buildTermItems(cached.terms, fieldKey);
      }

      const terms = await this.collectTermsForField(fieldKey);
      if (terms.length === 0) {
        return this.getEmptyPlaceholder(fieldKey);
      }

      terms.sort((a: { term: string; count: number }, b: { term: string; count: number }) => {
        if (b.count !== a.count) {
          return b.count - a.count;
        }
        return a.term.localeCompare(b.term);
      });

      this.facetCache.set(fieldKey, {
        terms,
        timestamp: Date.now()
      });

      return this.buildTermItems(terms, fieldKey);
    } catch (error) {
      console.error(`[TermsFacetTree] Error fetching facets for ${fieldKey}:`, error);
      const errorItem = new TermsFacetTreeItem(
        `Error loading ${fieldKey} facets`,
        vscode.TreeItemCollapsibleState.None
      );
      errorItem.iconPath = new vscode.ThemeIcon('error');
      errorItem.tooltip = `Failed to load facets: ${error}`;
      return [errorItem];
    }
  }

  private buildTermItems(
    terms: Array<{ term: string; count: number }>,
    field: string
  ): TermsFacetTreeItem[] {
    if (terms.length === 0) {
      return this.getEmptyPlaceholder(field);
    }

    return terms.map(({ term, count }) => {
      const label = `${term} (${count})`;

      const item = new TermsFacetTreeItem(
        label,
        vscode.TreeItemCollapsibleState.Collapsed,
        undefined,
        false,
        term,
        field
      );
      item.id = `facet:${field}:${term}`;

      item.tooltip = `${field}: ${term}\nCount: ${count} files`;
      item.description = `${count} files`;

      return item;
    });
  }

  private async collectTermsForField(
    field: string
  ): Promise<Array<{ term: string; count: number }>> {
    const backendTerms = await this.getBackendTermsForField(field);
    console.log(`[TermsFacetTree] collectTermsForField(${field}): backend returned ${backendTerms.length} terms`);
    
    if (backendTerms.length > 0) {
      console.log(`[TermsFacetTree] Using ${backendTerms.length} backend terms for ${field}`);
      const missingCount = await this.countMissingValues(field);
      if (missingCount > 0 && !backendTerms.some((entry) => entry.term === this.unknownLabel)) {
        return [...backendTerms, { term: this.unknownLabel, count: missingCount }];
      }
      return backendTerms;
    }
    
    console.log(`[TermsFacetTree] No backend terms for ${field}, trying local cache`);

    if (field === 'tag') {
      if (!this.metadataStore) {
        return [];
      }
      const tags = this.metadataStore.getAllTags();
      const counts = this.metadataStore.getTagCounts();
      return tags.map((tag) => ({
        term: tag,
        count: counts.get(tag) || 0,
      }));
    }

    // Note: 'type' facet should come from backend, not frontend classification
    // Frontend should not classify files - all classification is done by backend
    // The backend's GetTypeFacet provides the authoritative type classification

    if (field === 'context' || field === 'project') {
      const projects = await this.knowledgeClient.listProjects(this.workspaceId);
      if (!projects || projects.length === 0) {
        return [];
      }
      const entries = await Promise.all(
        projects.map(async (project) => {
          const documents = await this.knowledgeClient.queryDocuments(
            this.workspaceId,
            project.id,
            false
          );
          return { term: project.name, count: documents.length };
        })
      );
      // Filter out entries with empty terms and projects with 0 files
      return entries.filter((entry) => 
        entry.term !== undefined && 
        entry.term !== null && 
        entry.term !== '' && 
        entry.count > 0
      );
    }

    const filesCache = await this.fileCacheService.getFiles();
    const counts = new Map<string, number>();

    let missingCount = 0;
    for (const file of filesCache) {
      const values = this.extractFieldValues(file, field);
      if (values.length === 0) {
        missingCount += 1;
        continue;
      }
      for (const value of values) {
        if (!value) continue;
        counts.set(value, (counts.get(value) || 0) + 1);
      }
    }

    if (missingCount > 0) {
      counts.set(this.unknownLabel, (counts.get(this.unknownLabel) || 0) + missingCount);
    }

    return Array.from(counts.entries()).map(([term, count]) => ({ term, count }));
  }

  private async getBackendTermsForField(
    field: string
  ): Promise<Array<{ term: string; count: number }>> {
    try {
      const response = await this.knowledgeClient.getFacets(
        this.workspaceId,
        [{ field, type: 'terms' }]
      );
      
      console.log(`[TermsFacetTree] Backend response for ${field}:`, JSON.stringify(response, null, 2));
      
      const result = response?.results?.[0];
      const terms = result?.terms?.terms;
      
      if (!Array.isArray(terms)) {
        console.log(`[TermsFacetTree] No terms array for ${field}, result:`, result);
        return [];
      }
      
      console.log(`[TermsFacetTree] Found ${terms.length} terms for ${field} from backend`);
      
      const processedTermsMap = new Map<string, number>();
      for (const entry of terms) {
        let term = entry.term || '';
        // Remove "owner:" prefix if present (backend returns "owner:username" format)
        if (term.startsWith('owner:')) {
          term = term.substring(6); // Remove "owner:" prefix
        }
        term = this.normalizeFacetTerm(term);
        const count = entry.count || 0;
        if (!term || count <= 0) {
          continue;
        }
        processedTermsMap.set(term, (processedTermsMap.get(term) || 0) + count);
      }
      const processedTerms = Array.from(processedTermsMap.entries()).map(([term, count]) => ({ term, count }));
      
      console.log(`[TermsFacetTree] Processed ${processedTerms.length} valid terms for ${field}:`, processedTerms);
      
      return processedTerms;
    } catch (error) {
      console.warn(`[TermsFacetTree] Backend facets unavailable for ${field}:`, error);
      return [];
    }
  }

  private normalizeFacetTerm(term: string): string {
    const trimmed = term.trim();
    if (!trimmed) {
      return this.unknownLabel;
    }
    const lowered = trimmed.toLowerCase();
    if (lowered === 'unknown' || lowered === 'desconocido' || lowered === 'n/a') {
      return this.unknownLabel;
    }
    return trimmed;
  }

  private async countMissingValues(field: string): Promise<number> {
    if (!this.fileCacheService) {
      return 0;
    }
    const filesCache = await this.fileCacheService.getFiles();
    let missingCount = 0;
    for (const file of filesCache) {
      const values = this.extractFieldValues(file, field);
      if (values.length === 0) {
        missingCount += 1;
      }
    }
    return missingCount;
  }

  private async getFilesForTerm(term: string, field: string): Promise<TermsFacetTreeItem[]> {
    if (field === 'extension') {
      return await this.getFilesByExtension(term);
    }
    if (field === 'document_type') {
      return await this.getFilesByDocumentType(term);
    }
    if (field === 'mime_type') {
      return await this.getFilesByMimeType(term);
    }
    if (field === 'mime_category') {
      return await this.getFilesByMimeCategory(term);
    }
    if (field === 'folder') {
      return await this.getFilesByFolder(term);
    }
    if (field === 'tag') {
      return await this.getFilesByTag(term);
    }
    if (field === 'type') {
      return await this.getFilesByType(term);
    }
    if (field === 'context' || field === 'project') {
      return await this.getFilesByProjectName(term);
    }
    if (field === 'owner') {
      return await this.getFilesByOwner(term);
    }
    if (this.isMetadataFacet(field)) {
      return await this.getFilesByMetadataField(term, field);
    }

    const placeholder = new TermsFacetTreeItem(
      t('unsupportedFacetField', { field }),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('info');
    placeholder.id = `placeholder:unsupported:${field}`;
    return [placeholder];
  }

  private async getFilesByExtension(term: string): Promise<TermsFacetTreeItem[]> {
    // Use backend for extension facet - frontend should not filter files locally
    // The backend has the authoritative file list and extension data
    if (!this.knowledgeClient) {
      const placeholder = new TermsFacetTreeItem(
        `Backend unavailable for extension "${term}"`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('warning');
      placeholder.id = `placeholder:extension:${term}:no-backend`;
      placeholder.tooltip = `Extension "${term}" requires backend connection.`;
      return [placeholder];
    }
    
    try {
      // Use EntityClient to get files by extension facet from backend
      const entityClient = new GrpcEntityClient(this.extensionContext);
      
      // Normalize extension: backend expects extension with leading dot (e.g., ".csv")
      const normalizedTerm = this.normalizeExtension(term);
      
      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 8 seconds')), 8000);
      });
      
      console.log(`[TermsFacetTree] Fetching files for extension: "${normalizedTerm}" from backend`);
      const startTime = Date.now();
      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'extension',
          normalizedTerm,
          ['file']
        ),
        timeoutPromise
      ]);
      const duration = Date.now() - startTime;
      console.log(`[TermsFacetTree] getEntitiesByFacet completed in ${duration}ms for extension: "${normalizedTerm}"`);
      
      console.log(`[TermsFacetTree] Received ${entities?.length || 0} entities for extension: "${normalizedTerm}"`);
      
      if (!entities || entities.length === 0) {
        const placeholder = new TermsFacetTreeItem(
          `No files found for extension "${normalizedTerm}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('info');
        placeholder.id = `placeholder:extension:${normalizedTerm}:empty`;
        placeholder.tooltip = `No files have extension "${normalizedTerm}" in the workspace.`;
        return [placeholder];
      }
      
      // Convert entities to file items
      const files = entities
        .filter((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath);
          if (!relativePath) {
            console.warn(`[TermsFacetTree] Entity missing path:`, e);
          }
          return e.type === 'file' && relativePath;
        })
        .map((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath) || '';
          const absolutePath = path.join(this.workspaceRoot, relativePath);
          
          const item = new TermsFacetTreeItem(
            path.basename(relativePath),
            vscode.TreeItemCollapsibleState.None,
            vscode.Uri.file(absolutePath),
            true
          );
          item.id = `file:extension:${normalizedTerm}:${relativePath}`;
          item.description = path.dirname(relativePath);
          item.tooltip = `${relativePath}\nExtension: ${normalizedTerm}`;
          return item;
        });
      
      if (files.length === 0) {
        console.warn(`[TermsFacetTree] All entities filtered out (missing paths) for extension: "${normalizedTerm}"`);
        const placeholder = new TermsFacetTreeItem(
          `No valid files found for extension "${normalizedTerm}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('warning');
        placeholder.id = `placeholder:extension:${normalizedTerm}:invalid`;
        return [placeholder];
      }
      
      // Sort by activity (last_modified) if available
      files.sort((a: TermsFacetTreeItem, b: TermsFacetTreeItem) => {
        const aTime = (a as unknown as { lastModified?: number }).lastModified || 0;
        const bTime = (b as unknown as { lastModified?: number }).lastModified || 0;
        return bTime - aTime;
      });
      
      console.log(`[TermsFacetTree] Returning ${files.length} files for extension: "${normalizedTerm}"`);
      return files;
    } catch (error) {
      console.error(`[TermsFacetTree] Failed to get files by extension from backend:`, error);
      const placeholder = new TermsFacetTreeItem(
        `Error loading extension "${term}": ${(error as Error).message}`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('error');
      placeholder.id = `placeholder:extension:${term}:error`;
      placeholder.tooltip = `Failed to fetch files: ${(error as Error).message}`;
      return [placeholder];
    }
  }

  private async getFilesByDocumentType(term: string): Promise<TermsFacetTreeItem[]> {
    // document_type comes from suggested_taxonomy (AI-suggested), not MIME category
    // Must use backend - file cache doesn't include suggested_taxonomy
    if (!this.knowledgeClient) {
      const placeholder = new TermsFacetTreeItem(
        `Backend unavailable for document type "${term}"`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('warning');
      placeholder.id = `placeholder:document_type:${term}:no-backend`;
      placeholder.tooltip = `Document type "${term}" requires backend connection.`;
      return [placeholder];
    }
    
    try {
      // Use EntityClient to get files by document_type facet
      const entityClient = new GrpcEntityClient(this.extensionContext);
      
      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 15 seconds')), 15000);
      });
      
      console.log(`[TermsFacetTree] Fetching files for document_type: "${term}"`);
      const startTime = Date.now();
      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'document_type',
          term,
          ['file']
        ),
        timeoutPromise
      ]);
      const duration = Date.now() - startTime;
      console.log(`[TermsFacetTree] getEntitiesByFacet completed in ${duration}ms for document_type: "${term}"`);
      
      console.log(`[TermsFacetTree] Received ${entities?.length || 0} entities for document_type: "${term}"`);
      
      if (!entities || entities.length === 0) {
        const placeholder = new TermsFacetTreeItem(
          `No files found for document type "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('info');
        placeholder.id = `placeholder:document_type:${term}:empty`;
        placeholder.tooltip = `No files have document type "${term}" in suggested_taxonomy. Files may need to be reindexed with AI suggestions enabled.`;
        return [placeholder];
      }
      
      // Convert entities to file items
      const files = entities
        .filter((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath);
          if (!relativePath) {
            console.warn(`[TermsFacetTree] Entity missing path:`, e);
          }
          return e.type === 'file' && relativePath;
        })
        .map((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath) || '';
          const absolutePath = path.join(this.workspaceRoot, relativePath);
          
          const item = new TermsFacetTreeItem(
            path.basename(relativePath),
            vscode.TreeItemCollapsibleState.None,
            vscode.Uri.file(absolutePath),
            true
          );
          item.id = `file:document_type:${term}:${relativePath}`;
          item.description = path.dirname(relativePath);
          item.tooltip = `${relativePath}\nDocument Type: ${term}`;
          return item;
        });
      
      if (files.length === 0) {
        console.warn(`[TermsFacetTree] All entities filtered out (missing paths) for document_type: "${term}"`);
        const placeholder = new TermsFacetTreeItem(
          `No valid files found for document type "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('warning');
        placeholder.id = `placeholder:document_type:${term}:invalid`;
        return [placeholder];
      }
      
      // Files are already sorted by backend (most recent first)
      // No need to sort again as TermsFacetTreeItem doesn't have activity data
      console.log(`[TermsFacetTree] Returning ${files.length} files for document_type: "${term}"`);
      return files;
    } catch (error) {
      console.error(`[TermsFacetTree] Failed to get files by document_type from backend:`, error);
      const placeholder = new TermsFacetTreeItem(
        `Error loading document type "${term}": ${(error as Error).message}`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('error');
      placeholder.id = `placeholder:document_type:${term}:error`;
      placeholder.tooltip = `Failed to fetch files: ${(error as Error).message}`;
      return [placeholder];
    }
  }

  private async getFilesByMimeType(term: string): Promise<TermsFacetTreeItem[]> {
    // mime_type comes from enhanced metadata, can use backend or cache
    if (!this.knowledgeClient) {
      throw new Error(`Backend unavailable for MIME type "${term}"`);
    }
    try {
      const entityClient = new GrpcEntityClient(this.extensionContext);

      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 8 seconds')), 8000);
      });

      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'mime_type',
          term,
          ['file']
        ),
        timeoutPromise
      ]);

      if (entities && entities.length > 0) {
        const files = entities
          .filter((e: Entity) => e.type === 'file' && (e.path || e.fileData?.relativePath))
          .map((e: Entity) => {
            const relativePath = e.path || (e.fileData?.relativePath) || '';
            const absolutePath = path.join(this.workspaceRoot, relativePath);
            const item = new TermsFacetTreeItem(
              path.basename(relativePath),
              vscode.TreeItemCollapsibleState.None,
              vscode.Uri.file(absolutePath),
              true
            );
            item.id = `file:mime_type:${term}:${relativePath}`;
            item.description = path.dirname(relativePath);
            item.tooltip = `${relativePath}\nMIME Type: ${term}`;
            return item;
          });
        // Files are already sorted by backend (most recent first)
        // No need to sort again as TermsFacetTreeItem doesn't have activity data
        return files;
      }
      return [];
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`Backend query failed for mime_type "${term}": ${message}`);
    }
  }

  private async getFilesByMimeCategory(term: string): Promise<TermsFacetTreeItem[]> {
    // mime_category comes from enhanced metadata, can use backend or cache
    if (!this.knowledgeClient) {
      throw new Error(`Backend unavailable for MIME category "${term}"`);
    }
    try {
      const entityClient = new GrpcEntityClient(this.extensionContext);

      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 8 seconds')), 8000);
      });

      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'mime_category',
          term,
          ['file']
        ),
        timeoutPromise
      ]);

      if (entities && entities.length > 0) {
        const files = entities
          .filter((e: Entity) => e.type === 'file' && (e.path || e.fileData?.relativePath))
          .map((e: Entity) => {
            const relativePath = e.path || (e.fileData?.relativePath) || '';
            const absolutePath = path.join(this.workspaceRoot, relativePath);
            const item = new TermsFacetTreeItem(
              path.basename(relativePath),
              vscode.TreeItemCollapsibleState.None,
              vscode.Uri.file(absolutePath),
              true
            );
            item.id = `file:mime_category:${term}:${relativePath}`;
            item.description = path.dirname(relativePath);
            item.tooltip = `${relativePath}\nMIME Category: ${term}`;
            return item;
          });
        // Files are already sorted by backend (most recent first)
        // No need to sort again as TermsFacetTreeItem doesn't have activity data
        return files;
      }
      return [];
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(`Backend query failed for mime_category "${term}": ${message}`);
    }
  }

  private async getFilesByFolder(folder: string): Promise<TermsFacetTreeItem[]> {
    const filesCache = await this.fileCacheService.getFiles();
    const matchingFiles = filesCache.filter((file: ActivityFileEntry) => {
      const relativePath = file.relative_path || '';
      return relativePath.startsWith(`${folder}${path.sep}`);
    });

    sortFilesByActivity(matchingFiles);
    return this.buildFileItems(matchingFiles, `folder:${folder}`);
  }

  private async getFilesByTag(term: string): Promise<TermsFacetTreeItem[]> {
    if (!this.metadataStore) {
      return this.getMetadataStorePlaceholder('tags');
    }
    const relativePaths = this.metadataStore.getFilesByTag(term);
    return await this.buildFileItemsForPaths(relativePaths, `tag:${term}`);
  }

  private async getFilesByType(term: string): Promise<TermsFacetTreeItem[]> {
    // Use backend for type classification - frontend should not classify files
    // The backend has the authoritative type classification logic
    if (!this.knowledgeClient) {
      const placeholder = new TermsFacetTreeItem(
        `Backend unavailable for type "${term}"`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('warning');
      placeholder.id = `placeholder:type:${term}:no-backend`;
      placeholder.tooltip = `Type "${term}" requires backend connection.`;
      return [placeholder];
    }
    
    try {
      // Use EntityClient to get files by type facet from backend
      const entityClient = new GrpcEntityClient(this.extensionContext);
      
      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 8 seconds')), 8000);
      });
      
      console.log(`[TermsFacetTree] Fetching files for type: "${term}" from backend`);
      const startTime = Date.now();
      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'type',
          term,
          ['file']
        ),
        timeoutPromise
      ]);
      const duration = Date.now() - startTime;
      console.log(`[TermsFacetTree] getEntitiesByFacet completed in ${duration}ms for type: "${term}"`);
      
      console.log(`[TermsFacetTree] Received ${entities?.length || 0} entities for type: "${term}"`);
      
      if (!entities || entities.length === 0) {
        const placeholder = new TermsFacetTreeItem(
          `No files found for type "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('info');
        placeholder.id = `placeholder:type:${term}:empty`;
        placeholder.tooltip = `No files have type "${term}" according to backend classification.`;
        return [placeholder];
      }
      
      // Convert entities to file items
      const files = entities
        .filter((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath);
          if (!relativePath) {
            console.warn(`[TermsFacetTree] Entity missing path:`, e);
          }
          return e.type === 'file' && relativePath;
        })
        .map((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath) || '';
          const absolutePath = path.join(this.workspaceRoot, relativePath);
          
          const item = new TermsFacetTreeItem(
            path.basename(relativePath),
            vscode.TreeItemCollapsibleState.None,
            vscode.Uri.file(absolutePath),
            true
          );
          item.id = `file:type:${term}:${relativePath}`;
          item.description = path.dirname(relativePath);
          item.tooltip = `${relativePath}\nType: ${term}`;
          return item;
        });
      
      if (files.length === 0) {
        console.warn(`[TermsFacetTree] All entities filtered out (missing paths) for type: "${term}"`);
        const placeholder = new TermsFacetTreeItem(
          `No valid files found for type "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('warning');
        placeholder.id = `placeholder:type:${term}:invalid`;
        return [placeholder];
      }
      
      // Sort by activity (last_modified) if available
      files.sort((a: TermsFacetTreeItem, b: TermsFacetTreeItem) => {
        const aTime = (a as unknown as { lastModified?: number }).lastModified || 0;
        const bTime = (b as unknown as { lastModified?: number }).lastModified || 0;
        return bTime - aTime;
      });
      
      console.log(`[TermsFacetTree] Returning ${files.length} files for type: "${term}"`);
      return files;
    } catch (error) {
      console.error(`[TermsFacetTree] Failed to get files by type from backend:`, error);
      const placeholder = new TermsFacetTreeItem(
        `Error loading type "${term}": ${(error as Error).message}`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('error');
      placeholder.id = `placeholder:type:${term}:error`;
      placeholder.tooltip = `Failed to fetch files: ${(error as Error).message}`;
      return [placeholder];
    }
  }

  private async getFilesByOwner(term: string): Promise<TermsFacetTreeItem[]> {
    // Use EntityClient to get files by owner facet from backend
    const entityClient = new GrpcEntityClient(this.extensionContext);
    const startTime = Date.now();
    try {
      const timeoutPromise = new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('getEntitiesByFacet timeout after 8 seconds')), 8000);
      });

      const entities = await Promise.race([
        entityClient.getEntitiesByFacet(
          this.workspaceId,
          'owner',
          term,
          ['file']
        ),
        timeoutPromise
      ]);
      const duration = Date.now() - startTime;
      console.log(`[TermsFacetTree] getEntitiesByFacet completed in ${duration}ms for owner: "${term}"`);
      
      console.log(`[TermsFacetTree] Received ${entities?.length || 0} entities for owner: "${term}"`);
      
      if (!entities || entities.length === 0) {
        const placeholder = new TermsFacetTreeItem(
          `No files found for owner "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('info');
        placeholder.id = `placeholder:owner:${term}:empty`;
        placeholder.tooltip = `No files have owner "${term}" according to backend.`;
        return [placeholder];
      }
      
      // Convert entities to file items
      const files = entities
        .filter((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath);
          if (!relativePath) {
            console.warn(`[TermsFacetTree] Entity missing path:`, e);
          }
          return e.type === 'file' && relativePath;
        })
        .map((e: Entity) => {
          const relativePath = e.path || (e.fileData?.relativePath) || '';
          const absolutePath = path.join(this.workspaceRoot, relativePath);
          
          const item = new TermsFacetTreeItem(
            path.basename(relativePath),
            vscode.TreeItemCollapsibleState.None,
            vscode.Uri.file(absolutePath),
            true
          );
          item.id = `file:owner:${term}:${relativePath}`;
          item.description = path.dirname(relativePath);
          item.tooltip = `${relativePath}\nOwner: ${term}`;
          return item;
        });
      
      if (files.length === 0) {
        console.warn(`[TermsFacetTree] All entities filtered out (missing paths) for owner: "${term}"`);
        const placeholder = new TermsFacetTreeItem(
          `No valid files found for owner "${term}"`,
          vscode.TreeItemCollapsibleState.None
        );
        placeholder.iconPath = new vscode.ThemeIcon('warning');
        placeholder.id = `placeholder:owner:${term}:invalid`;
        return [placeholder];
      }
      
      // Sort by activity (last_modified) if available
      files.sort((a: TermsFacetTreeItem, b: TermsFacetTreeItem) => {
        const aTime = (a as unknown as { lastModified?: number }).lastModified || 0;
        const bTime = (b as unknown as { lastModified?: number }).lastModified || 0;
        return bTime - aTime;
      });
      
      console.log(`[TermsFacetTree] Returning ${files.length} files for owner: "${term}"`);
      return files;
    } catch (error) {
      console.error(`[TermsFacetTree] Failed to get files by owner from backend:`, error);
      const placeholder = new TermsFacetTreeItem(
        `Error loading owner "${term}": ${(error as Error).message}`,
        vscode.TreeItemCollapsibleState.None
      );
      placeholder.iconPath = new vscode.ThemeIcon('error');
      placeholder.id = `placeholder:owner:${term}:error`;
      placeholder.tooltip = `Failed to fetch files: ${(error as Error).message}`;
      return [placeholder];
    }
  }

  private async getFilesByProjectName(term: string): Promise<TermsFacetTreeItem[]> {
    const projects = await this.knowledgeClient.listProjects(this.workspaceId);
    const project = projects.find((p) => p.name === term);
    if (!project) {
      return [];
    }
    const documents = await this.knowledgeClient.queryDocuments(
      this.workspaceId,
      project.id,
      false
    );
    const relativePaths = documents.map((doc) => doc.path);
    return this.buildFileItemsForPaths(relativePaths, `context:${term}`);
  }

  private async getFilesByMetadataField(term: string, field: string): Promise<TermsFacetTreeItem[]> {
    const filesCache = await this.fileCacheService.getFiles();
    const matchingFiles = filesCache.filter((file: ActivityFileEntry) => {
      const values = this.extractFieldValues(file, field);
      return values.includes(term);
    });

    sortFilesByActivity(matchingFiles);
    return this.buildFileItems(matchingFiles, `${field}:${term}`);
  }

  private buildFileItems(files: ActivityFileEntry[], scope: string): TermsFacetTreeItem[] {
    return files.map((file) => {
      const relativePath = file.relative_path || '';
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);
      const activityTime = getFileActivityTimestamp(file);

      const item = new TermsFacetTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );
      item.id = `file:${scope}:${relativePath}`;

      item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private async buildFileItemsForPaths(relativePaths: string[], scope: string): Promise<TermsFacetTreeItem[]> {
    const filesCache = await this.fileCacheService.getFiles();
    const fileByPath = new Map<string, ActivityFileEntry>();
    for (const file of filesCache) {
      if (file.relative_path) {
        fileByPath.set(file.relative_path, file);
      }
    }

    sortRelativePathsByActivity(relativePaths, fileByPath);
    return relativePaths.map((relativePath) => {
      const absolutePath = path.join(this.workspaceRoot, relativePath);
      const uri = vscode.Uri.file(absolutePath);
      const filename = path.basename(relativePath);
      const activityTime = getFileActivityTimestamp(fileByPath.get(relativePath) || {});

      const item = new TermsFacetTreeItem(
        filename,
        vscode.TreeItemCollapsibleState.None,
        uri,
        true
      );
      item.id = `file:${scope}:${relativePath}`;

      item.tooltip = `${relativePath}\nLast activity: ${new Date(activityTime).toLocaleString()}`;
      item.description = path.dirname(relativePath);

      return item;
    });
  }

  private normalizeExtension(extension: string): string {
    const normalized = extension.trim().toLowerCase();
    return normalized.startsWith('.') ? normalized.slice(1) : normalized;
  }

  private isMetadataFacet(field: string): boolean {
    return [
      'indexing_status',
      'document_author',
      'document_title',
      'document_category',
      'document_creator',
      'document_producer',
      'document_language',
      'image_artist',
      'image_camera',
      'image_format',
      'image_color_space',
      'camera_make',
      'camera_model',
      'image_gps_location',
      'image_orientation',
      'image_has_transparency',
      'image_is_animated',
      'language',
      'content_encoding',
      'readability_level',
      'audio_codec',
      'audio_format',
      'audio_genre',
      'audio_artist',
      'audio_album',
      'audio_year',
      'audio_channels',
      'audio_has_album_art',
      'video_resolution',
      'video_codec',
      'video_audio_codec',
      'video_container',
      'video_aspect_ratio',
      'video_has_subtitles',
      'video_subtitle_languages',
      'video_has_chapters',
      'video_is_3d',
      'video_quality_tier',
      'owner_type',
      'group_category',
      'security_category',
      'security_attributes',
      'has_acls',
      'acl_complexity',
      'access_relation',
      'ownership_pattern',
      'access_frequency',
      'time_category',
      'system_file_type',
      'file_system_category',
      'system_attributes',
      'system_features',
      'filesystem_type',
      'mount_point',
      'permission_level',
    ].includes(field);
  }

  private extractFieldValues(file: ActivityFileEntry, field: string): string[] {
    const enhanced = (file as {
      enhanced?: {
        mime_type?: { mime_type?: string; category?: string; encoding?: string };
        indexed?: {
          basic?: boolean;
          mime?: boolean;
          code?: boolean;
          document?: boolean;
          mirror?: boolean;
        };
        document_metrics?: {
          author?: string;
          title?: string;
          category?: string;
          creator?: string;
          producer?: string;
          xmp_language?: string[] | string;
        };
        language?: string;
        image_metadata?: {
          artist?: string;
          camera_make?: string;
          camera_model?: string;
          exif_artist?: string;
          exif_camera_make?: string;
          exif_camera_model?: string;
          format?: string;
          color_space?: string;
          gps_location?: string;
          orientation?: number;
          has_transparency?: boolean;
          is_animated?: boolean;
        };
        audio_metadata?: {
          artist?: string;
          album?: string;
          genre?: string;
          year?: number;
          id3_artist?: string;
          vorbis_artist?: string;
          id3_album?: string;
          vorbis_album?: string;
          id3_genre?: string;
          vorbis_genre?: string;
          id3_year?: number;
          vorbis_date?: string;
          channels?: number;
          codec?: string;
          format?: string;
          has_album_art?: boolean;
        };
        video_metadata?: {
          width?: number;
          height?: number;
          codec?: string;
          video_codec?: string;
          audio_codec?: string;
          container?: string;
          aspect_ratio?: string;
          video_aspect_ratio?: string;
          has_subtitles?: boolean;
          subtitle_tracks?: string[];
          subtitle_languages?: string[];
          has_chapters?: boolean;
          is_3d?: boolean;
          is_hd?: boolean;
          is_4k?: boolean;
        };
        content_encoding?: string;
        os_metadata?: {
          file_system?: {
            mount_point?: string;
            fs_type?: string;
            file_system_type?: string;
          };
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
        os_taxonomy?: {
          security?: {
            permission_level?: string;
            security_flags?: string[];
            access_pattern?: string;
          };
          ownership?: {
            owner_type?: string;
            group_category?: string;
            sharing_scope?: string;
          };
          temporal?: {
            access_frequency?: string;
            age_category?: string;
            staleness_category?: string;
          };
          system?: {
            file_type_category?: string;
            fs_category?: string;
            system_attributes?: string[];
          };
        };
        content_quality?: {
          readability_level?: string;
        };
      };
    }).enhanced;

    const relativePath = file.relative_path || '';

    switch (field) {
      case 'extension': {
        const extValue = (file as { extension?: string }).extension;
        const ext = this.normalizeExtension(extValue || path.extname(relativePath));
        return ext ? [ext] : [];
      }
      case 'document_type': {
        // document_type comes from suggested_taxonomy (AI-suggested), not available in cache
        // Return empty array - will be populated from backend when needed
        return [];
      }
      case 'mime_type': {
        const mimeType = enhanced?.mime_type?.mime_type;
        return mimeType ? [mimeType] : [];
      }
      case 'mime_category': {
        const mimeCategory = enhanced?.mime_type?.category;
        return mimeCategory ? [mimeCategory] : [];
      }
      case 'indexing_status': {
        const indexed = enhanced?.indexed || {};
        const basic = indexed.basic;
        const mime = indexed.mime;
        const code = indexed.code;
        const document = indexed.document;
        const mirror = indexed.mirror;
        if (basic && mime && code && document && mirror) return ['complete'];
        if (basic && mime && code && document) return ['document_complete'];
        if (basic && mime && code) return ['code_complete'];
        if (basic && mime) return ['mime_complete'];
        if (basic) return ['basic_only'];
        return ['not_indexed'];
      }
      case 'folder': {
        const parts = relativePath.split(path.sep);
        return parts.length > 1 ? [parts[0]] : [];
      }
      case 'document_author':
        return enhanced?.document_metrics?.author ? [enhanced.document_metrics.author] : [];
      case 'document_title':
        return enhanced?.document_metrics?.title ? [enhanced.document_metrics.title] : [];
      case 'document_category':
        return enhanced?.document_metrics?.category ? [enhanced.document_metrics.category] : [];
      case 'document_creator':
        return enhanced?.document_metrics?.creator ? [enhanced.document_metrics.creator] : [];
      case 'document_producer':
        return enhanced?.document_metrics?.producer ? [enhanced.document_metrics.producer] : [];
      case 'document_language': {
        const lang = enhanced?.document_metrics?.xmp_language;
        if (Array.isArray(lang)) {
          return lang.filter(Boolean);
        }
        return lang ? [lang] : [];
      }
      case 'language':
        return enhanced?.language ? [enhanced.language] : [];
      case 'image_artist': {
        const artist = enhanced?.image_metadata?.artist || enhanced?.image_metadata?.exif_artist;
        return artist ? [artist] : [];
      }
      case 'image_camera': {
        const make = enhanced?.image_metadata?.camera_make ?? enhanced?.image_metadata?.exif_camera_make;
        const model = enhanced?.image_metadata?.camera_model ?? enhanced?.image_metadata?.exif_camera_model;
        if (make && model) {
          return [`${make} ${model}`];
        }
        if (make) {
          return [make];
        }
        return model ? [model] : [];
      }
      case 'image_format':
        return enhanced?.image_metadata?.format ? [enhanced.image_metadata.format] : [];
      case 'image_color_space':
        return enhanced?.image_metadata?.color_space ? [enhanced.image_metadata.color_space] : [];
      case 'camera_make':
        return enhanced?.image_metadata?.exif_camera_make ? [enhanced.image_metadata.exif_camera_make] : [];
      case 'camera_model':
        return enhanced?.image_metadata?.exif_camera_model ? [enhanced.image_metadata.exif_camera_model] : [];
      case 'image_gps_location':
        return enhanced?.image_metadata?.gps_location ? [enhanced.image_metadata.gps_location] : [];
      case 'image_orientation': {
        const orientation = enhanced?.image_metadata?.orientation;
        return orientation ? [String(orientation)] : [];
      }
      case 'image_has_transparency': {
        const hasTransparency = enhanced?.image_metadata?.has_transparency;
        if (hasTransparency === undefined) {
          return [];
        }
        return [String(hasTransparency)];
      }
      case 'image_is_animated': {
        const isAnimated = enhanced?.image_metadata?.is_animated;
        if (isAnimated === undefined) {
          return [];
        }
        return [String(isAnimated)];
      }
      case 'content_encoding': {
        const encoding = enhanced?.content_encoding || enhanced?.mime_type?.encoding;
        return encoding ? [encoding] : [];
      }
      case 'audio_codec':
        return enhanced?.audio_metadata?.codec ? [enhanced.audio_metadata.codec] : [];
      case 'audio_format':
        return enhanced?.audio_metadata?.format ? [enhanced.audio_metadata.format] : [];
      case 'audio_genre': {
        const genre = enhanced?.audio_metadata?.genre || enhanced?.audio_metadata?.id3_genre || enhanced?.audio_metadata?.vorbis_genre;
        return genre ? [genre] : [];
      }
      case 'audio_artist': {
        const artist = enhanced?.audio_metadata?.artist || enhanced?.audio_metadata?.id3_artist || enhanced?.audio_metadata?.vorbis_artist;
        return artist ? [artist] : [];
      }
      case 'audio_album': {
        const album = enhanced?.audio_metadata?.album || enhanced?.audio_metadata?.id3_album || enhanced?.audio_metadata?.vorbis_album;
        return album ? [album] : [];
      }
      case 'audio_year': {
        const year = enhanced?.audio_metadata?.year ?? enhanced?.audio_metadata?.id3_year ?? enhanced?.audio_metadata?.vorbis_date;
        return year ? [String(year)] : [];
      }
      case 'audio_channels': {
        const channels = enhanced?.audio_metadata?.channels;
        const label = channels ? this.getAudioChannelsLabel(channels) : null;
        return label ? [label] : [];
      }
      case 'audio_has_album_art': {
        const hasAlbumArt = enhanced?.audio_metadata?.has_album_art;
        if (hasAlbumArt === undefined) {
          return [];
        }
        return [String(hasAlbumArt)];
      }
      case 'video_resolution': {
        const width = enhanced?.video_metadata?.width ?? 0;
        const height = enhanced?.video_metadata?.height ?? 0;
        const label = this.getVideoResolutionLabel(width, height);
        return label ? [label] : [];
      }
      case 'video_codec': {
        const codec = enhanced?.video_metadata?.video_codec || enhanced?.video_metadata?.codec;
        return codec ? [codec] : [];
      }
      case 'video_audio_codec':
        return enhanced?.video_metadata?.audio_codec ? [enhanced.video_metadata.audio_codec] : [];
      case 'video_container':
        return enhanced?.video_metadata?.container ? [enhanced.video_metadata.container] : [];
      case 'video_aspect_ratio': {
        const aspectRatio = enhanced?.video_metadata?.aspect_ratio || enhanced?.video_metadata?.video_aspect_ratio;
        return aspectRatio ? [aspectRatio] : [];
      }
      case 'video_has_subtitles': {
        const hasSubtitles = enhanced?.video_metadata?.has_subtitles;
        if (hasSubtitles === undefined) {
          return [];
        }
        return [String(hasSubtitles)];
      }
      case 'video_subtitle_languages': {
        const tracks = enhanced?.video_metadata?.subtitle_languages || enhanced?.video_metadata?.subtitle_tracks || [];
        return tracks.filter(Boolean);
      }
      case 'video_has_chapters': {
        const hasChapters = enhanced?.video_metadata?.has_chapters;
        if (hasChapters === undefined) {
          return [];
        }
        return [String(hasChapters)];
      }
      case 'video_is_3d': {
        const is3d = enhanced?.video_metadata?.is_3d;
        if (is3d === undefined) {
          return [];
        }
        return [String(is3d)];
      }
      case 'video_quality_tier': {
        const width = enhanced?.video_metadata?.width ?? 0;
        const height = enhanced?.video_metadata?.height ?? 0;
        const label = this.getVideoQualityTierLabel(
          width,
          height,
          enhanced?.video_metadata?.is_hd,
          enhanced?.video_metadata?.is_4k
        );
        return label ? [label] : [];
      }
      case 'owner': {
        // Try to get owner from multiple sources:
        // 1. Direct owner field on file (from entity)
        const directOwner = (file as { owner?: string }).owner;
        if (directOwner) {
          return [directOwner];
        }
        // 2. From os_metadata.owner.username (runtime data, may not be in type)
        const osOwner = (enhanced?.os_metadata as { owner?: { username?: string } })?.owner?.username;
        if (osOwner) {
          return [osOwner];
        }
        return [];
      }
      case 'owner_type': {
        const ownerType = enhanced?.os_context_taxonomy?.ownership?.owner_type || enhanced?.os_taxonomy?.ownership?.owner_type;
        return ownerType ? [ownerType] : [];
      }
      case 'group_category': {
        const groupCategory = enhanced?.os_context_taxonomy?.ownership?.group_category || enhanced?.os_taxonomy?.ownership?.group_category;
        return groupCategory ? [groupCategory] : [];
      }
      case 'security_category':
        return enhanced?.os_context_taxonomy?.security?.security_category
          || enhanced?.os_taxonomy?.security?.security_flags
          || [];
      case 'security_attributes':
        return enhanced?.os_context_taxonomy?.security?.security_attributes
          || enhanced?.os_taxonomy?.security?.security_flags
          || [];
      case 'has_acls': {
        const hasAcls = enhanced?.os_context_taxonomy?.security?.has_acls;
        if (hasAcls === undefined) {
          return [];
        }
        return [String(hasAcls)];
      }
      case 'acl_complexity':
        return enhanced?.os_context_taxonomy?.security?.acl_complexity
          ? [enhanced.os_context_taxonomy.security.acl_complexity]
          : [];
      case 'access_relation':
        return enhanced?.os_context_taxonomy?.ownership?.access_relations || [];
      case 'ownership_pattern':
        return enhanced?.os_context_taxonomy?.ownership?.ownership_pattern
          ? [enhanced.os_context_taxonomy.ownership.ownership_pattern]
          : [];
      case 'access_frequency': {
        const accessFrequency = enhanced?.os_context_taxonomy?.temporal?.access_frequency || enhanced?.os_taxonomy?.temporal?.access_frequency;
        return accessFrequency ? [accessFrequency] : [];
      }
      case 'time_category':
        {
          const categories = enhanced?.os_context_taxonomy?.temporal?.time_category || [];
          if (categories.length > 0) {
            return categories;
          }
          const derived: string[] = [];
          if (enhanced?.os_taxonomy?.temporal?.age_category) {
            derived.push(enhanced.os_taxonomy.temporal.age_category);
          }
          if (enhanced?.os_taxonomy?.temporal?.staleness_category) {
            derived.push(enhanced.os_taxonomy.temporal.staleness_category);
          }
          return derived;
        }
      case 'system_file_type': {
        const systemFileType = enhanced?.os_context_taxonomy?.system?.system_file_type || enhanced?.os_taxonomy?.system?.file_type_category;
        return systemFileType ? [systemFileType] : [];
      }
      case 'file_system_category': {
        const fileSystemCategory = enhanced?.os_context_taxonomy?.system?.file_system_category || enhanced?.os_taxonomy?.system?.fs_category;
        return fileSystemCategory ? [fileSystemCategory] : [];
      }
      case 'system_attributes':
        return enhanced?.os_context_taxonomy?.system?.system_attributes
          || enhanced?.os_taxonomy?.system?.system_attributes
          || [];
      case 'system_features':
        return enhanced?.os_context_taxonomy?.system?.system_features || [];
      case 'filesystem_type': {
        const filesystemType = enhanced?.os_metadata?.file_system?.file_system_type || enhanced?.os_metadata?.file_system?.fs_type;
        return filesystemType ? [filesystemType] : [];
      }
      case 'mount_point':
        return enhanced?.os_metadata?.file_system?.mount_point
          ? [enhanced.os_metadata.file_system.mount_point]
          : [];
      case 'permission_level': {
        const level = enhanced?.os_context_taxonomy?.security?.permission_level
          || enhanced?.os_taxonomy?.security?.permission_level;
        return level ? [level] : [];
      }
      case 'readability_level':
        return enhanced?.content_quality?.readability_level
          ? [enhanced.content_quality.readability_level]
          : [];
      default:
        return [];
    }
  }

  private getVideoResolutionLabel(width: number, height: number): string | null {
    if (!width || !height) {
      return null;
    }
    const maxDim = Math.max(width, height);
    if (maxDim >= 2160) return '4K+';
    if (maxDim >= 1440) return '1440p';
    if (maxDim >= 1080) return '1080p';
    if (maxDim >= 720) return '720p';
    if (maxDim >= 480) return '480p';
    return 'SD (< 480p)';
  }

  private getAudioChannelsLabel(channels: number): string | null {
    if (!channels || channels <= 0) {
      return null;
    }
    if (channels === 1) return 'mono';
    if (channels === 2) return 'stereo';
    if (channels === 6) return '5.1';
    if (channels === 7) return '6.1';
    if (channels === 8) return '7.1';
    return `${channels} channels`;
  }

  private getVideoQualityTierLabel(
    width: number,
    height: number,
    isHD?: boolean,
    is4K?: boolean
  ): string | null {
    if (is4K) {
      return '4K';
    }
    const maxDim = Math.max(width, height);
    if (maxDim >= 2160) {
      return '4K';
    }
    if (isHD || maxDim >= 720) {
      return 'HD';
    }
    if (maxDim > 0) {
      return 'SD';
    }
    return null;
  }

  private getMetadataStorePlaceholder(label: string): TermsFacetTreeItem[] {
    const placeholder = new TermsFacetTreeItem(
      t('metadataStoreUnavailable', { label }),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('info');
    placeholder.id = `placeholder:metadata:${label}`;
    return [placeholder];
  }

  private getEmptyPlaceholder(field?: string): TermsFacetTreeItem[] {
    const fieldLabel = field ? String(field) : this.facetField;
    const placeholder = new TermsFacetTreeItem(
      t('noFacetData', { field: fieldLabel }),
      vscode.TreeItemCollapsibleState.None
    );
    placeholder.iconPath = new vscode.ThemeIcon('info');
    placeholder.tooltip = t('noFacetDataTooltip', { field: fieldLabel });
    placeholder.id = `placeholder:empty:${fieldLabel}`;
    // Explicitly set term and field to undefined to prevent it from being treated as a valid term
    (placeholder as any).term = undefined;
    (placeholder as any).field = undefined;
    return [placeholder];
  }
}
