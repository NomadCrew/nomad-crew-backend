package sqlcadapter

import (
	"context"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcPushTokenStore implements store.PushTokenStore
var _ store.PushTokenStore = (*sqlcPushTokenStore)(nil)

type sqlcPushTokenStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewSqlcPushTokenStore creates a new SQLC-based push token store
func NewSqlcPushTokenStore(pool *pgxpool.Pool) store.PushTokenStore {
	return &sqlcPushTokenStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// RegisterToken registers or updates a push token for a user
func (s *sqlcPushTokenStore) RegisterToken(ctx context.Context, userID, token, deviceType string) (*types.PushToken, error) {
	log := logger.GetLogger()

	result, err := s.queries.RegisterPushToken(ctx, sqlc.RegisterPushTokenParams{
		UserID:     userID,
		Token:      token,
		DeviceType: deviceType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register push token: %w", err)
	}

	pushToken := convertToPushToken(result)
	log.Infow("Successfully registered push token", "userID", userID, "deviceType", deviceType)
	return pushToken, nil
}

// DeactivateToken deactivates a specific token for a user
func (s *sqlcPushTokenStore) DeactivateToken(ctx context.Context, userID, token string) error {
	log := logger.GetLogger()

	err := s.queries.DeactivatePushToken(ctx, sqlc.DeactivatePushTokenParams{
		UserID: userID,
		Token:  token,
	})
	if err != nil {
		return fmt.Errorf("failed to deactivate push token: %w", err)
	}

	log.Infow("Successfully deactivated push token", "userID", userID)
	return nil
}

// DeactivateAllUserTokens deactivates all tokens for a user
func (s *sqlcPushTokenStore) DeactivateAllUserTokens(ctx context.Context, userID string) error {
	log := logger.GetLogger()

	err := s.queries.DeactivateAllUserTokens(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate all user tokens: %w", err)
	}

	log.Infow("Successfully deactivated all push tokens for user", "userID", userID)
	return nil
}

// GetActiveTokensForUser retrieves all active push tokens for a user
func (s *sqlcPushTokenStore) GetActiveTokensForUser(ctx context.Context, userID string) ([]*types.PushToken, error) {
	tokens, err := s.queries.GetActiveTokensForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tokens: %w", err)
	}

	result := make([]*types.PushToken, len(tokens))
	for i, t := range tokens {
		result[i] = convertToPushToken(t)
	}

	return result, nil
}

// GetActiveTokensForUsers retrieves all active push tokens for multiple users
func (s *sqlcPushTokenStore) GetActiveTokensForUsers(ctx context.Context, userIDs []string) ([]*types.PushToken, error) {
	tokens, err := s.queries.GetActiveTokensForUsers(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tokens for users: %w", err)
	}

	result := make([]*types.PushToken, len(tokens))
	for i, t := range tokens {
		result[i] = convertToPushToken(t)
	}

	return result, nil
}

// InvalidateToken marks a token as invalid
func (s *sqlcPushTokenStore) InvalidateToken(ctx context.Context, token string) error {
	log := logger.GetLogger()

	err := s.queries.InvalidateToken(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to invalidate push token: %w", err)
	}

	log.Infow("Successfully invalidated push token")
	return nil
}

// UpdateTokenLastUsed updates the last_used_at timestamp for a token
func (s *sqlcPushTokenStore) UpdateTokenLastUsed(ctx context.Context, token string) error {
	err := s.queries.UpdateTokenLastUsed(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to update token last used: %w", err)
	}
	return nil
}

// convertToPushToken converts a SQLC UserPushToken to types.PushToken
func convertToPushToken(t *sqlc.UserPushToken) *types.PushToken {
	if t == nil {
		return nil
	}

	isActive := false
	if t.IsActive != nil {
		isActive = *t.IsActive
	}

	pushToken := &types.PushToken{
		ID:         t.ID,
		UserID:     t.UserID,
		Token:      t.Token,
		DeviceType: types.DeviceType(t.DeviceType),
		IsActive:   isActive,
		CreatedAt:  t.CreatedAt.Time,
		UpdatedAt:  t.UpdatedAt.Time,
	}

	if t.LastUsedAt.Valid {
		pushToken.LastUsedAt = &t.LastUsedAt.Time
	}

	return pushToken
}
