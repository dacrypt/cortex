package mirror

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/rs/zerolog"
	"github.com/xuri/excelize/v2"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

type Extractor struct {
	Logger      zerolog.Logger
	MaxFileSize int64
}

// Mirrorable returns true if the file can be mirrored based on content detection.
func Mirrorable(absolutePath string) bool {
	detected, err := detectInputType(absolutePath)
	return err == nil && detected.Kind != ""
}

type inputDetection struct {
	Kind         string
	PandocFormat string
	MirrorFormat entity.MirrorFormat
	IsText       bool
}

// EnsureMirror extracts content and writes the mirror file if needed.
func (e *Extractor) EnsureMirror(ctx context.Context, workspaceRoot string, entry *entity.FileEntry) (*entity.MirrorMetadata, string, error) {
	if entry == nil {
		return nil, "", errors.New("entry is nil")
	}
	if e.MaxFileSize > 0 && entry.FileSize > e.MaxFileSize {
		return nil, "", errNoMirror
	}

	detected, err := detectInputType(entry.AbsolutePath)
	if err != nil {
		return nil, "", err
	}
	if detected.Kind == "" {
		return nil, "", errNoMirror
	}
	format := detected.MirrorFormat
	mirrorPath := buildMirrorPath(workspaceRoot, entry.RelativePath, format)

	e.Logger.Debug().
		Str("file", entry.RelativePath).
		Str("extension", entry.Extension).
		Str("format", string(format)).
		Str("mirror_path", mirrorPath).
		Int64("file_size", entry.FileSize).
		Msg("Checking if mirror file needs to be created/updated")

	sourceInfo, err := os.Stat(entry.AbsolutePath)
	if err != nil {
		e.Logger.Error().Err(err).
			Str("file", entry.AbsolutePath).
			Msg("Failed to stat source file")
		return nil, "", err
	}

	if info, err := os.Stat(mirrorPath); err == nil {
		if info.ModTime().After(sourceInfo.ModTime()) && info.Size() > 0 {
			e.Logger.Debug().
				Str("file", entry.RelativePath).
				Str("mirror_path", mirrorPath).
				Time("source_mtime", sourceInfo.ModTime()).
				Time("mirror_mtime", info.ModTime()).
				Int64("mirror_size", info.Size()).
				Msg("Using existing mirror file (up to date)")
			content, readErr := os.ReadFile(mirrorPath)
			if readErr == nil {
				meta := entity.MirrorMetadata{
					Format:      format,
					Path:        mirrorPath,
					SourceMtime: sourceInfo.ModTime(),
					UpdatedAt:   info.ModTime(),
				}
				return &meta, string(content), nil
			}
			e.Logger.Warn().Err(readErr).
				Str("mirror_path", mirrorPath).
				Msg("Failed to read existing mirror file, will regenerate")
		} else {
			e.Logger.Debug().
				Str("file", entry.RelativePath).
				Str("mirror_path", mirrorPath).
				Time("source_mtime", sourceInfo.ModTime()).
				Time("mirror_mtime", info.ModTime()).
				Int64("mirror_size", info.Size()).
				Msg("Mirror file is outdated or empty, will regenerate")
		}
	} else {
		e.Logger.Debug().
			Str("file", entry.RelativePath).
			Str("mirror_path", mirrorPath).
			Msg("Mirror file does not exist, will create")
	}

	e.Logger.Info().
		Str("file", entry.RelativePath).
		Str("extension", entry.Extension).
		Str("format", string(format)).
		Msg("Extracting content for mirror file")

	content, err := e.extract(ctx, entry.AbsolutePath, detected)
	if err != nil {
		e.Logger.Error().Err(err).
			Str("file", entry.RelativePath).
			Str("extension", entry.Extension).
			Msg("Failed to extract content for mirror")
		return nil, "", err
	}
	if strings.TrimSpace(content) == "" {
		e.Logger.Debug().
			Str("file", entry.RelativePath).
			Msg("Extracted content is empty, skipping mirror creation")
		return nil, "", errNoMirror
	}

	contentSize := len([]byte(content))
	e.Logger.Debug().
		Str("file", entry.RelativePath).
		Int("content_size", contentSize).
		Msg("Extracted content, writing mirror file")

	if err := os.MkdirAll(filepath.Dir(mirrorPath), 0755); err != nil {
		e.Logger.Error().Err(err).
			Str("dir", filepath.Dir(mirrorPath)).
			Msg("Failed to create mirror directory")
		return nil, "", err
	}
	e.Logger.Debug().
		Str("dir", filepath.Dir(mirrorPath)).
		Msg("Created mirror directory")

	if err := os.WriteFile(mirrorPath, []byte(content), 0644); err != nil {
		e.Logger.Error().Err(err).
			Str("mirror_path", mirrorPath).
			Int("content_size", contentSize).
			Msg("Failed to write mirror file")
		return nil, "", err
	}

	e.Logger.Info().
		Str("file", entry.RelativePath).
		Str("mirror_path", mirrorPath).
		Int("content_size", contentSize).
		Str("format", string(format)).
		Msg("Created mirror file")

	meta := entity.MirrorMetadata{
		Format:      format,
		Path:        mirrorPath,
		SourceMtime: sourceInfo.ModTime(),
		UpdatedAt:   time.Now(),
	}
	return &meta, content, nil
}

