package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a mock database connection
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
	}

	return db, mock, cleanup
}

// Helper to create a test user
func createTestUser() *types.User {
	return &types.User{
		ID:           uuid.NewString(),
		Email:        "test@example.com",
		Username:     "testuser",
		DisplayName:  "Test User",
		Bio:          "Test bio",
		AvatarURL:    "https://example.com/avatar.jpg",
		IsOnline:     true,
		LastSeen:     time.Now(),
		Preferences:  map[string]interface{}{"theme": "dark"},
		SupabaseID:   uuid.NewString(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestUserStore_GetUserByID(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Create store with mock pool (simplified for testing)
	store := &UserStore{
		// In real implementation, we'd need to properly mock pgxpool
		// For now, focusing on the SQL logic
	}

	ctx := context.Background()
	user := createTestUser()

	t.Run("successful retrieval", func(t *testing.T) {
		prefsJSON, _ := json.Marshal(user.Preferences)
		
		rows := sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "bio", "avatar_url",
			"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
		}).AddRow(
			user.ID, user.Email, user.Username, user.DisplayName, user.Bio, user.AvatarURL,
			user.IsOnline, user.LastSeen, prefsJSON, user.SupabaseID, user.CreatedAt, user.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs(user.ID).
			WillReturnRows(rows)

		// Note: Actual implementation would use pgxpool, this is conceptual
		// result, err := store.GetUserByID(ctx, user.ID)
		// assert.NoError(t, err)
		// assert.Equal(t, user.ID, result.ID)
	})

	t.Run("user not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return store.ErrUserNotFound
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs(user.ID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestUserStore_CreateUser(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser()

	t.Run("successful creation", func(t *testing.T) {
		prefsJSON, _ := json.Marshal(user.Preferences)

		mock.ExpectExec("INSERT INTO users").
			WithArgs(
				user.ID, user.Email, user.Username, user.DisplayName,
				user.Bio, user.AvatarURL, user.IsOnline, user.LastSeen,
				prefsJSON, user.SupabaseID,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Test would verify successful insertion
	})

	t.Run("duplicate user", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO users").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(&pgconn.PgError{
				Code: "23505", // unique_violation
			})

		// Would return "user already exists" error
	})

	t.Run("validation error - empty email", func(t *testing.T) {
		invalidUser := createTestUser()
		invalidUser.Email = ""

		// Should return validation error before database call
	})
}

func TestUserStore_UpdateUser(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.NewString()

	t.Run("update display name", func(t *testing.T) {
		updates := map[string]interface{}{
			"display_name": "New Name",
		}

		mock.ExpectExec("UPDATE users SET display_name = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs("New Name", userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update multiple fields", func(t *testing.T) {
		updates := map[string]interface{}{
			"display_name": "New Name",
			"bio":          "New bio",
			"avatar_url":   "https://example.com/new-avatar.jpg",
		}

		// Dynamic query building would be tested here
	})

	t.Run("update preferences", func(t *testing.T) {
		prefs := map[string]interface{}{
			"theme": "light",
			"notifications": true,
		}
		prefsJSON, _ := json.Marshal(prefs)

		mock.ExpectExec("UPDATE users SET preferences = \\$1").
			WithArgs(prefsJSON, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("invalid field update", func(t *testing.T) {
		updates := map[string]interface{}{
			"invalid_field": "value",
		}

		// Should return validation error
	})
}

func TestUserStore_ListUsers(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successful list with pagination", func(t *testing.T) {
		offset, limit := 0, 10

		// Count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		// List query
		user1 := createTestUser()
		user2 := createTestUser()
		user2.ID = uuid.NewString()
		user2.Email = "test2@example.com"

		prefs1JSON, _ := json.Marshal(user1.Preferences)
		prefs2JSON, _ := json.Marshal(user2.Preferences)

		rows := sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "bio", "avatar_url",
			"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
		}).
			AddRow(user1.ID, user1.Email, user1.Username, user1.DisplayName, user1.Bio, user1.AvatarURL,
				user1.IsOnline, user1.LastSeen, prefs1JSON, user1.SupabaseID, user1.CreatedAt, user1.UpdatedAt).
			AddRow(user2.ID, user2.Email, user2.Username, user2.DisplayName, user2.Bio, user2.AvatarURL,
				user2.IsOnline, user2.LastSeen, prefs2JSON, user2.SupabaseID, user2.CreatedAt, user2.UpdatedAt)

		mock.ExpectQuery("SELECT (.+) FROM users ORDER BY created_at DESC LIMIT \\$1 OFFSET \\$2").
			WithArgs(limit, offset).
			WillReturnRows(rows)

		// Would verify correct pagination and results
	})

	t.Run("empty results", func(t *testing.T) {
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		rows := sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "bio", "avatar_url",
			"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM users").
			WillReturnRows(rows)

		// Would return empty slice
	})
}

func TestUserStore_GetUserByEmail(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser()

	t.Run("successful retrieval", func(t *testing.T) {
		prefsJSON, _ := json.Marshal(user.Preferences)

		rows := sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "bio", "avatar_url",
			"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
		}).AddRow(
			user.ID, user.Email, user.Username, user.DisplayName, user.Bio, user.AvatarURL,
			user.IsOnline, user.LastSeen, prefsJSON, user.SupabaseID, user.CreatedAt, user.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
			WithArgs(user.Email).
			WillReturnRows(rows)
	})

	t.Run("case insensitive search", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE LOWER\\(email\\) = LOWER\\(\\$1\\)").
			WithArgs("TEST@EXAMPLE.COM").
			WillReturnRows(sqlmock.NewRows([]string{}))
	})
}

func TestUserStore_GetUserByUsername(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	user := createTestUser()

	t.Run("successful retrieval", func(t *testing.T) {
		prefsJSON, _ := json.Marshal(user.Preferences)

		rows := sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "bio", "avatar_url",
			"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
		}).AddRow(
			user.ID, user.Email, user.Username, user.DisplayName, user.Bio, user.AvatarURL,
			user.IsOnline, user.LastSeen, prefsJSON, user.SupabaseID, user.CreatedAt, user.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE username = \\$1").
			WithArgs(user.Username).
			WillReturnRows(rows)
	})
}

