package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/google/uuid"
)

// ConfigVersionRepository manages versioned configuration snapshots.
type ConfigVersionRepository struct {
	conn *Connection
}

// NewConfigVersionRepository creates a new config version repository.
func NewConfigVersionRepository(conn *Connection) *ConfigVersionRepository {
	return &ConfigVersionRepository{conn: conn}
}

// ConfigVersion represents a versioned configuration snapshot.
type ConfigVersion struct {
	VersionID  string
	CreatedAt  time.Time
	CreatedBy  string
	Description string
	Config     *config.Config
	Metadata   map[string]string
}

// Create creates a new configuration version.
func (r *ConfigVersionRepository) Create(ctx context.Context, cfg *config.Config, createdBy, description string, metadata map[string]string) (*ConfigVersion, error) {
	versionID := uuid.New().String()
	createdAt := time.Now()

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var metadataJSON []byte
	if len(metadata) > 0 {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO config_versions (
			version_id, created_at, created_by, description, config_json, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?)`

	_, err = r.conn.Exec(ctx, query,
		versionID,
		createdAt.UnixMilli(),
		createdBy,
		description,
		configJSON,
		metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create config version: %w", err)
	}

	return &ConfigVersion{
		VersionID:   versionID,
		CreatedAt:   createdAt,
		CreatedBy:   createdBy,
		Description: description,
		Config:      cfg,
		Metadata:    metadata,
	}, nil
}

// Get retrieves a configuration version by ID.
func (r *ConfigVersionRepository) Get(ctx context.Context, versionID string) (*ConfigVersion, error) {
	query := `
		SELECT version_id, created_at, created_by, description, config_json, metadata_json
		FROM config_versions
		WHERE version_id = ?`

	row := r.conn.QueryRow(ctx, query, versionID)
	return r.scanConfigVersion(row)
}

// List retrieves configuration versions with pagination.
func (r *ConfigVersionRepository) List(ctx context.Context, limit, offset int) ([]*ConfigVersion, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM config_versions`
	var total int
	if err := r.conn.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count config versions: %w", err)
	}

	// Get versions
	query := `
		SELECT version_id, created_at, created_by, description, config_json, metadata_json
		FROM config_versions
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list config versions: %w", err)
	}
	defer rows.Close()

	versions := []*ConfigVersion{}
	for rows.Next() {
		version, err := r.scanConfigVersion(rows)
		if err != nil {
			return nil, 0, err
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating config versions: %w", err)
	}

	return versions, total, nil
}

// Delete deletes a configuration version.
func (r *ConfigVersionRepository) Delete(ctx context.Context, versionID string) error {
	query := `DELETE FROM config_versions WHERE version_id = ?`
	_, err := r.conn.Exec(ctx, query, versionID)
	if err != nil {
		return fmt.Errorf("failed to delete config version: %w", err)
	}
	return nil
}

// scanConfigVersion scans a row into a ConfigVersion.
func (r *ConfigVersionRepository) scanConfigVersion(scanner interface {
	Scan(dest ...interface{}) error
}) (*ConfigVersion, error) {
	var versionID, createdBy, description string
	var createdAt int64
	var configJSON []byte
	var metadataJSON []byte

	err := scanner.Scan(&versionID, &createdAt, &createdBy, &description, &configJSON, &metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("config version not found")
		}
		return nil, fmt.Errorf("failed to scan config version: %w", err)
	}

	var cfg config.Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	var metadata map[string]string
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &ConfigVersion{
		VersionID:   versionID,
		CreatedAt:   time.Unix(0, createdAt*int64(time.Millisecond)),
		CreatedBy:   createdBy,
		Description: description,
		Config:      &cfg,
		Metadata:    metadata,
	}, nil
}

