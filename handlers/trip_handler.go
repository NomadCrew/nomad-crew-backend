package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	trip "github.com/NomadCrew/nomad-crew-backend/models/trip"
	tcommand "github.com/NomadCrew/nomad-crew-backend/models/trip/command"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/supabase-community/supabase-go"
)

type TripHandler struct {
	tripModel    *trip.TripModel
	eventService types.EventPublisher
	supabase     *supabase.Client
}

type UpdateTripStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func NewTripHandler(model *trip.TripModel, eventService types.EventPublisher, supabase *supabase.Client) *TripHandler {
	return &TripHandler{
		tripModel:    model,
		eventService: eventService,
		supabase:     supabase,
	}
}

// CreateTripRequest represents the request body for creating a trip
type CreateTripRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Destination types.Destination `json:"destination" binding:"required"`
	StartDate   time.Time         `json:"startDate" binding:"required"`
	EndDate     time.Time         `json:"endDate" binding:"required"`
	Status      types.TripStatus  `json:"status"`
}

func (h *TripHandler) CreateTripHandler(c *gin.Context) {
	var req types.Trip
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.ValidationFailed("invalid_request", err.Error()))
		return
	}

	cmd := &tcommand.CreateTripCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: c.GetString("user_id"),
			Ctx:    h.tripModel.GetCommandContext(),
		},
		Trip: &req,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish creation events
	for _, event := range result.Events {
		if err := h.eventService.Publish(c, result.Data.(*types.Trip).ID, event); err != nil {
			logger.GetLogger().Errorw("Failed to publish create event", "error", err)
		}
	}

	c.JSON(http.StatusCreated, result.Data)
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	cmd := &tcommand.GetTripCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		TripID: tripID,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, result.Data)
}

func (h *TripHandler) UpdateTripHandler(c *gin.Context) {
    log := logger.GetLogger()
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    var update types.TripUpdate
    if err := c.ShouldBindJSON(&update); err != nil {
        c.Error(errors.ValidationFailed("Invalid update data", err.Error()))
        return
    }

    cmd := &tcommand.UpdateTripCommand{
        BaseCommand: tcommand.BaseCommand{
            UserID: userID,
            Ctx:    h.tripModel.GetCommandContext(),
        },
        TripID: tripID,
        Update: &update,
    }

    result, err := cmd.Execute(c.Request.Context())
    if err != nil {
        h.handleModelError(c, err)
        return
    }

    // Process command result events
    for _, event := range result.Events {
        if err := h.eventService.Publish(c, tripID, event); err != nil {
            log.Errorw("Failed to publish update event", "error", err)
        }
    }

    c.JSON(http.StatusOK, result.Data)
}

// UpdateTripStatusHandler updates trip status using command pattern
func (h *TripHandler) UpdateTripStatusHandler(c *gin.Context) {
	log := logger.GetLogger()

	tripID := c.Param("id")
	userID := c.GetString("user_id")

	var req UpdateTripStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid status update request", "error", err)
		c.Error(errors.ValidationFailed("Invalid request", err.Error()))
		return
	}

	// Create and execute command
	cmd := &tcommand.UpdateTripStatusCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		TripID:    tripID,
		NewStatus: types.TripStatus(req.Status),
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	updatedTrip := result.Data.(*types.Trip)
	c.JSON(http.StatusOK, gin.H{
		"message": "Trip status updated successfully",
		"data":    updatedTrip,
	})
}

func (h *TripHandler) handleModelError(c *gin.Context, err error) {
	log := logger.GetLogger()

	var response types.ErrorResponse
	var statusCode int

	switch e := err.(type) {
	case *errors.AppError:
		response.Code = string(e.Type)
		response.Message = e.Message
		response.Error = e.Detail
		statusCode = e.HTTPStatus
	default:
		log.Errorw("Unexpected error", "error", err)
		response.Code = "INTERNAL_ERROR"
		response.Message = "An unexpected error occurred"
		response.Error = "Internal server error"
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, response)
}

