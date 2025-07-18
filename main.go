package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/heilerich/op-meeting-notes/api"
	"github.com/heilerich/op-meeting-notes/llm"
	"github.com/heilerich/op-meeting-notes/models"
	"github.com/heilerich/op-meeting-notes/ui"
)

func main() {
	// Initialize API client
	client, err := api.NewClient()
	if err != nil {
		fmt.Printf("Error initializing API: %v\n", err)
		os.Exit(1)
	}

	// Initialize service layer
	timeEntryService := models.NewTimeEntryService(client)

	// Initialize LLM service
	llmService, err := llm.NewService()
	if err != nil {
		fmt.Printf("Error initializing LLM service: %v\n", err)
		os.Exit(1)
	}

	// Initialize UI
	model := ui.NewModel(timeEntryService, llmService)

	// Start the program
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
