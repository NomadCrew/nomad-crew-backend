package handlers

import (
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

	"github.com/NomadCrew/nomad-crew-backend/config"
	apperrors "github.com/NomadCrew/nomad-crew-backend/errors"
	"github.com/NomadCrew/nomad-crew-backend/internal/auth"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	trip "github.com/NomadCrew/nomad-crew-backend/models/trip"
	"github.com/NomadCrew/nomad-crew-backend/store"
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
	config       *config.ServerConfig
	weatherSvc   types.WeatherServiceInterface
	chatStore    store.ChatStore
}

type UpdateTripStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func NewTripHandler(model *trip.TripModel, eventService types.EventPublisher, supabase *supabase.Client, cfg *config.ServerConfig, weatherSvc types.WeatherServiceInterface, chatStore store.ChatStore) *TripHandler {
	return &TripHandler{
		tripModel:    model,
		eventService: eventService,
		supabase:     supabase,
		config:       cfg,
		weatherSvc:   weatherSvc,
		chatStore:    chatStore,
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

	createdTrip, err := h.tripModel.CreateTrip(c.Request.Context(), &req)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusCreated, createdTrip)
}

func (h *TripHandler) GetTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trip)
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

	// Pass userID and update pointer, expect (*Trip, error)
	updatedTrip, err := h.tripModel.UpdateTrip(c.Request.Context(), tripID, userID, &update)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, updatedTrip)
}

// UpdateTripStatusHandler updates trip status using the facade
func (h *TripHandler) UpdateTripStatusHandler(c *gin.Context) {
	log := logger.GetLogger()

	tripID := c.Param("id")
	// userID := c.GetString("user_id") // UserID check should happen in the model/service layer now

	var req UpdateTripStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("Invalid status update request", "error", err)
		if err := c.Error(apperrors.ValidationFailed("Invalid request", err.Error())); err != nil {
			log.Errorw("Failed to set error in context", "error", err)
		}
		return
	}

	newStatus := types.TripStatus(req.Status)
	// TODO: Add validation for TripStatus enum if needed here or rely on service layer

	err := h.tripModel.UpdateTripStatus(c.Request.Context(), tripID, newStatus)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Fetch the updated trip to return it (optional, could just return success)
	updatedTrip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, c.GetString("user_id")) // Need user ID for fetch
	if err != nil {
		// Log this error but potentially still return success for the update itself
		log.Errorw("Failed to fetch updated trip after status change", "tripID", tripID, "error", err)
		c.JSON(http.StatusOK, gin.H{"message": "Trip status updated successfully"})
		return
	}

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
	userID := c.GetString("user_id")

	trips, err := h.tripModel.ListUserTrips(c.Request.Context(), userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) DeleteTripHandler(c *gin.Context) {
	tripID := c.Param("id")
	// userID := c.GetString("user_id") // UserID check should happen in the model/service layer now

	err := h.tripModel.DeleteTrip(c.Request.Context(), tripID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Event publishing should now happen within the service/model layer
	// Remove event publishing logic from here

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

	// UserID from context can be added to criteria if needed for filtering/auth
	// criteria.UserID = c.GetString("user_id")

	trips, err := h.tripModel.SearchTrips(c.Request.Context(), criteria)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, trips)
}

type AddMemberRequest struct {
	UserID string           `json:"userId" binding:"required"`
	Role   types.MemberRole `json:"role" binding:"required"`
}

type UpdateMemberRoleRequest struct {
	Role types.MemberRole `json:"role" binding:"required"`
}

func (h *TripHandler) AddMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	// invokerUserID := c.GetString("user_id") // Authorization should happen in service layer

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid member data", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	membership := &types.TripMembership{
		TripID: tripID,
		UserID: req.UserID,
		Role:   req.Role,
		Status: types.MembershipStatusActive, // Assuming direct add means active
	}

	err := h.tripModel.AddMember(c.Request.Context(), membership)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Member added successfully"})
}

func (h *TripHandler) UpdateMemberRoleHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	memberID := c.Param("memberId")
	// invokerUserID := c.GetString("user_id") // Authorization should happen in service layer

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid role data", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// The model method signature takes ctx, tripID, memberID, newRole
	_, err := h.tripModel.UpdateMemberRole(c.Request.Context(), tripID, memberID, req.Role)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member role updated successfully"})
}

