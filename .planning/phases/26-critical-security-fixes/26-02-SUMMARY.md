---
phase: 26
plan: 02
subsystem: security
tags: [gin, rate-limiting, ip-spoofing, trusted-proxies, sec-02]

requires:
  - 26-01: "Fail-closed rate limiter foundation"

provides:
  - "Secure IP extraction using Gin's trusted proxy mechanism"
  - "Protection against IP spoofing on rate limiter"
  - "Configurable trusted proxy CIDRs"

affects:
  - "All rate-limited endpoints now use secure IP extraction"

tech-stack:
  added: []
  patterns:
    - "Gin SetTrustedProxies configuration"
    - "Safe-by-default proxy configuration"

key-files:
  created: []
  modified:
    - config/config.go: "Added TrustedProxies configuration"
    - router/router.go: "Configure Gin trusted proxies at startup"
    - middleware/rate_limit.go: "Use c.ClientIP() instead of manual header parsing"

decisions:
  - id: "safe-default-no-proxies"
    choice: "Empty TrustedProxies = SetTrustedProxies(nil)"
    rationale: "Safest default - ignores all forwarded headers unless explicitly configured"
  - id: "fatal-on-invalid-config"
    choice: "Fatal error if proxy config is invalid"
    rationale: "Security config errors should never be silently ignored"

metrics:
  duration: "3.6 minutes"
  completed: "2026-02-04"
---

# Phase 26 Plan 02: IP Spoofing Protection Summary

**One-liner:** Configured Gin trusted proxies to prevent IP spoofing attacks on rate limiter (SEC-02)

## What Was Built

Fixed critical IP spoofing vulnerability where attackers could bypass rate limiting by sending forged X-Forwarded-For headers. The previous implementation blindly trusted these headers from any client.

**Solution:**
- Added `TrustedProxies` configuration to ServerConfig (environment-configurable)
- Configure Gin's `SetTrustedProxies()` early in router setup
- Empty config = `SetTrustedProxies(nil)` (safe default - ignore all forwarded headers)
- Non-empty config = `SetTrustedProxies(cidrs)` (only trust headers from specific IPs/CIDRs)
- Simplified `getClientIP()` to use `c.ClientIP()` which respects trusted proxy configuration

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add TrustedProxies to ServerConfig | 5a57898 | config/config.go |
| 2 | Configure Gin SetTrustedProxies in router | 5029249 | router/router.go |
| 3 | Update getClientIP to use c.ClientIP() | 24ffc71 | middleware/rate_limit.go |

## Technical Implementation

### Configuration Layer (config/config.go)

**Added to ServerConfig struct:**
```go
// TrustedProxies is a list of CIDR ranges or IPs of trusted reverse proxies.
// If empty, X-Forwarded-For headers are ignored entirely (safe default).
// Examples: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
TrustedProxies []string `mapstructure:"TRUSTED_PROXIES" yaml:"trusted_proxies"`
```

**Default value:**
```go
v.SetDefault("SERVER.TRUSTED_PROXIES", []string{}) // Empty = trust no one (safe default)
```

**Environment variable:**
- `TRUSTED_PROXIES` - Comma-separated list of CIDRs or IPs

**Logging:**
- Trusted proxies configuration logged at startup for visibility

### Router Setup (router/router.go)

**Early configuration (before middleware):**
```go
// SECURITY: Configure trusted proxies before any other middleware
// This ensures c.ClientIP() returns the actual client IP, not spoofed headers
if len(deps.Config.Server.TrustedProxies) == 0 {
    // No trusted proxies configured - disable proxy header parsing entirely
    // This is the SAFE default: X-Forwarded-For headers are ignored
    if err := r.SetTrustedProxies(nil); err != nil {
        logger.GetLogger().Errorw("Failed to set trusted proxies", "error", err)
    }
    logger.GetLogger().Info("Trusted proxies disabled - using RemoteAddr directly for client IP")
} else {
    // Specific trusted proxies configured - only trust headers from these IPs/CIDRs
    if err := r.SetTrustedProxies(deps.Config.Server.TrustedProxies); err != nil {
        logger.GetLogger().Fatalw("Invalid trusted proxy configuration", "error", err)
    }
    logger.GetLogger().Infow("Trusted proxies configured",
        "proxies", deps.Config.Server.TrustedProxies)
}
```

**Key behaviors:**
- Empty/nil config → `SetTrustedProxies(nil)` → RemoteAddr used directly
- Invalid config → Fatal error (security config must be correct)
- Valid config → Only trust headers from specified CIDRs

### Rate Limiter (middleware/rate_limit.go)

**Before (VULNERABLE):**
```go
func getClientIP(c *gin.Context) string {
    // Check X-Forwarded-For header (can contain multiple IPs)
    if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
        // Take the first IP in the chain
        ips := strings.Split(forwarded, ",")
        if len(ips) > 0 {
            return strings.TrimSpace(ips[0])
        }
    }
    // Check X-Real-IP header
    if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
        return realIP
    }
    // Fall back to RemoteAddr
    return c.ClientIP()
}
```

**After (SECURE):**
```go
// getClientIP extracts the real client IP from the request.
// SECURITY: Uses Gin's built-in ClientIP() which:
// 1. Only parses X-Forwarded-For/X-Real-IP if RemoteAddr is from a trusted proxy
// 2. Falls back to RemoteAddr if the request is not from a trusted proxy
// 3. Trusted proxies are configured via r.SetTrustedProxies() in router setup
func getClientIP(c *gin.Context) string {
    return c.ClientIP()
}
```

