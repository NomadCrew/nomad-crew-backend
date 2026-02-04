package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
)

// Helper function to create test chat group
func createTestChatGroup() types.ChatGroup {
	return types.ChatGroup{
		ID:        uuid.NewString(),
		TripID:    uuid.NewString(),
		Name:      "Test Group",
		CreatedBy: uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Helper function to create test chat message
func createTestChatMessage() types.ChatMessage {
	return types.ChatMessage{
		ID:        uuid.NewString(),
		GroupID:   uuid.NewString(),
		UserID:    uuid.NewString(),
		Content:   "Test message content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestChatStore_CreateChatGroup(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	group := createTestChatGroup()

	t.Run("successful creation", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id"}).AddRow(group.ID)
		
		mock.ExpectQuery("INSERT INTO chat_groups \\(trip_id, name, created_by\\) VALUES").
			WithArgs(group.TripID, group.Name, group.CreatedBy).
			WillReturnRows(rows)

		// Would test successful group creation
	})

	t.Run("duplicate group name", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO chat_groups").
			WithArgs(group.TripID, group.Name, group.CreatedBy).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would return appropriate error
	})

	t.Run("invalid trip ID", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO chat_groups").
			WithArgs(group.TripID, group.Name, group.CreatedBy).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "trip not found" error
	})
}

func TestChatStore_GetChatGroup(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	group := createTestChatGroup()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "name", "created_by", "created_at", "updated_at",
		}).AddRow(
			group.ID, group.TripID, group.Name, group.CreatedBy, group.CreatedAt, group.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM chat_groups WHERE id = \\$1").
			WithArgs(group.ID).
			WillReturnRows(rows)

		// Would verify successful retrieval
	})

	t.Run("group not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM chat_groups WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return ErrChatGroupNotFound
	})
}

func TestChatStore_UpdateChatGroup(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()

	t.Run("update name", func(t *testing.T) {
		update := types.ChatGroupUpdateRequest{
			Name: "Updated Group Name",
		}

		mock.ExpectExec("UPDATE chat_groups SET name = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(update.Name, groupID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("group not found", func(t *testing.T) {
		update := types.ChatGroupUpdateRequest{
			Name: "Updated Name",
		}

		mock.ExpectExec("UPDATE chat_groups SET").
			WithArgs(update.Name, groupID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})
}

func TestChatStore_DeleteChatGroup(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()

	t.Run("successful deletion", func(t *testing.T) {
		// Start transaction
		mock.ExpectBegin()

		// Delete messages
		mock.ExpectExec("DELETE FROM chat_messages WHERE group_id = \\$1").
			WithArgs(groupID).
			WillReturnResult(sqlmock.NewResult(0, 5))

		// Delete group members
		mock.ExpectExec("DELETE FROM chat_group_members WHERE group_id = \\$1").
			WithArgs(groupID).
			WillReturnResult(sqlmock.NewResult(0, 3))

		// Delete group
		mock.ExpectExec("DELETE FROM chat_groups WHERE id = \\$1").
			WithArgs(groupID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Commit transaction
		mock.ExpectCommit()

		// Would verify cascade deletion
	})

	t.Run("rollback on error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM chat_messages").
			WithArgs(groupID).
			WillReturnError(errors.New("deletion failed"))
		mock.ExpectRollback()

		// Would verify rollback behavior
	})
}

func TestChatStore_ListChatGroupsByTrip(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	tripID := uuid.NewString()

	t.Run("successful list with pagination", func(t *testing.T) {
		limit, offset := 10, 0

		// Count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM chat_groups WHERE trip_id = \\$1").
			WithArgs(tripID).
			WillReturnRows(countRows)

		// List query
		group1 := createTestChatGroup()
		group2 := createTestChatGroup()
		group2.ID = uuid.NewString()

		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "name", "created_by", "created_at", "updated_at",
		}).
			AddRow(group1.ID, group1.TripID, group1.Name, group1.CreatedBy, group1.CreatedAt, group1.UpdatedAt).
			AddRow(group2.ID, group2.TripID, group2.Name, group2.CreatedBy, group2.CreatedAt, group2.UpdatedAt)

		mock.ExpectQuery("SELECT (.+) FROM chat_groups WHERE trip_id = \\$1 ORDER BY created_at DESC LIMIT \\$2 OFFSET \\$3").
			WithArgs(tripID, limit, offset).
			WillReturnRows(rows)

		// Would verify pagination response
	})

	t.Run("empty results", func(t *testing.T) {
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM chat_groups").
			WithArgs(tripID).
			WillReturnRows(countRows)

		// Would return empty result with zero total
	})
}

