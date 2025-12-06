# Rate Limiting Quick Reference

## Quick Start

### Environment Variables
```bash
RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE=10
RATE_LIMIT_WINDOW_SECONDS=60
```

### Protected Endpoints
- `POST /v1/users/onboard` - 10 requests/minute by default

### Testing
```bash
# Run tests
go test ./middleware -v -run TestAuthRateLimiter

# Manual test (should block after 10 requests)
for i in {1..15}; do
  curl -X POST http://localhost:8080/v1/users/onboard \
    -H "Content-Type: application/json" \
    -d '{"email":"test@example.com"}' \
    -i | grep -E "(HTTP|X-RateLimit)"
done
```

### Response Headers
```http
X-RateLimit-Limit: 10           # Max requests allowed
X-RateLimit-Remaining: 7        # Requests remaining
X-RateLimit-Reset: 1701234567   # Reset time (Unix timestamp)
Retry-After: 42                 # Seconds to wait (when blocked)
```

### Redis Keys
```
ratelimit:auth:{ip_address}
```

### Debug Commands
```bash
# Check rate limit for an IP
redis-cli GET "ratelimit:auth:192.168.1.1"

# Check TTL
redis-cli TTL "ratelimit:auth:192.168.1.1"

# List all rate limit keys
redis-cli KEYS "ratelimit:auth:*"

# Clear rate limit for an IP
redis-cli DEL "ratelimit:auth:192.168.1.1"
```

### Adding to New Endpoints
```go
// In router/router.go
authRateLimiter := middleware.AuthRateLimiter(
    deps.RedisClient,
    deps.Config.RateLimit.AuthRequestsPerMinute,
    time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
)

v1.POST("/auth/login", authRateLimiter, handler.Login)
```

### Recommended Settings

| Environment | Requests/Min | Window |
|-------------|--------------|--------|
| Development | 100          | 60s    |
| Staging     | 20           | 60s    |
| Production  | 10           | 60s    |

### Error Response (429)
```json
{
  "error": "Too many requests. Please try again later.",
  "retry_after": 42
}
```

### Full Documentation
See `RATE_LIMITING.md` for complete details.
