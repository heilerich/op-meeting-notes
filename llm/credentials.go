package llm

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/keybase/go-keychain"
)

const BaseURL = "https://inference-api.metal.kn.uniklinik-freiburg.de/v1/chat/completions"
const DefaultModel = "gpt-oss-120b"

type Credentials struct {
	APIKey string
	Model  string
}

func GetCredentials() (*Credentials, error) {
	// Try to get credentials from keychain first
	creds, err := getFromKeychain()
	if err == nil {
		return creds, nil
	}

	// If keychain fails, prompt user and save to keychain
	fmt.Println("LLM credentials not found in keychain. Please enter your LLM provider credentials:")

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("LLM API Key: ")
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read API key: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	fmt.Printf("LLM Model (default: %s): ", DefaultModel)
	model, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read model: %w", err)
	}
	model = strings.TrimSpace(model)

	if model == "" {
		model = DefaultModel
	}

	creds = &Credentials{
		APIKey: apiKey,
		Model:  model,
	}

	// Save to keychain
	if err := saveToKeychain(creds); err != nil {
		fmt.Printf("Warning: failed to save LLM credentials to keychain: %v\n", err)
	} else {
		fmt.Println("LLM credentials saved to keychain successfully")
	}

	return creds, nil
}

func getFromKeychain() (*Credentials, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassInternetPassword)
	query.SetServer("op-meeting-notes-llm")
	query.SetReturnData(true)
	query.SetReturnAttributes(true)
	query.SetMatchLimit(keychain.MatchLimitOne)

	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query keychain: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no LLM credentials found in keychain")
	}

	item := results[0]

	// Get the API key from data
	apiKey := string(item.Data)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in keychain item")
	}

	// Get the model from account field (we'll store it there)
	model := item.Account
	if model == "" {
		model = DefaultModel
	}

	return &Credentials{
		APIKey: apiKey,
		Model:  model,
	}, nil
}

func saveToKeychain(creds *Credentials) error {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassInternetPassword)
	item.SetServer("op-meeting-notes-llm")
	item.SetAccount(creds.Model) // Store model in account field
	item.SetData([]byte(creds.APIKey))
	item.SetAccessible(keychain.AccessibleWhenUnlocked)

	return keychain.AddItem(item)
}
