package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
// This handler now processes all events for a trip, including chat messages.
// Chat messages are sent as events through the event system rather than through
// a separate websocket connection.
func (h *TripHandler) WSStreamEvents(c *gin.Context) {
	// Get logger instance once and reuse it
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

			// Read message from client
			_, message, err := conn.ReadMessage()
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

			// Process the message
			if len(message) > 0 {
				// Handle the message in a separate goroutine to avoid blocking the read loop
				go func(msg []byte) {
					if err := h.HandleChatMessage(wsCtx, conn, msg, userID, tripID); err != nil {
						log.Warnw("Failed to handle chat message",
							"error", err,
							"tripID", tripID,
							"userID", userID)
					}
				}(message)
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
				// Don't immediately return on ping error - check if connection is still valid
				if err.Error() == "connection is nil" || err == websocket.ErrCloseSent {
					log.Debugw("Connection is no longer valid, exiting ping loop",
						"tripID", tripID,
						"userID", userID)
					return
				}
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

	// Read and log the raw request body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorw("Failed to read request body", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", "Failed to read request body")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Log the raw request body
	log.Debugw("Raw invitation request body", "body", string(bodyBytes))

	// Restore the request body for binding
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Update the request structure to match the actual client format
	type InviteMemberRequest struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	var reqPayload InviteMemberRequest
	if err := c.ShouldBindJSON(&reqPayload); err != nil {
		log.Errorw("Invalid invitation request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("invalid_request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Log the parsed payload
	log.Debugw("Parsed invitation request",
		"email", reqPayload.Email,
		"role", reqPayload.Role)

	invitation := &types.TripInvitation{
		InviteeEmail: reqPayload.Email,
		TripID:       tripID,
		InviterID:    userID,
		Status:       types.InvitationStatusPending,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	if reqPayload.Role != "" {
		invitation.Role = types.MemberRole(reqPayload.Role)
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
	log := logger.GetLogger()
	var token string

	// Try to get token from URL query parameter first (for deep links)
	tokenParam := c.Query("token")
	if tokenParam != "" {
		token = tokenParam
		log.Debugw("Using token from URL query parameter", "token", token)
	} else {
		// If not in URL, try to get from JSON body (for API calls)
		var req struct {
			Token string `json:"token" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorw("Invalid invitation acceptance request", "error", err)
			if err := c.Error(apperrors.ValidationFailed("invalid_request", "Token is required. Provide it either as a query parameter or in the request body")); err != nil {
				log.Errorw("Failed to set error in context", "error", err)
			}
			return
		}
		token = req.Token
		log.Debugw("Using token from request body", "token", token)
	}

	// Validate JWT
	claims, err := auth.ValidateInvitationToken(token, h.tripModel.GetCommandContext().Config.JwtSecretKey)
	if err != nil {
		if err := c.Error(apperrors.Unauthorized("invalid_token", "Invalid or expired invitation")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Check if invitationId is empty (which would be an issue)
	if claims.InvitationID == "" {
		// If invitationId is empty but tripId and email are present, try to find the invitation
		if claims.TripID != "" && claims.InviteeEmail != "" {
			invitation, err := h.tripModel.FindInvitationByTripAndEmail(c.Request.Context(), claims.TripID, claims.InviteeEmail)
			if err != nil {
				log.Errorw("Failed to find invitation by trip and email",
					"error", err,
					"tripId", claims.TripID,
					"email", claims.InviteeEmail)
				if err := c.Error(apperrors.NotFound("invitation_not_found", "Invitation not found")); err != nil {
					log.Errorw("Failed to set error in context", "error", err)
				}
				return
			}
			claims.InvitationID = invitation.ID
			log.Infow("Found invitation ID from trip and email",
				"invitationId", claims.InvitationID,
				"tripId", claims.TripID,
				"email", claims.InviteeEmail)
		} else {
			if err := c.Error(apperrors.ValidationFailed("invalid_token", "Invitation token is missing required data")); err != nil {
				log.Errorw("Failed to set error in context", "error", err)
			}
			return
		}
	}

	// Get user ID from context or token
	userID := c.GetString("user_id")

	// If user is not authenticated but we have their email from the token,
	// we can try to find or create their account
	if userID == "" {
		// This would require integration with your auth system
		// For now, we'll return an error requiring authentication
		if err := c.Error(apperrors.Unauthorized("authentication_required", "You must be logged in to accept an invitation")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Execute accept invitation command
	cmd := &tcommand.AcceptInvitationCommand{
		BaseCommand: tcommand.BaseCommand{
			UserID: userID,
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

// HandleInvitationDeepLink handles direct URL access to invitation links
// and redirects to the appropriate app URL scheme
func (h *TripHandler) HandleInvitationDeepLink(c *gin.Context) {
	log := logger.GetLogger()
	token := c.Param("token")

	if token == "" {
		if err := c.Error(apperrors.ValidationFailed("invalid_request", "Token is required")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Validate the token to ensure it's legitimate before redirecting
	claims, err := auth.ValidateInvitationToken(token, h.tripModel.GetCommandContext().Config.JwtSecretKey)
	if err != nil {
		log.Errorw("Invalid invitation token", "error", err)
		if err := c.Error(apperrors.Unauthorized("invalid_token", "Invalid or expired invitation")); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	// Get the frontend URL from config
	frontendURL := h.tripModel.GetCommandContext().Config.FrontendURL

	// Ensure frontendURL is not empty and has a protocol
	if frontendURL == "" {
		frontendURL = "https://nomadcrew.uk" // Default fallback
		log.Warnw("FrontendURL not configured, using default", "default", frontendURL)
	}

	// Ensure URL has protocol
	if !strings.HasPrefix(frontendURL, "http://") && !strings.HasPrefix(frontendURL, "https://") {
		frontendURL = "https://" + frontendURL
		log.Warnw("FrontendURL missing protocol, adding https://", "frontendURL", frontendURL)
	}

	// Remove trailing slash if present
	frontendURL = strings.TrimSuffix(frontendURL, "/")

	// Check if the request is from a mobile device
	userAgent := c.Request.UserAgent()
	isMobile := strings.Contains(strings.ToLower(userAgent), "mobile") ||
		strings.Contains(strings.ToLower(userAgent), "android") ||
		strings.Contains(strings.ToLower(userAgent), "iphone") ||
		strings.Contains(strings.ToLower(userAgent), "ipad")

	var redirectURL string

	if isMobile {
		// For mobile devices, use the app scheme
		// This should match what's configured in your Expo app
		redirectURL = fmt.Sprintf("nomadcrew://invite/accept?token=%s&tripId=%s&email=%s",
			token, claims.TripID, claims.InviteeEmail)
	} else {
		// For web browsers, redirect to the web frontend
		redirectURL = fmt.Sprintf("%s/invite/accept/%s", frontendURL, token)
	}

	log.Infow("Redirecting invitation",
		"isMobile", isMobile,
		"redirectURL", redirectURL,
		"tripId", claims.TripID)

	// Set headers to prevent caching
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Redirect to the appropriate URL
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
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

// ListTripMessages lists all messages in a trip
func (h *TripHandler) ListTripMessages(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	_, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warnw("Trip ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Parse pagination parameters
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		limit = 20
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		offset = 0
	}

	// Get the chat store from the trip model
	chatStore := h.tripModel.GetChatStore()
	if chatStore == nil {
		log.Errorw("Chat store not available")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat service not available"})
		return
	}

	// List chat messages for the trip
	messages, total, err := chatStore.ListTripMessages(c.Request.Context(), tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to list trip messages", "error", err, "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list trip messages"})
		return
	}

	// Convert messages to ChatMessageWithUser format
	messagesWithUsers := make([]types.ChatMessageWithUser, 0, len(messages))
	for _, msg := range messages {
		// Get user information
		user, err := chatStore.GetUserInfo(c.Request.Context(), msg.UserID)
		if err != nil {
			log.Warnw("Failed to get user info for message", "error", err, "userID", msg.UserID)
			user = &types.UserResponse{
				ID: msg.UserID,
			}
		}

		messageWithUser := types.ChatMessageWithUser{
			Message: msg,
			User:    *user,
		}
		messagesWithUsers = append(messagesWithUsers, messageWithUser)
	}

	// Create response
	response := types.ChatMessagePaginatedResponse{
		Messages: messagesWithUsers,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateLastReadMessage updates the last read message for a user in a trip
func (h *TripHandler) UpdateLastReadMessage(c *gin.Context) {
	log := logger.GetLogger()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		log.Warnw("User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get trip ID from path
	tripID := c.Param("id")
	if tripID == "" {
		log.Warnw("Trip ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trip ID is required"})
		return
	}

	// Get message ID from request body
	var req struct {
		MessageID string `json:"messageId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnw("Failed to parse request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate message ID
	if req.MessageID == "" {
		log.Warnw("Empty message ID provided", "tripID", tripID, "userID", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID cannot be empty"})
		return
	}

	// Get the chat store from the trip model
	chatStore := h.tripModel.GetChatStore()
	if chatStore == nil {
		log.Errorw("Chat store not available")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat service not available"})
		return
	}

	// Update the last read message
	err := chatStore.UpdateLastReadMessage(c.Request.Context(), tripID, userID.(string), req.MessageID)
	if err != nil {
		log.Errorw("Failed to update last read message", "error", err, "tripID", tripID, "userID", userID, "messageID", req.MessageID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update last read message"})
		return
	}

	// Create a read receipt event
	readReceiptEvent := types.ChatReadReceiptEvent{
		TripID:    tripID,
		MessageID: req.MessageID,
		User: types.UserResponse{
			ID: userID.(string),
		},
	}

	// Marshal the event payload
	payload, err := json.Marshal(readReceiptEvent)
	if err != nil {
		log.Errorw("Failed to marshal read receipt event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process read receipt"})
		return
	}

	// Create and publish the event
	event := types.Event{
		BaseEvent: types.BaseEvent{
			Type:      types.EventTypeChatReadReceipt,
			TripID:    tripID,
			UserID:    userID.(string),
			Timestamp: time.Now(),
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "trip_handler",
		},
		Payload: payload,
	}

	// Publish the event asynchronously to not block the response
	go func() {
		if err := h.eventService.Publish(context.Background(), tripID, event); err != nil {
			log.Errorw("Failed to publish read receipt event", "error", err, "messageID", req.MessageID)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Last read message updated successfully"})
}

// HandleChatMessage handles incoming chat messages from WebSocket
func (h *TripHandler) HandleChatMessage(ctx context.Context, conn *middleware.SafeConn, message []byte, userID, tripID string) error {
	log := logger.GetLogger()

	// Parse the message
	var wsMessage types.WebSocketChatMessage
	err := json.Unmarshal(message, &wsMessage)
	if err != nil {
		log.Errorw("Failed to unmarshal WebSocket message", "error", err)
		return fmt.Errorf("invalid message format: %w", err)
	}

	// Set the trip ID
	wsMessage.TripID = tripID

	// Handle different message types
	switch wsMessage.Type {
	case types.WebSocketMessageTypeChat:
		// Create a new chat message
		chatMessage := types.ChatMessage{
			TripID:  tripID,
			UserID:  userID,
			Content: wsMessage.Content,
		}

		// Get the chat store from the trip model
		chatStore := h.tripModel.GetChatStore()
		if chatStore == nil {
			return fmt.Errorf("chat service not available")
		}

		// Get user information
		user, err := chatStore.GetUserInfo(ctx, userID)
		if err != nil {
			log.Warnw("Failed to get user info", "error", err, "userID", userID)
			user = &types.UserResponse{
				ID: userID,
			}
		}

		// Create the chat message
		messageID, err := chatStore.CreateChatMessage(ctx, chatMessage)
		if err != nil {
			log.Errorw("Failed to create chat message", "error", err)
			return fmt.Errorf("failed to send message: %w", err)
		}

		// Create a chat message event
		chatEvent := types.ChatMessageEvent{
			MessageID: messageID,
			TripID:    tripID,
			Content:   wsMessage.Content,
			User:      *user,
			Timestamp: time.Now(),
		}

		// Marshal the event payload
		payload, err := json.Marshal(chatEvent)
		if err != nil {
			log.Errorw("Failed to marshal chat message event", "error", err)
			return fmt.Errorf("failed to process message: %w", err)
		}

		// Create and publish the event
		event := types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeChatMessageSent,
				TripID:    tripID,
				UserID:    userID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_handler",
			},
			Payload: payload,
		}

		// Publish the event
		if err := h.eventService.Publish(ctx, tripID, event); err != nil {
			log.Errorw("Failed to publish chat message event", "error", err, "messageID", messageID)
			return fmt.Errorf("failed to publish message: %w", err)
		}

		return nil

	case types.WebSocketMessageTypeTypingStatus:
		// Create a typing status event
		typingEvent := types.ChatTypingStatusEvent{
			TripID:   tripID,
			IsTyping: wsMessage.IsTyping,
			User: types.UserResponse{
				ID: userID,
			},
		}

		// Get the chat store from the trip model
		chatStore := h.tripModel.GetChatStore()
		if chatStore == nil {
			return fmt.Errorf("chat service not available")
		}

		// Get user information
		user, err := chatStore.GetUserInfo(ctx, userID)
		if err != nil {
			log.Warnw("Failed to get user info", "error", err, "userID", userID)
		} else {
			typingEvent.User = *user
		}

		// Marshal the event payload
		payload, err := json.Marshal(typingEvent)
		if err != nil {
			log.Errorw("Failed to marshal typing status event", "error", err)
			return fmt.Errorf("failed to process typing status: %w", err)
		}

		// Create and publish the event
		event := types.Event{
			BaseEvent: types.BaseEvent{
				Type:      types.EventTypeChatTypingStatus,
				TripID:    tripID,
				UserID:    userID,
				Timestamp: time.Now(),
				Version:   1,
			},
			Metadata: types.EventMetadata{
				Source: "trip_handler",
			},
			Payload: payload,
		}

		// Publish the event
		if err := h.eventService.Publish(ctx, tripID, event); err != nil {
			log.Errorw("Failed to publish typing status event", "error", err)
			return fmt.Errorf("failed to publish typing status: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unsupported message type: %s", wsMessage.Type)
	}
}
