package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/heilerich/op-meeting-notes/api"
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
	TotalHours        float64
	Representative    api.TimeEntry // First entry used as representative
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
		var totalHours float64

		for _, entry := range entries {
			if entry.Comment.Raw != "" {
				comments = append(comments, entry.Comment.Raw)
			}
			// Parse hours and add to total
			if hours, err := strconv.ParseFloat(entry.Hours, 64); err == nil {
				totalHours += hours
			}
		}

		// Create the combined comment
		combinedComment := "(no comments)"
		if len(comments) > 0 {
			combinedComment = strings.Join(comments, ", ")
		}

		result = append(result, GroupedTimeEntry{
			ProjectTitle:     key.projectTitle,
			WorkPackageID:    key.workPackageID,
			WorkPackageTitle: key.workPackageTitle,
			CombinedComment:  combinedComment,
			TotalHours:       totalHours,
			Representative:   entries[0], // Use first entry as representative
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
