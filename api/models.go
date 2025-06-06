package api

// TimeEntry represents a time entry from the API
type TimeEntry struct {
	ID      int    `json:"id"`
	Hours   string `json:"hours"`
	SpentOn string `json:"spentOn"`
	Comment struct {
		Raw string `json:"raw"`
	} `json:"comment"`
	WorkPackage struct {
		Href  string `json:"href"`
		Title string `json:"title"`
	} `json:"workPackage"`
	Project struct {
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
