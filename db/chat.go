// Package db provides implementations for data access interfaces defined in internal/store.
// It interacts with the PostgreSQL database using pgxpool and potentially other external services like Supabase.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

// GetChatGroup retrieves a chat group by ID from the database.
func (s *PostgresChatStore) GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error) {
	log := logger.GetLogger()

	query := `
		SELECT id, trip_id, name, description, created_by, created_at, updated_at, deleted_at
		FROM chat_groups
		WHERE id = $1 AND deleted_at IS NULL
	`

	var group types.ChatGroup
	var deletedAt sql.NullTime // Use sql.NullTime for nullable deleted_at field

	err := s.db.QueryRow(ctx, query, groupID).Scan(
		&group.ID,
		&group.TripID,
		&group.Name,
		&group.Description,
		&group.CreatedBy,
		&group.CreatedAt,
		&group.UpdatedAt,
		&deletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Infow("Chat group not found", "groupID", groupID)
			return nil, apperrors.NotFound("ChatGroup", groupID)
		}
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		return nil, apperrors.NewDatabaseError(err)
	}

	// Convert sql.NullTime to *time.Time for DeletedAt
	if deletedAt.Valid {
		group.DeletedAt = &deletedAt.Time
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

// DeleteChatGroup performs a soft delete on a chat group by setting deleted_at timestamp.
func (s *PostgresChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	log := logger.GetLogger()

	query := `
		UPDATE chat_groups
		SET deleted_at = $1,
		    updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	now := time.Now()
	commandTag, err := s.db.Exec(ctx, query, now, groupID)

	if err != nil {
		log.Errorw("Failed to delete chat group", "error", err, "groupID", groupID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if it exists at all or is already deleted
		existsQuery := `SELECT 1 FROM chat_groups WHERE id = $1`
		var exists bool
		if err := s.db.QueryRow(ctx, existsQuery, groupID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("ChatGroup", groupID)
			}
			return apperrors.NewDatabaseError(err)
		}

		// Group exists but is already soft-deleted
		log.Warnw("Soft delete chat group affected 0 rows, group likely already deleted", "groupID", groupID)
		return nil
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
		offset = 0
	}

	response := &types.ChatGroupPaginatedResponse{
		Groups: make([]types.ChatGroup, 0, limit),
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
		SELECT id, trip_id, name, description, created_by, created_at, updated_at, deleted_at
		FROM chat_groups
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(ctx, query, tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to query chat groups", "error", err, "tripID", tripID)
		return response, apperrors.NewDatabaseError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var group types.ChatGroup
		var deletedAt sql.NullTime // Use sql.NullTime for nullable deleted_at field

		err := rows.Scan(
			&group.ID,
			&group.TripID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
			&deletedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan chat group row", "error", err)
			return response, apperrors.Wrap(err, apperrors.DatabaseError, "failed to scan chat group row")
		}

		// Convert sql.NullTime to *time.Time for DeletedAt
		if deletedAt.Valid {
			group.DeletedAt = &deletedAt.Time
		}

		response.Groups = append(response.Groups, group)
	}
	if err = rows.Err(); err != nil {
		log.Errorw("Error after iterating chat group rows", "error", err)
		return response, apperrors.Wrap(err, apperrors.DatabaseError, "error iterating chat group rows")
	}

	countQuery := `
		SELECT COUNT(*)
		FROM chat_groups
		WHERE trip_id = $1 AND deleted_at IS NULL
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

	// Updated query to match actual database schema
	query := `
		INSERT INTO chat_messages (id, group_id, user_id, content, content_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	now := time.Now()
	message.CreatedAt = now
	message.UpdatedAt = now

	var id string
	err := s.db.QueryRow(ctx, query,
		message.ID,
		message.GroupID,
		message.UserID,
		message.Content,
		message.ContentType,
		message.CreatedAt,
		message.UpdatedAt,
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

	// Updated query to match actual database schema
	query := `
		SELECT id, group_id, user_id, content, content_type, created_at, updated_at
		FROM chat_messages
		WHERE id = $1 AND deleted_at IS NULL
	`

	var msg types.ChatMessage
	err := s.db.QueryRow(ctx, query, messageID).Scan(
		&msg.ID,
		&msg.GroupID,
		&msg.UserID,
		&msg.Content,
		&msg.ContentType,
		&msg.CreatedAt,
		&msg.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warnw("Chat message not found", "messageID", messageID)
			return nil, apperrors.NotFound("ChatMessage", messageID)
		}
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return nil, apperrors.NewDatabaseError(err)
	}

	// Fetch reactions separately
	reactions, reactErr := s.ListChatMessageReactions(ctx, messageID)
	if reactErr != nil {
		log.Errorw("Failed to fetch reactions for get message", "error", reactErr, "messageID", messageID)
		msg.Reactions = []types.ChatMessageReaction{} // Ensure Reactions field exists and is empty on error
	} else {
		msg.Reactions = reactions
	}

	return &msg, nil
}

// UpdateChatMessage updates the content of an existing chat message.
func (s *PostgresChatStore) UpdateChatMessage(ctx context.Context, messageID string, content string) error {
	log := logger.GetLogger()

	// Updated query to match actual database schema
	query := `
		UPDATE chat_messages
		SET content = $1,
			updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	now := time.Now()

	commandTag, err := s.db.Exec(ctx, query, content, now, messageID)

	if err != nil {
		log.Errorw("Failed to update chat message", "error", err, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if message exists but is soft-deleted or doesn't exist at all
		existsQuery := `SELECT 1 FROM chat_messages WHERE id = $1`
		var exists bool
		if err := s.db.QueryRow(ctx, existsQuery, messageID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("ChatMessage", messageID)
			}
			return apperrors.NewDatabaseError(err)
		}

		// Message exists but is soft-deleted
		log.Warnw("Update chat message affected 0 rows, message likely deleted", "messageID", messageID)
		return apperrors.NotFound("ChatMessage (not updateable)", messageID)
	}

	return nil
}

// DeleteChatMessage performs a soft delete on a chat message by setting deleted_at timestamp.
func (s *PostgresChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	log := logger.GetLogger()

	// Updated query to use deleted_at timestamp for soft delete
	query := `
		UPDATE chat_messages
		SET deleted_at = $1,
			updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`
	now := time.Now()
	commandTag, err := s.db.Exec(ctx, query, now, messageID)

	if err != nil {
		log.Errorw("Failed to soft delete chat message", "error", err, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if it exists but was already deleted or never existed
		existsQuery := `SELECT 1 FROM chat_messages WHERE id = $1`
		var exists bool
		if err := s.db.QueryRow(ctx, existsQuery, messageID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("ChatMessage", messageID)
			}
			return apperrors.NewDatabaseError(err)
		}

		// Message exists but is already soft-deleted
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

	// Update query to filter by deleted_at IS NULL instead of is_deleted = false
	query := `
		SELECT id, group_id, user_id, content, content_type, created_at, updated_at
		FROM chat_messages
		WHERE group_id = $1 AND deleted_at IS NULL
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
		err = rows.Scan(
			&msg.ID,
			&msg.GroupID,
			&msg.UserID,
			&msg.Content,
			&msg.ContentType,
			&msg.CreatedAt,
			&msg.UpdatedAt,
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
	countQuery := `
		SELECT COUNT(*)
		FROM chat_messages
		WHERE group_id = $1 AND deleted_at IS NULL
	`
	var total int
	err = s.db.QueryRow(ctx, countQuery, groupID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to get total chat messages count", "error", err, "groupID", groupID)
		return nil, 0, apperrors.NewDatabaseError(err)
	}

	return messages, total, nil
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
// It updates the last_read_message_id column in the chat_group_members table.
func (s *PostgresChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	query := `
        UPDATE chat_group_members 
        SET last_read_message_id = $3
        WHERE group_id = $1 AND user_id = $2
    `
	commandTag, err := s.db.Exec(ctx, query, groupID, userID, messageID)
	if err != nil {
		logger.GetLogger().Errorw("Failed to update last read message", "error", err, "groupID", groupID, "userID", userID, "messageID", messageID)
		return apperrors.NewDatabaseError(err)
	}

	// If no row was updated, user is not in the group yet, so insert a membership record
	if commandTag.RowsAffected() == 0 {
		insertQuery := `
            INSERT INTO chat_group_members (group_id, user_id, last_read_message_id, joined_at)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (group_id, user_id) DO UPDATE 
            SET last_read_message_id = EXCLUDED.last_read_message_id
        `
		_, err = s.db.Exec(ctx, insertQuery, groupID, userID, messageID, time.Now())
		if err != nil {
			logger.GetLogger().Errorw("Failed to insert chat group member with last read", "error", err, "groupID", groupID, "userID", userID, "messageID", messageID)
			return apperrors.NewDatabaseError(err)
		}
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

	// Updated query to match database schema
	query := `
		SELECT message_id, user_id, reaction, created_at
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
		err := rows.Scan(&r.MessageID, &r.UserID, &r.Reaction, &r.CreatedAt)
		if err != nil {
			log.Errorw("Failed to scan reaction", "error", err)
			return nil, apperrors.Wrap(err, apperrors.DatabaseError, "failed to scan reaction row")
		}
		reactions = append(reactions, r)
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

	req.Header.Set("Apikey", s.supabaseAPIKey)
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
