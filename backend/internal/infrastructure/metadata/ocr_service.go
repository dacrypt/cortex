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

// OCRService extracts text from images and scanned PDFs using Tesseract OCR.
type OCRService struct {
	tesseractPath string
	logger        zerolog.Logger
}

// NewOCRService creates a new OCR service.
func NewOCRService(logger zerolog.Logger) *OCRService {
	// Try to find tesseract in common locations
	tesseractPath := "tesseract"
	if path, err := exec.LookPath("tesseract"); err == nil {
		tesseractPath = path
	}

	return &OCRService{
		tesseractPath: tesseractPath,
		logger:        logger.With().Str("component", "ocr_service").Logger(),
	}
}

// IsAvailable checks if Tesseract OCR is available.
func (s *OCRService) IsAvailable() bool {
	cmd := exec.Command(s.tesseractPath, "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// ExtractTextFromImage extracts text from an image file.
func (s *OCRService) ExtractTextFromImage(ctx context.Context, imagePath string, language string, outputDir string) (*entity.OCRResult, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("tesseract OCR not available")
	}

	// Default language
	if language == "" {
		language = "spa+eng" // Spanish + English
	}

	// Tesseract command: tesseract image.png output -l spa+eng
	outputPath := s.buildTempOutputPath(imagePath, outputDir)
	cmd := exec.CommandContext(ctx, s.tesseractPath, imagePath, outputPath, "-l", language)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tesseract failed: %w, output: %s", err, string(output))
	}

	// Read extracted text
	textPath := outputPath + ".txt"
	textBytes, err := os.ReadFile(textPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR output: %w", err)
	}

	// Clean up temp file
	os.Remove(textPath)

	text := strings.TrimSpace(string(textBytes))

	return &entity.OCRResult{
		Text:        text,
		Confidence:  0.8, // Tesseract doesn't provide per-word confidence in this mode
		Language:    language,
		PageCount:   1,
		ExtractedAt: time.Now(),
	}, nil
}

// ExtractTextFromPDF extracts text from a scanned PDF.
func (s *OCRService) ExtractTextFromPDF(ctx context.Context, pdfPath string, language string, outputDir string) (*entity.OCRResult, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("tesseract OCR not available")
	}

	// Check if pdfinfo is available to get page count
	pageCount := 1
	if pdfInfoPath, err := exec.LookPath("pdfinfo"); err == nil {
		cmd := exec.CommandContext(ctx, pdfInfoPath, pdfPath)
		output, err := cmd.Output()
		if err == nil {
			// Parse page count from pdfinfo output
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Pages:") {
					fmt.Sscanf(line, "Pages: %d", &pageCount)
					break
				}
			}
		}
	}

	// Default language
	if language == "" {
		language = "spa+eng"
	}

	// Tesseract command: tesseract pdf.pdf output -l spa+eng pdf
	outputPath := s.buildTempOutputPath(pdfPath, outputDir)
	cmd := exec.CommandContext(ctx, s.tesseractPath, pdfPath, outputPath, "-l", language, "pdf")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tesseract PDF extraction failed: %w, output: %s", err, string(output))
	}

	// For PDF output, tesseract creates a PDF with text layer
	// We need to extract text from it using pdftotext if available
	text := ""
	if pdftotextPath, err := exec.LookPath("pdftotext"); err == nil {
		ocrPdfPath := outputPath + ".pdf"
		cmd := exec.CommandContext(ctx, pdftotextPath, ocrPdfPath, "-")
		textBytes, err := cmd.Output()
		if err == nil {
			text = strings.TrimSpace(string(textBytes))
		}
		os.Remove(ocrPdfPath)
	}

	if text == "" {
		// Fallback: try to read as text file
		textPath := outputPath + ".txt"
		if textBytes, err := os.ReadFile(textPath); err == nil {
			text = strings.TrimSpace(string(textBytes))
			os.Remove(textPath)
		}
	}

	return &entity.OCRResult{
		Text:        text,
		Confidence:  0.75, // Lower confidence for PDFs
		Language:    language,
		PageCount:   pageCount,
		ExtractedAt: time.Now(),
	}, nil
}

func (s *OCRService) buildTempOutputPath(inputPath string, outputDir string) string {
	dir := strings.TrimSpace(outputDir)
	if dir == "" {
		dir = os.TempDir()
	}
	_ = os.MkdirAll(dir, 0o755)
	base := filepath.Base(inputPath)
	return filepath.Join(dir, ".ocr_temp_"+base)
}
