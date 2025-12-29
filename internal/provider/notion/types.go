package notion

import "time"

// Page represents a Notion page
type Page struct {
	CreatedTime    time.Time           `json:"created_time"`
	LastEditedTime time.Time           `json:"last_edited_time"`
	Properties     map[string]Property `json:"properties"`
	Parent         Parent              `json:"parent"`
	ID             string              `json:"id"`
	URL            string              `json:"url"`
	Archived       bool                `json:"archived"`
}

// Parent represents the parent of a page or database
type Parent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
}

// Property represents a Notion page property (various types)
type Property struct {
	People      *PeopleProp      `json:"people,omitempty"`
	CheckBox    *bool            `json:"checkbox,omitempty"`
	Title       *TitleProp       `json:"title,omitempty"`
	RichText    *RichTextProp    `json:"rich_text,omitempty"`
	Select      *SelectProp      `json:"select,omitempty"`
	Status      *StatusProp      `json:"status,omitempty"`
	Relation    *RelationProp    `json:"relation,omitempty"`
	MultiSelect *MultiSelectProp `json:"multi_select,omitempty"`
	Formula     *FormulaProp     `json:"formula,omitempty"`
	Date        *DateProp        `json:"date,omitempty"`
	Number      *NumberProp      `json:"number,omitempty"`
	URL         *string          `json:"url,omitempty"`
	Email       *string          `json:"email,omitempty"`
	PhoneNumber *string          `json:"phone_number,omitempty"`
	ID          string           `json:"id"`
	Type        string           `json:"type"`
}

// TitleProp represents a title property
type TitleProp struct {
	Type  string     `json:"type"`
	Title []RichText `json:"title"`
}

// RichTextProp represents a rich text property
type RichTextProp struct {
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
}

// RichText represents a rich text object
type RichText struct {
	Type        string       `json:"type"`
	Text        *TextContent `json:"text,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	PlainText   string       `json:"plain_text"`
	Href        string       `json:"href,omitempty"`
}

// TextContent contains the actual text
type TextContent struct {
	Link    *Link  `json:"link,omitempty"`
	Content string `json:"content"`
}

// Link represents a URL link
type Link struct {
	URL string `json:"url"`
}

// Annotations represents text formatting
type Annotations struct {
	Color         string `json:"color"`
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
}

// SelectProp represents a select property
type SelectProp struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// StatusProp represents a status property
type StatusProp struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// MultiSelectProp represents a multi-select property
type MultiSelectProp struct {
	Options []SelectProp `json:"options"`
}

// DateProp represents a date property
type DateProp struct {
	Start string `json:"start"`
	End   string `json:"end,omitempty"`
}

// PeopleProp represents a people property
type PeopleProp struct {
	Type   string `json:"type"`
	People []User `json:"people"`
}

// NumberProp represents a number property
type NumberProp struct {
	Number float64 `json:"number"`
}

// FormulaProp represents a formula property
type FormulaProp struct {
	Type   string  `json:"type"`
	String string  `json:"string,omitempty"`
	Number float64 `json:"number,omitempty"`
	Bool   bool    `json:"bool,omitempty"`
}

// RelationProp represents a relation property
type RelationProp struct {
	Type     string         `json:"type"`
	Relation []RelationItem `json:"relation"`
}

// RelationItem represents a single relation
type RelationItem struct {
	ID string `json:"id"`
}

// User represents a Notion user
type User struct {
	Person    *Person `json:"person,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Type      string  `json:"type"`
}

// Person represents user details
type Person struct {
	Email string `json:"email"`
}

// Block represents a Notion block (page content)
type Block struct {
	Heading3         *HeadingBlock   `json:"heading_3,omitempty"`
	Divider          *DividerBlock   `json:"divider,omitempty"`
	Callout          *CalloutBlock   `json:"callout,omitempty"`
	Paragraph        *ParagraphBlock `json:"paragraph,omitempty"`
	Heading1         *HeadingBlock   `json:"heading_1,omitempty"`
	Heading2         *HeadingBlock   `json:"heading_2,omitempty"`
	NumberedListItem *ListItemBlock  `json:"numbered_list_item,omitempty"`
	BulletedListItem *ListItemBlock  `json:"bulleted_list_item,omitempty"`
	Quote            *QuoteBlock     `json:"quote,omitempty"`
	ToDo             *ToDoBlock      `json:"to_do,omitempty"`
	Code             *CodeBlock      `json:"code,omitempty"`
	Type             string          `json:"type"`
	ID               string          `json:"id"`
	HasOnly          bool            `json:"has_only"`
}

// ParagraphBlock represents a paragraph block
type ParagraphBlock struct {
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
}