func TestChatStore_CreateChatMessage(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	message := createTestChatMessage()

	t.Run("successful creation", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id"}).AddRow(message.ID)

		mock.ExpectQuery("INSERT INTO chat_messages \\(group_id, user_id, content\\) VALUES").
			WithArgs(message.GroupID, message.UserID, message.Content).
			WillReturnRows(rows)

		// Would verify successful message creation
	})

	t.Run("invalid group ID", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO chat_messages").
			WithArgs(message.GroupID, message.UserID, message.Content).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "group not found" error
	})

	t.Run("empty content", func(t *testing.T) {
		emptyMessage := createTestChatMessage()
		emptyMessage.Content = ""

		// Should validate before database call
	})
}

func TestChatStore_ListChatMessages(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()

	t.Run("successful list with pagination", func(t *testing.T) {
		params := types.PaginationParams{
			Limit:  20,
			Offset: 0,
		}

		// Count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(50)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM chat_messages WHERE group_id = \\$1").
			WithArgs(groupID).
			WillReturnRows(countRows)

		// Messages query
		msg1 := createTestChatMessage()
		msg2 := createTestChatMessage()
		msg2.ID = uuid.NewString()

		rows := sqlmock.NewRows([]string{
			"id", "group_id", "user_id", "content", "created_at", "updated_at",
		}).
			AddRow(msg1.ID, msg1.GroupID, msg1.UserID, msg1.Content, msg1.CreatedAt, msg1.UpdatedAt).
			AddRow(msg2.ID, msg2.GroupID, msg2.UserID, msg2.Content, msg2.CreatedAt, msg2.UpdatedAt)

		mock.ExpectQuery("SELECT (.+) FROM chat_messages WHERE group_id = \\$1 ORDER BY created_at DESC LIMIT \\$2 OFFSET \\$3").
			WithArgs(groupID, params.Limit, params.Offset).
			WillReturnRows(rows)

		// Would verify messages and total count
	})

	t.Run("with offset pagination", func(t *testing.T) {
		params := types.PaginationParams{
			Limit:  20,
			Offset: 40,
		}

		// Would test offset-based pagination
		mock.ExpectQuery("SELECT (.+) FROM chat_messages WHERE group_id = \\$1 ORDER BY created_at DESC LIMIT \\$2 OFFSET \\$3").
			WithArgs(groupID, params.Limit, params.Offset).
			WillReturnRows(sqlmock.NewRows([]string{}))
	})
}

