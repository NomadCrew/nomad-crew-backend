package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Reuse the existing setupTestDB and teardownTestDB from todo_test.go
// Ensure the necessary chat-related migrations are run in setupTestDB.

func setupTestDBWithChat(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID) {
	// Skip container tests on Windows to avoid rootless Docker issues
	if runtime.GOOS == "windows" {
		t.Skip("Skipping PostgreSQL container tests on Windows")
		// Return nil placeholders as the test will be skipped
		return nil, uuid.New(), uuid.New(), uuid.New()
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable",
		host, port.Port())

	// Add a short delay to ensure the database is fully ready
	time.Sleep(2 * time.Second)

	config, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)

	testPool, err = pgxpool.ConnectConfig(ctx, config)
	require.NoError(t, err)

	// Run base migrations (adjust paths if necessary)
	migrationFiles := []string{
		"../../../db/migrations/000001_init.up.sql",
		// "../../../db/migrations/000005_create_chat_tables.up.sql", // Removed reference to non-existent file
	}

	for _, migrationFile := range migrationFiles {
		// Check if file exists before reading
		if _, err := os.Stat(migrationFile); err == nil {
			fmt.Printf("Applying migration: %s\n", migrationFile)
			sqlBytes, err := os.ReadFile(migrationFile)
			require.NoError(t, err, "Error reading migration file: %s", migrationFile)
			_, err = testPool.Exec(ctx, string(sqlBytes))
			require.NoError(t, err, "Error executing migration: %s", migrationFile)
		} else if os.IsNotExist(err) {
			// Handle missing optional files gracefully or fail if required
			fmt.Printf("Warning: Migration file not found, skipping: %s\n", migrationFile)
			/* // Commented out the faulty check
			if migrationFile == "../../../db/migrations/00000X_create_chat_tables.up.sql" { // Make chat migration mandatory for these tests
				t.Fatalf("Required chat migration file not found: %s", migrationFile)
			}
			*/
		} else {
			// Other error checking the file
			require.NoError(t, err, "Error checking migration file: %s", migrationFile)
		}
	}

	// Create test user and trip
	userID := uuid.New()
	tripID := uuid.New()
	anotherUserID := uuid.New() // For membership tests

	user1Meta := `{"username":"testuser","firstName":"Test","lastName":"User","avatar_url":"http://example.com/avatar1.png"}`
	user2Meta := `{"username":"anotheruser","avatar_url":"http://example.com/avatar2.png"}`

	// Insert into both users and auth.users tables to satisfy foreign key constraints
	_, err = testPool.Exec(ctx, `
		INSERT INTO users (id, supabase_id, email, username, name, raw_user_meta_data, created_at, updated_at)
		VALUES ($1, $2, 'test@example.com', 'testuser1', 'testuser', $3::jsonb, NOW(), NOW()),
		       ($4, $5, 'another@example.com', 'testuser2', 'anotheruser', $6::jsonb, NOW(), NOW())`,
		userID,
		uuid.New().String(),
		user1Meta,
		anotherUserID,
		uuid.New().String(),
		user2Meta,
	)
	require.NoError(t, err)

	// Insert into auth.users table for foreign key constraint satisfaction
	_, err = testPool.Exec(ctx, `
		INSERT INTO auth.users (id, email, created_at, updated_at)
		VALUES ($1, 'test@example.com', NOW(), NOW()),
		       ($2, 'another@example.com', NOW(), NOW())`,
		userID,
		anotherUserID,
	)
	require.NoError(t, err)

	_, err = testPool.Exec(ctx, `
		INSERT INTO trips (id, name, description, start_date, end_date, status, created_by, created_at, updated_at, destination_latitude, destination_longitude)
		VALUES ($1, 'Test Trip', 'Test Description', NOW(), NOW() + INTERVAL '1 day', 'PLANNING', $2, NOW(), NOW(), 0.0, 0.0)`,
		tripID,
		userID,
	)
	require.NoError(t, err)

	// Create trip membership linking user and trip
	_, err = testPool.Exec(ctx, `
		INSERT INTO trip_memberships (trip_id, user_id, role, status, created_at, updated_at)
		VALUES ($1, $2, 'OWNER', 'ACTIVE', NOW(), NOW()),
		       ($1, $3, 'MEMBER', 'ACTIVE', NOW(), NOW())`,
		tripID,
		userID,
		anotherUserID,
	)
	require.NoError(t, err)

	return testPool, userID, anotherUserID, tripID
}

