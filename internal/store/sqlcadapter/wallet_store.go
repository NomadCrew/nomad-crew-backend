package sqlcadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcWalletStore implements internal_store.WalletStore
var _ internal_store.WalletStore = (*sqlcWalletStore)(nil)

type sqlcWalletStore struct {
	pool *pgxpool.Pool
}

// NewSqlcWalletStore creates a new SQLC-based wallet store
func NewSqlcWalletStore(pool *pgxpool.Pool) internal_store.WalletStore {
	return &sqlcWalletStore{
		pool: pool,
	}
}

// CreateDocument creates a new wallet document in the database
func (s *sqlcWalletStore) CreateDocument(ctx context.Context, doc *types.WalletDocument) (string, error) {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO wallet_documents (user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		doc.UserID, doc.TripID, doc.WalletType, doc.DocumentType, doc.Name, doc.Description, doc.FilePath, doc.FileSize, doc.MimeType, metadataJSON,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create wallet document: %w", err)
	}
	return id, nil
}

// GetDocument retrieves a wallet document by ID
func (s *sqlcWalletStore) GetDocument(ctx context.Context, id string) (*types.WalletDocument, error) {
	doc := &types.WalletDocument{}
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata, created_at, updated_at
		FROM wallet_documents WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(
		&doc.ID, &doc.UserID, &doc.TripID, &doc.WalletType, &doc.DocumentType,
		&doc.Name, &doc.Description, &doc.FilePath, &doc.FileSize, &doc.MimeType,
		&metadataJSON, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NotFound("wallet_document", id)
		}
		return nil, fmt.Errorf("failed to get wallet document: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]interface{}{}
	}

	return doc, nil
}

// GetDocumentByFilePath retrieves a non-deleted wallet document by its storage path.
// Returns NotFound if no active document matches the path.
func (s *sqlcWalletStore) GetDocumentByFilePath(ctx context.Context, filePath string) (*types.WalletDocument, error) {
	doc := &types.WalletDocument{}
	var metadataJSON []byte

	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata, created_at, updated_at
		FROM wallet_documents WHERE file_path = $1 AND deleted_at IS NULL`, filePath,
	).Scan(
		&doc.ID, &doc.UserID, &doc.TripID, &doc.WalletType, &doc.DocumentType,
		&doc.Name, &doc.Description, &doc.FilePath, &doc.FileSize, &doc.MimeType,
		&metadataJSON, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NotFound("wallet_document", filePath)
		}
		return nil, fmt.Errorf("failed to get wallet document by file path: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]interface{}{}
	}

	return doc, nil
}

// ListPersonalDocuments retrieves personal documents for a user with pagination
func (s *sqlcWalletStore) ListPersonalDocuments(ctx context.Context, userID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	// Get total count
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM wallet_documents WHERE user_id = $1 AND wallet_type = 'personal' AND deleted_at IS NULL`,
		userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count personal documents: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata, created_at, updated_at
		FROM wallet_documents
		WHERE user_id = $1 AND wallet_type = 'personal' AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list personal documents: %w", err)
	}
	defer rows.Close()

	docs, err := scanWalletDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// ListGroupDocuments retrieves group documents for a trip with pagination