func buildMirrorPath(workspaceRoot, relativePath string, format entity.MirrorFormat) string {
	normalized := filepath.FromSlash(relativePath)
	return filepath.Join(workspaceRoot, ".cortex", "mirror", normalized+"."+string(format))
}

func (e *Extractor) extract(ctx context.Context, absolutePath string, detected inputDetection) (string, error) {
	switch detected.Kind {
	case "pdf":
		return extractPDF(ctx, absolutePath)
	case "docx":
		if content, ok, err := extractWithPandoc(ctx, absolutePath, "docx"); ok {
			return cleanText(content), err
		}
		content, err := extractDocxText(absolutePath)
		return cleanText(content), err
	case "doc":
		return extractLegacy(ctx, absolutePath, ".docx", func(convertedPath string) (string, error) {
			if content, ok, err := extractWithPandoc(ctx, convertedPath, "docx"); ok {
				return cleanText(content), err
			}
			content, err := extractDocxText(convertedPath)
			return cleanText(content), err
		})
	case "pptx":
		if content, ok, err := extractWithPandoc(ctx, absolutePath, "pptx"); ok {
			return cleanText(content), err
		}
		content, err := extractPptxText(absolutePath)
		return cleanText(content), err
	case "ppt":
		return extractLegacy(ctx, absolutePath, ".pptx", func(convertedPath string) (string, error) {
			if content, ok, err := extractWithPandoc(ctx, convertedPath, "pptx"); ok {
				return cleanText(content), err
			}
			content, err := extractPptxText(convertedPath)
			return cleanText(content), err
		})
	case "odt":
		if content, ok, err := extractWithPandoc(ctx, absolutePath, "odt"); ok {
			return cleanText(content), err
		}
		content, err := extractOdtText(absolutePath)
		return cleanText(content), err
	case "xlsx":
		return extractSpreadsheetCSV(absolutePath)
	case "xls":
		return extractLegacy(ctx, absolutePath, ".xlsx", extractSpreadsheetCSV)
	case "ods":
		return extractLegacy(ctx, absolutePath, ".xlsx", extractSpreadsheetCSV)
	case "csv":
		return extractDelimitedCSV(absolutePath, ',')
	case "tsv":
		return extractDelimitedCSV(absolutePath, '\t')
	default:
		if detected.PandocFormat != "" {
			if content, ok, err := extractWithPandoc(ctx, absolutePath, detected.PandocFormat); ok {
				if strings.TrimSpace(content) == "" && err != nil && detected.IsText {
					return "", errNoMirror
				}
				return cleanText(content), err
			}
		}
		if detected.IsText {
			return extractWithPandocGuessing(ctx, absolutePath)
		}
		return "", errNoMirror
	}
}

func extractPDF(ctx context.Context, absolutePath string) (string, error) {
	if content, ok, err := extractWithPandoc(ctx, absolutePath, "pdf"); ok {
		clean := cleanText(content)
		if !isLowQuality(clean) {
			return clean, err
		}
	}
	if content, ok, err := extractWithPdftotext(ctx, absolutePath); ok {
		clean := cleanText(content)
		if !isLowQuality(clean) {
			return clean, err
		}
	}
	content, err := extractPDFText(ctx, absolutePath)
	return cleanText(content), err
}

