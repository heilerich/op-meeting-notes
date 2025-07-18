package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/heilerich/op-meeting-notes/llm"
	"github.com/heilerich/op-meeting-notes/models"
)

const MEETING_NOTES_URL = "https://ukf.openproject.com/projects/idim-team/recurring_meetings/79"

// WeekSelection represents the week selection screen
type WeekSelectionModel struct {
	choices []string
	cursor  int
}

// TimeEntriesModel represents the time entries selection screen
type TimeEntriesModel struct {
	list     list.Model
	selected map[int]struct{}
}

// LoadingModel represents the loading screen
type LoadingModel struct {
	spinner spinner.Model
	week    string
}

// Main model that manages different screens
type Model struct {
	state            string // "week", "loading", "loadingTaskStatus", "loadingSummaries", "entries", "done", "confirm_url"
	weekModel        WeekSelectionModel
	loadingModel     LoadingModel
	entriesModel     TimeEntriesModel
	groupedEntries   []models.GroupedTimeEntry
	selectedWeek     string
	timeEntryService *models.TimeEntryService
	llmService       *llm.Service
}

// Messages
type TimeEntriesMsg []models.GroupedTimeEntry
type StatusUpdateCompleteMsg []models.GroupedTimeEntry
type SummarizationCompleteMsg []models.GroupedTimeEntry
type ErrorMsg error

// Item implements list.Item interface for time entries
type TimeEntryItem struct {
	groupedEntry models.GroupedTimeEntry
	title        string
}

func (i TimeEntryItem) FilterValue() string { return i.title }
func (i TimeEntryItem) Title() string       { return i.title }
func (i TimeEntryItem) Description() string { return "" }

// CustomDelegate extends list.DefaultDelegate with custom rendering
type CustomDelegate struct {
	list.DefaultDelegate
	selected map[int]struct{}
}

// NewItemDelegate creates a new item delegate with selection state
func NewItemDelegate(selected map[int]struct{}) list.ItemDelegate {
	d := CustomDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
		selected:        selected,
	}
	d.ShowDescription = false
	d.SetSpacing(0)
	d.Styles.NormalTitle = ItemStyle
	d.Styles.SelectedTitle = SelectedItemStyle

	// Create a custom render function to show selection indicators
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		return nil
	}

	return d
}

// Render implements custom rendering for items with selection indicators
func (d CustomDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(TimeEntryItem)
	if !ok {
		// Fall back to default rendering
		d.DefaultDelegate.Render(w, m, index, listItem)
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
	fn := ItemStyle.Render
	if index == m.Index() {
		// Current cursor position
		if isChosen {
			// Selected item with cursor
			fn = func(s ...string) string {
				return ChosenItemStyle.Render("> " + strings.Join(s, " "))
			}
		} else {
			// Unselected item with cursor
			fn = func(s ...string) string {
				return SelectedItemStyle.Render("> " + strings.Join(s, " "))
			}
		}
	} else if isChosen {
		// Selected item without cursor
		fn = ChosenItemStyle.Render
	}

	fmt.Fprint(w, fn(str))
}

// NewModel creates a new UI model
func NewModel(timeEntryService *models.TimeEntryService, llmService *llm.Service) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		state: "week",
		weekModel: WeekSelectionModel{
			choices: []string{"Current week", "Last week"},
		},
		loadingModel: LoadingModel{
			spinner: s,
		},
		entriesModel: TimeEntriesModel{
			selected: make(map[int]struct{}),
		},
		timeEntryService: timeEntryService,
		llmService:       llmService,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case "week":
			return m.updateWeekSelection(msg)
		case "entries":
			return m.updateTimeEntries(msg)
		case "confirm_url":
			return m.updateConfirmURL(msg)
		}

	case TimeEntriesMsg:
		m.groupedEntries = []models.GroupedTimeEntry(msg)

		// Convert grouped entries to list items
		items := make([]list.Item, 0, len(m.groupedEntries))
		for _, entry := range m.groupedEntries {
			items = append(items, TimeEntryItem{
				groupedEntry: entry,
				title:        entry.FormatTitle(),
			})
		}

		const defaultWidth = 80
		// Calculate list height based on terminal height
		listHeight := 20 // fallback height
		if physicalWidth, physicalHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			listHeight = physicalHeight - 9
			if listHeight < 5 {
				listHeight = 5 // minimum height
			}
			_ = physicalWidth // we have the width available if needed
		}

		l := list.New(items, NewItemDelegate(m.entriesModel.selected), defaultWidth, listHeight)
		l.Title = fmt.Sprintf("Time Entries for %s", m.selectedWeek)
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.Styles.Title = TitleStyle
		l.Styles.PaginationStyle = PaginationStyle
		l.Styles.HelpStyle = HelpStyle

		m.entriesModel.list = l
		m.state = "entries"

	case StatusUpdateCompleteMsg:
		// Status update completed, now fetch summaries
		m.groupedEntries = []models.GroupedTimeEntry(msg)
		m.state = "loadingSummaries"
		return m, tea.Batch(
			m.loadingModel.spinner.Tick,
			m.enrichEntriesWithSummaries(),
		)

	case SummarizationCompleteMsg:
		// Summarization completed, show the confirmation view
		m.groupedEntries = []models.GroupedTimeEntry(msg)
		m.state = "confirm_url"
		return m, nil

	case ErrorMsg:
		// Handle error
		fmt.Printf("Error: %v\n", msg)
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == "loading" || m.state == "loadingTaskStatus" || m.state == "loadingSummaries" {
			var cmd tea.Cmd
			m.loadingModel.spinner, cmd = m.loadingModel.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) updateWeekSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.fetchTimeEntries(m.selectedWeek),
		)
	}

	return m, nil
}

