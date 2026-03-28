// Package sqlite provides SQLite-based implementations of repositories.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// Connection manages SQLite database connections.
type Connection struct {
	db       *sql.DB
	path     string
	mu       sync.RWMutex
	migrator *Migrator
}

// NewConnection creates a new SQLite connection.
func NewConnection(dbPath string) (*Connection, error) {
	// Ensure directory exists and is writable
	// SQLite needs write access to the directory for WAL files
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	
	// Ensure directory is writable (SQLite needs this for WAL files)
	if dirInfo, err := os.Stat(dir); err == nil {
		dirMode := dirInfo.Mode()
		if dirMode&0200 == 0 {
			if err := os.Chmod(dir, dirMode|0200); err != nil {
				return nil, fmt.Errorf("failed to make database directory writable: %w", err)
			}
		}
	}

	// Ensure database file is writable if it exists
	if info, err := os.Stat(dbPath); err == nil {
		// File exists - check and fix permissions if needed
		mode := info.Mode()
		if mode&0200 == 0 {
			// File is not writable, make it writable
			if err := os.Chmod(dbPath, mode|0200); err != nil {
				return nil, fmt.Errorf("failed to make database writable: %w", err)
			}
		}
	}

	// Open database with WAL mode and foreign keys
	// File permissions are checked above to ensure write access
	// Increased busy_timeout to 30 seconds to handle concurrent operations during indexing
	// This prevents "database is locked" errors when multiple operations try to access the DB
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=30000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite doesn't support multiple writers
	db.SetMaxIdleConns(1)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	conn := &Connection{
		db:   db,
		path: dbPath,
	}

	conn.migrator = NewMigrator(conn)

	return conn, nil
}

// DB returns the underlying database connection.
func (c *Connection) DB() *sql.DB {
	return c.db
}

// Path returns the database file path.
func (c *Connection) Path() string {
	return c.path
}

// Close closes the database connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Migrate runs database migrations.
func (c *Connection) Migrate(ctx context.Context) error {
	return c.migrator.Migrate(ctx)
}

// Transaction executes a function within a transaction.
func (c *Connection) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Exec executes a query without returning rows.
func (c *Connection) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (c *Connection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (c *Connection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db.QueryRowContext(ctx, query, args...)
}

// Vacuum reclaims unused space in the database.
func (c *Connection) Vacuum(ctx context.Context) error {
	_, err := c.Exec(ctx, "VACUUM")
	return err
}

// Checkpoint forces a WAL checkpoint.
func (c *Connection) Checkpoint(ctx context.Context) error {
	_, err := c.Exec(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Size returns the size of the database file in bytes.
func (c *Connection) Size() (int64, error) {
	info, err := os.Stat(c.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
