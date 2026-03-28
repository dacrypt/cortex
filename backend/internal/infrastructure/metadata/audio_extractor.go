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

// AudioExtractor extracts comprehensive metadata from audio files (ID3, Vorbis Comments).
type AudioExtractor struct {
	logger zerolog.Logger
}

// NewAudioExtractor creates a new audio metadata extractor.
func NewAudioExtractor(logger zerolog.Logger) *AudioExtractor {
	return &AudioExtractor{
		logger: logger.With().Str("component", "audio_metadata_extractor").Logger(),
	}
}

// CanExtract returns true for audio files.
func (e *AudioExtractor) CanExtract(extension string) bool {
	ext := strings.ToLower(extension)
	audioExts := []string{".mp3", ".flac", ".ogg", ".wav", ".aac", ".m4a", ".opus", ".wma", ".ape", ".mpc"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

// Extract extracts all possible metadata from an audio file.
func (e *AudioExtractor) Extract(ctx context.Context, entry *entity.FileEntry) (*entity.EnhancedMetadata, error) {
	if !e.CanExtract(entry.Extension) {
		return nil, nil
	}

	enhanced := &entity.EnhancedMetadata{
		AudioMetadata: &entity.AudioMetadata{},
	}

	// Use exiftool or ffprobe for comprehensive extraction
	if err := e.extractWithExifTool(ctx, entry.AbsolutePath, enhanced); err != nil {
		e.logger.Warn().Err(err).Str("file", entry.RelativePath).Msg("Failed to extract audio metadata with exiftool")
		// Try ffprobe as fallback
		if err2 := e.extractWithFFProbe(ctx, entry.AbsolutePath, enhanced); err2 != nil {
			e.logger.Warn().Err(err2).Str("file", entry.RelativePath).Msg("Failed to extract audio metadata with ffprobe")
		}
	}

	return enhanced, nil
}

// extractWithExifTool uses exiftool to extract audio metadata.
func (e *AudioExtractor) extractWithExifTool(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found")
	}

	am := enhanced.AudioMetadata
	if am == nil {
		am = &entity.AudioMetadata{}
		enhanced.AudioMetadata = am
	}

	// Try JSON output first
	cmd := exec.CommandContext(ctx, "exiftool", "-j", "-all", filePath)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		if err := e.parseExifToolJSON(output, am); err == nil {
			return nil
		}
		e.logger.Debug().Err(err).Msg("Failed to parse exiftool JSON, falling back to text")
	}

	// Fallback to text output
	cmd = exec.CommandContext(ctx, "exiftool", filePath)
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w", err)
	}

	return e.parseExifToolText(output, am)
}

// extractWithFFProbe uses ffprobe to extract audio metadata (fallback).
func (e *AudioExtractor) extractWithFFProbe(ctx context.Context, filePath string, enhanced *entity.EnhancedMetadata) error {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found")
	}

	am := enhanced.AudioMetadata
	if am == nil {
		am = &entity.AudioMetadata{}
		enhanced.AudioMetadata = am
	}

	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	return e.parseFFProbeJSON(output, am)
}

// parseExifToolJSON parses exiftool JSON output for audio.
func (e *AudioExtractor) parseExifToolJSON(jsonData []byte, am *entity.AudioMetadata) error {
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

		// Parse ID3 tags
		if strings.HasPrefix(key, "ID3:") {
			id3Key := strings.TrimPrefix(key, "ID3:")
			e.setID3Field(am, id3Key, value, valueStr)
		} else if strings.HasPrefix(key, "Vorbis:") {
			vorbisKey := strings.TrimPrefix(key, "Vorbis:")
			e.setVorbisField(am, vorbisKey, value, valueStr)
		} else {
			// Standard audio properties
			e.setStandardField(am, key, value, valueStr)
		}
	}

	return nil
}

// parseExifToolText parses exiftool text output (fallback).
func (e *AudioExtractor) parseExifToolText(output []byte, am *entity.AudioMetadata) error {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=====") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.HasPrefix(key, "ID3:") {
			id3Key := strings.TrimPrefix(key, "ID3:")
			e.setID3Field(am, id3Key, value, value)
		} else if strings.HasPrefix(key, "Vorbis:") {
			vorbisKey := strings.TrimPrefix(key, "Vorbis:")
			e.setVorbisField(am, vorbisKey, value, value)
		} else {
			e.setStandardField(am, key, value, value)
		}
	}

	return nil
}

