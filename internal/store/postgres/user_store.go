package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Ensure UserStore implements store.UserStore interface.
var _ store.UserStore = (*UserStore)(nil)

// UserStore implements the store.UserStore interface for PostgreSQL.
type UserStore struct {
	pool        *pgxpool.Pool
	tx          pgx.Tx
	supabaseURL string
	supabaseKey string
	httpClient  *http.Client
}

// NewUserStore creates a new instance of UserStore
func NewUserStore(pool *pgxpool.Pool, supabaseURL, supabaseKey string) *UserStore {
	return &UserStore{
		pool:        pool,
		supabaseURL: supabaseURL,
		supabaseKey: supabaseKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPool returns the underlying connection pool
func (s *UserStore) GetPool() *pgxpool.Pool {
	return s.pool
}

// GetUserByID retrieves a user by their ID
func (s *UserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, 
			created_at, updated_at, profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		FROM users
		WHERE id = $1`

	user := &types.User{}
	var rawMetaData json.RawMessage
	var preferencesJSON json.RawMessage

	row := s.queryRow(ctx, query, userID)
	err := row.Scan(
		&user.ID,
		&user.SupabaseID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ProfilePictureURL,
		&rawMetaData,
		&user.LastSeenAt,
		&user.IsOnline,
		&preferencesJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("error getting user by ID: %w", err)
	}

	// Unmarshal raw user metadata
	if len(rawMetaData) > 0 {
		user.RawUserMetaData = rawMetaData
	}

	// Unmarshal preferences if available
	if len(preferencesJSON) > 0 {
		var prefs map[string]interface{}
		if err := json.Unmarshal(preferencesJSON, &prefs); err == nil {
			user.Preferences = prefs
		}
	}

	return user, nil
}

// GetUserByEmail retrieves a user by their email address
func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, 
			created_at, updated_at, profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		FROM users
		WHERE email = $1`

	user := &types.User{}
	var rawMetaData json.RawMessage
	var preferencesJSON json.RawMessage

	row := s.queryRow(ctx, query, email)
	err := row.Scan(
		&user.ID,
		&user.SupabaseID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ProfilePictureURL,
		&rawMetaData,
		&user.LastSeenAt,
		&user.IsOnline,
		&preferencesJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("error getting user by email: %w", err)
	}

	// Unmarshal raw user metadata
	if len(rawMetaData) > 0 {
		user.RawUserMetaData = rawMetaData
	}

	// Unmarshal preferences if available
	if len(preferencesJSON) > 0 {
		var prefs map[string]interface{}
		if err := json.Unmarshal(preferencesJSON, &prefs); err == nil {
			user.Preferences = prefs
		}
	}

	return user, nil
}

// GetUserBySupabaseID retrieves a user by their Supabase ID
func (s *UserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, 
			created_at, updated_at, profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		FROM users
		WHERE supabase_id = $1`

	user := &types.User{}
	var rawMetaData json.RawMessage
	var preferencesJSON json.RawMessage

	row := s.queryRow(ctx, query, supabaseID)
	err := row.Scan(
		&user.ID,
		&user.SupabaseID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ProfilePictureURL,
		&rawMetaData,
		&user.LastSeenAt,
		&user.IsOnline,
		&preferencesJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("error getting user by Supabase ID: %w", err)
	}

	// Unmarshal raw user metadata
	if len(rawMetaData) > 0 {
		user.RawUserMetaData = rawMetaData
	}

	// Unmarshal preferences if available
	if len(preferencesJSON) > 0 {
		var prefs map[string]interface{}
		if err := json.Unmarshal(preferencesJSON, &prefs); err == nil {
			user.Preferences = prefs
		}
	}

	return user, nil
}

// CreateUser creates a new user
func (s *UserStore) CreateUser(ctx context.Context, user *types.User) (string, error) {
	query := `
		INSERT INTO users (
			supabase_id, username, first_name, last_name, email, 
			profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	var preferencesJSON json.RawMessage
	if user.Preferences != nil {
		data, err := json.Marshal(user.Preferences)
		if err != nil {
			return "", fmt.Errorf("error marshalling preferences: %w", err)
		}
		preferencesJSON = data
	}

	var id string
	err := s.queryRow(ctx, query,
		user.SupabaseID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.Email,
		user.ProfilePictureURL,
		user.RawUserMetaData,
		user.LastSeenAt,
		user.IsOnline,
		preferencesJSON,
	).Scan(&id)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", fmt.Errorf("user already exists: %w", err)
		}
		return "", fmt.Errorf("error creating user: %w", err)
	}

	return id, nil
}

// UpdateUser updates an existing user
func (s *UserStore) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*types.User, error) {
	// Build dynamic SET clause
	setParts := []string{}
	args := []interface{}{userID} // First arg is the userID for the WHERE clause
	argPos := 2                   // Start with $2 since $1 is the userID

	validFields := map[string]string{
		"username":            "username",
		"first_name":          "first_name",
		"last_name":           "last_name",
		"email":               "email",
		"profile_picture_url": "profile_picture_url",
		"raw_user_meta_data":  "raw_user_meta_data",
		"is_online":           "is_online",
		"preferences":         "preferences",
	}

	for field, value := range updates {
		if dbField, ok := validFields[field]; ok {
			// Handle special case for JSON fields
			if field == "preferences" || field == "raw_user_meta_data" {
				jsonData, err := json.Marshal(value)
				if err != nil {
					return nil, fmt.Errorf("error marshalling JSON field %s: %w", field, err)
				}
				setParts = append(setParts, fmt.Sprintf("%s = $%d", dbField, argPos))
				args = append(args, jsonData)
			} else {
				setParts = append(setParts, fmt.Sprintf("%s = $%d", dbField, argPos))
				args = append(args, value)
			}
			argPos++
		}
	}

	// Always update the updated_at timestamp
	setParts = append(setParts, "updated_at = NOW()")

	if len(setParts) == 1 {
		// Only updated_at would be updated, which means no actual changes
		return s.GetUserByID(ctx, userID)
	}

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $1
		RETURNING id`, strings.Join(setParts, ", "))

	var id string
	err := s.queryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("error updating user: %w", err)
	}

	// Fetch the updated user
	return s.GetUserByID(ctx, id)
}

// ListUsers retrieves a paginated list of users
func (s *UserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	// First, get the total count
	countQuery := `SELECT COUNT(*) FROM users`
	var total int
	err := s.queryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("error counting users: %w", err)
	}

	// Then, fetch the users
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, 
			created_at, updated_at, profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		FROM users
		ORDER BY username
		LIMIT $1 OFFSET $2`

	rows, err := s.query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("error listing users: %w", err)
	}
	defer rows.Close()

	var users []*types.User
	for rows.Next() {
		user := &types.User{}
		var rawMetaData json.RawMessage
		var preferencesJSON json.RawMessage

		err := rows.Scan(
			&user.ID,
			&user.SupabaseID,
			&user.Username,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.ProfilePictureURL,
			&rawMetaData,
			&user.LastSeenAt,
			&user.IsOnline,
			&preferencesJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("error scanning user row: %w", err)
		}

		// Unmarshal raw user metadata
		if len(rawMetaData) > 0 {
			user.RawUserMetaData = rawMetaData
		}

		// Unmarshal preferences if available
		if len(preferencesJSON) > 0 {
			var prefs map[string]interface{}
			if err := json.Unmarshal(preferencesJSON, &prefs); err == nil {
				user.Preferences = prefs
			}
		}

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, total, nil
}

// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database
func (s *UserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	// First, check if the user already exists in our database
	existingUser, err := s.GetUserBySupabaseID(ctx, supabaseID)
	if err == nil {
		// User exists, just return it
		return existingUser, nil
	}

	// User doesn't exist, fetch from Supabase
	supabaseUser, err := s.getSupabaseUserByID(ctx, supabaseID)
	if err != nil {
		return nil, fmt.Errorf("error fetching user from Supabase: %w", err)
	}

	// Convert to our user model
	now := time.Now() // Helper for pointer
	user := &types.User{
		SupabaseID:        supabaseID,
		Email:             supabaseUser.Email,
		Username:          supabaseUser.UserMetadata.Username,
		FirstName:         supabaseUser.UserMetadata.FirstName,
		LastName:          supabaseUser.UserMetadata.LastName,
		ProfilePictureURL: supabaseUser.UserMetadata.ProfilePicture,
		LastSeenAt:        &now, // Corrected: assign address of time.Now()
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

	// Fetch the created user
	return s.GetUserByID(ctx, id)
}

// GetSupabaseUser fetches a user directly from Supabase
func (s *UserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
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
	return s.getSupabaseUserByID(ctx, userID)
}

// GetUserProfile retrieves a user profile with minimal information for display
func (s *UserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, profile_picture_url,
			last_seen_at, is_online
		FROM users
		WHERE id = $1`

	row := s.queryRow(ctx, query, userID)
	profile := &types.UserProfile{}
	var supabaseID string
	err := row.Scan(
		&profile.ID,
		&supabaseID,
		&profile.Username,
		&profile.FirstName,
		&profile.LastName,
		&profile.Email,
		&profile.AvatarURL,
		&profile.LastSeenAt,
		&profile.IsOnline,
	)
	profile.SupabaseID = supabaseID

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("error getting user profile: %w", err)
	}

	return profile, nil
}

// GetUserProfiles retrieves multiple user profiles by their IDs
func (s *UserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	if len(userIDs) == 0 {
		return make(map[string]*types.UserProfile), nil
	}

	// Convert the slice of IDs to a string for the IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT 
			id, username, first_name, last_name, email, profile_picture_url,
			last_seen_at, is_online
		FROM users
		WHERE id IN (%s)`, strings.Join(placeholders, ", "))

	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying user profiles: %w", err)
	}
	defer rows.Close()

	profiles := make(map[string]*types.UserProfile)
	for rows.Next() {
		profile := &types.UserProfile{}
		var supabaseID string
		err := rows.Scan(
			&profile.ID,
			&supabaseID,
			&profile.Username,
			&profile.FirstName,
			&profile.LastName,
			&profile.Email,
			&profile.AvatarURL,
			&profile.LastSeenAt,
			&profile.IsOnline,
		)
		profile.SupabaseID = supabaseID
		if err != nil {
			return nil, fmt.Errorf("error scanning user profile row: %w", err)
		}
		profiles[profile.ID] = profile
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user profile rows: %w", err)
	}

	return profiles, nil
}

