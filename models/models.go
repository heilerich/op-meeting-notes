package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	ProjectTitle     string
	WorkPackageID    int
	WorkPackageTitle string
	CombinedComment  string
	TotalHours       float64
	Representative   api.TimeEntry // First entry used as representative
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