func TestChatStore_UpdateChatMessage(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	messageID := uuid.NewString()
	newContent := "Updated message content"

	t.Run("successful update", func(t *testing.T) {
		mock.ExpectExec("UPDATE chat_messages SET content = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(newContent, messageID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("message not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE chat_messages").
			WithArgs(newContent, messageID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return appropriate error
	})
}

func TestChatStore_DeleteChatMessage(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	messageID := uuid.NewString()

	t.Run("successful deletion", func(t *testing.T) {
		// Begin transaction
		mock.ExpectBegin()

		// Delete reactions first
		mock.ExpectExec("DELETE FROM chat_message_reactions WHERE message_id = \\$1").
			WithArgs(messageID).
			WillReturnResult(sqlmock.NewResult(0, 3))

		// Delete message
		mock.ExpectExec("DELETE FROM chat_messages WHERE id = \\$1").
			WithArgs(messageID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Commit
		mock.ExpectCommit()

		// Would verify cascade deletion
	})
}

func TestChatStore_AddChatGroupMember(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()
	userID := uuid.NewString()

	t.Run("successful addition", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO chat_group_members \\(group_id, user_id\\) VALUES").
			WithArgs(groupID, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify member added
	})

	t.Run("duplicate member", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO chat_group_members").
			WithArgs(groupID, userID).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would return "already a member" error
	})
}

func TestChatStore_RemoveChatGroupMember(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()
	userID := uuid.NewString()

	t.Run("successful removal", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM chat_group_members WHERE group_id = \\$1 AND user_id = \\$2").
			WithArgs(groupID, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Would verify member removed
	})

	t.Run("member not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM chat_group_members").
			WithArgs(groupID, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return appropriate error
	})
}

func TestChatStore_ListChatGroupMembers(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()

	t.Run("successful list", func(t *testing.T) {
		user1ID := uuid.NewString()
		user2ID := uuid.NewString()

		rows := sqlmock.NewRows([]string{
			"user_id", "username", "display_name", "avatar_url", "joined_at",
		}).
			AddRow(user1ID, "user1", "User One", "https://example.com/avatar1.jpg", time.Now()).
			AddRow(user2ID, "user2", "User Two", "https://example.com/avatar2.jpg", time.Now())

		mock.ExpectQuery("SELECT (.+) FROM chat_group_members cgm JOIN users u ON cgm.user_id = u.id WHERE cgm.group_id = \\$1").
			WithArgs(groupID).
			WillReturnRows(rows)

		// Would return list of user responses
	})

	t.Run("empty group", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "username", "display_name", "avatar_url", "joined_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM chat_group_members").
			WithArgs(groupID).
			WillReturnRows(rows)

		// Would return empty slice
	})
}

func TestChatStore_UpdateLastReadMessage(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	groupID := uuid.NewString()
	userID := uuid.NewString()
	messageID := uuid.NewString()

	t.Run("successful update", func(t *testing.T) {
		mock.ExpectExec("UPDATE chat_group_members SET last_read_message_id = \\$1 WHERE group_id = \\$2 AND user_id = \\$3").
			WithArgs(messageID, groupID, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify last read updated
	})

	t.Run("member not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE chat_group_members").
			WithArgs(messageID, groupID, userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would return appropriate error
	})
}

func TestChatStore_AddReaction(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	messageID := uuid.NewString()
	userID := uuid.NewString()
	reaction := "👍"

	t.Run("successful addition", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO chat_message_reactions \\(message_id, user_id, reaction\\) VALUES").
			WithArgs(messageID, userID, reaction).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify reaction added
	})

	t.Run("duplicate reaction", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO chat_message_reactions").
			WithArgs(messageID, userID, reaction).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would handle gracefully or return error
	})
}

func TestChatStore_RemoveReaction(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	messageID := uuid.NewString()
	userID := uuid.NewString()
	reaction := "👍"

	t.Run("successful removal", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM chat_message_reactions WHERE message_id = \\$1 AND user_id = \\$2 AND reaction = \\$3").
			WithArgs(messageID, userID, reaction).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Would verify reaction removed
	})

	t.Run("reaction not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM chat_message_reactions").
			WithArgs(messageID, userID, reaction).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would handle gracefully
	})
}

func TestChatStore_ListChatMessageReactions(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	messageID := uuid.NewString()

	t.Run("successful list", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "reaction", "created_at",
		}).
			AddRow(uuid.NewString(), "👍", time.Now()).
			AddRow(uuid.NewString(), "❤️", time.Now()).
			AddRow(uuid.NewString(), "😊", time.Now())

		mock.ExpectQuery("SELECT user_id, reaction, created_at FROM chat_message_reactions WHERE message_id = \\$1").
			WithArgs(messageID).
			WillReturnRows(rows)

		// Would return list of reactions
	})

	t.Run("no reactions", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"user_id", "reaction", "created_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM chat_message_reactions").
			WithArgs(messageID).
			WillReturnRows(rows)

		// Would return empty slice
	})
}

// Benchmark tests
func BenchmarkChatStore_CreateChatMessage(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	message := createTestChatMessage()
	rows := sqlmock.NewRows([]string{"id"}).AddRow(message.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("INSERT INTO chat_messages").
			WithArgs(message.GroupID, message.UserID, message.Content).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}

func BenchmarkChatStore_ListChatMessages(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	groupID := uuid.NewString()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Setup count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(100)
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(groupID).
			WillReturnRows(countRows)

		// Setup list query
		// Execute in actual implementation
	}
}