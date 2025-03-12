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
			// Add support for pattern matching (e.g., *.nomadcrew.uk)
			corsConfig.AllowOriginFunc = func(origin string) bool {
				for _, allowedOrigin := range cfg.AllowedOrigins {
					if allowedOrigin == origin {
						return true
					}
					// Handle wildcard subdomains
					if strings.HasPrefix(allowedOrigin, "*.") {
						domain := strings.TrimPrefix(allowedOrigin, "*")
						if strings.HasSuffix(origin, domain) {
							return true
						}
					}
				}
				return false
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