func (h *TripHandler) RemoveMemberHandler(c *gin.Context) {
	tripID := c.Param("id")
	memberID := c.Param("memberId") // This is the User ID to be removed
	// invokerUserID := c.GetString("user_id") // Authorization should happen in service layer

	err := h.tripModel.RemoveMember(c.Request.Context(), tripID, memberID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

func (h *TripHandler) GetTripMembersHandler(c *gin.Context) {
	tripID := c.Param("id")
	// userID := c.GetString("user_id") // Authorization should happen in service layer

	members, err := h.tripModel.GetTripMembers(c.Request.Context(), tripID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, members)
}

func (h *TripHandler) GetTripWithMembersHandler(c *gin.Context) {
	tripID := c.Param("id")
	userID := c.GetString("user_id") // UserID needed for GetTripWithMembers

	tripWithMembers, err := h.tripModel.GetTripWithMembers(c.Request.Context(), tripID, userID)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	c.JSON(http.StatusOK, tripWithMembers)
}

// WSStreamEvents handles WebSocket connections for streaming trip events.
func (h *TripHandler) WSStreamEvents(c *gin.Context) {
	log := logger.GetLogger()

	// Get SafeConn from context (set by WSMiddleware)
	safeConnGeneric, exists := c.Get(middleware.WebSocketConnectionKey) // Use the correct key
	if !exists {
		log.Error("WebSocket connection not found in context")
		// Don't use handleModelError, just abort
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "WebSocket connection unavailable"})
		return
	}
	safeConn, ok := safeConnGeneric.(*middleware.SafeConn)
	if !ok || safeConn == nil {
		log.Error("WebSocket connection in context has incorrect type or is nil")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "WebSocket connection invalid"})
		return
	}

	// UserID and TripID should be available on SafeConn, set by AuthWS middleware
	userID := safeConn.UserID
	tripID := safeConn.TripID

	if userID == "" || tripID == "" {
		log.Errorw("Missing UserID or TripID on WebSocket connection context", "userID", userID, "tripID", tripID)
		// Close the connection cleanly if possible
		safeConn.Close() // SafeConn.Close is safe to call multiple times
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "WebSocket authentication missing"})
		return
	}

	log.Infow("WebSocket connection context established", "tripID", tripID, "userID", userID, "remoteAddr", safeConn.RemoteAddr())

	// Create context tied to the request lifecycle for graceful shutdown
	wsCtx, cancel := context.WithCancel(c.Request.Context())
	defer cancel() // Ensure cancellation happens on handler exit

	// Subscribe the user to trip-specific events via the injected eventService
	subChan, err := h.eventService.Subscribe(wsCtx, tripID, userID)
	if err != nil {
		log.Errorw("Failed to subscribe to events", "tripID", tripID, "userID", userID, "error", err)
		// Try to send error before closing
		safeConn.WriteJSON(gin.H{"error": "Failed to subscribe to events"})
		safeConn.Close()
		return // Don't proceed if subscription fails
	}
	// Defer Unsubscribe: Use context.Background() for Unsubscribe to ensure it runs
	// even if the request context (wsCtx) is cancelled prematurely.
	defer func() {
		if err := h.eventService.Unsubscribe(context.Background(), tripID, userID); err != nil {
			log.Errorw("Error during WebSocket unsubscribe", "tripID", tripID, "userID", userID, "error", err)
		}
	}()

	// Goroutine to forward subscribed events to the WebSocket client
	go func() {
		defer cancel() // If this goroutine exits (e.g., error), cancel the context
		for {
			select {
			case event, ok := <-subChan:
				if !ok {
					log.Infow("Subscription channel closed by publisher", "tripID", tripID, "userID", userID)
					return // Exit goroutine if channel is closed
				}

				// Marshal the event struct to JSON bytes for sending
				eventBytes, marshalErr := json.Marshal(event)
				if marshalErr != nil {
					log.Errorw("Error marshalling event to JSON for WebSocket", "tripID", tripID, "userID", userID, "eventType", event.Type, "eventID", event.ID, "error", marshalErr)
					continue // Skip this event, but keep listening
				}

				// Send the JSON bytes as a WebSocket text message
				if writeErr := safeConn.WriteMessage(websocket.TextMessage, eventBytes); writeErr != nil {
					log.Errorw("Error writing event to WebSocket", "tripID", tripID, "userID", userID, "eventType", event.Type, "eventID", event.ID, "error", writeErr)
					// Check if the error indicates the connection is closed
					if websocket.IsCloseError(writeErr, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
						strings.Contains(writeErr.Error(), "use of closed network connection") {
						log.Infow("WebSocket connection closed while writing event", "tripID", tripID, "userID", userID)
					} else {
						log.Warnw("Unexpected WebSocket write error", "tripID", tripID, "userID", userID, "error", writeErr)
					}
					return // Stop sending on any write error
				}

			case <-wsCtx.Done(): // Use the handler's context for cancellation
				log.Infow("WebSocket context cancelled, stopping event forwarder goroutine", "tripID", tripID, "userID", userID)
				return
			}
		}
	}()

	// Read loop (blocking) - This goroutine handles incoming messages
	// It runs in the main handler goroutine context
	// It relies on SafeConn's internal readPump to put messages onto sc.readBuffer
	// and handle pings/pongs/close messages internally.
	// The readPump will close the connection and signal `sc.done` on error/close.

	for {
		select {
		case message, ok := <-safeConn.ReadChannel(): // Use the read channel provided by SafeConn
			if !ok {
				log.Infow("SafeConn read channel closed, likely connection terminated.", "tripID", tripID, "userID", userID)
				// Connection is handled by SafeConn's readPump, just exit handler
				return
			}

			log.Debugw("Received WebSocket message from client", "tripID", tripID, "userID", userID, "size", len(message))
			// Handle incoming message (e.g., chat) in a separate goroutine
			// to avoid blocking the read loop if processing takes time.
			go func(msg []byte) {
				if err := h.HandleChatMessage(wsCtx, safeConn, msg, userID, tripID); err != nil {
					log.Errorw("Failed to handle incoming WebSocket message", "tripID", tripID, "userID", userID, "error", err)
					// Optionally notify the client about the error
					safeConn.WriteJSON(gin.H{"error": "Failed to process message", "details": err.Error()})
				}
			}(message)

		case <-wsCtx.Done():
			log.Infow("WebSocket handler context done, exiting read loop.", "tripID", tripID, "userID", userID)
			// SafeConn.Close() will be called by its internal pumps or defer
			return
		case <-safeConn.DoneChannel(): // Listen for done signal from SafeConn's internal pumps
			log.Infow("SafeConn done channel closed, connection terminated.", "tripID", tripID, "userID", userID)
			return
		}
	}
}

