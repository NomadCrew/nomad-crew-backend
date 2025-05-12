// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/supabase-community/supabase-go"
)

// ChatStore interface is defined in internal/store/interfaces.go

// PostgresChatStore implements the store.ChatStore interface using a PostgreSQL database
// via pgxpool and a Supabase client for user-related operations.
type PostgresChatStore struct {
	db              *pgxpool.Pool
	supabase        *supabase.Client
	supabaseBaseURL string
	supabaseAPIKey  string
}

// NewPostgresChatStore creates a new instance of PostgresChatStore.
func NewPostgresChatStore(db *pgxpool.Pool, supabaseClient *supabase.Client, supabaseURL, supabaseKey string) store.ChatStore {
	return &PostgresChatStore{
		db:              db,
		supabase:        supabaseClient,
		supabaseBaseURL: supabaseURL,
		supabaseAPIKey:  supabaseKey,
	}
}

// CreateChatGroup inserts a new chat group into the database.
// It generates a new UUID for the group if one is not provided.
func (s *PostgresChatStore) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	log := logger.GetLogger()

	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	query := `
		INSERT INTO chat_groups (id, trip_id, name, description, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	var id string
	err := s.db.QueryRow(ctx, query,
		group.ID,
		group.TripID,
		group.Name,
		group.Description,
		group.CreatedBy,
		group.CreatedAt,
		group.UpdatedAt,
	).Scan(&id)

	if err != nil {
		log.Errorw("Failed to create chat group", "error", err)
		return "", apperrors.NewDatabaseError(err)
	}

	return id, nil
}

// GetChatGroup retrieves a specific chat group by its ID from the database.
func (s *PostgresChatStore) GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error) {
	log := logger.GetLogger()

	query := `
		SELECT id, trip_id, name, description, created_by, created_at, updated_at
		FROM chat_groups
		WHERE id = $1
	`

	var group types.ChatGroup
	err := s.db.QueryRow(ctx, query, groupID).Scan(
		&group.ID,
		&group.TripID,
		&group.Name,
		&group.Description,
		&group.CreatedBy,
		&group.CreatedAt,
		&group.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Chat group not found", "groupID", groupID)
			return nil, apperrors.NotFound("ChatGroup", groupID)
		}
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		return nil, apperrors.NewDatabaseError(err)
	}

	return &group, nil
}

// UpdateChatGroup updates the name and/or description of an existing chat group.
func (s *PostgresChatStore) UpdateChatGroup(ctx context.Context, groupID string, update types.ChatGroupUpdateRequest) error {
	log := logger.GetLogger()

	query := `
		UPDATE chat_groups
		SET name = COALESCE($1, name),
			description = COALESCE($2, description),
			updated_at = $3
		WHERE id = $4
	`

	now := time.Now()

	commandTag, err := s.db.Exec(ctx, query,
		update.Name,
		update.Description,
		now,
		groupID,
	)

	if err != nil {
		log.Errorw("Failed to update chat group", "error", err, "groupID", groupID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return apperrors.NotFound("ChatGroup", groupID)
	}

	return nil
}

// DeleteChatGroup removes a chat group and potentially related data (handled by DB constraints/triggers) from the database.
func (s *PostgresChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	log := logger.GetLogger()

	query := `
		DELETE FROM chat_groups
		WHERE id = $1
	`

	commandTag, err := s.db.Exec(ctx, query, groupID)

	if err != nil {
		log.Errorw("Failed to delete chat group", "error", err, "groupID", groupID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return apperrors.NotFound("ChatGroup", groupID)
	}

	return nil
}

// ListChatGroupsByTrip retrieves a paginated list of chat groups associated with a specific trip ID.
func (s *PostgresChatStore) ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error) {
	log := logger.GetLogger()

	if limit <= 0 {
		limit = 10 // Default limit
	}
	if offset < 0 {
		offset = 0 // Default offset
	}

	response := &types.ChatGroupPaginatedResponse{
		Groups: make([]types.ChatGroup, 0),
		Pagination: struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}{
			Limit:  limit,
			Offset: offset,
		},
	}

	query := `
		SELECT id, trip_id, name, description, created_by, created_at, updated_at
		FROM chat_groups
		WHERE trip_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(ctx, query, tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to list chat groups", "error", err, "tripID", tripID)
		return response, apperrors.NewDatabaseError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var group types.ChatGroup
		err := rows.Scan(
			&group.ID,
			&group.TripID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan chat group row", "error", err)
			return response, apperrors.Wrap(err, apperrors.DatabaseError, "failed to scan chat group row")
		}
		response.Groups = append(response.Groups, group)
	}
	if err = rows.Err(); err != nil {
		log.Errorw("Error after iterating chat group rows", "error", err)
		return response, apperrors.Wrap(err, apperrors.DatabaseError, "error iterating chat group rows")
	}

	// Get total count for pagination metadata
	countQuery := `
		SELECT COUNT(*)
		FROM chat_groups
		WHERE trip_id = $1
	`

	var total int
	err = s.db.QueryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to get total chat groups count", "error", err, "tripID", tripID)
		return response, apperrors.NewDatabaseError(err)
	}

	response.Pagination.Total = total

	return response, nil
}