func extractWithPandoc(ctx context.Context, absolutePath, inputFormat string) (string, bool, error) {
	if !pandocAvailable() {
		return "", false, nil
	}
	args := []string{
		"--wrap=none",
		"-f", inputFormat,
		"-t", "gfm",
		absolutePath,
	}
	cmd := exec.CommandContext(ctx, "pandoc", args...)
	out, err := cmd.Output()
	content := strings.TrimSpace(string(out))
	if content == "" {
		if err != nil {
			return "", true, err
		}
		return "", true, errNoMirror
	}
	if err != nil {
		// Keep usable output even if pandoc exits non-zero.
		return content, true, nil
	}
	return content, true, nil
}

func extractWithPandocGuessing(ctx context.Context, absolutePath string) (string, error) {
	if !pandocAvailable() {
		return "", errNoMirror
	}
	for _, format := range pandocInputFormats() {
		if !shouldTryPandocFormat(format) {
			continue
		}
		content, ok, err := extractWithPandoc(ctx, absolutePath, format)
		if !ok {
			return "", errNoMirror
		}
		clean := cleanText(content)
		if strings.TrimSpace(clean) != "" {
			return clean, err
		}
	}
	return "", errNoMirror
}

func extractWithPdftotext(ctx context.Context, absolutePath string) (string, bool, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", false, nil
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", absolutePath, "-")
	out, err := cmd.Output()
	content := strings.TrimSpace(string(out))
	if content == "" {
		if err != nil {
			return "", true, err
		}
		return "", true, errNoMirror
	}
	if err != nil {
		// Keep usable output even if pdftotext exits non-zero.
		return content, true, nil
	}
	return content, true, nil
}

func cleanText(text string) string {
	if text == "" {
		return text
	}
	// Strip control chars and normalize whitespace.
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\r':
			continue
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 32:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	cleaned := strings.Join(lines, "\n")
	// Collapse excessive blank lines.
	cleaned = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleaned, "\n\n")
	return strings.TrimSpace(cleaned)
}

func isLowQuality(text string) bool {
	if len(text) < 200 {
		return true
	}
	letters := 0
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x00C0 && r <= 0x017F) {
			letters++
		}
	}
	return float64(letters)/float64(len([]rune(text))) < 0.2
}

func extractPDFText(ctx context.Context, absolutePath string) (string, error) {
	reader, file, err := openPDF(absolutePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var builder strings.Builder
	totalPage := reader.NumPage()
	for i := 1; i <= totalPage; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		builder.WriteString(text)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), nil
}

func openPDF(path string) (*pdf.Reader, *os.File, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return reader, file, nil
}

func extractDocxText(absolutePath string) (string, error) {
	return extractXMLFromZip(absolutePath, "word/document.xml", `<w:t[^>]*>([\s\S]*?)</w:t>`, "\n\n")
}

func extractPptxText(absolutePath string) (string, error) {
	zipReader, err := zip.OpenReader(absolutePath)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	var slideFiles []string
	entries := map[string]*zip.File{}
	for _, f := range zipReader.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideFiles = append(slideFiles, f.Name)
			entries[f.Name] = f
		}
	}
	if len(slideFiles) == 0 {
		return "", nil
	}
	sort.Strings(slideFiles)

	var slides []string
	re := regexp.MustCompile(`<a:t[^>]*>([\s\S]*?)</a:t>`)
	for _, name := range slideFiles {
		entry := entries[name]
		content, err := readZipFile(entry)
		if err != nil {
			continue
		}
		slides = append(slides, strings.TrimSpace(strings.Join(extractXMLText(string(content), re), " ")))
	}
	return strings.TrimSpace(strings.Join(filterNonEmpty(slides), "\n\n")), nil
}

func extractOdtText(absolutePath string) (string, error) {
	return extractXMLFromZip(absolutePath, "content.xml", `<text:p[^>]*>([\s\S]*?)</text:p>`, "\n\n")
}

