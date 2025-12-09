package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	istore "github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/services"
	appstore "github.com/NomadCrew/nomad-crew-backend/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
)

// JWTValidator defines the interface for JWT validation (from middleware package).
// This avoids circular import while allowing UserService to use JWKS validation.
type JWTValidator interface {
	Validate(tokenString string) (string, error)
	ValidateAndGetClaims(tokenString string) (*types.JWTClaims, error)
}

// UserService manages user operations
type UserService struct {
	userStore       istore.UserStore
	jwtSecret       string
	jwtValidator    JWTValidator
	supabaseService *services.SupabaseService
}

// NewUserService creates a new UserService.
// jwtValidator is optional (can be nil) for backwards compatibility - if nil, falls back to HS256 jwtSecret.
func NewUserService(userStore istore.UserStore, jwtSecret string, supabaseService *services.SupabaseService, jwtValidator JWTValidator) *UserService {
	return &UserService{
		userStore:       userStore,
		jwtSecret:       jwtSecret,
		jwtValidator:    jwtValidator,
		supabaseService: supabaseService,
	}
}

// Helper functions

// maskEmail masks an email address for logging (e.g., "user@example.com" -> "u***@e***.com")
func maskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		// Invalid email format, mask most of it
		if len(email) <= 3 {
			return "***"
		}
		return email[:1] + "***"
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Mask local part (keep first character)
	maskedLocal := localPart
	if len(localPart) > 1 {
		maskedLocal = localPart[:1] + "***"
	}

	// Mask domain part (keep first character and TLD)
	maskedDomain := domainPart
	domainParts := strings.Split(domainPart, ".")
	if len(domainParts) > 1 {
		firstPart := domainParts[0]
		if len(firstPart) > 1 {
			maskedDomain = firstPart[:1] + "***." + strings.Join(domainParts[1:], ".")
		}
	}

	return maskedLocal + "@" + maskedDomain
}

// GetUserByID retrieves a user by their internal ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	log := logger.GetLogger()

	// Convert uuid.UUID to string for the store call
	userIDStr := id.String()
	typesUser, err := s.userStore.GetUserByID(ctx, userIDStr)
	if err != nil {
		log.Errorw("Failed to get user by ID from store", "error", err, "userID", userIDStr)
		return nil, err
	}

	// Convert *types.User to *models.User
	if typesUser == nil {
		// Should not happen if err is nil, but good practice
		return nil, errors.New("user store returned nil user without error")
	}

	var preferencesJSON []byte
	if typesUser.Preferences != nil {
		preferencesJSON, err = json.Marshal(typesUser.Preferences)
		if err != nil {
			log.Warnw("Failed to marshal preferences from types.User", "error", err, "userID", typesUser.ID)
			// Continue without preferences or return error, for now, continue
		}
	}

	modelUser := &models.User{
		ID:                id, // Use the original uuid.UUID
		Username:          typesUser.Username,
		FirstName:         typesUser.FirstName,
		LastName:          typesUser.LastName,
		Email:             typesUser.Email,
		ProfilePictureURL: typesUser.ProfilePictureURL,
		RawUserMetaData:   typesUser.RawUserMetaData, // Assuming types.RawMessage is compatible or nil
		CreatedAt:         typesUser.CreatedAt,
		UpdatedAt:         typesUser.UpdatedAt,
		LastSeenAt:        typesUser.LastSeenAt,
		IsOnline:          typesUser.IsOnline,
		Preferences:       preferencesJSON,
	}

	// If the user data is stale, try to sync with Supabase
	if modelUser.ShouldSync() {
		log.Infow("User data is stale, syncing with Supabase", "userID", id)
		// SyncWithSupabase internally calls store and handles conversion
		syncedModelUser, syncErr := s.SyncWithSupabase(ctx, modelUser.ID.String())
		if syncErr == nil {
			return syncedModelUser, nil
		}
		log.Warnw("Failed to sync user with Supabase, using local data", "error", syncErr, "userID", id)
	}

	return modelUser, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	log := logger.GetLogger()

	typesUser, err := s.userStore.GetUserByEmail(ctx, email)
	if err != nil {
		log.Errorw("Failed to get user by email from store", "error", err, "email", maskEmail(email))
		return nil, err
	}

	if typesUser == nil {
		return nil, errors.New("user store returned nil user without error for email")
	}

	userID, parseErr := uuid.Parse(typesUser.ID)
	if parseErr != nil {
		log.Errorw("Failed to parse user ID from types.User", "error", parseErr, "userIDStr", typesUser.ID)
		return nil, fmt.Errorf("failed to parse user ID from store data: %w", parseErr)
	}

	var preferencesJSON []byte
	if typesUser.Preferences != nil {
		preferencesJSON, err = json.Marshal(typesUser.Preferences)
		if err != nil {
			log.Warnw("Failed to marshal preferences from types.User", "error", err, "userID", typesUser.ID)
		}
	}
	modelUser := &models.User{
		ID:                userID,
		Username:          typesUser.Username,
		FirstName:         typesUser.FirstName,
		LastName:          typesUser.LastName,
		Email:             typesUser.Email,
		ProfilePictureURL: typesUser.ProfilePictureURL,
		RawUserMetaData:   typesUser.RawUserMetaData,
		CreatedAt:         typesUser.CreatedAt,
		UpdatedAt:         typesUser.UpdatedAt,
		LastSeenAt:        typesUser.LastSeenAt,
		IsOnline:          typesUser.IsOnline,
		Preferences:       preferencesJSON,
	}
	return modelUser, nil
}