// HeadingBlock represents a heading block
type HeadingBlock struct {
	Type         string     `json:"type"`
	RichText     []RichText `json:"rich_text"`
	IsToggleable bool       `json:"is_toggleable,omitempty"`
}

// ListItemBlock represents a list item block
type ListItemBlock struct {
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
}

// ToDoBlock represents a to-do block
type ToDoBlock struct {
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
	Checked  bool       `json:"checked"`
}

// CodeBlock represents a code block
type CodeBlock struct {
	Type     string     `json:"type"`
	Language string     `json:"language"`
	RichText []RichText `json:"rich_text"`
}

// QuoteBlock represents a quote block
type QuoteBlock struct {
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
}

// DividerBlock represents a divider block
type DividerBlock struct {
	Type string `json:"type"`
}

// CalloutBlock represents a callout block
type CalloutBlock struct {
	Icon     *Icon      `json:"icon,omitempty"`
	Type     string     `json:"type"`
	RichText []RichText `json:"rich_text"`
}

// Icon represents an icon (emoji or file)
type Icon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

// Comment represents a Notion comment
type Comment struct {
	CreatedTime    time.Time     `json:"created_time"`
	LastEditedTime time.Time     `json:"last_edited_time"`
	CreatedBy      RichText      `json:"created_by"`
	ID             string        `json:"id"`
	Parent         CommentParent `json:"parent"`
	RichText       []RichText    `json:"rich_text"`
}

// CommentParent represents the parent of a comment
type CommentParent struct {
	BlockID string `json:"block_id"`
}

// Database represents a Notion database
type Database struct {
	Properties map[string]Property `json:"properties"`
	Parent     Parent              `json:"parent"`
	ID         string              `json:"id"`
	Title      []RichText          `json:"title"`
}

// DatabaseQueryRequest represents a database query request
type DatabaseQueryRequest struct {
	Filter      *Filter `json:"filter,omitempty"`
	StartCursor string  `json:"start_cursor,omitempty"`
	Sorts       []Sort  `json:"sorts,omitempty"`
	PageSize    int     `json:"page_size,omitempty"`
}

// Filter represents a query filter
type Filter struct {
	Property    string             `json:"property"`
	Status      *StatusFilter      `json:"status,omitempty"`
	Select      *SelectFilter      `json:"select,omitempty"`
	MultiSelect *MultiSelectFilter `json:"multi_select,omitempty"`
	And         []Filter           `json:"and,omitempty"`
	Or          []Filter           `json:"or,omitempty"`
}

// StatusFilter filters by status property
type StatusFilter struct {
	Equals       string `json:"equals,omitempty"`
	DoesNotEqual string `json:"does_not_equal,omitempty"`
	IsEmpty      bool   `json:"is_empty,omitempty"`
	IsNotEmpty   bool   `json:"is_not_empty,omitempty"`
}

// SelectFilter filters by select property
type SelectFilter struct {
	Equals       string `json:"equals,omitempty"`
	DoesNotEqual string `json:"does_not_equal,omitempty"`
	IsEmpty      bool   `json:"is_empty,omitempty"`
	IsNotEmpty   bool   `json:"is_not_empty,omitempty"`
}

// MultiSelectFilter filters by multi_select property
type MultiSelectFilter struct {
	Contains       string `json:"contains,omitempty"`
	DoesNotContain string `json:"does_not_contain,omitempty"`
	IsEmpty        bool   `json:"is_empty,omitempty"`
	IsNotEmpty     bool   `json:"is_not_empty,omitempty"`
}

// Sort represents a sort order
type Sort struct {
	Property  string `json:"property"`
	Direction string `json:"direction"` // "ascending" or "descending"
}

// DatabaseQueryResponse represents a database query response
type DatabaseQueryResponse struct {
	Object     string `json:"object"`
	NextCursor string `json:"next_cursor,omitempty"`
	Results    []Page `json:"results"`
	HasMore    bool   `json:"has_more"`
}

// PageResponse represents a page response
type PageResponse struct {
	Page   *Page  `json:"-"`
	Object string `json:"object"`
}

// CommentResponse represents a list of comments response
type CommentResponse struct {
	Object     string    `json:"object"`
	NextCursor string    `json:"next_cursor,omitempty"`
	Results    []Comment `json:"results"`
	HasMore    bool      `json:"has_more"`
}

// CreatePageInput represents input for creating a page
type CreatePageInput struct {
	Properties map[string]Property `json:"properties"`
	Parent     Parent              `json:"parent"`
}

// UpdatePageInput represents input for updating a page
type UpdatePageInput struct {
	Properties map[string]Property `json:"properties"`
	Archived   *bool               `json:"archived,omitempty"`
}

// AddCommentInput represents input for adding a comment
type AddCommentInput struct {
	Parent   CommentParent `json:"parent"`
	RichText []RichText    `json:"rich_text"`
}
