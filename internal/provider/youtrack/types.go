package youtrack

import "time"

// Issue represents a YouTrack issue from the API
type Issue struct {
	ID            string        `json:"id"`
	IDReadable    string        `json:"idReadable"` // e.g., "ABC-123"
	Summary       string        `json:"summary"`    // Title
	Description   string        `json:"description"`
	Created       int64         `json:"created"` // Unix timestamp ms
	Updated       int64         `json:"updated"`
	Resolved      int64         `json:"resolved"` // 0 if unresolved
	Project       Project       `json:"project"`
	Reporter      User          `json:"reporter"`
	Updater       User          `json:"updater"`
	CustomFields  []CustomField `json:"customFields"`
	Tags          []Tag         `json:"tags"`
	Comments      []Comment     `json:"comments"`
	Attachments   []Attachment  `json:"attachments"`
	CommentsCount int           `json:"commentsCount"`
	Subtasks      []IssueLink   `json:"subtasks"`
	Parent        *IssueLink    `json:"parent"`
}

// Project represents a YouTrack project
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"` // For IDReadable prefix
}

// User represents a YouTrack user
type User struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	FullName string `json:"name"`
	Email    string `json:"email"`
}

// CustomField represents a custom field value in YouTrack
// The Value field can be:
// - A map[string]interface{} for single enum/user values
// - A []interface{} for multi-value fields
// - A string for text fields
type CustomField struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
	Type  string      `json:"$type"`
}

// Tag represents a YouTrack tag
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Comment represents a YouTrack comment
type Comment struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Author  User   `json:"author"`
	Created int64  `json:"created"` // Unix timestamp ms
	Updated int64  `json:"updated"`
	Deleted bool   `json:"deleted"`
}

// Attachment represents a YouTrack attachment
type Attachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Created  int64  `json:"created"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	URL      string `json:"url"`
}

// IssueLink represents a link to another issue
type IssueLink struct {
	ID         string `json:"id"`
	IDReadable string `json:"idReadable"`
	Type       string `json:"type"`
}

// issueResponse wraps API response for a single issue
type issueResponse struct {
	Data Issue `json:"data"`
}

// issuesResponse wraps API response for multiple issues
type issuesResponse struct {
	Data []Issue `json:"data"`
}

// commentsResponse wraps API response for comments
type commentsResponse struct {
	Data []Comment `json:"data"`
}

// tagsResponse wraps API response for tags
type tagsResponse struct {
	Data []Tag `json:"data"`
}

// timeFromMillis converts Unix timestamp in milliseconds to time.Time
func timeFromMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.Unix(ms/1000, (ms%1000)*1e6)
}
