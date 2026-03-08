package ui

import (
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"

	"github.com/heilerich/op-meeting-notes/models"
)

// activityChars maps activity types to distinct ASCII/Unicode characters for the bar chart
var activityChars = []struct {
	char rune
}{
	{'█'}, {'▓'}, {'▒'}, {'░'}, {'#'}, {'='}, {'~'}, {'+'}, {':'}, {'.'},
}

// projectActivityTotals holds per-project, per-activity-type hour totals
type projectActivityTotals struct {
	projectTotals     map[string]float64
	projectActivities map[string]map[string]float64 // project -> activity -> hours
	activityOrder     []string                      // sorted list of all activity types
}

// calculateProjectTotals calculates total hours per project and per activity type
func calculateProjectTotals(entries []models.GroupedTimeEntry) projectActivityTotals {
	totals := projectActivityTotals{
		projectTotals:     make(map[string]float64),
		projectActivities: make(map[string]map[string]float64),
	}
	allActivities := make(map[string]bool)

	for _, entry := range entries {
		totals.projectTotals[entry.ProjectTitle] += entry.TotalHours
		if totals.projectActivities[entry.ProjectTitle] == nil {
			totals.projectActivities[entry.ProjectTitle] = make(map[string]float64)
		}
		for activity, hours := range entry.ActivityHours {
			totals.projectActivities[entry.ProjectTitle][activity] += hours
			allActivities[activity] = true
		}
	}

	// Sort activity types alphabetically for consistent ordering
	for activity := range allActivities {
		totals.activityOrder = append(totals.activityOrder, activity)
	}
	sort.Strings(totals.activityOrder)

	return totals
}