func (h *TripHandler) TriggerWeatherUpdateHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")

	// Fetch trip details to get destination - Requires GetTripByID
	trip, err := h.tripModel.GetTripByID(c.Request.Context(), tripID, c.GetString("user_id")) // Need user ID for fetch
	if err != nil {
		log.Errorw("Failed to get trip details for weather update trigger", "tripID", tripID, "error", err)
		h.handleModelError(c, err)
		return
	}

	// Use the injected weather service
	if h.weatherSvc == nil {
		log.Errorw("Weather service not configured in handler", "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Weather service unavailable"})
		return
	}

	log.Infow("Triggering immediate weather update", "tripID", tripID, "destination", trip.Destination.Address)
	// Pass the whole Destination struct as per interface expectation (based on previous analysis)
	if err := h.weatherSvc.TriggerImmediateUpdate(c.Request.Context(), tripID, trip.Destination); err != nil {
		log.Errorw("Failed to trigger immediate weather update", "tripID", tripID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger weather update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Weather update triggered successfully"})
}

// InviteMemberRequest structure
type InviteMemberRequest struct {
	Email string           `json:"email" binding:"required,email"`
	Role  types.MemberRole `json:"role" binding:"required"`
}

func (h *TripHandler) InviteMemberHandler(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	invokerUserID := c.GetString("user_id")

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid invitation data", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Check if user exists in Supabase (optional, could be done in service)
	invitedUser, err := h.tripModel.LookupUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Handle cases where lookup fails, but maybe still allow invitation
		log.Warnw("Failed to lookup user by email during invitation", "email", req.Email, "error", err)
		// Decide if this is a fatal error or just a warning
		// h.handleModelError(c, err) // Or maybe not fatal?
	}

	invitation := &types.TripInvitation{
		TripID:      tripID,
		InviterID:   invokerUserID,
		InviteeEmail: req.Email,
		Role:        req.Role,
		Status:      types.InvitationStatusPending,
		// InviteeID might be set later or if user lookup was successful
	}
	if invitedUser != nil {
		invitation.InviteeID = &invitedUser.ID
	}

	// Create invitation using the model facade
	err = h.tripModel.CreateInvitation(c.Request.Context(), invitation)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Note: Email sending logic should now be within the CreateInvitation service/model method.
	// We don't need JWT generation or frontend URL construction here anymore.

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Invitation sent successfully",
		"invitationId": invitation.ID, // Return the ID if needed
	})
}

