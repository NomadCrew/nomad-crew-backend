package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Verify API key
        apiKey := c.GetHeader("apikey")
        if apiKey != cfg.SupabaseAnonKey {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            c.Abort()
            return
        }

        // Verify user JWT
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
            c.Abort()
            return
        }

        token := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := validateSupabaseToken(token)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
            c.Abort()
            return
        }

        // Set authenticated user info
        c.Set("user_id", claims.Subject)
        c.Set("user_email", claims.Email)
        c.Set("user_role", claims.Role)
        c.Next()
    }
}

type SupabaseClaims struct {
    Subject string `json:"sub"`
    Email   string `json:"email"`
    Role    string `json:"role"`
}

func validateSupabaseToken(tokenString string) (*SupabaseClaims, error) {
    log := logger.GetLogger()
    parsed, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
    if err != nil {
        return nil, fmt.Errorf("invalid token format")
    }

    // Get claims safely with type assertions
    email, ok := parsed.PrivateClaims()["email"].(string)
    if !ok {
        return nil, fmt.Errorf("invalid email claim")
    }

    role, ok := parsed.PrivateClaims()["role"].(string)
    if !ok {
        return nil, fmt.Errorf("invalid role claim")
    }

    claims := &SupabaseClaims{
        Subject: parsed.Subject(),
        Email:   email,
        Role:    role,
    }

    log.Info("Supabase claims: ", claims.Subject)

    if claims.Role != "authenticated" {
        return nil, fmt.Errorf("insufficient permissions")
    }

    // Validate subject is present
    if claims.Subject == "" {
        return nil, fmt.Errorf("invalid user ID")
    }

    return claims, nil
}