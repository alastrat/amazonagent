package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const maxBatchChunk = 50 // Postgres parameter limit is 65535; 50 rows × ~10 cols = 500 params per chunk

// BatchInsert executes a multi-row INSERT in chunks.
// columns: column names. rows: each row is a slice of values matching columns.
func BatchInsert(ctx context.Context, pool *pgxpool.Pool, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	colCount := len(columns)
	colList := strings.Join(columns, ", ")

	for i := 0; i < len(rows); i += maxBatchChunk {
		end := i + maxBatchChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]

		var valueParts []string
		var args []any
		argN := 1
		for _, row := range chunk {
			placeholders := make([]string, colCount)
			for j := range row {
				placeholders[j] = fmt.Sprintf("$%d", argN)
				args = append(args, row[j])
				argN++
			}
			valueParts = append(valueParts, "("+strings.Join(placeholders, ", ")+")")
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, colList, strings.Join(valueParts, ", "))
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("batch insert into %s: %w", table, err)
		}
	}
	return nil
}

// BatchUpsert executes a multi-row INSERT ON CONFLICT DO UPDATE in chunks.
// conflictColumns: columns for ON CONFLICT clause.
// updateColumns: columns to update on conflict (subset of columns).
func BatchUpsert(ctx context.Context, pool *pgxpool.Pool, table string, columns []string, conflictColumns []string, updateColumns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	colCount := len(columns)
	colList := strings.Join(columns, ", ")
	conflictList := strings.Join(conflictColumns, ", ")

	var setClauses []string
	for _, col := range updateColumns {
		setClauses = append(setClauses, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}
	setClause := strings.Join(setClauses, ", ")

	for i := 0; i < len(rows); i += maxBatchChunk {
		end := i + maxBatchChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]

		var valueParts []string
		var args []any
		argN := 1
		for _, row := range chunk {
			placeholders := make([]string, colCount)
			for j := range row {
				placeholders[j] = fmt.Sprintf("$%d", argN)
				args = append(args, row[j])
				argN++
			}
			valueParts = append(valueParts, "("+strings.Join(placeholders, ", ")+")")
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s ON CONFLICT (%s) DO UPDATE SET %s",
			table, colList, strings.Join(valueParts, ", "), conflictList, setClause)
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			return fmt.Errorf("batch upsert into %s: %w", table, err)
		}
	}
	return nil
}