// AcceptInvitationRequest structure
type AcceptInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}

func (h *TripHandler) AcceptInvitationHandler(c *gin.Context) {
	log := logger.GetLogger()
	// UserID from context is the user accepting the invitation
	acceptingUserID := c.GetString("user_id")

	var req AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid token data", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Validate the token using the injected config
	claims, err := auth.ValidateInvitationToken(req.Token, h.config.JwtSecretKey)
	if err != nil {
		log.Errorw("Invalid invitation token", "error", err)
		c.Error(apperrors.AuthenticationFailed("Invalid or expired invitation token"))
		return
	}

	// Basic validation: Ensure the user accepting is the one invited (if email matches)
	// More robust checks (e.g., if user ID was in token) should happen in service layer
	userProfile, ok := c.Get(middleware.UserProfileKey)
	if !ok {
		log.Error("User profile not found in context for invitation acceptance")
		c.Error(apperrors.InternalServerError("User profile unavailable"))
		return
	}
	profile, ok := userProfile.(types.UserProfile)
	if !ok {
		log.Error("User profile has incorrect type")
		c.Error(apperrors.InternalServerError("Invalid user profile type"))
		return
	}

	if claims.Email != "" && claims.Email != profile.Email {
		log.Warnw("Invitation email mismatch", "tokenEmail", claims.Email, "userEmail", profile.Email)
		c.Error(apperrors.AuthorizationFailed("Invitation not intended for this user"))
		return
	}

	invitationID := claims.InvitationID

	// Update invitation status via model facade
	err = h.tripModel.UpdateInvitationStatus(c.Request.Context(), invitationID, types.InvitationStatusAccepted)
	if err != nil {
		h.handleModelError(c, err)
		return
	}

	// Add member implicitly via the UpdateInvitationStatus logic (should be handled in service)
	// No need to call AddMember separately here if the service does it.

	// Fetch the trip details to return (optional)
	trip, err := h.tripModel.GetTripByID(c.Request.Context(), claims.TripID, acceptingUserID)
	if err != nil {
		log.Warnw("Failed to fetch trip details after accepting invitation", "tripID", claims.TripID, "error", err)
		// Still return success for the acceptance itself
		c.JSON(http.StatusOK, gin.H{"message": "Invitation accepted successfully"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invitation accepted successfully",
		"trip":    trip,
	})
}

// HandleInvitationDeepLink handles clicks from email links
func (h *TripHandler) HandleInvitationDeepLink(c *gin.Context) {
	log := logger.GetLogger()
	token := c.Query("token")
	if token == "" {
		log.Warn("Invitation deep link called without token")
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/invitation-error?error=missing_token", h.config.FrontendURL))
		return
	}

	// Validate the token using the injected config
	claims, err := auth.ValidateInvitationToken(token, h.config.JwtSecretKey)
	if err != nil {
		log.Errorw("Invalid invitation token from deep link", "error", err)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/invitation-error?error=invalid_token", h.config.FrontendURL))
		return
	}

	invitationID := claims.InvitationID
	tripID := claims.TripID

	// Check invitation status - Ensure it's still pending
	invitation, err := h.tripModel.GetInvitation(c.Request.Context(), invitationID)
	if err != nil {
		log.Errorw("Failed to retrieve invitation details for deep link", "invitationID", invitationID, "error", err)
		h.handleModelError(c, err) // Or redirect to a generic error page
		// c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/invitation-error?error=invitation_not_found", h.config.FrontendURL))
		return
	}

	if invitation.Status != types.InvitationStatusPending {
		log.Warnw("Invitation already processed", "invitationID", invitationID, "status", invitation.Status)
		// Redirect based on status? e.g., to the trip page if already accepted
		redirectURL := fmt.Sprintf("%s/trips/%s?invitation_status=%s", h.config.FrontendURL, tripID, strings.ToLower(string(invitation.Status)))
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		return
	}

	// At this point, the token is valid and the invitation is pending.
	// The frontend should handle the actual acceptance logic.
	// We redirect the user to a frontend page that can use the token.
	// This page might prompt login/signup if needed, then call AcceptInvitationHandler.

	redirectURL := fmt.Sprintf("%s/auth/accept-invitation?token=%s", h.config.FrontendURL, token)
	log.Infow("Redirecting valid invitation deep link to frontend handler", "invitationID", invitationID, "tripID", tripID, "redirectURL", redirectURL)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)

	/*
		// --- Alternative: Auto-accept if user is logged in and matches ---
		// This is less common as it bypasses explicit user confirmation on the frontend

		acceptingUserID, exists := c.Get("user_id")
		if !exists {
			// User not logged in, redirect to frontend to handle login/signup then acceptance
			redirectURL := fmt.Sprintf("%s/auth/accept-invitation?token=%s", h.config.FrontendURL, token)
			log.Infow("User not logged in, redirecting invitation deep link", "redirectURL", redirectURL)
			c.Redirect(http.StatusTemporaryRedirect, redirectURL)
			return
		}

		userIDStr := acceptingUserID.(string)

		// Check if logged-in user matches invitation email (requires profile access)
		userProfile, ok := c.Get(middleware.UserProfileKey)
		// ... (fetch profile and check email against claims.Email) ...
		// if !ok || profile.Email != claims.Email {
		//    // Mismatch, redirect to error or standard acceptance page
		//    // ...
		//    return
		// }

		// If logged in user matches, attempt auto-acceptance
		err = h.tripModel.UpdateInvitationStatus(c.Request.Context(), invitationID, types.InvitationStatusAccepted)
		if err != nil {
			log.Errorw("Auto-acceptance failed for deep link", "invitationID", invitationID, "error", err)
			// Redirect to frontend error page
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/invitation-error?error=acceptance_failed", h.config.FrontendURL))
			return
		}

		// Redirect to the trip page after successful auto-acceptance
		log.Infow("Invitation auto-accepted successfully via deep link", "invitationID", invitationID, "tripID", tripID, "userID", userIDStr)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/trips/%s", h.config.FrontendURL, tripID))

	*/
}

