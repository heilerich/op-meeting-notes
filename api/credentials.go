package api

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/keybase/go-keychain"
)

const BaseURL = "https://ukf.openproject.com"

type Credentials struct {
	APIKey string
}

func GetCredentials() (*Credentials, error) {
	// Try to get credentials from keychain first
	creds, err := getFromKeychain()
	if err == nil {
		return creds, nil
	}

	// If keychain fails, prompt user and save to keychain
	fmt.Println("Credentials not found in keychain. Please enter your OpenProject credentials:")

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("OpenProject API Key: ")
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read API key: %w", err)
	}
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	creds = &Credentials{
		APIKey: apiKey,
	}

	// Save to keychain
	if err := saveToKeychain(creds); err != nil {
		fmt.Printf("Warning: failed to save credentials to keychain: %v\n", err)
	} else {
		fmt.Println("Credentials saved to keychain successfully")
	}

	return creds, nil
}

func getFromKeychain() (*Credentials, error) {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassInternetPassword)
	query.SetServer(BaseURL)
	query.SetReturnData(true)
	query.SetReturnAttributes(true)
	query.SetMatchLimit(keychain.MatchLimitOne)

	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query keychain: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no credentials found in keychain")
	}

	item := results[0]

	// Get the server (base URL) from attributes
	baseURL := item.Server
	if baseURL == "" {
		return nil, fmt.Errorf("base URL not found in keychain item")
	}

	// Get the API key from data
	apiKey := string(item.Data)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in keychain item")
	}

	return &Credentials{
		APIKey: apiKey,
	}, nil
}

func saveToKeychain(creds *Credentials) error {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassInternetPassword)
	item.SetServer(BaseURL)
	item.SetData([]byte(creds.APIKey))
	item.SetAccessible(keychain.AccessibleWhenUnlocked)

	return keychain.AddItem(item)
}
