package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure pgNotificationStore implements store.NotificationStore.
var _ store.NotificationStore = (*pgNotificationStore)(nil)

type pgNotificationStore struct {
	pool *pgxpool.Pool
}

// NewPgNotificationStore creates a new PostgreSQL notification store.
func NewPgNotificationStore(pool *pgxpool.Pool) store.NotificationStore {
	return &pgNotificationStore{pool: pool}
}

// Create inserts a new notification into the database.
func (s *pgNotificationStore) Create(ctx context.Context, n *models.Notification) error {
	query := `INSERT INTO notifications (user_id, type, metadata, is_read, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6)
	          RETURNING id, created_at, updated_at`

	// Use current time for created_at and updated_at if not provided
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = now
	}

	err := s.pool.QueryRow(ctx, query,
		n.UserID,
		n.Type,
		n.Metadata,
		n.IsRead,
		n.CreatedAt,
		n.UpdatedAt,
	).Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// GetByID retrieves a notification by its ID.
func (s *pgNotificationStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Notification, error) {
	query := `SELECT id, user_id, type, metadata, is_read, created_at, updated_at
	          FROM notifications
	          WHERE id = $1`

	n := &models.Notification{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&n.ID, &n.UserID, &n.Type, &n.Metadata, &n.IsRead, &n.CreatedAt, &n.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("notification with id %s not found: %w", id, store.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to get notification by id: %w", err)
	}
	return n, nil
}

// GetByUser retrieves notifications for a user with pagination and status filtering.
func (s *pgNotificationStore) GetByUser(ctx context.Context, userID uuid.UUID, limit, offset int, status *bool) ([]models.Notification, error) {
	baseQuery := `SELECT id, user_id, type, metadata, is_read, created_at, updated_at
	              FROM notifications
	              WHERE user_id = $1`
	args := []interface{}{userID}
	argCount := 1

	if status != nil {
		argCount++
		baseQuery += fmt.Sprintf(" AND is_read = $%d", argCount)
		args = append(args, *status)
	}

	argCount++
	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argCount)
	args = append(args, limit)

	argCount++
	baseQuery += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, offset)

	rows, err := s.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications by user: %w", err)
	}
	defer rows.Close()

	notifications := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Metadata, &n.IsRead, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan notification row: %w", err)
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration for notifications: %w", err)
	}

	return notifications, nil
}

// MarkRead marks a single notification as read for a specific user.
func (s *pgNotificationStore) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `UPDATE notifications
	          SET is_read = TRUE
	          WHERE id = $1 AND user_id = $2 AND is_read = FALSE`

	cmdTag, err := s.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		// Check if the notification exists at all for this user
		checkQuery := `SELECT EXISTS(SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2)`
		var exists bool
		if checkErr := s.pool.QueryRow(ctx, checkQuery, id, userID).Scan(&exists); checkErr != nil {
			return fmt.Errorf("failed to check notification existence during mark read: %w", checkErr)
		}

		if !exists {
			return fmt.Errorf("cannot mark notification %s as read: %w", id, store.ErrNotFound)
		}
		// If it exists but RowsAffected is 0, it was likely already read or doesn't belong to user.
		ownerCheckQuery := `SELECT user_id FROM notifications WHERE id = $1`
		var ownerID uuid.UUID
		if ownerErr := s.pool.QueryRow(ctx, ownerCheckQuery, id).Scan(&ownerID); ownerErr != nil {
			if errors.Is(ownerErr, pgx.ErrNoRows) {
				return fmt.Errorf("notification %s not found during ownership check: %w", id, store.ErrNotFound)
			}
			return fmt.Errorf("failed to check notification owner: %w", ownerErr)
		}
		if ownerID != userID {
			return fmt.Errorf("user %s not authorized to mark notification %s as read: %w", userID, id, store.ErrForbidden)
		}
		// If owner is correct and RowsAffected is 0, it was already read. No error needed.
	}

	return nil
}

// MarkAllReadByUser marks all unread notifications as read for a specific user.
func (s *pgNotificationStore) MarkAllReadByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `UPDATE notifications
	          SET is_read = TRUE
	          WHERE user_id = $1 AND is_read = FALSE`

	cmdTag, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}

// GetUnreadCount retrieves the count of unread notifications for a specific user.
func (s *pgNotificationStore) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*)
	          FROM notifications
	          WHERE user_id = $1 AND is_read = FALSE`

	var count int64
	err := s.pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get unread notification count: %w", err)
	}

	return count, nil
}

// Delete removes a notification by its ID, but only if the provided userID matches the notification's owner.
func (s *pgNotificationStore) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM notifications
	          WHERE id = $1 AND user_id = $2`

	cmdTag, err := s.pool.Exec(ctx, query, id, userID)
	if err != nil {
		// Wrap the error for better context upstream
		return fmt.Errorf("failed to execute delete for notification %s: %w", id, err)
	}

	// Check if any row was actually deleted
	if cmdTag.RowsAffected() == 0 {
		// If no rows were affected, determine if it's because the notification doesn't exist
		// or because the user doesn't own it.
		checkQuery := `SELECT EXISTS(SELECT 1 FROM notifications WHERE id = $1)`
		var exists bool
		if checkErr := s.pool.QueryRow(ctx, checkQuery, id).Scan(&exists); checkErr != nil {
			// If the check itself fails, return that error
			return fmt.Errorf("failed to check existence for notification %s during delete: %w", id, checkErr)
		}

		if !exists {
			// Notification not found at all
			return fmt.Errorf("cannot delete notification %s: %w", id, store.ErrNotFound)
		}
		// Notification exists, but was not deleted (likely belongs to another user)
		return fmt.Errorf("user %s not authorized to delete notification %s: %w", userID, id, store.ErrForbidden)
	}

	// Notification successfully deleted
	return nil
}
