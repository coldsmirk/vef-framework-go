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

// Normalize normalizes the pageable parameters. The optional size argument
// supplies the fallback page size used when Size is unset; only the first
// value is considered and any further arguments are ignored. When omitted,
// DefaultPageSize is used.
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

// Map converts a page's items to another type, carrying the pagination
// metadata across unchanged.
//
// Turning a Page[Model] into a Page[VO] is the last step of nearly every list
// endpoint, and hand-rolling it means restating Page, Size and Total at each
// call site — the three fields a conversion has no business touching. The
// method form is what keeps that from happening: before Go 1.27 a method could
// not introduce a type parameter, so this had to be a free function taking the
// page as an argument, which reads no better than the loop it replaces.
//
// Like New it never produces a nil slice, so an empty page still serializes as
// [] rather than null.
func (page Page[T]) Map[R any](convert func(T) R) Page[R] {
	items := make([]R, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, convert(item))
	}

	return Page[R]{
		Page:  page.Page,
		Size:  page.Size,
		Total: page.Total,
		Items: items,
	}
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