func extractSpreadsheetCSV(absolutePath string) (string, error) {
	f, err := excelize.OpenFile(absolutePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", nil
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func extractDelimitedCSV(absolutePath string, delimiter rune) (string, error) {
	file, err := os.Open(absolutePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func extractLegacy(ctx context.Context, absolutePath, targetExtension string, extractor func(string) (string, error)) (string, error) {
	convertedPath, cleanup, err := convertLegacy(ctx, absolutePath, targetExtension)
	if err != nil {
		return "", err
	}
	defer cleanup()
	return extractor(convertedPath)
}

func convertLegacy(ctx context.Context, absolutePath, targetExtension string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "cortex-mirror-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	cmd := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", strings.TrimPrefix(targetExtension, "."), "--outdir", tmpDir, absolutePath)
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, err
	}

	base := strings.TrimSuffix(filepath.Base(absolutePath), filepath.Ext(absolutePath))
	converted := filepath.Join(tmpDir, base+targetExtension)
	if _, err := os.Stat(converted); err != nil {
		cleanup()
		return "", nil, err
	}
	return converted, cleanup, nil
}

func extractXMLFromZip(absolutePath, entryName, pattern, separator string) (string, error) {
	re := regexp.MustCompile(pattern)
	zipReader, err := zip.OpenReader(absolutePath)
	if err != nil {
		return "", err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		if f.Name == entryName {
			data, err := readZipFile(f)
			if err != nil {
				return "", err
			}
			chunks := extractXMLText(string(data), re)
			return strings.TrimSpace(strings.Join(filterNonEmpty(chunks), separator)), nil
		}
	}
	return "", nil
}

func extractXMLText(xmlContent string, re *regexp.Regexp) []string {
	matches := re.FindAllStringSubmatch(xmlContent, -1)
	results := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		text := decodeEntities(stripTags(match[1]))
		if strings.TrimSpace(text) != "" {
			results = append(results, text)
		}
	}
	return results
}

func stripTags(value string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(value, "")
}

func decodeEntities(value string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", "\"",
		"&apos;", "'",
	)
	return replacer.Replace(value)
}

func readZipFile(entry *zip.File) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("zip entry missing")
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func filterNonEmpty(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			trimmed = append(trimmed, val)
		}
	}
	return trimmed
}

var (
	pandocAvailableOnce  sync.Once
	pandocAvailableValue bool
	pandocFormatsOnce    sync.Once
	pandocFormats        []string
)

var defaultPandocInputFormats = []string{
	"asciidoc",
	"biblatex",
	"bibtex",
	"bits",
	"commonmark",
	"commonmark_x",
	"creole",
	"csljson",
	"csv",
	"djot",
	"docbook",
	"docx",
	"dokuwiki",
	"endnotexml",
	"epub",
	"fb2",
	"gfm",
	"haddock",
	"html",
	"ipynb",
	"jats",
	"jira",
	"json",
	"latex",
	"man",
	"markdown",
	"markdown_github",
	"markdown_mmd",
	"markdown_phpextra",
	"markdown_strict",
	"mdoc",
	"mediawiki",
	"muse",
	"native",
	"odt",
	"opml",
	"org",
	"pod",
	"pptx",
	"ris",
	"rst",
	"rtf",
	"t2t",
	"textile",
	"tikiwiki",
	"tsv",
	"twiki",
	"typst",
	"vimwiki",
	"xlsx",
	"xml",
}

var (
	bibtexRe   = regexp.MustCompile(`(?m)^@\w+\s*[{(]`)
	risRe      = regexp.MustCompile(`(?m)^TY\s{2}-`)
	orgRe      = regexp.MustCompile(`(?m)^\*+\s+`)
	rstRe      = regexp.MustCompile(`(?m)^.+\n[=\\-]{3,}\s*$`)
	latexRe    = regexp.MustCompile(`\\documentclass|\\begin\{document\}`)
	markdownRe = regexp.MustCompile("(?m)^#{1,6}\\s+|\\[[^\\]]+\\]\\([^\\)]+\\)|```")
)

func pandocAvailable() bool {
	pandocAvailableOnce.Do(func() {
		_, err := exec.LookPath("pandoc")
		pandocAvailableValue = err == nil
	})
	return pandocAvailableValue
}

