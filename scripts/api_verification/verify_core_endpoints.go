// Package main provides a utility script to verify core API endpoints for stability.
// It can be run with: go run scripts/api_verification/verify_core_endpoints.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	baseURLEnvVar     = "API_BASE_URL"
	accessTokenEnvVar = "TEST_ACCESS_TOKEN"
	defaultBaseURL    = "http://localhost:8080"
)

type EndpointTest struct {
	Name           string
	Method         string
	Path           string
	RequiresAuth   bool
	RequestBody    interface{}
	ExpectedStatus int
}

func main() {
	// Get base URL from environment or use default
	baseURL := os.Getenv(baseURLEnvVar)
	if baseURL == "" {
		baseURL = defaultBaseURL
		fmt.Printf("No %s environment variable found, using default: %s\n", baseURLEnvVar, defaultBaseURL)
	}

	// Get authentication token if present
	accessToken := os.Getenv(accessTokenEnvVar)
	if accessToken == "" {
		fmt.Printf("⚠️  No %s environment variable found. Only testing public endpoints.\n", accessTokenEnvVar)
	}

	fmt.Println("NomadCrew Core API Verification Tool")
	fmt.Println("====================================")
	fmt.Printf("Target API: %s\n\n", baseURL)

	// Define core endpoints to test
	coreEndpoints := []EndpointTest{
		// Public endpoints
		{Name: "Health Check", Method: "GET", Path: "/health", RequiresAuth: false, ExpectedStatus: http.StatusOK},
		{Name: "Liveness Check", Method: "GET", Path: "/health/liveness", RequiresAuth: false, ExpectedStatus: http.StatusOK},
		{Name: "Readiness Check", Method: "GET", Path: "/health/readiness", RequiresAuth: false, ExpectedStatus: http.StatusOK},

		// Auth required endpoints - only tested if access token is provided
		{Name: "Current User", Method: "GET", Path: "/v1/users/me", RequiresAuth: true, ExpectedStatus: http.StatusOK},
		{Name: "User Trips", Method: "GET", Path: "/v1/trips", RequiresAuth: true, ExpectedStatus: http.StatusOK},
		{Name: "User Notifications", Method: "GET", Path: "/v1/notifications", RequiresAuth: true, ExpectedStatus: http.StatusOK},
	}

	// Run tests and track results
	successCount := 0
	totalCount := 0

	for _, test := range coreEndpoints {
		// Skip auth-required tests if no token provided
		if test.RequiresAuth && accessToken == "" {
			fmt.Printf("⏭️  Skipping %s (requires authentication)\n", test.Name)
			continue
		}

		totalCount++
		fmt.Printf("🔍 Testing %s... ", test.Name)

		success, statusCode, _ := testEndpoint(baseURL, test, accessToken)

		if success {
			successCount++
			fmt.Printf("✅ Success (HTTP %d)\n", statusCode)
		} else {
			fmt.Printf("❌ Failed (HTTP %d, expected %d)\n", statusCode, test.ExpectedStatus)
		}
	}

	// Print summary
	fmt.Println("\nTest Summary:")
	fmt.Printf("Passed: %d/%d\n", successCount, totalCount)

	if successCount == totalCount {
		fmt.Println("✅ All tested endpoints are working as expected!")
	} else {
		fmt.Println("❌ Some endpoints failed testing.")
		os.Exit(1) // Non-zero exit code for failed tests
	}
}

func testEndpoint(baseURL string, test EndpointTest, accessToken string) (bool, int, string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create request
	var req *http.Request
	var err error

	if test.RequestBody != nil {
		jsonBody, _ := json.Marshal(test.RequestBody)
		req, err = http.NewRequest(test.Method, baseURL+test.Path, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequest(test.Method, baseURL+test.Path, nil)
	}

	if err != nil {
		return false, 0, fmt.Sprintf("Error creating request: %v", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	if test.RequiresAuth && accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, fmt.Sprintf("Error executing request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)

	// Check if status code matches expected
	return resp.StatusCode == test.ExpectedStatus, resp.StatusCode, string(body)
}