// GetUserBySupabaseID retrieves a user by Supabase ID
func (s *UserService) GetUserBySupabaseID(ctx context.Context, supabaseID string) (*models.User, error) {
	log := logger.GetLogger()

	typesUser, err := s.userStore.GetUserBySupabaseID(ctx, supabaseID)
	if err != nil {
		log.Errorw("Failed to get user by Supabase ID from store", "error", err, "supabaseID", supabaseID)
		// If not found, try to sync from Supabase (SyncWithSupabase returns *models.User)
		if errors.Is(err, appstore.ErrNotFound) || (typesUser == nil && err.Error() == "user not found") { // Broaden check for not found
			log.Infow("User not found locally by SupabaseID, syncing from Supabase", "supabaseID", supabaseID)
			return s.SyncWithSupabase(ctx, supabaseID)
		}
		return nil, err
	}

	if typesUser == nil {
		// This case might occur if store returns nil, nil but we expect ErrNotFound for sync trigger
		log.Warnw("User store returned nil user for SupabaseID without specific ErrNotFound", "supabaseID", supabaseID)
		// Attempt sync as a fallback
		return s.SyncWithSupabase(ctx, supabaseID)
	}

	userID, parseErr := uuid.Parse(typesUser.ID)
	if parseErr != nil {
		log.Errorw("Failed to parse user ID from types.User", "error", parseErr, "userIDStr", typesUser.ID)
		return nil, fmt.Errorf("failed to parse user ID from store data: %w", parseErr)
	}

	var preferencesJSON []byte
	if typesUser.Preferences != nil {
		preferencesJSON, err = json.Marshal(typesUser.Preferences)
		if err != nil {
			log.Warnw("Failed to marshal preferences from types.User", "error", err, "userID", typesUser.ID)
		}
	}

	modelUser := &models.User{
		ID:                userID,
		Username:          typesUser.Username,
		FirstName:         typesUser.FirstName,
		LastName:          typesUser.LastName,
		Email:             typesUser.Email,
		ProfilePictureURL: typesUser.ProfilePictureURL,
		RawUserMetaData:   typesUser.RawUserMetaData,
		CreatedAt:         typesUser.CreatedAt,
		UpdatedAt:         typesUser.UpdatedAt,
		LastSeenAt:        typesUser.LastSeenAt,
		IsOnline:          typesUser.IsOnline,
		Preferences:       preferencesJSON,
	}

	// If the user data is stale, sync with Supabase
	if modelUser.ShouldSync() {
		log.Infow("User data is stale, syncing with Supabase", "supabaseID", supabaseID)
		// SyncWithSupabase returns *models.User
		syncedModelUser, syncErr := s.SyncWithSupabase(ctx, supabaseID)
		if syncErr == nil {
			return syncedModelUser, nil
		}
		log.Warnw("Failed to sync user with Supabase, using local data", "error", syncErr, "supabaseID", supabaseID)
	}

	return modelUser, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, user *models.User) (uuid.UUID, error) {
	log := logger.GetLogger()

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.Email == "" {
		return uuid.Nil, errors.New("Email is required for creating user")
	}
	if user.Username == "" {
		user.Username = generateUsernameFromEmail(user.Email)
	}

	now := time.Now()      // Helper variable for pointer
	user.LastSeenAt = &now // Assign address of time.Now()
	user.IsOnline = true

	// Convert *models.User to *types.User for the store call
	var preferencesMap map[string]interface{}
	if user.Preferences != nil {
		if err := json.Unmarshal(user.Preferences, &preferencesMap); err != nil {
			log.Warnw("Failed to unmarshal preferences from models.User for store", "error", err, "userID", user.ID)
			// Proceed with nil preferences or return error, for now proceed
		}
	}

	typesUser := &types.User{
		ID:                user.ID.String(), // Store expects string ID, but CreateUser takes *types.User without ID usually
		Username:          user.Username,
		FirstName:         user.FirstName,
		LastName:          user.LastName,
		Email:             user.Email,
		ProfilePictureURL: user.ProfilePictureURL,
		RawUserMetaData:   user.RawUserMetaData,
		CreatedAt:         user.CreatedAt, // Timestamps might be set by DB or store
		UpdatedAt:         user.UpdatedAt,
		LastSeenAt:        user.LastSeenAt, // This should now be correct (*time.Time to *time.Time)
		IsOnline:          user.IsOnline,
		Preferences:       preferencesMap,
	}
	// The store's CreateUser might ignore the ID field in typesUser and generate its own.
	// It returns a string ID.

	createdUserIDStr, err := s.userStore.CreateUser(ctx, typesUser)
	if err != nil {
		log.Errorw("Failed to create user in store", "error", err)
		return uuid.Nil, err
	}

	createdUUID, parseErr := uuid.Parse(createdUserIDStr)
	if parseErr != nil {
		log.Errorw("Failed to parse created user ID string from store", "error", parseErr, "userIDStr", createdUserIDStr)
		// This is problematic as user is created but we can't return proper ID
		return uuid.Nil, fmt.Errorf("failed to parse created user ID from store: %w", parseErr)
	}

	log.Infow("User created successfully in store", "userID", createdUUID)

	// Sync user data to Supabase for RLS validation
	if s.supabaseService != nil && s.supabaseService.IsEnabled() {
		syncData := services.UserSyncData{
			ID:       user.ID.String(),
			Email:    user.Email,
			Username: user.Username,
		}

		// Sync asynchronously to avoid blocking user creation
		go func() {
			syncCtx := context.Background() // Use background context for async operation
			if err := s.supabaseService.SyncUser(syncCtx, syncData); err != nil {
				log.Errorw("Failed to sync user to Supabase", "error", err, "userID", createdUUID, "supabaseID", user.ID.String())
			} else {
				log.Infow("Successfully synced user to Supabase", "userID", createdUUID, "supabaseID", user.ID.String())
			}
		}()
	}

	return createdUUID, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, updates models.UserUpdateRequest) (*models.User, error) {
	log := logger.GetLogger()

	userIDStr := id.String()

	updatesMap := make(map[string]interface{})
	if updates.Username != nil {
		updatesMap["username"] = *updates.Username
	}
	if updates.FirstName != nil {
		updatesMap["firstName"] = *updates.FirstName
	}
	if updates.LastName != nil {
		updatesMap["lastName"] = *updates.LastName
	}
	if updates.ProfilePictureURL != nil {
		updatesMap["profilePictureURL"] = *updates.ProfilePictureURL
	}

	if len(updatesMap) == 0 {
		log.Infow("No changes to update for user, fetching current user", "userID", userIDStr)
		return s.GetUserByID(ctx, id)
	}

	updatesMap["updated_at"] = time.Now()

	updatedTypesUser, err := s.userStore.UpdateUser(ctx, userIDStr, updatesMap)
	if err != nil {
		log.Errorw("Failed to update user in store", "error", err, "userID", userIDStr)
		return nil, err
	}

	if updatedTypesUser == nil {
		return nil, errors.New("user store returned nil user after update without error")
	}

	// Convert *types.User back to *models.User
	var updatedPreferencesJSON []byte
	if updatedTypesUser.Preferences != nil {
		updatedPreferencesJSON, err = json.Marshal(updatedTypesUser.Preferences)
		if err != nil {
			log.Warnw("Failed to marshal preferences from updated types.User", "error", err, "userID", updatedTypesUser.ID)
		}
	}

	// The ID from updatedTypesUser is string, we need the original uuid.UUID 'id'
	updatedModelUser := &models.User{
		ID:                id, // Use the original uuid.UUID
		Username:          updatedTypesUser.Username,
		FirstName:         updatedTypesUser.FirstName,
		LastName:          updatedTypesUser.LastName,
		Email:             updatedTypesUser.Email,
		ProfilePictureURL: updatedTypesUser.ProfilePictureURL,
		RawUserMetaData:   updatedTypesUser.RawUserMetaData,
		CreatedAt:         updatedTypesUser.CreatedAt,
		UpdatedAt:         updatedTypesUser.UpdatedAt,
		LastSeenAt:        updatedTypesUser.LastSeenAt,
		IsOnline:          updatedTypesUser.IsOnline,
		Preferences:       updatedPreferencesJSON,
	}

	log.Infow("User updated successfully via store", "userID", id)

	// Sync user data to Supabase if email or username changed
	if s.supabaseService != nil && s.supabaseService.IsEnabled() {
		shouldSync := false
		if updates.Username != nil || updates.FirstName != nil || updates.LastName != nil {
			shouldSync = true
		}

		if shouldSync {
			syncData := services.UserSyncData{
				ID:       updatedModelUser.ID.String(),
				Email:    updatedModelUser.Email,
				Username: updatedModelUser.Username,
			}

			// Sync asynchronously to avoid blocking user update
			go func() {
				syncCtx := context.Background()
				if err := s.supabaseService.SyncUser(syncCtx, syncData); err != nil {
					log.Errorw("Failed to sync updated user to Supabase", "error", err, "userID", id, "supabaseID", updatedModelUser.ID.String())
				} else {
					log.Infow("Successfully synced updated user to Supabase", "userID", id, "supabaseID", updatedModelUser.ID.String())
				}
			}()
		}
	}

	return updatedModelUser, nil
}