// Helper function to map TripError codes to HTTP status codes
func mapErrorToStatusCode(code string) int {
	switch code {
	case errors.ErrorTypeTripNotFound:
		return http.StatusNotFound
	case errors.ErrorTypeTripAccessDenied:
		return http.StatusForbidden
	case errors.ErrorTypeValidation, errors.ErrorTypeInvalidStatusTransition:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (h *TripHandler) ListUserTripsHandler(c *gin.Context) {
    cmd := &tcommand.ListTripsCommand{
        BaseCommand: tcommand.BaseCommand{
            UserID: c.GetString("user_id"),
            Ctx:    h.tripModel.GetCommandContext(),
        },
    }

    result, err := cmd.Execute(c.Request.Context())
    if err != nil {
        h.handleModelError(c, err)
        return
    }

    c.JSON(http.StatusOK, result.Data)
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
    tripID := c.Param("id")
    userID := c.GetString("user_id")

    cmd := &tcommand.DeleteTripCommand{
        BaseCommand: tcommand.BaseCommand{
            UserID: userID,
            Ctx:    h.tripModel.GetCommandContext(),
        },
        TripID: tripID,
    }

    result, err := cmd.Execute(c.Request.Context())
    if err != nil {
        h.handleModelError(c, err)
        return
    }

    // Publish deletion events
    for _, event := range result.Events {
        if err := h.eventService.Publish(c, tripID, event); err != nil {
            logger.GetLogger().Errorw("Failed to publish delete event", "error", err)
        }
    }

    c.Status(http.StatusNoContent)
}

func (h *TripHandler) SearchTripsHandler(c *gin.Context) {
    log := logger.GetLogger()

    var criteria types.TripSearchCriteria
    if err := c.ShouldBindJSON(&criteria); err != nil {
        if err := c.Error(errors.ValidationFailed("Invalid search criteria", err.Error())); err != nil {
            log.Errorw("Failed to add validation error", "error", err)
        }
        return
    }

    cmd := &tcommand.SearchTripsCommand{
        BaseCommand: tcommand.BaseCommand{
            UserID: c.GetString("user_id"),
            Ctx:    h.tripModel.GetCommandContext(),
        },
        Criteria: criteria,
    }

    result, err := cmd.Execute(c.Request.Context())
    if err != nil {
        h.handleModelError(c, err)
        return
    }

    c.JSON(http.StatusOK, result.Data)
}

type AddMemberRequest struct {
	UserID string           `json:"userId" binding:"required"`
	Role   types.MemberRole `json:"role" binding:"required"`
}

type UpdateMemberRoleRequest struct {
	Role types.MemberRole `json:"role" binding:"required"`
}

// AddMemberHandler handles adding a new member to a trip
func (h *TripHandler) AddMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	// requestingUserID := c.GetString("user_id")

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	membership := &types.TripMembership{
		TripID: tripID,
		UserID: req.UserID,
		Role:   req.Role,
	}

	err := h.tripModel.AddMember(c.Request.Context(), membership)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to add member error", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Member added successfully",
	})
}

// UpdateMemberRoleHandler handles updating a member's role
func (h *TripHandler) UpdateMemberRoleHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.Param("userId")

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(errors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	result, err := h.tripModel.UpdateMemberRole(c.Request.Context(), tripID, userID, req.Role)
	if err != nil {
		c.Error(err)
		return
	}

	// Handle successful command events
	for _, event := range result.Events {
		if pubErr := h.eventService.Publish(c.Request.Context(), event.TripID, event); pubErr != nil {
			log.Errorw("Failed to publish role update event", "error", pubErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Member role updated successfully",
	})
}

// RemoveMemberHandler handles removing a member from a trip
func (h *TripHandler) RemoveMemberHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id")
	memberID := c.Param("memberId")

	cmd := &tcommand.RemoveMemberCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		TripID:   tripID,
		MemberID: memberID,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish removal events
	for _, event := range result.Events {
		if err := h.eventService.Publish(c, tripID, event); err != nil {
			logger.GetLogger().Errorw("Failed to publish member removal event", "error", err)
		}
	}

	c.Status(http.StatusNoContent)
}

// GetTripMembersHandler handles getting all members of a trip
func (h *TripHandler) GetTripMembersHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	members, err := h.tripModel.GetTripMembers(c.Request.Context(), tripID)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to get members error", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"members": members,
	})
}

// GetTripWithMembersHandler handles getting a trip with its members
func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	trip, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to get trip with members error", "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, trip)
}

