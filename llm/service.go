package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service handles the business logic for LLM summarization
type Service struct {
	client *Client
	cache  *sync.Map
}

// NewService creates a new summarization service
func NewService() (*Service, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}
	return &Service{
		client: client,
		cache:  &sync.Map{},
	}, nil
}

// SummarizeWorkPackages processes a batch of work packages in parallel
func (s *Service) SummarizeWorkPackages(ctx context.Context, inputs []SummarizationInput) map[int]string {
	var wg sync.WaitGroup
	resultsChan := make(chan SummarizationResult, len(inputs))

	for _, input := range inputs {
		wg.Add(1)
		go func(input SummarizationInput) {
			defer wg.Done()
			summary, err := s.getSummary(ctx, input)
			resultsChan <- SummarizationResult{
				WorkPackageID: input.WorkPackageID,
				Summary:       summary,
				Error:         err,
			}
		}(input)
	}

	wg.Wait()
	close(resultsChan)

	summaries := make(map[int]string)
	for result := range resultsChan {
		if result.Error != nil {
			// On error, we'll have an empty summary, and the caller can decide what to do.
			fmt.Printf("Failed to summarize work package %d: %v\n", result.WorkPackageID, result.Error)
			continue
		}
		summaries[result.WorkPackageID] = result.Summary
	}
	return summaries
}

// getSummary retrieves a summary from cache or by calling the LLM
func (s *Service) getSummary(ctx context.Context, input SummarizationInput) (string, error) {
	cacheKey := getCacheKey(input)
	if cached, found := s.cache.Load(cacheKey); found {
		if entry, ok := cached.(CacheEntry); ok {
			return entry.Summary, nil
		}
	}

	prompt := buildPrompt(input)
	summary, err := s.client.Summarize(ctx, prompt)
	if err != nil {
		return "", err
	}

	s.cache.Store(cacheKey, CacheEntry{Summary: summary, Timestamp: time.Now().Unix()})
	return summary, nil
}

// buildPrompt creates the summarization prompt for the LLM
func buildPrompt(input SummarizationInput) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Please summarize the following work package titled '%s'.\n", input.Subject))
	sb.WriteString(fmt.Sprintf("The user is interested in the period starting from %s. Your summary should focus on activities and comments from this period.\n\n", input.PeriodStart))
	sb.WriteString("You should provide a concise summary of the work done by the user while focusing on the time log entries and comments made by the user.\n\n")
	sb.WriteString("Description:\n")
	sb.WriteString(input.Description + "\n\n")
	sb.WriteString("Status: ")
	sb.WriteString(input.Status + "\n\n")

	if len(input.Activities) > 0 {
		sb.WriteString("Recent Activities/Comments:\n")
		for _, activity := range input.Activities {
			label := getLabel(activity.Timestamp, input.PeriodStart)
			sb.WriteString(fmt.Sprintf("- %s: %s\n", label, activity.Content))
		}
		sb.WriteString("\n")
	}

	if len(input.TimeEntryComments) > 0 {
		sb.WriteString("Associated Time Entry Comments:\n")
		for _, comment := range input.TimeEntryComments {
			sb.WriteString(fmt.Sprintf("- %s\n", comment.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Provide a concise summary of one to two sentences. The context will be clear to the reader, so no need to explain the work package or its purpose. Any answer that contains the phrase 'work package' or the repeats the work package title or description is definitely too long. Good examples would be 'moved remaining servers to NEZ, got network card from old ESX server', 'setup complete, waiting for firewall', 'meeting with John Doe: firewall was misconfigured'.\n")
	return sb.String()
}

// getCacheKey generates a SHA256 hash for the input content to use as a cache key
func getCacheKey(input SummarizationInput) string {
	var content strings.Builder
	content.WriteString(input.Description)
	for _, activity := range input.Activities {
		content.WriteString(activity.Content)
	}
	for _, comment := range input.TimeEntryComments {
		content.WriteString(comment.Content)
	}

	hash := sha256.Sum256([]byte(content.String()))
	return hex.EncodeToString(hash[:])
}

// getLabel determines if the timestamp is before or within the period of interest
func getLabel(timestamp, periodStart string) string {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Fallback for invalid timestamps
		return "UNKNOWN"
	}

	ps, err := time.Parse("2006-01-02", periodStart)
	if err != nil {
		// Fallback for invalid period start
		return "UNKNOWN"
	}

	if ts.Before(ps) {
		return "Before Period"
	}
	return "In Period"
}
