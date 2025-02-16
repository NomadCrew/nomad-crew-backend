package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/models"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TodoHandler struct {
	todoModel    *models.TodoModel
	eventService types.EventPublisher
}

func NewTodoHandler(model *models.TodoModel, eventService types.EventPublisher) *TodoHandler {
	return &TodoHandler{
		todoModel:    model,
		eventService: eventService,
	}
}

// CreateTodoHandler
// Uses the trip ID from the parent route (c.Param("id")).
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

	userID := c.GetString("user_id")

	todo := &types.Todo{
		TripID:    req.TripID,
		Text:      req.Text,
		CreatedBy: userID,
		Status:    types.TodoStatusIncomplete,
	}

	if err := h.todoModel.CreateTodo(c.Request.Context(), todo); err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add model error", "error", err)
		}
		return
	}

	// Publish event
	payload, _ := json.Marshal(todo)
	log.Debugw("Todo created, publishing event",
		"todoID", todo.ID,
		"tripID", todo.TripID,
		"payloadSize", len(payload))

	if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.NewString(),
			Type:      types.EventTypeTodoCreated,
			TripID:    todo.TripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "todo_handler",
		},
		Payload: payload,
	}); err != nil {
		log.Errorw("Failed to publish todo created event",
			"error", err,
			"tripID", todo.TripID)
	}

	c.JSON(http.StatusCreated, todo)
}

// UpdateTodoHandler
// Extracts trip ID from the parent route and todo ID from c.Param("todoId").
func (h *TodoHandler) UpdateTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	todoID := c.Param("todoId")
	userID := c.GetString("user_id")

	if tripID == "" || todoID == "" {
		log.Error("Missing trip ID or todo ID in URL parameters")
		if err := c.Error(errors.ValidationFailed("Missing parameters", "trip ID and todo ID are required")); err != nil {
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

	if err := h.todoModel.UpdateTodo(c.Request.Context(), todoID, userID, &req); err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to update todo", "error", err)
		}
		return
	}

	// Get updated todo for event payload
	todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to retrieve updated todo", "error", err)
		}
		return
	}

	// Publish event
	payload, _ := json.Marshal(todo)
	if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.NewString(),
			Type:      types.EventTypeTodoUpdated,
			TripID:    todo.TripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "todo_handler",
		},
		Payload: payload,
	}); err != nil {
		log.Errorw("Failed to publish todo updated event", "error", err)
	}

	c.JSON(http.StatusOK, todo)
}

// DeleteTodoHandler
// Uses trip ID from c.Param("id") and todo ID from c.Param("todoId").
func (h *TodoHandler) DeleteTodoHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	todoID := c.Param("todoId")
	userID := c.GetString("user_id")

	if tripID == "" || todoID == "" {
		log.Error("Missing trip ID or todo ID in URL parameters")
		if err := c.Error(errors.ValidationFailed("Missing parameters", "trip ID and todo ID are required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Get todo for event payload before deletion
	todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to retrieve todo", "error", err)
		}
		return
	}

	if err := h.todoModel.DeleteTodo(c.Request.Context(), todoID, userID); err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to delete todo", "error", err)
		}
		return
	}

	// Publish event
	payload, _ := json.Marshal(map[string]string{"id": todoID})
	if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.NewString(),
			Type:      types.EventTypeTodoDeleted,
			TripID:    todo.TripID,
			UserID:    userID,
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "todo_handler",
		},
		Payload: payload,
	}); err != nil {
		log.Errorw("Failed to publish todo deleted event", "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Todo deleted successfully",
	})
}

// ListTodosHandler
// Uses trip ID from the parent route.
func (h *TodoHandler) ListTodosHandler(c *gin.Context) {
	log := logger.GetLogger()
	var params types.ListTodosParams
	if err := c.ShouldBindQuery(&params); err != nil {
		log.Errorw("Invalid query parameters", "error", err)
		if err := c.Error(errors.ValidationFailed("Invalid query parameters", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Validate and adjust limits
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	tripID := c.Param("id")
	if tripID == "" {
		log.Error("Trip ID missing in URL parameters")
		if err := c.Error(errors.ValidationFailed("Trip ID missing", "trip id is required")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}
	userID := c.GetString("user_id")

	response, err := h.todoModel.ListTripTodos(
		c.Request.Context(),
		tripID,
		userID,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		log.Errorw("Failed to list todos", "error", err)
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add model error", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, response)
}
