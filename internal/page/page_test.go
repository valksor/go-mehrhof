package page

import (
	"testing"
)

func TestDefaultPagination(t *testing.T) {
	p := DefaultPagination()
	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}

	if p.PerPage != 20 {
		t.Errorf("PerPage = %d, want 20", p.PerPage)
	}
}

func TestPagination_Normalize(t *testing.T) {
	tests := []struct {
		name  string
		input Pagination
		want  Pagination
	}{
		{
			name:  "valid values unchanged",
			input: Pagination{Page: 2, PerPage: 10},
			want:  Pagination{Page: 2, PerPage: 10},
		},
		{
			name:  "negative page defaults to 1",
			input: Pagination{Page: -5, PerPage: 10},
			want:  Pagination{Page: 1, PerPage: 10},
		},
		{
			name:  "zero page defaults to 1",
			input: Pagination{Page: 0, PerPage: 10},
			want:  Pagination{Page: 1, PerPage: 10},
		},
		{
			name:  "zero perPage defaults to 20",
			input: Pagination{Page: 1, PerPage: 0},
			want:  Pagination{Page: 1, PerPage: 20},
		},
		{
			name:  "negative perPage defaults to 20",
			input: Pagination{Page: 1, PerPage: -3},
			want:  Pagination{Page: 1, PerPage: 20},
		},
		{
			name:  "perPage over 100 capped to 100",
			input: Pagination{Page: 1, PerPage: 500},
			want:  Pagination{Page: 1, PerPage: 100},
		},
		{
			name:  "perPage exactly 100 unchanged",
			input: Pagination{Page: 1, PerPage: 100},
			want:  Pagination{Page: 1, PerPage: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.Normalize()
			if got != tt.want {
				t.Errorf("Normalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPagination_Offset(t *testing.T) {
	tests := []struct {
		name string
		p    Pagination
		want int
	}{
		{"page 1 perPage 20", Pagination{Page: 1, PerPage: 20}, 0},
		{"page 2 perPage 20", Pagination{Page: 2, PerPage: 20}, 20},
		{"page 3 perPage 10", Pagination{Page: 3, PerPage: 10}, 20},
		{"page 5 perPage 5", Pagination{Page: 5, PerPage: 5}, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Offset(); got != tt.want {
				t.Errorf("Offset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPagination_Limit(t *testing.T) {
	p := Pagination{Page: 1, PerPage: 25}
	if got := p.Limit(); got != 25 {
		t.Errorf("Limit() = %d, want 25", got)
	}
}

func TestNewPage_EmptyItems(t *testing.T) {
	pg := NewPage[string]([]string{}, Pagination{Page: 1, PerPage: 10})

	if !pg.IsEmpty() {
		t.Error("expected empty page")
	}

	if pg.Total != 0 {
		t.Errorf("Total = %d, want 0", pg.Total)
	}

	if pg.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0", pg.TotalPages)
	}

	if pg.HasNext {
		t.Error("HasNext should be false")
	}

	if pg.HasPrev {
		t.Error("HasPrev should be false")
	}
}

func TestNewPage_SinglePage(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	pg := NewPage(items, Pagination{Page: 1, PerPage: 10})

	if len(pg.Items) != 5 {
		t.Errorf("Items count = %d, want 5", len(pg.Items))
	}

	if pg.Total != 5 {
		t.Errorf("Total = %d, want 5", pg.Total)
	}

	if pg.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", pg.TotalPages)
	}

	if pg.HasNext {
		t.Error("HasNext should be false for single page")
	}

	if pg.HasPrev {
		t.Error("HasPrev should be false for first page")
	}
}

func TestNewPage_MultiplePages_FirstPage(t *testing.T) {
	items := make([]int, 25)
	for i := range items {
		items[i] = i + 1
	}

	pg := NewPage(items, Pagination{Page: 1, PerPage: 10})

	if len(pg.Items) != 10 {
		t.Errorf("Items count = %d, want 10", len(pg.Items))
	}

	if pg.Items[0] != 1 || pg.Items[9] != 10 {
		t.Errorf("Items = %v, want 1..10", pg.Items)
	}

	if pg.Total != 25 {
		t.Errorf("Total = %d, want 25", pg.Total)
	}

	if pg.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", pg.TotalPages)
	}

	if !pg.HasNext {
		t.Error("HasNext should be true for first page")
	}

	if pg.HasPrev {
		t.Error("HasPrev should be false for first page")
	}
}

func TestNewPage_MultiplePages_MiddlePage(t *testing.T) {
	items := make([]int, 25)
	for i := range items {
		items[i] = i + 1
	}

	pg := NewPage(items, Pagination{Page: 2, PerPage: 10})

	if len(pg.Items) != 10 {
		t.Errorf("Items count = %d, want 10", len(pg.Items))
	}

	if pg.Items[0] != 11 || pg.Items[9] != 20 {
		t.Errorf("Items = %v, want 11..20", pg.Items)
	}

	if !pg.HasNext {
		t.Error("HasNext should be true for middle page")
	}

	if !pg.HasPrev {
		t.Error("HasPrev should be true for middle page")
	}
}

func TestNewPage_MultiplePages_LastPage(t *testing.T) {
	items := make([]int, 25)
	for i := range items {
		items[i] = i + 1
	}

	pg := NewPage(items, Pagination{Page: 3, PerPage: 10})

	if len(pg.Items) != 5 {
		t.Errorf("Items count = %d, want 5", len(pg.Items))
	}

	if pg.Items[0] != 21 || pg.Items[4] != 25 {
		t.Errorf("Items = %v, want 21..25", pg.Items)
	}

	if pg.HasNext {
		t.Error("HasNext should be false for last page")
	}

	if !pg.HasPrev {
		t.Error("HasPrev should be true for last page")
	}
}

func TestNewPage_BeyondRange(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	pg := NewPage(items, Pagination{Page: 10, PerPage: 10})

	if !pg.IsEmpty() {
		t.Error("expected empty items for page beyond range")
	}

	if pg.Total != 5 {
		t.Errorf("Total = %d, want 5", pg.Total)
	}

	if pg.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", pg.TotalPages)
	}
}

func TestNewPageFromSlice(t *testing.T) {
	// Simulate database query that returned items 11-20 of 25 total.
	items := []int{11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	pg := NewPageFromSlice(items, 25, Pagination{Page: 2, PerPage: 10})

	if len(pg.Items) != 10 {
		t.Errorf("Items count = %d, want 10", len(pg.Items))
	}

	if pg.Total != 25 {
		t.Errorf("Total = %d, want 25", pg.Total)
	}

	if pg.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", pg.TotalPages)
	}

	if pg.PageNum != 2 {
		t.Errorf("PageNum = %d, want 2", pg.PageNum)
	}

	if !pg.HasNext {
		t.Error("HasNext should be true")
	}

	if !pg.HasPrev {
		t.Error("HasPrev should be true")
	}
}

func TestNewPageFromSlice_NilItems(t *testing.T) {
	pg := NewPageFromSlice[string](nil, 0, Pagination{Page: 1, PerPage: 10})

	if pg.Items == nil {
		t.Error("Items should be non-nil empty slice, got nil")
	}

	if len(pg.Items) != 0 {
		t.Errorf("Items count = %d, want 0", len(pg.Items))
	}
}

func TestEmptyPage(t *testing.T) {
	pg := EmptyPage[int](Pagination{Page: 1, PerPage: 10})

	if !pg.IsEmpty() {
		t.Error("expected empty page")
	}

	if pg.Total != 0 {
		t.Errorf("Total = %d, want 0", pg.Total)
	}
}

func TestMapPage(t *testing.T) {
	items := []int{1, 2, 3}
	pg := NewPage(items, Pagination{Page: 1, PerPage: 10})

	doubled := MapPage(pg, func(n int) int { return n * 2 })

	if len(doubled.Items) != 3 {
		t.Fatalf("Items count = %d, want 3", len(doubled.Items))
	}

	want := []int{2, 4, 6}
	for i, v := range doubled.Items {
		if v != want[i] {
			t.Errorf("Items[%d] = %d, want %d", i, v, want[i])
		}
	}

	if doubled.Total != pg.Total {
		t.Errorf("Total = %d, want %d", doubled.Total, pg.Total)
	}

	if doubled.TotalPages != pg.TotalPages {
		t.Errorf("TotalPages = %d, want %d", doubled.TotalPages, pg.TotalPages)
	}
}

func TestMapPage_TypeConversion(t *testing.T) {
	items := []int{10, 20, 30}
	pg := NewPage(items, Pagination{Page: 1, PerPage: 10})

	stringPage := MapPage(pg, func(n int) string {
		return "item"
	})

	if len(stringPage.Items) != 3 {
		t.Fatalf("Items count = %d, want 3", len(stringPage.Items))
	}

	for i, v := range stringPage.Items {
		if v != "item" {
			t.Errorf("Items[%d] = %q, want %q", i, v, "item")
		}
	}
}

func TestPage_IsFirstPage(t *testing.T) {
	pg := NewPage([]int{1, 2, 3}, Pagination{Page: 1, PerPage: 2})

	if !pg.IsFirstPage() {
		t.Error("expected IsFirstPage to be true for page 1")
	}

	pg2 := NewPage([]int{1, 2, 3}, Pagination{Page: 2, PerPage: 2})

	if pg2.IsFirstPage() {
		t.Error("expected IsFirstPage to be false for page 2")
	}
}

func TestPage_IsLastPage(t *testing.T) {
	pg := NewPage([]int{1, 2, 3}, Pagination{Page: 2, PerPage: 2})

	if !pg.IsLastPage() {
		t.Error("expected IsLastPage to be true for last page")
	}

	pg1 := NewPage([]int{1, 2, 3}, Pagination{Page: 1, PerPage: 2})

	if pg1.IsLastPage() {
		t.Error("expected IsLastPage to be false for first page")
	}
}

func TestPage_NextPage(t *testing.T) {
	pg := NewPage([]int{1, 2, 3, 4, 5}, Pagination{Page: 1, PerPage: 2})

	if got := pg.NextPage(); got != 2 {
		t.Errorf("NextPage() = %d, want 2", got)
	}

	last := NewPage([]int{1, 2, 3, 4, 5}, Pagination{Page: 3, PerPage: 2})

	if got := last.NextPage(); got != 3 {
		t.Errorf("NextPage() on last page = %d, want 3", got)
	}
}

func TestPage_PrevPage(t *testing.T) {
	pg := NewPage([]int{1, 2, 3, 4, 5}, Pagination{Page: 2, PerPage: 2})

	if got := pg.PrevPage(); got != 1 {
		t.Errorf("PrevPage() = %d, want 1", got)
	}

	first := NewPage([]int{1, 2, 3, 4, 5}, Pagination{Page: 1, PerPage: 2})

	if got := first.PrevPage(); got != 1 {
		t.Errorf("PrevPage() on first page = %d, want 1", got)
	}
}
