package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/rs/zerolog"
)

// MetadataEnrichmentService enriches document metadata by querying external APIs
// (Amazon, Open Library, Google Books) when ISBN is available.
type MetadataEnrichmentService struct {
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewMetadataEnrichmentService creates a new metadata enrichment service.
func NewMetadataEnrichmentService(logger zerolog.Logger) *MetadataEnrichmentService {
	return &MetadataEnrichmentService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With().Str("component", "metadata_enrichment").Logger(),
	}
}

// EnrichedBookMetadata contains metadata enriched from external APIs.
type EnrichedBookMetadata struct {
	// Basic Information
	Title           string   `json:"title"`
	Subtitle        string   `json:"subtitle,omitempty"`
	Authors         []string `json:"authors"`
	Publisher       string   `json:"publisher,omitempty"`
	PublicationDate string   `json:"publication_date,omitempty"` // YYYY-MM-DD
	ISBN10          string   `json:"isbn10,omitempty"`
	ISBN13          string   `json:"isbn13,omitempty"`
	Pages           int      `json:"pages,omitempty"`
	Language        string   `json:"language,omitempty"`
	
	// Descriptions
	Description     string   `json:"description,omitempty"`
	ShortDescription string  `json:"short_description,omitempty"`
	
	// Categorization
	Categories      []string `json:"categories,omitempty"`
	Genre           string   `json:"genre,omitempty"`
	Subject         string   `json:"subject,omitempty"`
	
	// Ratings and Reviews
	Rating          float64  `json:"rating,omitempty"`
	RatingCount     int      `json:"rating_count,omitempty"`
	ReviewCount     int      `json:"review_count,omitempty"`
	
	// Images
	CoverImageURL   string   `json:"cover_image_url,omitempty"`
	ThumbnailURL     string   `json:"thumbnail_url,omitempty"`
	
	// Links
	AmazonURL       string   `json:"amazon_url,omitempty"`
	GoodreadsURL   string   `json:"goodreads_url,omitempty"`
	
	// Additional Metadata
	Edition         string   `json:"edition,omitempty"`
	Format          string   `json:"format,omitempty"` // Hardcover, Paperback, eBook, etc.
	Dimensions      string   `json:"dimensions,omitempty"`
	Weight          string   `json:"weight,omitempty"`
	
	// Source information
	Source          string   `json:"source"` // "amazon", "open_library", "google_books"
	EnrichedAt     time.Time `json:"enriched_at"`
}

// EnrichWithISBN enriches book metadata by querying external APIs using ISBN.
// It tries multiple sources in order: Open Library, Google Books, Amazon.
func (s *MetadataEnrichmentService) EnrichWithISBN(ctx context.Context, isbn string) (*EnrichedBookMetadata, error) {
	// Clean ISBN (remove hyphens, spaces)
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	
	if len(isbn) != 10 && len(isbn) != 13 {
		return nil, fmt.Errorf("invalid ISBN length: %d (expected 10 or 13)", len(isbn))
	}

	s.logger.Debug().Str("isbn", isbn).Msg("Enriching metadata with ISBN")

	// Try Open Library first (free, no API key required)
	if enriched, err := s.enrichFromOpenLibrary(ctx, isbn); err == nil && enriched != nil {
		s.logger.Info().Str("isbn", isbn).Str("source", "open_library").Msg("Successfully enriched metadata from Open Library")
		return enriched, nil
	} else {
		s.logger.Debug().Err(err).Str("isbn", isbn).Msg("Open Library enrichment failed, trying next source")
	}

	// Try Google Books (free, no API key required)
	if enriched, err := s.enrichFromGoogleBooks(ctx, isbn); err == nil && enriched != nil {
		s.logger.Info().Str("isbn", isbn).Str("source", "google_books").Msg("Successfully enriched metadata from Google Books")
		return enriched, nil
	} else {
		s.logger.Debug().Err(err).Str("isbn", isbn).Msg("Google Books enrichment failed, trying next source")
	}

	// Try Amazon (requires API key or scraping - for now we'll skip)
	// Amazon Product Advertising API requires AWS credentials
	// For now, we'll return an error if other sources fail
	return nil, fmt.Errorf("could not enrich metadata from any source for ISBN: %s", isbn)
}

