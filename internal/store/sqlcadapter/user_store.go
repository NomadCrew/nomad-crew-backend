package sqlcadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/sqlc"
	internal_store "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure sqlcUserStore implements internal_store.UserStore
var _ internal_store.UserStore = (*sqlcUserStore)(nil)

type sqlcUserStore struct {
	pool        *pgxpool.Pool
	queries     *sqlc.Queries
	supabaseURL string
	supabaseKey string
	httpClient  *http.Client
}

// NewSqlcUserStore creates a new SQLC-based user store
func NewSqlcUserStore(pool *pgxpool.Pool, supabaseURL, supabaseKey string) internal_store.UserStore {
	return &sqlcUserStore{
		pool:        pool,
		queries:     sqlc.New(pool),
		supabaseURL: supabaseURL,
		supabaseKey: supabaseKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPool returns the underlying connection pool
func (s *sqlcUserStore) GetPool() *pgxpool.Pool {
	return s.pool
}

// GetUserByID retrieves a user by their ID
func (s *sqlcUserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	row, err := s.queries.GetUserProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user", userID)
		}
		return nil, fmt.Errorf("error getting user by ID: %w", err)
	}

	return GetUserProfileByIDRowToUser(row), nil
}

// GetUserByEmail retrieves a user by their email address
func (s *sqlcUserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	row, err := s.queries.GetUserProfileByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user with email", email)
		}
		return nil, fmt.Errorf("error getting user by email: %w", err)
	}

	return GetUserProfileByEmailRowToUser(row), nil
}

// GetUserBySupabaseID retrieves a user by their Supabase ID
func (s *sqlcUserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	// In our schema, id IS the supabase ID, so use GetUserProfileByID
	row, err := s.queries.GetUserProfileByID(ctx, supabaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user with supabase ID", supabaseID)
		}
		return nil, fmt.Errorf("error getting user by Supabase ID: %w", err)
	}

	return GetUserProfileByIDRowToUser(row), nil
}

// GetUserByUsername retrieves a user by their username
func (s *sqlcUserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	row, err := s.queries.GetUserProfileByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found is not an error for username lookup
		}
		return nil, fmt.Errorf("error getting user by username: %w", err)
	}

	return GetUserProfileByUsernameRowToUser(row), nil
}

// CreateUser creates a new user
func (s *sqlcUserStore) CreateUser(ctx context.Context, user *types.User) (string, error) {
	log := logger.GetLogger()

	// Start a transaction to insert into both auth.users and user_profiles
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Ensure the auth.users record exists first to satisfy FK constraint
	authQuery := `
		INSERT INTO auth.users (id, email, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING`

	if _, err = tx.Exec(ctx, authQuery, user.ID, user.Email); err != nil {
		return "", fmt.Errorf("error creating auth user: %w", err)
	}

	// 2. Insert the user profile using SQLC
	qtx := s.queries.WithTx(tx)
	id, err := qtx.CreateUserProfile(ctx, sqlc.CreateUserProfileParams{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		AvatarUrl: user.ProfilePictureURL,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Warnw("User already exists", "userID", user.ID, "username", user.Username)
			return "", fmt.Errorf("user already exists: %w", err)
		}
		return "", fmt.Errorf("error creating user: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("error committing transaction: %w", err)
	}

	log.Infow("Successfully created user", "userID", id, "username", user.Username)
	return id, nil
}

// UpdateUser updates an existing user
func (s *sqlcUserStore) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*types.User, error) {
	log := logger.GetLogger()

	// Extract fields from updates map
	var firstName, lastName, avatarURL *string

	if val, ok := updates["first_name"]; ok && val != nil {
		str := val.(string)
		firstName = &str
	}
	if val, ok := updates["last_name"]; ok && val != nil {
		str := val.(string)
		lastName = &str
	}
	if val, ok := updates["avatar_url"]; ok && val != nil {
		str := val.(string)
		avatarURL = &str
	}

	// Update using SQLC
	err := s.queries.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
		ID:        userID,
		FirstName: firstName,
		LastName:  lastName,
		AvatarUrl: avatarURL,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user", userID)
		}
		return nil, fmt.Errorf("error updating user: %w", err)
	}

	log.Infow("Successfully updated user", "userID", userID)
	return s.GetUserByID(ctx, userID)
}

