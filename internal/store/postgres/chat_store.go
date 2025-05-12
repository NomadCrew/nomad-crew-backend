package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NomadCrew/nomad-crew-backend/internal/store"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// ChatStore implements the store.ChatStore interface using PostgreSQL
type ChatStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewChatStore creates a new ChatStore instance
func NewChatStore(pool *pgxpool.Pool) *ChatStore {
	return &ChatStore{
		pool: pool,
	}
}

// BeginTx starts a new database transaction
func (s *ChatStore) BeginTx(ctx context.Context) (store.Transaction, error) {
	if s.tx != nil {
		return nil, errors.New("transaction already started")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	s.tx = tx
	return &Transaction{tx: tx}, nil
}

// Implement ChatStore interface methods here...
// Initially, these will just return errors or default values.

func (s *ChatStore) CreateChatGroup(ctx context.Context, group types.ChatGroup) (string, error) {
	query := `
		INSERT INTO chat_groups (trip_id, name, created_by)
		VALUES ($1, $2, $3)
		RETURNING id`

	var groupID string
	row := s.queryRow(ctx, query,
		group.TripID,
		group.Name,
		group.CreatedBy,
	)

	err := row.Scan(&groupID)
	if err != nil {
		// TODO: Add more specific error handling (e.g., constraint violations)
		return "", fmt.Errorf("error creating chat group: %w", err)
	}

	return groupID, nil
}

func (s *ChatStore) GetChatGroup(ctx context.Context, groupID string) (*types.ChatGroup, error) {
	query := `
		SELECT id, trip_id, name, created_by, created_at, updated_at
		FROM chat_groups
		WHERE id = $1 AND deleted_at IS NULL`

	group := &types.ChatGroup{}
	row := s.queryRow(ctx, query, groupID)

	err := row.Scan(
		&group.ID,
		&group.TripID,
		&group.Name,
		&group.CreatedBy,
		&group.CreatedAt,
		&group.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Use the ErrNotFound defined in todo.go for now, or define a shared one
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting chat group: %w", err)
	}

	return group, nil
}

func (s *ChatStore) UpdateChatGroup(ctx context.Context, groupID string, update types.ChatGroupUpdateRequest) error {
	query := `
		UPDATE chat_groups
		SET name = COALESCE($1, name),
			updated_at = NOW() -- Trigger should handle this, but explicit doesn't hurt
		WHERE id = $2 AND deleted_at IS NULL`

	cmdTag, err := s.exec(ctx, query, update.Name, groupID)
	if err != nil {
		return fmt.Errorf("error updating chat group: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound // Group didn't exist or was already deleted
	}

	return nil
}

func (s *ChatStore) DeleteChatGroup(ctx context.Context, groupID string) error {
	query := `
		UPDATE chat_groups
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	cmdTag, err := s.exec(ctx, query, groupID)
	if err != nil {
		return fmt.Errorf("error deleting chat group: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound // Group didn't exist or was already deleted
	}

	return nil
}

func (s *ChatStore) ListChatGroupsByTrip(ctx context.Context, tripID string, limit, offset int) (*types.ChatGroupPaginatedResponse, error) {
	response := &types.ChatGroupPaginatedResponse{
		Groups: []types.ChatGroup{},
		Pagination: struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}{Limit: limit, Offset: offset},
	}

	// Query to get the groups for the current page
	query := `
		SELECT id, trip_id, name, created_by, created_at, updated_at
		FROM chat_groups
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC -- Or maybe name? Decide on default order
		LIMIT $2 OFFSET $3`

	rows, err := s.query(ctx, query, tripID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listing chat groups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		group := types.ChatGroup{}
		err := rows.Scan(
			&group.ID,
			&group.TripID,
			&group.Name,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning chat group row: %w", err)
		}
		response.Groups = append(response.Groups, group)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat group rows: %w", err)
	}

	// Query to get the total count of groups for the trip
	countQuery := `SELECT COUNT(*) FROM chat_groups WHERE trip_id = $1 AND deleted_at IS NULL`
	var total int
	err = s.queryRow(ctx, countQuery, tripID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("error counting chat groups: %w", err)
	}
	response.Pagination.Total = total

	return response, nil
}

func (s *ChatStore) CreateChatMessage(ctx context.Context, message types.ChatMessage) (string, error) {
	query := `
		INSERT INTO chat_messages (group_id, user_id, content)
		VALUES ($1, $2, $3)
		RETURNING id`

	var messageID string
	row := s.queryRow(ctx, query,
		message.GroupID,
		message.UserID,
		message.Content,
	)

	err := row.Scan(&messageID)
	if err != nil {
		// TODO: Handle potential foreign key errors (invalid group_id or user_id)
		return "", fmt.Errorf("error creating chat message: %w", err)
	}

	return messageID, nil
}

func (s *ChatStore) GetChatMessageByID(ctx context.Context, messageID string) (*types.ChatMessage, error) {
	query := `
		SELECT id, group_id, user_id, content, created_at, updated_at
		FROM chat_messages
		WHERE id = $1 AND deleted_at IS NULL`

	message := &types.ChatMessage{}
	row := s.queryRow(ctx, query, messageID)

	err := row.Scan(
		&message.ID,
		&message.GroupID,
		&message.UserID,
		&message.Content,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting chat message: %w", err)
	}

	// TODO: Optionally fetch reactions here or leave it to the service layer

	return message, nil
}

func (s *ChatStore) UpdateChatMessage(ctx context.Context, messageID string, content string) error {
	query := `
		UPDATE chat_messages
		SET content = $1,
			updated_at = NOW() -- Trigger might handle this, but explicit is fine
		WHERE id = $2 AND deleted_at IS NULL`

	cmdTag, err := s.exec(ctx, query, content, messageID)
	if err != nil {
		return fmt.Errorf("error updating chat message: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound // Message didn't exist or was deleted
	}

	return nil
}

func (s *ChatStore) DeleteChatMessage(ctx context.Context, messageID string) error {
	query := `
		UPDATE chat_messages
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	cmdTag, err := s.exec(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("error deleting chat message: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound // Message didn't exist or was already deleted
	}

	return nil
}

func (s *ChatStore) ListChatMessages(ctx context.Context, groupID string, params types.PaginationParams) ([]types.ChatMessage, int, error) {
	var messages []types.ChatMessage

	// Build the base query
	baseQuery := `
		FROM chat_messages
		WHERE group_id = $1 AND deleted_at IS NULL
	`

	// Query for the messages on the current page
	listQuery := `SELECT id, group_id, user_id, content, created_at, updated_at ` + baseQuery +
		`ORDER BY created_at DESC ` +
		`LIMIT $2 OFFSET $3`

	rows, err := s.query(ctx, listQuery, groupID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("error listing chat messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var msg types.ChatMessage
		err := rows.Scan(
			&msg.ID,
			&msg.GroupID,
			&msg.UserID,
			&msg.Content,
			&msg.CreatedAt,
			&msg.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("error scanning chat message row: %w", err)
		}
		messages = append(messages, msg)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating chat message rows: %w", err)
	}

	// Query for the total count
	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	err = s.queryRow(ctx, countQuery, groupID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("error counting chat messages: %w", err)
	}

	return messages, total, nil
}

func (s *ChatStore) AddChatGroupMember(ctx context.Context, groupID, userID string) error {
	query := `
		INSERT INTO chat_group_members (group_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (group_id, user_id) DO NOTHING` // Ignore if member already exists

	_, err := s.exec(ctx, query, groupID, userID)
	if err != nil {
		// TODO: Check for specific errors like foreign key violation if groupID or userID is invalid
		return fmt.Errorf("error adding chat group member: %w", err)
	}
	return nil
}

func (s *ChatStore) RemoveChatGroupMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM chat_group_members WHERE group_id = $1 AND user_id = $2`

	_, err := s.exec(ctx, query, groupID, userID)
	if err != nil {
		return fmt.Errorf("error removing chat group member: %w", err)
	}

	// If RowsAffected was 0, the member wasn't in the group, which is acceptable for idempotency.
	// cmdTag is not used, so assigned to blank identifier.
	// Optionally log a warning if needed (would require checking RowsAffected from cmdTag if it were used).

	return nil
}

func (s *ChatStore) ListChatGroupMembers(ctx context.Context, groupID string) ([]types.UserResponse, error) {
	query := `
		SELECT cgm.user_id, u.email, u.raw_user_meta_data
		FROM chat_group_members cgm
		JOIN users u ON cgm.user_id = u.id
		WHERE cgm.group_id = $1`

	rows, err := s.query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("error querying chat group members: %w", err)
	}
	defer rows.Close()

	var members []types.UserResponse
	for rows.Next() {
		var userID string
		var email string
		var rawMetaData []byte
		err := rows.Scan(&userID, &email, &rawMetaData)
		if err != nil {
			return nil, fmt.Errorf("error scanning chat group member row: %w", err)
		}

		member := types.UserResponse{
			ID:    userID,
			Email: email,
		}

		// Unmarshal metadata
		var metadata types.UserMetadata
		if len(rawMetaData) > 0 {
			err = json.Unmarshal(rawMetaData, &metadata)
			if err != nil {
				fmt.Printf("Warning: Failed to unmarshal user metadata for member %s in group %s: %v\n", userID, groupID, err)
				// Continue with potentially incomplete member data
			}
		}

		member.Username = metadata.Username
		member.FirstName = metadata.FirstName
		member.LastName = metadata.LastName
		member.AvatarURL = metadata.ProfilePicture

		// Construct DisplayName
		if metadata.FirstName != "" {
			member.DisplayName = metadata.FirstName
			if metadata.LastName != "" {
				member.DisplayName += " " + metadata.LastName
			}
		} else if metadata.Username != "" {
			member.DisplayName = metadata.Username
		} else {
			member.DisplayName = "User " + userID[:6] // Fallback display name
		}

		members = append(members, member)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat group member rows: %w", err)
	}

	return members, nil
}

func (s *ChatStore) UpdateLastReadMessage(ctx context.Context, groupID, userID, messageID string) error {
	// Check if the message exists and belongs to the group (optional but good practice)
	// We can skip this check here and assume the service layer validates it.

	query := `
		UPDATE chat_group_members
		SET last_read_message_id = $1
		WHERE group_id = $2 AND user_id = $3`
	// Add condition to only update if new message is later? Requires message timestamps.
	// Example: AND (SELECT created_at FROM chat_messages WHERE id = $1) > (SELECT created_at FROM chat_messages WHERE id = last_read_message_id)
	// Keeping it simple for now: unconditionally update.

	cmdTag, err := s.exec(ctx, query, messageID, groupID, userID)
	if err != nil {
		// Handle potential foreign key errors if messageID is invalid
		return fmt.Errorf("error updating last read message: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		// This means the user is not a member of the group.
		// Return an error or handle as appropriate (e.g., log warning).
		// For now, let's return a specific error or ErrNotFound.
		return fmt.Errorf("user %s is not a member of group %s: %w", userID, groupID, ErrNotFound)
	}

	return nil
}

func (s *ChatStore) AddReaction(ctx context.Context, messageID, userID, reaction string) error {
	query := `
		INSERT INTO chat_message_reactions (message_id, user_id, reaction)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id, reaction) DO NOTHING` // Ignore if reaction already exists

	_, err := s.exec(ctx, query, messageID, userID, reaction)
	if err != nil {
		// Handle potential FK violations (invalid messageID or userID)
		return fmt.Errorf("error adding reaction: %w", err)
	}
	return nil
}

func (s *ChatStore) RemoveReaction(ctx context.Context, messageID, userID, reaction string) error {
	query := `DELETE FROM chat_message_reactions WHERE message_id = $1 AND user_id = $2 AND reaction = $3`

	_, err := s.exec(ctx, query, messageID, userID, reaction)
	if err != nil {
		return fmt.Errorf("error removing reaction: %w", err)
	}

	// If RowsAffected was 0, the reaction didn't exist, which is acceptable for idempotency.
	// cmdTag is not used, so assigned to blank identifier.
	// Optionally log a warning if needed (would require checking RowsAffected from cmdTag if it were used).

	return nil
}

func (s *ChatStore) ListChatMessageReactions(ctx context.Context, messageID string) ([]types.ChatMessageReaction, error) {
	query := `
		SELECT message_id, user_id, reaction, created_at
		FROM chat_message_reactions
		WHERE message_id = $1
		ORDER BY created_at ASC` // Order reactions chronologically

	rows, err := s.query(ctx, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("error listing reactions: %w", err)
	}
	defer rows.Close()

	var reactions []types.ChatMessageReaction
	for rows.Next() {
		var r types.ChatMessageReaction
		// Assuming ChatMessageReaction struct has fields: MessageID, UserID, Reaction, CreatedAt
		err := rows.Scan(
			&r.MessageID,
			&r.UserID,
			&r.Reaction,
			&r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning reaction row: %w", err)
		}
		reactions = append(reactions, r)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reaction rows: %w", err)
	}

	return reactions, nil
}

// GetUserByID retrieves a user by ID
// This method is added to support tests and interfaces,
// but user operations should be performed through UserStore
func (s *ChatStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	// Delegate to the companion UserStore if possible
	// But for standalone usage/testing, implement basic functionality

	query := `
		SELECT id, email, raw_user_meta_data, username, first_name, last_name, profile_picture_url
		FROM users
		WHERE id = $1`

	var (
		id                string
		email             string
		rawUserMetaData   []byte
		username          string
		firstName         string
		lastName          string
		profilePictureURL string
	)

	row := s.queryRow(ctx, query, userID)
	err := row.Scan(&id, &email, &rawUserMetaData, &username, &firstName, &lastName, &profilePictureURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	user := &types.User{
		ID:                id,
		Email:             email,
		RawUserMetaData:   rawUserMetaData,
		Username:          username,
		FirstName:         firstName,
		LastName:          lastName,
		ProfilePictureURL: profilePictureURL,
	}

	return user, nil
}

// Helper methods for database operations (copied from todo.go for now)

func (s *ChatStore) queryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	if s.tx != nil {
		return s.tx.QueryRow(ctx, query, args...)
	}
	return s.pool.QueryRow(ctx, query, args...)
}

func (s *ChatStore) query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if s.tx != nil {
		return s.tx.Query(ctx, query, args...)
	}
	return s.pool.Query(ctx, query, args...)
}

func (s *ChatStore) exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	if s.tx != nil {
		return s.tx.Exec(ctx, query, args...)
	}
	return s.pool.Exec(ctx, query, args...)
}
