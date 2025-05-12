package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode"

	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/models"
	appstore "github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
)

// Ensure pgUserStore implements store.UserStore.
var _ istore.UserStore = (*pgUserStore)(nil)

type pgUserStore struct {
	pool           *pgxpool.Pool
	supabaseURL    string
	supabaseAPIKey string
}

// NewPgUserStore creates a new PostgreSQL user store.
// NOTE: This assumes a local 'users' table mirroring models.User exists.
// A migration would be needed to create this table.
func NewPgUserStore(pool *pgxpool.Pool, supabaseURL, supabaseAPIKey string) istore.UserStore {
	return &pgUserStore{
		pool:           pool,
		supabaseURL:    supabaseURL,
		supabaseAPIKey: supabaseAPIKey,
	}
}

// Helper function to convert types.User to models.User for DB operations
func typesUserToModelsUser(tUser *types.User) *models.User {
	if tUser == nil {
		return nil
	}
	userID, _ := uuid.Parse(tUser.ID)
	var preferencesJSON []byte
	if tUser.Preferences != nil {
		preferencesJSON, _ = json.Marshal(tUser.Preferences) // Handle error from Marshal
	}
	return &models.User{
		ID:                userID,
		SupabaseID:        tUser.SupabaseID,
		Username:          tUser.Username,
		FirstName:         tUser.FirstName,
		LastName:          tUser.LastName,
		Email:             tUser.Email,
		ProfilePictureURL: tUser.ProfilePictureURL,
		RawUserMetaData:   tUser.RawUserMetaData,
		CreatedAt:         tUser.CreatedAt,
		UpdatedAt:         tUser.UpdatedAt,
		LastSeenAt:        tUser.LastSeenAt,
		IsOnline:          tUser.IsOnline,
		Preferences:       preferencesJSON,
	}
}

// Helper function to convert models.User (from DB) to types.User for interface consistency
func modelsUserToTypesUser(mUser *models.User) *types.User {
	if mUser == nil {
		return nil
	}
	var preferencesMap map[string]interface{}
	if mUser.Preferences != nil {
		_ = json.Unmarshal(mUser.Preferences, &preferencesMap) // Handle error from Unmarshal
	}
	return &types.User{
		ID:                mUser.ID.String(),
		SupabaseID:        mUser.SupabaseID,
		Username:          mUser.Username,
		FirstName:         mUser.FirstName,
		LastName:          mUser.LastName,
		Email:             mUser.Email,
		ProfilePictureURL: mUser.ProfilePictureURL,
		RawUserMetaData:   mUser.RawUserMetaData,
		CreatedAt:         mUser.CreatedAt,
		UpdatedAt:         mUser.UpdatedAt,
		LastSeenAt:        mUser.LastSeenAt,
		IsOnline:          mUser.IsOnline,
		Preferences:       preferencesMap,
	}
}

// GetUserByID retrieves a user by ID from the database and returns *types.User.
func (s *pgUserStore) GetUserByID(ctx context.Context, userIDStr string) (*types.User, error) {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}
	query := `SELECT id, supabase_id, username, first_name, last_name, email, 
              profile_picture_url, raw_user_meta_data, created_at, updated_at,
              last_seen_at, is_online, preferences
	          FROM users
	          WHERE id = $1`
	mUser := &models.User{}
	err = s.pool.QueryRow(ctx, query, userID).Scan(
		&mUser.ID, &mUser.SupabaseID, &mUser.Username, &mUser.FirstName, &mUser.LastName, &mUser.Email,
		&mUser.ProfilePictureURL, &mUser.RawUserMetaData, &mUser.CreatedAt, &mUser.UpdatedAt,
		&mUser.LastSeenAt, &mUser.IsOnline, &mUser.Preferences,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with id %s not found: %w", userIDStr, appstore.ErrNotFound)
		}
		return nil, errors.Wrap(err, "failed to get user by id")
	}
	return modelsUserToTypesUser(mUser), nil
}

// GetUserByEmail retrieves a user by email from the database and returns *types.User.
func (s *pgUserStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	query := `SELECT id, supabase_id, username, first_name, last_name, email, 
              profile_picture_url, raw_user_meta_data, created_at, updated_at,
              last_seen_at, is_online, preferences
	          FROM users
	          WHERE email = $1`
	mUser := &models.User{}
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&mUser.ID, &mUser.SupabaseID, &mUser.Username, &mUser.FirstName, &mUser.LastName, &mUser.Email,
		&mUser.ProfilePictureURL, &mUser.RawUserMetaData, &mUser.CreatedAt, &mUser.UpdatedAt,
		&mUser.LastSeenAt, &mUser.IsOnline, &mUser.Preferences,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with email %s not found: %w", email, appstore.ErrNotFound)
		}
		return nil, errors.Wrap(err, "failed to get user by email")
	}
	return modelsUserToTypesUser(mUser), nil
}