// ValidateUserUpdateRequest validates a user update request
func (s *UserService) ValidateUserUpdateRequest(update models.UserUpdateRequest) error {
	var validationErrors []string

	if update.Username != nil && (strings.TrimSpace(*update.Username) == "" || len(*update.Username) > 30) {
		validationErrors = append(validationErrors, "Username must be between 1 and 30 characters")
	}

	if update.FirstName != nil && len(*update.FirstName) > 50 {
		validationErrors = append(validationErrors, "First name cannot exceed 50 characters")
	}

	if update.LastName != nil && len(*update.LastName) > 50 {
		validationErrors = append(validationErrors, "Last name cannot exceed 50 characters")
	}

	if len(validationErrors) > 0 {
		return errors.New("Validation failed: " + strings.Join(validationErrors, "; "))
	}

	return nil
}

// UpdateUserProfile handles profile updates with authorization and validation
func (s *UserService) UpdateUserProfile(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, updates models.UserUpdateRequest) (*models.User, error) {
	log := logger.GetLogger()

	// Authorization check - only allow users to update their own profile unless they're admin
	if !isAdmin && currentUserID != id {
		return nil, errors.New("Unauthorized: You can only update your own profile")
	}

	// Validate the update request
	if err := s.ValidateUserUpdateRequest(updates); err != nil {
		log.Errorw("Invalid user update request", "error", err, "userID", id)
		return nil, err
	}

	// Perform the update
	return s.UpdateUser(ctx, id, updates)
}

