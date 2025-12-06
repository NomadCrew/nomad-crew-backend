package middleware

import (
	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security-related HTTP headers to all responses.
// These headers help protect against common web vulnerabilities like clickjacking,
// XSS attacks, and MIME type sniffing.
func SecurityHeadersMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// X-Frame-Options: Prevents clickjacking attacks by disallowing the page
		// from being embedded in frames, iframes, or objects
		c.Header("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevents MIME type sniffing by forcing browsers
		// to respect the declared Content-Type
		c.Header("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection: Enables the browser's built-in XSS filter
		// (legacy header, but still useful for older browsers)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Controls how much referrer information is sent with requests
		// strict-origin-when-cross-origin sends full URL for same-origin, origin only for cross-origin HTTPS
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS): Forces HTTPS connections for the specified duration
		// Only enable in production to avoid issues during local development
		if cfg.IsProduction() {
			// max-age=31536000 (1 year), includeSubDomains applies to all subdomains
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
