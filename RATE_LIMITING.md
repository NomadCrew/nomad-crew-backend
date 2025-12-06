# Rate Limiting Implementation

## Overview

This document describes the rate limiting implementation for the NomadCrew backend API, specifically for authentication endpoints to prevent brute force attacks.

## Implementation Details

### Architecture

The rate limiting system uses:
- **Redis** for distributed rate limit tracking
- **IP-based limiting** for unauthenticated endpoints
- **Sliding window** approach with Redis INCR and EXPIRE commands
- **Graceful degradation** if Redis is unavailable

### Components

#### 1. Configuration (`config/config.go`)

Rate limiting configuration is managed through environment variables:

```go
type RateLimitConfig struct {
    AuthRequestsPerMinute int // Max requests per minute for auth endpoints
    WindowSeconds         int // Time window in seconds
}
```

**Environment Variables:**
- `RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE` (default: 10)
- `RATE_LIMIT_WINDOW_SECONDS` (default: 60)

#### 2. Middleware (`middleware/rate_limit.go`)

**`AuthRateLimiter`** - Main rate limiting middleware
- Limits requests based on client IP address
- Uses Redis pipelines for atomic operations
- Returns 429 Too Many Requests when limit exceeded
- Includes standard rate limit headers

**`getClientIP`** - IP extraction utility
- Checks `X-Forwarded-For` header (for proxies/load balancers)
- Falls back to `X-Real-IP` header
- Uses `RemoteAddr` as final fallback

### Protected Endpoints

Currently, rate limiting is applied to:
- `POST /v1/users/onboard` - User registration/onboarding

### Rate Limit Headers

All responses from rate-limited endpoints include:

```http
X-RateLimit-Limit: 10           # Maximum requests allowed
X-RateLimit-Remaining: 7        # Requests remaining in window
X-RateLimit-Reset: 1701234567   # Unix timestamp when limit resets
Retry-After: 42                 # Seconds until retry (when blocked)
```

### Error Response

When rate limit is exceeded:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1701234567
Retry-After: 42

{
  "error": "Too many requests. Please try again later.",
  "retry_after": 42
}
```

## Configuration

### Environment Setup

Add to your `.env` file:

```bash
# Rate Limiting Configuration
RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE=10
RATE_LIMIT_WINDOW_SECONDS=60
```

### Recommended Settings

| Environment | Requests/Minute | Window (seconds) | Notes |
|-------------|----------------|------------------|-------|
| Development | 100 | 60 | Relaxed for testing |
| Staging | 20 | 60 | Moderate protection |
| Production | 10 | 60 | Strict brute force protection |

## Usage

### Adding Rate Limiting to New Endpoints

In `router/router.go`:

```go
// Create rate limiter instance
authRateLimiter := middleware.AuthRateLimiter(
    deps.RedisClient,
    deps.Config.RateLimit.AuthRequestsPerMinute,
    time.Duration(deps.Config.RateLimit.WindowSeconds)*time.Second,
)

// Apply to route
v1.POST("/auth/login", authRateLimiter, handler.Login)
```

### Testing Rate Limiting

Use the provided test suite:

```bash
go test ./middleware -v -run TestAuthRateLimiter
```

Example manual test:

```bash
# Make multiple requests quickly
for i in {1..15}; do
  curl -X POST http://localhost:8080/v1/users/onboard \
    -H "Content-Type: application/json" \
    -d '{"email":"test@example.com"}' \
    -i
done
```

## Implementation Benefits

1. **Brute Force Protection**: Prevents automated attacks on auth endpoints
2. **Distributed**: Works across multiple server instances using Redis
3. **Graceful Degradation**: API remains available if Redis fails
4. **Standard Headers**: Follows RFC 6585 for rate limiting
5. **IP Intelligence**: Properly handles proxied requests
6. **Configurable**: Easy to adjust limits per environment

## Security Considerations

### IP Spoofing Prevention

The implementation checks headers in priority order:
1. `X-Forwarded-For` (first IP in chain)
2. `X-Real-IP`
3. `RemoteAddr`

**Important**: Ensure your load balancer/proxy is configured to:
- Set `X-Forwarded-For` correctly
- Strip client-provided forwarding headers

### Redis Security

- Use Redis password authentication in production
- Enable TLS for Redis connections (already configured for Upstash)
- Regularly monitor Redis memory usage
- Set appropriate key expiration to prevent memory leaks

### Bypass Protection

To prevent rate limit bypass:
- Don't allow client-provided IP headers to override proxied IPs
- Monitor for unusual patterns (distributed attacks)
- Consider additional layers (WAF, CDN rate limiting)

## Monitoring

### Redis Keys

Rate limit data is stored with keys:
```
ratelimit:auth:{ip_address}
```

### Metrics to Monitor

1. **Rate limit hits**: Count of 429 responses
2. **Redis latency**: INCR/EXPIRE operation timing
3. **Blocked IPs**: Frequently blocked addresses
4. **False positives**: Legitimate users hitting limits

### Debugging

Check current rate limit for an IP:

```bash
redis-cli GET "ratelimit:auth:192.168.1.1"
redis-cli TTL "ratelimit:auth:192.168.1.1"
```

List all rate limit keys:

```bash
redis-cli KEYS "ratelimit:auth:*"
```

## Future Enhancements

Potential improvements:

1. **Dynamic Rate Limits**: Adjust based on user reputation
2. **Account-Based Limiting**: Track authenticated user limits
3. **Distributed Rate Limiting**: More sophisticated algorithms (token bucket)
4. **Allowlisting**: Bypass for trusted IPs/services
5. **Analytics Dashboard**: Visualize rate limit patterns
6. **Custom Limits per Endpoint**: Different limits for different endpoints

## Troubleshooting

### Rate limiting not working

1. Check Redis connection:
   ```bash
   redis-cli PING
   ```

2. Verify configuration loaded:
   ```bash
   # Check logs for:
   # "Configuration loaded" with rate_limit settings
   ```

3. Ensure Redis client passed to router:
   ```go
   // In main.go, check:
   RedisClient: redisClient
   ```

### Legitimate users being blocked

1. Increase `RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE`
2. Review proxy configuration (IP detection)
3. Check for shared IPs (NAT, corporate networks)
4. Consider implementing user-specific rate limits

### Redis memory issues

1. Monitor key count:
   ```bash
   redis-cli DBSIZE
   ```

2. Verify TTL is set correctly:
   ```bash
   redis-cli TTL "ratelimit:auth:192.168.1.1"
   ```

3. Adjust window duration if needed

## References

- [RFC 6585 - Additional HTTP Status Codes](https://tools.ietf.org/html/rfc6585)
- [OWASP - Blocking Brute Force Attacks](https://owasp.org/www-community/controls/Blocking_Brute_Force_Attacks)
- [Redis Rate Limiting Patterns](https://redis.io/docs/manual/patterns/rate-limiter/)

## Change Log

### 2025-11-28
- Initial implementation of auth rate limiting
- Added IP-based limiting for `/v1/users/onboard`
- Implemented graceful Redis failure handling
- Added comprehensive test suite