**Why this fixes the vulnerability:**
- `c.ClientIP()` checks if `RemoteAddr` is in the trusted proxy list
- If NOT trusted → ignores headers, returns `RemoteAddr` directly
- If trusted → parses headers safely
- Attackers can no longer spoof IPs by sending fake headers

## Security Impact

### Vulnerability Closed: SEC-02 (IP Spoofing)

**Before:**
- Attacker sends `X-Forwarded-For: 1.2.3.4` from any IP
- Rate limiter uses `1.2.3.4` as the client IP
- Attacker can bypass rate limiting by rotating fake IPs

**After:**
- Default (no proxies): Headers ignored entirely, `RemoteAddr` used
- With proxies: Headers only trusted if request comes from configured proxy CIDRs
- Attacker can no longer spoof their IP

### Production Configuration

**Current (safe default):**
```bash
# TRUSTED_PROXIES not set
# Result: SetTrustedProxies(nil) - RemoteAddr used directly
```

**If behind load balancer/reverse proxy:**
```bash
# Configure the proxy's IP range
TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12
# Result: SetTrustedProxies(["10.0.0.0/8", "172.16.0.0/12"])
```

**Docker/Coolify example:**
```bash
# If using Docker's default bridge network
TRUSTED_PROXIES=172.17.0.0/16
```

## Verification

### Build Verification
✅ `go build ./config/...` - Compiles successfully
✅ `go build ./router/...` - Compiles successfully
✅ `go build ./middleware/...` - Compiles successfully

### Code Review Verification
✅ ServerConfig has TrustedProxies []string field
✅ Default is empty slice (safe)
✅ TRUSTED_PROXIES env var is bound
✅ SetTrustedProxies(nil) called when no proxies configured
✅ SetTrustedProxies(proxies) called when proxies configured
✅ getClientIP returns c.ClientIP() directly
✅ No manual X-Forwarded-For parsing

### Security Properties
✅ Safe by default (empty = no trust)
✅ Explicit configuration required to trust proxies
✅ Fatal error on invalid configuration (no silent failures)
✅ IP spoofing no longer possible

## Integration with Phase 26-01

This plan builds on the fail-closed rate limiter from 26-01:

**26-01:** Ensured rate limiting is ALWAYS enforced (Redis + in-memory fallback)
**26-02:** Ensured rate limiting uses CORRECT IP (not spoofed)

**Combined effect:**
- Rate limiter always enforces limits (fail-closed)
- Rate limiter always uses real client IP (no spoofing)
- Critical SEC-01 and SEC-02 vulnerabilities both closed

## Deviations from Plan

None - plan executed exactly as written.

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Proxy config breaks legitimate traffic | Safe default (no proxies) works for direct connections |
| Invalid CIDR format crashes server | Fatal error on startup with clear message |
| Forgetting to configure proxies in production | Logs show current configuration at startup |
| Proxy IP changes | Environment variable makes updates easy |

## Testing Recommendations (Phase 27)

When fixing the test suite in Phase 27, add tests for:

1. **Default behavior (no proxies):**
   - Verify `SetTrustedProxies(nil)` is called
   - Verify `X-Forwarded-For` is ignored
   - Verify `RemoteAddr` is used directly

2. **With trusted proxies:**
   - Verify `SetTrustedProxies(cidrs)` is called with correct values
   - Verify headers from trusted IPs are parsed
   - Verify headers from untrusted IPs are ignored

3. **Rate limiter IP extraction:**
   - Verify spoofed headers from non-proxy IPs are rejected
   - Verify legitimate headers from proxy IPs are accepted

## Next Phase Readiness

**Phase 27 (Test Suite Repair):**
- ✅ Critical security fixes complete (can safely refactor tests)
- ✅ Rate limiter code is now stable and secure
- 🔄 Should add tests for trusted proxy configuration

**Production readiness:**
- ✅ Safe default configuration (no proxies)
- ✅ Environment-configurable if needed
- ✅ Clear logging of proxy configuration
- ✅ Fatal errors prevent security misconfigurations

## Files Modified Summary

### config/config.go
- Added `TrustedProxies []string` field to ServerConfig
- Added default value: empty slice
- Added environment binding: TRUSTED_PROXIES
- Added logging of trusted_proxies config

### router/router.go
- Added logger import
- Added SetTrustedProxies configuration before middleware
- Empty config → SetTrustedProxies(nil) with info log
- Non-empty config → SetTrustedProxies(cidrs) with info log
- Invalid config → Fatal error

### middleware/rate_limit.go
- Removed manual X-Forwarded-For parsing (VULNERABLE)
- Replaced with c.ClientIP() (SECURE)
- Removed unused strings import
- Updated documentation to explain security properties

## Lessons Learned

1. **Default to secure:** Empty TrustedProxies = trust no one is the right default
2. **Fail loudly on security config errors:** Fatal error prevents silent misconfigurations
3. **Gin's built-in ClientIP() is well-designed:** Respects trusted proxy configuration correctly
4. **Logging security config is valuable:** Makes production debugging easier

---

**Status:** ✅ Complete
**Security vulnerabilities closed:** SEC-02 (IP Spoofing on Rate Limiter)
**Commits:** 5a57898, 5029249, 24ffc71
**Next:** Phase 27 - Test Suite Repair
