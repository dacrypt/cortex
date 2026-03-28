import { GrpcAdminClient } from "./GrpcAdminClient";

type FileEntry = {
  relative_path?: string;
  filename?: string;
  extension?: string;
  file_size?: number;
  last_modified?: number;
  created_at?: number;
  accessed_at?: number;
  changed_at?: number;
  project_score?: number;
  assignment_score?: number;
  enhanced?: {
    stats?: {
      size?: number;
      modified?: number;
      accessed?: number;
      created?: number;
      changed?: number;
    };
    language?: string;
    mime_type?: {
      mime_type?: string;
      category?: string;
      encoding?: string;
    };
    indexed?: {
      basic?: boolean;
      mime?: boolean;
      code?: boolean;
      document?: boolean;
      mirror?: boolean;
    };
    code_metrics?: any;
    document_metrics?: any;
    image_metadata?: {
      width?: number;
      height?: number;
      color_depth?: number;
      format?: string;
      color_space?: string;
      camera_make?: string;
      camera_model?: string;
      exif_camera_make?: string;
      exif_camera_model?: string;
      gps_location?: string;
      orientation?: number;
      has_transparency?: boolean;
      is_animated?: boolean;
      iso?: number;
      f_number?: number;
      focal_length?: number;
      exif_iso?: number;
      exif_fnumber?: number;
      exif_focal_length?: number;
      artist?: string;
    };
    audio_metadata?: {
      duration?: number;
      bitrate?: number;
      sample_rate?: number;
      channels?: number;
      codec?: string;
      format?: string;
      title?: string;
      artist?: string;
      album?: string;
      year?: number;
      genre?: string;
      id3_genre?: string;
      vorbis_genre?: string;
      id3_artist?: string;
      vorbis_artist?: string;
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
      aspect_ratio?: string;
      has_subtitles?: boolean;
      subtitle_tracks?: string[];
      subtitle_languages?: string[];
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
      quality_score?: number;
      readability_level?: string;
    };
    content_encoding?: string;
    language_confidence?: number;
    os_metadata?: {
      file_system?: {
        mount_point?: string;
        file_system_type?: string;
      };
    };
    error?: {
      code?: string;
      message?: string;
    };
  };
};

export class FileCacheService {
  private static instance: FileCacheService | undefined;
  private cache: FileEntry[] = [];
  private cacheTimestamp = 0;
  private readonly CACHE_TTL = 30000; // 30 seconds
  private refreshPromise: Promise<FileEntry[]> | null = null;
  private workspaceId?: string;

  private constructor(private adminClient: GrpcAdminClient) {}

  static getInstance(adminClient: GrpcAdminClient): FileCacheService {
    if (!FileCacheService.instance) {
      FileCacheService.instance = new FileCacheService(adminClient);
    }
    return FileCacheService.instance;
  }

  setWorkspaceId(workspaceId: string): void {
    if (this.workspaceId !== workspaceId) {
      // Clear cache when workspace changes
      this.cache = [];
      this.cacheTimestamp = 0;
    }
    this.workspaceId = workspaceId;
  }

  /**
   * Get files from cache or trigger a refresh if needed.
   * Multiple concurrent calls will share the same refresh promise.
   */
  async getFiles(): Promise<FileEntry[]> {
    const now = Date.now();
    
    // Return cached data if still valid
    if (this.cache.length > 0 && now - this.cacheTimestamp < this.CACHE_TTL) {
      return this.cache;
    }

    // If a refresh is already in progress, wait for it
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    // Start a new refresh
    this.refreshPromise = this.refreshCache();
    try {
      const files = await this.refreshPromise;
      return files;
    } finally {
      this.refreshPromise = null;
    }
  }

  /**
   * Force a cache refresh (ignores TTL)
   */
  async refresh(): Promise<FileEntry[]> {
    // Cancel any pending refresh
    this.refreshPromise = null;
    this.cacheTimestamp = 0; // Force refresh
    return this.getFiles();
  }

  /**
   * Clear the cache
   */
  clear(): void {
    this.cache = [];
    this.cacheTimestamp = 0;
    this.refreshPromise = null;
  }

  /**
   * Get current cache (may be stale)
   */
  getCachedFiles(): FileEntry[] {
    return this.cache;
  }

  /**
   * Check if cache is valid
   */
  isCacheValid(): boolean {
    const now = Date.now();
    return this.cache.length > 0 && now - this.cacheTimestamp < this.CACHE_TTL;
  }

  private async refreshCache(): Promise<FileEntry[]> {
    if (!this.workspaceId) {
      console.warn("[FileCacheService] No workspace ID set");
      return [];
    }

    try {
      const files = await this.adminClient.listFiles(this.workspaceId);
      this.cache = files || [];
      this.cacheTimestamp = Date.now();
      console.log(
        `[FileCacheService] Refreshed cache: ${this.cache.length} files`
      );
      return this.cache;
    } catch (error) {
      console.warn(
        `[FileCacheService] Failed to refresh cache: ${error}`
      );
      // Return stale cache if available, otherwise empty array
      return this.cache.length > 0 ? this.cache : [];
    }
  }
}
