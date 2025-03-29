package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/middleware"
	"github.com/NomadCrew/nomad-crew-backend/services"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WSHandler handles WebSocket connections
type WSHandler struct {
	rateLimitService *services.RateLimitService
	eventService     types.EventPublisher
}

// NewWSHandler creates a new WebSocket handler
func NewWSHandler(rateLimitService *services.RateLimitService, eventService types.EventPublisher) *WSHandler {
	return &WSHandler{
		rateLimitService: rateLimitService,
		eventService:     eventService,
	}
}

// EnforceWSRateLimit applies rate limiting to WebSocket connections
func (h *WSHandler) EnforceWSRateLimit(userID string, actionType string, limit int) error {
	key := fmt.Sprintf("ws:%s:%s", actionType, userID)
	allowed, retryAfter, err := h.rateLimitService.CheckLimit(context.Background(), key, limit, 1*time.Minute)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("rate limit exceeded, retry after %v", retryAfter)
	}
	return nil
}

// HandleWebSocketConnection creates an optimized WebSocket handler with proper error handling and rate limiting
func (h *WSHandler) HandleWebSocketConnection(c *gin.Context) {
	// Get user ID from context (set by WSJwtAuth middleware)
	userID, exists := c.Get(string(middleware.UserIDKey))
	if !exists || userID == "" {
		zap.L().Warn("WebSocket connection attempt without authenticated user",
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Apply rate limiting
	if err := h.EnforceWSRateLimit(userID.(string), "connect", 5); err != nil {
		zap.L().Warn("WebSocket rate limit exceeded",
			zap.String("userID", userID.(string)),
			zap.Error(err))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":       "Too many connections",
			"retry_after": 60, // seconds
		})
		return
	}

	// Configure WebSocket upgrader with size limits
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Maintain current origin policy
		},
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("WebSocket upgrade failed",
			zap.String("userID", userID.(string)),
			zap.String("ip", c.ClientIP()),
			zap.Error(err))
		return
	}

	// Create a safe connection wrapper
	safeConn := middleware.NewSafeConn(conn, nil, middleware.DefaultWSConfig())
	safeConn.UserID = userID.(string)

	// Set connection in context for downstream handlers
	c.Set("wsConnection", safeConn)

	// Log successful connection
	zap.L().Info("WebSocket connection established",
		zap.String("userID", userID.(string)),
		zap.String("remoteAddr", conn.RemoteAddr().String()),
		zap.String("path", c.Request.URL.Path))

	// Continue to the next handler which will use the WebSocket connection
	c.Next()
}