func (s *sqlcWalletStore) ListGroupDocuments(ctx context.Context, tripID string, limit, offset int) ([]*types.WalletDocument, int, error) {
	// Get total count
	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM wallet_documents WHERE trip_id = $1 AND wallet_type = 'group' AND deleted_at IS NULL`,
		tripID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count group documents: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata, created_at, updated_at
		FROM wallet_documents
		WHERE trip_id = $1 AND wallet_type = 'group' AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		tripID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list group documents: %w", err)
	}
	defer rows.Close()

	docs, err := scanWalletDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// UpdateDocument updates a wallet document's mutable fields
func (s *sqlcWalletStore) UpdateDocument(ctx context.Context, id string, update *types.WalletDocumentUpdate) (*types.WalletDocument, error) {
	var metadataJSON []byte
	if update.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(update.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	doc := &types.WalletDocument{}
	var resultMetadataJSON []byte

	err := s.pool.QueryRow(ctx,
		`UPDATE wallet_documents SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			document_type = COALESCE($4, document_type),
			metadata = COALESCE($5, metadata),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, trip_id, wallet_type, document_type, name, description, file_path, file_size, mime_type, metadata, created_at, updated_at`,
		id, update.Name, update.Description, update.DocumentType, metadataJSON,
	).Scan(
		&doc.ID, &doc.UserID, &doc.TripID, &doc.WalletType, &doc.DocumentType,
		&doc.Name, &doc.Description, &doc.FilePath, &doc.FileSize, &doc.MimeType,
		&resultMetadataJSON, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NotFound("wallet_document", id)
		}
		return nil, fmt.Errorf("failed to update wallet document: %w", err)
	}

	if len(resultMetadataJSON) > 0 {
		if err := json.Unmarshal(resultMetadataJSON, &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]interface{}{}
	}

	return doc, nil
}

// SoftDeleteDocument soft-deletes a wallet document
func (s *sqlcWalletStore) SoftDeleteDocument(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE wallet_documents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id,
	)
	if err != nil {
		return fmt.Errorf("failed to soft-delete wallet document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("wallet_document", id)
	}
	return nil
}

// GetUserStorageUsage returns the total file size of personal documents for a user
func (s *sqlcWalletStore) GetUserStorageUsage(ctx context.Context, userID string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(file_size), 0) FROM wallet_documents WHERE user_id = $1 AND wallet_type = 'personal' AND deleted_at IS NULL`,
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get user storage usage: %w", err)
	}
	return total, nil
}

// GetTripStorageUsage returns the total file size of group documents for a trip
func (s *sqlcWalletStore) GetTripStorageUsage(ctx context.Context, tripID string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(file_size), 0) FROM wallet_documents WHERE trip_id = $1 AND wallet_type = 'group' AND deleted_at IS NULL`,
		tripID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get trip storage usage: %w", err)
	}
	return total, nil
}

// PurgeDeletedDocuments hard-deletes soft-deleted documents older than olderThan
// and returns the file paths of purged records for storage cleanup.
func (s *sqlcWalletStore) PurgeDeletedDocuments(ctx context.Context, olderThan time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`DELETE FROM wallet_documents WHERE deleted_at IS NOT NULL AND deleted_at < $1 RETURNING file_path`,
		olderThan,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to purge deleted wallet documents: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan purged file path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate purged documents: %w", err)
	}
	return paths, nil
}

// HardDeleteAllByUser hard-deletes all documents for a user and returns
// the file paths of deleted records for storage cleanup.
func (s *sqlcWalletStore) HardDeleteAllByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`DELETE FROM wallet_documents WHERE user_id = $1 RETURNING file_path`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to hard-delete wallet documents for user: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan deleted file path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate deleted documents: %w", err)
	}
	return paths, nil
}

// scanWalletDocuments scans multiple rows into wallet document structs
func scanWalletDocuments(rows pgx.Rows) ([]*types.WalletDocument, error) {
	var docs []*types.WalletDocument
	for rows.Next() {
		doc := &types.WalletDocument{}
		var metadataJSON []byte
		err := rows.Scan(
			&doc.ID, &doc.UserID, &doc.TripID, &doc.WalletType, &doc.DocumentType,
			&doc.Name, &doc.Description, &doc.FilePath, &doc.FileSize, &doc.MimeType,
			&metadataJSON, &doc.CreatedAt, &doc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan wallet document: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		if doc.Metadata == nil {
			doc.Metadata = map[string]interface{}{}
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate wallet documents: %w", err)
	}
	if docs == nil {
		docs = []*types.WalletDocument{}
	}
	return docs, nil
}
