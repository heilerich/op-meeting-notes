package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	chosenItemStyle   = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("34")).Bold(true) // Green and bold for selected items
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle()
)

// API client configuration
var (
	client *http.Client
	host   *url.URL
	token  string
)

// TimeEntry represents a time entry from the API
type TimeEntry struct {
	ID      int    `json:"id"`
	Hours   string `json:"hours"`
	SpentOn string `json:"spentOn"`
	Comment struct {
		Raw string `json:"raw"`
	} `json:"comment"`
	WorkPackage struct {
		ID    int    `json:"id"`
		Href  string `json:"href"`
		Title string `json:"title"`
	} `json:"workPackage"`
	Project struct {
		ID    int    `json:"id"`
		Href  string `json:"href"`
		Title string `json:"title"`
	} `json:"project"`
	Activity struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"activity"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		WorkPackage struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"workPackage"`
		Project struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"project"`
	} `json:"_links"`
}

// TimeEntriesResponse represents the API response
type TimeEntriesResponse struct {
	Type     string `json:"_type"`
	Total    int    `json:"total"`
	Count    int    `json:"count"`
	Embedded struct {
		Elements []TimeEntry `json:"elements"`
	} `json:"_embedded"`
}

// WorkPackage represents work package details
type WorkPackage struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
	Type    struct {
		Name string `json:"name"`
	} `json:"type"`
}

// Project represents project details
type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Item implements list.Item interface
type item struct {
	timeEntry TimeEntry
	title     string
}

func (i item) FilterValue() string { return i.title }

// WeekSelection represents the week selection screen
type weekSelectionModel struct {
	choices []string
	cursor  int
}

// timeEntriesModel represents the time entries selection screen
type timeEntriesModel struct {
	list     list.Model
	selected map[int]struct{}
}

// loadingModel represents the loading screen
type loadingModel struct {
	spinner spinner.Model
	week    string
}

// Main model that manages different screens
type model struct {
	state        string // "week", "loading", "entries", "done"
	weekModel    weekSelectionModel
	loadingModel loadingModel
	entriesModel timeEntriesModel
	timeEntries  []TimeEntry
	selectedWeek string
}

type timeEntriesMsg []TimeEntry
type errorMsg error

func initAPI() error {
	// Get API credentials from environment
	apiKey := os.Getenv("OPENPROJECT_API_KEY")
	baseURL := os.Getenv("OPENPROJECT_BASE_URL")

	if apiKey == "" || baseURL == "" {
		return fmt.Errorf("please set OPENPROJECT_API_KEY and OPENPROJECT_BASE_URL environment variables")
	}

	// Parse the base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %v", err)
	}

	host = parsedURL
	token = apiKey
	client = &http.Client{Timeout: 30 * time.Second}

	return nil
}

func makeRequest(path string, queryParams map[string]string) ([]byte, error) {
	// Build the full URL
	fullURL := *host
	fullURL.Path = path

	// Add query parameters
	if len(queryParams) > 0 {
		q := fullURL.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		fullURL.RawQuery = q.Encode()
	}

	// Create request
	req, err := http.NewRequest("GET", fullURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Set authentication header (using API key as username with "apikey")
	req.SetBasicAuth("apikey", token)
	req.Header.Set("Accept", "application/hal+json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		state: "week",
		weekModel: weekSelectionModel{
			choices: []string{"Current week", "Last week"},
		},
		loadingModel: loadingModel{
			spinner: s,
		},
		entriesModel: timeEntriesModel{
			selected: make(map[int]struct{}),
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case "week":
			return m.updateWeekSelection(msg)
		case "entries":
			return m.updateTimeEntries(msg)
		}

	case timeEntriesMsg:
		m.timeEntries = []TimeEntry(msg)

		// Group entries by project and work package
		type entryKey struct {
			projectTitle     string
			workPackageID    int
			workPackageTitle string
		}

		groupedEntries := make(map[entryKey][]TimeEntry)
		for _, entry := range m.timeEntries {
			key := entryKey{
				projectTitle:     entry.Links.Project.Title,
				workPackageID:    entry.WorkPackage.ID,
				workPackageTitle: entry.Links.WorkPackage.Title,
			}
			groupedEntries[key] = append(groupedEntries[key], entry)
		}

		// Convert grouped entries to list items
		items := make([]list.Item, 0, len(groupedEntries))
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

			// Create work package info
			workPackageInfo := fmt.Sprintf("WP #%d", key.workPackageID)
			if key.workPackageTitle != "" {
				workPackageInfo = key.workPackageTitle
			}

			items = append(items, item{
				timeEntry: entries[0], // Use first entry as representative
				title:     fmt.Sprintf("Project %s: %s - %s", key.projectTitle, workPackageInfo, combinedComment),
			})
		}

		// Sort items by project name first, then by work package title
		sort.Slice(items, func(i, j int) bool {
			itemI := items[i].(item)
			itemJ := items[j].(item)

			projectI := itemI.timeEntry.Links.Project.Title
			projectJ := itemJ.timeEntry.Links.Project.Title

			if projectI != projectJ {
				return projectI < projectJ
			}

			// If same project, sort by work package title
			wpI := itemI.timeEntry.Links.WorkPackage.Title
			wpJ := itemJ.timeEntry.Links.WorkPackage.Title
			return wpI < wpJ
		})

		const defaultWidth = 80
		// Calculate list height based on terminal height
		// Leave space for title, selected count, help text, and some padding
		listHeight := 20 // fallback height
		if physicalWidth, physicalHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			// Reserve space for:
			// - Title (1 line)
			// - Selected count (2 lines)
			// - Help text (2 lines)
			// - Some padding (4 lines)
			listHeight = physicalHeight - 9
			if listHeight < 5 {
				listHeight = 5 // minimum height
			}
			_ = physicalWidth // we have the width available if needed
		}

		l := list.New(items, itemDelegate{selected: m.entriesModel.selected}, defaultWidth, listHeight)
		l.Title = fmt.Sprintf("Time Entries for %s", m.selectedWeek)
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.Styles.Title = titleStyle
		l.Styles.PaginationStyle = paginationStyle
		l.Styles.HelpStyle = helpStyle

		m.entriesModel.list = l
		m.state = "entries"

	case errorMsg:
		// Handle error
		fmt.Printf("Error: %v\n", msg)
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == "loading" {
			var cmd tea.Cmd
			m.loadingModel.spinner, cmd = m.loadingModel.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m model) updateWeekSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.weekModel.cursor > 0 {
			m.weekModel.cursor--
		}

	case "down", "j":
		if m.weekModel.cursor < len(m.weekModel.choices)-1 {
			m.weekModel.cursor++
		}

	case "enter", " ":
		m.selectedWeek = m.weekModel.choices[m.weekModel.cursor]
		m.state = "loading"
		m.loadingModel.week = m.selectedWeek
		return m, tea.Batch(
			m.loadingModel.spinner.Tick,
			fetchTimeEntries(m.selectedWeek),
		)
	}

	return m, nil
}

