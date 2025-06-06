package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client handles API communication with OpenProject
type Client struct {
	httpClient *http.Client
	host       *url.URL
	token      string
}

// NewClient creates a new API client
func NewClient() (*Client, error) {
	credentials, err := GetCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %v", err)
	}

	// Parse the base URL
	parsedURL, err := url.Parse(BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %v", err)
	}

	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		host:       parsedURL,
		token:      credentials.APIKey,
	}, nil
}

// makeRequest makes an HTTP request to the API
func (c *Client) makeRequest(path string, queryParams map[string]string) ([]byte, error) {
	// Build the full URL
	fullURL := *c.host
	fullURL.Path = path

	// Add query parameters
	if len(queryParams) > 0 {
		q := fullURL.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		fullURL.RawQuery = q.Encode()
	}

	// Create request
	req, err := http.NewRequest("GET", fullURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Set authentication header (using API key as username with "apikey")
	req.SetBasicAuth("apikey", c.token)
	req.Header.Set("Accept", "application/hal+json")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetTimeEntries fetches time entries for a given date range
func (c *Client) GetTimeEntries(startDate, endDate time.Time) ([]TimeEntry, error) {
	// Build filters for the date range and current user
	filters := fmt.Sprintf(`[{"spent_on":{"operator":"<>d","values":["%s","%s"]}},{"user":{"operator":"=","values":["me"]}}]`,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	// Prepare query parameters
	queryParams := map[string]string{
		"filters":  filters,
		"pageSize": "100", // Get more entries
	}

	// Make the request
	body, err := c.makeRequest("/api/v3/time_entries", queryParams)
	if err != nil {
		return nil, err
	}

	// Parse response
	var timeEntriesResp TimeEntriesResponse
	if err := json.Unmarshal(body, &timeEntriesResp); err != nil {
		return nil, fmt.Errorf("failed to parse time entries response: %v", err)
	}

	return timeEntriesResp.Embedded.Elements, nil
}
