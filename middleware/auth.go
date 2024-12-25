package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/NomadCrew/nomad-crew-backend/errors"
    "github.com/NomadCrew/nomad-crew-backend/logger"
    "strings"
    "os"
    "github.com/golang-jwt/jwt"

)

var log = logger.GetLogger()

func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            if err := c.Error(errors.AuthenticationFailed("No authorization header")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }

        tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
        if tokenString == "" {
            if err := c.Error(errors.AuthenticationFailed("Invalid token format")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }

        // Parse and validate JWT token
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(os.Getenv("JWT_SECRET_KEY")), nil
        })

        if err != nil || !token.Valid {
            if err := c.Error(errors.AuthenticationFailed("Invalid token")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }

        // Extract claims
        if claims, ok := token.Claims.(jwt.MapClaims); ok {
            userID := int64(claims["user_id"].(float64))
            c.Set("user_id", userID)
            c.Next()
        } else {
            if err := c.Error(errors.AuthenticationFailed("Invalid token claims")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }
    }
}

func RequireRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, exists := c.Get("user_role")
        if !exists {
            if err := c.Error(errors.AuthenticationFailed("User role not found")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }

        roleStr, ok := userRole.(string)
        if !ok {
            if err := c.Error(errors.AuthenticationFailed("Invalid role type")); err != nil {
                log.Error("Failed to attach error to context: %v", err)
            }
            c.Abort()
            return
        }

        for _, role := range roles {
            if roleStr == role {
                c.Next()
                return
            }
        }

        if err := c.Error(errors.AuthenticationFailed("Insufficient permissions")); err != nil {
            log.Error("Failed to attach error to context: %v", err)
        }
        c.Abort()
    }
}
