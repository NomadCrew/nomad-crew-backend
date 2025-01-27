package middleware

import (
    "context"
    "fmt"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/redis/go-redis/v9"
)

func RateLimiter(redisClient *redis.Client, requests int, duration time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        key := fmt.Sprintf("rate_limit:%s", userID)
        ctx := context.Background()

        // Initialize counter if not exists
        exists, err := redisClient.Exists(ctx, key).Result()
        if err != nil {
            c.AbortWithStatus(500)
            return
        }

        if exists == 0 {
            pipe := redisClient.Pipeline()
            pipe.SetNX(ctx, key, 1, duration)
            pipe.Expire(ctx, key, duration)
            _, err = pipe.Exec(ctx)
            if err != nil {
                c.AbortWithStatus(500)
                return
            }
        } else {
            // Increment counter
            count, err := redisClient.Incr(ctx, key).Result()
            if err != nil {
                c.AbortWithStatus(500)
                return
            }

            if count > int64(requests) {
                c.AbortWithStatusJSON(429, gin.H{
                    "error": "Too many requests",
                    "retry_after": duration.Seconds(),
                })
                return
            }
        }

        c.Next()
    }
}