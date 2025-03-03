package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/supabase-community/supabase-go"
)

// ChatStore interface is now defined in internal/store/interfaces.go

// PostgresChatStore represents a PostgreSQL-backed chat store
type PostgresChatStore struct {
	db              *pgxpool.Pool
	supabase        *supabase.Client
	supabaseBaseURL string
	supabaseAPIKey  string
}

// NewPostgresChatStore creates a new PostgreSQL-backed chat store
func NewPostgresChatStore(db *pgxpool.Pool, supabaseClient *supabase.Client, supabaseURL, supabaseKey string) store.ChatStore {
	return &PostgresChatStore{
		db:              db,
		supabase:        supabaseClient,
		supabaseBaseURL: supabaseURL,
		supabaseAPIKey:  supabaseKey,
	}
}

// CreateChatGroup creates a new chat group
func (s *PostgresChatStore) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	log := logger.GetLogger()

	// Generate a new UUID if not provided
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
		return "", err
	}

	return id, nil
}

// GetChatGroup retrieves a chat group by ID
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
			return nil, fmt.Errorf("chat group not found: %w", err)
		}
		log.Errorw("Failed to get chat group", "error", err, "groupID", groupID)
		return nil, err
	}

	return &group, nil
}

// UpdateChatGroup updates a chat group
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

	_, err := s.db.Exec(ctx, query,
		update.Name,
		update.Description,
		now,
		groupID,
	)

	if err != nil {
		log.Errorw("Failed to update chat group", "error", err, "groupID", groupID)
		return err
	}

	return nil
}

// DeleteChatGroup deletes a chat group
func (s *PostgresChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	log := logger.GetLogger()

	query := `
		DELETE FROM chat_groups
		WHERE id = $1
	`

	_, err := s.db.Exec(ctx, query, groupID)

	if err != nil {
		log.Errorw("Failed to delete chat group", "error", err, "groupID", groupID)
		return err
	}

	return nil
}

// ListChatGroupsByTrip lists all chat groups for a trip
func (s *PostgresChatStore) ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error) {
	log := logger.GetLogger()

	// Set default limit if not provided
	if limit <= 0 {
		limit = 10
	}

	// Ensure offset is not negative
	if offset < 0 {
		offset = 0
	}

	// Initialize response with empty arrays
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

	// Query to get chat groups
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
		return response, nil // Return empty response instead of error
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
			log.Errorw("Failed to scan chat group", "error", err)
			return response, nil // Return empty response instead of error
		}
		response.Groups = append(response.Groups, group)
	}

	// Query to get total count
	countQuery := `
		SELECT COUNT(*)
		FROM chat_groups
		WHERE trip_id = $1
	`

	var total int
	err = s.db.QueryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to get total chat groups count", "error", err, "tripID", tripID)
		return response, nil // Return empty response with zero total
	}

	response.Pagination.Total = total

	return response, nil
}

// CreateChatMessage creates a new chat message
func (s *PostgresChatStore) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	log := logger.GetLogger()

	// Generate a new UUID if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	query := `
		INSERT INTO chat_messages (id, group_id, user_id, content, created_at, updated_at, is_edited, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	now := time.Now()
	message.CreatedAt = now
	message.UpdatedAt = now

	// For now, we'll use the TripID as the GroupID
	// In a real implementation, you would look up the appropriate group ID for the trip
	groupID := message.TripID

	var id string
	err := s.db.QueryRow(ctx, query,
		message.ID,
		groupID,
		message.UserID,
		message.Content,
		message.CreatedAt,
		message.UpdatedAt,
		message.IsEdited,
		message.IsDeleted,
	).Scan(&id)

	if err != nil {
		log.Errorw("Failed to create chat message", "error", err)
		return "", err
	}

	return id, nil
}

// GetChatMessage retrieves a chat message by ID
func (s *PostgresChatStore) GetChatMessage(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	log := logger.GetLogger()

	query := `
		SELECT m.id, m.group_id, m.user_id, m.content, m.created_at, m.updated_at, m.is_edited, m.is_deleted, g.trip_id
		FROM chat_messages m
		JOIN chat_groups g ON m.group_id = g.id
		WHERE m.id = $1
	`

	var message types.ChatMessage
	var groupID string
	err := s.db.QueryRow(ctx, query, messageID).Scan(
		&message.ID,
		&groupID,
		&message.UserID,
		&message.Content,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.IsEdited,
		&message.IsDeleted,
		&message.TripID,
	)

	if err != nil {
		log.Errorw("Failed to get chat message", "error", err, "messageID", messageID)
		return nil, err
	}

	return &message, nil
}

