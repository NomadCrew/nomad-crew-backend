package middleware

import (
	"strings"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware creates a middleware for handling CORS with the given configuration.
// It properly uses gin-contrib/cors without manual header manipulation.
func CORSMiddleware(cfg *config.ServerConfig) gin.HandlerFunc {
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
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	}

	// Configure allowed origins based on configuration
	if len(cfg.AllowedOrigins) == 0 || containsOrigin(cfg.AllowedOrigins, "*") {
		// Allow all origins (development mode or explicitly configured)
		// Note: When AllowAllOrigins is true, AllowCredentials must be false per CORS spec
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowCredentials = false
	} else {
		// Production mode with specific allowed origins
		corsConfig.AllowAllOrigins = false
		corsConfig.AllowCredentials = true

		// Check if any origin uses wildcard subdomain pattern (e.g., "*.example.com")
		hasWildcardPattern := false
		for _, origin := range cfg.AllowedOrigins {
			if strings.HasPrefix(origin, "*.") {
				hasWildcardPattern = true
				break
			}
		}

		if hasWildcardPattern {
			// Use AllowOriginFunc for wildcard subdomain support
			corsConfig.AllowOriginFunc = func(origin string) bool {
				// Empty origin is allowed (same-origin requests)
				if origin == "" {
					return true
				}

				for _, allowedOrigin := range cfg.AllowedOrigins {
					// Exact match
					if allowedOrigin == origin {
						return true
					}

					// Wildcard subdomain match (e.g., "*.example.com")
					if strings.HasPrefix(allowedOrigin, "*.") {
						domain := strings.TrimPrefix(allowedOrigin, "*")
						if strings.HasSuffix(origin, domain) {
							return true
						}
					}
				}

				return false
			}
		} else {
			// Simple exact match - use AllowOrigins for better performance
			corsConfig.AllowOrigins = cfg.AllowedOrigins
		}
	}

	return cors.New(corsConfig)
}

// containsOrigin checks if a string is present in the allowed origins slice.
func containsOrigin(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
