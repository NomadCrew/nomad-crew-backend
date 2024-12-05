package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/NomadCrew/nomad-crew-backend/errors"
    "strings"
    "os"
    "github.com/golang-jwt/jwt"
)

func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.Error(errors.AuthenticationFailed("No authorization header"))
            c.Abort()
            return
        }

        tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
        if tokenString == "" {
            c.Error(errors.AuthenticationFailed("Invalid token format"))
            c.Abort()
            return
        }

        // Parse and validate JWT token
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(os.Getenv("JWT_SECRET_KEY")), nil
        })

        if err != nil || !token.Valid {
            c.Error(errors.AuthenticationFailed("Invalid token"))
            c.Abort()
            return
        }

        // Extract claims
        if claims, ok := token.Claims.(jwt.MapClaims); ok {
            userID := int64(claims["user_id"].(float64))
            c.Set("user_id", userID)
            c.Next()
        } else {
            c.Error(errors.AuthenticationFailed("Invalid token claims"))
            c.Abort()
            return
        }
    }
}