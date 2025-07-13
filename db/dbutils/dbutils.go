package dbutils

import (
	"context"
	"database/sql"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/jackc/pgx/v4/pgxpool"
)

// EnsureTableExists checks if a table exists in the database
func EnsureTableExists(ctx context.Context, pool *pgxpool.Pool, tableName string) error {
	query := `SELECT table_name FROM information_schema.tables WHERE table_name = $1`
	var existingTable string
	err := pool.QueryRow(ctx, query, tableName).Scan(&existingTable)
	if err != nil && err != sql.ErrNoRows {
		logger.GetLogger().Errorf("Failed to check for table %s: %v", tableName, err)
		return err
	}
	if err == sql.ErrNoRows {
		logger.GetLogger().Warnf("Table %s does not exist", tableName)
	}
	return nil
}