// UpdateUserPreferencesWithValidation validates and updates user preferences
func (s *UserService) UpdateUserPreferencesWithValidation(ctx context.Context, id uuid.UUID, currentUserID uuid.UUID, isAdmin bool, preferences map[string]interface{}) error {
	// Authorization check - only allow users to update their own preferences unless they're admin
	if !isAdmin && currentUserID != id {
		return errors.New("Unauthorized: You can only update your own preferences")
	}

	// Validate preferences (could add specific validation rules here if needed)
	if preferences == nil {
		return errors.New("Preferences cannot be null")
	}

	// Perform the update
	return s.UpdateUserPreferences(ctx, id, preferences)
}

// ListUsers retrieves a paginated list of users
// This service method returns []*models.User
func (s *UserService) ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int, error) {
	log := logger.GetLogger()

	// istore.UserStore.ListUsers returns []*types.User
	typesUsers, total, err := s.userStore.ListUsers(ctx, offset, limit)
	if err != nil {
		log.Errorw("Failed to list users from store", "error", err, "offset", offset, "limit", limit)
		return nil, 0, err
	}

	modelUsers := make([]*models.User, 0, len(typesUsers))
	for _, typesUser := range typesUsers {
		if typesUser == nil {
			continue
		} // Should not happen

		userID, parseErr := uuid.Parse(typesUser.ID)
		if parseErr != nil {
			log.Errorw("Failed to parse user ID in ListUsers from types.User", "error", parseErr, "userIDStr", typesUser.ID)
			// Skip this user or return error? For now, skip.
			continue
		}

		var preferencesJSON []byte
		if typesUser.Preferences != nil {
			prefJSON, marshalErr := json.Marshal(typesUser.Preferences)
			if marshalErr != nil {
				log.Warnw("Failed to marshal preferences for user in list", "error", marshalErr, "userID", typesUser.ID)
			} else {
				preferencesJSON = prefJSON
			}
		}

		modelUsers = append(modelUsers, &models.User{
			ID:                userID,
			Username:          typesUser.Username,
			FirstName:         typesUser.FirstName,
			LastName:          typesUser.LastName,
			Email:             typesUser.Email,
			ProfilePictureURL: typesUser.ProfilePictureURL,
			RawUserMetaData:   typesUser.RawUserMetaData,
			CreatedAt:         typesUser.CreatedAt,
			UpdatedAt:         typesUser.UpdatedAt,
			LastSeenAt:        typesUser.LastSeenAt,
			IsOnline:          typesUser.IsOnline,
			Preferences:       preferencesJSON,
		})
	}

	log.Infow("Listed users successfully from store and converted", "totalInStore", total, "convertedCount", len(modelUsers))
	// Note: total count comes from store, refers to total types.User. We return converted models.User.
	return modelUsers, total, nil
}

