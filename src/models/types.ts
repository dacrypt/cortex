/**
 * Core type definitions for Cortex
 */

export interface EnhancedMetadata {
  mimeType?: string;
  fileType?: string;
  lineCount?: number;
  language?: string;
  codeMetrics?: {
    linesOfCode?: number;
    commentLines?: number;
    blankLines?: number;
    functionCount?: number;
    classCount?: number;
    complexity?: number;
  };
  documentMetrics?: {
    // Basic metrics
    pageCount?: number;
    wordCount?: number;
    characterCount?: number;
    
    // PDF Info Dictionary
    author?: string;
    title?: string;
    subject?: string;
    keywords?: string[];
    creator?: string;
    producer?: string;
    createdDate?: number; // Unix timestamp
    modifiedDate?: number; // Unix timestamp
    trapped?: string;
    
    // XMP Metadata
    xmpTitle?: string;
    xmpDescription?: string;
    xmpCreator?: string[];
    xmpContributor?: string[];
    xmpRights?: string;
    xmpRightsOwner?: string[];
    xmpCopyright?: string;
    xmpCopyrightURL?: string;
    xmpIdentifier?: string[];
    xmpLanguage?: string[];
    xmpRating?: number;
    xmpMetadataDate?: number;
    xmpModifyDate?: number;
    xmpCreateDate?: number;
    xmpNickname?: string;
    xmpLabel?: string[];
    xmpMarked?: boolean;
    xmpUsageTerms?: string;
    xmpWebStatement?: string;
    
    // PDF technical metadata
    pdfVersion?: string;
    pdfEncrypted?: boolean;
    pdfLinearized?: boolean;
    pdfTagged?: boolean;
    pdfPageLayout?: string;
    pdfPageMode?: string;
    
    // Additional properties
    company?: string;
    category?: string;
    comments?: string;
    hyperlinks?: string[];
    fonts?: string[];
    colorSpace?: string[];
    imageCount?: number;
    formFields?: number;
    annotations?: number;
    
    // Custom properties
    customProperties?: Record<string, string>;
  };
  imageMetadata?: {
    width?: number;
    height?: number;
    colorDepth?: number;
    colorSpace?: string;
    format?: string;
    orientation?: number;
    // EXIF
    exifCameraMake?: string;
    exifCameraModel?: string;
    exifSoftware?: string;
    exifDateTimeOriginal?: number;
    exifDateTimeDigitized?: number;
    exifDateTimeModified?: number;
    exifArtist?: string;
    exifCopyright?: string;
    exifImageDescription?: string;
    exifUserComment?: string;
    exifFNumber?: number;
    exifExposureTime?: string;
    exifISO?: number;
    exifFocalLength?: number;
    exifFocalLength35mm?: number;
    exifExposureMode?: string;
    exifWhiteBalance?: string;
    exifFlash?: string;
    exifMeteringMode?: string;
    exifExposureProgram?: string;
    // GPS
    gpsLatitude?: number;
    gpsLongitude?: number;
    gpsAltitude?: number;
    gpsLatitudeRef?: string;
    gpsLongitudeRef?: string;
    gpsAltitudeRef?: string;
    gpsLocation?: string;
    // IPTC
    iptcObjectName?: string;
    iptcCaption?: string;
    iptcKeywords?: string[];
    iptcCopyrightNotice?: string;
    iptcByline?: string;
    iptcBylineTitle?: string;
    iptcHeadline?: string;
    iptcContact?: string;
    iptcContactCity?: string;
    iptcContactCountry?: string;
    iptcContactEmail?: string;
    iptcContactPhone?: string;
    iptcContactWebsite?: string;
    iptcSource?: string;
    iptcUsageTerms?: string;
    // XMP
    xmpTitle?: string;
    xmpDescription?: string;
    xmpCreator?: string[];
    xmpRights?: string;
    xmpRating?: number;
    xmpLabel?: string[];
    xmpSubject?: string[];
    // Analysis
    dominantColors?: string[];
    hasTransparency?: boolean;
    isAnimated?: boolean;
    frameCount?: number;
  };
  audioMetadata?: {
    duration?: number;
    bitrate?: number;
    sampleRate?: number;
    channels?: number;
    bitDepth?: number;
    codec?: string;
    format?: string;
    // ID3
    id3Title?: string;
    id3Artist?: string;
    id3Album?: string;
    id3Year?: number;
    id3Genre?: string;
    id3Track?: number;
    id3Disc?: number;
    id3Composer?: string;
    id3Conductor?: string;
    id3Performer?: string;
    id3Publisher?: string;
    id3Comment?: string;
    id3Lyrics?: string;
    id3BPM?: number;
    id3ISRC?: string;
    id3Copyright?: string;
    id3EncodedBy?: string;
    id3AlbumArtist?: string;
    // Vorbis
    vorbisTitle?: string;
    vorbisArtist?: string;
    vorbisAlbum?: string;
    vorbisDate?: string;
    vorbisGenre?: string;
    vorbisTrack?: string;
    vorbisComment?: string;
    // Technical
    hasAlbumArt?: boolean;
    albumArtFormat?: string;
    albumArtSize?: number;
    replayGain?: number;
    normalized?: boolean;
    lossless?: boolean;
  };
  videoMetadata?: {
    duration?: number;
    width?: number;
    height?: number;
    frameRate?: number;
    bitrate?: number;
    codec?: string;
    container?: string;
    videoCodec?: string;
    videoBitrate?: number;
    videoPixelFormat?: string;
    videoColorSpace?: string;
    videoAspectRatio?: string;
    audioCodec?: string;
    audioBitrate?: number;
    audioSampleRate?: number;
    audioChannels?: number;
    audioLanguage?: string;
    title?: string;
    artist?: string;
    album?: string;
    genre?: string;
    year?: number;
    director?: string;
    producer?: string;
    copyright?: string;
    description?: string;
    comment?: string;
    hasSubtitles?: boolean;
    subtitleTracks?: string[];
    hasChapters?: boolean;
    chapterCount?: number;
    is3D?: boolean;
    isHD?: boolean;
    is4K?: boolean;
  };
}

