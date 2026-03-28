package metadata

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// TranscriptionService transcribes audio and video files using Whisper.
type TranscriptionService struct {
	whisperPath string
	logger      zerolog.Logger
}

// NewTranscriptionService creates a new transcription service.
func NewTranscriptionService(logger zerolog.Logger) *TranscriptionService {
	// Try to find whisper in common locations
	whisperPath := "whisper"
	if path, err := exec.LookPath("whisper"); err == nil {
		whisperPath = path
	} else if path, err := exec.LookPath("whisper-ctranslate2"); err == nil {
		whisperPath = path
	}

	return &TranscriptionService{
		whisperPath: whisperPath,
		logger:      logger.With().Str("component", "transcription_service").Logger(),
	}
}

// IsAvailable checks if Whisper is available.
func (s *TranscriptionService) IsAvailable() bool {
	cmd := exec.Command(s.whisperPath, "--help")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// TranscribeAudio transcribes an audio file.
func (s *TranscriptionService) TranscribeAudio(ctx context.Context, audioPath string, language string) (*entity.TranscriptionResult, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("whisper not available")
	}

	// Default language
	if language == "" {
		language = "auto" // Auto-detect
	}

	// Whisper command: whisper audio.mp3 --language es --output_format txt
	outputDir := filepath.Join(filepath.Dir(audioPath), ".transcription_temp")
	cmd := exec.CommandContext(ctx, s.whisperPath,
		audioPath,
		"--language", language,
		"--output_dir", outputDir,
		"--output_format", "txt",
		"--verbose", "false",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper failed: %w, output: %s", err, string(output))
	}

	// Read transcribed text
	baseName := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	textPath := filepath.Join(outputDir, baseName+".txt")
	
	textBytes, err := os.ReadFile(textPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcription: %w", err)
	}

	text := strings.TrimSpace(string(textBytes))

	// Try to read segments from JSON if available
	var segments []entity.Segment
	jsonPath := filepath.Join(outputDir, baseName+".json")
	if jsonBytes, err := os.ReadFile(jsonPath); err == nil {
		// Parse JSON segments (simplified - would need proper JSON parsing)
		// For now, we'll skip detailed segment parsing
		_ = jsonBytes
	}

	// Clean up temp directory
	os.RemoveAll(outputDir)

	// Get duration using ffprobe if available
	duration := 0.0
	if ffprobePath, err := exec.LookPath("ffprobe"); err == nil {
		cmd := exec.CommandContext(ctx, ffprobePath,
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			audioPath,
		)
		if output, err := cmd.Output(); err == nil {
			fmt.Sscanf(string(output), "%f", &duration)
		}
	}

	return &entity.TranscriptionResult{
		Text:        text,
		Language:    language,
		Duration:    duration,
		Confidence:  0.85, // Whisper doesn't provide per-segment confidence easily
		Segments:    segments,
		ExtractedAt: time.Now(),
	}, nil
}

// TranscribeVideo transcribes a video file (extracts audio first).
func (s *TranscriptionService) TranscribeVideo(ctx context.Context, videoPath string, language string) (*entity.TranscriptionResult, error) {
	// Extract audio using ffmpeg first
	tempAudioPath := filepath.Join(filepath.Dir(videoPath), ".temp_audio_"+filepath.Base(videoPath)+".wav")
	
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not available for audio extraction")
	} else {
		// Extract audio: ffmpeg -i video.mp4 -vn -acodec pcm_s16le audio.wav
		cmd := exec.CommandContext(ctx, ffmpegPath,
			"-i", videoPath,
			"-vn",
			"-acodec", "pcm_s16le",
			"-ar", "16000", // 16kHz sample rate for Whisper
			"-ac", "1",     // Mono
			tempAudioPath,
		)
		
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to extract audio: %w", err)
		}
		
		// Clean up temp audio file after transcription
		defer os.Remove(tempAudioPath)
	}

	// Transcribe the extracted audio
	return s.TranscribeAudio(ctx, tempAudioPath, language)
}