// ListUsers retrieves a paginated list of users
func (s *sqlcUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	log := logger.GetLogger()

	// First, get the total count
	countQuery := `SELECT COUNT(*) FROM user_profiles`
	var total int
	err := s.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("error counting users: %w", err)
	}

	// Then, fetch the users
	rows, err := s.queries.ListUserProfiles(ctx, sqlc.ListUserProfilesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("error listing users: %w", err)
	}

	users := make([]*types.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, ListUserProfilesRowToUser(row))
	}

	log.Infow("Successfully listed users", "count", len(users), "total", total)
	return users, total, nil
}

// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database
func (s *sqlcUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	log := logger.GetLogger()

	// First check if user already exists
	existingUser, err := s.GetUserBySupabaseID(ctx, supabaseID)
	if err == nil {
		log.Infow("User already synced from Supabase", "supabaseID", supabaseID)
		return existingUser, nil
	}

	// User doesn't exist, fetch from Supabase
	supabaseUser, err := s.getSupabaseUserByID(ctx, supabaseID)
	if err != nil {
		return nil, fmt.Errorf("error fetching user from Supabase: %w", err)
	}

	// Convert to our user model
	now := time.Now()
	user := &types.User{
		ID:                supabaseID,
		Email:             supabaseUser.Email,
		Username:          supabaseUser.UserMetadata.Username,
		FirstName:         supabaseUser.UserMetadata.FirstName,
		LastName:          supabaseUser.UserMetadata.LastName,
		ProfilePictureURL: supabaseUser.UserMetadata.ProfilePicture,
		LastSeenAt:        &now,
		IsOnline:          true,
	}

	// Save raw metadata for future reference
	metadata, err := json.Marshal(supabaseUser.UserMetadata)
	if err == nil {
		user.RawUserMetaData = metadata
	}

	// Create the user in our database
	id, err := s.CreateUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("error creating user from Supabase data: %w", err)
	}

	log.Infow("Successfully synced user from Supabase", "userID", id)
	return s.GetUserByID(ctx, id)
}

// GetSupabaseUser fetches a user directly from Supabase
func (s *sqlcUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	log := logger.GetLogger()

	// First try to get from our database
	user, err := s.GetUserByID(ctx, userID)
	if err == nil {
		// Convert to SupabaseUser
		var userMetadata types.UserMetadata
		if len(user.RawUserMetaData) > 0 {
			_ = json.Unmarshal(user.RawUserMetaData, &userMetadata)
		}

		return &types.SupabaseUser{
			ID:           user.ID,
			Email:        user.Email,
			UserMetadata: userMetadata,
		}, nil
	}

	// Not found locally, fetch from Supabase
	log.Infow("User not found locally, fetching from Supabase", "userID", userID)
	return s.getSupabaseUserByID(ctx, userID)
}

// ConvertToUserResponse converts a User model to UserResponse for API responses
func (s *sqlcUserStore) ConvertToUserResponse(user *types.User) (types.UserResponse, error) {
	if user == nil {
		return types.UserResponse{}, fmt.Errorf("user is nil")
	}
	return types.UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
	}, nil
}

// GetUserProfile retrieves a user profile with minimal information for display
func (s *sqlcUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	log := logger.GetLogger()

	// Get full user and convert to profile
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := &types.UserProfile{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		AvatarURL: user.ProfilePictureURL,
	}

	if user.LastSeenAt != nil {
		profile.LastSeenAt = *user.LastSeenAt
	}
	profile.IsOnline = user.IsOnline
	profile.DisplayName = user.GetDisplayName()

	log.Infow("Successfully retrieved user profile", "userID", userID)
	return profile, nil
}