// enrichFromOpenLibrary queries Open Library API for book metadata.
func (s *MetadataEnrichmentService) enrichFromOpenLibrary(ctx context.Context, isbn string) (*EnrichedBookMetadata, error) {
	// Open Library API: https://openlibrary.org/api/books?bibkeys=ISBN:{isbn}&format=json&jscmd=data
	apiURL := fmt.Sprintf("https://openlibrary.org/api/books?bibkeys=ISBN:%s&format=json&jscmd=data", url.QueryEscape(isbn))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Open Library API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var openLibResponse map[string]interface{}
	if err := json.Unmarshal(body, &openLibResponse); err != nil {
		return nil, err
	}

	// Extract data for this ISBN
	isbnKey := fmt.Sprintf("ISBN:%s", isbn)
	bookData, ok := openLibResponse[isbnKey].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no data found for ISBN %s", isbn)
	}

	enriched := &EnrichedBookMetadata{
		Source:     "open_library",
		EnrichedAt: time.Now(),
	}

	// Extract title
	if title, ok := bookData["title"].(string); ok {
		enriched.Title = title
	}

	// Extract authors
	if authors, ok := bookData["authors"].([]interface{}); ok {
		for _, author := range authors {
			if authorMap, ok := author.(map[string]interface{}); ok {
				if name, ok := authorMap["name"].(string); ok {
					enriched.Authors = append(enriched.Authors, name)
				}
			}
		}
	}

	// Extract publisher
	if publishers, ok := bookData["publishers"].([]interface{}); ok && len(publishers) > 0 {
		if pubMap, ok := publishers[0].(map[string]interface{}); ok {
			if name, ok := pubMap["name"].(string); ok {
				enriched.Publisher = name
			}
		}
	}

	// Extract publish date
	if publishDate, ok := bookData["publish_date"].(string); ok {
		enriched.PublicationDate = publishDate
	}

	// Extract number of pages
	if numPages, ok := bookData["number_of_pages"].(float64); ok {
		enriched.Pages = int(numPages)
	}

	// Extract ISBNs
	if identifiers, ok := bookData["identifiers"].(map[string]interface{}); ok {
		if isbn10, ok := identifiers["isbn_10"].([]interface{}); ok && len(isbn10) > 0 {
			if isbn10Str, ok := isbn10[0].(string); ok {
				enriched.ISBN10 = isbn10Str
			}
		}
		if isbn13, ok := identifiers["isbn_13"].([]interface{}); ok && len(isbn13) > 0 {
			if isbn13Str, ok := isbn13[0].(string); ok {
				enriched.ISBN13 = isbn13Str
			}
		}
	}

	// Extract cover image
	if cover, ok := bookData["cover"].(map[string]interface{}); ok {
		if large, ok := cover["large"].(string); ok {
			enriched.CoverImageURL = large
		} else if medium, ok := cover["medium"].(string); ok {
			enriched.CoverImageURL = medium
		} else if small, ok := cover["small"].(string); ok {
			enriched.ThumbnailURL = small
		}
	}

	// Extract subjects/categories
	if subjects, ok := bookData["subjects"].([]interface{}); ok {
		for _, subject := range subjects {
			if subjectMap, ok := subject.(map[string]interface{}); ok {
				if name, ok := subjectMap["name"].(string); ok {
					enriched.Categories = append(enriched.Categories, name)
				}
			}
		}
	}

	return enriched, nil
}