// SyncWithSupabase syncs a user with Supabase
// This function is expected to return *models.User
func (s *UserService) SyncWithSupabase(ctx context.Context, supabaseID string) (*models.User, error) {
	log := logger.GetLogger().With("supabaseID", supabaseID)
	log.Info("Attempting to sync user from Supabase")

	typesUser, err := s.userStore.SyncUserFromSupabase(ctx, supabaseID)
	if err != nil {
		log.Errorw("Failed to sync user from Supabase via store", "error", err)
		return nil, err
	}

	if typesUser == nil {
		log.Error("User store returned nil user from SyncUserFromSupabase without error")
		return nil, errors.New("failed to sync user from Supabase: store returned nil user")
	}

	// Convert *types.User to *models.User
	userID, parseErr := uuid.Parse(typesUser.ID)
	if parseErr != nil {
		log.Errorw("Failed to parse user ID from synced types.User", "error", parseErr, "userIDStr", typesUser.ID)
		return nil, fmt.Errorf("failed to parse synced user ID: %w", parseErr)
	}

	var preferencesJSON []byte
	if typesUser.Preferences != nil {
		preferencesJSON, err = json.Marshal(typesUser.Preferences)
		if err != nil {
			log.Warnw("Failed to marshal preferences from synced types.User", "error", err, "userID", typesUser.ID)
		}
	}

	modelUser := &models.User{
		ID:                userID,
		Username:          typesUser.Username,
		FirstName:         typesUser.FirstName,
		LastName:          typesUser.LastName,
		Email:             typesUser.Email,
		ProfilePictureURL: typesUser.ProfilePictureURL,
		RawUserMetaData:   typesUser.RawUserMetaData,
		CreatedAt:         typesUser.CreatedAt,
		UpdatedAt:         typesUser.UpdatedAt,
		LastSeenAt:        typesUser.LastSeenAt,
		IsOnline:          typesUser.IsOnline,
		Preferences:       preferencesJSON,
	}
	log.Infow("User synced successfully from Supabase and converted to model", "userID", modelUser.ID)
	return modelUser, nil
}

// GetUserProfile gets a user profile for API responses
func (s *UserService) GetUserProfile(ctx context.Context, id uuid.UUID) (*types.UserProfile, error) {
	log := logger.GetLogger()

	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		log.Errorw("Failed to get user profile", "error", err, "userID", id)
		return nil, err
	}

	profile := &types.UserProfile{
		ID:          user.ID.String(),
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		AvatarURL:   user.ProfilePictureURL,
		DisplayName: user.GetDisplayName(),
		IsOnline:    user.IsOnline,
	}

	if user.LastSeenAt != nil {
		profile.LastSeenAt = *user.LastSeenAt
	}

	return profile, nil
}

