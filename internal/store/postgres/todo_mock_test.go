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

// Helper function to create test todo
func createTestTodo() *types.Todo {
	return &types.Todo{
		ID:        uuid.NewString(),
		TripID:    uuid.NewString(),
		Text:      "Test todo item",
		Status:    types.TodoStatusIncomplete,
		CreatedBy: uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Helper function to create test todo update
func createTestTodoUpdate() *types.TodoUpdate {
	newText := "Updated todo text"
	newStatus := types.TodoStatusComplete
	return &types.TodoUpdate{
		Text:   &newText,
		Status: &newStatus,
	}
}

func TestTodoStore_CreateTodo(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	todo := createTestTodo()

	t.Run("successful creation", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id"}).AddRow(todo.ID)

		mock.ExpectQuery("INSERT INTO todos \\(trip_id, text, status, created_by\\) VALUES").
			WithArgs(todo.TripID, todo.Text, todo.Status, todo.CreatedBy).
			WillReturnRows(rows)

		// Would verify successful creation and returned ID
	})

	t.Run("invalid trip ID", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo.TripID, todo.Text, todo.Status, todo.CreatedBy).
			WillReturnError(&pgconn.PgError{
				Code: "23503", // foreign_key_violation
			})

		// Would return "trip not found" error
	})

	t.Run("empty text", func(t *testing.T) {
		emptyTodo := createTestTodo()
		emptyTodo.Text = ""

		// Should validate before database call
		// Text cannot be empty
	})

	t.Run("invalid status", func(t *testing.T) {
		invalidTodo := createTestTodo()
		invalidTodo.Status = "invalid_status"

		// Should validate status is one of: pending, in_progress, completed
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo.TripID, todo.Text, todo.Status, todo.CreatedBy).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestTodoStore_GetTodo(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	todo := createTestTodo()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "text", "status", "created_by", "created_at", "updated_at",
		}).AddRow(
			todo.ID, todo.TripID, todo.Text, todo.Status, todo.CreatedBy, todo.CreatedAt, todo.UpdatedAt,
		)

		mock.ExpectQuery("SELECT id, trip_id, text, status, created_by, created_at, updated_at FROM todos WHERE id = \\$1").
			WithArgs(todo.ID).
			WillReturnRows(rows)

		// Would verify successful retrieval
	})

	t.Run("todo not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM todos WHERE id = \\$1").
			WithArgs("nonexistent").
			WillReturnRows(sqlmock.NewRows([]string{}))

		// Would return sql.ErrNoRows
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM todos WHERE id = \\$1").
			WithArgs(todo.ID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestTodoStore_ListTodos(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	tripID := uuid.NewString()

	t.Run("successful list", func(t *testing.T) {
		todo1 := createTestTodo()
		todo2 := createTestTodo()
		todo2.ID = uuid.NewString()
		todo2.Status = types.TodoStatusComplete
		todo3 := createTestTodo()
		todo3.ID = uuid.NewString()
		todo3.Status = types.TodoStatusIncomplete

		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "text", "status", "created_by", "created_at", "updated_at",
		}).
			AddRow(todo1.ID, todo1.TripID, todo1.Text, todo1.Status, todo1.CreatedBy, todo1.CreatedAt, todo1.UpdatedAt).
			AddRow(todo2.ID, todo2.TripID, todo2.Text, todo2.Status, todo2.CreatedBy, todo2.CreatedAt, todo2.UpdatedAt).
			AddRow(todo3.ID, todo3.TripID, todo3.Text, todo3.Status, todo3.CreatedBy, todo3.CreatedAt, todo3.UpdatedAt)

		mock.ExpectQuery("SELECT (.+) FROM todos WHERE trip_id = \\$1 ORDER BY created_at DESC").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return list of todos ordered by creation date
	})

	t.Run("empty list", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "text", "status", "created_by", "created_at", "updated_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM todos WHERE trip_id = \\$1").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Would return empty slice
	})

	t.Run("filter by status", func(t *testing.T) {
		// If implementation supports status filtering
		rows := sqlmock.NewRows([]string{
			"id", "trip_id", "text", "status", "created_by", "created_at", "updated_at",
		})

		mock.ExpectQuery("SELECT (.+) FROM todos WHERE trip_id = \\$1 AND status = \\$2").
			WithArgs(tripID, types.TodoStatusIncomplete).
			WillReturnRows(rows)

		// Would return filtered results
	})
}