// GetUserBySupabaseID retrieves a user by Supabase ID and returns *types.User.
func (s *pgUserStore) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*types.User, error) {
	query := `SELECT id, supabase_id, username, first_name, last_name, email, 
              profile_picture_url, raw_user_meta_data, created_at, updated_at,
              last_seen_at, is_online, preferences
	          FROM users
	          WHERE supabase_id = $1`
	mUser := &models.User{}
	err := s.pool.QueryRow(ctx, query, supabaseID).Scan(
		&mUser.ID, &mUser.SupabaseID, &mUser.Username, &mUser.FirstName, &mUser.LastName, &mUser.Email,
		&mUser.ProfilePictureURL, &mUser.RawUserMetaData, &mUser.CreatedAt, &mUser.UpdatedAt,
		&mUser.LastSeenAt, &mUser.IsOnline, &mUser.Preferences,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with Supabase ID %s not found: %w", supabaseID, appstore.ErrNotFound)
		}
		return nil, errors.Wrap(err, "failed to get user by Supabase ID")
	}
	return modelsUserToTypesUser(mUser), nil
}

// CreateUser creates a new user in the database, taking *types.User.
func (s *pgUserStore) CreateUser(ctx context.Context, user *types.User) (string, error) {
	mUser := typesUserToModelsUser(user)
	if mUser == nil {
		return "", errors.New("cannot create user from nil types.User")
	}
	if mUser.ID == uuid.Nil { // If ID is not pre-set, generate one
		mUser.ID = uuid.New()
	}

	query := `
		INSERT INTO users (
			id, supabase_id, username, first_name, last_name, email, 
			profile_picture_url, raw_user_meta_data, last_seen_at, is_online, preferences, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING id, created_at, updated_at 
	` // Ensure 'NOW()' is appropriate for your DB or pass mUser.CreatedAt, mUser.UpdatedAt
	var returnedID uuid.UUID
	var createdAt, updatedAt time.Time

	err := s.pool.QueryRow(ctx, query,
		mUser.ID,
		mUser.SupabaseID, mUser.Username, mUser.FirstName, mUser.LastName, mUser.Email,
		mUser.ProfilePictureURL, mUser.RawUserMetaData, mUser.LastSeenAt, mUser.IsOnline, mUser.Preferences,
	).Scan(&returnedID, &createdAt, &updatedAt)

	if err != nil {
		return "", errors.Wrap(err, "failed to create user in DB")
	}
	return returnedID.String(), nil
}

// UpdateUser updates an existing user, taking updates map and returning *types.User.
func (s *pgUserStore) UpdateUser(ctx context.Context, userIDStr string, updates map[string]interface{}) (*types.User, error) {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format for update: %w", err)
	}

	query := "UPDATE users SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{userID}
	paramCount := 1

	for field, value := range updates {
		paramCount++
		// Ensure camelToSnake is defined or imported
		dbField := camelToSnake(field)

		if dbField == "preferences" && value != nil {
			if prefMap, ok := value.(map[string]interface{}); ok {
				preferencesJSON, MappedPort := json.Marshal(prefMap)
				if MappedPort != nil {
					return nil, errors.Wrap(MappedPort, "failed to marshal preferences for update")
				}
				value = preferencesJSON
			}
		}
		query += fmt.Sprintf(", %s = $%d", dbField, paramCount)
		args = append(args, value)
	}

	query += ` WHERE id = $1
		RETURNING id, supabase_id, username, first_name, last_name, email, 
		profile_picture_url, raw_user_meta_data, created_at, updated_at,
		last_seen_at, is_online, preferences`

	mUser := &models.User{}
	err = s.pool.QueryRow(ctx, query, args...).Scan(
		&mUser.ID, &mUser.SupabaseID, &mUser.Username, &mUser.FirstName, &mUser.LastName, &mUser.Email,
		&mUser.ProfilePictureURL, &mUser.RawUserMetaData, &mUser.CreatedAt, &mUser.UpdatedAt,
		&mUser.LastSeenAt, &mUser.IsOnline, &mUser.Preferences,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user with id %s not found for update: %w", userIDStr, appstore.ErrNotFound)
		}
		return nil, errors.Wrap(err, "failed to update user")
	}
	return modelsUserToTypesUser(mUser), nil
}

