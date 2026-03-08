package models

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Achsion/iso8601/v2"
	"github.com/heilerich/op-meeting-notes/api"
	"github.com/heilerich/op-meeting-notes/llm"
)

// TimeEntryService handles business logic for time entries
type TimeEntryService struct {
	client *api.Client
}

// NewTimeEntryService creates a new time entry service
func NewTimeEntryService(client *api.Client) *TimeEntryService {
	return &TimeEntryService{
		client: client,
	}
}

// GroupedTimeEntry represents time entries grouped by project and work package
type GroupedTimeEntry struct {
	ProjectTitle      string
	WorkPackageID     int
	WorkPackageTitle  string
	WorkPackageClosed bool
	CombinedComment   string
	LLMSummary        string // New field for the summary
	TotalHours        float64
	ActivityHours     map[string]float64 // Hours broken down by activity type (e.g. "Development", "Support")
	TimeEntryComments []llm.TimeEntryComment
	Representative    api.TimeEntry
}

// GetTimeEntriesForWeek fetches and groups time entries for a specific week
func (s *TimeEntryService) GetTimeEntriesForWeek(week string) ([]GroupedTimeEntry, error) {
	// Calculate date range based on selected week
	var startDate, endDate time.Time
	now := time.Now()

	if week == "Current week" {
		// Current week (Monday to Sunday)
		startDate = startOfWeek(now)
		endDate = startDate.AddDate(0, 0, 6)
	} else {
		// Last week
		lastWeek := now.AddDate(0, 0, -7)
		startDate = startOfWeek(lastWeek)
		endDate = startDate.AddDate(0, 0, 6)
	}

	// Fetch time entries from API
	timeEntries, err := s.client.GetTimeEntries(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Group entries by project and work package
	return s.groupTimeEntries(timeEntries), nil
}

// groupTimeEntries groups time entries by project and work package
func (s *TimeEntryService) groupTimeEntries(entries []api.TimeEntry) []GroupedTimeEntry {
	type entryKey struct {
		projectTitle     string
		workPackageID    int
		workPackageTitle string
	}

	groupedEntries := make(map[entryKey][]api.TimeEntry)
	for _, entry := range entries {
		// Extract work package ID from the href if the direct ID is not available
		workPackageID := extractWorkPackageIDFromHref(entry.Links.WorkPackage.Href)

		key := entryKey{
			projectTitle:     entry.Links.Project.Title,
			workPackageID:    workPackageID,
			workPackageTitle: entry.Links.WorkPackage.Title,
		}
		groupedEntries[key] = append(groupedEntries[key], entry)
	}

	// Convert grouped entries to structured format
	result := make([]GroupedTimeEntry, 0, len(groupedEntries))
	for key, entries := range groupedEntries {
		// Collect all comments for this work package
		var comments []string
		var timeEntryComments []llm.TimeEntryComment
		var totalHours float64
		activityHours := make(map[string]float64)

		for _, entry := range entries {
			if entry.Comment.Raw != "" {
				comments = append(comments, entry.Comment.Raw)
				timeEntryComments = append(timeEntryComments, llm.TimeEntryComment{
					Content:   entry.Comment.Raw,
					Timestamp: entry.SpentOn,
				})
			}
			// Parse hours and add to total
			duration, err := iso8601.ParseToDuration(entry.Hours)
			if err != nil {
				fmt.Println("conv error: %s", err)
				continue
			}
			hours := duration.Hours()
			totalHours += hours

			// Track hours by activity type
			activityType := entry.Links.Activity.Title
			if activityType == "" {
				activityType = "Other"
			}
			activityHours[activityType] += hours
		}

		// Create the combined comment
		combinedComment := "(no comments)"
		if len(comments) > 0 {
			combinedComment = strings.Join(comments, ", ")
		}

		result = append(result, GroupedTimeEntry{
			ProjectTitle:      key.projectTitle,
			WorkPackageID:     key.workPackageID,
			WorkPackageTitle:  key.workPackageTitle,
			CombinedComment:   combinedComment,
			TotalHours:        totalHours,
			ActivityHours:     activityHours,
			TimeEntryComments: timeEntryComments,
			Representative:    entries[0], // Use first entry as representative
		})
	}

	// Sort by project name first, then by work package title
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProjectTitle != result[j].ProjectTitle {
			return result[i].ProjectTitle < result[j].ProjectTitle
		}
		return result[i].WorkPackageTitle < result[j].WorkPackageTitle
	})

	return result
}

