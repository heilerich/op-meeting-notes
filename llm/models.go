package llm

// OpenAI-compatible API request structure
type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI-compatible API response structure
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a response choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ActivityDetail holds the content and timestamp of a work package activity.
type ActivityDetail struct {
	Content   string
	Timestamp string
}

// TimeEntryComment holds the content and timestamp of a time entry comment.
type TimeEntryComment struct {
	Content   string
	Timestamp string
}

// SummarizationInput contains all data needed for work package summarization
type SummarizationInput struct {
	WorkPackageID     int
	Subject           string
	Description       string
	Status            string
	Activities        []ActivityDetail
	TimeEntryComments []TimeEntryComment
	PeriodStart       string
}

// SummarizationResult contains the result of LLM summarization
type SummarizationResult struct {
	WorkPackageID int
	Summary       string
	Error         error
}

// CacheEntry represents a cached summarization result
type CacheEntry struct {
	Summary   string
	Timestamp int64
}