// UpdateChatMessage updates a chat message
func (s *PostgresChatStore) UpdateChatMessage(ctx context.Context, messageID string, content string) error {
	log := logger.GetLogger()

	query := `
		UPDATE chat_messages
		SET content = $1,
			is_edited = true,
			updated_at = $2
		WHERE id = $3
	`

	now := time.Now()

	_, err := s.db.Exec(ctx, query, content, now, messageID)

	if err != nil {
		log.Errorw("Failed to update chat message", "error", err, "messageID", messageID)
		return err
	}

	return nil
}

// DeleteChatMessage marks a chat message as deleted
func (s *PostgresChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	log := logger.GetLogger()

	query := `
		UPDATE chat_messages
		SET is_deleted = true,
			updated_at = $1
		WHERE id = $2
	`

	now := time.Now()

	_, err := s.db.Exec(ctx, query, now, messageID)

	if err != nil {
		log.Errorw("Failed to delete chat message", "error", err, "messageID", messageID)
		return err
	}

	return nil
}

// fetchUserFromSupabase fetches a user from Supabase REST API
func (s *PostgresChatStore) fetchUserFromSupabase(userID string) (*types.UserResponse, error) {
	// Construct the Supabase auth API URL
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", s.supabaseBaseURL, userID)

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add required headers
	req.Header.Add("apikey", s.supabaseAPIKey)
	req.Header.Add("Authorization", "Bearer "+s.supabaseAPIKey)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user, status: %d", resp.StatusCode)
	}

	// Parse response
	var supabaseUser struct {
		ID           string                 `json:"id"`
		Email        string                 `json:"email"`
		UserMetadata map[string]interface{} `json:"user_metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&supabaseUser); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to UserResponse
	user := &types.UserResponse{
		ID:    supabaseUser.ID,
		Email: supabaseUser.Email,
	}

	// Safely extract user metadata
	if supabaseUser.UserMetadata != nil {
		if username, ok := supabaseUser.UserMetadata["username"].(string); ok {
			user.Username = username
		}
		if firstName, ok := supabaseUser.UserMetadata["firstName"].(string); ok {
			user.FirstName = firstName
		}
		if lastName, ok := supabaseUser.UserMetadata["lastName"].(string); ok {
			user.LastName = lastName
		}
		if profilePicture, ok := supabaseUser.UserMetadata["profilePicture"].(string); ok {
			user.ProfilePicture = profilePicture
		}
	}

	return user, nil
}

// ListChatMessages lists all messages in a chat group
func (s *PostgresChatStore) ListChatMessages(ctx context.Context, groupID string, limit, offset int) (*types.ChatMessagePaginatedResponse, error) {
	log := logger.GetLogger()

	// Set default limit if not provided
	if limit <= 0 {
		limit = 20
	}

	// Ensure offset is not negative
	if offset < 0 {
		offset = 0
	}

	// Initialize response with empty arrays
	response := &types.ChatMessagePaginatedResponse{
		Messages: make([]types.ChatMessageWithUser, 0),
		Total:    0,
		Limit:    limit,
		Offset:   offset,
	}

	// Query to get chat messages
	query := `
		SELECT m.id, m.group_id, m.user_id, m.content, m.created_at, m.updated_at, m.is_edited, m.is_deleted
		FROM chat_messages m
		WHERE m.group_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(ctx, query, groupID, limit, offset)
	if err != nil {
		log.Errorw("Failed to list chat messages", "error", err, "groupID", groupID)
		return response, nil // Return empty response instead of error
	}
	defer rows.Close()

	var userIDs []string

	// First, collect all messages and user IDs
	for rows.Next() {
		var message types.ChatMessage
		var groupIDValue string
		err := rows.Scan(
			&message.ID,
			&groupIDValue,
			&message.UserID,
			&message.Content,
			&message.CreatedAt,
			&message.UpdatedAt,
			&message.IsEdited,
			&message.IsDeleted,
		)
		if err != nil {
			log.Errorw("Failed to scan chat message", "error", err)
			return response, nil // Return empty response instead of error
		}

		// Set the TripID from the group's trip_id
		// For now, we'll leave it empty and fill it later if needed
		message.TripID = ""

		userIDs = append(userIDs, message.UserID)
		response.Messages = append(response.Messages, types.ChatMessageWithUser{
			Message: message,
		})
	}

	// Get user information from Supabase for all users at once
	userMap := make(map[string]types.UserResponse)
	for _, userID := range userIDs {
		user, err := s.fetchUserFromSupabase(userID)
		if err != nil {
			log.Warnw("Failed to fetch user from Supabase", "error", err, "userID", userID)
			// Create a minimal user response if fetch fails
			userMap[userID] = types.UserResponse{
				ID: userID,
			}
			continue
		}
		userMap[userID] = *user
	}

	// Attach user information to messages
	for i := range response.Messages {
		if user, ok := userMap[response.Messages[i].Message.UserID]; ok {
			response.Messages[i].User = user
		}
	}

	// Query to get total count
	countQuery := `
		SELECT COUNT(*)
		FROM chat_messages
		WHERE group_id = $1
	`

	var total int
	err = s.db.QueryRow(ctx, countQuery, groupID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to get total chat messages count", "error", err, "groupID", groupID)
		return response, nil // Return empty response with zero total
	}

	response.Total = total

	return response, nil
}

