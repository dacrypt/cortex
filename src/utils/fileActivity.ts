export type ActivityFileEntry = {
  relative_path?: string;
  last_modified?: number;
  created_at?: number;
  accessed_at?: number;
  changed_at?: number;
  file_size?: number;
  enhanced?: {
    stats?: {
      modified?: number;
      accessed?: number;
      created?: number;
      changed?: number;
      size?: number;
    };
    language?: string;
    code_metrics?: {
      complexity?: number;
      function_count?: number;
      lines_of_code?: number;
      comment_percentage?: number;
    };
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
      has_subtitles?: boolean;
      subtitle_tracks?: string[];
      has_chapters?: boolean;
      is_3d?: boolean;
      is_hd?: boolean;
      is_4k?: boolean;
    };
    content_quality?: {
      quality_score?: number;
    };
    content_encoding?: string;
    language_confidence?: number;
    os_metadata?: {
      file_system?: {
        mount_point?: string;
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
  };
};

export function getFileActivityTimestamp(file: ActivityFileEntry): number {
  const lastModified = Number(file.last_modified || 0);
  const createdAt = Number(file.created_at || 0);
  const statsModified = Number(file.enhanced?.stats?.modified || 0);
  const statsAccessed = Number(file.enhanced?.stats?.accessed || 0);
  const statsCreated = Number(file.enhanced?.stats?.created || 0);
  return Math.max(lastModified, statsModified, statsAccessed, createdAt, statsCreated);
}

export function sortFilesByActivity<T extends ActivityFileEntry>(files: T[]): T[] {
  return files.sort((a, b) => getFileActivityTimestamp(b) - getFileActivityTimestamp(a));
}

export function sortRelativePathsByActivity(
  paths: string[],
  fileByPath: Map<string, ActivityFileEntry>
): string[] {
  return paths.sort((a, b) => {
    const aTime = getFileActivityTimestamp(fileByPath.get(a) || {});
    const bTime = getFileActivityTimestamp(fileByPath.get(b) || {});
    return bTime - aTime;
  });
}
