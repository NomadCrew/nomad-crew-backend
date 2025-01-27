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
        if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
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
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Publish event
    payload, _ := json.Marshal(todo)
    if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoCreated,
        Payload: payload,
    }); err != nil {
        log.Errorw("Failed to publish todo created event", "error", err)
    }

    c.JSON(http.StatusCreated, todo)
}

func (h *TodoHandler) UpdateTodoHandler(c *gin.Context) {
    log := logger.GetLogger()
    todoID := c.Param("id")
    userID := c.GetString("user_id")

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
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Get updated todo for event payload
    todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Publish event
    payload, _ := json.Marshal(todo)
    if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoUpdated,
        Payload: payload,
    }); err != nil {
        log.Errorw("Failed to publish todo updated event", "error", err)
    }

    c.JSON(http.StatusOK, todo)
}

func (h *TodoHandler) DeleteTodoHandler(c *gin.Context) {
    log := logger.GetLogger()
    todoID := c.Param("id")
    userID := c.GetString("user_id")

    // Get todo for event payload before deletion
    todo, err := h.todoModel.GetTodo(c.Request.Context(), todoID)
    if err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    if err := h.todoModel.DeleteTodo(c.Request.Context(), todoID, userID); err != nil {
        if err := c.Error(err); err != nil {
            log.Errorw("Failed to add model error", "error", err)
        }
        return
    }

    // Publish event
    payload, _ := json.Marshal(map[string]string{"id": todoID})
    if err := h.eventService.Publish(c.Request.Context(), todo.TripID, types.Event{
        Type:    types.EventTypeTodoDeleted,
        Payload: payload,
    }); err != nil {
        log.Errorw("Failed to publish todo deleted event", "error", err)
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Todo deleted successfully",
    })
}

func (h *TodoHandler) ListTodosHandler(c *gin.Context) {
    var params types.ListTodosParams
    if err := c.ShouldBindQuery(&params); err != nil {
        log := logger.GetLogger()
        log.Errorw("Invalid query parameters", "error", err)
        c.Error(errors.ValidationFailed("invalid query parameters", err.Error()))
        return
    }

    // Validate and adjust limits
    if params.Limit <= 0 || params.Limit > 100 {
        params.Limit = 20
    }
    if params.Offset < 0 {
        params.Offset = 0
    }

    tripID := c.Param("tripId")
    userID := c.GetString("user_id")

    response, err := h.todoModel.ListTripTodos(
        c.Request.Context(), 
        tripID, 
        userID,
        params.Limit,
        params.Offset,
    )
    if err != nil {
        c.Error(err)
        return
    }

    c.JSON(http.StatusOK, response)
}

func (h *TodoHandler) StreamTodoEvents(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("tripId")
    userID := c.GetString("user_id")

    // Verify trip access
    if _, err := h.todoModel.ListTripTodos(c.Request.Context(), tripID, userID, 1, 0); err != nil {
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