// --- Utility Functions ---

func secureRandomFloat() float64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to pseudo-random if crypto/rand fails
		return float64(time.Now().UnixNano()%1000000) / 1000000.0
	}
	// Convert random bytes to a uint64, then scale to [0, 1)
	val := binary.LittleEndian.Uint64(b[:])
	return float64(val) / float64(^uint64(0)) // Divide by max uint64 value
}

// --- Chat related handlers ---

func (h *TripHandler) ListTripMessages(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	// Pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, errL := strconv.Atoi(limitStr)
	offset, errO := strconv.Atoi(offsetStr)

	if errL != nil || errO != nil || limit <= 0 || offset < 0 {
		if err := c.Error(apperrors.ValidationFailed("Invalid pagination parameters", "limit or offset invalid")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Authorization should happen in service/model layer

	// Use injected chatStore
	if h.chatStore == nil {
		log.Errorw("Chat store is not available in handler", "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat service unavailable"})
		return
	}

	messages, err := h.chatStore.GetMessages(c.Request.Context(), tripID, limit, offset)
	if err != nil {
		log.Errorw("Failed to retrieve chat messages", "tripID", tripID, "userID", userID, "error", err)
		h.handleModelError(c, err) // Handle potential db errors
		return
	}

	// Get total message count for pagination headers/metadata (optional)
	totalMessages, err := h.chatStore.GetMessageCount(c.Request.Context(), tripID)
	if err != nil {
		log.Warnw("Failed to get total message count", "tripID", tripID, "error", err)
		// Don't fail the request, just omit the total count
	}

	response := gin.H{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	}
	if err == nil {
		response["total"] = totalMessages
	}

	c.JSON(http.StatusOK, response)
}

type UpdateLastReadRequest struct {
	LastReadMessageID *string    `json:"lastReadMessageId"` // Pointer to allow null/clearing
	LastReadTimestamp *time.Time `json:"lastReadTimestamp"` // Alternatively use timestamp
}

func (h *TripHandler) UpdateLastReadMessage(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id")

	var req UpdateLastReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid request body", err.Error())); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Validate: At least one field should be provided
	if req.LastReadMessageID == nil && req.LastReadTimestamp == nil {
		if err := c.Error(apperrors.ValidationFailed("Invalid request", "Either lastReadMessageId or lastReadTimestamp must be provided")); err != nil {
			log.Errorw("Failed to add validation error", "error", err)
		}
		return
	}

	// Authorization check should happen in service/model layer

	// Use injected chatStore
	if h.chatStore == nil {
		log.Errorw("Chat store is not available in handler", "tripID", tripID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Chat service unavailable"})
		return
	}

	// Determine which update method to call based on request
	var err error
	if req.LastReadMessageID != nil {
		err = h.chatStore.UpdateLastReadMessageID(c.Request.Context(), tripID, userID, *req.LastReadMessageID)
	} else if req.LastReadTimestamp != nil {
		err = h.chatStore.UpdateLastReadTimestamp(c.Request.Context(), tripID, userID, *req.LastReadTimestamp)
	}

	if err != nil {
		log.Errorw("Failed to update last read message indicator", "tripID", tripID, "userID", userID, "error", err)
		h.handleModelError(c, err) // Handle potential db errors
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Last read message updated successfully"})
}

// HandleChatMessage processes incoming WebSocket messages (e.g., new chat messages)
func (h *TripHandler) HandleChatMessage(ctx context.Context, conn *middleware.SafeConn, message []byte, userID, tripID string) error {
	// Ensure logger includes context
	log := logger.FromContext(ctx).With("tripID", tripID, "userID", userID)

	var chatMsg types.ChatMessagePayload
	if err := json.Unmarshal(message, &chatMsg); err != nil {
		log.Warnw("Failed to unmarshal incoming WebSocket message", "error", err, "rawMessage", string(message))
		conn.WriteJSON(gin.H{"error": "Invalid message format", "details": err.Error()}) // Notify client
		return fmt.Errorf("invalid message format: %w", err)
	}

	// Basic validation
	if strings.TrimSpace(chatMsg.Content) == "" {
		log.Warn("Received empty or whitespace-only chat message content")
		conn.WriteJSON(gin.H{"error": "Message content cannot be empty"})
		return fmt.Errorf("empty message content")
	}
	// Add more validation as needed (length limits, etc.)
	const maxChatMsgLength = 2048
	if len(chatMsg.Content) > maxChatMsgLength {
		log.Warnw("Chat message exceeds length limit", "length", len(chatMsg.Content), "limit", maxChatMsgLength)
		conn.WriteJSON(gin.H{"error": fmt.Sprintf("Message exceeds maximum length of %d characters", maxChatMsgLength)})
		return fmt.Errorf("message too long")
	}

	// Create the message object to be stored and broadcasted
	dbMessage := &types.ChatMessage{
		TripID:    tripID,
		UserID:    userID,
		Content:   chatMsg.Content,
		Type:      types.ChatMessageTypeText, // Assuming text for now
		// MessageID and Timestamp will be set by the store/DB
	}

	// Use injected chatStore
	if h.chatStore == nil {
		log.Error("Chat store is not available in HandleChatMessage")
		conn.WriteJSON(gin.H{"error": "Chat service temporarily unavailable"})
		return fmt.Errorf("chat store unavailable")
	}

	// Store the message
	storedMessage, err := h.chatStore.SaveMessage(ctx, dbMessage)
	if err != nil {
		log.Errorw("Failed to save chat message to store", "error", err)
		conn.WriteJSON(gin.H{"error": "Failed to save message", "details": "Internal server error"})
		return fmt.Errorf("failed to save message: %w", err)
	}
	log.Infow("Chat message saved", "messageID", storedMessage.ID)

	// Prepare the event payload (using the stored message)
	eventPayload := types.ChatMessageEvent{
		MessageID: storedMessage.ID,
		TripID:    storedMessage.TripID,
		Content:   storedMessage.Content,
		Timestamp: storedMessage.CreatedAt, // Use timestamp from DB
		// User details might be enriched by subscriber or fetched separately if needed
		User: types.UserResponse{ID: storedMessage.UserID},
	}

	// Marshal the specific payload struct
	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		log.Errorw("Failed to marshal chat message event payload", "error", err, "messageID", storedMessage.ID)
		// Message is saved, but broadcast fails. Log and potentially notify sender.
		conn.WriteJSON(gin.H{"warning": "Message saved, but broadcast failed internally"})
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Create the generic Event wrapper
	chatEvent := types.Event{
		BaseEvent: types.BaseEvent{
			ID:        uuid.New().String(), // Generate event ID
			Type:      types.EventTypeChatMessageSent,
			TripID:    tripID,
			UserID:    userID, // User who sent the message
			Timestamp: storedMessage.CreatedAt, // Align event timestamp with message creation
			Version:   1,
		},
		Metadata: types.EventMetadata{
			Source: "HandleChatMessage",
			// CorrelationID could be passed via message payload if needed
		},
		Payload: payloadBytes, // Use the marshaled specific payload
	}

	// Publish the wrapped event using the injected event service
	err = h.eventService.Publish(ctx, tripID, chatEvent)
	if err != nil {
		log.Errorw("Failed to publish chat message event", "error", err, "messageID", storedMessage.ID, "eventID", chatEvent.ID)
		conn.WriteJSON(gin.H{"error": "Failed to broadcast message", "details": "Internal server error"})
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Infow("Chat message processed and published", "messageID", storedMessage.ID, "eventID", chatEvent.ID)
	// Optionally send confirmation back to the original sender immediately
	// conn.WriteJSON(gin.H{"status": "Message sent", "messageId": storedMessage.ID})

	return nil
}

// --- File Upload Handlers ---

// UploadTripImage handles uploading an image for a specific trip.
func (h *TripHandler) UploadTripImage(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	userID := c.GetString("user_id") // For authorization and associating the upload

	// Authorization check should happen in service layer

	// Retrieve the file from the form-data
	fileHeader, err := c.FormFile("file") // "file" is the name attribute in the form
	if err != nil {
		log.Errorw("Failed to get file from form", "error", err)
		c.Error(apperrors.ValidationFailed("Missing or invalid file in request", err.Error()))
		return
	}

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		log.Errorw("Failed to open uploaded file", "filename", fileHeader.Filename, "error", err)
		c.Error(apperrors.InternalServerError("Failed to process uploaded file"))
		return
	}
	defer file.Close()

	// Read the file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Errorw("Failed to read uploaded file content", "filename", fileHeader.Filename, "error", err)
		c.Error(apperrors.InternalServerError("Failed to read uploaded file"))
		return
	}

	// TODO: Add validation for file type and size here if needed
	// E.g., check http.DetectContentType(fileBytes[:512])
	// E.g., check len(fileBytes) against a limit

	// Define storage path in Supabase Storage
	filePath := fmt.Sprintf("trips/%s/images/%s_%d_%s",
		tripID,
		userID,
		time.Now().UnixNano(),
		strings.ReplaceAll(fileHeader.Filename, " ", "_"), // Basic sanitization
	)

	log.Infow("Attempting to upload file to Supabase Storage", "bucket", "trip_assets", "path", filePath, "size", len(fileBytes))

	// Upload to Supabase Storage using the injected client
	res, err := h.supabase.Storage.UploadFile(c.Request.Context(), "trip_assets", filePath, fileBytes, supabase.FileUploadOptions{
		ContentType: http.DetectContentType(fileBytes),
		Upsert:      false,
	})
	if err != nil {
		log.Errorw("Failed to upload file to Supabase Storage", "bucket", "trip_assets", "path", filePath, "error", err)
		c.Error(apperrors.InternalServerError("Failed to upload image"))
		return
	}

	log.Infow("File uploaded successfully to Supabase Storage", "response", res, "path", filePath)

	// Get the public URL
	publicURL := h.supabase.Storage.GetPublicUrl("trip_assets", filePath)

	// TODO: Persist the image URL associated with the trip (Service layer responsibility)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Image uploaded successfully",
		"imageUrl": publicURL,
		"filePath": filePath,
	})
}