// UpdateLastSeen updates a user's last seen timestamp
func (s *UserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	query := `
		UPDATE users
		SET last_seen_at = NOW(), updated_at = NOW()
		WHERE id = $1`

	_, err := s.exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("error updating last seen: %w", err)
	}
	return nil
}

// SetOnlineStatus sets a user's online status
func (s *UserStore) SetOnlineStatus(ctx context.Context, userID string, isOnline bool) error {
	query := `
		UPDATE users
		SET is_online = $2, last_seen_at = NOW(), updated_at = NOW()
		WHERE id = $1`

	_, err := s.exec(ctx, query, userID, isOnline)
	if err != nil {
		return fmt.Errorf("error setting online status: %w", err)
	}
	return nil
}

// UpdateUserPreferences updates a user's preferences stored as JSON
func (s *UserStore) UpdateUserPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	// First get the current preferences
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("error getting user: %w", err)
	}

	// If there are no current preferences, just set the new ones
	if user.Preferences == nil {
		user.Preferences = preferences
	} else {
		// Otherwise, merge the new preferences with the existing ones
		for k, v := range preferences {
			user.Preferences[k] = v
		}
	}

	// Update the user with the merged preferences
	updates := map[string]interface{}{
		"preferences": user.Preferences,
	}
	_, err = s.UpdateUser(ctx, userID, updates)
	if err != nil {
		return fmt.Errorf("error updating user preferences: %w", err)
	}

	return nil
}

// Helper method to fetch a user from Supabase
func (s *UserStore) getSupabaseUserByID(ctx context.Context, supabaseID string) (*types.SupabaseUser, error) {
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
	req.Header.Add("apikey", s.supabaseKey)
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

// BeginTx starts a transaction
func (s *UserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	if s.tx != nil {
		return nil, fmt.Errorf("transaction already started")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("error beginning transaction: %w", err)
	}

	txStore := &UserStore{
		pool:        s.pool,
		tx:          tx,
		supabaseURL: s.supabaseURL,
		supabaseKey: s.supabaseKey,
		httpClient:  s.httpClient,
	}

	return txStore, nil
}

// Commit commits the transaction
func (s *UserStore) Commit() error {
	if s.tx == nil {
		return fmt.Errorf("no transaction to commit")
	}

	err := s.tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	s.tx = nil
	return nil
}

// Rollback aborts the transaction
func (s *UserStore) Rollback() error {
	if s.tx == nil {
		return fmt.Errorf("no transaction to rollback")
	}

	err := s.tx.Rollback(context.Background())
	if err != nil {
		return fmt.Errorf("error rolling back transaction: %w", err)
	}

	s.tx = nil
	return nil
}

// Helper methods for database operations

func (s *UserStore) query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if s.tx != nil {
		return s.tx.Query(ctx, query, args...)
	}
	return s.pool.Query(ctx, query, args...)
}

func (s *UserStore) queryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	if s.tx != nil {
		return s.tx.QueryRow(ctx, query, args...)
	}
	return s.pool.QueryRow(ctx, query, args...)
}

func (s *UserStore) exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if s.tx != nil {
		return s.tx.Exec(ctx, query, args...)
	}
	return s.pool.Exec(ctx, query, args...)
}

// ConvertToUserResponse converts a User model to UserResponse for API responses
func (s *UserStore) ConvertToUserResponse(user *types.User) (types.UserResponse, error) {
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

// GetUserByUsername retrieves a user by their username
func (s *UserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	query := `
		SELECT 
			id, supabase_id, username, first_name, last_name, email, 
			created_at, updated_at, profile_picture_url, raw_user_meta_data,
			last_seen_at, is_online, preferences
		FROM users
		WHERE username = $1`

	user := &types.User{}
	var rawMetaData json.RawMessage
	var preferencesJSON json.RawMessage

	row := s.queryRow(ctx, query, username)
	err := row.Scan(
		&user.ID,
		&user.SupabaseID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.ProfilePictureURL,
		&rawMetaData,
		&user.LastSeenAt,
		&user.IsOnline,
		&preferencesJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("error getting user by username: %w", err)
	}

	// Unmarshal raw user metadata
	if len(rawMetaData) > 0 {
		user.RawUserMetaData = rawMetaData
	}

	// Unmarshal preferences if available
	if len(preferencesJSON) > 0 {
		var prefs map[string]interface{}
		if err := json.Unmarshal(preferencesJSON, &prefs); err == nil {
			user.Preferences = prefs
		}
	}

	return user, nil
}