// GetUserProfiles gets multiple user profiles for API responses
func (s *UserService) GetUserProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*types.UserProfile, error) {
	log := logger.GetLogger()

	if len(ids) == 0 {
		return make(map[uuid.UUID]*types.UserProfile), nil
	}

	// Get profiles one by one (could be optimized with batch query)
	profiles := make(map[uuid.UUID]*types.UserProfile, len(ids))
	for _, id := range ids {
		profile, err := s.GetUserProfile(ctx, id)
		if err != nil {
			log.Warnw("Failed to get one user profile", "error", err, "userID", id)
			continue // Skip this user but continue with others
		}

		userID, err := uuid.Parse(profile.ID)
		if err != nil {
			log.Warnw("Failed to parse UUID from profile ID", "error", err, "profileID", profile.ID)
			continue // Skip this user
		}

		profiles[userID] = profile
	}

	return profiles, nil
}

// UpdateLastSeen updates a user's last seen timestamp
func (s *UserService) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	log := logger.GetLogger()

	timestamp := time.Now()
	updates := map[string]interface{}{
		"lastSeenAt": timestamp,
	}

	_, err := s.userStore.UpdateUser(ctx, id.String(), updates)
	if err != nil {
		log.Errorw("Failed to update last seen", "error", err, "userID", id)
		return err
	}

	log.Debugw("Updated user last seen timestamp", "userID", id, "timestamp", timestamp)
	return nil
}

// SetOnlineStatus sets a user's online status
func (s *UserService) SetOnlineStatus(ctx context.Context, id uuid.UUID, isOnline bool) error {
	log := logger.GetLogger()

	updates := map[string]interface{}{
		"isOnline": isOnline,
	}

	// If going offline, update last seen as well
	if !isOnline {
		updates["lastSeenAt"] = time.Now()
	}

	_, err := s.userStore.UpdateUser(ctx, id.String(), updates)
	if err != nil {
		log.Errorw("Failed to set online status", "error", err, "userID", id, "isOnline", isOnline)
		return err
	}

	log.Infow("Updated user online status", "userID", id, "isOnline", isOnline)
	return nil
}

// UpdateUserPreferences updates a user's preferences
func (s *UserService) UpdateUserPreferences(ctx context.Context, id uuid.UUID, preferences map[string]interface{}) error {
	log := logger.GetLogger()

	// First, get the current user to ensure they exist
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		log.Errorw("Failed to get user for preference update", "error", err, "userID", id)
		return err
	}

	// Merge the new preferences with existing ones
	currentPreferences := make(map[string]interface{})
	if len(user.Preferences) > 0 {
		if err := json.Unmarshal([]byte(user.Preferences), &currentPreferences); err != nil {
			log.Errorw("Failed to parse existing preferences", "error", err, "userID", id)
			// If we can't parse existing preferences, start fresh
			currentPreferences = make(map[string]interface{})
		}
	}

	// Update with new preferences
	for k, v := range preferences {
		if v == nil {
			// Remove preference if value is null
			delete(currentPreferences, k)
		} else {
			currentPreferences[k] = v
		}
	}

	// Convert back to JSON
	preferencesJSON, err := json.Marshal(currentPreferences)
	if err != nil {
		log.Errorw("Failed to marshal preferences", "error", err, "userID", id)
		return err
	}

	// Update in database
	updates := map[string]interface{}{
		"preferences": string(preferencesJSON),
	}

	_, err = s.userStore.UpdateUser(ctx, id.String(), updates)
	if err != nil {
		log.Errorw("Failed to update preferences", "error", err, "userID", id)
		return err
	}

	log.Infow("Updated user preferences", "userID", id)
	return nil
}

// Helper functions

// generateUsernameFromEmail generates a username from an email address
func generateUsernameFromEmail(email string) string {
	// Use part before @ symbol, and ensure it has no invalid chars
	parts := strings.Split(email, "@")
	if len(parts) < 1 {
		return fmt.Sprintf("user_%d", time.Now().UnixNano())
	}

	username := parts[0]
	// Append a timestamp if less than 4 characters
	if len(username) < 4 {
		username = fmt.Sprintf("%s_%d", username, time.Now().Unix()%10000)
	}
	return username
}

