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
		Activity struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"activity"`
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

// Activity represents a single activity on a work package
type Activity struct {
	ID      int `json:"id"`
	Comment struct {
		Raw string `json:"raw"`
	} `json:"comment"`
	CreatedAt string `json:"createdAt"`
	Links     struct {
		User struct {
			Title string `json:"title"`
		} `json:"user"`
	} `json:"_links"`
}

// ActivitiesResponse represents the API response for a work package's activities
type ActivitiesResponse struct {
	Embedded struct {
		Elements []Activity `json:"elements"`
	} `json:"_embedded"`
}

// WorkPackage represents work package details from the OpenProject API
// Following HAL+JSON format as documented at https://www.openproject.org/docs/api/endpoints/work-packages/
type WorkPackage struct {
	Type        string `json:"_type"`
	ID          int    `json:"id"`
	Subject     string `json:"subject"`
	Description struct {
		Format string `json:"format"`
		Raw    string `json:"raw"`
		HTML   string `json:"html"`
	} `json:"description"`
	Links struct {
		Self struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"self"`
		Status struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"status"`
		Type struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"type"`
		Project struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"project"`
	} `json:"_links"`
}

// Status represents a work package status from the OpenProject API
type Status struct {
	Type               string `json:"_type"`
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	IsClosed           bool   `json:"isClosed"`
	Color              string `json:"color"`
	IsDefault          bool   `json:"isDefault"`
	IsReadonly         bool   `json:"isReadonly"`
	ExcludedFromTotals bool   `json:"excludedFromTotals"`
	DefaultDoneRatio   int    `json:"defaultDoneRatio"`
	Position           int    `json:"position"`
	Links              struct {
		Self struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"self"`
	} `json:"_links"`
}

// Project represents project details
type Project struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// User represents an OpenProject user
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Login string `json:"login"`
}