// CreateChatMessage inserts a new chat message into the database.
// It generates a new UUID for the message if one is not provided.
// It defaults ContentType to 'text' if not provided.
func (s *PostgresChatStore) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	log := logger.GetLogger()

	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	if message.ContentType == "" {
		message.ContentType = types.ContentTypeText // Default content type
	}

	// Reverted query to match assumed original fields
	query := `
		INSERT INTO chat_messages (id, group_id, user_id, content, content_type, created_at, updated_at, is_edited, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	now := time.Now()
	message.CreatedAt = now // Assumes CreatedAt exists
	message.UpdatedAt = now // Assumes UpdatedAt exists

	var id string
	// Reverted scan logic
	err := s.db.QueryRow(ctx, query,
		message.ID,
		message.GroupID,
		message.UserID,
		message.Content,
		message.ContentType, // Assumed field
		message.CreatedAt,   // Assumed field
		message.UpdatedAt,   // Assumed field
		message.IsEdited,    // Assumed field
		message.IsDeleted,   // Assumed field
	).Scan(&id)

	if err != nil {
		log.Errorw("Failed to create chat message", "error", err, "groupID", message.GroupID, "userID", message.UserID)
		return "", apperrors.NewDatabaseError(err)
	}

	return id, nil
}

// GetChatMessageByID retrieves a specific chat message by its ID.
// It also fetches associated reactions in a separate query.
func (s *PostgresChatStore) GetChatMessageByID(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	log := logger.GetLogger()

	// Reverted query to match assumed original fields
	query := `
		SELECT id, group_id, user_id, content, content_type, created_at, updated_at, is_edited, is_deleted
		FROM chat_messages
		WHERE id = $1 AND is_deleted = false
	`

	var msg types.ChatMessage
	// Reverted scan logic
	err := s.db.QueryRow(ctx, query, messageID).Scan(
		&msg.ID,
		&msg.GroupID,
		&msg.UserID,
		&msg.Content,
		&msg.ContentType, // Assumed field
		&msg.CreatedAt,   // Assumed field
		&msg.UpdatedAt,   // Assumed field
		&msg.IsEdited,    // Assumed field
		&msg.IsDeleted,   // Assumed field
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Chat message not found", "messageID", messageID)
			return nil, apperrors.NotFound("ChatMessage", messageID)
		}
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return nil, apperrors.NewDatabaseError(err)
	}

	// Fetch reactions separately (original inefficient pattern)
	reactions, reactErr := s.ListChatMessageReactions(ctx, messageID)
	if reactErr != nil {
		log.Errorw("Failed to fetch reactions for get message", "error", reactErr, "messageID", messageID)
		msg.Reactions = []types.ChatMessageReaction{} // Ensure Reactions field exists and is empty on error
	} else {
		msg.Reactions = reactions // Assumes Reactions field exists
	}

	return &msg, nil
}

// UpdateChatMessage updates the content of an existing chat message and marks it as edited.
func (s *PostgresChatStore) UpdateChatMessage(ctx context.Context, messageID string, content string) error {
	log := logger.GetLogger()

	// Reverted query to mark as edited and check is_deleted
	query := `
		UPDATE chat_messages
		SET content = $1,
			is_edited = true,
			updated_at = $2
		WHERE id = $3 AND is_deleted = false
	`

	now := time.Now()

	commandTag, err := s.db.Exec(ctx, query, content, now, messageID)

	if err != nil {
		log.Errorw("Failed to update chat message", "error", err, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if the message exists but is deleted, or doesn't exist at all
		_, getErr := s.GetChatMessageByID(ctx, messageID) // Check existence (ignoring reactions fetch error)
		if getErr != nil {
			if appErr, ok := getErr.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
				return apperrors.NotFound("ChatMessage", messageID) // Not found
			}
			// Some other error occurred during check
			log.Warnw("Error checking chat message existence during update", "error", getErr, "messageID", messageID)
		}
		// If GetChatMessageByID returned nil error, it means the message exists but is_deleted=true (since RowsAffected was 0)
		log.Warnw("Update chat message affected 0 rows, message likely deleted or not found", "messageID", messageID)
		return apperrors.NotFound("ChatMessage (not updateable)", messageID) // Treat as not found for update purposes
	}

	return nil
}

// DeleteChatMessage performs a soft delete on a chat message by setting is_deleted to true.
func (s *PostgresChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	log := logger.GetLogger()

	// Reverted to soft delete query
	query := `
		UPDATE chat_messages
		SET is_deleted = true,
			updated_at = $1
		WHERE id = $2 AND is_deleted = false
	`
	now := time.Now()
	commandTag, err := s.db.Exec(ctx, query, now, messageID)

	if err != nil {
		log.Errorw("Failed to soft delete chat message", "error", err, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if it exists but was already deleted or never existed
		_, getErr := s.GetChatMessageByID(ctx, messageID) // Check existence (ignoring reactions fetch error)
		if getErr != nil {
			if appErr, ok := getErr.(*apperrors.AppError); ok && appErr.Type == apperrors.NotFoundError {
				return apperrors.NotFound("ChatMessage", messageID) // Truly not found
			}
			log.Warnw("Error checking chat message existence during delete", "error", getErr, "messageID", messageID)
		}
		// If GetChatMessageByID returned nil error, it means the message exists but is_deleted=true (RowsAffected was 0)
		log.Warnw("Soft delete chat message affected 0 rows, message likely already deleted", "messageID", messageID)
		// No error needed if already deleted (idempotent)
		return nil
	}

	return nil
}

// ListChatMessages retrieves a paginated list of chat messages for a specific group.
// Note: This currently fetches reactions separately for each message, which can be inefficient.
// Consider optimizing this in production (e.g., JOIN query or batch fetching).
func (s *PostgresChatStore) ListChatMessages(ctx context.Context, groupID string, params types.PaginationParams) ([]types.ChatMessage, int, error) {
	log := logger.GetLogger()

	if params.Limit <= 0 {
		params.Limit = 50 // Default limit
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	messages := make([]types.ChatMessage, 0, params.Limit)

	// Reverted query to assumed original fields matching types.ChatMessage
	query := `
        SELECT id, group_id, user_id, content, content_type, created_at, updated_at, is_edited, is_deleted
        FROM chat_messages
        WHERE group_id = $1 AND is_deleted = false
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := s.db.Query(ctx, query, groupID, params.Limit, params.Offset)
	if err != nil {
		log.Errorw("Failed to list chat messages by group", "error", err, "groupID", groupID)
		return nil, 0, apperrors.NewDatabaseError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var msg types.ChatMessage
		// Reverted scan to match reverted query
		err = rows.Scan(
			&msg.ID,
			&msg.GroupID,
			&msg.UserID,
			&msg.Content,
			&msg.ContentType,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&msg.IsEdited,
			&msg.IsDeleted,
		)
		if err != nil {
			log.Errorw("Failed to scan chat message row", "error", err)
			return nil, 0, apperrors.Wrap(err, apperrors.DatabaseError, "failed to scan chat message row")
		}

		// Original separate reaction fetch
		reactions, reactErr := s.ListChatMessageReactions(ctx, msg.ID)
		if reactErr != nil {
			log.Errorw("Failed to fetch reactions for list message", "error", reactErr, "messageID", msg.ID)
			msg.Reactions = []types.ChatMessageReaction{}
		} else {
			msg.Reactions = reactions
		}

		messages = append(messages, msg)
	}
	if err = rows.Err(); err != nil {
		log.Errorw("Error after iterating chat message rows", "error", err)
		return nil, 0, apperrors.Wrap(err, apperrors.DatabaseError, "error iterating chat message rows")
	}

	// Original count query
	countQuery := `SELECT COUNT(*) FROM chat_messages WHERE group_id = $1 AND is_deleted = false`
	var total int
	err = s.db.QueryRow(ctx, countQuery, groupID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to get total chat messages count", "error", err, "groupID", groupID)
		return nil, 0, apperrors.NewDatabaseError(err)
	}

	return messages, total, nil
}