// enrichFromGoogleBooks queries Google Books API for book metadata.
func (s *MetadataEnrichmentService) enrichFromGoogleBooks(ctx context.Context, isbn string) (*EnrichedBookMetadata, error) {
	// Google Books API: https://www.googleapis.com/books/v1/volumes?q=isbn:{isbn}
	apiURL := fmt.Sprintf("https://www.googleapis.com/books/v1/volumes?q=isbn:%s", url.QueryEscape(isbn))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Books API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gbResponse struct {
		Items []struct {
			VolumeInfo struct {
				Title               string   `json:"title"`
				Subtitle            string   `json:"subtitle"`
				Authors             []string `json:"authors"`
				Publisher           string   `json:"publisher"`
				PublishedDate       string   `json:"publishedDate"`
				Description         string   `json:"description"`
				PageCount           int      `json:"pageCount"`
				Language            string   `json:"language"`
				Categories          []string `json:"categories"`
				AverageRating       float64  `json:"averageRating"`
				RatingsCount        int      `json:"ratingsCount"`
				ImageLinks          struct {
					Thumbnail string `json:"thumbnail"`
					Small     string `json:"small"`
					Medium    string `json:"medium"`
					Large     string `json:"large"`
				} `json:"imageLinks"`
				IndustryIdentifiers []struct {
					Type       string `json:"type"` // "ISBN_10" or "ISBN_13"
					Identifier string `json:"identifier"`
				} `json:"industryIdentifiers"`
				PreviewLink string `json:"previewLink"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &gbResponse); err != nil {
		return nil, err
	}

	if len(gbResponse.Items) == 0 {
		return nil, fmt.Errorf("no results found for ISBN %s", isbn)
	}

	volumeInfo := gbResponse.Items[0].VolumeInfo

	enriched := &EnrichedBookMetadata{
		Source:     "google_books",
		EnrichedAt: time.Now(),
		Title:      volumeInfo.Title,
		Subtitle:   volumeInfo.Subtitle,
		Authors:    volumeInfo.Authors,
		Publisher:  volumeInfo.Publisher,
		PublicationDate: volumeInfo.PublishedDate,
		Description: volumeInfo.Description,
		Pages:      volumeInfo.PageCount,
		Language:   volumeInfo.Language,
		Categories: volumeInfo.Categories,
		Rating:     volumeInfo.AverageRating,
		RatingCount: volumeInfo.RatingsCount,
	}

	// Extract ISBNs
	for _, id := range volumeInfo.IndustryIdentifiers {
		if id.Type == "ISBN_10" {
			enriched.ISBN10 = id.Identifier
		} else if id.Type == "ISBN_13" {
			enriched.ISBN13 = id.Identifier
		}
	}

	// Extract cover image (prefer large, fallback to medium/small)
	if volumeInfo.ImageLinks.Large != "" {
		enriched.CoverImageURL = volumeInfo.ImageLinks.Large
	} else if volumeInfo.ImageLinks.Medium != "" {
		enriched.CoverImageURL = volumeInfo.ImageLinks.Medium
	} else if volumeInfo.ImageLinks.Small != "" {
		enriched.CoverImageURL = volumeInfo.ImageLinks.Small
	} else if volumeInfo.ImageLinks.Thumbnail != "" {
		enriched.ThumbnailURL = volumeInfo.ImageLinks.Thumbnail
	}

	// Extract short description (first 200 chars)
	if len(volumeInfo.Description) > 200 {
		enriched.ShortDescription = volumeInfo.Description[:200] + "..."
	} else {
		enriched.ShortDescription = volumeInfo.Description
	}

	return enriched, nil
}

// EnrichAIContext enriches an AIContext with external API data if ISBN is available.
// Returns the enriched metadata and updates aiContext with missing fields.
func (s *MetadataEnrichmentService) EnrichAIContext(ctx context.Context, aiContext *entity.AIContext) (*entity.EnrichedMetadata, error) {
	// Try to find ISBN in AIContext
	var isbn string
	if aiContext.ISBN != nil && *aiContext.ISBN != "" {
		isbn = *aiContext.ISBN
	} else if aiContext.ISSN != nil && *aiContext.ISSN != "" {
		// ISSN is different, but we can try
		s.logger.Debug().Str("issn", *aiContext.ISSN).Msg("Found ISSN instead of ISBN, skipping enrichment")
		return nil, fmt.Errorf("ISSN found instead of ISBN, cannot enrich")
	} else {
		return nil, fmt.Errorf("no ISBN found in AIContext")
	}

	// Enrich with ISBN
	enriched, err := s.EnrichWithISBN(ctx, isbn)
	if err != nil {
		return nil, err
	}

	// Convert to entity.EnrichedMetadata
	enrichedMeta := &entity.EnrichedMetadata{
		Title:            enriched.Title,
		Subtitle:         enriched.Subtitle,
		Authors:          enriched.Authors,
		Publisher:        enriched.Publisher,
		PublicationDate:  enriched.PublicationDate,
		ISBN10:           enriched.ISBN10,
		ISBN13:           enriched.ISBN13,
		Pages:            enriched.Pages,
		Language:         enriched.Language,
		Description:      enriched.Description,
		ShortDescription: enriched.ShortDescription,
		Categories:       enriched.Categories,
		Genre:            enriched.Genre,
		Subject:          enriched.Subject,
		Rating:           enriched.Rating,
		RatingCount:      enriched.RatingCount,
		ReviewCount:      enriched.ReviewCount,
		CoverImageURL:    enriched.CoverImageURL,
		ThumbnailURL:     enriched.ThumbnailURL,
		AmazonURL:        enriched.AmazonURL,
		GoodreadsURL:     enriched.GoodreadsURL,
		Edition:          enriched.Edition,
		Format:           enriched.Format,
		Dimensions:       enriched.Dimensions,
		Weight:           enriched.Weight,
		Source:           enriched.Source,
		EnrichedAt:       enriched.EnrichedAt,
	}

	// Merge enriched data back into AIContext if fields are missing
	if len(aiContext.Authors) == 0 && len(enriched.Authors) > 0 {
		for _, author := range enriched.Authors {
			aiContext.Authors = append(aiContext.Authors, entity.AuthorInfo{
				Name:       author,
				Role:       "author",
				Confidence: 0.9, // High confidence from external API
			})
		}
	}

	if aiContext.Publisher == nil && enriched.Publisher != "" {
		aiContext.Publisher = &enriched.Publisher
	}

	if aiContext.PublicationYear == nil && enriched.PublicationDate != "" {
		// Try to extract year from publication date
		if year, err := extractYearFromDate(enriched.PublicationDate); err == nil {
			aiContext.PublicationYear = &year
		}
	}

	// Store enriched metadata in AIContext
	aiContext.EnrichedMetadata = enrichedMeta

	return enrichedMeta, nil
}

// extractYearFromDate extracts year from various date formats.
func extractYearFromDate(dateStr string) (int, error) {
	// Try common formats: "2024", "2024-01-15", "January 2024", etc.
	dateStr = strings.TrimSpace(dateStr)
	
	// Try YYYY format
	if len(dateStr) >= 4 {
		if year, err := parseInt(dateStr[:4]); err == nil && year >= 1000 && year <= 3000 {
			return year, nil
		}
	}
	
	// Try YYYY-MM-DD format
	if len(dateStr) >= 10 {
		if year, err := parseInt(dateStr[:4]); err == nil && year >= 1000 && year <= 3000 {
			return year, nil
		}
	}
	
	return 0, fmt.Errorf("could not extract year from date: %s", dateStr)
}

// parseInt is a simple helper to parse integer from string.
func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit: %c", c)
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}