func (m model) updateTimeEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "enter":
		if len(m.entriesModel.selected) == 0 {
			// No selection made, quit
			return m, tea.Quit
		}

		// Process selected entries
		m.state = "done"
		return m, tea.Quit

	case " ":
		// Toggle selection
		i := m.entriesModel.list.Index()
		if _, ok := m.entriesModel.selected[i]; ok {
			delete(m.entriesModel.selected, i)
		} else {
			m.entriesModel.selected[i] = struct{}{}
		}

		// Update the delegate with new selection state
		m.entriesModel.list.SetDelegate(itemDelegate{selected: m.entriesModel.selected})
	}

	var cmd tea.Cmd
	m.entriesModel.list, cmd = m.entriesModel.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	switch m.state {
	case "week":
		return m.weekSelectionView()
	case "loading":
		return m.loadingView()
	case "entries":
		return m.timeEntriesView()
	case "done":
		return m.doneView()
	}
	return ""
}

func (m model) weekSelectionView() string {
	s := "Which week's time entries would you like to fetch?\n\n"

	for i, choice := range m.weekModel.choices {
		cursor := " "
		if m.weekModel.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress q to quit.\n"
	return s
}

func (m model) loadingView() string {
	return fmt.Sprintf("\n\n   %s Fetching time entries for %s...\n\n", m.loadingModel.spinner.View(), m.loadingModel.week)
}

func (m model) timeEntriesView() string {
	selectedCount := len(m.entriesModel.selected)

	view := m.entriesModel.list.View()
	view += fmt.Sprintf("\n\nSelected: %d entries", selectedCount)
	view += "\n\nPress space to toggle selection, enter to confirm, q to quit."

	return view
}

// Custom item delegate for the list
type itemDelegate struct {
	selected map[int]struct{}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	// Check if this item is selected
	_, isChosen := d.selected[index]

	// Create the display string with selection indicator
	indicator := " "
	if isChosen {
		indicator = "✓"
	}

	str := fmt.Sprintf("%s %d. %s", indicator, index+1, i.title)

	// Apply different styles based on cursor position and selection state
	fn := itemStyle.Render
	if index == m.Index() {
		// Current cursor position
		if isChosen {
			// Selected item with cursor
			fn = func(s ...string) string {
				return chosenItemStyle.Render("> " + strings.Join(s, " "))
			}
		} else {
			// Unselected item with cursor
			fn = func(s ...string) string {
				return selectedItemStyle.Render("> " + strings.Join(s, " "))
			}
		}
	} else if isChosen {
		// Selected item without cursor
		fn = chosenItemStyle.Render
	}

	fmt.Fprint(w, fn(str))
}

// fetchTimeEntries creates a command to fetch time entries
func fetchTimeEntries(week string) tea.Cmd {
	return func() tea.Msg {
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

		// Build filters for the date range and current user
		filters := fmt.Sprintf(`[{"spent_on":{"operator":"<>d","values":["%s","%s"]}},{"user":{"operator":"=","values":["me"]}}]`,
			startDate.Format("2006-01-02"),
			endDate.Format("2006-01-02"))

		// Prepare query parameters
		queryParams := map[string]string{
			"filters":  filters,
			"pageSize": "100", // Get more entries
		}

		// Make the request
		body, err := makeRequest("/api/v3/time_entries", queryParams)
		if err != nil {
			return errorMsg(err)
		}

		// Parse response
		var timeEntriesResp TimeEntriesResponse
		if err := json.Unmarshal(body, &timeEntriesResp); err != nil {
			return errorMsg(fmt.Errorf("failed to parse time entries response: %v", err))
		}

		return timeEntriesMsg(timeEntriesResp.Embedded.Elements)
	}
}

// Helper function to get start of week (Monday)
func startOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday)).Truncate(24 * time.Hour)
}

func main() {
	// Initialize API client
	if err := initAPI(); err != nil {
		fmt.Printf("Error initializing API: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
