package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func (m model) doneView() string {
	if len(m.entriesModel.selected) == 0 {
		return quitTextStyle.Render("No entries selected. Goodbye!")
	}

	// Group selected entries by project
	projectEntries := make(map[string][]item)

	for i := range m.entriesModel.selected {
		selectedItem := m.entriesModel.list.Items()[i].(item)
		projectTitle := selectedItem.timeEntry.Links.Project.Title
		projectEntries[projectTitle] = append(projectEntries[projectTitle], selectedItem)
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
		markdown.WriteString(fmt.Sprintf("## %s\n\n", project))

		// Sort entries by work package ID
		sort.Slice(entries, func(i, j int) bool {
			// Try to extract work package ID from the href or use a fallback
			idA := extractWorkPackageID(entries[i])
			idB := extractWorkPackageID(entries[j])
			return idA < idB
		})

		// Add work package entries
		for _, entry := range entries {
			workPackageID := extractWorkPackageID(entry)

			// Extract comments from the combined title
			// The title format is: "Project X: WP Title - comments"
			titleParts := strings.Split(entry.title, " - ")
			comments := "(no comments)"
			if len(titleParts) > 1 {
				comments = strings.Join(titleParts[1:], " - ")
			}

			if workPackageID > 0 {
				markdown.WriteString(fmt.Sprintf("- #%d: %s\n", workPackageID, comments))
			} else {
				// Fallback if we can't get the ID
				markdown.WriteString(fmt.Sprintf("- %s: %s\n", entry.timeEntry.Links.WorkPackage.Title, comments))
			}
		}

		markdown.WriteString("\n")
	}

	return quitTextStyle.Render(markdown.String())
}

// Helper function to extract work package ID from various sources
func extractWorkPackageID(entry item) int {
	// Try the direct ID field first
	if entry.timeEntry.WorkPackage.ID > 0 {
		return entry.timeEntry.WorkPackage.ID
	}

	// Try to extract from the href URL
	href := entry.timeEntry.Links.WorkPackage.Href
	if href != "" {
		// URL format is typically /api/v3/work_packages/123
		parts := strings.Split(href, "/")
		if len(parts) > 0 {
			if id, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				return id
			}
		}
	}

	// Fallback: return 0
	return 0
}
