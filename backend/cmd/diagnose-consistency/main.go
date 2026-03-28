package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
	"github.com/rs/zerolog"
)

func main() {
	var dbPath string
	var workspaceID string
	flag.StringVar(&dbPath, "db", "", "Path to SQLite database file")
	flag.StringVar(&workspaceID, "workspace", "", "Workspace ID to check (optional, checks all if not specified)")
	flag.Parse()

	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -db flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	// Open database
	conn, err := sqlite.NewConnection(dbPath)
	if err != nil {
		logger.Fatal().Err(err).Str("db", dbPath).Msg("Failed to open database")
	}
	defer conn.Close()

	ctx := context.Background()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔍 DIAGNÓSTICO DE CONSISTENCIA: Documentos vs Archivos")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Check 1: Documents without corresponding files
	fmt.Println("📋 Verificando documentos sin archivos correspondientes...")
	documentsWithoutFiles, err := findDocumentsWithoutFiles(ctx, conn, workspaceID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check documents without files")
	} else {
		if len(documentsWithoutFiles) > 0 {
			fmt.Printf("⚠️  Encontrados %d documentos sin archivos correspondientes:\n", len(documentsWithoutFiles))
			for _, doc := range documentsWithoutFiles {
				fmt.Printf("   - %s (workspace: %s)\n", doc.path, doc.workspaceID)
			}
		} else {
			fmt.Println("✅ Todos los documentos tienen archivos correspondientes")
		}
	}
	fmt.Println()

	// Check 2: Files without corresponding documents
	fmt.Println("📁 Verificando archivos sin documentos correspondientes...")
	filesWithoutDocuments, err := findFilesWithoutDocuments(ctx, conn, workspaceID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check files without documents")
	} else {
		if len(filesWithoutDocuments) > 0 {
			fmt.Printf("⚠️  Encontrados %d archivos sin documentos correspondientes:\n", len(filesWithoutDocuments))
			for _, file := range filesWithoutDocuments {
				fmt.Printf("   - %s (workspace: %s)\n", file.path, file.workspaceID)
			}
		} else {
			fmt.Println("✅ Todos los archivos tienen documentos correspondientes")
		}
	}
	fmt.Println()

	// Check 3: Path normalization issues
	fmt.Println("🔄 Verificando problemas de normalización de rutas...")
	pathIssues, err := findPathNormalizationIssues(ctx, conn, workspaceID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to check path normalization")
	} else {
		if len(pathIssues) > 0 {
			fmt.Printf("⚠️  Encontrados %d problemas de normalización de rutas:\n", len(pathIssues))
			for _, issue := range pathIssues {
				fmt.Printf("   - Documento: %s\n", issue.docPath)
				fmt.Printf("     Archivo:   %s\n", issue.filePath)
				fmt.Printf("     Workspace: %s\n", issue.workspaceID)
				fmt.Println()
			}
		} else {
			fmt.Println("✅ No se encontraron problemas de normalización de rutas")
		}
	}
	fmt.Println()

	// Summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 RESUMEN")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Documentos sin archivos: %d\n", len(documentsWithoutFiles))
	fmt.Printf("Archivos sin documentos: %d\n", len(filesWithoutDocuments))
	fmt.Printf("Problemas de normalización: %d\n", len(pathIssues))
	fmt.Println()
}

type pathInfo struct {
	workspaceID string
	path        string
}

type pathMismatch struct {
	workspaceID string
	docPath     string
	filePath    string
}

