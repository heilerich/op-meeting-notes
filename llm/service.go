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
	sb.WriteString(fmt.Sprintf("The summary week covers %s to %s.\n", input.PeriodStart, input.PeriodEnd))
	sb.WriteString("IMPORTANT: Only include work done during THIS WEEK in your summary. Entries labeled [context only] are provided as background information and must NOT be included in the summary.\n")
	sb.WriteString("IMPORTANT: Only summarize work done by 'Me' (the current user). Ignore work done by other team members.\n\n")
	sb.WriteString("Description:\n")
	sb.WriteString(input.Description + "\n\n")
	sb.WriteString("Status: ")
	sb.WriteString(input.Status + "\n\n")

	if len(input.Activities) > 0 {
		sb.WriteString("Work Package Comments/Activities:\n")
		for _, activity := range input.Activities {
			label := getLabel(activity.Timestamp, input.PeriodStart, input.PeriodEnd)
			sb.WriteString(fmt.Sprintf("- %s: %s\n", label, activity.Content))
		}
		sb.WriteString("\n")
	}

	if len(input.TimeEntryComments) > 0 {
		sb.WriteString("My Time Log Entries for This Week (by Me, all within the summary week):\n")
		for _, comment := range input.TimeEntryComments {
			dateLabel := formatSpentOnLabel(comment.Timestamp)
			sb.WriteString(fmt.Sprintf("- %s: %s\n", dateLabel, comment.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Provide a concise summary of one to two sentences covering only what 'Me' accomplished THIS WEEK. The context will be clear to the reader, so no need to explain the work package or its purpose. Any answer that contains the phrase 'work package' or repeats the work package title or description is definitely too long. Good examples would be 'moved remaining servers to NEZ, got network card from old ESX server', 'setup complete, waiting for firewall', 'meeting with John Doe: firewall was misconfigured'.\n")
	return sb.String()
}

// getCacheKey generates a SHA256 hash for the input content to use as a cache key
func getCacheKey(input SummarizationInput) string {
	var content strings.Builder
	content.WriteString(input.PeriodStart)
	content.WriteString(input.PeriodEnd)
	content.WriteString(input.Description)
	for _, activity := range input.Activities {
		content.WriteString(activity.Timestamp)
		content.WriteString(activity.Content)
	}
	for _, comment := range input.TimeEntryComments {
		content.WriteString(comment.Timestamp)
		content.WriteString(comment.Content)
	}

	hash := sha256.Sum256([]byte(content.String()))
	return hex.EncodeToString(hash[:])
}

// getLabel returns a formatted date label indicating whether the activity falls within the summary week
func getLabel(timestamp, periodStart, periodEnd string) string {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Sprintf("UNKNOWN DATE [context only]")
	}

	ps, err := time.Parse("2006-01-02", periodStart)
	if err != nil {
		return fmt.Sprintf("UNKNOWN DATE [context only]")
	}

	pe, err := time.Parse("2006-01-02", periodEnd)
	if err != nil {
		return fmt.Sprintf("UNKNOWN DATE [context only]")
	}
	// Include the full end day
	pe = pe.Add(24*time.Hour - time.Second)

	dateStr := ts.Format("Mon 2006-01-02")
	if ts.Before(ps) || ts.After(pe) {
		return fmt.Sprintf("%s [context only]", dateStr)
	}
	return fmt.Sprintf("%s [THIS WEEK]", dateStr)
}

// formatSpentOnLabel formats a "2006-01-02" date string as "Mon 2006-01-02"
func formatSpentOnLabel(spentOn string) string {
	t, err := time.Parse("2006-01-02", spentOn)
	if err != nil {
		t, err = time.Parse(time.RFC3339, spentOn)
		if err != nil {
			return spentOn
		}
	}
	return t.Format("Mon 2006-01-02")
}
