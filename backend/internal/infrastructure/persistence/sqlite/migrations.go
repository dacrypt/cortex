package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Migration represents a database migration.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// Migrator handles database migrations.
type Migrator struct {
	conn       *Connection
	migrations []Migration
}

// NewMigrator creates a new migrator.
func NewMigrator(conn *Connection) *Migrator {
	return &Migrator{
		conn:       conn,
		migrations: allMigrations(),
	}
}

// Migrate runs all pending migrations.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Create migrations table if not exists
	if err := m.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Run pending migrations
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			if err := m.runMigration(ctx, migration); err != nil {
				return fmt.Errorf("failed to run migration %d (%s): %w", migration.Version, migration.Name, err)
			}
		}
	}

	return nil
}

func (m *Migrator) createMigrationsTable(ctx context.Context) error {
	_, err := m.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000)
		)
	`)
	return err
}

func (m *Migrator) getCurrentVersion(ctx context.Context) (int, error) {
	row := m.conn.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM _migrations")
	var version int
	if err := row.Scan(&version); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return version, nil
}

func (m *Migrator) runMigration(ctx context.Context, migration Migration) error {
	return m.conn.Transaction(ctx, func(tx *sql.Tx) error {
		// Run migration SQL
		if _, err := tx.ExecContext(ctx, migration.Up); err != nil {
			return fmt.Errorf("migration SQL failed: %w", err)
		}

		// Record migration
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO _migrations (version, name) VALUES (?, ?)",
			migration.Version, migration.Name,
		); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return nil
	})
}

// allMigrations returns all defined migrations.
func allMigrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "initial_schema",
			Up: `
				-- Workspaces table
				CREATE TABLE IF NOT EXISTS workspaces (
					id TEXT PRIMARY KEY,
					path TEXT UNIQUE NOT NULL,
					name TEXT NOT NULL,
					active INTEGER NOT NULL DEFAULT 1,
					last_indexed INTEGER,
					file_count INTEGER NOT NULL DEFAULT 0,
					config TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL
				);

				CREATE INDEX IF NOT EXISTS idx_workspaces_path ON workspaces(path);
				CREATE INDEX IF NOT EXISTS idx_workspaces_active ON workspaces(active);

				-- Files table (index)
				CREATE TABLE IF NOT EXISTS files (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					absolute_path TEXT NOT NULL,
					filename TEXT NOT NULL,
					extension TEXT NOT NULL,
					file_size INTEGER NOT NULL,
					last_modified INTEGER NOT NULL,
					created_at INTEGER NOT NULL,
					enhanced TEXT,
					indexed_basic INTEGER NOT NULL DEFAULT 0,
					indexed_mime INTEGER NOT NULL DEFAULT 0,
					indexed_code INTEGER NOT NULL DEFAULT 0,
					indexed_document INTEGER NOT NULL DEFAULT 0,
					indexed_mirror INTEGER NOT NULL DEFAULT 0,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_files_workspace ON files(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_files_path ON files(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_files_extension ON files(workspace_id, extension);
				CREATE INDEX IF NOT EXISTS idx_files_modified ON files(workspace_id, last_modified);
				CREATE INDEX IF NOT EXISTS idx_files_size ON files(workspace_id, file_size);

				-- File metadata table
				CREATE TABLE IF NOT EXISTS file_metadata (
					file_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					type TEXT NOT NULL,
					notes TEXT,
					ai_summary TEXT,
					ai_summary_hash TEXT,
					ai_key_terms TEXT,
					mirror_format TEXT,
					mirror_path TEXT,
					mirror_source_mtime INTEGER,
					mirror_updated_at INTEGER,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE UNIQUE INDEX IF NOT EXISTS idx_file_metadata_path ON file_metadata(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_file_metadata_type ON file_metadata(workspace_id, type);

				-- File tags table
				CREATE TABLE IF NOT EXISTS file_tags (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					tag TEXT NOT NULL,
					PRIMARY KEY (workspace_id, file_id, tag),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(workspace_id, tag);

				-- File contexts table
				CREATE TABLE IF NOT EXISTS file_contexts (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					context TEXT NOT NULL,
					PRIMARY KEY (workspace_id, file_id, context),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_contexts_context ON file_contexts(workspace_id, context);

				-- File context suggestions table
				CREATE TABLE IF NOT EXISTS file_context_suggestions (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					context TEXT NOT NULL,
					PRIMARY KEY (workspace_id, file_id, context),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_context_suggestions_context ON file_context_suggestions(workspace_id, context);

				-- Tasks table
				CREATE TABLE IF NOT EXISTS tasks (
					id TEXT PRIMARY KEY,
					type TEXT NOT NULL,
					status TEXT NOT NULL,
					priority INTEGER NOT NULL,
					payload BLOB,
					result BLOB,
					error TEXT,
					retry_count INTEGER NOT NULL DEFAULT 0,
					max_retries INTEGER NOT NULL DEFAULT 3,
					progress_processed INTEGER,
					progress_total INTEGER,
					progress_message TEXT,
					progress_percentage REAL,
					workspace_id TEXT,
					created_at INTEGER NOT NULL,
					started_at INTEGER,
					completed_at INTEGER,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
				CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
				CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
				CREATE INDEX IF NOT EXISTS idx_tasks_workspace ON tasks(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);

				-- Scheduled tasks table
				CREATE TABLE IF NOT EXISTS scheduled_tasks (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					cron_expression TEXT NOT NULL,
					task_type TEXT NOT NULL,
					task_payload BLOB,
					enabled INTEGER NOT NULL DEFAULT 1,
					next_run INTEGER,
					last_run INTEGER,
					workspace_id TEXT,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_enabled ON scheduled_tasks(enabled);
				CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks(next_run);
			`,
			Down: `
				DROP TABLE IF EXISTS scheduled_tasks;
				DROP TABLE IF EXISTS tasks;
				DROP TABLE IF EXISTS file_context_suggestions;
				DROP TABLE IF EXISTS file_contexts;
				DROP TABLE IF EXISTS file_tags;
				DROP TABLE IF EXISTS file_metadata;
				DROP TABLE IF EXISTS files;
				DROP TABLE IF EXISTS workspaces;
			`,
		},
		{
			Version: 2,
			Name:    "documents_and_vectors",
			Up: `
				-- Documents table
				CREATE TABLE IF NOT EXISTS documents (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					title TEXT NOT NULL,
					frontmatter TEXT,
					checksum TEXT NOT NULL,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_path ON documents(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_documents_file_id ON documents(workspace_id, file_id);

				-- Chunks table
				CREATE TABLE IF NOT EXISTS chunks (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					document_id TEXT NOT NULL,
					ordinal INTEGER NOT NULL,
					heading TEXT NOT NULL,
					heading_path TEXT NOT NULL,
					text TEXT NOT NULL,
					token_count INTEGER NOT NULL,
					start_line INTEGER NOT NULL,
					end_line INTEGER NOT NULL,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(workspace_id, document_id);
				CREATE INDEX IF NOT EXISTS idx_chunks_heading ON chunks(workspace_id, heading_path);

				-- Chunk embeddings table
				CREATE TABLE IF NOT EXISTS chunk_embeddings (
					workspace_id TEXT NOT NULL,
					chunk_id TEXT NOT NULL,
					dimensions INTEGER NOT NULL,
					vector BLOB NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, chunk_id),
					FOREIGN KEY (workspace_id, chunk_id) REFERENCES chunks(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_workspace ON chunk_embeddings(workspace_id);
			`,
			Down: `
				DROP TABLE IF EXISTS chunk_embeddings;
				DROP TABLE IF EXISTS chunks;
				DROP TABLE IF EXISTS documents;
			`,
		},
		{
			Version: 3,
			Name:    "ai_metadata_fields",
			Up: `
				ALTER TABLE file_metadata ADD COLUMN ai_category TEXT;
				ALTER TABLE file_metadata ADD COLUMN ai_category_confidence REAL;
				ALTER TABLE file_metadata ADD COLUMN ai_category_updated_at INTEGER;
				ALTER TABLE file_metadata ADD COLUMN ai_related TEXT;
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
			`,
		},
		{
			Version: 4,
			Name:    "file_processing_traces",
			Up: `
				CREATE TABLE IF NOT EXISTS file_traces (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					stage TEXT NOT NULL,
					operation TEXT NOT NULL,
					prompt_path TEXT,
					output_path TEXT,
					prompt_preview TEXT,
					output_preview TEXT,
					model TEXT,
					tokens_used INTEGER,
					duration_ms INTEGER,
					error TEXT,
					created_at INTEGER NOT NULL
				);

				CREATE INDEX IF NOT EXISTS idx_file_traces_workspace ON file_traces(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_file_traces_file ON file_traces(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_traces_path ON file_traces(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_file_traces_created ON file_traces(workspace_id, created_at);
			`,
			Down: `
				DROP TABLE IF EXISTS file_traces;
			`,
		},
		{
			Version: 5,
			Name:    "knowledge_engine_features",
			Up: `
				-- Projects table (hierarchical)
				CREATE TABLE IF NOT EXISTS projects (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					name TEXT NOT NULL,
					description TEXT,
					parent_id TEXT,
					path TEXT NOT NULL,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, parent_id) REFERENCES projects(workspace_id, id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_projects_parent ON projects(workspace_id, parent_id);
				CREATE INDEX IF NOT EXISTS idx_projects_path ON projects(workspace_id, path);
				CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(workspace_id, name);

				-- Document states (add columns to existing documents table)
				ALTER TABLE documents ADD COLUMN state TEXT NOT NULL DEFAULT 'draft';
				ALTER TABLE documents ADD COLUMN state_changed_at INTEGER;

				CREATE INDEX IF NOT EXISTS idx_documents_state ON documents(workspace_id, state);

				-- Document state history
				CREATE TABLE IF NOT EXISTS document_state_history (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					document_id TEXT NOT NULL,
					from_state TEXT,
					to_state TEXT NOT NULL,
					reason TEXT,
					changed_by TEXT,
					changed_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_state_history_document ON document_state_history(workspace_id, document_id);
				CREATE INDEX IF NOT EXISTS idx_state_history_changed_at ON document_state_history(workspace_id, changed_at);

				-- Document relationships
				CREATE TABLE IF NOT EXISTS document_relationships (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					from_document_id TEXT NOT NULL,
					to_document_id TEXT NOT NULL,
					type TEXT NOT NULL,
					strength REAL,
					metadata TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, from_document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, to_document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, from_document_id, to_document_id, type)
				);

				CREATE INDEX IF NOT EXISTS idx_relationships_from ON document_relationships(workspace_id, from_document_id);
				CREATE INDEX IF NOT EXISTS idx_relationships_to ON document_relationships(workspace_id, to_document_id);
				CREATE INDEX IF NOT EXISTS idx_relationships_type ON document_relationships(workspace_id, type);
				CREATE INDEX IF NOT EXISTS idx_relationships_from_type ON document_relationships(workspace_id, from_document_id, type);

				-- Project-document relationships
				CREATE TABLE IF NOT EXISTS project_documents (
					workspace_id TEXT NOT NULL,
					project_id TEXT NOT NULL,
					document_id TEXT NOT NULL,
					role TEXT NOT NULL DEFAULT 'primary',
					added_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, project_id, document_id),
					FOREIGN KEY (workspace_id, project_id) REFERENCES projects(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_project_documents_project ON project_documents(workspace_id, project_id);
				CREATE INDEX IF NOT EXISTS idx_project_documents_document ON project_documents(workspace_id, document_id);
				CREATE INDEX IF NOT EXISTS idx_project_documents_role ON project_documents(workspace_id, role);

				-- Document usage events (temporal memory)
				CREATE TABLE IF NOT EXISTS document_usage_events (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					document_id TEXT NOT NULL,
					event_type TEXT NOT NULL,
					context TEXT,
					metadata TEXT,
					timestamp INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_usage_events_document ON document_usage_events(workspace_id, document_id);
				CREATE INDEX IF NOT EXISTS idx_usage_events_timestamp ON document_usage_events(workspace_id, timestamp);
				CREATE INDEX IF NOT EXISTS idx_usage_events_type ON document_usage_events(workspace_id, event_type);
				CREATE INDEX IF NOT EXISTS idx_usage_events_doc_timestamp ON document_usage_events(workspace_id, document_id, timestamp);
			`,
			Down: `
				DROP TABLE IF EXISTS document_usage_events;
				DROP TABLE IF EXISTS project_documents;
				DROP TABLE IF EXISTS document_relationships;
				DROP TABLE IF EXISTS document_state_history;
				DROP TABLE IF EXISTS projects;
				-- Note: Cannot drop columns from documents table in SQLite, but state will be ignored
			`,
		},
		{
			Version: 6,
			Name:    "detected_language_field",
			Up: `
				ALTER TABLE file_metadata ADD COLUMN detected_language TEXT;
				CREATE INDEX IF NOT EXISTS idx_file_metadata_language ON file_metadata(workspace_id, detected_language);
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
			`,
		},
		{
			Version: 7,
			Name:    "project_nature_and_attributes",
			Up: `
				-- Add nature column to projects table (default to 'generic' for existing projects)
				ALTER TABLE projects ADD COLUMN nature TEXT NOT NULL DEFAULT 'generic';
				
				-- Add attributes column (JSON) for additional project attributes
				ALTER TABLE projects ADD COLUMN attributes TEXT;
				
				-- Create index for searching by nature
				CREATE INDEX IF NOT EXISTS idx_projects_nature ON projects(workspace_id, nature);
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
				-- The nature and attributes columns will be ignored if migration is rolled back
			`,
		},
		{
			Version: 8,
			Name:    "config_versions",
			Up: `
				-- Configuration versions table for versioned config snapshots
				CREATE TABLE IF NOT EXISTS config_versions (
					version_id TEXT PRIMARY KEY,
					created_at INTEGER NOT NULL,
					created_by TEXT,
					description TEXT,
					config_json TEXT NOT NULL,
					metadata_json TEXT,
					UNIQUE(version_id)
				);

				CREATE INDEX IF NOT EXISTS idx_config_versions_created_at ON config_versions(created_at DESC);
			`,
			Down: `
				DROP TABLE IF EXISTS config_versions;
			`,
		},
		{
			Version: 9,
			Name:    "suggested_metadata",
			Up: `
				-- Suggested metadata table (stores AI-generated suggestions)
				CREATE TABLE IF NOT EXISTS suggested_metadata (
					file_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					confidence REAL NOT NULL DEFAULT 0.0,
					source TEXT NOT NULL DEFAULT 'rag_llm',
					generated_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE UNIQUE INDEX IF NOT EXISTS idx_suggested_metadata_path ON suggested_metadata(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_suggested_metadata_confidence ON suggested_metadata(workspace_id, confidence);

				-- Suggested tags table
				CREATE TABLE IF NOT EXISTS suggested_tags (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					tag TEXT NOT NULL,
					confidence REAL NOT NULL DEFAULT 0.0,
					reason TEXT,
					source TEXT NOT NULL DEFAULT 'llm',
					category TEXT,
					PRIMARY KEY (workspace_id, file_id, tag),
					FOREIGN KEY (workspace_id, file_id) REFERENCES suggested_metadata(workspace_id, file_id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_suggested_tags_tag ON suggested_tags(workspace_id, tag);
				CREATE INDEX IF NOT EXISTS idx_suggested_tags_confidence ON suggested_tags(workspace_id, confidence);

				-- Suggested projects table
				CREATE TABLE IF NOT EXISTS suggested_projects (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					project_id TEXT,
					project_name TEXT NOT NULL,
					confidence REAL NOT NULL DEFAULT 0.0,
					reason TEXT,
					source TEXT NOT NULL DEFAULT 'llm',
					is_new INTEGER NOT NULL DEFAULT 0,
					PRIMARY KEY (workspace_id, file_id, project_name),
					FOREIGN KEY (workspace_id, file_id) REFERENCES suggested_metadata(workspace_id, file_id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, project_id) REFERENCES projects(workspace_id, id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_suggested_projects_project ON suggested_projects(workspace_id, project_id);
				CREATE INDEX IF NOT EXISTS idx_suggested_projects_name ON suggested_projects(workspace_id, project_name);
				CREATE INDEX IF NOT EXISTS idx_suggested_projects_confidence ON suggested_projects(workspace_id, confidence);

				-- Suggested taxonomy table
				CREATE TABLE IF NOT EXISTS suggested_taxonomy (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					category TEXT,
					subcategory TEXT,
					domain TEXT,
					subdomain TEXT,
					content_type TEXT,
					purpose TEXT,
					audience TEXT,
					language TEXT,
					category_confidence REAL DEFAULT 0.0,
					domain_confidence REAL DEFAULT 0.0,
					content_type_confidence REAL DEFAULT 0.0,
					reasoning TEXT,
					source TEXT NOT NULL DEFAULT 'llm',
					PRIMARY KEY (workspace_id, file_id),
					FOREIGN KEY (workspace_id, file_id) REFERENCES suggested_metadata(workspace_id, file_id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_suggested_taxonomy_category ON suggested_taxonomy(workspace_id, category);
				CREATE INDEX IF NOT EXISTS idx_suggested_taxonomy_domain ON suggested_taxonomy(workspace_id, domain);
				CREATE INDEX IF NOT EXISTS idx_suggested_taxonomy_content_type ON suggested_taxonomy(workspace_id, content_type);

				-- Suggested taxonomy topics (many-to-many)
				CREATE TABLE IF NOT EXISTS suggested_taxonomy_topics (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					topic TEXT NOT NULL,
					PRIMARY KEY (workspace_id, file_id, topic),
					FOREIGN KEY (workspace_id, file_id) REFERENCES suggested_taxonomy(workspace_id, file_id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_suggested_taxonomy_topics_topic ON suggested_taxonomy_topics(workspace_id, topic);

				-- Suggested fields table (key-value pairs for additional metadata)
				CREATE TABLE IF NOT EXISTS suggested_fields (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					field_name TEXT NOT NULL,
					field_value TEXT NOT NULL,
					field_type TEXT NOT NULL DEFAULT 'string',
					confidence REAL NOT NULL DEFAULT 0.0,
					reason TEXT,
					source TEXT NOT NULL DEFAULT 'llm',
					PRIMARY KEY (workspace_id, file_id, field_name),
					FOREIGN KEY (workspace_id, file_id) REFERENCES suggested_metadata(workspace_id, file_id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_suggested_fields_name ON suggested_fields(workspace_id, field_name);
			`,
			Down: `
				DROP TABLE IF EXISTS suggested_fields;
				DROP TABLE IF EXISTS suggested_taxonomy_topics;
				DROP TABLE IF EXISTS suggested_taxonomy;
				DROP TABLE IF EXISTS suggested_projects;
				DROP TABLE IF EXISTS suggested_tags;
				DROP TABLE IF EXISTS suggested_metadata;
			`,
		},
		{
			Version: 10,
			Name:    "ai_context_field",
			Up: `
				ALTER TABLE file_metadata ADD COLUMN ai_context TEXT;
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
			`,
		},
		{
			Version: 11,
			Name:    "enrichment_data_field",
			Up: `
				ALTER TABLE file_metadata ADD COLUMN enrichment_data TEXT;
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
			`,
		},
		{
			Version: 12,
			Name:    "os_metadata_and_users",
			Up: `
				-- Add OS metadata columns to files table
				ALTER TABLE files ADD COLUMN os_metadata TEXT; -- JSON of OSMetadata
				ALTER TABLE files ADD COLUMN os_taxonomy TEXT; -- JSON of OSContextTaxonomy

				-- Persons table (human identities)
				CREATE TABLE IF NOT EXISTS persons (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					name TEXT NOT NULL,
					email TEXT,
					display_name TEXT,
					notes TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_persons_workspace ON persons(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_persons_email ON persons(workspace_id, email);

				-- System users table (OS user accounts)
				CREATE TABLE IF NOT EXISTS system_users (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					person_id TEXT, -- FK to persons (optional)
					username TEXT NOT NULL,
					uid INTEGER NOT NULL,
					full_name TEXT,
					home_dir TEXT,
					shell TEXT,
					is_system INTEGER NOT NULL DEFAULT 0,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE SET NULL,
					UNIQUE(workspace_id, username, uid)
				);

				CREATE INDEX IF NOT EXISTS idx_system_users_workspace ON system_users(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_system_users_person ON system_users(person_id);
				CREATE INDEX IF NOT EXISTS idx_system_users_username ON system_users(workspace_id, username);

				-- File ownership table
				CREATE TABLE IF NOT EXISTS file_ownership (
					file_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					user_id TEXT NOT NULL,
					ownership_type TEXT NOT NULL, -- "owner", "group_member", "other"
					permissions TEXT,
					detected_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id, user_id),
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES system_users(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_ownership_user ON file_ownership(workspace_id, user_id);
				CREATE INDEX IF NOT EXISTS idx_file_ownership_type ON file_ownership(workspace_id, ownership_type);

				-- File access table (ACLs, etc.)
				CREATE TABLE IF NOT EXISTS file_access (
					id TEXT PRIMARY KEY,
					file_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					user_id TEXT NOT NULL,
					access_type TEXT NOT NULL, -- "read", "write", "execute", "full"
					source TEXT NOT NULL, -- "permissions", "acl", "group_membership"
					detected_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES system_users(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_access_file ON file_access(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_access_user ON file_access(workspace_id, user_id);

				-- Project memberships table
				CREATE TABLE IF NOT EXISTS project_memberships (
					project_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					person_id TEXT NOT NULL,
					role TEXT NOT NULL, -- "owner", "contributor", "viewer"
					joined_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, project_id, person_id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, project_id) REFERENCES projects(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (person_id) REFERENCES persons(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_project_memberships_person ON project_memberships(workspace_id, person_id);
				CREATE INDEX IF NOT EXISTS idx_project_memberships_project ON project_memberships(workspace_id, project_id);
			`,
			Down: `
				DROP TABLE IF EXISTS project_memberships;
				DROP TABLE IF EXISTS file_access;
				DROP TABLE IF EXISTS file_ownership;
				DROP TABLE IF EXISTS system_users;
				DROP TABLE IF EXISTS persons;
				-- Note: Cannot drop columns from files table in SQLite, but os_metadata and os_taxonomy will be ignored
			`,
		},
		{
			Version: 13,
			Name:    "additional_timestamps",
			Up: `
				-- Add additional timestamp columns to files table
				ALTER TABLE files ADD COLUMN accessed_at INTEGER;
				ALTER TABLE files ADD COLUMN changed_at INTEGER;
				ALTER TABLE files ADD COLUMN backup_at INTEGER;

				CREATE INDEX IF NOT EXISTS idx_files_accessed ON files(workspace_id, accessed_at);
				CREATE INDEX IF NOT EXISTS idx_files_changed ON files(workspace_id, changed_at);
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
				-- The columns will be ignored if migration is rolled back
			`,
		},
		{
			Version: 14,
			Name:    "path_components",
			Up: `
				-- Add path component extraction columns to files table
				ALTER TABLE files ADD COLUMN path_components TEXT; -- JSON array of path components
				ALTER TABLE files ADD COLUMN path_pattern TEXT;    -- Normalized path pattern

				CREATE INDEX IF NOT EXISTS idx_files_path_pattern ON files(workspace_id, path_pattern);
			`,
			Down: `
				-- SQLite does not support DROP COLUMN; no-op.
				-- The columns will be ignored if migration is rolled back
			`,
		},
		{
			Version: 15,
			Name:    "denormalize_ai_context",
			Up: `
				-- Create tables for denormalized AI context data
				CREATE TABLE IF NOT EXISTS file_authors (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					role TEXT,
					affiliation TEXT,
					confidence REAL,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, name)
				);

				CREATE TABLE IF NOT EXISTS file_locations (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					type TEXT,
					coordinates TEXT,
					context TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, name)
				);

				CREATE TABLE IF NOT EXISTS file_people (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					role TEXT,
					context TEXT,
					confidence REAL,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, name)
				);

				CREATE TABLE IF NOT EXISTS file_organizations (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					type TEXT,
					context TEXT,
					confidence REAL,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, name)
				);

				CREATE TABLE IF NOT EXISTS file_events (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					date TEXT,
					location TEXT,
					context TEXT,
					confidence REAL,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS file_references (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					title TEXT,
					author TEXT,
					year TEXT,
					type TEXT,
					doi TEXT,
					url TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS file_publication_info (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					publisher TEXT,
					publication_year TEXT,
					publication_place TEXT,
					isbn TEXT,
					issn TEXT,
					doi TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id)
				);

				-- Create indexes for efficient queries
				CREATE INDEX IF NOT EXISTS idx_file_authors_file ON file_authors(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_authors_name ON file_authors(workspace_id, name);
				CREATE INDEX IF NOT EXISTS idx_file_locations_file ON file_locations(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_people_file ON file_people(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_people_name ON file_people(workspace_id, name);
				CREATE INDEX IF NOT EXISTS idx_file_organizations_file ON file_organizations(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_events_file ON file_events(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_references_file ON file_references(workspace_id, file_id);
			`,
			Down: `
				DROP TABLE IF EXISTS file_publication_info;
				DROP TABLE IF EXISTS file_references;
				DROP TABLE IF EXISTS file_events;
				DROP TABLE IF EXISTS file_organizations;
				DROP TABLE IF EXISTS file_people;
				DROP TABLE IF EXISTS file_locations;
				DROP TABLE IF EXISTS file_authors;
			`,
		},
		{
			Version: 16,
			Name:    "denormalize_enrichment",
			Up: `
				-- Create tables for denormalized enrichment data
				CREATE TABLE IF NOT EXISTS file_named_entities (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					text TEXT NOT NULL,
					type TEXT NOT NULL,
					start_pos INTEGER,
					end_pos INTEGER,
					confidence REAL,
					context TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS file_citations (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					text TEXT NOT NULL,
					authors TEXT,
					title TEXT,
					year TEXT,
					doi TEXT,
					url TEXT,
					type TEXT,
					confidence REAL,
					page INTEGER,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS file_dependencies (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					name TEXT NOT NULL,
					version TEXT,
					type TEXT,
					language TEXT,
					path TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, name, type)
				);

				CREATE TABLE IF NOT EXISTS file_duplicates (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					duplicate_file_id TEXT NOT NULL,
					similarity REAL NOT NULL,
					type TEXT,
					reason TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, duplicate_file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id, duplicate_file_id)
				);

				CREATE TABLE IF NOT EXISTS file_sentiment (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					overall_sentiment TEXT NOT NULL,
					score REAL NOT NULL,
					confidence REAL,
					emotions_json TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, file_id)
				);

				-- Create indexes for efficient queries
				CREATE INDEX IF NOT EXISTS idx_file_named_entities_file ON file_named_entities(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_named_entities_type ON file_named_entities(workspace_id, type);
				CREATE INDEX IF NOT EXISTS idx_file_citations_file ON file_citations(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_dependencies_file ON file_dependencies(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_dependencies_name ON file_dependencies(workspace_id, name);
				CREATE INDEX IF NOT EXISTS idx_file_duplicates_file ON file_duplicates(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_sentiment_file ON file_sentiment(workspace_id, file_id);
			`,
			Down: `
				DROP TABLE IF EXISTS file_sentiment;
				DROP TABLE IF EXISTS file_duplicates;
				DROP TABLE IF EXISTS file_dependencies;
				DROP TABLE IF EXISTS file_citations;
				DROP TABLE IF EXISTS file_named_entities;
			`,
		},
		{
			Version: 17,
			Name:    "index_frequently_queried_metadata",
			Up: `
				-- Create indexes on frequently queried metadata fields
				-- Note: SQLite doesn't support adding STORED generated columns via ALTER TABLE
				-- Instead, we create indexes that use JSON extraction functions directly
				-- These indexes will be used by the query planner when appropriate
				
				-- Index on extension (commonly queried)
				CREATE INDEX IF NOT EXISTS idx_files_extension ON files(workspace_id, extension);
				
				-- Index on path_pattern (from migration 14)
				CREATE INDEX IF NOT EXISTS idx_files_path_pattern ON files(workspace_id, path_pattern);
				
				-- Note: For JSON fields in the enhanced column, we can't create efficient indexes
				-- directly via ALTER TABLE. The application layer should handle filtering on
				-- these fields, or we can create virtual tables/views in the future if needed.
				-- For now, we index the columns that are stored directly in the files table.
			`,
			Down: `
				-- Drop indexes
				DROP INDEX IF EXISTS idx_files_path_pattern;
				DROP INDEX IF EXISTS idx_files_extension;
			`,
		},
		{
			Version: 18,
			Name:    "file_relationships",
			Up: `
				-- Create table for code import/export relationships
				CREATE TABLE IF NOT EXISTS file_relationships (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					from_file_id TEXT NOT NULL,
					to_file_id TEXT NOT NULL,
					type TEXT NOT NULL, -- "import", "export", "include", "require", "reference"
					language TEXT,
					confidence REAL,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, from_file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, to_file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE,
					UNIQUE(workspace_id, from_file_id, to_file_id, type)
				);

				-- Create indexes for efficient queries
				CREATE INDEX IF NOT EXISTS idx_file_relationships_from ON file_relationships(workspace_id, from_file_id);
				CREATE INDEX IF NOT EXISTS idx_file_relationships_to ON file_relationships(workspace_id, to_file_id);
				CREATE INDEX IF NOT EXISTS idx_file_relationships_type ON file_relationships(workspace_id, type);
				CREATE INDEX IF NOT EXISTS idx_file_relationships_language ON file_relationships(workspace_id, language);
			`,
			Down: `
				DROP INDEX IF EXISTS idx_file_relationships_language;
				DROP INDEX IF EXISTS idx_file_relationships_type;
				DROP INDEX IF EXISTS idx_file_relationships_to;
				DROP INDEX IF EXISTS idx_file_relationships_from;
				DROP TABLE IF EXISTS file_relationships;
			`,
		},
		{
			Version: 19,
			Name:    "enhance_document_relationships",
			Up: `
				-- Add confidence and discovery_method columns to document_relationships
				ALTER TABLE document_relationships ADD COLUMN confidence REAL;
				ALTER TABLE document_relationships ADD COLUMN discovery_method TEXT; -- "explicit", "implicit", "rag", "filename", "version", "template"

				-- Create index on confidence for filtering
				CREATE INDEX IF NOT EXISTS idx_relationships_confidence ON document_relationships(workspace_id, confidence);
			`,
			Down: `
				DROP INDEX IF EXISTS idx_relationships_confidence;
				-- SQLite does not support DROP COLUMN; columns will be ignored if rolled back
			`,
		},
		{
			Version: 21,
			Name:    "file_hashes",
			Up: `
				-- Add file hash columns for duplicate detection
				ALTER TABLE files ADD COLUMN file_hash_md5 TEXT;
				ALTER TABLE files ADD COLUMN file_hash_sha256 TEXT;
				ALTER TABLE files ADD COLUMN file_hash_sha512 TEXT;

				-- Create index on SHA256 for duplicate detection
				CREATE INDEX IF NOT EXISTS idx_files_hash_sha256 ON files(workspace_id, file_hash_sha256);
			`,
			Down: `
				DROP INDEX IF EXISTS idx_files_hash_sha256;
				-- SQLite does not support DROP COLUMN; columns will be ignored if rolled back
			`,
		},
		{
			Version: 23,
			Name:    "mirror_extraction_metadata",
			Up: `
				-- Add mirror extraction metadata columns to file_metadata
				-- Note: These are stored in the mirror JSON blob, but we add columns for querying
				-- The actual data is in the mirror JSON field in file_metadata table
				-- This migration is for future denormalization if needed
			`,
			Down: `
				-- No-op: data is in JSON blob
			`,
		},
		{
			Version: 20,
			Name:    "temporal_analysis",
			Up: `
				-- Create tables for temporal analysis
				CREATE TABLE IF NOT EXISTS file_access_events (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					event_type TEXT NOT NULL, -- "read", "write", "open", "close"
					timestamp INTEGER NOT NULL,
					metadata TEXT, -- JSON metadata about the event
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS file_modification_history (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					timestamp INTEGER NOT NULL,
					size_before INTEGER,
					size_after INTEGER,
					metadata TEXT, -- JSON metadata about the modification
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id, file_id) REFERENCES files(workspace_id, id) ON DELETE CASCADE
				);

				CREATE TABLE IF NOT EXISTS temporal_clusters (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					cluster_id TEXT NOT NULL,
					file_ids TEXT NOT NULL, -- JSON array of file IDs
					time_window_start INTEGER NOT NULL,
					time_window_end INTEGER NOT NULL,
					pattern_type TEXT, -- "edit_session", "project_work", "backup", etc.
					created_at INTEGER NOT NULL,
					UNIQUE(workspace_id, cluster_id)
				);

				-- Create indexes
				CREATE INDEX IF NOT EXISTS idx_access_events_file ON file_access_events(workspace_id, file_id, timestamp);
				CREATE INDEX IF NOT EXISTS idx_access_events_type ON file_access_events(workspace_id, event_type, timestamp);
				CREATE INDEX IF NOT EXISTS idx_modification_history_file ON file_modification_history(workspace_id, file_id, timestamp);
				CREATE INDEX IF NOT EXISTS idx_temporal_clusters_workspace ON temporal_clusters(workspace_id, time_window_start, time_window_end);
			`,
			Down: `
				DROP TABLE IF EXISTS temporal_clusters;
				DROP TABLE IF EXISTS file_modification_history;
				DROP TABLE IF EXISTS file_access_events;
			`,
		},
		{
			Version: 24,
			Name:    "benchmark_results",
			Up: `
				-- Benchmark results table for tracking AI/RAG quality metrics
				CREATE TABLE IF NOT EXISTS benchmark_results (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					test_suite TEXT NOT NULL,
					metric_type TEXT NOT NULL,
					precision REAL,
					recall REAL,
					f1_score REAL,
					accuracy REAL,
					retrieval_hit_rate REAL,
					grounding_accuracy REAL,
					hallucination_rate REAL,
					tokens_used INTEGER,
					latency_ms INTEGER,
					model_version TEXT,
					details TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_benchmark_results_workspace ON benchmark_results(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_benchmark_results_type ON benchmark_results(workspace_id, metric_type);
				CREATE INDEX IF NOT EXISTS idx_benchmark_results_suite ON benchmark_results(workspace_id, test_suite);
				CREATE INDEX IF NOT EXISTS idx_benchmark_results_time ON benchmark_results(workspace_id, created_at);

				-- Benchmark baselines table to store designated baseline benchmarks
				CREATE TABLE IF NOT EXISTS benchmark_baselines (
					workspace_id TEXT NOT NULL,
					metric_type TEXT NOT NULL,
					benchmark_id TEXT NOT NULL,
					set_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, metric_type),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (benchmark_id) REFERENCES benchmark_results(id) ON DELETE CASCADE
				);
			`,
			Down: `
				DROP TABLE IF EXISTS benchmark_baselines;
				DROP TABLE IF EXISTS benchmark_results;
			`,
		},
		{
			Version: 25,
			Name:    "model_usage_tracking",
			Up: `
				-- Model usage tracking for observability and cost monitoring
				CREATE TABLE IF NOT EXISTS model_usage (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					model_id TEXT NOT NULL,
					model_version TEXT,
					provider TEXT NOT NULL,
					operation TEXT NOT NULL,
					prompt_tokens INTEGER NOT NULL DEFAULT 0,
					completion_tokens INTEGER NOT NULL DEFAULT 0,
					total_tokens INTEGER NOT NULL DEFAULT 0,
					estimated_cost REAL,
					latency_ms INTEGER NOT NULL,
					success INTEGER NOT NULL DEFAULT 1,
					error_message TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_model_usage_workspace ON model_usage(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_model_usage_model ON model_usage(workspace_id, model_id);
				CREATE INDEX IF NOT EXISTS idx_model_usage_operation ON model_usage(workspace_id, operation);
				CREATE INDEX IF NOT EXISTS idx_model_usage_time ON model_usage(workspace_id, created_at);
				CREATE INDEX IF NOT EXISTS idx_model_usage_provider ON model_usage(workspace_id, provider);

				-- Extraction events table for tracking failures and successes
				CREATE TABLE IF NOT EXISTS extraction_events (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					stage TEXT NOT NULL,
					event_type TEXT NOT NULL,
					error_type TEXT,
					error_message TEXT,
					retryable INTEGER DEFAULT 0,
					retry_count INTEGER DEFAULT 0,
					items_extracted INTEGER,
					confidence REAL,
					duration_ms INTEGER,
					model_version TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_extraction_events_workspace ON extraction_events(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_extraction_events_file ON extraction_events(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_extraction_events_stage ON extraction_events(workspace_id, stage);
				CREATE INDEX IF NOT EXISTS idx_extraction_events_type ON extraction_events(workspace_id, event_type);
				CREATE INDEX IF NOT EXISTS idx_extraction_events_time ON extraction_events(workspace_id, created_at);
			`,
			Down: `
				DROP TABLE IF EXISTS extraction_events;
				DROP TABLE IF EXISTS model_usage;
			`,
		},
		{
			Version: 26,
			Name:    "project_assignments",
			Up: `
				-- Project assignments with scoring and provenance
				CREATE TABLE IF NOT EXISTS project_assignments (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					project_id TEXT,
					project_name TEXT NOT NULL,
					score REAL NOT NULL DEFAULT 0,
					sources TEXT,
					status TEXT NOT NULL,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id, project_name),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_project_assignments_workspace ON project_assignments(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_project_assignments_project ON project_assignments(workspace_id, project_id);
				CREATE INDEX IF NOT EXISTS idx_project_assignments_file ON project_assignments(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_project_assignments_score ON project_assignments(workspace_id, score);
				CREATE INDEX IF NOT EXISTS idx_project_assignments_status ON project_assignments(workspace_id, status);
			`,
			Down: `
				DROP TABLE IF EXISTS project_assignments;
			`,
		},
		{
			Version: 27,
			Name:    "folders_and_inferred_projects",
			Up: `
				-- Folder entries with aggregated metrics
				CREATE TABLE IF NOT EXISTS folders (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					name TEXT NOT NULL,
					parent_path TEXT,
					depth INTEGER NOT NULL DEFAULT 0,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					metrics TEXT,
					metadata TEXT,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_folders_workspace ON folders(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_folders_path ON folders(workspace_id, relative_path);
				CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders(workspace_id, parent_path);
				CREATE INDEX IF NOT EXISTS idx_folders_depth ON folders(workspace_id, depth);
				CREATE INDEX IF NOT EXISTS idx_folders_name ON folders(workspace_id, name);

				-- Inferred projects from folder structure
				CREATE TABLE IF NOT EXISTS inferred_projects (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					name TEXT NOT NULL,
					folder_path TEXT NOT NULL,
					nature TEXT,
					confidence REAL NOT NULL DEFAULT 0,
					file_count INTEGER NOT NULL DEFAULT 0,
					indicator_files TEXT,
					dominant_language TEXT,
					description TEXT,
					status TEXT NOT NULL DEFAULT 'suggested',
					accepted_project_id TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (accepted_project_id) REFERENCES projects(id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_inferred_projects_workspace ON inferred_projects(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_inferred_projects_folder ON inferred_projects(workspace_id, folder_path);
				CREATE INDEX IF NOT EXISTS idx_inferred_projects_confidence ON inferred_projects(workspace_id, confidence);
				CREATE INDEX IF NOT EXISTS idx_inferred_projects_status ON inferred_projects(workspace_id, status);
				CREATE INDEX IF NOT EXISTS idx_inferred_projects_nature ON inferred_projects(workspace_id, nature);
			`,
			Down: `
				DROP TABLE IF EXISTS inferred_projects;
				DROP TABLE IF EXISTS folders;
			`,
		},
		{
			Version: 28,
			Name:    "enhanced_indexing_flags",
			Up: `
				-- Add new indexing state flags to files table
				ALTER TABLE files ADD COLUMN indexed_os_metadata INTEGER NOT NULL DEFAULT 0;
				ALTER TABLE files ADD COLUMN indexed_enrichment INTEGER NOT NULL DEFAULT 0;
				ALTER TABLE files ADD COLUMN indexed_folder INTEGER NOT NULL DEFAULT 0;
				ALTER TABLE files ADD COLUMN indexed_project_inference INTEGER NOT NULL DEFAULT 0;

				-- Add folder_id foreign key
				ALTER TABLE files ADD COLUMN folder_id TEXT;

				-- Create indexes for new columns
				CREATE INDEX IF NOT EXISTS idx_files_folder ON files(workspace_id, folder_id);
				CREATE INDEX IF NOT EXISTS idx_files_indexed_os ON files(workspace_id, indexed_os_metadata);
				CREATE INDEX IF NOT EXISTS idx_files_indexed_enrichment ON files(workspace_id, indexed_enrichment);

				-- Note: SQLite doesn't support adding GENERATED ALWAYS AS ... STORED columns with ALTER TABLE
				-- We'll use json_extract() directly in queries instead of computed columns
				-- Indexes on JSON expressions are not supported, but queries will still work
			`,
			Down: `
				-- SQLite doesn't support DROP COLUMN, so we'd need to recreate the table
				-- For simplicity, this migration cannot be fully rolled back
			`,
		},
		{
			Version: 29,
			Name:    "file_hashes_table",
			Up: `
				-- File hashes for duplicate detection (denormalized for faster queries)
				CREATE TABLE IF NOT EXISTS file_hashes (
					workspace_id TEXT NOT NULL,
					file_id TEXT NOT NULL,
					relative_path TEXT NOT NULL,
					md5 TEXT,
					sha256 TEXT,
					sha512 TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_hashes_workspace ON file_hashes(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_file_hashes_md5 ON file_hashes(workspace_id, md5);
				CREATE INDEX IF NOT EXISTS idx_file_hashes_sha256 ON file_hashes(workspace_id, sha256);

				-- Index for finding duplicates
				CREATE INDEX IF NOT EXISTS idx_file_hashes_duplicate ON file_hashes(workspace_id, sha256, file_id);
			`,
			Down: `
				DROP TABLE IF EXISTS file_hashes;
			`,
		},
		{
			Version: 30,
			Name:    "document_clusters_and_edges",
			Up: `
				-- Document clusters table
				CREATE TABLE IF NOT EXISTS document_clusters (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					name TEXT NOT NULL,
					summary TEXT,
					status TEXT NOT NULL DEFAULT 'pending',
					confidence REAL NOT NULL DEFAULT 0,
					member_count INTEGER NOT NULL DEFAULT 0,
					central_nodes TEXT,
					top_entities TEXT,
					top_keywords TEXT,
					merged_into TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_clusters_workspace ON document_clusters(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_clusters_status ON document_clusters(workspace_id, status);
				CREATE INDEX IF NOT EXISTS idx_clusters_confidence ON document_clusters(workspace_id, confidence);

				-- Cluster memberships table
				CREATE TABLE IF NOT EXISTS cluster_memberships (
					cluster_id TEXT NOT NULL,
					document_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					score REAL NOT NULL DEFAULT 0,
					is_central INTEGER NOT NULL DEFAULT 0,
					joined_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, cluster_id, document_id),
					FOREIGN KEY (workspace_id, cluster_id) REFERENCES document_clusters(workspace_id, id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_memberships_cluster ON cluster_memberships(workspace_id, cluster_id);
				CREATE INDEX IF NOT EXISTS idx_memberships_document ON cluster_memberships(workspace_id, document_id);
				CREATE INDEX IF NOT EXISTS idx_memberships_score ON cluster_memberships(workspace_id, score);
				CREATE INDEX IF NOT EXISTS idx_memberships_central ON cluster_memberships(workspace_id, is_central);

				-- Document edges table (semantic graph)
				CREATE TABLE IF NOT EXISTS document_edges (
					from_doc TEXT NOT NULL,
					to_doc TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					weight REAL NOT NULL DEFAULT 0,
					sources TEXT,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, from_doc, to_doc),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_edges_workspace ON document_edges(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_edges_from ON document_edges(workspace_id, from_doc);
				CREATE INDEX IF NOT EXISTS idx_edges_to ON document_edges(workspace_id, to_doc);
				CREATE INDEX IF NOT EXISTS idx_edges_weight ON document_edges(workspace_id, weight);
			`,
			Down: `
				DROP TABLE IF EXISTS document_edges;
				DROP TABLE IF EXISTS cluster_memberships;
				DROP TABLE IF EXISTS document_clusters;
			`,
		},
		{
			Version: 31,
			Name:    "dynamic_taxonomy",
			Up: `
				-- Taxonomy nodes table (hierarchical categories)
				CREATE TABLE IF NOT EXISTS taxonomy_nodes (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					name TEXT NOT NULL,
					description TEXT,
					parent_id TEXT,
					path TEXT NOT NULL,
					level INTEGER NOT NULL DEFAULT 0,
					source TEXT NOT NULL DEFAULT 'inferred',
					confidence REAL NOT NULL DEFAULT 1.0,
					keywords TEXT,
					example_docs TEXT,
					child_count INTEGER NOT NULL DEFAULT 0,
					doc_count INTEGER NOT NULL DEFAULT 0,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, parent_id) REFERENCES taxonomy_nodes(workspace_id, id) ON DELETE SET NULL
				);

				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_workspace ON taxonomy_nodes(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_parent ON taxonomy_nodes(workspace_id, parent_id);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_path ON taxonomy_nodes(workspace_id, path);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_level ON taxonomy_nodes(workspace_id, level);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_source ON taxonomy_nodes(workspace_id, source);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_name ON taxonomy_nodes(workspace_id, name);
				CREATE INDEX IF NOT EXISTS idx_taxonomy_nodes_confidence ON taxonomy_nodes(workspace_id, confidence);

				-- File taxonomy mappings (many-to-many: files can be in multiple taxonomy nodes)
				CREATE TABLE IF NOT EXISTS file_taxonomy_mappings (
					file_id TEXT NOT NULL,
					node_id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					score REAL NOT NULL DEFAULT 1.0,
					source TEXT NOT NULL DEFAULT 'auto',
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, file_id, node_id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
					FOREIGN KEY (workspace_id, node_id) REFERENCES taxonomy_nodes(workspace_id, id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_file_taxonomy_workspace ON file_taxonomy_mappings(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_file_taxonomy_file ON file_taxonomy_mappings(workspace_id, file_id);
				CREATE INDEX IF NOT EXISTS idx_file_taxonomy_node ON file_taxonomy_mappings(workspace_id, node_id);
				CREATE INDEX IF NOT EXISTS idx_file_taxonomy_score ON file_taxonomy_mappings(workspace_id, score);
				CREATE INDEX IF NOT EXISTS idx_file_taxonomy_source ON file_taxonomy_mappings(workspace_id, source);

				-- Taxonomy induction history (track induction runs)
				CREATE TABLE IF NOT EXISTS taxonomy_induction_history (
					id TEXT PRIMARY KEY,
					workspace_id TEXT NOT NULL,
					nodes_created INTEGER NOT NULL DEFAULT 0,
					nodes_merged INTEGER NOT NULL DEFAULT 0,
					nodes_updated INTEGER NOT NULL DEFAULT 0,
					mappings_added INTEGER NOT NULL DEFAULT 0,
					seed_categories TEXT,
					errors TEXT,
					created_at INTEGER NOT NULL,
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_induction_history_workspace ON taxonomy_induction_history(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_induction_history_created ON taxonomy_induction_history(workspace_id, created_at);
			`,
			Down: `
				DROP TABLE IF EXISTS taxonomy_induction_history;
				DROP TABLE IF EXISTS file_taxonomy_mappings;
				DROP TABLE IF EXISTS taxonomy_nodes;
			`,
		},
		{
			Version: 32,
			Name:    "preference_learning",
			Up: `
				-- User feedback table (tracks user responses to suggestions)
				CREATE TABLE IF NOT EXISTS user_feedback (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					action_type TEXT NOT NULL,
					suggestion_type TEXT NOT NULL,
					suggestion_value TEXT,
					suggestion_confidence REAL,
					suggestion_reasoning TEXT,
					suggestion_source TEXT,
					suggestion_metadata TEXT,
					correction_value TEXT,
					correction_reason TEXT,
					correction_metadata TEXT,
					context_file_id TEXT,
					context_file_path TEXT,
					context_file_type TEXT,
					context_folder_path TEXT,
					context_session_id TEXT,
					context_full TEXT,
					response_time INTEGER,
					created_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_user_feedback_workspace ON user_feedback(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_user_feedback_action ON user_feedback(workspace_id, action_type);
				CREATE INDEX IF NOT EXISTS idx_user_feedback_suggestion_type ON user_feedback(workspace_id, suggestion_type);
				CREATE INDEX IF NOT EXISTS idx_user_feedback_created ON user_feedback(workspace_id, created_at);
				CREATE INDEX IF NOT EXISTS idx_user_feedback_file ON user_feedback(workspace_id, context_file_id);

				-- Learned preferences table (patterns learned from feedback)
				CREATE TABLE IF NOT EXISTS learned_preferences (
					id TEXT NOT NULL,
					workspace_id TEXT NOT NULL,
					preference_type TEXT NOT NULL,
					pattern TEXT NOT NULL,
					behavior TEXT NOT NULL,
					confidence REAL NOT NULL DEFAULT 0.5,
					examples INTEGER NOT NULL DEFAULT 1,
					last_used INTEGER,
					created_at INTEGER NOT NULL,
					updated_at INTEGER NOT NULL,
					PRIMARY KEY (workspace_id, id),
					FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_learned_preferences_workspace ON learned_preferences(workspace_id);
				CREATE INDEX IF NOT EXISTS idx_learned_preferences_type ON learned_preferences(workspace_id, preference_type);
				CREATE INDEX IF NOT EXISTS idx_learned_preferences_confidence ON learned_preferences(workspace_id, confidence);
				CREATE INDEX IF NOT EXISTS idx_learned_preferences_updated ON learned_preferences(workspace_id, updated_at);
				CREATE INDEX IF NOT EXISTS idx_learned_preferences_last_used ON learned_preferences(workspace_id, last_used);
			`,
			Down: `
				DROP TABLE IF EXISTS learned_preferences;
				DROP TABLE IF EXISTS user_feedback;
			`,
		},
	}
}