func findDocumentsWithoutFiles(ctx context.Context, conn *sqlite.Connection, workspaceID string) ([]pathInfo, error) {
	var query string
	var args []interface{}

	if workspaceID != "" {
		query = `
			SELECT d.workspace_id, d.relative_path
			FROM documents d
			LEFT JOIN files f ON d.workspace_id = f.workspace_id 
				AND (d.relative_path = f.relative_path 
					OR REPLACE(d.relative_path, '\', '/') = REPLACE(f.relative_path, '\', '/'))
			WHERE d.workspace_id = ? AND f.id IS NULL
			ORDER BY d.workspace_id, d.relative_path
		`
		args = []interface{}{workspaceID}
	} else {
		query = `
			SELECT d.workspace_id, d.relative_path
			FROM documents d
			LEFT JOIN files f ON d.workspace_id = f.workspace_id 
				AND (d.relative_path = f.relative_path 
					OR REPLACE(d.relative_path, '\', '/') = REPLACE(f.relative_path, '\', '/'))
			WHERE f.id IS NULL
			ORDER BY d.workspace_id, d.relative_path
		`
		args = []interface{}{}
	}

	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = conn.Query(ctx, query, args...)
	} else {
		rows, err = conn.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []pathInfo
	for rows.Next() {
		var info pathInfo
		if err := rows.Scan(&info.workspaceID, &info.path); err != nil {
			return nil, err
		}
		results = append(results, info)
	}

	return results, rows.Err()
}

func findFilesWithoutDocuments(ctx context.Context, conn *sqlite.Connection, workspaceID string) ([]pathInfo, error) {
	var query string
	var args []interface{}

	if workspaceID != "" {
		query = `
			SELECT f.workspace_id, f.relative_path
			FROM files f
			LEFT JOIN documents d ON f.workspace_id = d.workspace_id 
				AND (f.relative_path = d.relative_path 
					OR REPLACE(f.relative_path, '\', '/') = REPLACE(d.relative_path, '\', '/'))
			WHERE f.workspace_id = ? AND d.id IS NULL
			ORDER BY f.workspace_id, f.relative_path
		`
		args = []interface{}{workspaceID}
	} else {
		query = `
			SELECT f.workspace_id, f.relative_path
			FROM files f
			LEFT JOIN documents d ON f.workspace_id = d.workspace_id 
				AND (f.relative_path = d.relative_path 
					OR REPLACE(f.relative_path, '\', '/') = REPLACE(d.relative_path, '\', '/'))
			WHERE d.id IS NULL
			ORDER BY f.workspace_id, f.relative_path
		`
		args = []interface{}{}
	}

	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = conn.Query(ctx, query, args...)
	} else {
		rows, err = conn.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []pathInfo
	for rows.Next() {
		var info pathInfo
		if err := rows.Scan(&info.workspaceID, &info.path); err != nil {
			return nil, err
		}
		results = append(results, info)
	}

	return results, rows.Err()
}

func findPathNormalizationIssues(ctx context.Context, conn *sqlite.Connection, workspaceID string) ([]pathMismatch, error) {
	var query string
	var args []interface{}

	if workspaceID != "" {
		query = `
			SELECT d.workspace_id, d.relative_path, f.relative_path
			FROM documents d
			INNER JOIN files f ON d.workspace_id = f.workspace_id
			WHERE d.workspace_id = ?
				AND d.relative_path != f.relative_path
				AND REPLACE(d.relative_path, '\', '/') = REPLACE(f.relative_path, '\', '/')
			ORDER BY d.workspace_id, d.relative_path
		`
		args = []interface{}{workspaceID}
	} else {
		query = `
			SELECT d.workspace_id, d.relative_path, f.relative_path
			FROM documents d
			INNER JOIN files f ON d.workspace_id = f.workspace_id
			WHERE d.relative_path != f.relative_path
				AND REPLACE(d.relative_path, '\', '/') = REPLACE(f.relative_path, '\', '/')
			ORDER BY d.workspace_id, d.relative_path
		`
		args = []interface{}{}
	}

	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = conn.Query(ctx, query, args...)
	} else {
		rows, err = conn.Query(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []pathMismatch
	for rows.Next() {
		var issue pathMismatch
		if err := rows.Scan(&issue.workspaceID, &issue.docPath, &issue.filePath); err != nil {
			return nil, err
		}
		results = append(results, issue)
	}

	return results, rows.Err()
}

