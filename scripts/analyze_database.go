package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
)

func main() {
	// Load config
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	dbPath := cfg.DatabasePath()
	fmt.Printf("📊 Database: %s\n", dbPath)
	
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("❌ Database not found at: %s\n", dbPath)
		os.Exit(1)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	// Check migrations
	fmt.Println("\n🔍 Applied Migrations:")
	rows, err := db.QueryContext(ctx, "SELECT version, name, datetime(applied_at/1000, 'unixepoch') as applied_at FROM _migrations ORDER BY version")
	if err != nil {
		fmt.Printf("  ⚠️  No migrations table: %v\n", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var version int
			var name, appliedAt string
			if err := rows.Scan(&version, &name, &appliedAt); err == nil {
				fmt.Printf("  ✅ v%d: %s (applied: %s)\n", version, name, appliedAt)
			}
		}
	}

	// Table counts
	fmt.Println("\n📊 Table Statistics:")
	tables := []string{
		"workspaces", "files", "file_metadata", "file_tags", "file_contexts",
		"file_context_suggestions", "documents", "chunks", "chunk_embeddings",
		"file_traces", "tasks", "scheduled_tasks",
	}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			fmt.Printf("  %s: ❌ (table not found or error)\n", table)
		} else {
			fmt.Printf("  %s: %d\n", table, count)
		}
	}

	// Workspaces
	fmt.Println("\n📁 Workspaces:")
	rows, err = db.QueryContext(ctx, `
		SELECT id, name, path, file_count, 
		       datetime(last_indexed/1000, 'unixepoch') as last_indexed
		FROM workspaces
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name, path string
			var fileCount int
			var lastIndexed sql.NullString
			if err := rows.Scan(&id, &name, &path, &fileCount, &lastIndexed); err == nil {
				idx := "never"
				if lastIndexed.Valid {
					idx = lastIndexed.String
				}
				fmt.Printf("  • %s (%s)\n    Path: %s\n    Files: %d, Last indexed: %s\n", name, id[:8], path, fileCount, idx)
			}
		}
	}

	// Indexing status
	fmt.Println("\n🔍 Indexing Status:")
	var total, basic, mime, mirror, code, document int
	err = db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			SUM(indexed_basic) as basic,
			SUM(indexed_mime) as mime,
			SUM(indexed_mirror) as mirror,
			SUM(indexed_code) as code,
			SUM(indexed_document) as document
		FROM files
	`).Scan(&total, &basic, &mime, &mirror, &code, &document)
	if err == nil {
		fmt.Printf("  Total files: %d\n", total)
		fmt.Printf("  ✅ Basic: %d (%.1f%%)\n", basic, float64(basic)/float64(total)*100)
		fmt.Printf("  ✅ MIME: %d (%.1f%%)\n", mime, float64(mime)/float64(total)*100)
		fmt.Printf("  ✅ Mirror: %d (%.1f%%)\n", mirror, float64(mirror)/float64(total)*100)
		fmt.Printf("  ✅ Code: %d (%.1f%%)\n", code, float64(code)/float64(total)*100)
		fmt.Printf("  ✅ Document: %d (%.1f%%)\n", document, float64(document)/float64(total)*100)
	}

	// AI Metadata
	fmt.Println("\n🤖 AI Metadata:")
	var totalMeta, withSummary, withCategory, withRelated int
	err = db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(ai_summary) as with_summary,
			COUNT(ai_category) as with_category,
			COUNT(ai_related) as with_related
		FROM file_metadata
	`).Scan(&totalMeta, &withSummary, &withCategory, &withRelated)
	if err == nil {
		fmt.Printf("  Total metadata entries: %d\n", totalMeta)
		fmt.Printf("  ✅ With AI Summary: %d\n", withSummary)
		fmt.Printf("  ✅ With AI Category: %d\n", withCategory)
		fmt.Printf("  ✅ With AI Related: %d\n", withRelated)
	}

	// Tags and Contexts
	fmt.Println("\n🏷️  Tags (top 10):")
	rows, err = db.QueryContext(ctx, `
		SELECT tag, COUNT(*) as count 
		FROM file_tags 
		GROUP BY tag 
		ORDER BY count DESC 
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tag string
			var count int
			if err := rows.Scan(&tag, &count); err == nil {
				fmt.Printf("  • %s: %d files\n", tag, count)
			}
		}
	}

	fmt.Println("\n📂 Contexts/Projects (top 10):")
	rows, err = db.QueryContext(ctx, `
		SELECT context, COUNT(*) as count 
		FROM file_contexts 
		GROUP BY context 
		ORDER BY count DESC 
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var context string
			var count int
			if err := rows.Scan(&context, &count); err == nil {
				fmt.Printf("  • %s: %d files\n", context, count)
			}
		}
	}

	// Documents and Embeddings
	fmt.Println("\n📄 Documents & Embeddings:")
	var docCount, chunkCount, embedCount int
	var workspacesWithEmbeddings int
	err = db.QueryRowContext(ctx, `
		SELECT 
			(SELECT COUNT(*) FROM documents) as documents,
			(SELECT COUNT(*) FROM chunks) as chunks,
			(SELECT COUNT(*) FROM chunk_embeddings) as embeddings,
			(SELECT COUNT(DISTINCT workspace_id) FROM chunk_embeddings) as workspaces_with_embeddings
	`).Scan(&docCount, &chunkCount, &embedCount, &workspacesWithEmbeddings)
	if err == nil {
		fmt.Printf("  Documents: %d\n", docCount)
		fmt.Printf("  Chunks: %d\n", chunkCount)
		fmt.Printf("  Embeddings: %d\n", embedCount)
		fmt.Printf("  Workspaces with embeddings: %d\n", workspacesWithEmbeddings)
		if chunkCount > 0 {
			fmt.Printf("  Embedding coverage: %.1f%% (%d/%d chunks)\n", 
				float64(embedCount)/float64(chunkCount)*100, embedCount, chunkCount)
		}
	}

	// Recent traces
	fmt.Println("\n📝 Recent Processing Traces (last 5):")
	rows, err = db.QueryContext(ctx, `
		SELECT relative_path, stage, operation, 
		       datetime(created_at/1000, 'unixepoch') as created_at,
		       CASE WHEN error IS NOT NULL THEN '❌' ELSE '✅' END as status
		FROM file_traces
		ORDER BY created_at DESC
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var path, stage, operation, createdAt, status string
			if err := rows.Scan(&path, &stage, &operation, &createdAt, &status); err == nil {
				fmt.Printf("  %s %s/%s: %s (%s)\n", status, stage, operation, path, createdAt)
			}
		}
	}

	fmt.Println("\n✅ Database analysis complete")
}







