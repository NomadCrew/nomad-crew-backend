package handlers

import (
    "net/http"
    "encoding/json"
	"time"

    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/models"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/gin-gonic/gin"
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

func (h *TodoHandler) CreateTodoHandler(c *gin.Context) {
    log := logger.GetLogger()
    
    var req types.TodoCreate
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid request body", "error", err)
        c.Error(errors.ValidationFailed("Invalid request body", err.Error()))
        return
    }

    userID := c.GetString("user_id")
    
    todo := &types.Todo{
        TripID:    req.TripID,
        Text:      req.Text,
        CreatedBy: userID,
        Status:    types.TodoStatusIncomplete,
    }

    if err := h.todoModel.CreateTodo(c.Request.Context(), todo); err != nil {
        c.Error(err)
        return
    }

    // Publish event
    payload, _ := json.Marshal(todo)
    h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoCreated,
        Payload: payload,
    })

    c.JSON(http.StatusCreated, todo)
}

func (h *TodoHandler) UpdateTodoHandler(c *gin.Context) {
    log := logger.GetLogger()
    todoID := c.Param("id")
    userID := c.GetString("user_id")

    var req types.TodoUpdate
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Errorw("Invalid request body", "error", err)
        c.Error(errors.ValidationFailed("Invalid request body", err.Error()))
        return
    }

    if err := h.todoModel.UpdateTodo(c.Request.Context(), todoID, userID, &req); err != nil {
        c.Error(err)
        return
    }

    // Get updated todo for event payload
    todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
    if err != nil {
        c.Error(err)
        return
    }

    // Publish event
    payload, _ := json.Marshal(todo)
    h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoUpdated,
        Payload: payload,
    })

    c.JSON(http.StatusOK, todo)
}

func (h *TodoHandler) DeleteTodoHandler(c *gin.Context) {
    todoID := c.Param("id")
    userID := c.GetString("user_id")

    // Get todo for event payload before deletion
    todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
    if err != nil {
        c.Error(err)
        return
    }

    if err := h.todoModel.DeleteTodo(c.Request.Context(), todoID, userID); err != nil {
        c.Error(err)
        return
    }

    // Publish event
    payload, _ := json.Marshal(map[string]string{"id": todoID})
    h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoDeleted,
        Payload: payload,
    })

    c.JSON(http.StatusOK, gin.H{
        "message": "Todo deleted successfully",
    })
}

func (h *TodoHandler) ListTodosHandler(c *gin.Context) {
    tripID := c.Param("tripId")
    userID := c.GetString("user_id")

    todos, err := h.todoModel.ListTripTodos(c.Request.Context(), tripID, userID)
    if err != nil {
        c.Error(err)
        return
    }

    c.JSON(http.StatusOK, todos)
}

func (h *TodoHandler) StreamTodoEvents(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("tripId")
    userID := c.GetString("user_id")

    // Verify trip access
    if _, err := h.todoModel.ListTripTodos(c.Request.Context(), tripID, userID); err != nil {
        c.Error(errors.AuthenticationFailed("Not authorized to access this trip's todos"))
        return
    }

    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Transfer-Encoding", "chunked")

    // Subscribe to trip's todo events
    eventChan, err := h.eventService.Subscribe(c.Request.Context(), tripID)
    if err != nil {
        log.Errorw("Failed to subscribe to todo events",
            "tripId", tripID,
            "error", err,
        )
        c.Error(err)
        return
    }

    // Send keep-alive pulses and handle events
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-c.Request.Context().Done():
            log.Debugw("Client disconnected",
                "tripId", tripID,
                "userId", userID,
            )
            return

        case <-ticker.C:
            // Send keep-alive
            c.SSEvent("ping", nil)
            c.Writer.Flush()

        case event, ok := <-eventChan:
            if !ok {
                log.Debugw("Event channel closed",
                    "tripId", tripID,
                    "userId", userID,
                )
                return
            }

            // Only send todo-related events
            switch event.Type {
            case types.EventTypeTodoCreated,
                 types.EventTypeTodoUpdated,
                 types.EventTypeTodoDeleted:
                c.SSEvent("event", event)
                c.Writer.Flush()
            }
        }
    }
}