// ListUsers retrieves a paginated list of users.
func (s *pgUserStore) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, int, error) {
	countQuery := "SELECT COUNT(*) FROM users"
	var total int
	if err := s.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "failed to count users")
	}

	if total == 0 {
		return []*types.User{}, 0, nil
	}

	query := `SELECT id, supabase_id, username, first_name, last_name, email, 
              profile_picture_url, raw_user_meta_data, created_at, updated_at,
              last_seen_at, is_online, preferences
	          FROM users
	          ORDER BY created_at DESC
	          LIMIT $1 OFFSET $2`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to list users")
	}
	defer rows.Close()

	users := make([]*types.User, 0)
	for rows.Next() {
		mUser := &models.User{}
		err := rows.Scan(
			&mUser.ID, &mUser.SupabaseID, &mUser.Username, &mUser.FirstName, &mUser.LastName, &mUser.Email,
			&mUser.ProfilePictureURL, &mUser.RawUserMetaData, &mUser.CreatedAt, &mUser.UpdatedAt,
			&mUser.LastSeenAt, &mUser.IsOnline, &mUser.Preferences,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "failed to scan user row")
		}
		users = append(users, modelsUserToTypesUser(mUser))
	}

	if err := rows.Err(); err != nil {
		return nil, 0, errors.Wrap(err, "error iterating user rows")
	}

	return users, total, nil
}

// SyncUserFromSupabase fetches user data from Supabase and syncs it with the local database.
// This is a placeholder implementation. Actual Supabase interaction needs to be added.
func (s *pgUserStore) SyncUserFromSupabase(ctx context.Context, supabaseID string) (*types.User, error) {
	// 1. Fetch user from Supabase API (not implemented here)
	// Example: fetchedSupabaseUser, err := fetchSupabaseUserAPI(s.supabaseURL, s.supabaseAPIKey, supabaseID)
	// if err != nil { return nil, err }

	// For now, we'll simulate a fetch or just try to get by supabase ID
	// and if not found, it implies it might need creation/syncing
	existingUser, err := s.GetUserBySupabaseID(ctx, supabaseID)
	if err == nil && existingUser != nil {
		// User exists, potentially update if needed (e.g. if fetchedSupabaseUser has newer data)
		// For now, just return existing user
		return existingUser, nil
	}

	// If user not found by Supabase ID, or if fetchedSupabaseUser indicates a new user:
	// Potentially create a new types.User from fetchedSupabaseUser data
	// newUser := &types.User{ SupabaseID: supabaseID, Email: ..., Username: ...}
	// _, createErr := s.CreateUser(ctx, newUser)
	// if createErr != nil { return nil, createErr }
	// return newUser, nil

	return nil, errors.New("SyncUserFromSupabase: full implementation pending Supabase API call and logic")
}

// GetSupabaseUser gets a user directly from Supabase.
// This is a placeholder implementation. Actual Supabase interaction needs to be added.
func (s *pgUserStore) GetSupabaseUser(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	// This method would typically fetch a user from Supabase API by the local user ID's associated Supabase ID.
	// 1. Get local user to find SupabaseID
	// localUser, err := s.GetUserByID(ctx, userID)
	// if err != nil { return nil, err }
	// if localUser.SupabaseID == "" { return nil, errors.New("user does not have a Supabase ID") }
	// 2. Fetch from Supabase API using localUser.SupabaseID (not implemented here)
	// supabaseAPIUser, err := fetchRawSupabaseUser(s.supabaseURL, s.supabaseAPIKey, localUser.SupabaseID)
	// if err != nil { return nil, err }
	// return supabaseAPIUser, nil

	return nil, errors.New("GetSupabaseUser: full implementation pending Supabase API call")
}

// ConvertToUserResponse converts a *types.User to a types.UserResponse.
func (s *pgUserStore) ConvertToUserResponse(user *types.User) types.UserResponse {
	if user == nil {
		// Return a zero-value UserResponse or handle as an error appropriately
		return types.UserResponse{}
	}
	return types.UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		AvatarURL:   user.ProfilePictureURL, // Map ProfilePictureURL to AvatarURL
		DisplayName: user.GetDisplayName(),  // Use the GetDisplayName method
	}
}

