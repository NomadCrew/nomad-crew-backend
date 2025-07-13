package middleware

import (
	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds essential security headers to all responses
// This middleware protects against common web vulnerabilities including:
// - Clickjacking (X-Frame-Options)
// - MIME type sniffing (X-Content-Type-Options)
// - XSS attacks (X-XSS-Protection, Content-Security-Policy)
// - Man-in-the-middle attacks (Strict-Transport-Security)
func SecurityHeaders(cfg *config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")
		
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection in older browsers
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Content Security Policy - restrictive by default
		// Allows resources only from the same origin
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " + // Allow inline scripts for Swagger UI
			"style-src 'self' 'unsafe-inline'; " + // Allow inline styles for Swagger UI
			"img-src 'self' data: https:; " + // Allow images from self, data URIs, and HTTPS
			"font-src 'self' data:; " + // Allow fonts from self and data URIs
			"connect-src 'self' " + cfg.FrontendURL + " wss: https:; " + // Allow API and WebSocket connections
			"frame-ancestors 'none';" // Reinforce frame options
		
		c.Header("Content-Security-Policy", csp)
		
		// Force HTTPS in production
		if cfg.Environment == config.EnvProduction {
			// HSTS - Force HTTPS for 1 year, include subdomains
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		
		// Referrer Policy - Don't leak referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy (formerly Feature Policy)
		// Disable potentially dangerous features
		c.Header("Permissions-Policy", "geolocation=(self), microphone=(), camera=()")
		
		// Additional security headers
		c.Header("X-Permitted-Cross-Domain-Policies", "none")
		
		c.Next()
	}
}