export interface MirrorMetadata {
  format: 'md' | 'csv';
  path: string;
  sourceMtime: number;
  updatedAt: number;
}

/**
 * In-memory index entry for a workspace file
 */
export interface FileIndexEntry {
  absolutePath: string;
  relativePath: string;
  filename: string;
  extension: string;
  lastModified: number; // Unix timestamp
  fileSize: number; // bytes
  enhanced?: EnhancedMetadata; // Rich metadata
}

/**
 * Persistent metadata for a file
 */
export interface FileMetadata {
  file_id: string; // Stable hash of relative path
  relativePath: string; // For reference
  tags: string[];
  contexts: string[]; // Projects, clients, cases, etc.
  suggestedContexts?: string[]; // Suggested projects (not yet confirmed)
  
  // New suggested metadata structure
  suggestedMetadata?: SuggestedMetadata;
  type: string; // Inferred from extension (ts, pdf, md, etc.)
  notes?: string;
  aiSummary?: string;
  aiSummaryHash?: string;
  aiKeyTerms?: string[];
  aiCategory?: string;
  aiCategoryConfidence?: number;
  aiRelated?: Array<{ relativePath: string; similarity?: number; reason?: string }>;
  mirror?: MirrorMetadata;
  created_at: number; // Unix timestamp
  updated_at: number; // Unix timestamp
}

/**
 * Tree item types for virtual views
 */
export enum CortexTreeItemType {
  Context = 'context',
  Tag = 'tag',
  FileType = 'fileType',
  File = 'file',
}

/**
 * Data structure for tree items
 */
export interface CortexTreeItem {
  type: CortexTreeItemType;
  label: string;
  filePath?: string; // Only for File type
  children?: CortexTreeItem[];
}

/**
 * Suggested metadata structure (AI-generated suggestions)
 */
export interface SuggestedMetadata {
  fileId: string;
  relativePath: string;
  suggestedTags: SuggestedTag[];
  suggestedProjects: SuggestedProject[];
  suggestedTaxonomy?: SuggestedTaxonomy;
  suggestedFields: SuggestedField[];
  confidence: number;
  source: string; // "rag", "llm", "metadata"
  generatedAt: number; // Unix timestamp
  updatedAt: number; // Unix timestamp
}

export interface SuggestedTag {
  tag: string;
  confidence: number;
  reason: string;
  source: string; // "rag", "llm", "metadata"
  category?: string;
}

export interface SuggestedProject {
  projectId?: string;
  projectName: string;
  confidence: number;
  reason: string;
  source: string; // "rag", "llm", "metadata"
  isNew: boolean; // Whether this is a new project suggestion
}

export interface SuggestedTaxonomy {
  category?: string;
  subcategory?: string;
  domain?: string;
  subdomain?: string;
  contentType?: string;
  purpose?: string;
  topic?: string[];
  audience?: string;
  language?: string;
  categoryConfidence?: number;
  domainConfidence?: number;
  contentTypeConfidence?: number;
  reasoning?: string;
  source?: string;
}

export interface SuggestedField {
  fieldName: string;
  value: unknown; // Can be string, number, array, etc.
  fieldType: string; // "string", "number", "array", "date", etc.
  confidence: number;
  reason: string;
  source: string;
}