// GetUserProfile retrieves a user profile for API responses.
// This might involve joining data or specific formatting.
// For now, it can reuse GetUserByID and ConvertToUserResponse.
func (s *pgUserStore) GetUserProfile(ctx context.Context, userID string) (*types.UserProfile, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err // Handles store.ErrNotFound appropriately
	}

	// types.UserProfile includes LastSeenAt and IsOnline, which types.User has.
	// It also includes AvatarURL and DisplayName, similar to UserResponse.
	return &types.UserProfile{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
		LastSeenAt:  user.LastSeenAt,
		IsOnline:    user.IsOnline,
	}, nil
}

// GetUserProfiles retrieves multiple user profiles.
func (s *pgUserStore) GetUserProfiles(ctx context.Context, userIDs []string) (map[string]*types.UserProfile, error) {
	if len(userIDs) == 0 {
		return make(map[string]*types.UserProfile), nil
	}

	// Create a placeholder for each ID to ensure all are checked
	// This helps in differentiating between a user not found and an error during fetch.
	// However, for simplicity here, we will just fetch and populate.

	// Build the query with IN clause
	// Note: Using uuid.Parse for each ID and then joining them for the query string
	// can be complex. Using ANY($1::uuid[]) is often better with pgx.

	// Simplified approach: fetch one by one. Not efficient for many IDs.
	// For a more performant version, use an IN clause or JOIN.
	profiles := make(map[string]*types.UserProfile)
	for _, userID := range userIDs {
		profile, err := s.GetUserProfile(ctx, userID)
		if err != nil {
			if errors.Is(err, appstore.ErrNotFound) {
				// Optionally, decide if not finding a user is an error for the whole batch
				// or if it should be skipped. Here, we skip.
				continue
			}
			return nil, errors.Wrapf(err, "failed to get user profile for ID %s", userID)
		}
		profiles[userID] = profile
	}
	return profiles, nil
}

// UpdateLastSeen updates a user's last seen timestamp.
func (s *pgUserStore) UpdateLastSeen(ctx context.Context, userIDStr string) error {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID format for UpdateLastSeen: %w", err)
	}
	query := `UPDATE users SET last_seen_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err = s.pool.Exec(ctx, query, userID)
	if err != nil {
		return errors.Wrap(err, "failed to update last seen time")
	}
	return nil
}

// SetOnlineStatus sets a user's online status.
func (s *pgUserStore) SetOnlineStatus(ctx context.Context, userIDStr string, isOnline bool) error {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID format for SetOnlineStatus: %w", err)
	}
	query := `UPDATE users SET is_online = $1, updated_at = NOW() WHERE id = $2`
	_, err = s.pool.Exec(ctx, query, isOnline, userID)
	if err != nil {
		return errors.Wrap(err, "failed to set online status")
	}
	return nil
}

// UpdateUserPreferences updates a user's preferences.
func (s *pgUserStore) UpdateUserPreferences(ctx context.Context, userIDStr string, preferences map[string]interface{}) error {
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user ID format for UpdateUserPreferences: %w", err)
	}

	preferencesJSON, MappedPort := json.Marshal(preferences)
	if MappedPort != nil {
		return errors.Wrap(MappedPort, "failed to marshal preferences for update")
	}

	query := `UPDATE users SET preferences = $1, updated_at = NOW() WHERE id = $2`
	_, err = s.pool.Exec(ctx, query, preferencesJSON, userID)
	if err != nil {
		return errors.Wrap(err, "failed to update user preferences")
	}
	return nil
}

// BeginTx starts a new database transaction.
func (s *pgUserStore) BeginTx(ctx context.Context) (types.DatabaseTransaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	return &pgTransaction{tx: tx}, nil
}

// pgTransaction implements types.DatabaseTransaction
type pgTransaction struct {
	tx pgx.Tx
}

func (t *pgTransaction) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *pgTransaction) Rollback() error {
	return t.tx.Rollback(context.Background())
}

func (t *pgTransaction) GetRawConnection() interface{} {
	return t.tx
}

// camelToSnake converts camelCase to snake_case.
// Ensure this is correctly implemented and handles edge cases.
// This is a simplified version.
func camelToSnake(s string) string {
	// This is a very basic version. A more robust one would handle acronyms etc.
	// For example, "SupabaseID" -> "supabase_id"
	// This simple one might do "supabase_i_d"
	// Consider using a library or a more robust implementation if needed.
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if (s[i-1] >= 'a' && s[i-1] <= 'z') || (i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' && s[i-1] >= 'A' && s[i-1] <= 'Z') {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}