// ListTripImages retrieves a list of images associated with a trip.
func (h *TripHandler) ListTripImages(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	// userID := c.GetString("user_id") // For authorization - should be handled in service/model layer

	// Define the directory path in Supabase Storage
	dirPath := fmt.Sprintf("trips/%s/images", tripID)

	log.Infow("Listing files from Supabase Storage", "bucket", "trip_assets", "path", dirPath)

	// List files from Supabase Storage
	files, err := h.supabase.Storage.ListFiles(c.Request.Context(), "trip_assets", dirPath, supabase.FileSearchOptions{})
	if err != nil {
		log.Errorw("Failed to list files from Supabase Storage", "bucket", "trip_assets", "path", dirPath, "error", err)
		c.Error(apperrors.InternalServerError("Failed to retrieve trip images"))
		return
	}

	// Process the file list to generate public URLs
	type ImageInfo struct {
		Name      string     `json:"name"`
		ID        string     `json:"id"`
		Size      int64      `json:"size"`
		MimeType  string     `json:"mimeType"`
		CreatedAt *time.Time `json:"createdAt"`
		UpdatedAt *time.Time `json:"updatedAt"`
		PublicURL string     `json:"publicUrl"`
		FilePath  string     `json:"filePath"`
	}

	imageList := make([]ImageInfo, 0, len(files))
	for _, file := range files {
		if file.Name == ".emptyFolderPlaceholder" { continue }
		fullPath := dirPath + "/" + file.Name
		publicURL := h.supabase.Storage.GetPublicUrl("trip_assets", fullPath)
		imageList = append(imageList, ImageInfo{
			Name:      file.Name,
			ID:        file.Id,
			Size:      file.Metadata.Size,
			MimeType:  file.Metadata.Mimetype,
			CreatedAt: file.CreatedAt,
			UpdatedAt: file.UpdatedAt,
			PublicURL: publicURL,
			FilePath:  fullPath,
		})
	}

	log.Infow("Successfully listed files", "count", len(imageList), "path", dirPath)

	c.JSON(http.StatusOK, gin.H{
		"images": imageList,
	})
}

