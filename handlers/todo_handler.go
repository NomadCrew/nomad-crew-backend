package handlers

import (
	"net/http"
	"strconv"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
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

// CreateTodoHandler
// Uses the trip ID from the parent route to create todos with correct association.
func (h *TodoHandler) CreateTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	var req types.TodoCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid request body", "error", err)
		if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Get trip ID from the parent route ("/trips/:id/todos")
	tripID := c.Param("id")
	if tripID == "" {
		log.Error("Trip ID missing in URL parameters")
		if err := c.Error(errors.ValidationFailed("Trip ID missing", "trip id is required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}
	// Override the TripID from the request to ensure consistency.
	req.TripID = tripID

	// Set the UserID from the authenticated user in the context
	userID := c.GetString(string(middleware.UserIDKey))

	todo := &types.Todo{
		TripID:    req.TripID,
		Text:      req.Text,
		CreatedBy: userID,
		Status:    types.TodoStatusIncomplete,
	}

	// Use CreateTodoWithEvent which handles both creation and event publishing
	todoID, err := h.todoModel.CreateTodoWithEvent(c.Request.Context(), todo)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add model error", "error", err)
		}
		return
	}

	// Ensure ID is set (should already be from model, but double-check)
	if todo.ID == "" {
		todo.ID = todoID
	}

	c.JSON(http.StatusCreated, todo)
}

// UpdateTodoHandler
// Extracts trip ID from the parent route and todo ID from c.Param("todoID").
func (h *TodoHandler) UpdateTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	todoID := c.Param("todoID")
	userID := c.GetString(string(middleware.UserIDKey))

	if todoID == "" {
		log.Error("Missing todo ID in URL parameters")
		if err := c.Error(errors.ValidationFailed("Missing parameters", "todo ID is required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	var req types.TodoUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid request body", "error", err)
		if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Use UpdateTodoWithEvent which handles both update and event publishing
	todo, err := h.todoModel.UpdateTodoWithEvent(c.Request.Context(), todoID, userID, &req)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to update todo", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, todo)
}

// DeleteTodoHandler
// Uses trip ID from c.Param("id") and todo ID from c.Param("todoID").
func (h *TodoHandler) DeleteTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	todoID := c.Param("todoID")
	userID := c.GetString(string(middleware.UserIDKey))

	if todoID == "" {
		log.Error("Missing todo ID in URL parameters")
		if err := c.Error(errors.ValidationFailed("Missing parameters", "todo ID is required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Use DeleteTodoWithEvent which handles both deletion and event publishing
	if err := h.todoModel.DeleteTodoWithEvent(c.Request.Context(), todoID, userID); err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to delete todo", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Todo deleted successfully",
	})
}

// getPaginationParams extracts and validates pagination parameters from the request
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

// ListTodosHandler retrieves todos for a given trip.
func (h *TodoHandler) ListTodosHandler(c *gin.Context) {
	log := logger.GetLogger()

	tripID := c.Param("id")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		log.Warn("ListTodosHandler: missing trip ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Parse pagination parameters
	params := getPaginationParams(c, 100, 0)

	todos, err := h.todoModel.ListTripTodos(c.Request.Context(), tripID, userID, params.Limit, params.Offset)
	if err != nil {
		log.Errorw("ListTodosHandler: error listing todos", "error", err, "tripID", tripID)
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, todos)
}

// GetTodoHandler
// Uses todo ID from the URL parameter.
func (h *TodoHandler) GetTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	todoID := c.Param("todoID")

	if todoID == "" {
		log.Error("Todo ID missing in URL parameters")
		if err := c.Error(errors.ValidationFailed("Todo ID missing", "todo id is required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to get todo", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, todo)
}
