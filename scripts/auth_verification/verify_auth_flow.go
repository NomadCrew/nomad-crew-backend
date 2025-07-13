// Package main provides a utility script to verify the authentication flow end-to-end.
// It can be run with: go run scripts/auth_verification/verify_auth_flow.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURLEnvVar      = "API_BASE_URL"
	refreshTokenEnvVar = "TEST_REFRESH_TOKEN"
	defaultBaseURL     = "http://localhost:8080"
)

type TestResult struct {
	Name       string
	Success    bool
	Message    string
	StatusCode int
	Response   string
}

func main() {
	// Get base URL from environment or use default
	baseURL := os.Getenv(baseURLEnvVar)
	if baseURL == "" {
		baseURL = defaultBaseURL
		fmt.Printf("No %s environment variable found, using default: %s\n", baseURLEnvVar, defaultBaseURL)
	}

	fmt.Println("NomadCrew Auth Flow Verification Tool")
	fmt.Println("=====================================")
	fmt.Printf("Target API: %s\n\n", baseURL)

	// Run test suite
	results := []TestResult{}

	// Test health endpoint
	results = append(results, testHealthEndpoint(baseURL))

	// Test auth flow
	refreshToken := os.Getenv(refreshTokenEnvVar)
	if refreshToken == "" {
		fmt.Printf("⚠️  No %s environment variable found. Skipping token refresh test.\n", refreshTokenEnvVar)
		fmt.Println("To fully test the auth flow, set a valid refresh token in this environment variable.")
	} else {
		results = append(results, testTokenRefresh(baseURL, refreshToken))

		// Get a new access token for authenticated endpoint tests
		accessToken := getAccessToken(baseURL, refreshToken)
		if accessToken != "" {
			results = append(results, testAuthenticatedEndpoint(baseURL, accessToken))
		} else {
			fmt.Println("⚠️  Failed to get access token. Skipping authenticated endpoint tests.")
		}
	}

	// Print results summary
	printResults(results)
}

func testHealthEndpoint(baseURL string) TestResult {
	fmt.Println("🔍 Testing health endpoint...")

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return TestResult{
			Name:    "Health Check",
			Success: false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return TestResult{
		Name:       "Health Check",
		Success:    resp.StatusCode == http.StatusOK,
		Message:    fmt.Sprintf("Status: %d", resp.StatusCode),
		StatusCode: resp.StatusCode,
		Response:   string(body),
	}
}

func testTokenRefresh(baseURL string, refreshToken string) TestResult {
	fmt.Println("🔍 Testing token refresh...")

	// Create request payload
	payload := map[string]string{
		"refresh_token": refreshToken,
	}
	jsonPayload, _ := json.Marshal(payload)

	// Create request
	req, err := http.NewRequest("POST", baseURL+"/v1/auth/refresh", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return TestResult{
			Name:    "Token Refresh",
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			Name:    "Token Refresh",
			Success: false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Check for successful response (should contain access_token)
	success := resp.StatusCode == http.StatusOK && strings.Contains(string(body), "access_token")

	return TestResult{
		Name:       "Token Refresh",
		Success:    success,
		Message:    fmt.Sprintf("Status: %d", resp.StatusCode),
		StatusCode: resp.StatusCode,
		Response:   maskTokensInResponse(string(body)),
	}
}

func getAccessToken(baseURL string, refreshToken string) string {
	// Create request payload
	payload := map[string]string{
		"refresh_token": refreshToken,
	}
	jsonPayload, _ := json.Marshal(payload)

	// Create request
	req, err := http.NewRequest("POST", baseURL+"/v1/auth/refresh", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	// Parse response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		return ""
	}

	return tokenResponse.AccessToken
}

func testAuthenticatedEndpoint(baseURL string, accessToken string) TestResult {
	fmt.Println("🔍 Testing authenticated endpoint...")

	// Create request for a protected endpoint
	req, err := http.NewRequest("GET", baseURL+"/v1/users/me", nil)
	if err != nil {
		return TestResult{
			Name:    "Authenticated Request",
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			Name:    "Authenticated Request",
			Success: false,
			Message: fmt.Sprintf("Failed to connect: %v", err),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return TestResult{
		Name:       "Authenticated Request",
		Success:    resp.StatusCode == http.StatusOK,
		Message:    fmt.Sprintf("Status: %d", resp.StatusCode),
		StatusCode: resp.StatusCode,
		Response:   string(body),
	}
}

func maskTokensInResponse(response string) string {
	// Redact the actual token values for logging
	response = maskJSONValue(response, "access_token")
	response = maskJSONValue(response, "refresh_token")
	return response
}

func maskJSONValue(jsonStr, key string) string {
	keyWithQuotes := fmt.Sprintf("\"%s\":\"", key)
	if idx := strings.Index(jsonStr, keyWithQuotes); idx != -1 {
		startIdx := idx + len(keyWithQuotes)
		endIdx := strings.Index(jsonStr[startIdx:], "\"")
		if endIdx != -1 {
			endIdx += startIdx
			// Show a few characters at the beginning if the token is long enough
			token := jsonStr[startIdx:endIdx]
			var maskedToken string
			if len(token) > 10 {
				maskedToken = token[:3] + "..." + token[len(token)-3:]
			} else {
				maskedToken = "***"
			}
			return jsonStr[:startIdx] + maskedToken + jsonStr[endIdx:]
		}
	}
	return jsonStr
}

func printResults(results []TestResult) {
	fmt.Println("\nTest Results:")
	fmt.Println("============")

	for _, result := range results {
		var status string
		if result.Success {
			status = "✅ PASS"
		} else {
			status = "❌ FAIL"
		}

		fmt.Printf("%s | %s | %s\n", status, result.Name, result.Message)

		// Print detailed response for failures
		if !result.Success {
			fmt.Printf("    Response: %s\n", result.Response)
		}
	}

	fmt.Println("\nSummary:")
	allPassed := true
	for _, result := range results {
		if !result.Success {
			allPassed = false
			break
		}
	}

	if allPassed {
		fmt.Println("✅ All tests passed!")
	} else {
		fmt.Println("❌ Some tests failed.")
	}
}
