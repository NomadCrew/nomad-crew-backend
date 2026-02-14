package db

import (
	"embed"
	"errors"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// lastManualMigration is the version of the last migration that was applied
// manually (before golang-migrate was adopted). On first run against an existing
// database, we force-set this version so golang-migrate skips already-applied
// migrations and only runs new ones.
const lastManualMigration = 6

// RunMigrations applies all pending database migrations using golang-migrate.
// It reads migration files embedded in the binary, connects to the database,
// and applies any migrations that haven't been run yet (in numeric order).
// Safe to call on every startup — already-applied migrations are skipped.
//
// Bootstrap handling: if this is the first run against a database that was
// previously managed with manual migrations, the schema_migrations table won't
// exist yet. We detect this (ErrNilVersion) and force-set the version to
// lastManualMigration so that only newer migrations are applied.
func RunMigrations(dbURL string) error {
	log := logger.GetLogger()

	// Create an iofs source from the embedded migration files
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create the migrate instance
	// golang-migrate uses pgx v5 driver, connection string must use pgx5:// scheme
	m, err := migrate.NewWithSourceInstance("iofs", source, convertToPgx5URL(dbURL))
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Check current migration state and handle edge cases.
	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			// Bootstrap: pre-existing database without schema_migrations table.
			// Force-set to lastManualMigration so we skip already-applied migrations.
			log.Infow("No schema_migrations table found, bootstrapping from manual migrations",
				"forcingVersion", lastManualMigration)
			if err := m.Force(lastManualMigration); err != nil {
				return fmt.Errorf("failed to force migration version: %w", err)
			}
		} else {
			return fmt.Errorf("failed to read migration version: %w", err)
		}
	} else if dirty {
		// A previous migration failed partway, leaving dirty state.
		// Force-set to the last known good version so we can retry cleanly.
		cleanVersion := version - 1
		if cleanVersion < uint(lastManualMigration) {
			cleanVersion = uint(lastManualMigration)
		}
		log.Infow("Dirty migration state detected, resetting to retry",
			"dirtyVersion", version,
			"resettingTo", cleanVersion)
		if err := m.Force(int(cleanVersion)); err != nil {
			return fmt.Errorf("failed to reset dirty migration: %w", err)
		}
	} else {
		log.Infow("Current migration version", "version", version)
	}

	// Apply all pending migrations
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("Database is up to date, no migrations to apply")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	// Get the current version after migration
	version, dirty, err = m.Version()
	if err != nil {
		log.Infow("Migrations applied successfully")
	} else {
		log.Infow("Migrations applied successfully",
			"currentVersion", version,
			"dirty", dirty)
	}

	return nil
}

// convertToPgx5URL converts a standard postgres:// URL to the pgx5:// scheme
// required by golang-migrate's pgx v5 driver.
func convertToPgx5URL(dbURL string) string {
	if len(dbURL) >= 11 && dbURL[:11] == "postgresql:" {
		return "pgx5:" + dbURL[11:]
	}
	if len(dbURL) >= 9 && dbURL[:9] == "postgres:" {
		return "pgx5:" + dbURL[9:]
	}
	return dbURL
}
