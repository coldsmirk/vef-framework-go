package page

const (
	// DefaultPageNumber is the default page number for pagination (starts from 1).
	DefaultPageNumber int = 1
	// DefaultPageSize is the default page size for pagination.
	DefaultPageSize int = 15
	// MaxPageSize is the maximum allowed page size to prevent excessive data loading.
	MaxPageSize int = 1000
)

// Pageable represents pagination parameters for querying data.
type Pageable struct {
	Page int `json:"page"` // 1-based
	Size int `json:"size"`
}

// Normalize normalizes the pageable parameters.
func (p *Pageable) Normalize(size ...int) {
	if p.Page < 1 {
		p.Page = DefaultPageNumber
	}

	if p.Size < 1 {
		if len(size) > 0 {
			p.Size = size[0]
		} else {
			p.Size = DefaultPageSize
		}
	}

	if p.Size > MaxPageSize {
		p.Size = MaxPageSize
	}
}

// Offset returns the zero-based offset for database queries.
func (p Pageable) Offset() int {
	return (p.Page - 1) * p.Size
}

// Page represents a paginated response with metadata and items.
type Page[T any] struct {
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Total int64 `json:"total"`
	Items []T   `json:"items"`
}

// TotalPages returns the total number of pages based on the total count.
func (page Page[T]) TotalPages() int {
	if page.Size == 0 {
		return 0
	}

	return int((page.Total + int64(page.Size) - 1) / int64(page.Size))
}

// HasNext returns true if there are more pages after the current one.
func (page Page[T]) HasNext() bool {
	return page.Page < page.TotalPages()
}

// HasPrevious returns true if there are pages before the current one.
func (page Page[T]) HasPrevious() bool {
	return page.Page > 1
}

// New creates a new page from pageable parameters, total count, and items.
// It ensures items is never nil and returns an empty slice if needed.
func New[T any](pageable Pageable, total int64, items []T) Page[T] {
	if items == nil {
		items = []T{}
	}

	return Page[T]{
		Page:  pageable.Page,
		Size:  pageable.Size,
		Total: total,
		Items: items,
	}
}