// parseFFProbeJSON parses ffprobe JSON output.
func (e *AudioExtractor) parseFFProbeJSON(jsonData []byte, am *entity.AudioMetadata) error {
	var result struct {
		Format struct {
			Duration    string            `json:"duration"`
			BitRate     string            `json:"bit_rate"`
			Tags        map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			CodecName      string `json:"codec_name"`
			CodecType      string `json:"codec_type"`
			SampleRate     string `json:"sample_rate"`
			Channels       int    `json:"channels"`
			BitsPerSample  int    `json:"bits_per_sample"`
			BitRate        string `json:"bit_rate"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return fmt.Errorf("failed to unmarshal ffprobe JSON: %w", err)
	}

	// Parse duration
	if result.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			am.Duration = &duration
		}
	}

	// Parse bitrate
	if result.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(result.Format.BitRate); err == nil {
			am.Bitrate = &bitrate
		}
	}

	// Parse tags
	for key, value := range result.Format.Tags {
		e.setStandardField(am, key, value, value)
	}

	// Parse stream info
	for _, stream := range result.Streams {
		if stream.CodecType == "audio" {
			if am.Codec == nil {
				codec := stream.CodecName
				am.Codec = &codec
			}
			if stream.SampleRate != "" {
				if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
					am.SampleRate = &sr
				}
			}
			if stream.Channels > 0 {
				am.Channels = &stream.Channels
			}
			if stream.BitsPerSample > 0 {
				am.BitDepth = &stream.BitsPerSample
			}
		}
	}

	return nil
}

// setID3Field sets ID3 metadata fields.
func (e *AudioExtractor) setID3Field(am *entity.AudioMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "Title", "TIT2":
		am.ID3Title = &valueStr
	case "Artist", "TPE1":
		am.ID3Artist = &valueStr
	case "Album", "TALB":
		am.ID3Album = &valueStr
	case "Year", "TYER", "TDRC":
		if year, err := strconv.Atoi(valueStr); err == nil {
			am.ID3Year = &year
		}
	case "Genre", "TCON":
		am.ID3Genre = &valueStr
	case "Track", "TRCK":
		if track, err := strconv.Atoi(valueStr); err == nil {
			am.ID3Track = &track
		}
	case "Disc", "TPOS":
		if disc, err := strconv.Atoi(valueStr); err == nil {
			am.ID3Disc = &disc
		}
	case "Composer", "TCOM":
		am.ID3Composer = &valueStr
	case "Conductor", "TPE3":
		am.ID3Conductor = &valueStr
	case "Performer":
		am.ID3Performer = &valueStr
	case "Publisher", "TPUB":
		am.ID3Publisher = &valueStr
	case "Comment", "COMM":
		am.ID3Comment = &valueStr
	case "Lyrics", "USLT":
		am.ID3Lyrics = &valueStr
	case "BPM", "TBPM":
		if bpm, err := strconv.Atoi(valueStr); err == nil {
			am.ID3BPM = &bpm
		}
	case "ISRC":
		am.ID3ISRC = &valueStr
	case "Copyright", "TCOP":
		am.ID3Copyright = &valueStr
	case "EncodedBy", "TENC":
		am.ID3EncodedBy = &valueStr
	case "AlbumArtist", "TPE2":
		am.ID3AlbumArtist = &valueStr
	}
}

// setVorbisField sets Vorbis Comments fields.
func (e *AudioExtractor) setVorbisField(am *entity.AudioMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "TITLE":
		am.VorbisTitle = &valueStr
	case "ARTIST":
		am.VorbisArtist = &valueStr
	case "ALBUM":
		am.VorbisAlbum = &valueStr
	case "DATE":
		am.VorbisDate = &valueStr
	case "GENRE":
		am.VorbisGenre = &valueStr
	case "TRACKNUMBER", "TRACK":
		am.VorbisTrack = &valueStr
	case "COMMENT":
		am.VorbisComment = &valueStr
	}
}

// setStandardField sets standard audio properties.
func (e *AudioExtractor) setStandardField(am *entity.AudioMetadata, key string, value interface{}, valueStr string) {
	switch key {
	case "Duration", "Duration#1":
		if duration, err := parseFloat(valueStr); err == nil {
			am.Duration = &duration
		}
	case "BitRate", "Bitrate":
		// Remove "kbps" or "bps" suffix
		valueStr = strings.TrimSuffix(strings.TrimSpace(valueStr), " kbps")
		valueStr = strings.TrimSuffix(valueStr, " bps")
		if bitrate, err := strconv.Atoi(valueStr); err == nil {
			am.Bitrate = &bitrate
		}
	case "SampleRate", "AudioSampleRate":
		// Remove "Hz" suffix
		valueStr = strings.TrimSuffix(strings.TrimSpace(valueStr), " Hz")
		if sr, err := strconv.Atoi(valueStr); err == nil {
			am.SampleRate = &sr
		}
	case "Channels", "AudioChannels":
		if channels, err := strconv.Atoi(valueStr); err == nil {
			am.Channels = &channels
		}
	case "BitsPerSample", "BitDepth":
		if depth, err := strconv.Atoi(valueStr); err == nil {
			am.BitDepth = &depth
		}
	case "Codec", "AudioCodec":
		am.Codec = &valueStr
	case "FileType", "MIMEType":
		am.Format = &valueStr
	}
}

