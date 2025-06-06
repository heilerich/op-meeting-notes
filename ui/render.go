package ui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

func (m Model) doneView() string {
	if len(m.entriesModel.selected) == 0 {
		return QuitTextStyle.Render("No entries selected. Goodbye!")
	}

	// Group selected entries by project
	projectEntries := make(map[string][]TimeEntryItem)

	for i := range m.entriesModel.selected {
		selectedItem := m.entriesModel.list.Items()[i].(TimeEntryItem)
		projectTitle := selectedItem.groupedEntry.ProjectTitle
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
			idA := entries[i].groupedEntry.WorkPackageID
			idB := entries[j].groupedEntry.WorkPackageID
			return idA < idB
		})

		// Add work package entries
		for _, entry := range entries {
			workPackageID := entry.groupedEntry.WorkPackageID

			// Use the combined comment from the grouped entry
			comments := entry.groupedEntry.CombinedComment
			if comments == "" {
				comments = "(no comments)"
			}

			if workPackageID > 0 {
				markdown.WriteString(fmt.Sprintf("- #%d: %s\n", workPackageID, comments))
			} else {
				// Fallback if we can't get the ID
				markdown.WriteString(fmt.Sprintf("- %s: %s\n", entry.groupedEntry.WorkPackageTitle, comments))
			}
		}

		markdown.WriteString("\n")
	}

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
