package handlers

import (
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PaginationParams defines pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

type TodoHandler struct {
	todoModel    *models.TodoModel
	eventService types.EventPublisher
	logger       *zap.Logger
}

func NewTodoHandler(model *models.TodoModel, eventService types.EventPublisher, logger *zap.Logger) *TodoHandler {
	return &TodoHandler{
		todoModel:    model,
		eventService: eventService,
		logger:       logger,
	}
}

// CreateTodoHandler godoc
// @Summary Create a new todo item for a trip
// @Description Creates a new todo item associated with the specified trip.
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param request body types.TodoCreateRequest true "Todo details"
// @Success 201 {object} types.TodoResponse "Successfully created todo item"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid input data or missing Trip ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to create todos for this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/todos [post]
// @Security BearerAuth
// Uses the trip ID from the parent route to create todos with correct association.
func (h *TodoHandler) CreateTodoHandler(c *gin.Context) {
	var req types.TodoCreate
	if !bindJSONOrError(c, &req) {
		return
	}

	// Get trip ID from the parent route ("/trips/:id/todos")
	tripID := c.Param("id")
	if tripID == "" {
		_ = c.Error(errors.ValidationFailed("missing_trip_id", "trip ID is required"))
		return
	}
	// Override the TripID from the request to ensure consistency.
	req.TripID = tripID

	// Set the UserID from the authenticated user in the context (use Supabase UUID for created_by FK)
	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	todo := &types.Todo{
		TripID:    req.TripID,
		Text:      req.Text,
		CreatedBy: userID,
		Status:    types.TodoStatusIncomplete,
	}

	// Use CreateTodoWithEvent which handles both creation and event publishing
	todoID, err := h.todoModel.CreateTodoWithEvent(c.Request.Context(), todo)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// Ensure ID is set (should already be from model, but double-check)
	if todo.ID == "" {
		todo.ID = todoID
	}

	c.JSON(http.StatusCreated, todo)
}

// UpdateTodoHandler godoc
// @Summary Update an existing todo item
// @Description Updates the text and/or status of an existing todo item.
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param todoID path string true "Todo ID to update"
// @Param request body types.TodoUpdateRequest true "Fields to update"
// @Success 200 {object} types.TodoResponse "Successfully updated todo item"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid input data or IDs"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to update this todo"
// @Failure 404 {object} types.ErrorResponse "Not found - Todo item not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/todos/{todoID} [put]
// @Security BearerAuth
// Extracts trip ID from the parent route and todo ID from c.Param("todoID").
func (h *TodoHandler) UpdateTodoHandler(c *gin.Context) {
	todoID := c.Param("todoID")
	if todoID == "" {
		_ = c.Error(errors.ValidationFailed("missing_todo_id", "todo ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	var req types.TodoUpdate
	if !bindJSONOrError(c, &req) {
		return
	}

	// Use UpdateTodoWithEvent which handles both update and event publishing
	todo, err := h.todoModel.UpdateTodoWithEvent(c.Request.Context(), todoID, userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, todo)
}

// DeleteTodoHandler godoc
// @Summary Delete a todo item
// @Description Deletes a specific todo item.
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param todoID path string true "Todo ID to delete"
// @Success 200 {object} types.StatusResponse "Todo item deleted successfully"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid IDs"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to delete this todo"
// @Failure 404 {object} types.ErrorResponse "Not found - Todo item not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/todos/{todoID} [delete]
// @Security BearerAuth
// Uses trip ID from c.Param("id") and todo ID from c.Param("todoID").
func (h *TodoHandler) DeleteTodoHandler(c *gin.Context) {
	todoID := c.Param("todoID")
	if todoID == "" {
		_ = c.Error(errors.ValidationFailed("missing_todo_id", "todo ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Use DeleteTodoWithEvent which handles both deletion and event publishing
	if err := h.todoModel.DeleteTodoWithEvent(c.Request.Context(), todoID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Todo deleted successfully",
	})
}

// getPaginationParams extracts and validates pagination parameters from the request
// This is an internal helper and does not need Swagger annotations.
func getPaginationParams(c *gin.Context, defaultLimit, defaultOffset int) PaginationParams {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", strconv.Itoa(defaultOffset)))
	if err != nil || offset < 0 {
		offset = defaultOffset
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// ListTodosHandler godoc
// @Summary List todo items for a trip
// @Description Retrieves a list of todo items associated with the specified trip, with pagination.
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param limit query int false "Number of items to return (default 100)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Success 200 {array} types.TodoResponse "List of todo items"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid Trip ID or pagination parameters"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to view todos for this trip"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/todos [get]
// @Security BearerAuth
// ListTodosHandler retrieves todos for a given trip.
func (h *TodoHandler) ListTodosHandler(c *gin.Context) {
	tripID := c.Param("id")
	if tripID == "" {
		_ = c.Error(errors.ValidationFailed("missing_trip_id", "trip ID is required"))
		return
	}

	userID := getUserIDFromContext(c)
	if userID == "" {
		_ = c.Error(errors.Unauthorized("not_authenticated", "user not authenticated"))
		return
	}

	// Parse pagination parameters
	params := getPaginationParams(c, 100, 0)

	todos, err := h.todoModel.ListTripTodos(c.Request.Context(), tripID, userID, params.Limit, params.Offset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, todos)
}

// GetTodoHandler godoc
// @Summary Get a specific todo item
// @Description Retrieves details for a specific todo item.
// @Tags todos
// @Accept json
// @Produce json
// @Param id path string true "Trip ID"
// @Param todoID path string true "Todo ID to retrieve"
// @Success 200 {object} types.TodoResponse "Details of the todo item"
// @Failure 400 {object} types.ErrorResponse "Bad request - Invalid IDs"
// @Failure 401 {object} types.ErrorResponse "Unauthorized - User not logged in"
// @Failure 403 {object} types.ErrorResponse "Forbidden - User not authorized to view this todo"
// @Failure 404 {object} types.ErrorResponse "Not found - Todo item not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /trips/{id}/todos/{todoID} [get]
// @Security BearerAuth
// Uses todo ID from the URL parameter.
func (h *TodoHandler) GetTodoHandler(c *gin.Context) {
	todoID := c.Param("todoID")
	if todoID == "" {
		_ = c.Error(errors.ValidationFailed("missing_todo_id", "todo ID is required"))
		return
	}

	todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, todo)
}
