package middleware

import (
	"net/http"
	"os"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure
type WSClaims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

// isWSSimulatorBypassEnabled checks if simulator bypass should be allowed for WebSocket.
// Only enabled when SERVER_ENVIRONMENT is "development".
func isWSSimulatorBypassEnabled() bool {
	env := os.Getenv("SERVER_ENVIRONMENT")
	return env == "development"
}

// WSJwtAuth provides optimized JWT authentication middleware for WebSocket connections
func WSJwtAuth(validator Validator) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger()
		startTime := time.Now()

		// Extract token from query param or Sec-WebSocket-Protocol header
		tokenString := c.Query("token")
		if tokenString == "" {
			tokenString = c.GetHeader("Sec-WebSocket-Protocol")
		}

		if tokenString == "" {
			log.Warnw("WebSocket auth failed: missing token",
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication token"})
			return
		}

		// DEVELOPMENT ONLY: Check for simulator bypass
		// This allows iOS simulator development without real authentication
		if isWSSimulatorBypassEnabled() && isSimulatorToken(tokenString) {
			log.Warnw("WEBSOCKET SIMULATOR BYPASS ACTIVE - Using mock authentication (development only)",
				"path", c.Request.URL.Path)

			// Store simulator userID in context
			c.Set(string(UserIDKey), simulatorUserID)

			// Log auth processing time
			authTime := time.Since(startTime)
			log.Debugw("WebSocket auth successful (simulator bypass)",
				"userID", simulatorUserID,
				"path", c.Request.URL.Path,
				"duration_ms", authTime.Milliseconds())

			c.Next()
			return
		}

		// Validate token
		userID, err := validator.Validate(tokenString)
		if err != nil {
			log.Warnw("WebSocket auth failed: invalid token",
				"error", err,
				"path", c.Request.URL.Path,
				"ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
			return
		}

		// Store userID in context
		c.Set(string(UserIDKey), userID)

		// Log auth processing time
		authTime := time.Since(startTime)
		log.Debugw("WebSocket auth successful",
			"userID", userID,
			"path", c.Request.URL.Path,
			"duration_ms", authTime.Milliseconds())

		c.Next()
	}
}
