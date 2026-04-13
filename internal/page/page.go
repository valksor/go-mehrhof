package page

// Pagination represents input pagination parameters.
type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// DefaultPagination returns pagination with sensible defaults.
func DefaultPagination() Pagination {
	return Pagination{Page: 1, PerPage: 20}
}

// Normalize ensures valid pagination values.
// Page must be >= 1, PerPage must be between 1 and 100.
func (p Pagination) Normalize() Pagination {
	if p.Page < 1 {
		p.Page = 1
	}

	if p.PerPage < 1 {
		p.PerPage = 20
	}

	if p.PerPage > 100 {
		p.PerPage = 100
	}

	return p
}

// Offset returns the offset for slice or SQL operations.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// Limit returns the limit value (same as PerPage).
func (p Pagination) Limit() int {
	return p.PerPage
}

// Page represents a paginated result set.
type Page[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	PageNum    int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// NewPage creates a Page from a full item list and pagination params.
// It handles slicing the items to the requested page internally.
func NewPage[T any](allItems []T, p Pagination) Page[T] {
	p = p.Normalize()

	total := int64(len(allItems))

	totalPages := int(total) / p.PerPage
	if int(total)%p.PerPage > 0 {
		totalPages++
	}

	offset := p.Offset()
	end := offset + p.PerPage

	var items []T

	switch {
	case offset >= len(allItems):
		items = []T{}
	case end > len(allItems):
		items = allItems[offset:]
	default:
		items = allItems[offset:end]
	}

	return Page[T]{
		Items:      items,
		Total:      total,
		PageNum:    p.Page,
		PerPage:    p.PerPage,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
		HasPrev:    p.Page > 1,
	}
}

// NewPageFromSlice creates a Page from pre-sliced items with a known total.
// Use when the caller has already sliced (e.g., database LIMIT/OFFSET).
func NewPageFromSlice[T any](items []T, total int64, p Pagination) Page[T] {
	p = p.Normalize()

	if items == nil {
		items = []T{}
	}

	totalPages := int(total) / p.PerPage
	if int(total)%p.PerPage > 0 {
		totalPages++
	}

	return Page[T]{
		Items:      items,
		Total:      total,
		PageNum:    p.Page,
		PerPage:    p.PerPage,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
		HasPrev:    p.Page > 1,
	}
}

// EmptyPage returns an empty page with the given pagination.
func EmptyPage[T any](p Pagination) Page[T] {
	return NewPageFromSlice[T]([]T{}, 0, p)
}

// MapPage transforms items in a page from one type to another.
func MapPage[T, U any](pg Page[T], fn func(T) U) Page[U] {
	items := make([]U, len(pg.Items))
	for i, item := range pg.Items {
		items[i] = fn(item)
	}

	return Page[U]{
		Items:      items,
		Total:      pg.Total,
		PageNum:    pg.PageNum,
		PerPage:    pg.PerPage,
		TotalPages: pg.TotalPages,
		HasNext:    pg.HasNext,
		HasPrev:    pg.HasPrev,
	}
}

// IsEmpty returns true if the page has no items.
func (p Page[T]) IsEmpty() bool {
	return len(p.Items) == 0
}

// IsFirstPage returns true if this is page 1.
func (p Page[T]) IsFirstPage() bool {
	return p.PageNum == 1
}

// IsLastPage returns true if this is the last page.
func (p Page[T]) IsLastPage() bool {
	return p.PageNum >= p.TotalPages
}

// NextPage returns the next page number, or current if already at last page.
func (p Page[T]) NextPage() int {
	if p.HasNext {
		return p.PageNum + 1
	}

	return p.PageNum
}

// PrevPage returns the previous page number, or current if already at first page.
func (p Page[T]) PrevPage() int {
	if p.HasPrev {
		return p.PageNum - 1
	}

	return p.PageNum
}