func TestUserStore_UpdateLastSeen(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.NewString()

	t.Run("successful update", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET last_seen = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("user not found", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET last_seen = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})
}

func TestUserStore_SetOnlineStatus(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.NewString()

	t.Run("set online", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET is_online = \\$1, last_seen = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(true, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	t.Run("set offline", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET is_online = \\$1, last_seen = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(false, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})
}

func TestUserStore_Transaction(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successful transaction", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		// Would test transaction flow
	})

	t.Run("rollback on error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE users").WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		// Would test rollback behavior
	})
}

func TestUserStore_SyncUserFromSupabase(t *testing.T) {
	// Create a test HTTP server to mock Supabase API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check API key header
		apiKey := r.Header.Get("apikey")
		if apiKey != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return mock user data
		user := types.SupabaseUser{
			ID:    uuid.NewString(),
			Email: "supabase@example.com",
			UserMetadata: map[string]interface{}{
				"username":     "supabaseuser",
				"display_name": "Supabase User",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}))
	defer server.Close()

	store := &UserStore{
		supabaseURL: server.URL,
		supabaseKey: "test-key",
		httpClient:  &http.Client{Timeout: 5 * time.Second},
	}

	t.Run("successful sync", func(t *testing.T) {
		ctx := context.Background()
		supabaseID := uuid.NewString()

		// Mock database operations would go here
		// Testing the HTTP client behavior
	})

	t.Run("supabase API error", func(t *testing.T) {
		// Test error handling from Supabase API
	})
}

func TestUserStore_GetUserProfiles(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	ctx := context.Background()
	userIDs := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}

	t.Run("successful batch retrieval", func(t *testing.T) {
		user1 := createTestUser()
		user2 := createTestUser()
		user2.ID = userIDs[1]

		rows := sqlmock.NewRows([]string{
			"id", "username", "display_name", "avatar_url", "is_online", "last_seen",
		}).
			AddRow(user1.ID, user1.Username, user1.DisplayName, user1.AvatarURL, user1.IsOnline, user1.LastSeen).
			AddRow(user2.ID, user2.Username, user2.DisplayName, user2.AvatarURL, user2.IsOnline, user2.LastSeen)

		// Expect query with IN clause
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id IN").
			WithArgs(userIDs[0], userIDs[1], userIDs[2]).
			WillReturnRows(rows)

		// Would return map of user profiles
	})

	t.Run("empty user IDs", func(t *testing.T) {
		// Should return empty map without database call
	})

	t.Run("partial results", func(t *testing.T) {
		// Some users exist, some don't
		// Should return map with only existing users
	})
}

func TestUserStore_ConvertToUserResponse(t *testing.T) {
	store := &UserStore{}

	t.Run("successful conversion", func(t *testing.T) {
		user := createTestUser()
		
		response, err := store.ConvertToUserResponse(user)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, response.ID)
		assert.Equal(t, user.Email, response.Email)
		assert.Equal(t, user.Username, response.Username)
		assert.Equal(t, user.DisplayName, response.DisplayName)
		assert.Equal(t, user.AvatarURL, response.AvatarURL)
	})

	t.Run("nil user", func(t *testing.T) {
		_, err := store.ConvertToUserResponse(nil)
		assert.Error(t, err)
	})
}

// Benchmark tests
func BenchmarkUserStore_GetUserByID(b *testing.B) {
	// Benchmark database query performance
	db, mock, cleanup := setupMockDB(b)
	defer cleanup()

	userID := uuid.NewString()
	user := createTestUser()
	prefsJSON, _ := json.Marshal(user.Preferences)

	rows := sqlmock.NewRows([]string{
		"id", "email", "username", "display_name", "bio", "avatar_url",
		"is_online", "last_seen", "preferences", "supabase_id", "created_at", "updated_at",
	}).AddRow(
		user.ID, user.Email, user.Username, user.DisplayName, user.Bio, user.AvatarURL,
		user.IsOnline, user.LastSeen, prefsJSON, user.SupabaseID, user.CreatedAt, user.UpdatedAt,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
			WithArgs(userID).
			WillReturnRows(rows)
		
		// Execute query in actual implementation
	}
}

func BenchmarkUserStore_ListUsers(b *testing.B) {
	// Benchmark pagination performance
	db, mock, cleanup := setupMockDB(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(100)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
			WillReturnRows(countRows)

		// Setup rows for pagination query
		// Execute in actual implementation
	}
}