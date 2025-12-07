package ui

import (
	"strings"
	"testing"

	"github.com/heilerich/op-meeting-notes/models"
)

func TestCalculateProjectTotals(t *testing.T) {
	entries := []models.GroupedTimeEntry{
		{ProjectTitle: "Project A", TotalHours: 5.0},
		{ProjectTitle: "Project B", TotalHours: 3.5},
		{ProjectTitle: "Project A", TotalHours: 2.5},
		{ProjectTitle: "Project C", TotalHours: 1.0},
	}

	totals := calculateProjectTotals(entries)

	expected := map[string]float64{
		"Project A": 7.5,
		"Project B": 3.5,
		"Project C": 1.0,
	}

	for project, expectedHours := range expected {
		if totals[project] != expectedHours {
			t.Errorf("Expected %s to have %.1f hours, got %.1f", project, expectedHours, totals[project])
		}
	}
}

func TestGenerateBarChart(t *testing.T) {
	projectTotals := map[string]float64{
		"Project A": 10.0,
		"Project B": 5.0,
		"Project C": 2.5,
	}

	chart := generateBarChart(projectTotals)

	// Check that chart contains expected elements
	if !strings.Contains(chart, "**Time Spent per Project**") {
		t.Error("Chart should contain title")
	}

	if !strings.Contains(chart, "```") {
		t.Error("Chart should be in a code block")
	}

	if !strings.Contains(chart, "Project A") {
		t.Error("Chart should contain Project A")
	}

	if !strings.Contains(chart, "10.0h") {
		t.Error("Chart should show hours for Project A")
	}

	// Check that bars are present (using the block character)
	if !strings.Contains(chart, "█") {
		t.Error("Chart should contain bar characters")
	}
}

func TestGenerateBarChartEmpty(t *testing.T) {
	projectTotals := map[string]float64{}

	chart := generateBarChart(projectTotals)

	if chart != "" {
		t.Error("Empty project totals should return empty string")
	}
}

func TestGenerateBarChartSingleProject(t *testing.T) {
	projectTotals := map[string]float64{
		"Single Project": 8.0,
	}

	chart := generateBarChart(projectTotals)

	if !strings.Contains(chart, "Single Project") {
		t.Error("Chart should contain project name")
	}

	if !strings.Contains(chart, "8.0h") {
		t.Error("Chart should show hours")
	}
}
