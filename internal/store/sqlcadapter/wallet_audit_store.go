package sqlcadapter

import (
	"context"
	"fmt"

	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcWalletAuditStore implements internal_store.WalletAuditStore
var _ internal_store.WalletAuditStore = (*sqlcWalletAuditStore)(nil)

type sqlcWalletAuditStore struct {
	pool *pgxpool.Pool
}

// NewSqlcWalletAuditStore creates a new SQLC-based wallet audit store
func NewSqlcWalletAuditStore(pool *pgxpool.Pool) internal_store.WalletAuditStore {
	return &sqlcWalletAuditStore{pool: pool}
}

// LogAccess inserts an audit log entry for a wallet operation
func (s *sqlcWalletAuditStore) LogAccess(ctx context.Context, entry *types.WalletAuditEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO wallet_audit_log (user_id, document_id, action, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)`,
		entry.UserID, entry.DocumentID, entry.Action, entry.IPAddress, entry.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to insert wallet audit log: %w", err)
	}
	return nil
}