func pandocInputFormats() []string {
	pandocFormatsOnce.Do(func() {
		if !pandocAvailable() {
			pandocFormats = defaultPandocInputFormats
			return
		}
		cmd := exec.Command("pandoc", "--list-input-formats")
		out, err := cmd.Output()
		if err != nil {
			pandocFormats = defaultPandocInputFormats
			return
		}
		fields := strings.Fields(string(out))
		if len(fields) == 0 {
			pandocFormats = defaultPandocInputFormats
			return
		}
		pandocFormats = fields
	})
	return pandocFormats
}

func shouldTryPandocFormat(format string) bool {
	switch format {
	case "docx", "pptx", "xlsx", "odt", "epub", "pdf":
		return false
	default:
		return true
	}
}

func detectInputType(absolutePath string) (inputDetection, error) {
	header, err := readFilePrefix(absolutePath, 64*1024)
	if err != nil {
		return inputDetection{}, err
	}
	if isPDFMagic(header) {
		return inputDetection{
			Kind:         "pdf",
			PandocFormat: "pdf",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	}
	if isZipMagic(header) {
		return detectZipInputType(absolutePath)
	}
	if isOLEMagic(header) {
		return detectOLEInputType(absolutePath)
	}
	if isProbablyText(header) {
		return detectTextInputType(header), nil
	}
	return inputDetection{}, nil
}

func detectZipInputType(absolutePath string) (inputDetection, error) {
	reader, err := zip.OpenReader(absolutePath)
	if err != nil {
		return inputDetection{}, err
	}
	defer reader.Close()

	var (
		hasDocx bool
		hasPptx bool
		hasXlsx bool
		mime    string
	)

	for _, f := range reader.File {
		switch f.Name {
		case "word/document.xml":
			hasDocx = true
		case "ppt/presentation.xml":
			hasPptx = true
		case "xl/workbook.xml":
			hasXlsx = true
		case "mimetype":
			if mime == "" {
				if data, readErr := readZipFile(f); readErr == nil {
					mime = strings.TrimSpace(string(data))
				}
			}
		}
	}

	switch mime {
	case "application/vnd.oasis.opendocument.text":
		return inputDetection{
			Kind:         "odt",
			PandocFormat: "odt",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	case "application/vnd.oasis.opendocument.spreadsheet":
		return inputDetection{
			Kind:         "ods",
			MirrorFormat: entity.MirrorFormatCSV,
		}, nil
	case "application/epub+zip":
		return inputDetection{
			Kind:         "epub",
			PandocFormat: "epub",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	}

	if hasDocx {
		return inputDetection{
			Kind:         "docx",
			PandocFormat: "docx",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	}
	if hasPptx {
		return inputDetection{
			Kind:         "pptx",
			PandocFormat: "pptx",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	}
	if hasXlsx {
		return inputDetection{
			Kind:         "xlsx",
			MirrorFormat: entity.MirrorFormatCSV,
		}, nil
	}

	for _, f := range reader.File {
		if f.Name == "META-INF/container.xml" || strings.HasSuffix(f.Name, ".opf") {
			return inputDetection{
				Kind:         "epub",
				PandocFormat: "epub",
				MirrorFormat: entity.MirrorFormatMarkdown,
			}, nil
		}
	}

	return inputDetection{}, nil
}

func detectOLEInputType(absolutePath string) (inputDetection, error) {
	data, err := readFilePrefix(absolutePath, 1024*1024)
	if err != nil {
		return inputDetection{}, err
	}
	switch {
	case bytes.Contains(data, []byte("WordDocument")):
		return inputDetection{
			Kind:         "doc",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	case bytes.Contains(data, []byte("PowerPoint Document")):
		return inputDetection{
			Kind:         "ppt",
			MirrorFormat: entity.MirrorFormatMarkdown,
		}, nil
	case bytes.Contains(data, []byte("Workbook")):
		return inputDetection{
			Kind:         "xls",
			MirrorFormat: entity.MirrorFormatCSV,
		}, nil
	}
	return inputDetection{}, nil
}

func detectTextInputType(data []byte) inputDetection {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return inputDetection{
			Kind:         "text",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	}

	if delim, ok := detectDelimitedFormat(text); ok {
		kind := "csv"
		if delim == '\t' {
			kind = "tsv"
		}
		return inputDetection{
			Kind:         kind,
			MirrorFormat: entity.MirrorFormatCSV,
			IsText:       true,
		}
	}

	if format := detectJSONFormat(text); format != "" {
		return inputDetection{
			Kind:         format,
			PandocFormat: format,
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	}

	if format := detectXMLFormat(text); format != "" {
		return inputDetection{
			Kind:         format,
			PandocFormat: format,
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	}

	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, "{\\rtf"):
		return inputDetection{
			Kind:         "rtf",
			PandocFormat: "rtf",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html"):
		return inputDetection{
			Kind:         "html",
			PandocFormat: "html",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case latexRe.MatchString(text):
		return inputDetection{
			Kind:         "latex",
			PandocFormat: "latex",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case orgRe.MatchString(text):
		return inputDetection{
			Kind:         "org",
			PandocFormat: "org",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case rstRe.MatchString(text):
		return inputDetection{
			Kind:         "rst",
			PandocFormat: "rst",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case bibtexRe.MatchString(text):
		return inputDetection{
			Kind:         "bibtex",
			PandocFormat: "bibtex",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case risRe.MatchString(text):
		return inputDetection{
			Kind:         "ris",
			PandocFormat: "ris",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	case markdownRe.MatchString(text):
		return inputDetection{
			Kind:         "gfm",
			PandocFormat: "gfm",
			MirrorFormat: entity.MirrorFormatMarkdown,
			IsText:       true,
		}
	}

	return inputDetection{
		Kind:         "text",
		PandocFormat: "markdown",
		MirrorFormat: entity.MirrorFormatMarkdown,
		IsText:       true,
	}
}

func detectJSONFormat(text string) string {
	if !json.Valid([]byte(text)) {
		return ""
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "\"nbformat\"") && strings.Contains(lower, "\"cells\""):
		return "ipynb"
	case strings.Contains(lower, "\"pandoc-api-version\"") || strings.Contains(lower, "\"blocks\""):
		return "json"
	case strings.Contains(lower, "\"items\"") || strings.Contains(lower, "\"references\""):
		return "csljson"
	default:
		return ""
	}
}

func detectXMLFormat(text string) string {
	if !strings.HasPrefix(strings.TrimSpace(text), "<") {
		return ""
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "<html") {
		return "html"
	}
	if strings.Contains(lower, "docbook") {
		return "docbook"
	}
	if strings.Contains(lower, "jats") {
		return "jats"
	}
	if strings.Contains(lower, "tei") {
		return "tei"
	}
	if strings.Contains(lower, "<opml") {
		return "opml"
	}
	return "xml"
}

func detectDelimitedFormat(text string) (rune, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return 0, false
	}
	if delim, ok := detectDelimiter(lines, ','); ok {
		return delim, ok
	}
	return detectDelimiter(lines, '\t')
}

func detectDelimiter(lines []string, delim rune) (rune, bool) {
	counts := make([]int, 0, 5)
	for _, line := range lines {
		if len(counts) >= 5 {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		counts = append(counts, strings.Count(line, string(delim)))
	}
	if len(counts) < 2 {
		return 0, false
	}
	minCount, maxCount := counts[0], counts[0]
	for _, count := range counts[1:] {
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}
	if minCount == 0 || maxCount != minCount {
		return 0, false
	}
	return delim, true
}

func isPDFMagic(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF-"))
}

func isZipMagic(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	prefix := string(data[:4])
	return prefix == "PK\x03\x04" || prefix == "PK\x05\x06" || prefix == "PK\x07\x08"
}

func isOLEMagic(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
}

func isProbablyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0x00) != -1 {
		return false
	}
	if !utf8.Valid(data) {
		return false
	}
	nonPrintable := 0
	total := 0
	for _, r := range string(data) {
		total++
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 32 {
			nonPrintable++
		}
	}
	if total == 0 {
		return false
	}
	return float64(nonPrintable)/float64(total) < 0.3
}

func readFilePrefix(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

var errNoMirror = errors.New("no mirror content")