// AddChatGroupMember adds a user to a chat group
func (s *PostgresChatStore) AddChatGroupMember(ctx context.Context, groupID, userID string) error {
	log := logger.GetLogger()

	query := `
		INSERT INTO chat_group_members (id, group_id, user_id, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`

	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(ctx, query, id, groupID, userID, now)

	if err != nil {
		log.Errorw("Failed to add chat group member", "error", err, "groupID", groupID, "userID", userID)
		return err
	}

	return nil
}

// RemoveChatGroupMember removes a user from a chat group
func (s *PostgresChatStore) RemoveChatGroupMember(ctx context.Context, groupID, userID string) error {
	log := logger.GetLogger()

	query := `
		DELETE FROM chat_group_members
		WHERE group_id = $1 AND user_id = $2
	`

	_, err := s.db.Exec(ctx, query, groupID, userID)

	if err != nil {
		log.Errorw("Failed to remove chat group member", "error", err, "groupID", groupID, "userID", userID)
		return err
	}

	return nil
}

// ListChatGroupMembers lists all members of a chat group
func (s *PostgresChatStore) ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	log := logger.GetLogger()

	// Initialize empty response array
	users := make([]types.UserResponse, 0)

	query := `
		SELECT user_id
		FROM chat_group_members
		WHERE group_id = $1
	`

	rows, err := s.db.Query(ctx, query, groupID)
	if err != nil {
		log.Errorw("Failed to list chat group members", "error", err, "groupID", groupID)
		return users, nil // Return empty array instead of error
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		err := rows.Scan(&userID)
		if err != nil {
			log.Errorw("Failed to scan chat group member", "error", err)
			return users, nil // Return empty array instead of error
		}
		userIDs = append(userIDs, userID)
	}

	for _, userID := range userIDs {
		user, err := s.fetchUserFromSupabase(userID)
		if err != nil {
			log.Warnw("Failed to fetch user from Supabase", "error", err, "userID", userID)
			// Add a minimal user response if fetch fails
			users = append(users, types.UserResponse{
				ID: userID,
			})
			continue
		}
		users = append(users, *user)
	}

	return users, nil
}

// UpdateLastReadMessage updates the last read message for a user in a chat group
func (s *PostgresChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	log := logger.GetLogger()

	query := `
		UPDATE chat_group_members
		SET last_read_message_id = $1
		WHERE group_id = $2 AND user_id = $3
	`

	_, err := s.db.Exec(ctx, query, messageID, groupID, userID)

	if err != nil {
		log.Errorw("Failed to update last read message", "error", err, "groupID", groupID, "userID", userID, "messageID", messageID)
		return err
	}

	return nil
}

// AddChatMessageReaction adds a reaction to a chat message
func (s *PostgresChatStore) AddChatMessageReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	query := `
		INSERT INTO chat_message_reactions (id, message_id, user_id, reaction, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (message_id, user_id, reaction) DO NOTHING
	`

	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(ctx, query, id, messageID, userID, reaction, now)

	if err != nil {
		log.Errorw("Failed to add chat message reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return err
	}

	return nil
}

