package middleware

import (
    "fmt"
    "net/http"
    "time"
    "strings"

    "github.com/NomadCrew/nomad-crew-backend/config"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "github.com/NomadCrew/nomad-crew-backend/types"
    "github.com/gin-gonic/gin"
    "github.com/lestrrat-go/jwx/v2/jwt"
)

// SupabaseClaims represents the expected claims in a Supabase JWT
type SupabaseClaims struct {
    Subject    string `json:"sub"`
    Email      string `json:"email"`
    Role       string `json:"role"`
    Exp        int64  `json:"exp"`
    AppMetadata struct {
        Provider  string   `json:"provider"`
        Providers []string `json:"providers"`
    } `json:"app_metadata"`
    UserMetadata types.UserMetadata `json:"user_metadata"`
}

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        log := logger.GetLogger()

        // Verify Supabase API key
        apiKey := c.GetHeader("apikey")
        if apiKey != cfg.SupabaseAnonKey {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "Invalid API key",
            })
            return
        }

        // Extract and validate Bearer token
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "Invalid authorization header",
            })
            return
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        
        claims, err := validateSupabaseToken(tokenString)
        if err != nil {
            log.Errorw("Token validation failed", "error", err)
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": err.Error(),
            })
            return
        }

        // Store user information in context
        c.Set("user_id", claims.Subject)
        c.Set("user_email", claims.Email)
        c.Set("user_role", claims.Role)
        c.Set("user_metadata", claims.UserMetadata)

        c.Next()
    }
}

func validateSupabaseToken(tokenString string) (*SupabaseClaims, error) {
    log := logger.GetLogger()
    
    // Parse without verification first to extract claims
    parsed, err := jwt.Parse([]byte(tokenString), jwt.WithVerify(false))
    if err != nil {
        return nil, fmt.Errorf("invalid token format: %v", err)
    }

    // Validate token expiration
    if parsed.Expiration().Before(time.Now()) {
        return nil, fmt.Errorf("token has expired")
    }

    claims := &SupabaseClaims{
        Subject: parsed.Subject(),
        Email:   parsed.PrivateClaims()["email"].(string),
        Role:    parsed.PrivateClaims()["role"].(string),
        Exp:     parsed.Expiration().Unix(),
    }

    // Extract user metadata
    if metadata, ok := parsed.PrivateClaims()["user_metadata"].(map[string]interface{}); ok {
        claims.UserMetadata = types.UserMetadata{
            Username:       getStringValue(metadata, "username"),
            FirstName:     getStringValue(metadata, "firstName"),
            LastName:      getStringValue(metadata, "lastName"),
            ProfilePicture: getStringValue(metadata, "avatar_url"),
        }
    }

    log.Debugw("Validated token claims",
        "subject", claims.Subject,
        "email", claims.Email,
        "role", claims.Role,
    )

    // Validate required claims
    if claims.Subject == "" {
        return nil, fmt.Errorf("missing subject claim")
    }
    if claims.Role != "authenticated" {
        return nil, fmt.Errorf("insufficient permissions")
    }

    return claims, nil
}

// Helper function to safely extract string values from metadata
func getStringValue(m map[string]interface{}, key string) string {
    if val, ok := m[key].(string); ok {
        return val
    }
    return ""
}