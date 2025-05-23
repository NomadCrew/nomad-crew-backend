package handlers

import (
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ChatHandlerSupabase handles chat-related HTTP requests with Supabase Realtime integration
type ChatHandlerSupabase struct {
	tripService     TripServiceInterface
	supabaseService *services.SupabaseService
	logger          *zap.SugaredLogger
	featureFlags    config.FeatureFlags
}

// NewChatHandlerSupabase creates a new instance of ChatHandlerSupabase
func NewChatHandlerSupabase(
	tripService TripServiceInterface,
	supabaseService *services.SupabaseService,
	featureFlags config.FeatureFlags,
) *ChatHandlerSupabase {
	return &ChatHandlerSupabase{
		tripService:     tripService,
		supabaseService: supabaseService,
		logger:          logger.GetLogger(),
		featureFlags:    featureFlags,
	}
}

// SendMessage handles sending a new chat message
// @Summary Send a chat message
// @Description Sends a new message to a trip chat
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param message body ChatMessageRequest true "Message data"
// @Success 201 {object} ChatMessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages [post]
func (h *ChatHandlerSupabase) SendMessage(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// Parse the request
	var req struct {
		Message   string  `json:"message" binding:"required,min=1,max=1000"`
		ReplyToID *string `json:"replyToId,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid message data: " + err.Error(),
		})
		return
	}

	// Generate a new UUID for the message
	messageID := uuid.New().String()

	// Check if we should use Supabase
	if h.featureFlags.EnableSupabaseRealtime && h.supabaseService != nil {
		// Send to Supabase
		now := time.Now()
		err := h.supabaseService.SendChatMessage(c.Request.Context(), services.ChatMessage{
			ID:        messageID,
			TripID:    tripID,
			UserID:    userID,
			Message:   req.Message,
			ReplyToID: req.ReplyToID,
			CreatedAt: now,
		})

		if err != nil {
			h.logger.Error("Failed to send message to Supabase", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to send message",
			})
			return
		}

		// Return success
		c.JSON(http.StatusCreated, gin.H{
			"id":        messageID,
			"tripId":    tripID,
			"userId":    userID,
			"message":   req.Message,
			"replyToId": req.ReplyToID,
			"createdAt": now,
		})
		return
	}

	// If Supabase is not enabled, respond with an error
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Chat via Supabase is not enabled",
	})
}

// GetMessages handles retrieving chat messages
// @Summary Get chat messages
// @Description Retrieves messages from a trip chat with pagination
// @Tags chat
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param limit query int false "Maximum number of messages to return" default(50)
// @Param before query string false "Return messages before this message ID (for pagination)"
// @Success 200 {array} ChatMessageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages [get]
func (h *ChatHandlerSupabase) GetMessages(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// For the Supabase implementation, we simply return an empty array
	// The actual chat messages will be retrieved by the client directly from Supabase
	c.JSON(http.StatusOK, []gin.H{})
}

// AddReaction handles adding a reaction to a message
// @Summary Add a reaction
// @Description Adds an emoji reaction to a chat message
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param reaction body ChatReactionRequest true "Reaction data"
// @Success 201 {object} ChatReactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages/{messageId}/reactions [post]
func (h *ChatHandlerSupabase) AddReaction(c *gin.Context) {
	tripID := c.Param("tripId")
	messageID := c.Param("messageId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" || messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID and Message ID are required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// Parse the request
	var req struct {
		Emoji string `json:"emoji" binding:"required,max=10"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid reaction data: " + err.Error(),
		})
		return
	}

	// Check if we should use Supabase
	if h.featureFlags.EnableSupabaseRealtime && h.supabaseService != nil {
		// Send to Supabase
		err := h.supabaseService.AddChatReaction(c.Request.Context(), services.ChatReaction{
			MessageID: messageID,
			UserID:    userID,
			Emoji:     req.Emoji,
		})

		if err != nil {
			h.logger.Error("Failed to add reaction in Supabase", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to add reaction",
			})
			return
		}

		// Return success
		c.JSON(http.StatusCreated, gin.H{
			"messageId": messageID,
			"userId":    userID,
			"emoji":     req.Emoji,
		})
		return
	}

	// If Supabase is not enabled, respond with an error
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Chat reactions via Supabase are not enabled",
	})
}

// RemoveReaction handles removing a reaction from a message
// @Summary Remove a reaction
// @Description Removes an emoji reaction from a chat message
// @Tags chat
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param messageId path string true "Message ID"
// @Param emoji path string true "Emoji to remove"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/messages/{messageId}/reactions/{emoji} [delete]
func (h *ChatHandlerSupabase) RemoveReaction(c *gin.Context) {
	tripID := c.Param("tripId")
	messageID := c.Param("messageId")
	emoji := c.Param("emoji")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" || messageID == "" || emoji == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID, Message ID, and Emoji are required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// Check if we should use Supabase
	if h.featureFlags.EnableSupabaseRealtime && h.supabaseService != nil {
		// Remove from Supabase
		err := h.supabaseService.RemoveChatReaction(c.Request.Context(), services.ChatReaction{
			MessageID: messageID,
			UserID:    userID,
			Emoji:     emoji,
		})

		if err != nil {
			h.logger.Error("Failed to remove reaction from Supabase", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to remove reaction",
			})
			return
		}

		// Return success
		c.JSON(http.StatusOK, gin.H{
			"status": "Reaction removed",
		})
		return
	}

	// If Supabase is not enabled, respond with an error
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "Chat reactions via Supabase are not enabled",
	})
}

// UpdateReadStatus handles updating the user's read status
// @Summary Update read status
// @Description Updates the user's last read message for a trip
// @Tags chat
// @Accept json
// @Produce json
// @Param tripId path string true "Trip ID"
// @Param status body ChatReadStatusRequest true "Read status data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/trips/{tripId}/chat/read-status [put]
func (h *ChatHandlerSupabase) UpdateReadStatus(c *gin.Context) {
	tripID := c.Param("tripId")
	userID := c.GetString(string(middleware.UserIDKey))

	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Trip ID is required",
		})
		return
	}

	// Verify user is a member of the trip
	member, err := h.tripService.GetTripMember(c.Request.Context(), tripID, userID)
	if err != nil || member == nil || member.DeletedAt != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You are not an active member of this trip",
		})
		return
	}

	// Parse the request
	var req struct {
		LastReadMessageID string `json:"lastReadMessageId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid read status data: " + err.Error(),
		})
		return
	}

	// For the Supabase implementation, we don't need to implement this on the backend
	// The client will update read status directly in Supabase
	c.JSON(http.StatusOK, gin.H{
		"status": "Read status updated",
	})
}
