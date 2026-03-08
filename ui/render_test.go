package ui

import (
	"strings"
	"testing"

	"github.com/heilerich/op-meeting-notes/models"
)

func TestCalculateProjectTotals(t *testing.T) {
	entries := []models.GroupedTimeEntry{
		{ProjectTitle: "Project A", TotalHours: 5.0, ActivityHours: map[string]float64{"Development": 3.0, "Support": 2.0}},
		{ProjectTitle: "Project B", TotalHours: 3.5, ActivityHours: map[string]float64{"Development": 3.5}},
		{ProjectTitle: "Project A", TotalHours: 2.5, ActivityHours: map[string]float64{"Support": 2.5}},
		{ProjectTitle: "Project C", TotalHours: 1.0, ActivityHours: map[string]float64{"Development": 1.0}},
	}

	totals := calculateProjectTotals(entries)

	expectedTotals := map[string]float64{
		"Project A": 7.5,
		"Project B": 3.5,
		"Project C": 1.0,
	}

	for project, expectedHours := range expectedTotals {
		if totals.projectTotals[project] != expectedHours {
			t.Errorf("Expected %s to have %.1f hours, got %.1f", project, expectedHours, totals.projectTotals[project])
		}
	}

	// Verify activity breakdown for Project A
	if totals.projectActivities["Project A"]["Development"] != 3.0 {
		t.Errorf("Expected Project A Development to have 3.0h, got %.1f", totals.projectActivities["Project A"]["Development"])
	}
	if totals.projectActivities["Project A"]["Support"] != 4.5 {
		t.Errorf("Expected Project A Support to have 4.5h, got %.1f", totals.projectActivities["Project A"]["Support"])
	}

	// Verify activity order is sorted
	if len(totals.activityOrder) != 2 {
		t.Errorf("Expected 2 activity types, got %d", len(totals.activityOrder))
	}
	if totals.activityOrder[0] != "Development" || totals.activityOrder[1] != "Support" {
		t.Errorf("Expected activity order [Development, Support], got %v", totals.activityOrder)
	}
}

func TestGenerateBarChart(t *testing.T) {
	totals := projectActivityTotals{
		projectTotals: map[string]float64{
			"Project A": 10.0,
			"Project B": 5.0,
			"Project C": 2.5,
		},
		projectActivities: map[string]map[string]float64{
			"Project A": {"Development": 7.0, "Support": 3.0},
			"Project B": {"Development": 5.0},
			"Project C": {"Support": 2.5},
		},
		activityOrder: []string{"Development", "Support"},
	}

	chart := generateBarChart(totals)

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

	// Check that legend is present
	if !strings.Contains(chart, "Legend:") {
		t.Error("Chart should contain a legend")
	}

	if !strings.Contains(chart, "Development") {
		t.Error("Legend should contain Development activity type")
	}

	if !strings.Contains(chart, "Support") {
		t.Error("Legend should contain Support activity type")
	}
}

func TestGenerateBarChartEmpty(t *testing.T) {
	totals := projectActivityTotals{
		projectTotals:     map[string]float64{},
		projectActivities: map[string]map[string]float64{},
		activityOrder:     nil,
	}

	chart := generateBarChart(totals)

	if chart != "" {
		t.Error("Empty project totals should return empty string")
	}
}

func TestGenerateBarChartSingleProject(t *testing.T) {
	totals := projectActivityTotals{
		projectTotals: map[string]float64{
			"Single Project": 8.0,
		},
		projectActivities: map[string]map[string]float64{
			"Single Project": {"Development": 8.0},
		},
		activityOrder: []string{"Development"},
	}

	chart := generateBarChart(totals)

	if !strings.Contains(chart, "Single Project") {
		t.Error("Chart should contain project name")
	}

	if !strings.Contains(chart, "8.0h") {
		t.Error("Chart should show hours")
	}
}

func TestGenerateBarChartMultipleActivities(t *testing.T) {
	totals := projectActivityTotals{
		projectTotals: map[string]float64{
			"My Project": 10.0,
		},
		projectActivities: map[string]map[string]float64{
			"My Project": {"Development": 5.0, "Support": 3.0, "Testing": 2.0},
		},
		activityOrder: []string{"Development", "Support", "Testing"},
	}

	chart := generateBarChart(totals)

	// Verify all activity types appear in the legend
	if !strings.Contains(chart, "Development") {
		t.Error("Legend should contain Development")
	}
	if !strings.Contains(chart, "Support") {
		t.Error("Legend should contain Support")
	}
	if !strings.Contains(chart, "Testing") {
		t.Error("Legend should contain Testing")
	}

	// The bar should use different characters (█ for Development, ▓ for Support, ▒ for Testing)
	if !strings.Contains(chart, "█") {
		t.Error("Chart should contain █ character for first activity type")
	}
	if !strings.Contains(chart, "▓") {
		t.Error("Chart should contain ▓ character for second activity type")
	}
	if !strings.Contains(chart, "▒") {
		t.Error("Chart should contain ▒ character for third activity type")
	}
}
