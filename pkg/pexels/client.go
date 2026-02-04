package pexels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/NomadCrew/nomad-crew-backend/logger"
)

const pexelsAPIBaseURL = "https://api.pexels.com/v1"

// ClientInterface defines the interface for Pexels client operations
type ClientInterface interface {
	SearchDestinationImage(ctx context.Context, query string) (string, error)
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type SearchResponse struct {
	Photos []Photo `json:"photos"`
}

type Photo struct {
	ID     int    `json:"id"`
	Source Source `json:"src"`
}

type Source struct {
	Landscape string `json:"landscape"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (c *Client) SearchDestinationImage(ctx context.Context, query string) (string, error) {
	logger.GetLogger().Debugw("Starting Pexels image search", "query", query)

	endpoint := fmt.Sprintf("%s/search", pexelsAPIBaseURL)

	// Build query params
	params := url.Values{}
	params.Add("query", query)
	params.Add("per_page", "1")
	params.Add("orientation", "landscape")

	finalURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())
	logger.GetLogger().Debugw("Constructed Pexels search URL", "url", finalURL)

	req, err := http.NewRequestWithContext(ctx, "GET", finalURL, nil)
	if err != nil {
		logger.GetLogger().Errorw("Failed to create Pexels request", "error", err)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.apiKey)

	logger.GetLogger().Debug("Executing Pexels HTTP request")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.GetLogger().Errorw("Failed to execute Pexels HTTP request", "error", err)
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	logger.GetLogger().Debugw("Pexels HTTP response received", "statusCode", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		logger.GetLogger().Warnw("Pexels API returned non-OK status", "statusCode", resp.StatusCode)
		return "", fmt.Errorf("pexels API returned status: %d", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		logger.GetLogger().Errorw("Failed to decode Pexels response", "error", err)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	logger.GetLogger().Debugw("Pexels response decoded", "photosReturned", len(searchResp.Photos))
	if len(searchResp.Photos) == 0 {
		logger.GetLogger().Debug("No photos found in Pexels response", "query", query)
		return "", nil
	}

	imageURL := searchResp.Photos[0].Source.Landscape
	logger.GetLogger().Debugw("Returning background image URL from Pexels", "imageURL", imageURL)
	return imageURL, nil
}
