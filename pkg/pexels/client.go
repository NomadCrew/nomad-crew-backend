package pexels

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

const pexelsAPIBaseURL = "https://api.pexels.com/v1"

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

func (c *Client) SearchDestinationImage(destination string) (string, error) {
    endpoint := fmt.Sprintf("%s/search", pexelsAPIBaseURL)
    
    // Build query params
    params := url.Values{}
    params.Add("query", fmt.Sprintf("%s travel destination", destination))
    params.Add("per_page", "1")
    params.Add("orientation", "landscape")
    
    req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, params.Encode()), nil)
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", c.apiKey)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("failed to execute request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("pexels API returned status: %d", resp.StatusCode)
    }

    var searchResp SearchResponse
    if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
        return "", fmt.Errorf("failed to decode response: %w", err)
    }

    if len(searchResp.Photos) == 0 {
        return "", nil // No images found
    }

    return searchResp.Photos[0].Source.Landscape, nil
}