// RemoveChatMessageReaction removes a reaction from a chat message
func (s *PostgresChatStore) RemoveChatMessageReaction(ctx context.Context, messageID, userID, reaction string) error {
	log := logger.GetLogger()

	query := `
		DELETE FROM chat_message_reactions
		WHERE message_id = $1 AND user_id = $2 AND reaction = $3
	`

	_, err := s.db.Exec(ctx, query, messageID, userID, reaction)

	if err != nil {
		log.Errorw("Failed to remove chat message reaction", "error", err, "messageID", messageID, "userID", userID, "reaction", reaction)
		return err
	}

	return nil
}

// ListChatMessageReactions lists all reactions for a chat message
func (s *PostgresChatStore) ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error) {
	log := logger.GetLogger()

	query := `
		SELECT id, message_id, user_id, reaction, created_at
		FROM chat_message_reactions
		WHERE message_id = $1
	`

	rows, err := s.db.Query(ctx, query, messageID)
	if err != nil {
		log.Errorw("Failed to list chat message reactions", "error", err, "messageID", messageID)
		return nil, err
	}
	defer rows.Close()

	var reactions []types.ChatMessageReaction
	for rows.Next() {
		var reaction types.ChatMessageReaction
		err := rows.Scan(
			&reaction.ID,
			&reaction.MessageID,
			&reaction.UserID,
			&reaction.Reaction,
			&reaction.CreatedAt,
		)
		if err != nil {
			log.Errorw("Failed to scan chat message reaction", "error", err)
			return nil, err
		}
		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

// GetUserInfo retrieves user information from Supabase
func (s *PostgresChatStore) GetUserInfo(ctx context.Context, userID string) (*types.UserResponse, error) {
	log := logger.GetLogger()

	// First try to fetch from Supabase
	user, err := s.fetchUserFromSupabase(userID)
	if err != nil {
		log.Warnw("Failed to fetch user from Supabase, falling back to local data", "error", err, "userID", userID)

		// Fallback to local database if available
		// This is a simplified implementation - in a real app, you might have a users table
		// For now, we'll just return a minimal user object with the ID
		return &types.UserResponse{
			ID:       userID,
			Username: "User " + userID[:8], // Use first 8 chars of UUID as username
		}, nil
	}

	return user, nil
}

// ListTripMessages lists all messages for a trip
func (s *PostgresChatStore) ListTripMessages(ctx context.Context, tripID string, limit, offset int) ([]types.ChatMessage, int, error) {
	log := logger.GetLogger()

	// First, get the total count
	var total int
	countQuery := `
		SELECT COUNT(*) 
		FROM chat_messages cm
		JOIN chat_groups cg ON cm.group_id = cg.id
		WHERE cg.trip_id = $1
	`
	err := s.db.QueryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		log.Errorw("Failed to count trip messages", "error", err, "tripID", tripID)
		return nil, 0, fmt.Errorf("failed to count trip messages: %w", err)
	}

	// Then get the messages with pagination
	query := `
		SELECT 
			cm.id, 
			cm.group_id, 
			cm.user_id, 
			cm.content, 
			cm.created_at, 
			cm.updated_at,
			cm.is_edited,
			cm.is_deleted
		FROM chat_messages cm
		JOIN chat_groups cg ON cm.group_id = cg.id
		WHERE cg.trip_id = $1
		ORDER BY cm.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(ctx, query, tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to query trip messages", "error", err, "tripID", tripID)
		return nil, 0, fmt.Errorf("failed to query trip messages: %w", err)
	}
	defer rows.Close()

	messages := []types.ChatMessage{}
	for rows.Next() {
		var msg types.ChatMessage
		var groupID string
		err := rows.Scan(
			&msg.ID,
			&groupID, // We don't use this in the response
			&msg.UserID,
			&msg.Content,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&msg.IsEdited,
			&msg.IsDeleted,
		)
		if err != nil {
			log.Errorw("Failed to scan message row", "error", err)
			continue
		}

		// Set the TripID instead of GroupID
		msg.TripID = tripID

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		log.Errorw("Error iterating message rows", "error", err)
		return nil, 0, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, total, nil
}