// Helper function to extract work package ID from href URL
func extractWorkPackageIDFromHref(href string) int {
	if href == "" {
		return 0
	}

	// URL format is typically /api/v3/work_packages/123
	parts := strings.Split(href, "/")
	if len(parts) > 0 {
		if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return id
		}
	}

	return 0
}

// FormatEntryTitle creates a formatted title for a grouped time entry
func (entry *GroupedTimeEntry) FormatTitle() string {
	workPackageInfo := fmt.Sprintf("WP #%d", entry.WorkPackageID)
	if entry.WorkPackageTitle != "" {
		workPackageInfo = entry.WorkPackageTitle
	}

	return fmt.Sprintf("Project %s: %s - %s",
		entry.ProjectTitle,
		workPackageInfo,
		entry.CombinedComment)
}

// Helper function to get start of week (Monday)
func startOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday)).Truncate(24 * time.Hour)
}

// UpdateWorkPackageClosedStatus checks the status of work packages and updates the WorkPackageClosed field
// This function is optimized to batch and parallelize API calls for better performance
func (s *TimeEntryService) UpdateWorkPackageClosedStatus(entries []GroupedTimeEntry) error {
	// Step 1: Collect unique work package IDs
	uniqueWorkPackageIDs := make(map[int]bool)
	for _, entry := range entries {
		if entry.WorkPackageID != 0 {
			uniqueWorkPackageIDs[entry.WorkPackageID] = true
		}
	}

	if len(uniqueWorkPackageIDs) == 0 {
		return nil // Nothing to process
	}

	// Convert to slice for iteration
	workPackageIDs := make([]int, 0, len(uniqueWorkPackageIDs))
	for id := range uniqueWorkPackageIDs {
		workPackageIDs = append(workPackageIDs, id)
	}

	// Step 2: Fetch all work packages in parallel
	workPackageResults := make(map[int]*api.WorkPackage)
	workPackageErrors := make(map[int]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, id := range workPackageIDs {
		wg.Add(1)
		go func(workPackageID int) {
			defer wg.Done()

			workPackage, err := s.client.GetWorkPackage(workPackageID)

			mu.Lock()
			if err != nil {
				workPackageErrors[workPackageID] = err
			} else {
				workPackageResults[workPackageID] = workPackage
			}
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	// Step 3: Collect unique status hrefs from successfully fetched work packages
	uniqueStatusHrefs := make(map[string]bool)
	statusHrefToWorkPackageIDs := make(map[string][]int) // Track which work packages use each status

	for id, workPackage := range workPackageResults {
		if workPackage.Links.Status.Href != "" {
			href := workPackage.Links.Status.Href
			uniqueStatusHrefs[href] = true
			statusHrefToWorkPackageIDs[href] = append(statusHrefToWorkPackageIDs[href], id)
		}
	}

	// Step 4: Fetch all statuses in parallel
	statusResults := make(map[string]*api.Status)
	statusErrors := make(map[string]error)

	for href := range uniqueStatusHrefs {
		wg.Add(1)
		go func(statusHref string) {
			defer wg.Done()

			status, err := s.client.GetStatus(statusHref)

			mu.Lock()
			if err != nil {
				statusErrors[statusHref] = err
			} else {
				statusResults[statusHref] = status
			}
			mu.Unlock()
		}(href)
	}
	wg.Wait()

	// Step 5: Build final status cache
	statusCache := make(map[int]bool)

	for href, status := range statusResults {
		workPackageIDsForStatus := statusHrefToWorkPackageIDs[href]
		for _, workPackageID := range workPackageIDsForStatus {
			statusCache[workPackageID] = status.IsClosed
		}
	}

	// Step 6: Update entries with cached status information
	for i := range entries {
		workPackageID := entries[i].WorkPackageID

		if workPackageID == 0 {
			continue
		}

		// Check for work package fetch errors
		if err, hasError := workPackageErrors[workPackageID]; hasError {
			fmt.Printf("Warning: Failed to fetch work package %d: %v\n", workPackageID, err)
			continue
		}

		// Check if work package was found but had no status link
		if workPackage, exists := workPackageResults[workPackageID]; exists {
			if workPackage.Links.Status.Href == "" {
				fmt.Printf("Warning: Work package %d has no status link\n", workPackageID)
				continue
			}

			// Check for status fetch errors
			if err, hasError := statusErrors[workPackage.Links.Status.Href]; hasError {
				fmt.Printf("Warning: Failed to fetch status for work package %d: %v\n", workPackageID, err)
				continue
			}
		}

		// Update entry with cached status
		if isClosed, exists := statusCache[workPackageID]; exists {
			entries[i].WorkPackageClosed = isClosed
		}
	}

	return nil
}

// EnrichWithSummaries fetches additional data and generates LLM summaries for each work package.
func (s *TimeEntryService) EnrichWithSummaries(entries []GroupedTimeEntry, llmService *llm.Service, week string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	inputs := make([]llm.SummarizationInput, 0, len(entries))
	ctx := context.Background()

	// Get current user
	currentUser, err := s.client.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Calculate start and end date for the period of interest
	var startDate time.Time
	now := time.Now()
	if week == "Current week" {
		startDate = startOfWeek(now)
	} else {
		lastWeek := now.AddDate(0, 0, -7)
		startDate = startOfWeek(lastWeek)
	}
	endDate := startDate.AddDate(0, 0, 6)
	periodStart := startDate.Format("2006-01-02")
	periodEnd := endDate.Format("2006-01-02")

	// Step 1: Fetch work package details and activities in parallel
	for _, entry := range entries {
		wg.Add(1)
		go func(entry GroupedTimeEntry) {
			defer wg.Done()

			// Fetch work package for description
			workPackage, err := s.client.GetWorkPackage(entry.WorkPackageID)
			if err != nil {
				fmt.Printf("Failed to get work package %d: %v\n", entry.WorkPackageID, err)
				return
			}

			// Fetch activities for comments
			activities, err := s.client.GetWorkPackageActivities(entry.WorkPackageID)
			if err != nil {
				fmt.Printf("Failed to get activities for work package %d: %v\n", entry.WorkPackageID, err)
				return
			}

			var activityDetails []llm.ActivityDetail
			for _, act := range activities {
				if act.Comment.Raw != "" {
					author := act.Links.User.Title
					if author == currentUser.Name {
						author = "Me"
					}
					activityDetails = append(activityDetails, llm.ActivityDetail{
						Content:   fmt.Sprintf("%s: %s", author, act.Comment.Raw),
						Timestamp: act.CreatedAt,
					})
				}
			}

			mu.Lock()
			inputs = append(inputs, llm.SummarizationInput{
				WorkPackageID:     entry.WorkPackageID,
				Subject:           entry.WorkPackageTitle,
				Description:       workPackage.Description.Raw,
				Status:            workPackage.Links.Status.Title,
				Activities:        activityDetails,
				TimeEntryComments: entry.TimeEntryComments,
				PeriodStart:       periodStart,
				PeriodEnd:         periodEnd,
			})
			mu.Unlock()
		}(entry)
	}
	wg.Wait()

	// Step 2: Get summaries from the LLM service
	summaries := llmService.SummarizeWorkPackages(ctx, inputs)

	// Step 3: Update entries with summaries
	for i := range entries {
		if summary, ok := summaries[entries[i].WorkPackageID]; ok {
			entries[i].LLMSummary = summary
		}
	}

	return nil
}
