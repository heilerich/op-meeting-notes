package ui

import (
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"

	"github.com/heilerich/op-meeting-notes/models"
)

// calculateProjectTotals calculates total hours per project from all time entries
func calculateProjectTotals(entries []models.GroupedTimeEntry) map[string]float64 {
	projectTotals := make(map[string]float64)
	for _, entry := range entries {
		projectTotals[entry.ProjectTitle] += entry.TotalHours
		// Debug: print entry details
		fmt.Printf("DEBUG: Project=%s, WorkPackage=%d, TotalHours=%.2f\n",
			entry.ProjectTitle, entry.WorkPackageID, entry.TotalHours)
	}
	// Debug: print final totals
	fmt.Printf("DEBUG: Final project totals: %+v\n", projectTotals)
	return projectTotals
}

// generateBarChart creates an ASCII horizontal bar chart for project time totals
func generateBarChart(projectTotals map[string]float64) string {
	if len(projectTotals) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("**Time Spent per Project**\n\n")
	result.WriteString("```\n")

	// Sort projects alphabetically
	var projects []string
	for project := range projectTotals {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	// Find max hours for scaling
	maxHours := 0.0
	for _, hours := range projectTotals {
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

	// Generate bar chart
	const maxBarWidth = 40
	for _, project := range projects {
		hours := projectTotals[project]

		// Calculate bar width (scaled to max bar width)
		barWidth := 0
		if maxHours > 0 {
			barWidth = int(math.Round((hours / maxHours) * float64(maxBarWidth)))
		}
		if barWidth < 1 && hours > 0 {
			barWidth = 1 // Ensure at least 1 character for non-zero values
		}

		// Create the bar
		bar := strings.Repeat("█", barWidth)

		// Format: "Project Name    ███████████ 12.5h"
		result.WriteString(fmt.Sprintf("%-*s %s %.1fh\n",
			maxNameLen, project, bar, hours))
	}

	result.WriteString("```\n\n")
	return result.String()
}

func (m Model) doneView() string {
	if len(m.entriesModel.selected) == 0 {
		return QuitTextStyle.Render("No entries selected. Goodbye!")
	}

	// Calculate total time per project from ALL ORIGINAL entries (before any modifications)
	// Use originalEntries which was saved when entries were first loaded
	projectTotals := calculateProjectTotals(m.originalEntries)

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
	markdown.WriteString(generateBarChart(projectTotals))

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