func (m Model) updateTimeEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "enter":
		if len(m.entriesModel.selected) == 0 {
			// No selection made, quit
			return m, tea.Quit
		}

		// First update the work package closed status for selected entries
		m.state = "loadingTaskStatus"
		return m, tea.Batch(
			m.loadingModel.spinner.Tick,
			m.updateSelectedEntriesStatus(),
		)

	case " ":
		// Toggle selection
		i := m.entriesModel.list.Index()
		if _, ok := m.entriesModel.selected[i]; ok {
			delete(m.entriesModel.selected, i)
		} else {
			m.entriesModel.selected[i] = struct{}{}
		}

		// Update the delegate with new selection state
		m.entriesModel.list.SetDelegate(NewItemDelegate(m.entriesModel.selected))
	}

	var cmd tea.Cmd
	m.entriesModel.list, cmd = m.entriesModel.list.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmURL(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "y", "Y":
		// Open URL and quit
		if err := openURL(MEETING_NOTES_URL); err != nil {
			// Handle error but still quit
			fmt.Printf("Error opening URL: %v\n", err)
		}
		return m, tea.Quit

	default:
		// Any other key - just quit
		return m, tea.Quit
	}
}

// fetchTimeEntries creates a command to fetch time entries
func (m Model) fetchTimeEntries(week string) tea.Cmd {
	return func() tea.Msg {
		groupedEntries, err := m.timeEntryService.GetTimeEntriesForWeek(week)
		if err != nil {
			return ErrorMsg(err)
		}
		return TimeEntriesMsg(groupedEntries)
	}
}

// updateSelectedEntriesStatus creates a command to update work package closed status for selected entries
func (m Model) updateSelectedEntriesStatus() tea.Cmd {
	return func() tea.Msg {
		// Get only the selected entries
		selectedEntries := make([]models.GroupedTimeEntry, 0, len(m.entriesModel.selected))
		for index := range m.entriesModel.selected {
			if index < len(m.groupedEntries) {
				selectedEntries = append(selectedEntries, m.groupedEntries[index])
			}
		}

		// Update the status for selected entries
		err := m.timeEntryService.UpdateWorkPackageClosedStatus(selectedEntries)
		if err != nil {
			return ErrorMsg(err)
		}

		// Create updated copy of all entries with the new status information
		updatedEntries := make([]models.GroupedTimeEntry, len(m.groupedEntries))
		copy(updatedEntries, m.groupedEntries)

		// Update the entries with the status information from selectedEntries
		for _, selectedEntry := range selectedEntries {
			// Find the corresponding entry in the updated slice and update it
			for j := range updatedEntries {
				if updatedEntries[j].WorkPackageID == selectedEntry.WorkPackageID &&
					updatedEntries[j].ProjectTitle == selectedEntry.ProjectTitle {
					updatedEntries[j].WorkPackageClosed = selectedEntry.WorkPackageClosed
					break
				}
			}
		}

		return StatusUpdateCompleteMsg(updatedEntries)
	}
}

// enrichEntriesWithSummaries creates a command to enrich entries with LLM summaries
func (m Model) enrichEntriesWithSummaries() tea.Cmd {
	return func() tea.Msg {
		// Get only the selected entries
		selectedEntries := make([]models.GroupedTimeEntry, 0, len(m.entriesModel.selected))
		for index := range m.entriesModel.selected {
			if index < len(m.groupedEntries) {
				selectedEntries = append(selectedEntries, m.groupedEntries[index])
			}
		}

		// Enrich the selected entries with summaries
		err := m.timeEntryService.EnrichWithSummaries(selectedEntries, m.llmService, m.selectedWeek)
		if err != nil {
			return ErrorMsg(err)
		}

		// Create updated copy of all entries with the new summary information
		updatedEntries := make([]models.GroupedTimeEntry, len(m.groupedEntries))
		copy(updatedEntries, m.groupedEntries)

		// Update the entries with the summary information from selectedEntries
		for _, selectedEntry := range selectedEntries {
			// Find the corresponding entry in the updated slice and update it
			for j := range updatedEntries {
				if updatedEntries[j].WorkPackageID == selectedEntry.WorkPackageID &&
					updatedEntries[j].ProjectTitle == selectedEntry.ProjectTitle {
					updatedEntries[j].LLMSummary = selectedEntry.LLMSummary
					break
				}
			}
		}

		return SummarizationCompleteMsg(updatedEntries)
	}
}

func (m Model) View() string {
	switch m.state {
	case "week":
		return m.weekSelectionView()
	case "loading":
		return m.loadingView("time entries")
	case "loadingTaskStatus":
		return m.loadingView("task status")
	case "loadingSummaries":
		return m.loadingView("AI summaries")
	case "entries":
		return m.timeEntriesView()
	case "confirm_url":
		return m.doneView()
	case "done":
		return "Done! Goodbye!"
	}
	return ""
}

func (m Model) weekSelectionView() string {
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

func (m Model) loadingView(item string) string {
	return fmt.Sprintf("\n\n   %s Fetching %s for %s...\n\n", m.loadingModel.spinner.View(), item, m.loadingModel.week)
}

func (m Model) timeEntriesView() string {
	selectedCount := len(m.entriesModel.selected)

	view := m.entriesModel.list.View()
	view += fmt.Sprintf("\n\nSelected: %d entries", selectedCount)
	view += "\n\nPress space to toggle selection, enter to confirm, q to quit."

	return view
}

// openURL opens a URL using the system's default browser
func openURL(url string) error {
	args := []string{"-b", "com.apple.Safari.WebApp.A0223800-1DC9-499E-83C3-AF3D5ACF32D1", url}
	return exec.Command("open", args...).Start()
}
