package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
)

func TestFileRepositoryNumericRangeFacets(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "cortex.sqlite")
	conn, err := NewConnection(dbPath)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	if err := conn.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	workspace := entity.NewWorkspace("/tmp/workspace", "test-workspace")
	workspaceRepo := NewWorkspaceRepository(conn)
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	repo := NewFileRepository(conn)
	now := time.Now()
	root := t.TempDir()

	imageSmall := entity.NewFileEntry(root, "images/small.jpg", 120, now)
	imageSmall.Enhanced = &entity.EnhancedMetadata{
		ImageMetadata: &entity.ImageMetadata{
			Width:           800,
			Height:          600,
			ColorDepth:      8,
			Format:          stringPtr("JPEG"),
			ColorSpace:      stringPtr("RGB"),
			EXIFCameraMake:  stringPtr("Canon"),
			EXIFCameraModel: stringPtr("5D"),
			EXIFISO:         intPtr(100),
			EXIFFNumber:     floatPtr(2.0),
			EXIFFocalLength: floatPtr(24.0),
			GPSLocation:     stringPtr("NYC"),
			Orientation:     intPtr(1),
			HasTransparency: boolPtr(false),
			IsAnimated:      boolPtr(false),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	imageMedium := entity.NewFileEntry(root, "images/medium.jpg", 220, now)
	imageMedium.Enhanced = &entity.EnhancedMetadata{
		ImageMetadata: &entity.ImageMetadata{
			Width:           1920,
			Height:          1080,
			ColorDepth:      24,
			Format:          stringPtr("PNG"),
			ColorSpace:      stringPtr("RGB"),
			EXIFCameraMake:  stringPtr("Sony"),
			EXIFCameraModel: stringPtr("A7"),
			EXIFISO:         intPtr(800),
			EXIFFNumber:     floatPtr(4.0),
			EXIFFocalLength: floatPtr(85.0),
			GPSLocation:     stringPtr("LA"),
			HasTransparency: boolPtr(true),
			IsAnimated:      boolPtr(false),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	imageLarge := entity.NewFileEntry(root, "images/large.jpg", 320, now)
	imageLarge.Enhanced = &entity.EnhancedMetadata{
		ImageMetadata: &entity.ImageMetadata{
			Width:           4000,
			Height:          3000,
			ColorDepth:      32,
			Format:          stringPtr("GIF"),
			ColorSpace:      stringPtr("Indexed"),
			EXIFCameraMake:  stringPtr("Canon"),
			EXIFCameraModel: stringPtr("R5"),
			EXIFISO:         intPtr(6400),
			EXIFFNumber:     floatPtr(16.0),
			EXIFFocalLength: floatPtr(300.0),
			Orientation:     intPtr(6),
			HasTransparency: boolPtr(true),
			IsAnimated:      boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}

	videoHD := entity.NewFileEntry(root, "video/hd.mp4", 120, now)
	videoHD.Enhanced = &entity.EnhancedMetadata{
		VideoMetadata: &entity.VideoMetadata{
			Width:            1280,
			Height:           720,
			Duration:         floatPtr(240),
			Bitrate:          intPtr(800),
			FrameRate:        floatPtr(23),
			VideoCodec:       stringPtr("H.264"),
			AudioCodec:       stringPtr("AAC"),
			Container:        stringPtr("MP4"),
			VideoAspectRatio: stringPtr("16:9"),
			HasSubtitles:     boolPtr(true),
			SubtitleTracks:   []string{"en", "es"},
			HasChapters:      boolPtr(true),
			Is3D:             boolPtr(false),
			IsHD:             boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	videoFullHD := entity.NewFileEntry(root, "video/fullhd.mp4", 220, now)
	videoFullHD.Enhanced = &entity.EnhancedMetadata{
		VideoMetadata: &entity.VideoMetadata{
			Width:            1920,
			Height:           1080,
			Duration:         floatPtr(1200),
			Bitrate:          intPtr(3500),
			FrameRate:        floatPtr(29),
			Codec:            stringPtr("H.265"),
			AudioCodec:       stringPtr("AAC"),
			Container:        stringPtr("MKV"),
			VideoAspectRatio: stringPtr("16:9"),
			HasSubtitles:     boolPtr(false),
			HasChapters:      boolPtr(false),
			Is3D:             boolPtr(false),
			IsHD:             boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	video4K := entity.NewFileEntry(root, "video/4k.mp4", 320, now)
	video4K.Enhanced = &entity.EnhancedMetadata{
		VideoMetadata: &entity.VideoMetadata{
			Width:            3840,
			Height:           2160,
			Duration:         floatPtr(7200),
			Bitrate:          intPtr(12000),
			FrameRate:        floatPtr(50),
			VideoCodec:       stringPtr("AV1"),
			AudioCodec:       stringPtr("Opus"),
			Container:        stringPtr("MP4"),
			VideoAspectRatio: stringPtr("21:9"),
			HasSubtitles:     boolPtr(true),
			SubtitleTracks:   []string{"en"},
			HasChapters:      boolPtr(true),
			Is3D:             boolPtr(true),
			Is4K:             boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}

	permissionPublic := entity.NewFileEntry(root, "secure/public.txt", 120, now)
	permissionPublic.Enhanced = &entity.EnhancedMetadata{
		OSContextTaxonomy: &entity.OSContextTaxonomy{
			Security: &entity.SecurityTaxonomy{PermissionLevel: "public"},
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	permissionPrivate := entity.NewFileEntry(root, "secure/private.txt", 220, now)
	permissionPrivate.Enhanced = &entity.EnhancedMetadata{
		OSContextTaxonomy: &entity.OSContextTaxonomy{
			Security: &entity.SecurityTaxonomy{PermissionLevel: "private"},
		},
		IndexedState: entity.IndexedState{Basic: true},
	}

	qualityLow := entity.NewFileEntry(root, "quality/low.txt", 120, now)
	qualityLow.Enhanced = &entity.EnhancedMetadata{
		ContentQuality: &entity.ContentQuality{QualityScore: 0.2},
		IndexedState:   entity.IndexedState{Basic: true},
	}
	qualityMedium := entity.NewFileEntry(root, "quality/medium.txt", 220, now)
	qualityMedium.Enhanced = &entity.EnhancedMetadata{
		ContentQuality: &entity.ContentQuality{QualityScore: 0.5},
		IndexedState:   entity.IndexedState{Basic: true},
	}
	qualityHigh := entity.NewFileEntry(root, "quality/high.txt", 320, now)
	qualityHigh.Enhanced = &entity.EnhancedMetadata{
		ContentQuality: &entity.ContentQuality{QualityScore: 0.9},
		IndexedState:   entity.IndexedState{Basic: true},
	}

	audioShort := entity.NewFileEntry(root, "audio/short.mp3", 120, now)
	audioShort.Enhanced = &entity.EnhancedMetadata{
		AudioMetadata: &entity.AudioMetadata{
			Duration:    floatPtr(20),
			Bitrate:     intPtr(96),
			SampleRate:  intPtr(22050),
			Channels:    intPtr(1),
			Codec:       stringPtr("MP3"),
			Format:      stringPtr("MP3"),
			ID3Genre:    stringPtr("Podcast"),
			ID3Artist:   stringPtr("Artist A"),
			ID3Album:    stringPtr("Album A"),
			ID3Year:     intPtr(2020),
			HasAlbumArt: boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	audioMedium := entity.NewFileEntry(root, "audio/medium.mp3", 220, now)
	audioMedium.Enhanced = &entity.EnhancedMetadata{
		AudioMetadata: &entity.AudioMetadata{
			Duration:     floatPtr(300),
			Bitrate:      intPtr(192),
			SampleRate:   intPtr(44100),
			Channels:     intPtr(2),
			Codec:        stringPtr("AAC"),
			Format:       stringPtr("M4A"),
			VorbisGenre:  stringPtr("Ambient"),
			VorbisArtist: stringPtr("Artist B"),
			VorbisAlbum:  stringPtr("Album B"),
			VorbisDate:   stringPtr("1999"),
			HasAlbumArt:  boolPtr(false),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}
	audioLong := entity.NewFileEntry(root, "audio/long.mp3", 320, now)
	audioLong.Enhanced = &entity.EnhancedMetadata{
		AudioMetadata: &entity.AudioMetadata{
			Duration:    floatPtr(7200),
			Bitrate:     intPtr(500),
			SampleRate:  intPtr(96000),
			Channels:    intPtr(6),
			Codec:       stringPtr("FLAC"),
			Format:      stringPtr("FLAC"),
			ID3Genre:    stringPtr("Classical"),
			ID3Artist:   stringPtr("Artist C"),
			ID3Album:    stringPtr("Album C"),
			ID3Year:     intPtr(1984),
			HasAlbumArt: boolPtr(true),
		},
		IndexedState: entity.IndexedState{Basic: true},
	}

	osLocal := entity.NewFileEntry(root, "system/local.txt", 120, now)
	osLocal.Enhanced = &entity.EnhancedMetadata{
		OSMetadata: &entity.OSMetadata{
			FileSystem: &entity.FileSystemInfo{
				MountPoint:     "/",
				FileSystemType: "apfs",
			},
		},
		OSContextTaxonomy: &entity.OSContextTaxonomy{
			Security: &entity.SecurityTaxonomy{
				SecurityCategory:   []string{"readable_by_group", "writable_by_owner"},
				SecurityAttributes: []string{"encrypted"},
				HasACLs:            true,
				ACLComplexity:      "complex",
			},
			Ownership: &entity.OwnershipTaxonomy{
				OwnerType:        "user",
				GroupCategory:    "developer",
				AccessRelations:  []string{"owned_by_user"},
				OwnershipPattern: "single_owner",
			},
			Temporal: &entity.TemporalTaxonomy{
				AccessFrequency: "frequent",
				TimeCategory:    []string{"created_recently", "modified_this_week"},
			},
			System: &entity.SystemTaxonomy{
				SystemFileType:     "regular",
				FileSystemCategory: "local",
				SystemAttributes:   []string{"hidden"},
				SystemFeatures:     []string{"acls", "extended_attrs"},
			},
		},
		ContentEncoding:    stringPtr("UTF-8"),
		LanguageConfidence: floatPtr(0.92),
		IndexedState:       entity.IndexedState{Basic: true},
	}

	osNetwork := entity.NewFileEntry(root, "system/network.txt", 220, now)
	osNetwork.Enhanced = &entity.EnhancedMetadata{
		OSMetadata: &entity.OSMetadata{
			FileSystem: &entity.FileSystemInfo{
				MountPoint:     "/Volumes/Share",
				FileSystemType: "smbfs",
			},
		},
		OSContextTaxonomy: &entity.OSContextTaxonomy{
			Security: &entity.SecurityTaxonomy{
				SecurityCategory:   []string{"public_read"},
				SecurityAttributes: []string{"quarantined"},
				HasACLs:            false,
				ACLComplexity:      "simple",
			},
			Ownership: &entity.OwnershipTaxonomy{
				OwnerType:        "service",
				GroupCategory:    "service",
				AccessRelations:  []string{"accessible_by_group"},
				OwnershipPattern: "shared_group",
			},
			Temporal: &entity.TemporalTaxonomy{
				AccessFrequency: "rare",
				TimeCategory:    []string{"accessed_today"},
			},
			System: &entity.SystemTaxonomy{
				SystemFileType:     "regular",
				FileSystemCategory: "network",
				SystemAttributes:   []string{"archive"},
				SystemFeatures:     []string{"sparse"},
			},
		},
		ContentEncoding:    stringPtr("Latin-1"),
		LanguageConfidence: floatPtr(0.6),
		IndexedState:       entity.IndexedState{Basic: true},
	}

	entries := []*entity.FileEntry{
		imageSmall,
		imageMedium,
		imageLarge,
		videoHD,
		videoFullHD,
		video4K,
		permissionPublic,
		permissionPrivate,
		qualityLow,
		qualityMedium,
		qualityHigh,
		audioShort,
		audioMedium,
		audioLong,
		osLocal,
		osNetwork,
	}
	for _, file := range entries {
		if err := repo.Upsert(ctx, workspace.ID, file); err != nil {
			t.Fatalf("upsert %s: %v", file.RelativePath, err)
		}
	}

	imageIDs := []entity.FileID{imageSmall.ID, imageMedium.ID, imageLarge.ID}
	audioIDs := []entity.FileID{audioShort.ID, audioMedium.ID, audioLong.ID}
	videoIDs := []entity.FileID{videoHD.ID, videoFullHD.ID, video4K.ID}
	osIDs := []entity.FileID{osLocal.ID, osNetwork.ID}

	imageRanges, err := repo.GetImageDimensionsRangeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image dimensions facet: %v", err)
	}
	imageCounts := numericRangeCountsByLabel(imageRanges)
	if imageCounts["Tiny (< 0.5 MP)"] != 1 {
		t.Fatalf("unexpected tiny image count: %#v", imageCounts)
	}
	if imageCounts["Medium (2 - 8 MP)"] != 1 {
		t.Fatalf("unexpected medium image count: %#v", imageCounts)
	}
	if imageCounts["Large (8 - 20 MP)"] != 1 {
		t.Fatalf("unexpected large image count: %#v", imageCounts)
	}

	formatCounts, err := repo.GetImageFormatFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image format facet: %v", err)
	}
	if formatCounts["JPEG"] != 1 || formatCounts["PNG"] != 1 || formatCounts["GIF"] != 1 {
		t.Fatalf("unexpected image format counts: %#v", formatCounts)
	}

	colorSpaceCounts, err := repo.GetImageColorSpaceFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image color space facet: %v", err)
	}
	if colorSpaceCounts["RGB"] != 2 || colorSpaceCounts["Indexed"] != 1 {
		t.Fatalf("unexpected image color space counts: %#v", colorSpaceCounts)
	}

	cameraMakeCounts, err := repo.GetCameraMakeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("camera make facet: %v", err)
	}
	if cameraMakeCounts["Canon"] != 2 || cameraMakeCounts["Sony"] != 1 {
		t.Fatalf("unexpected camera make counts: %#v", cameraMakeCounts)
	}

	cameraModelCounts, err := repo.GetCameraModelFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("camera model facet: %v", err)
	}
	if cameraModelCounts["5D"] != 1 || cameraModelCounts["A7"] != 1 || cameraModelCounts["R5"] != 1 {
		t.Fatalf("unexpected camera model counts: %#v", cameraModelCounts)
	}

	gpsCounts, err := repo.GetImageGPSLocationFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("gps location facet: %v", err)
	}
	if gpsCounts["NYC"] != 1 || gpsCounts["LA"] != 1 || gpsCounts["unknown"] != 1 {
		t.Fatalf("unexpected gps location counts: %#v", gpsCounts)
	}

	orientationCounts, err := repo.GetImageOrientationFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image orientation facet: %v", err)
	}
	if orientationCounts["1"] != 1 || orientationCounts["6"] != 1 || orientationCounts["unknown"] != 1 {
		t.Fatalf("unexpected orientation counts: %#v", orientationCounts)
	}

	transparencyCounts, err := repo.GetImageTransparencyFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image transparency facet: %v", err)
	}
	if transparencyCounts["true"] != 2 || transparencyCounts["false"] != 1 {
		t.Fatalf("unexpected transparency counts: %#v", transparencyCounts)
	}

	animatedCounts, err := repo.GetImageAnimatedFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image animated facet: %v", err)
	}
	if animatedCounts["true"] != 1 || animatedCounts["false"] != 2 {
		t.Fatalf("unexpected animated counts: %#v", animatedCounts)
	}

	depthRanges, err := repo.GetImageColorDepthRangeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image color depth facet: %v", err)
	}
	depthCounts := numericRangeCountsByLabel(depthRanges)
	if depthCounts["Low (<= 8 bpp)"] != 1 || depthCounts["High (17 - 32 bpp)"] != 2 {
		t.Fatalf("unexpected color depth counts: %#v", depthCounts)
	}

	isoRanges, err := repo.GetImageISORangeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image iso facet: %v", err)
	}
	isoCounts := numericRangeCountsByLabel(isoRanges)
	if isoCounts["Low (< 200)"] != 1 || isoCounts["High (800 - 3200)"] != 1 || isoCounts["Extreme (> 3200)"] != 1 {
		t.Fatalf("unexpected iso counts: %#v", isoCounts)
	}

	apertureRanges, err := repo.GetImageApertureRangeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image aperture facet: %v", err)
	}
	apertureCounts := numericRangeCountsByLabel(apertureRanges)
	if apertureCounts["Wide (<= 2.8)"] != 1 || apertureCounts["Standard (2.8 - 5.6)"] != 1 || apertureCounts["Very Narrow (> 11)"] != 1 {
		t.Fatalf("unexpected aperture counts: %#v", apertureCounts)
	}

	focalRanges, err := repo.GetImageFocalLengthRangeFacet(ctx, workspace.ID, imageIDs)
	if err != nil {
		t.Fatalf("image focal length facet: %v", err)
	}
	focalCounts := numericRangeCountsByLabel(focalRanges)
	if focalCounts["Wide (< 35mm)"] != 1 || focalCounts["Telephoto (70 - 200mm)"] != 1 || focalCounts["Super Telephoto (> 200mm)"] != 1 {
		t.Fatalf("unexpected focal length counts: %#v", focalCounts)
	}

	audioRanges, err := repo.GetAudioDurationRangeFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio duration facet: %v", err)
	}
	audioCounts := numericRangeCountsByLabel(audioRanges)
	if audioCounts["Very Short (< 30s)"] != 1 {
		t.Fatalf("unexpected very short audio count: %#v", audioCounts)
	}
	if audioCounts["Medium (2m - 10m)"] != 1 {
		t.Fatalf("unexpected medium audio count: %#v", audioCounts)
	}
	if audioCounts["Very Long (> 1h)"] != 1 {
		t.Fatalf("unexpected very long audio count: %#v", audioCounts)
	}

	audioBitrateRanges, err := repo.GetAudioBitrateRangeFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio bitrate facet: %v", err)
	}
	audioBitrateCounts := numericRangeCountsByLabel(audioBitrateRanges)
	if audioBitrateCounts["Low (< 128 kbps)"] != 1 || audioBitrateCounts["Standard (128 - 256 kbps)"] != 1 || audioBitrateCounts["Lossless (> 320 kbps)"] != 1 {
		t.Fatalf("unexpected audio bitrate counts: %#v", audioBitrateCounts)
	}

	audioSampleRanges, err := repo.GetAudioSampleRateRangeFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio sample rate facet: %v", err)
	}
	audioSampleCounts := numericRangeCountsByLabel(audioSampleRanges)
	if audioSampleCounts["Standard (22.05 - 44.1 kHz)"] != 1 || audioSampleCounts["High (44.1 - 96 kHz)"] != 1 || audioSampleCounts["Ultra (> 96 kHz)"] != 1 {
		t.Fatalf("unexpected audio sample rate counts: %#v", audioSampleCounts)
	}

	audioCodecCounts, err := repo.GetAudioCodecFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio codec facet: %v", err)
	}
	if audioCodecCounts["MP3"] != 1 || audioCodecCounts["AAC"] != 1 || audioCodecCounts["FLAC"] != 1 {
		t.Fatalf("unexpected audio codec counts: %#v", audioCodecCounts)
	}

	audioFormatCounts, err := repo.GetAudioFormatFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio format facet: %v", err)
	}
	if audioFormatCounts["MP3"] != 1 || audioFormatCounts["M4A"] != 1 || audioFormatCounts["FLAC"] != 1 {
		t.Fatalf("unexpected audio format counts: %#v", audioFormatCounts)
	}

	audioGenreCounts, err := repo.GetAudioGenreFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio genre facet: %v", err)
	}
	if audioGenreCounts["Podcast"] != 1 || audioGenreCounts["Ambient"] != 1 || audioGenreCounts["Classical"] != 1 {
		t.Fatalf("unexpected audio genre counts: %#v", audioGenreCounts)
	}

	audioArtistCounts, err := repo.GetAudioArtistFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio artist facet: %v", err)
	}
	if audioArtistCounts["Artist A"] != 1 || audioArtistCounts["Artist B"] != 1 || audioArtistCounts["Artist C"] != 1 {
		t.Fatalf("unexpected audio artist counts: %#v", audioArtistCounts)
	}

	audioAlbumCounts, err := repo.GetAudioAlbumFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio album facet: %v", err)
	}
	if audioAlbumCounts["Album A"] != 1 || audioAlbumCounts["Album B"] != 1 || audioAlbumCounts["Album C"] != 1 {
		t.Fatalf("unexpected audio album counts: %#v", audioAlbumCounts)
	}

	audioYearCounts, err := repo.GetAudioYearFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio year facet: %v", err)
	}
	if audioYearCounts["2020"] != 1 || audioYearCounts["1999"] != 1 || audioYearCounts["1984"] != 1 {
		t.Fatalf("unexpected audio year counts: %#v", audioYearCounts)
	}

	audioChannelCounts, err := repo.GetAudioChannelsFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio channels facet: %v", err)
	}
	if audioChannelCounts["mono"] != 1 || audioChannelCounts["stereo"] != 1 || audioChannelCounts["5.1"] != 1 {
		t.Fatalf("unexpected audio channel counts: %#v", audioChannelCounts)
	}

	audioArtCounts, err := repo.GetAudioHasAlbumArtFacet(ctx, workspace.ID, audioIDs)
	if err != nil {
		t.Fatalf("audio album art facet: %v", err)
	}
	if audioArtCounts["true"] != 2 || audioArtCounts["false"] != 1 {
		t.Fatalf("unexpected audio album art counts: %#v", audioArtCounts)
	}

	videoCounts, err := repo.GetVideoResolutionFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video resolution facet: %v", err)
	}
	if videoCounts["1080p"] != 1 || videoCounts["1440p"] != 1 || videoCounts["4K+"] != 1 {
		t.Fatalf("unexpected video resolution counts: %#v", videoCounts)
	}

	videoDurationRanges, err := repo.GetVideoDurationRangeFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video duration facet: %v", err)
	}
	videoDurationCounts := numericRangeCountsByLabel(videoDurationRanges)
	if videoDurationCounts["Short (< 5m)"] != 1 || videoDurationCounts["Medium (5m - 30m)"] != 1 || videoDurationCounts["Feature (> 2h)"] != 1 {
		t.Fatalf("unexpected video duration counts: %#v", videoDurationCounts)
	}

	videoBitrateRanges, err := repo.GetVideoBitrateRangeFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video bitrate facet: %v", err)
	}
	videoBitrateCounts := numericRangeCountsByLabel(videoBitrateRanges)
	if videoBitrateCounts["Low (< 1000 kbps)"] != 1 || videoBitrateCounts["High (3000 - 8000 kbps)"] != 1 || videoBitrateCounts["Ultra (> 8000 kbps)"] != 1 {
		t.Fatalf("unexpected video bitrate counts: %#v", videoBitrateCounts)
	}

	videoFrameRateRanges, err := repo.GetVideoFrameRateRangeFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video frame rate facet: %v", err)
	}
	videoFrameRateCounts := numericRangeCountsByLabel(videoFrameRateRanges)
	if videoFrameRateCounts["Low (< 24 fps)"] != 1 || videoFrameRateCounts["Standard (24 - 30 fps)"] != 1 || videoFrameRateCounts["High (30 - 60 fps)"] != 1 {
		t.Fatalf("unexpected video frame rate counts: %#v", videoFrameRateCounts)
	}

	videoCodecCounts, err := repo.GetVideoCodecFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video codec facet: %v", err)
	}
	if videoCodecCounts["H.264"] != 1 || videoCodecCounts["H.265"] != 1 || videoCodecCounts["AV1"] != 1 {
		t.Fatalf("unexpected video codec counts: %#v", videoCodecCounts)
	}

	videoAudioCodecCounts, err := repo.GetVideoAudioCodecFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video audio codec facet: %v", err)
	}
	if videoAudioCodecCounts["AAC"] != 2 || videoAudioCodecCounts["Opus"] != 1 {
		t.Fatalf("unexpected video audio codec counts: %#v", videoAudioCodecCounts)
	}

	videoContainerCounts, err := repo.GetVideoContainerFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video container facet: %v", err)
	}
	if videoContainerCounts["MP4"] != 2 || videoContainerCounts["MKV"] != 1 {
		t.Fatalf("unexpected video container counts: %#v", videoContainerCounts)
	}

	videoAspectCounts, err := repo.GetVideoAspectRatioFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video aspect ratio facet: %v", err)
	}
	if videoAspectCounts["16:9"] != 2 || videoAspectCounts["21:9"] != 1 {
		t.Fatalf("unexpected video aspect ratio counts: %#v", videoAspectCounts)
	}

	videoSubtitlesCounts, err := repo.GetVideoHasSubtitlesFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video subtitles facet: %v", err)
	}
	if videoSubtitlesCounts["true"] != 2 || videoSubtitlesCounts["false"] != 1 {
		t.Fatalf("unexpected video subtitles counts: %#v", videoSubtitlesCounts)
	}

	videoSubtitleLangCounts, err := repo.GetVideoSubtitleLanguageFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video subtitle languages facet: %v", err)
	}
	if videoSubtitleLangCounts["en"] != 2 || videoSubtitleLangCounts["es"] != 1 {
		t.Fatalf("unexpected subtitle language counts: %#v", videoSubtitleLangCounts)
	}

	videoChaptersCounts, err := repo.GetVideoHasChaptersFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video chapters facet: %v", err)
	}
	if videoChaptersCounts["true"] != 2 || videoChaptersCounts["false"] != 1 {
		t.Fatalf("unexpected video chapters counts: %#v", videoChaptersCounts)
	}

	video3DCounts, err := repo.GetVideoIs3DFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video 3d facet: %v", err)
	}
	if video3DCounts["true"] != 1 || video3DCounts["false"] != 2 {
		t.Fatalf("unexpected video 3d counts: %#v", video3DCounts)
	}

	videoQualityCounts, err := repo.GetVideoQualityTierFacet(ctx, workspace.ID, videoIDs)
	if err != nil {
		t.Fatalf("video quality tier facet: %v", err)
	}
	if videoQualityCounts["HD"] != 2 || videoQualityCounts["4K"] != 1 {
		t.Fatalf("unexpected video quality tier counts: %#v", videoQualityCounts)
	}

	permissionCounts, err := repo.GetPermissionLevelFacet(ctx, workspace.ID, nil)
	if err != nil {
		t.Fatalf("permission level facet: %v", err)
	}
	if permissionCounts["public"] != 1 || permissionCounts["private"] != 1 {
		t.Fatalf("unexpected permission level counts: %#v", permissionCounts)
	}

	qualityRanges, err := repo.GetContentQualityRangeFacet(ctx, workspace.ID, nil)
	if err != nil {
		t.Fatalf("content quality facet: %v", err)
	}
	qualityCounts := numericRangeCountsByLabel(qualityRanges)
	if qualityCounts["Low (< 0.3)"] != 1 || qualityCounts["Medium (0.3 - 0.7)"] != 1 || qualityCounts["High (0.7 - 1.0)"] != 1 {
		t.Fatalf("unexpected content quality counts: %#v", qualityCounts)
	}

	encodingCounts, err := repo.GetContentEncodingFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("content encoding facet: %v", err)
	}
	if encodingCounts["UTF-8"] != 1 || encodingCounts["Latin-1"] != 1 {
		t.Fatalf("unexpected content encoding counts: %#v", encodingCounts)
	}

	langRanges, err := repo.GetLanguageConfidenceRangeFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("language confidence facet: %v", err)
	}
	langCounts := numericRangeCountsByLabel(langRanges)
	if langCounts["Medium (0.5 - 0.8)"] != 1 || langCounts["High (0.8 - 1.0)"] != 1 {
		t.Fatalf("unexpected language confidence counts: %#v", langCounts)
	}

	fsTypeCounts, err := repo.GetFilesystemTypeFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("filesystem type facet: %v", err)
	}
	if fsTypeCounts["apfs"] != 1 || fsTypeCounts["smbfs"] != 1 {
		t.Fatalf("unexpected filesystem type counts: %#v", fsTypeCounts)
	}

	mountCounts, err := repo.GetMountPointFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("mount point facet: %v", err)
	}
	if mountCounts["/"] != 1 || mountCounts["/Volumes/Share"] != 1 {
		t.Fatalf("unexpected mount point counts: %#v", mountCounts)
	}

	securityCategoryCounts, err := repo.GetSecurityCategoryFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("security category facet: %v", err)
	}
	if securityCategoryCounts["readable_by_group"] != 1 || securityCategoryCounts["public_read"] != 1 {
		t.Fatalf("unexpected security category counts: %#v", securityCategoryCounts)
	}

	securityAttributeCounts, err := repo.GetSecurityAttributesFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("security attributes facet: %v", err)
	}
	if securityAttributeCounts["encrypted"] != 1 || securityAttributeCounts["quarantined"] != 1 {
		t.Fatalf("unexpected security attribute counts: %#v", securityAttributeCounts)
	}

	aclCounts, err := repo.GetHasACLsFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("has acls facet: %v", err)
	}
	if aclCounts["true"] != 1 || aclCounts["false"] != 1 {
		t.Fatalf("unexpected has acls counts: %#v", aclCounts)
	}

	aclComplexityCounts, err := repo.GetACLComplexityFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("acl complexity facet: %v", err)
	}
	if aclComplexityCounts["complex"] != 1 || aclComplexityCounts["simple"] != 1 {
		t.Fatalf("unexpected acl complexity counts: %#v", aclComplexityCounts)
	}

	ownerTypeCounts, err := repo.GetOwnerTypeFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("owner type facet: %v", err)
	}
	if ownerTypeCounts["user"] != 1 || ownerTypeCounts["service"] != 1 {
		t.Fatalf("unexpected owner type counts: %#v", ownerTypeCounts)
	}

	groupCategoryCounts, err := repo.GetGroupCategoryFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("group category facet: %v", err)
	}
	if groupCategoryCounts["developer"] != 1 || groupCategoryCounts["service"] != 1 {
		t.Fatalf("unexpected group category counts: %#v", groupCategoryCounts)
	}

	accessRelationCounts, err := repo.GetAccessRelationFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("access relation facet: %v", err)
	}
	if accessRelationCounts["owned_by_user"] != 1 || accessRelationCounts["accessible_by_group"] != 1 {
		t.Fatalf("unexpected access relation counts: %#v", accessRelationCounts)
	}

	ownershipPatternCounts, err := repo.GetOwnershipPatternFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("ownership pattern facet: %v", err)
	}
	if ownershipPatternCounts["single_owner"] != 1 || ownershipPatternCounts["shared_group"] != 1 {
		t.Fatalf("unexpected ownership pattern counts: %#v", ownershipPatternCounts)
	}

	accessFrequencyCounts, err := repo.GetAccessFrequencyFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("access frequency facet: %v", err)
	}
	if accessFrequencyCounts["frequent"] != 1 || accessFrequencyCounts["rare"] != 1 {
		t.Fatalf("unexpected access frequency counts: %#v", accessFrequencyCounts)
	}

	timeCategoryCounts, err := repo.GetTimeCategoryFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("time category facet: %v", err)
	}
	if timeCategoryCounts["created_recently"] != 1 || timeCategoryCounts["accessed_today"] != 1 {
		t.Fatalf("unexpected time category counts: %#v", timeCategoryCounts)
	}

	systemFileTypeCounts, err := repo.GetSystemFileTypeFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("system file type facet: %v", err)
	}
	if systemFileTypeCounts["regular"] != 2 {
		t.Fatalf("unexpected system file type counts: %#v", systemFileTypeCounts)
	}

	fileSystemCategoryCounts, err := repo.GetFileSystemCategoryFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("file system category facet: %v", err)
	}
	if fileSystemCategoryCounts["local"] != 1 || fileSystemCategoryCounts["network"] != 1 {
		t.Fatalf("unexpected file system category counts: %#v", fileSystemCategoryCounts)
	}

	systemAttributesCounts, err := repo.GetSystemAttributesFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("system attributes facet: %v", err)
	}
	if systemAttributesCounts["hidden"] != 1 || systemAttributesCounts["archive"] != 1 {
		t.Fatalf("unexpected system attributes counts: %#v", systemAttributesCounts)
	}

	systemFeaturesCounts, err := repo.GetSystemFeaturesFacet(ctx, workspace.ID, osIDs)
	if err != nil {
		t.Fatalf("system features facet: %v", err)
	}
	if systemFeaturesCounts["acls"] != 1 || systemFeaturesCounts["sparse"] != 1 {
		t.Fatalf("unexpected system features counts: %#v", systemFeaturesCounts)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func numericRangeCountsByLabel(ranges []repository.NumericRangeCount) map[string]int {
	counts := make(map[string]int)
	for _, rng := range ranges {
		counts[rng.Label] = rng.Count
	}
	return counts
}