// DeleteTripImage handles deleting an image associated with a trip.
func (h *TripHandler) DeleteTripImage(c *gin.Context) {
	log := logger.GetLogger()
	tripID := c.Param("id")
	imageName := c.Param("imageName")
	// userID := c.GetString("user_id") // For authorization - should be handled in service/model layer

	if imageName == "" {
		c.Error(apperrors.ValidationFailed("Missing image name in request path", ""))
		return
	}

	// Authorization check should happen in service layer

	// Construct the full path to the file in Supabase Storage
	filePath := fmt.Sprintf("trips/%s/images/%s", tripID, imageName)

	log.Infow("Attempting to delete file from Supabase Storage", "bucket", "trip_assets", "path", filePath)

	// Delete the file from Supabase Storage
	deletedFiles, err := h.supabase.Storage.DeleteFile(c.Request.Context(), "trip_assets", []string{filePath})
	if err != nil {
		log.Errorw("Failed to delete file from Supabase Storage", "bucket", "trip_assets", "path", filePath, "error", err)
		c.Error(apperrors.InternalServerError("Failed to delete image"))
		return
	}

	if len(deletedFiles) == 0 {
		log.Warnw("File not found or already deleted in Supabase Storage", "path", filePath)
		c.Error(apperrors.NotFound("Image not found", fmt.Sprintf("Image '%s' not found for trip '%s'", imageName, tripID)))
		return
	}

	log.Infow("Successfully deleted file from Supabase Storage", "path", filePath, "deletedInfo", deletedFiles)

	// TODO: Remove the image URL reference from the Trip model/record (Service layer responsibility)

	c.JSON(http.StatusOK, gin.H{
		"message": "Image deleted successfully",
		"filePath": filePath,
	})
}
