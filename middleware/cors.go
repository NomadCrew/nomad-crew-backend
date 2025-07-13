package middleware

import (
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware creates a middleware for handling CORS with the given configuration
func CORSMiddleware(cfg *config.ServerConfig) gin.HandlerFunc {
	// Default configuration
	corsConfig := cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Length",
			"Content-Type",
			"Authorization",
			"X-Requested-With",
			"Accept",
			"X-CSRF-Token",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// Set allowed origins
	if len(cfg.AllowedOrigins) > 0 {
		if containsOrigin(cfg.AllowedOrigins, "*") {
			corsConfig.AllowAllOrigins = true
		} else {
			corsConfig.AllowOrigins = cfg.AllowedOrigins

			// For test compatibility: Default to AllowAllOrigins and use custom origin check
			corsConfig.AllowAllOrigins = true

			// Use custom handler to set CORS headers
			return func(c *gin.Context) {
				origin := c.Request.Header.Get("Origin")

				// When no origin is provided, we need to set the Access-Control-Allow-Origin to *
				if origin == "" {
					c.Header("Access-Control-Allow-Origin", "*")
					c.Header("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowMethods, ", "))
					c.Header("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowHeaders, ", "))
					c.Header("Access-Control-Allow-Credentials", "true")
					c.Header("Access-Control-Max-Age", "43200") // 12 hours in seconds

					c.Next()
					return
				}

				// Check if origin is allowed
				allowed := false
				for _, allowedOrigin := range cfg.AllowedOrigins {
					if allowedOrigin == origin {
						allowed = true
						break
					}
					// Handle wildcard subdomains
					if strings.HasPrefix(allowedOrigin, "*.") {
						domain := strings.TrimPrefix(allowedOrigin, "*")
						if strings.HasSuffix(origin, domain) {
							allowed = true
							break
						}
					}
				}

				// Set appropriate CORS headers based on if origin is allowed
				if allowed {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowMethods, ", "))
					c.Header("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowHeaders, ", "))
					c.Header("Access-Control-Allow-Credentials", "true")
					c.Header("Access-Control-Max-Age", "43200") // 12 hours in seconds
					c.Header("Vary", "Origin")

					// Handle preflight requests
					if c.Request.Method == "OPTIONS" {
						c.AbortWithStatus(204)
						return
					}
				}

				// Always allow the request to proceed - we just don't set CORS headers for disallowed origins
				c.Next()
			}
		}
	} else {
		corsConfig.AllowAllOrigins = true
	}

	return cors.New(corsConfig)
}

// containsOrigin checks if a string is present in the allowed origins slice
func containsOrigin(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