func TestChatStore(t *testing.T) {
	pool, userID, anotherUserID, tripID := setupTestDBWithChat(t)
	defer func() {
		if pool != nil {
			pool.Close()
		}
	}()

	ctx := context.Background()
	store := NewChatStore(pool)
	now := time.Now().UTC()

	t.Run("CreateChatGroup", func(t *testing.T) {
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Test Group for Create",
			CreatedBy: userID.String(),
			// CreatedAt/UpdatedAt are set by DB
		}

		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		assert.NotEmpty(t, groupID)

		// Verify the created group directly in DB
		var fetchedGroup struct {
			ID        string
			TripID    string
			Name      string
			CreatedBy string
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		selectQuery := `SELECT id, trip_id, name, created_by, created_at, updated_at FROM chat_groups WHERE id = $1`
		err = pool.QueryRow(ctx, selectQuery, groupID).Scan(
			&fetchedGroup.ID,
			&fetchedGroup.TripID,
			&fetchedGroup.Name,
			&fetchedGroup.CreatedBy,
			&fetchedGroup.CreatedAt,
			&fetchedGroup.UpdatedAt,
		)
		require.NoError(t, err, "Failed to fetch created group for verification")

		assert.Equal(t, groupID, fetchedGroup.ID)
		assert.Equal(t, group.TripID, fetchedGroup.TripID)
		assert.Equal(t, group.Name, fetchedGroup.Name)
		assert.Equal(t, group.CreatedBy, fetchedGroup.CreatedBy)
		assert.WithinDuration(t, now, fetchedGroup.CreatedAt, 5*time.Second) // Check creation time is recent
		assert.WithinDuration(t, now, fetchedGroup.UpdatedAt, 5*time.Second) // Check update time is recent
	})

	t.Run("AddChatGroupMember", func(t *testing.T) {
		// 1. Create a group first
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for Member Add Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)

		// 2. Add the creator as a member (should happen implicitly in service layer, but test store method here)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err)

		// 3. Verify membership in DB
		var count int
		countQuery := `SELECT COUNT(*) FROM chat_group_members WHERE group_id = $1 AND user_id = $2`
		err = pool.QueryRow(ctx, countQuery, groupID, userID.String()).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Creator should be added as a member")

		// 4. Add another member (anotherUserID)
		err = store.AddChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)

		// Verify the second member
		err = pool.QueryRow(ctx, countQuery, groupID, anotherUserID.String()).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Second member should be added")

		// 5. Add existing member again (should be idempotent)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err, "Adding an existing member should not error")

		// Verify count hasn't changed
		totalCountQuery := `SELECT COUNT(*) FROM chat_group_members WHERE group_id = $1`
		err = pool.QueryRow(ctx, totalCountQuery, groupID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "Adding existing member again should not increase count")

		// 6. Try adding member to non-existent group
		nonExistentGroupID := uuid.NewString()
		err = store.AddChatGroupMember(ctx, nonExistentGroupID, userID.String())
		require.Error(t, err, "Adding member to non-existent group should error")
		// TODO: Check for specific foreign key violation error if possible/desired
	})

	t.Run("CreateChatMessage", func(t *testing.T) {
		// Need a group first, placeholder for now
		groupID := uuid.New().String()
		message := types.ChatMessage{
			GroupID:   groupID,
			UserID:    userID.String(),
			Content:   "Hello world!",
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Expect error because not implemented yet
		_, err := store.CreateChatMessage(ctx, message)
		assert.Error(t, err)
		// Add specific implementation tests later...
	})

	// ... Test cases for GetChatGroup, UpdateChatGroup, DeleteChatGroup, ListChatGroupsByTrip ...
	// ... Test cases for GetChatMessageByID, UpdateChatMessage, DeleteChatMessage, ListChatMessages ...
	// ... Test cases for AddChatGroupMember, RemoveChatGroupMember, ListChatGroupMembers, UpdateLastReadMessage ...
	// ... Test cases for AddReaction, RemoveReaction, ListChatMessageReactions ...

	t.Run("ListChatMessageReactions", func(t *testing.T) {
		// 1. Setup: Group, members, message
		group := types.ChatGroup{TripID: tripID.String(), Name: "Group for List Reactions", CreatedBy: userID.String()}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)
		message := types.ChatMessage{GroupID: groupID, UserID: userID.String(), Content: "Message for Reaction List"}
		messageID, err := store.CreateChatMessage(ctx, message)
		require.NoError(t, err)

		// 2. Add reactions from different users
		reaction1 := "👍"
		reaction2 := "❤️"
		err = store.AddReaction(ctx, messageID, userID.String(), reaction1)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond) // Ensure order
		err = store.AddReaction(ctx, messageID, anotherUserID.String(), reaction2)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
		err = store.AddReaction(ctx, messageID, userID.String(), reaction2) // User 1 also adds reaction 2
		require.NoError(t, err)

		// 3. List reactions
		reactions, err := store.ListChatMessageReactions(ctx, messageID)
		require.NoError(t, err)
		require.Len(t, reactions, 3, "Should list 3 reactions")

		// 4. Verify reaction details and order (ASC by created_at)
		assert.Equal(t, messageID, reactions[0].MessageID)
		assert.Equal(t, userID.String(), reactions[0].UserID)
		assert.Equal(t, reaction1, reactions[0].Reaction)

		assert.Equal(t, messageID, reactions[1].MessageID)
		assert.Equal(t, anotherUserID.String(), reactions[1].UserID)
		assert.Equal(t, reaction2, reactions[1].Reaction)

		assert.Equal(t, messageID, reactions[2].MessageID)
		assert.Equal(t, userID.String(), reactions[2].UserID)
		assert.Equal(t, reaction2, reactions[2].Reaction)

		assert.True(t, reactions[1].CreatedAt.After(reactions[0].CreatedAt))
		assert.True(t, reactions[2].CreatedAt.After(reactions[1].CreatedAt))

		// 5. List reactions for message with no reactions
		messageNoReactions := types.ChatMessage{GroupID: groupID, UserID: userID.String(), Content: "No Reactions Msg"}
		noReactionsMsgID, err := store.CreateChatMessage(ctx, messageNoReactions)
		require.NoError(t, err)
		emptyReactions, err := store.ListChatMessageReactions(ctx, noReactionsMsgID)
		require.NoError(t, err)
		assert.Empty(t, emptyReactions, "Should return empty list for message with no reactions")
	})

	t.Run("GetUserByID", func(t *testing.T) {
		// 1. Test fetching the first user created in setup
		user1, err := store.GetUserByID(ctx, userID.String())
		require.NoError(t, err)
		require.NotNil(t, user1)
		assert.Equal(t, userID.String(), user1.ID)
		assert.Equal(t, "test@example.com", user1.Email)
		assert.Equal(t, "testuser", user1.Username)
		assert.Equal(t, "Test", user1.FirstName)
		assert.Equal(t, "User", user1.LastName)
		assert.Equal(t, "http://example.com/avatar1.png", user1.ProfilePictureURL)

		// 2. Test fetching the second user created in setup
		user2, err := store.GetUserByID(ctx, anotherUserID.String())
		require.NoError(t, err)
		require.NotNil(t, user2)
		assert.Equal(t, anotherUserID.String(), user2.ID)
		assert.Equal(t, "another@example.com", user2.Email)
		assert.Equal(t, "anotheruser", user2.Username)
		assert.Empty(t, user2.FirstName)
		assert.Empty(t, user2.LastName)
		assert.Equal(t, "http://example.com/avatar2.png", user2.ProfilePictureURL)

		// 3. Test fetching non-existent user
		nonExistentID := uuid.New().String()
		nilUser, err := store.GetUserByID(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, nilUser)
		assert.True(t, errors.Is(err, ErrNotFound), "Expected ErrNotFound for non-existent user")
	})

	t.Run("RemoveChatGroupMember", func(t *testing.T) {
		// 1. Create a group and add two members
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for Member Remove Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)

		// Verify initial member count
		var count int
		countQuery := `SELECT COUNT(*) FROM chat_group_members WHERE group_id = $1`
		err = pool.QueryRow(ctx, countQuery, groupID).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "Should start with 2 members")

		// 2. Remove one member (anotherUserID)
		err = store.RemoveChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)

		// Verify member count is now 1
		err = pool.QueryRow(ctx, countQuery, groupID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Member count should be 1 after removal")

		// Verify the correct member was removed
		var remainingUserID string
		memberQuery := `SELECT user_id FROM chat_group_members WHERE group_id = $1`
		err = pool.QueryRow(ctx, memberQuery, groupID).Scan(&remainingUserID)
		require.NoError(t, err)
		assert.Equal(t, userID.String(), remainingUserID, "Incorrect member was removed")

		// 3. Try removing a non-member (the one already removed)
		err = store.RemoveChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err, "Removing a non-member should not error (idempotent)")

		// Verify count is still 1
		err = pool.QueryRow(ctx, countQuery, groupID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Removing non-member should not change count")

		// 4. Try removing member from non-existent group
		nonExistentGroupID := uuid.NewString()
		err = store.RemoveChatGroupMember(ctx, nonExistentGroupID, userID.String())
		require.NoError(t, err, "Removing member from non-existent group should not error")
	})

	t.Run("ListChatGroupMembers", func(t *testing.T) {
		// 1. Create a group and add two members
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for List Members Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)

		// 2. List members
		members, err := store.ListChatGroupMembers(ctx, groupID)
		require.NoError(t, err)
		require.Len(t, members, 2, "Should list 2 members")

		// 3. Verify member details (using a map for easier lookup)
		memberMap := make(map[string]types.UserResponse)
		for _, m := range members {
			memberMap[m.ID] = m
		}

		// Check user 1 (userID)
		user1, ok1 := memberMap[userID.String()]
		require.True(t, ok1, "User 1 (creator) not found in member list")
		assert.Equal(t, "testuser", user1.Username, "User 1 username mismatch")
		assert.Equal(t, "http://example.com/avatar1.png", user1.AvatarURL, "User 1 avatar URL mismatch")

		// Check user 2 (anotherUserID)
		user2, ok2 := memberMap[anotherUserID.String()]
		require.True(t, ok2, "User 2 (other member) not found in member list")
		assert.Equal(t, "anotheruser", user2.Username, "User 2 username mismatch")
		assert.Equal(t, "http://example.com/avatar2.png", user2.AvatarURL, "User 2 avatar URL mismatch")

		// 4. List members for an empty group
		emptyGroup := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Empty Group for List Members Test",
			CreatedBy: userID.String(),
		}
		emptyGroupID, err := store.CreateChatGroup(ctx, emptyGroup)
		require.NoError(t, err)
		emptyMembers, err := store.ListChatGroupMembers(ctx, emptyGroupID)
		require.NoError(t, err)
		assert.Len(t, emptyMembers, 0, "Should return no members for an empty group")

		// 5. List members for a non-existent group
		nonExistentGroupID := uuid.NewString()
		nonExistentMembers, err := store.ListChatGroupMembers(ctx, nonExistentGroupID)
		require.NoError(t, err, "Listing members for non-existent group should not error")
		assert.Len(t, nonExistentMembers, 0, "Should return no members for non-existent group")
	})

	t.Run("UpdateLastReadMessage", func(t *testing.T) {
		// 1. Create group, add members, add messages
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for Last Read Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, userID.String())
		require.NoError(t, err)
		err = store.AddChatGroupMember(ctx, groupID, anotherUserID.String())
		require.NoError(t, err)

		msg1 := types.ChatMessage{GroupID: groupID, UserID: userID.String(), Content: "LR Test Msg 1"}
		msgID1, err := store.CreateChatMessage(ctx, msg1)
		require.NoError(t, err)
		msg2 := types.ChatMessage{GroupID: groupID, UserID: anotherUserID.String(), Content: "LR Test Msg 2"}
		msgID2, err := store.CreateChatMessage(ctx, msg2)
		require.NoError(t, err)

		// 2. Update last read for userID to msgID2
		err = store.UpdateLastReadMessage(ctx, groupID, userID.String(), msgID2)
		require.NoError(t, err)

		// 3. Verify last_read_message_id for userID in DB
		var lastReadMsgID pgtype.UUID
		selectQuery := `SELECT last_read_message_id FROM chat_group_members WHERE group_id = $1 AND user_id = $2`
		err = pool.QueryRow(ctx, selectQuery, groupID, userID.String()).Scan(&lastReadMsgID)
		require.NoError(t, err)
		require.Equal(t, pgtype.Present, lastReadMsgID.Status, "last_read_message_id should be set for userID")
		expectedMsgID2Bytes, _ := uuid.Parse(msgID2)                                                                      // Parse string UUID to bytes
		assert.Equal(t, expectedMsgID2Bytes, uuid.UUID(lastReadMsgID.Bytes), "Incorrect last_read_message_id for userID") // Compare bytes

		// 4. Verify last_read_message_id for anotherUserID is still NULL
		var lastReadMsgIDOther pgtype.UUID
		err = pool.QueryRow(ctx, selectQuery, groupID, anotherUserID.String()).Scan(&lastReadMsgIDOther)
		require.NoError(t, err)
		assert.Equal(t, pgtype.Null, lastReadMsgIDOther.Status, "last_read_message_id should be NULL for anotherUserID")

		// 5. Update for user not in group (use a new user ID)
		newUser := uuid.NewString()
		err = store.UpdateLastReadMessage(ctx, groupID, newUser, msgID1)
		assert.ErrorIs(t, err, ErrNotFound, "Updating last read for non-member should return ErrNotFound")

		// 6. Update for non-existent group
		nonExistentGroupID := uuid.NewString()
		err = store.UpdateLastReadMessage(ctx, nonExistentGroupID, userID.String(), msgID1)
		assert.ErrorIs(t, err, ErrNotFound, "Updating last read for non-existent group should return ErrNotFound")
	})

	t.Run("DeleteChatMessage", func(t *testing.T) {
		// 1. Create a group and a message
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for Delete Message Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		require.NotEmpty(t, groupID)

		message := types.ChatMessage{
			GroupID: groupID,
			UserID:  userID.String(),
			Content: "Message to be deleted",
		}
		messageID, err := store.CreateChatMessage(ctx, message)
		require.NoError(t, err)
		require.NotEmpty(t, messageID)

		// 2. Delete the message
		err = store.DeleteChatMessage(ctx, messageID)
		require.NoError(t, err)

		// 3. Verify it's marked as deleted in DB (check deleted_at)
		var deletedAt pgtype.Timestamp
		selectQuery := `SELECT deleted_at FROM chat_messages WHERE id = $1`
		err = pool.QueryRow(ctx, selectQuery, messageID).Scan(&deletedAt)
		require.NoError(t, err)
		assert.Equal(t, pgtype.Present, deletedAt.Status, "deleted_at should not be NULL")
		assert.WithinDuration(t, time.Now().UTC(), deletedAt.Time, 5*time.Second)

		// 4. Try to get the message, should return ErrNotFound
		_, err = store.GetChatMessageByID(ctx, messageID)
		assert.ErrorIs(t, err, ErrNotFound, "Getting deleted message should return ErrNotFound")

		// 5. Try deleting non-existent message
		err = store.DeleteChatMessage(ctx, uuid.NewString())
		assert.ErrorIs(t, err, ErrNotFound, "Deleting non-existent message should return ErrNotFound")

		// 6. Try deleting already deleted message
		err = store.DeleteChatMessage(ctx, messageID)
		assert.ErrorIs(t, err, ErrNotFound, "Deleting already deleted message should return ErrNotFound")
	})

	t.Run("ListChatMessages", func(t *testing.T) {
		// 1. Create a group
		listGroup := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for List Messages Test",
			CreatedBy: userID.String(),
		}
		listGroupID, err := store.CreateChatGroup(ctx, listGroup)
		require.NoError(t, err)
		require.NotEmpty(t, listGroupID)

		// 2. Create multiple messages (e.g., 5)
		numMessages := 5
		messageIDs := make([]string, numMessages)
		for i := 0; i < numMessages; i++ {
			msg := types.ChatMessage{
				GroupID: listGroupID,
				UserID:  userID.String(),
				Content: fmt.Sprintf("Message %d", i+1),
			}
			// Introduce slight delay to ensure distinct created_at for ordering test
			time.Sleep(10 * time.Millisecond)
			id, err := store.CreateChatMessage(ctx, msg)
			require.NoError(t, err)
			require.NotEmpty(t, id)
			messageIDs[i] = id
		}

		// 3. Test listing with different pagination
		// Case A: List first 2 messages
		paramsA := types.PaginationParams{Limit: 2, Offset: 0}
		messagesA, totalA, errA := store.ListChatMessages(ctx, listGroupID, paramsA)
		require.NoError(t, errA)
		assert.Equal(t, numMessages, totalA, "Total count mismatch case A")
		assert.Len(t, messagesA, 2, "Incorrect number of messages returned case A")
		assert.Equal(t, "Message 5", messagesA[0].Content, "Message order incorrect case A - oldest first?") // Assuming default order is created_at DESC
		assert.Equal(t, "Message 4", messagesA[1].Content, "Message order incorrect case A")

		// Case B: List next 2 messages
		paramsB := types.PaginationParams{Limit: 2, Offset: 2}
		messagesB, totalB, errB := store.ListChatMessages(ctx, listGroupID, paramsB)
		require.NoError(t, errB)
		assert.Equal(t, numMessages, totalB, "Total count mismatch case B")
		assert.Len(t, messagesB, 2, "Incorrect number of messages returned case B")
		assert.Equal(t, "Message 3", messagesB[0].Content, "Message order incorrect case B")
		assert.Equal(t, "Message 2", messagesB[1].Content, "Message order incorrect case B")

		// Case C: List last message
		paramsC := types.PaginationParams{Limit: 2, Offset: 4}
		messagesC, totalC, errC := store.ListChatMessages(ctx, listGroupID, paramsC)
		require.NoError(t, errC)
		assert.Equal(t, numMessages, totalC, "Total count mismatch case C")
		assert.Len(t, messagesC, 1, "Incorrect number of messages returned case C")
		assert.Equal(t, "Message 1", messagesC[0].Content, "Message order incorrect case C")

		// Case D: List all messages (high limit)
		paramsD := types.PaginationParams{Limit: 10, Offset: 0}
		messagesD, totalD, errD := store.ListChatMessages(ctx, listGroupID, paramsD)
		require.NoError(t, errD)
		assert.Equal(t, numMessages, totalD, "Total count mismatch case D")
		assert.Len(t, messagesD, numMessages, "Incorrect number of messages returned case D")

		// 4. Test listing for an empty group
		emptyGroup := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Empty Group for List Test",
			CreatedBy: userID.String(),
		}
		emptyGroupID, err := store.CreateChatGroup(ctx, emptyGroup)
		require.NoError(t, err)
		paramsEmpty := types.PaginationParams{Limit: 10, Offset: 0}
		messagesEmpty, totalEmpty, errEmpty := store.ListChatMessages(ctx, emptyGroupID, paramsEmpty)
		require.NoError(t, errEmpty)
		assert.Equal(t, 0, totalEmpty, "Total count mismatch for empty group")
		assert.Len(t, messagesEmpty, 0, "Should return no messages for empty group")

		// 5. Test listing for a non-existent group ID (should not error, just return empty/zero)
		nonExistentGroupID := uuid.NewString()
		paramsNonExistent := types.PaginationParams{Limit: 10, Offset: 0}
		messagesNonExistent, totalNonExistent, errNonExistent := store.ListChatMessages(ctx, nonExistentGroupID, paramsNonExistent)
		require.NoError(t, errNonExistent, "Listing messages for non-existent group should not error")
		assert.Equal(t, 0, totalNonExistent, "Total count mismatch for non-existent group")
		assert.Len(t, messagesNonExistent, 0, "Should return no messages for non-existent group")
	})

	t.Run("ChatMessageReactions", func(t *testing.T) {
		// 1. Create group and message
		group := types.ChatGroup{
			TripID:    tripID.String(),
			Name:      "Group for Reactions Test",
			CreatedBy: userID.String(),
		}
		groupID, err := store.CreateChatGroup(ctx, group)
		require.NoError(t, err)
		msg := types.ChatMessage{GroupID: groupID, UserID: userID.String(), Content: "Message for Reactions"}
		msgID, err := store.CreateChatMessage(ctx, msg)
		require.NoError(t, err)
		require.NotEmpty(t, msgID)

		reaction1 := "👍"
		reaction2 := "❤️"

		// 2. Add reaction1 by userID
		err = store.AddReaction(ctx, msgID, userID.String(), reaction1)
		require.NoError(t, err)

		// 3. Verify reaction1 exists in DB
		var count int
		countQuery := `SELECT COUNT(*) FROM chat_message_reactions WHERE message_id = $1 AND user_id = $2 AND reaction = $3`
		err = pool.QueryRow(ctx, countQuery, msgID, userID.String(), reaction1).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Reaction1 should exist in DB")

		// 4. Add reaction1 by userID again (idempotent)
		err = store.AddReaction(ctx, msgID, userID.String(), reaction1)
		require.NoError(t, err, "Adding same reaction again should not error")
		err = pool.QueryRow(ctx, countQuery, msgID, userID.String(), reaction1).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Reaction count should remain 1 after adding same reaction")

		// 5. Add reaction2 by anotherUserID
		err = store.AddReaction(ctx, msgID, anotherUserID.String(), reaction2)
		require.NoError(t, err)

		// 6. List reactions and verify both
		reactions, err := store.ListChatMessageReactions(ctx, msgID)
		require.NoError(t, err)
		require.Len(t, reactions, 2, "Should list 2 reactions")

		reactionMap := make(map[string]string) // Map user ID to reaction
		for _, r := range reactions {
			reactionMap[r.UserID] = r.Reaction
		}
		assert.Equal(t, reaction1, reactionMap[userID.String()], "Reaction1 mismatch")
		assert.Equal(t, reaction2, reactionMap[anotherUserID.String()], "Reaction2 mismatch")

		// 7. Remove reaction1
		err = store.RemoveReaction(ctx, msgID, userID.String(), reaction1)
		require.NoError(t, err)

		// 8. Verify reaction1 removed from DB
		err = pool.QueryRow(ctx, countQuery, msgID, userID.String(), reaction1).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Reaction1 should be removed from DB")

		// 9. List reactions again, verify only reaction2 remains
		reactions, err = store.ListChatMessageReactions(ctx, msgID)
		require.NoError(t, err)
		require.Len(t, reactions, 1, "Should list 1 reaction after removal")
		assert.Equal(t, reaction2, reactions[0].Reaction, "Remaining reaction should be reaction2")
		assert.Equal(t, anotherUserID.String(), reactions[0].UserID, "Remaining reaction user ID mismatch")

		// 10. Remove non-existent reaction (idempotent)
		err = store.RemoveReaction(ctx, msgID, userID.String(), "❓")
		require.NoError(t, err, "Removing non-existent reaction should not error")

		// 11. List reactions for a message with no reactions
		msgNoReactions := types.ChatMessage{GroupID: groupID, UserID: userID.String(), Content: "Message with No Reactions"}
		msgNoReactionsID, err := store.CreateChatMessage(ctx, msgNoReactions)
		require.NoError(t, err)
		reactionsEmpty, err := store.ListChatMessageReactions(ctx, msgNoReactionsID)
		require.NoError(t, err)
		assert.Len(t, reactionsEmpty, 0, "Should return empty list for message with no reactions")

		// 12. List reactions for non-existent message ID
		nonExistentMsgID := uuid.NewString()
		reactionsNonExistent, err := store.ListChatMessageReactions(ctx, nonExistentMsgID)
		require.NoError(t, err, "Listing reactions for non-existent message should not error")
		assert.Len(t, reactionsNonExistent, 0, "Should return empty list for non-existent message")

		// 13. Add reaction to non-existent message (should error - FK violation)
		err = store.AddReaction(ctx, nonExistentMsgID, userID.String(), "❓")
		require.Error(t, err, "Adding reaction to non-existent message should error")
		// TODO: Assert specific FK error if desired

		// 14. Add reaction by non-existent user (should error - FK violation)
		nonExistentUserID := uuid.NewString()
		err = store.AddReaction(ctx, msgID, nonExistentUserID, "❓")
		require.Error(t, err, "Adding reaction by non-existent user should error")
		// TODO: Assert specific FK error if desired
	})
}