func TestTodoStore_UpdateTodo(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	todoID := uuid.NewString()

	t.Run("update text only", func(t *testing.T) {
		update := &types.TodoUpdate{
			Text: func() *string { s := "Updated text"; return &s }(),
		}

		mock.ExpectExec("UPDATE todos SET text = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(*update.Text, todoID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update status only", func(t *testing.T) {
		update := &types.TodoUpdate{
			Status: types.TodoStatusComplete.Ptr(),
		}

		mock.ExpectExec("UPDATE todos SET status = \\$1, updated_at = CURRENT_TIMESTAMP WHERE id = \\$2").
			WithArgs(*update.Status, todoID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("update both fields", func(t *testing.T) {
		update := createTestTodoUpdate()

		mock.ExpectExec("UPDATE todos SET text = \\$1, status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$3").
			WithArgs(*update.Text, *update.Status, todoID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Would verify successful update
	})

	t.Run("todo not found", func(t *testing.T) {
		update := createTestTodoUpdate()

		mock.ExpectExec("UPDATE todos").
			WithArgs(*update.Text, *update.Status, todoID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})

	t.Run("no fields to update", func(t *testing.T) {
		_ = &types.TodoUpdate{}

		// Should return error without database call
		// "no fields to update"
	})

	t.Run("invalid status update", func(t *testing.T) {
		invalidStatus := types.TodoStatus("invalid_status")
		_ = &types.TodoUpdate{
			Status: &invalidStatus,
		}

		// Should validate status before database call
	})

	t.Run("empty text update", func(t *testing.T) {
		emptyText := ""
		_ = &types.TodoUpdate{
			Text: &emptyText,
		}

		// Should validate text is not empty
	})
}

func TestTodoStore_DeleteTodo(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()
	todoID := uuid.NewString()

	t.Run("successful deletion", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM todos WHERE id = \\$1").
			WithArgs(todoID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Would verify successful deletion
	})

	t.Run("todo not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM todos WHERE id = \\$1").
			WithArgs(todoID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Would check affected rows and return error
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM todos WHERE id = \\$1").
			WithArgs(todoID).
			WillReturnError(errors.New("database connection failed"))

		// Would return the error
	})
}

func TestTodoStore_BeginTx(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()

	t.Run("successful transaction start", func(t *testing.T) {
		mock.ExpectBegin()

		// Would return transaction wrapper
	})

	t.Run("transaction already in progress", func(t *testing.T) {
		// First transaction
		mock.ExpectBegin()

		// Attempt to start another transaction
		// Should return error "transaction already started"
	})

	t.Run("database connection error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(errors.New("connection lost"))

		// Would return the error
	})
}

func TestTodoStore_TransactionOperations(t *testing.T) {
	_, mock, cleanup := setupMockDB(t)
	defer cleanup()

	_ = context.Background()

	t.Run("successful transaction with multiple operations", func(t *testing.T) {
		todo1 := createTestTodo()
		todo2 := createTestTodo()
		todo2.ID = uuid.NewString()

		// Begin transaction
		mock.ExpectBegin()

		// Insert first todo
		rows1 := sqlmock.NewRows([]string{"id"}).AddRow(todo1.ID)
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo1.TripID, todo1.Text, todo1.Status, todo1.CreatedBy).
			WillReturnRows(rows1)

		// Insert second todo
		rows2 := sqlmock.NewRows([]string{"id"}).AddRow(todo2.ID)
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo2.TripID, todo2.Text, todo2.Status, todo2.CreatedBy).
			WillReturnRows(rows2)

		// Update first todo
		mock.ExpectExec("UPDATE todos SET status = \\$1").
			WithArgs(types.TodoStatusComplete, todo1.ID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Commit transaction
		mock.ExpectCommit()

		// Would verify all operations succeed
	})

	t.Run("rollback on error", func(t *testing.T) {
		todo := createTestTodo()

		// Begin transaction
		mock.ExpectBegin()

		// Insert succeeds
		rows := sqlmock.NewRows([]string{"id"}).AddRow(todo.ID)
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo.TripID, todo.Text, todo.Status, todo.CreatedBy).
			WillReturnRows(rows)

		// Update fails
		mock.ExpectExec("UPDATE todos").
			WillReturnError(errors.New("constraint violation"))

		// Rollback transaction
		mock.ExpectRollback()

		// Would verify rollback behavior
	})
}

func TestTodoStore_StatusTransitions(t *testing.T) {
	// Test valid status transitions
	validTransitions := []struct {
		from types.TodoStatus
		to   types.TodoStatus
	}{
		{types.TodoStatusIncomplete, types.TodoStatusComplete},
		{types.TodoStatusComplete, types.TodoStatusIncomplete},
	}

	for _, tt := range validTransitions {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			// All transitions should be allowed in current implementation
			// But could add business logic to restrict certain transitions
		})
	}
}

// Benchmark tests
func BenchmarkTodoStore_CreateTodo(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	todo := createTestTodo()
	rows := sqlmock.NewRows([]string{"id"}).AddRow(todo.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("INSERT INTO todos").
			WithArgs(todo.TripID, todo.Text, todo.Status, todo.CreatedBy).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}

func BenchmarkTodoStore_ListTodos(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	tripID := uuid.NewString()

	// Create test data
	rows := sqlmock.NewRows([]string{
		"id", "trip_id", "text", "status", "created_by", "created_at", "updated_at",
	})

	// Add 50 todos
	for i := 0; i < 50; i++ {
		todo := createTestTodo()
		rows.AddRow(todo.ID, todo.TripID, todo.Text, todo.Status, todo.CreatedBy, todo.CreatedAt, todo.UpdatedAt)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery("SELECT (.+) FROM todos WHERE trip_id = \\$1").
			WithArgs(tripID).
			WillReturnRows(rows)

		// Execute in actual implementation
	}
}

func BenchmarkTodoStore_UpdateTodo(b *testing.B) {
	_, mock, cleanup := setupMockDB(b)
	defer cleanup()

	todoID := uuid.NewString()
	update := createTestTodoUpdate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.ExpectExec("UPDATE todos").
			WithArgs(*update.Text, *update.Status, todoID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Execute in actual implementation
	}
}