func (s *UserService) ValidateAndExtractClaims(tokenString string) (*types.JWTClaims, error) {
	// Prefer JWKS validator if available (supports new Supabase API keys)
	if s.jwtValidator != nil {
		return s.jwtValidator.ValidateAndGetClaims(tokenString)
	}
	// Fallback to legacy HS256 validation
	return auth.ValidateAccessToken(tokenString, s.jwtSecret)
}

func (s *UserService) OnboardUserFromJWTClaims(ctx context.Context, claims *types.JWTClaims) (*types.UserProfile, error) {
	if claims == nil || claims.UserID == "" || claims.Email == "" {
		return nil, errors.New("missing required user info in JWT claims")
	}

	username := strings.TrimSpace(claims.Username)
	if username == "" {
		return nil, errors.New("username is required and cannot be empty")
	}

	// Check if username is already taken by another user
	existingByUsername, err := s.userStore.GetUserByUsername(ctx, username)
	if err == nil && existingByUsername != nil && existingByUsername.ID != claims.UserID {
		return nil, errors.New("username is already taken")
	}

	// Try to get user by SupabaseID (UserID in claims)
	typesUser, err := s.userStore.GetUserBySupabaseID(ctx, claims.UserID)
	if err != nil {
		// Check if it's a "not found" error - this is expected for new users
		if appErr, ok := err.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
			// User doesn't exist yet, continue to creation flow
			typesUser = nil
		} else {
			return nil, err
		}
	}

	if typesUser == nil {
		// User does not exist, create
		idUUID, _ := uuid.Parse(claims.UserID)
		user := &models.User{
			ID:       idUUID,
			Email:    claims.Email,
			Username: username,
		}
		id, err := s.CreateUser(ctx, user)
		if err != nil {
			return nil, err
		}
		// CreateUser already handles sync, so just return the profile
		return s.GetUserProfile(ctx, id)
	}

	// User exists, update if needed
	updates := make(map[string]interface{})
	if typesUser.Email != claims.Email {
		updates["email"] = claims.Email
	}
	if typesUser.Username != username {
		updates["username"] = username
	}
	if len(updates) > 0 {
		_, err := s.userStore.UpdateUser(ctx, typesUser.ID, updates)
		if err != nil {
			return nil, err
		}

		// Sync updated user data to Supabase for RLS validation
		if s.supabaseService != nil && s.supabaseService.IsEnabled() {
			syncData := services.UserSyncData{
				ID:       claims.UserID,
				Email:    claims.Email,
				Username: username,
			}

			// Sync asynchronously to avoid blocking onboarding
			go func() {
				syncCtx := context.Background()
				if err := s.supabaseService.SyncUser(syncCtx, syncData); err != nil {
					log := logger.GetLogger()
					log.Errorw("Failed to sync onboarded user to Supabase", "error", err, "supabaseID", claims.UserID)
				} else {
					log := logger.GetLogger()
					log.Infow("Successfully synced onboarded user to Supabase", "supabaseID", claims.UserID)
				}
			}()
		}
	}
	return s.GetUserProfile(ctx, uuid.MustParse(typesUser.ID))
}

// SearchUsers searches for users by query across username, email, contact_email, first_name, last_name
func (s *UserService) SearchUsers(ctx context.Context, query string, limit int) ([]*types.UserSearchResult, error) {
	log := logger.GetLogger()

	if len(query) < 2 {
		return nil, apperrors.ValidationFailed("search query must be at least 2 characters", "")
	}

	if limit <= 0 || limit > 20 {
		limit = 10 // Default limit
	}

	results, err := s.userStore.SearchUsers(ctx, query, limit)
	if err != nil {
		log.Errorw("Failed to search users", "query", query, "error", err)
		return nil, fmt.Errorf("error searching users: %w", err)
	}

	log.Infow("Successfully searched users", "query", query, "count", len(results))
	return results, nil
}

// UpdateContactEmail updates the user's contact email
func (s *UserService) UpdateContactEmail(ctx context.Context, userID uuid.UUID, email string) error {
	log := logger.GetLogger()

	// Validate email format
	if email == "" {
		return apperrors.ValidationFailed("email is required", "")
	}

	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return apperrors.ValidationFailed("invalid email format", "")
	}

	err := s.userStore.UpdateContactEmail(ctx, userID.String(), email)
	if err != nil {
		log.Errorw("Failed to update contact email", "userID", userID, "error", err)
		return fmt.Errorf("error updating contact email: %w", err)
	}

	log.Infow("Successfully updated contact email", "userID", userID, "email", maskEmail(email))
	return nil
}
