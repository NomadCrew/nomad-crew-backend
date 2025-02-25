package handlers

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	trip "github.com/NomadCrew/nomad-crew-backend/models/trip"
	tcommand "github.com/NomadCrew/nomad-crew-backend/models/trip/command"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/supabase-community/supabase-go"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Define context keys
const (
	userIDKey contextKey = "user_id"
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
		log := logger.GetLogger()
		log.Errorw("Invalid request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Set the creator from the context.
	req.CreatedBy = c.GetString("user_id")

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
		log.Errorw("Invalid update data", "error", err)
		if err := c.Error(apperrors.ValidationFailed("Invalid update data", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
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
		if err := c.Error(apperrors.ValidationFailed("Invalid request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
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

	log.Debugw("Command context state before execution",
		"emailSvcExists", cmd.Ctx.EmailSvc != nil,
		"configExists", cmd.Ctx.Config != nil,
		"storeExists", cmd.Ctx.Store != nil)

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
	case *apperrors.AppError:
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
		if err := c.Error(apperrors.ValidationFailed("Invalid search criteria", err.Error())); err != nil {
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
		if err := c.Error(apperrors.ValidationFailed("Invalid request body", err.Error())); err != nil {
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
		if err := c.Error(apperrors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	result, err := h.tripModel.UpdateMemberRole(c.Request.Context(), tripID, userID, req.Role)
	if err != nil {
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
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
// WSStreamEvents upgrades the HTTP connection to a WebSocket and streams events.
func (h *TripHandler) WSStreamEvents(c *gin.Context) {
	log := logger.GetLogger()

	// Get WebSocket connection from middleware
	conn, ok := c.MustGet("wsConnection").(*middleware.SafeConn)
	if !ok {
		log.Error("WebSocket connection not found in context")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Get user and trip IDs
	userID := conn.UserID
	tripID := conn.TripID

	if userID == "" || tripID == "" {
		log.Error("Missing user or trip ID in WebSocket connection")
		if err := conn.Close(); err != nil {
			log.Warnw("Error closing connection with missing IDs", "error", err)
		}
		return
	}

	log.Infow("WebSocket connection established",
		"tripID", tripID,
		"userID", userID,
		"remoteAddr", conn.RemoteAddr())

	// Create context with cancellation for cleanup
	wsCtx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Ensure the user_id is propagated to the new context
	wsCtx = context.WithValue(wsCtx, userIDKey, userID)

	// Debug log to confirm user_id is in the context
	log.Debugw("User ID set in WebSocket context",
		"userID", userID,
		"contextHasUserID", wsCtx.Value(userIDKey) != nil,
		"contextUserIDValue", wsCtx.Value(userIDKey))

	// Subscribe to events
	eventChan, err := h.eventService.Subscribe(wsCtx, tripID, userID)
	if err != nil {
		log.Errorw("Failed to subscribe to events",
			"error", err,
			"tripID", tripID,
			"userID", userID)
		if closeErr := conn.Close(); closeErr != nil {
			log.Errorw("Error closing WebSocket connection", "error", closeErr, "tripID", tripID)
		}
		return
	}

	// Set up cleanup on exit
	defer func() {
		if err := h.eventService.Unsubscribe(context.Background(), tripID, userID); err != nil {
			log.Errorw("Failed to unsubscribe properly",
				"error", err,
				"tripID", tripID,
				"userID", userID)
		}
	}()

	// Start weather service for this trip
	// Use an explicit GetTripByID implementation that doesn't rely on the context for user ID
	cmd := &tcommand.GetTripCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID, // Set user ID directly rather than extracting from context
			Ctx:    h.tripModel.GetCommandContext(),
		},
		TripID: tripID,
	}

	// Debug log to confirm user ID is set in the command
	log.Debugw("Command created with user ID",
		"commandUserID", cmd.UserID,
		"tripID", cmd.TripID)

	result, err := cmd.Execute(wsCtx)
	if err != nil {
		log.Errorw("Failed to get trip details",
			"error", err,
			"tripID", tripID)
		if closeErr := conn.Close(); closeErr != nil {
			log.Errorw("Error closing WebSocket connection", "error", closeErr, "tripID", tripID)
		}
		return
	}

	trip := result.Data.(*types.Trip)

	// Continue with the existing weather service setup
	h.tripModel.GetCommandContext().WeatherSvc.IncrementSubscribers(tripID, trip.Destination)
	defer h.tripModel.GetCommandContext().WeatherSvc.DecrementSubscribers(tripID)

	// Create dedicated goroutine for reading from the client (if needed)
	// This is mostly for future bi-directional communication
	go func() {
		defer func() {
			// Recover from any panics in the read loop
			if r := recover(); r != nil {
				log.Errorw("Panic in WebSocket read goroutine",
					"recover", r,
					"tripID", tripID,
					"userID", userID)
			}

			log.Debugw("WebSocket read goroutine exiting",
				"tripID", tripID,
				"userID", userID)

			// Ensure context is cancelled to clean up other goroutines
			cancel()
		}()

		for {
			// Check if connection is still valid
			if conn == nil || middleware.ConnIsClosed(conn) {
				log.Debugw("Connection closed or nil in read goroutine",
					"conn_nil", conn == nil,
					"tripID", tripID,
					"userID", userID)
				return
			}

			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debugw("WebSocket closed normally",
						"tripID", tripID,
						"userID", userID)
				} else {
					log.Warnw("WebSocket read error",
						"error", err,
						"tripID", tripID,
						"userID", userID)
				}
				return
			}
		}
	}()

	// Process incoming events and send to client
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-wsCtx.Done():
			log.Debugw("WebSocket context done",
				"tripID", tripID,
				"userID", userID)
			return

		case <-pingTicker.C:
			// Send ping to keep connection alive
			deadline := time.Now().Add(10 * time.Second)
			if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				log.Warnw("Failed to write ping message",
					"error", err,
					"tripID", tripID,
					"userID", userID)
				return
			}

		case event, ok := <-eventChan:
			if !ok {
				log.Infow("Event channel closed",
					"tripID", tripID,
					"userID", userID)
				return
			}

			// Check if connection is closed before sending
			if conn == nil || middleware.ConnIsClosed(conn) {
				log.Debugw("Connection closed or nil before sending event",
					"conn_nil", conn == nil,
					"tripID", tripID,
					"userID", userID,
					"eventType", event.Type)
				return
			}

			// Serialize event efficiently using pool
			data, err := json.Marshal(event)
			if err != nil {
				log.Errorw("Failed to marshal event",
					"error", err,
					"eventType", event.Type,
					"tripID", tripID)
				continue
			}

			// Add debugging info for event types
			log.Debugw("Sending event to WebSocket client",
				"eventType", event.Type,
				"dataSize", len(data),
				"tripID", tripID,
				"userID", userID)

			// Send event to client - add timeout to prevent blocking indefinitely
			writeErr := make(chan error, 1)

			// Use a context with timeout for cancellation
			writeCtx, writeCancel := context.WithTimeout(context.Background(), 5*time.Second)

			go func() {
				defer writeCancel()
				defer close(writeErr)

				// Check once more if connection is closed or became nil
				if conn == nil || middleware.ConnIsClosed(conn) {
					writeErr <- fmt.Errorf("connection closed or nil before write attempt")
					return
				}

				// Add one more defensive check
				select {
				case <-writeCtx.Done():
					// Don't even attempt to write if already timed out
					return
				default:
					writeErr <- conn.WriteMessage(websocket.TextMessage, data)
				}
			}()

			// Wait with timeout
			select {
			case err := <-writeErr:
				if err != nil {
					log.Warnw("Failed to write event to WebSocket",
						"error", err,
						"eventType", event.Type,
						"tripID", tripID,
						"userID", userID,
						"errorType", fmt.Sprintf("%T", err))

					// Check for specific error types that indicate client issues
					if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure,
						websocket.CloseNoStatusReceived) {
						log.Infow("Client closed connection normally",
							"tripID", tripID,
							"userID", userID)
					} else if strings.Contains(err.Error(), "broken pipe") ||
						strings.Contains(err.Error(), "connection reset by peer") ||
						strings.Contains(err.Error(), "use of closed network connection") {
						log.Warnw("Client connection lost unexpectedly",
							"errorDetails", err.Error(),
							"tripID", tripID,
							"userID", userID)
					}

					// When write timeouts, we should terminate the websocket connection
					if conn != nil {
						if err := conn.Close(); err != nil {
							log.Warnw("Error closing connection", "error", err, "userID", userID)
						}
					}
					return
				}

				// Log occasional success for debugging
				if secureRandomFloat() < 0.01 { // Log ~1% of successful sends
					log.Debugw("Successfully sent event to client",
						"eventType", event.Type,
						"tripID", tripID,
						"userID", userID)
				}

			case <-writeCtx.Done():
				log.Warnw("WebSocket write timed out",
					"eventType", event.Type,
					"tripID", tripID,
					"userID", userID)
				// When write timeouts, we should terminate the websocket connection
				if conn != nil {
					if err := conn.Close(); err != nil {
						log.Warnw("Error closing connection", "error", err, "userID", userID)
					}
				}
				return
			}
		}
	}
}

func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	tripID := c.Param("id")
	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID)
	if err != nil {
		log := logger.GetLogger()
		if err := c.Error(err); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	h.tripModel.GetCommandContext().WeatherSvc.TriggerImmediateUpdate(
		c.Request.Context(),
		tripID,
		trip.Destination,
	)

	c.Status(http.StatusAccepted)
}

func (h *TripHandler) InviteMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	type InviteMemberRequest struct {
		Data struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}

	var reqPayload InviteMemberRequest
	if err := c.ShouldBindJSON(&reqPayload); err != nil {
		log.Errorw("Invalid invitation request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	invitation := &types.TripInvitation{
		InviteeEmail: reqPayload.Data.Email,
		TripID:       tripID,
		InviterID:    userID,
		Status:       types.InvitationStatusPending,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	if reqPayload.Data.Role != "" {
		invitation.Role = types.MemberRole(reqPayload.Data.Role)
	} else {
		invitation.Role = types.MemberRoleMember
	}

	cmd := &tcommand.InviteMemberCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
			Ctx:    h.tripModel.GetCommandContext(),
		},
		Invitation: invitation,
	}

	if err := c.Error(apperrors.InternalServerError("Server configuration missing")); err != nil {
		log.Errorw("Failed to set error in context", "error", err)
		return
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish events from command result.
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
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log := logger.GetLogger()
		log.Errorw("Invalid invitation acceptance request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Validate JWT
	claims, err := auth.ValidateInvitationToken(req.Token, h.tripModel.GetCommandContext().Config.JwtSecretKey)
	if err != nil {
		log := logger.GetLogger()
		if err := c.Error(apperrors.Unauthorized("invalid_token", "Invalid or expired invitation")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Execute accept invitation command
	cmd := &tcommand.AcceptInvitationCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: c.GetString("user_id"),
			Ctx:    h.tripModel.GetCommandContext(),
		},
		InvitationID: claims.InvitationID,
	}

	result, err := cmd.Execute(c.Request.Context())
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Publish events from command result
	log := logger.GetLogger()
	for _, event := range result.Events {
		if err := h.eventService.Publish(c.Request.Context(), event.TripID, event); err != nil {
			log.Errorw("Failed to publish event", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation accepted successfully",
		"data":    result.Data,
	})
}

// secureRandomFloat returns a cryptographically secure random float64 between 0 and 1
func secureRandomFloat() float64 {
	var buf [8]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		// If crypto/rand fails, return 1.0 to ensure logging happens rather than silently failing
		return 1.0
	}
	return float64(binary.LittleEndian.Uint64(buf[:])) / float64(1<<64)
}