// generateBarChart creates an ASCII horizontal bar chart for project time totals
// with activity type breakdown using different characters
func generateBarChart(totals projectActivityTotals) string {
	if len(totals.projectTotals) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("**Time Spent per Project**\n\n")
	result.WriteString("```\n")

	// Sort projects alphabetically
	var projects []string
	for project := range totals.projectTotals {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	// Find max hours for scaling
	maxHours := 0.0
	for _, hours := range totals.projectTotals {
		if hours > maxHours {
			maxHours = hours
		}
	}

	// Find longest project name for alignment
	maxNameLen := 0
	for _, project := range projects {
		if len(project) > maxNameLen {
			maxNameLen = len(project)
		}
	}

	// Build activity type to character mapping
	activityCharMap := make(map[string]rune)
	for i, activity := range totals.activityOrder {
		charIdx := i % len(activityChars)
		activityCharMap[activity] = activityChars[charIdx].char
	}

	// Generate bar chart
	const maxBarWidth = 40
	for _, project := range projects {
		hours := totals.projectTotals[project]

		// Calculate total bar width (scaled to max bar width)
		totalBarWidth := 0
		if maxHours > 0 {
			totalBarWidth = int(math.Round((hours / maxHours) * float64(maxBarWidth)))
		}
		if totalBarWidth < 1 && hours > 0 {
			totalBarWidth = 1
		}

		// Build stacked bar segments by activity type
		var bar strings.Builder
		activities := totals.projectActivities[project]
		usedWidth := 0

		for i, activity := range totals.activityOrder {
			actHours, ok := activities[activity]
			if !ok || actHours <= 0 {
				continue
			}

			// Calculate segment width proportional to activity hours
			segWidth := int(math.Round((actHours / hours) * float64(totalBarWidth)))
			if segWidth < 1 && actHours > 0 {
				segWidth = 1
			}

			// Last activity segment gets remaining width to avoid rounding issues
			isLast := true
			for _, remaining := range totals.activityOrder[i+1:] {
				if h, ok := activities[remaining]; ok && h > 0 {
					isLast = false
					break
				}
			}
			if isLast {
				segWidth = totalBarWidth - usedWidth
			} else if usedWidth+segWidth > totalBarWidth {
				segWidth = totalBarWidth - usedWidth
			}

			if segWidth > 0 {
				bar.WriteString(strings.Repeat(string(activityCharMap[activity]), segWidth))
				usedWidth += segWidth
			}
		}

		result.WriteString(fmt.Sprintf("%-*s %s %.1fh\n",
			maxNameLen, project, bar.String(), hours))
	}

	result.WriteString("```\n\n")

	// Add legend with per-activity totals
	if len(totals.activityOrder) > 0 {
		activityTotalHours := make(map[string]float64)
		for _, activities := range totals.projectActivities {
			for activity, hours := range activities {
				activityTotalHours[activity] += hours
			}
		}

		result.WriteString("Legend: ")
		var legendParts []string
		for _, activity := range totals.activityOrder {
			legendParts = append(legendParts,
				fmt.Sprintf("`%s` %s (%.1fh)", string(activityCharMap[activity]), activity, activityTotalHours[activity]))
		}
		result.WriteString(strings.Join(legendParts, "  ") + "\n\n")
	}

	return result.String()
}

func (m Model) doneView() string {
	if len(m.entriesModel.selected) == 0 {
		return QuitTextStyle.Render("No entries selected. Goodbye!")
	}

	// Calculate total time per project from ALL ORIGINAL entries (before any modifications)
	// Use originalEntries which was saved when entries were first loaded
	totals := calculateProjectTotals(m.originalEntries)

	// Group selected entries by project using updated entries
	projectEntries := make(map[string][]models.GroupedTimeEntry)

	for i := range m.entriesModel.selected {
		if i < len(m.groupedEntries) {
			selectedEntry := m.groupedEntries[i] // Use updated entries instead of list items
			projectTitle := selectedEntry.ProjectTitle
			projectEntries[projectTitle] = append(projectEntries[projectTitle], selectedEntry)
		}
	}

	// Build markdown document
	var markdown strings.Builder

	// Sort projects alphabetically
	var projects []string
	for project := range projectEntries {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	// Generate markdown for each project
	for _, project := range projects {
		entries := projectEntries[project]

		// Project heading
		markdown.WriteString(fmt.Sprintf("**%s**\n\n", project))

		// Sort entries by work package ID
		sort.Slice(entries, func(i, j int) bool {
			// Try to extract work package ID from the href or use a fallback
			idA := entries[i].WorkPackageID
			idB := entries[j].WorkPackageID
			return idA < idB
		})

		// Add work package entries
		for _, entry := range entries {
			workPackageID := entry.WorkPackageID

			// Use the LLM summary if available, otherwise fall back to the combined comment
			comments := entry.CombinedComment
			if entry.LLMSummary != "" {
				comments = entry.LLMSummary
			} else if comments == "" {
				comments = "(no comments)"
			}

			if workPackageID > 0 {
				if entry.WorkPackageClosed {
					markdown.WriteString(fmt.Sprintf("- %s (~~#%d~~): %s\n", entry.WorkPackageTitle, workPackageID, comments))
				} else {
					markdown.WriteString(fmt.Sprintf("- %s (#%d): %s\n", entry.WorkPackageTitle, workPackageID, comments))
				}
			} else {
				// Fallback if we can't get the ID
				markdown.WriteString(fmt.Sprintf("- %s: %s\n", entry.WorkPackageTitle, comments))
			}
		}

		markdown.WriteString("\n")
	}

	// Add bar chart at the end
	markdown.WriteString(generateBarChart(totals))

	// Copy to clipboard
	if err := copyToClipboard(markdown.String()); err != nil {
		return QuitTextStyle.Render(fmt.Sprintf("Failed to copy to clipboard: %v", err))
	}

	// Show markdown and ask about opening meeting notes
	result := fmt.Sprintf("Markdown copied to clipboard!\n\n%s\n", markdown.String())
	result += "Would you like to open the meeting notes?\n"
	result += "Press 'y' to open, any other key to exit."

	return QuitTextStyle.Render(result)
}

// copy the markdown to the clipboard by piping into a spawned pbcopy process
func copyToClipboard(markdown string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(markdown)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	return nil
}
