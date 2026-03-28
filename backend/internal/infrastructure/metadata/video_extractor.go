package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// VideoExtractor extracts comprehensive metadata from video files using ffprobe.
type VideoExtractor struct {
	logger zerolog.Logger
}

// NewVideoExtractor creates a new video metadata extractor.
func NewVideoExtractor(logger zerolog.Logger) *VideoExtractor {
	return &VideoExtractor{
		logger: logger.With().Str("component", "video_metadata_extractor").Logger(),
	}
}

// CanExtract returns true for video files.
func (e *VideoExtractor) CanExtract(extension string) bool {
	ext := strings.ToLower(extension)
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".3gp", ".mpg", ".mpeg", ".ts", ".m2ts"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// Extract extracts all possible metadata from a video file.
func (e *VideoExtractor) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	if !e.CanExtract(entry.Extension) {
		return nil, nil
	}

	enhanced := &entity.EnhancedMetadata{
		VideoMetadata: &entity.VideoMetadata{},
	}

	// Use ffprobe for comprehensive extraction
	if err := e.extractWithFFProbe(ctx, entry.AbsolutePath, enhanced); err != nil {
		e.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract video metadata with ffprobe")
		// Try exiftool as fallback
		if err2 := e.extractWithExifTool(ctx, entry.AbsolutePath, enhanced); err2 != nil {
			e.logger.Warn().Err(err2).Str("file", entry.RelativePath).Msg("Failed to extract video metadata with exiftool")
		}
	}

	return enhanced, nil
}

// extractWithFFProbe uses ffprobe to extract video metadata.
func (e *VideoExtractor) extractWithFFProbe(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found")
	}

	vm := enhanced.VideoMetadata
	if vm == nil {
		vm = &entity.VideoMetadata{}
		enhanced.VideoMetadata = vm
	}

	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	return e.parseFFProbeJSON(output, vm)
}

// extractWithExifTool uses exiftool as fallback for video metadata.
func (e *VideoExtractor) extractWithExifTool(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found")
	}

	vm := enhanced.VideoMetadata
	if vm == nil {
		vm = &entity.VideoMetadata{}
		enhanced.VideoMetadata = vm
	}

	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-all", filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w", err)
	}

	return e.parseExifToolJSON(output, vm)
}

