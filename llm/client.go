package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Client for interacting with an OpenAI-compatible LLM API
type Client struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
	model      string
}

// NewClient creates a new LLM client
func NewClient() (*Client, error) {
	creds, err := GetCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		endpoint:   BaseURL,
		apiKey:     creds.APIKey,
		model:      creds.Model,
	}, nil
}

// Summarize sends content to the LLM for summarization
func (c *Client) Summarize(ctx context.Context, prompt string) (string, error) {
	reqPayload := OpenAIRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant that generates concise status updates for weekly meeting notes based on project management data. You will receive activities and time log entries annotated with dates and [THIS WEEK] or [context only] tags. Only summarize work labeled [THIS WEEK] and only work done by 'Me' (the current user). Ignore [context only] entries and other team members' contributions entirely. The summary should be one to two sentences, ready to be pasted into a meeting notes document."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.5,
		MaxTokens:   2000,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}

	summary := openAIResp.Choices[0].Message.Content
	summary = cleanResponse(summary)

	return summary, nil
}

// cleanResponse removes unwanted content from the LLM response
func cleanResponse(resp string) string {
	// Remove <think>...</think> blocks
	re := regexp.MustCompile(`(?s)<think>.*</think>`)
	cleaned := re.ReplaceAllString(resp, "")

	// Remove newlines and trim whitespace
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}
