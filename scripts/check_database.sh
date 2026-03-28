#!/bin/bash
# Script to check database status and indexes

DB_PATH="${1:-tmp/cortex-test-data/cortex.sqlite}"

if [ ! -f "$DB_PATH" ]; then
    echo "❌ Database not found at: $DB_PATH"
    echo "Looking for database files..."
    find . -name "*.sqlite" -o -name "*.db" 2>/dev/null | head -5
    exit 1
fi

echo "📊 Database: $DB_PATH"
echo "📏 Size: $(du -h "$DB_PATH" | cut -f1)"
echo ""

# Check if sqlite3 is available
if ! command -v sqlite3 &> /dev/null; then
    echo "⚠️  sqlite3 not found. Install it to run database checks."
    exit 1
fi

echo "🔍 Database Schema Check"
echo "========================"
echo ""

echo "📋 Applied Migrations:"
sqlite3 "$DB_PATH" "SELECT version, name, datetime(applied_at/1000, 'unixepoch') as applied_at FROM _migrations ORDER BY version;" 2>/dev/null || echo "  No migrations table found"
echo ""

echo "📊 Table Counts:"
echo "  workspaces: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM workspaces;" 2>/dev/null || echo "0")"
echo "  files: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM files;" 2>/dev/null || echo "0")"
echo "  file_metadata: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM file_metadata;" 2>/dev/null || echo "0")"
echo "  file_tags: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM file_tags;" 2>/dev/null || echo "0")"
echo "  file_contexts: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM file_contexts;" 2>/dev/null || echo "0")"
echo "  documents: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM documents;" 2>/dev/null || echo "0")"
echo "  chunks: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM chunks;" 2>/dev/null || echo "0")"
echo "  chunk_embeddings: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM chunk_embeddings;" 2>/dev/null || echo "0")"
echo "  file_traces: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM file_traces;" 2>/dev/null || echo "0")"
echo ""

echo "🔍 Index Status:"
echo "  Indexed files breakdown:"
sqlite3 "$DB_PATH" "SELECT 
  COUNT(*) as total,
  SUM(indexed_basic) as basic,
  SUM(indexed_mime) as mime,
  SUM(indexed_mirror) as mirror,
  SUM(indexed_code) as code,
  SUM(indexed_document) as document
FROM files;" 2>/dev/null || echo "  No files table"
echo ""

echo "📁 Workspaces:"
sqlite3 "$DB_PATH" "SELECT id, name, path, file_count, datetime(last_indexed/1000, 'unixepoch') as last_indexed FROM workspaces;" 2>/dev/null || echo "  No workspaces"
echo ""

echo "🏷️  Tags (top 10):"
sqlite3 "$DB_PATH" "SELECT tag, COUNT(*) as count FROM file_tags GROUP BY tag ORDER BY count DESC LIMIT 10;" 2>/dev/null || echo "  No tags"
echo ""

echo "📂 Contexts/Projects (top 10):"
sqlite3 "$DB_PATH" "SELECT context, COUNT(*) as count FROM file_contexts GROUP BY context ORDER BY count DESC LIMIT 10;" 2>/dev/null || echo "  No contexts"
echo ""

echo "🤖 AI Metadata:"
sqlite3 "$DB_PATH" "SELECT 
  COUNT(*) as total_metadata,
  COUNT(ai_summary) as with_summary,
  COUNT(ai_category) as with_category,
  COUNT(ai_related) as with_related
FROM file_metadata;" 2>/dev/null || echo "  No metadata"
echo ""

echo "📄 Documents & Embeddings:"
sqlite3 "$DB_PATH" "SELECT 
  (SELECT COUNT(*) FROM documents) as documents,
  (SELECT COUNT(*) FROM chunks) as chunks,
  (SELECT COUNT(*) FROM chunk_embeddings) as embeddings,
  (SELECT COUNT(DISTINCT workspace_id) FROM chunk_embeddings) as workspaces_with_embeddings;" 2>/dev/null || echo "  No documents/embeddings"
echo ""

echo "✅ Database check complete"