// parseFFProbeJSON parses ffprobe JSON output.
func (e *VideoExtractor) parseFFProbeJSON(jsonData []byte, vm *entity.VideoMetadata) error {
	var result struct {
		Format struct {
			Duration    string            `json:"duration"`
			BitRate     string            `json:"bit_rate"`
			FormatName  string            `json:"format_name"`
			Tags        map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecName      string `json:"codec_name"`
			CodecType      string `json:"codec_type"`
			CodecLongName  string `json:"codec_long_name"`
			Width          int    `json:"width"`
			Height         int    `json:"height"`
			RFrameRate     string `json:"r_frame_rate"`
			BitRate        string `json:"bit_rate"`
			PixelFormat    string `json:"pix_fmt"`
			ColorSpace     string `json:"color_space"`
			SampleRate     string `json:"sample_rate"`
			Channels       int    `json:"channels"`
			ChannelLayout  string `json:"channel_layout"`
			Language       string `json:"tags.language"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return fmt.Errorf("failed to unmarshal ffprobe JSON: %w", err)
	}

	// Parse format info
	if result.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			vm.Duration = &duration
		}
	}

	if result.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(result.Format.BitRate); err == nil {
			vm.Bitrate = &bitrate
		}
	}

	if result.Format.FormatName != "" {
		vm.Container = &result.Format.FormatName
	}

	// Parse tags
	for key, value := range result.Format.Tags {
		e.setTagField(vm, key, value)
	}

	// Parse streams
	var subtitleTracks []string

	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			if vm.Width == 0 {
				vm.Width = stream.Width
			}
			if vm.Height == 0 {
				vm.Height = stream.Height
			}
			if vm.VideoCodec == nil {
				vm.VideoCodec = &stream.CodecName
			}
			if stream.BitRate != "" {
				if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
					vm.VideoBitrate = &bitrate
				}
			}
			if stream.PixelFormat != "" {
				vm.VideoPixelFormat = &stream.PixelFormat
			}
			if stream.ColorSpace != "" {
				vm.VideoColorSpace = &stream.ColorSpace
			}
			if stream.RFrameRate != "" {
				// Parse frame rate (e.g., "30/1" -> 30.0)
				parts := strings.Split(stream.RFrameRate, "/")
				if len(parts) == 2 {
					if num, err := strconv.ParseFloat(parts[0], 64); err == nil {
						if den, err := strconv.ParseFloat(parts[1], 64); err == nil && den != 0 {
							fps := num / den
							vm.FrameRate = &fps
						}
					}
				}
			}
			// Calculate aspect ratio
			if stream.Width > 0 && stream.Height > 0 {
				aspectRatio := fmt.Sprintf("%d:%d", stream.Width, stream.Height)
				vm.VideoAspectRatio = &aspectRatio
			}

		case "audio":
			if vm.AudioCodec == nil {
				vm.AudioCodec = &stream.CodecName
			}
			if stream.BitRate != "" {
				if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
					vm.AudioBitrate = &bitrate
				}
			}
			if stream.SampleRate != "" {
				if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
					vm.AudioSampleRate = &sr
				}
			}
			if stream.Channels > 0 {
				vm.AudioChannels = &stream.Channels
			}
			if stream.Language != "" {
				vm.AudioLanguage = &stream.Language
			}

		case "subtitle":
			if stream.Language != "" {
				subtitleTracks = append(subtitleTracks, stream.Language)
			}
		}
	}

	if len(subtitleTracks) > 0 {
		hasSubtitles := true
		vm.HasSubtitles = &hasSubtitles
		vm.SubtitleTracks = subtitleTracks
	}

	// Determine video quality
	if vm.Width >= 3840 || vm.Height >= 2160 {
		is4K := true
		vm.Is4K = &is4K
	} else if vm.Width >= 1920 || vm.Height >= 1080 {
		isHD := true
		vm.IsHD = &isHD
	}

	// Set codec if not set
	if vm.Codec == nil && vm.VideoCodec != nil {
		vm.Codec = vm.VideoCodec
	}

	return nil
}

// parseExifToolJSON parses exiftool JSON output for video.
func (e *VideoExtractor) parseExifToolJSON(jsonData []byte, vm *entity.VideoMetadata) error {
	var results []map[string]interface{}
	if err := json.Unmarshal(jsonData, &results); err != nil {
		return fmt.Errorf("failed to unmarshal exiftool JSON: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no data in exiftool output")
	}

	data := results[0]

	for key, value := range data {
		if value == nil {
			continue
		}

		valueStr := fmt.Sprintf("%v", value)
		if valueStr == "" {
			continue
		}

		e.setTagField(vm, key, valueStr)
	}

	return nil
}

// setTagField sets video tag fields.
func (e *VideoExtractor) setTagField(vm *entity.VideoMetadata, key string, value string) {
	switch key {
	case "Title", "title":
		vm.Title = &value
	case "Artist", "artist":
		vm.Artist = &value
	case "Album", "album":
		vm.Album = &value
	case "Genre", "genre":
		vm.Genre = &value
	case "Year", "year":
		if year, err := strconv.Atoi(value); err == nil {
			vm.Year = &year
		}
	case "Director", "director":
		vm.Director = &value
	case "Producer", "producer":
		vm.Producer = &value
	case "Copyright", "copyright":
		vm.Copyright = &value
	case "Description", "description", "comment":
		if vm.Description == nil {
			vm.Description = &value
		} else if vm.Comment == nil {
			vm.Comment = &value
		}
	}
}