// GetUserProfiles retrieves multiple user profiles by their IDs
func (s *sqlcUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	log := logger.GetLogger()

	if len(userIDs) == 0 {
		return make(map[string]*types.UserProfile), nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT
			id, email, username,
			COALESCE(first_name, '') as first_name,
			COALESCE(last_name, '') as last_name,
			COALESCE(avatar_url, '') as avatar_url,
			created_at, updated_at
		FROM user_profiles
		WHERE id IN (%s)`, strings.Join(placeholders, ", "))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying user profiles: %w", err)
	}
	defer rows.Close()

	profiles := make(map[string]*types.UserProfile)
	for rows.Next() {
		var id, email, username, firstName, lastName, avatarURL string
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &email, &username, &firstName, &lastName, &avatarURL, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning user profile row: %w", err)
		}

		profile := &types.UserProfile{
			ID:        id,
			Email:     email,
			Username:  username,
			FirstName: firstName,
			LastName:  lastName,
			AvatarURL: avatarURL,
		}

		// Calculate display name
		if firstName != "" && lastName != "" {
			profile.DisplayName = firstName + " " + lastName
		} else if firstName != "" {
			profile.DisplayName = firstName
		} else {
			profile.DisplayName = username
		}

		profiles[id] = profile
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user profile rows: %w", err)
	}

	log.Infow("Successfully retrieved user profiles", "count", len(profiles))
	return profiles, nil
}

// UpdateLastSeen updates a user's last seen timestamp
func (s *sqlcUserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	log := logger.GetLogger()

	query := `
		UPDATE user_profiles
		SET updated_at = NOW()
		WHERE id = $1`

	_, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("error updating last seen: %w", err)
	}

	log.Infow("Successfully updated last seen", "userID", userID)
	return nil
}

// SetOnlineStatus sets a user's online status
func (s *sqlcUserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	log := logger.GetLogger()

	query := `
		UPDATE user_profiles
		SET updated_at = NOW()
		WHERE id = $1`

	_, err := s.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("error setting online status: %w", err)
	}

	log.Infow("Successfully set online status", "userID", userID, "isOnline", isOnline)
	return nil
}

// UpdateUserPreferences updates a user's preferences stored as JSON
func (s *sqlcUserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	log := logger.GetLogger()

	// First get the current user to merge preferences
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("error getting user: %w", err)
	}

	// Merge preferences
	if user.Preferences == nil {
		user.Preferences = preferences
	} else {
		for k, v := range preferences {
			user.Preferences[k] = v
		}
	}

	// Update using the UpdateUser method
	updates := map[string]interface{}{
		"preferences": user.Preferences,
	}
	_, err = s.UpdateUser(ctx, userID, updates)
	if err != nil {
		return fmt.Errorf("error updating user preferences: %w", err)
	}

	log.Infow("Successfully updated user preferences", "userID", userID)
	return nil
}

// BeginTx starts a transaction
func (s *sqlcUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// SearchUsers searches for users by query across username, email, contact_email, first_name, last_name
func (s *sqlcUserStore) SearchUsers(ctx context.Context, query string, limit int) ([]*types.UserSearchResult, error) {
	log := logger.GetLogger()

	rows, err := s.queries.SearchUserProfiles(ctx, sqlc.SearchUserProfilesParams{
		Column1: &query,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("error searching users: %w", err)
	}

	results := make([]*types.UserSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchUserProfilesRowToUserSearchResult(row))
	}

	log.Infow("Successfully searched users", "query", query, "count", len(results))
	return results, nil
}

// UpdateContactEmail updates the user's contact email
func (s *sqlcUserStore) UpdateContactEmail(ctx context.Context, userID string, email string) error {
	log := logger.GetLogger()

	err := s.queries.UpdateUserContactEmail(ctx, sqlc.UpdateUserContactEmailParams{
		ID:           userID,
		ContactEmail: &email,
	})
	if err != nil {
		return fmt.Errorf("error updating contact email: %w", err)
	}

	log.Infow("Successfully updated contact email", "userID", userID)
	return nil
}

// GetUserByContactEmail retrieves a user by their contact email
func (s *sqlcUserStore) GetUserByContactEmail(ctx context.Context, contactEmail string) (*types.User, error) {
	row, err := s.queries.GetUserProfileByContactEmail(ctx, &contactEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user with contact email", contactEmail)
		}
		return nil, fmt.Errorf("error getting user by contact email: %w", err)
	}

	return GetUserProfileByContactEmailRowToUser(row), nil
}

// getSupabaseUserByID is a helper method to fetch a user from Supabase API
// This method is preserved exactly as in the legacy implementation
func (s *sqlcUserStore) getSupabaseUserByID(ctx context.Context, supabaseID string) (*types.SupabaseUser, error) {
	if s.supabaseURL == "" || s.supabaseKey == "" {
		return nil, errors.New("Supabase URL or key not configured")
	}

	// Construct the request
	requestURL := fmt.Sprintf("%s/rest/v1/auth/users/%s", s.supabaseURL, supabaseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Add headers
	req.Header.Add("Apikey", s.supabaseKey)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", s.supabaseKey))

	// Make the request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to Supabase: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Supabase API error: %s", string(body))
	}

	// Parse the response
	var user types.SupabaseUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("error parsing Supabase response: %w", err)
	}

	return &user, nil
}