// WSStreamEvents upgrades the HTTP connection to a WebSocket and streams events.
// It both sends events from the Redis-based event service to the client
// and reads client messages to publish them as events.
func (h *TripHandler) WSStreamEvents(c *gin.Context) {
	conn := c.MustGet("wsConnection").(*middleware.SafeConn)
	defer conn.Close()

	// Add heartbeat
	conn.SetPingHandler(func(string) error {
		return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(10*time.Second))
	})

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	tripID := c.Param("id")
	userID := c.GetString("user_id")
	log := logger.GetLogger()
	ctx := c.Request.Context()

	log.Infow("WebSocket connection established",
		"tripID", tripID,
		"userID", userID,
		"remoteAddr", conn.RemoteAddr())

	// Create cancellable context for cleanup
	wsCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Get trip for weather updates
	trip, err := h.tripModel.GetTripByID(wsCtx, tripID)
	if err != nil {
		log.Errorw("Failed to get trip for weather updates", "error", err)
		if err := conn.WriteJSON(types.ErrorResponse{
			Error: "Failed to initialize connection",
			Code:  "INITIALIZATION_ERROR",
		}); err != nil {
			log.Errorw("Failed to send error message", "error", err)
		}
		if err := conn.Close(); err != nil {
			log.Errorw("Failed to close WebSocket connection", "error", err)
		}
		return
	}

	// Subscribe to events FIRST
	eventChan, err := h.eventService.Subscribe(wsCtx, tripID, userID)
	if err != nil {
		log.Errorw("Failed to subscribe to events",
			"error", err,
			"tripID", tripID)
		if err := conn.Close(); err != nil {
			log.Errorw("Failed to close WebSocket connection", "error", err)
		}
		return
	}

	// THEN start tracking connection
	h.tripModel.GetCommandContext().WeatherSvc.IncrementSubscribers(tripID, trip.Destination)
	defer h.tripModel.GetCommandContext().WeatherSvc.DecrementSubscribers(tripID)

	// Ping/Pong handling
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Message handling goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorw("Recovered from panic in event handler", "panic", r)
			}
		}()

		for {
			select {
			case <-wsCtx.Done():
				return
			case event, ok := <-eventChan:
				if !ok {
					return
				}

				if middleware.ConnIsClosed(conn) {
					return
				}

				msg, err := json.Marshal(event)
				if err != nil {
					log.Errorw("Failed to marshal event",
						"error", err,
						"eventType", event.Type)
					continue
				}

				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Errorw("Failed to send event to client",
						"error", err,
						"client", conn.RemoteAddr())
					return
				}
			}
		}
	}()

	// Keep-alive loop
	ticker = time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wsCtx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	tripID := c.Param("id")
	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
	if err != nil {
		c.Error(err)
		return
	}

	h.tripModel.GetCommandContext().WeatherSvc.TriggerImmediateUpdate(
		c.Request.Context(),
		tripID,
		trip.Destination,
	)

	c.Status(http.StatusAccepted)
}

// InviteMemberHandler handles member invitations using command pattern
func (h *TripHandler) InviteMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	var req types.TripInvitation
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid invitation request", "error", err)
		c.Error(errors.ValidationFailed("Invalid request", err.Error()))
		return
	}

	// Populate invitation data
	req.TripID = tripID
	req.InviterID = userID
	req.Status = types.InvitationStatusPending
	req.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)

	// Create and execute command
	cmd := &tcommand.InviteMemberCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		Invitation: &req,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish events from command result
	for _, event := range result.Events {
		if err := h.eventService.Publish(c.Request.Context(), tripID, event); err != nil {
			log.Errorw("Failed to publish event", "error", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Invitation sent successfully",
		"data":    result.Data,
	})
}

func (h *TripHandler) AcceptInvitationHandler(c *gin.Context) {
	invitationID := c.Param("invitationId")
	userID := c.GetString("user_id")

	// Execute accept invitation command
	cmd := &tcommand.AcceptInvitationCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		InvitationID: invitationID,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish events from command result
	for _, event := range result.Events {
		if err := h.eventService.Publish(c.Request.Context(), event.ID, event); err != nil {
			logger.GetLogger().Errorw("Failed to publish event", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation accepted successfully",
		"tripId":  result.Data.(*types.TripMembership).TripID,
	})
}