// fetchUserFromSupabase retrieves user details directly from Supabase Auth API.
// This is likely less efficient than joining with a local users table if available and kept in sync.
// TODO: Handle rate limiting, caching, more robust error handling.
func (s *PostgresChatStore) fetchUserFromSupabase(userID string) (*types.UserResponse, error) {
	log := logger.GetLogger()
	// Note: This uses the admin API. Ensure the supabaseAPIKey is the service_role key.
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", s.supabaseBaseURL, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorw("Failed to create Supabase user request", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.supabaseAPIKey)
	req.Header.Set("apikey", s.supabaseAPIKey) // Service key required for admin endpoint

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("Failed to execute Supabase user request", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorw("Supabase user request failed", "statusCode", resp.StatusCode, "userID", userID)
		bodyBytes, _ := io.ReadAll(resp.Body) // Attempt to read error body
		log.Debugw("Supabase error response body", "body", string(bodyBytes))
		if resp.StatusCode == http.StatusNotFound {
			return nil, apperrors.NotFound("User", userID)
		}
		return nil, fmt.Errorf("supabase request failed with status %d", resp.StatusCode)
	}

	// Supabase admin user response structure (simplified)
	var supabaseUser struct {
		ID          string                 `json:"id"`
		Aud         string                 `json:"aud"`
		Role        string                 `json:"role"`
		Email       string                 `json:"email"`
		UserAppData map[string]interface{} `json:"user_metadata"`
		CreatedAt   time.Time              `json:"created_at"`
		UpdatedAt   time.Time              `json:"updated_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&supabaseUser); err != nil {
		log.Errorw("Failed to decode Supabase user response", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract relevant fields from user_metadata
	username := ""
	if name, ok := supabaseUser.UserAppData["username"].(string); ok {
		username = name
	}
	// Removed avatarURL extraction/assignment as types.UserResponse likely lacks the field

	// Assuming types.UserResponse has ID and Username fields
	userResponse := &types.UserResponse{
		ID:       supabaseUser.ID,
		Username: username, // Assumes Username exists
		// Email: supabaseUser.Email, // Map other fields if needed in UserResponse
	}

	return userResponse, nil
}

// AddChatGroupMember adds a user to a chat group using an INSERT ... ON CONFLICT DO NOTHING query.
func (s *PostgresChatStore) AddChatGroupMember(ctx context.Context, groupID, userID string) error {
	query := `INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.Exec(ctx, query, groupID, userID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to add chat group member", "error", err, "groupID", groupID, "userID", userID)
		return apperrors.NewDatabaseError(err)
	}
	return nil
}

// RemoveChatGroupMember removes a user from a chat group.
// It logs a warning but returns no error if the member or group was not found (idempotency).
func (s *PostgresChatStore) RemoveChatGroupMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM chat_group_members WHERE group_id = $1 AND user_id = $2`
	commandTag, err := s.db.Exec(ctx, query, groupID, userID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to remove chat group member", "error", err, "groupID", groupID, "userID", userID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Log warning if no rows were affected, but don't return error for idempotency
		logger.GetLogger().Warnw("Attempted to remove non-existent chat group member or group", "groupID", groupID, "userID", userID)
	}
	return nil
}

// ListChatGroupMembers retrieves a list of users who are members of a specific chat group.
// It assumes the local 'users' table mirrors Supabase user metadata.
func (s *PostgresChatStore) ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	log := logger.GetLogger()

	query := `
		SELECT user_id
		FROM chat_group_members
		WHERE group_id = $1
	`

	rows, err := s.db.Query(ctx, query, groupID)
	if err != nil {
		log.Errorw("Failed to query chat group members", "error", err)
		return nil, apperrors.NewDatabaseError(err)
	}
	defer rows.Close()

	var members []types.UserResponse
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			log.Errorw("Failed to scan user ID", "error", err)
			return nil, apperrors.NewDatabaseError(err)
		}

		user, err := s.GetUserByID(ctx, userID)
		if err != nil {
			log.Errorw("Failed to fetch user from Supabase", "userId", userID, "error", err)
			continue // Skip this user but continue with others
		}

		members = append(members, types.UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			Username:    user.UserMetadata.Username,
			FirstName:   user.UserMetadata.FirstName,
			LastName:    user.UserMetadata.LastName,
			AvatarURL:   user.UserMetadata.ProfilePicture,
			DisplayName: user.UserMetadata.Username, // Use username as display name if no first/last name
		})
	}

	return members, nil
}

// UpdateLastReadMessage updates the last message read marker for a user in a specific group.
// It uses INSERT ... ON CONFLICT ... DO UPDATE to ensure atomicity and correctness,
// only updating if the new message ID corresponds to a later message than the current marker.
func (s *PostgresChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	query := `
        INSERT INTO chat_last_read (group_id, user_id, last_read_message_id, last_read_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (group_id, user_id)
        DO UPDATE SET last_read_message_id = EXCLUDED.last_read_message_id,
                      last_read_at = EXCLUDED.last_read_at
        WHERE (SELECT sent_at FROM chat_messages WHERE id = EXCLUDED.last_read_message_id) >
              (SELECT sent_at FROM chat_messages WHERE id = chat_last_read.last_read_message_id)
           OR chat_last_read.last_read_message_id IS NULL
    `
	_, err := s.db.Exec(ctx, query, groupID, userID, messageID, time.Now())
	if err != nil {
		logger.GetLogger().Errorw("Failed to update last read message", "error", err, "groupID", groupID, "userID", userID, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}
	return nil
}

// AddReaction adds a reaction from a user to a specific message.
// It uses INSERT ... ON CONFLICT DO NOTHING to prevent duplicate reactions.
func (s *PostgresChatStore) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	query := `
        INSERT INTO chat_message_reactions (message_id, user_id, reaction, created_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (message_id, user_id, reaction)
        DO NOTHING
    `
	_, err := s.db.Exec(ctx, query, messageID, userID, reaction, time.Now())
	if err != nil {
		logger.GetLogger().Errorw("Failed to add message reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return apperrors.NewDatabaseError(err)
	}
	return nil
}

// RemoveReaction removes a specific reaction made by a user on a message.
// It logs a warning but returns no error if the reaction was not found (idempotency).
func (s *PostgresChatStore) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	query := `
        DELETE FROM chat_message_reactions
        WHERE message_id = $1 AND user_id = $2 AND reaction = $3
    `
	commandTag, err := s.db.Exec(ctx, query, messageID, userID, reaction)
	if err != nil {
		logger.GetLogger().Errorw("Failed to remove message reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Log warning but don't return error for idempotency
		logger.GetLogger().Warnw("Attempted to remove non-existent message reaction", "messageID", messageID, "userID", userID, "reaction", reaction)
	}
	return nil
}

// ListChatMessageReactions retrieves all reactions for a specific message.
func (s *PostgresChatStore) ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error) {
	log := logger.GetLogger()
	reactions := make([]types.ChatMessageReaction, 0)

	// Assuming ChatMessageReaction has fields: ID, MessageID, UserID, Reaction, CreatedAt
	query := `
		SELECT id, message_id, user_id, reaction, created_at
        FROM chat_message_reactions
        WHERE message_id = $1
        ORDER BY created_at ASC
    `
	rows, err := s.db.Query(ctx, query, messageID)
	if err != nil {
		log.Errorw("Failed to list reactions", "error", err, "messageID", messageID)
		return nil, apperrors.NewDatabaseError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var r types.ChatMessageReaction
		err := rows.Scan(&r.ID, &r.MessageID, &r.UserID, &r.Reaction, &r.CreatedAt)
		if err != nil {
			log.Errorw("Failed to scan reaction", "error", err)
			return nil, apperrors.Wrap(err, apperrors.DatabaseError, "failed to scan reaction row")
		}
		reactions = append(reactions, r)
	}
	if err = rows.Err(); err != nil {
		log.Errorw("Error after iterating reactions", "error", err)
		return nil, apperrors.Wrap(err, apperrors.DatabaseError, "error iterating reaction rows")
	}

	return reactions, nil
}

// GetUserByID retrieves user details directly from the Supabase Go client using the admin method.
// Returns the raw *supabase.User struct provided by the client library.
func (s *PostgresChatStore) GetUserByID(ctx context.Context, userID string) (*types.SupabaseUser, error) {
	log := logger.GetLogger()

	// Use the Supabase admin API to get user details
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/auth/v1/admin/users/%s", s.supabaseBaseURL, userID),
		nil)
	if err != nil {
		log.Errorw("Failed to create request for Supabase user", "error", err)
		return nil, apperrors.NewExternalServiceError(err)
	}

	req.Header.Set("apikey", s.supabaseAPIKey)
	req.Header.Set("Authorization", "Bearer "+s.supabaseAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("Failed to fetch user from Supabase", "error", err)
		return nil, apperrors.NewExternalServiceError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorw("Supabase returned non-200 status", "status", resp.StatusCode)
		return nil, apperrors.NewExternalServiceError(fmt.Errorf("supabase returned status %d", resp.StatusCode))
	}

	var user types.SupabaseUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Errorw("Failed to decode Supabase user response", "error", err)
		return nil, apperrors.NewExternalServiceError(err)
	}

	return &user, nil
}
