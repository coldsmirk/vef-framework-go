package page

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageableNormalize tests pageable normalize functionality.
func TestPageableNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    Pageable
		expected Pageable
	}{
		{
			"NormalValues",
			Pageable{Page: 2, Size: 10},
			Pageable{Page: 2, Size: 10},
		},
		{
			"PageLessThanOne",
			Pageable{Page: 0, Size: 10},
			Pageable{Page: DefaultPageNumber, Size: 10},
		},
		{
			"NegativePage",
			Pageable{Page: -1, Size: 10},
			Pageable{Page: DefaultPageNumber, Size: 10},
		},
		{
			"SizeLessThanOne",
			Pageable{Page: 1, Size: 0},
			Pageable{Page: 1, Size: DefaultPageSize},
		},
		{
			"NegativeSize",
			Pageable{Page: 1, Size: -5},
			Pageable{Page: 1, Size: DefaultPageSize},
		},
		{
			"SizeExceedsMaximum",
			Pageable{Page: 1, Size: 2000},
			Pageable{Page: 1, Size: MaxPageSize},
		},
		{
			"AllInvalidValues",
			Pageable{Page: -1, Size: -1},
			Pageable{Page: DefaultPageNumber, Size: DefaultPageSize},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.Normalize()

			assert.Equal(t, tt.expected.Page, tt.input.Page, "Page should be normalized correctly")
			assert.Equal(t, tt.expected.Size, tt.input.Size, "Size should be normalized correctly")
			t.Logf("✓ Normalized - Page: %d, Size: %d", tt.input.Page, tt.input.Size)
		})
	}
}

// TestPageableOffset tests pageable offset functionality.
func TestPageableOffset(t *testing.T) {
	tests := []struct {
		name     string
		pageable Pageable
		expected int
	}{
		{"Page1Size10", Pageable{Page: 1, Size: 10}, 0},
		{"Page2Size10", Pageable{Page: 2, Size: 10}, 10},
		{"Page3Size15", Pageable{Page: 3, Size: 15}, 30},
		{"Page1Size1", Pageable{Page: 1, Size: 1}, 0},
		{"Page5Size20", Pageable{Page: 5, Size: 20}, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.pageable.Offset()
			assert.Equal(t, tt.expected, offset, "Offset should be calculated correctly")
			t.Logf("✓ Offset calculated - Page: %d, Size: %d, Offset: %d", tt.pageable.Page, tt.pageable.Size, offset)
		})
	}
}

// TestNewPage tests new page functionality.
func TestNewPage(t *testing.T) {
	pageable := Pageable{Page: 2, Size: 10}
	items := []string{"item1", "item2", "item3"}
	total := int64(25)

	page := New(pageable, total, items)

	assert.Equal(t, 2, page.Page, "Page number should match")
	assert.Equal(t, 10, page.Size, "Page size should match")
	assert.Equal(t, int64(25), page.Total, "Total count should match")
	assert.Len(t, page.Items, 3, "Should have correct number of items")
	assert.Equal(t, "item1", page.Items[0], "First item should match")
	t.Logf("✓ Page created - Page: %d, Size: %d, Total: %d, Items: %d",
		page.Page, page.Size, page.Total, len(page.Items))
}

// TestNewPageWithNilItems tests new page with nil items functionality.
func TestNewPageWithNilItems(t *testing.T) {
	pageable := Pageable{Page: 1, Size: 10}
	total := int64(0)

	page := New[string](pageable, total, nil)

	assert.NotNil(t, page.Items, "Items should be empty slice, not nil")
	assert.Empty(t, page.Items, "Items should be empty")
	t.Log("✓ Page created with nil items - Items initialized as empty slice")
}

// TestPageTotalPages tests page total pages functionality.
func TestPageTotalPages(t *testing.T) {
	tests := []struct {
		name     string
		page     Page[string]
		expected int
	}{
		{
			"NormalCase",
			Page[string]{Size: 10, Total: 25},
			3,
		},
		{
			"ExactDivision",
			Page[string]{Size: 10, Total: 20},
			2,
		},
		{
			"SingleItem",
			Page[string]{Size: 10, Total: 1},
			1,
		},
		{
			"ZeroItems",
			Page[string]{Size: 10, Total: 0},
			0,
		},
		{
			"ZeroSize",
			Page[string]{Size: 0, Total: 10},
			0,
		},
		{
			"LargeNumbers",
			Page[string]{Size: 15, Total: 100},
			7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalPages := tt.page.TotalPages()
			assert.Equal(t, tt.expected, totalPages, "Total pages should be calculated correctly")
			t.Logf("✓ Total pages - Size: %d, Total: %d, Pages: %d", tt.page.Size, tt.page.Total, totalPages)
		})
	}
}

// TestPageHasNext tests page has next functionality.
func TestPageHasNext(t *testing.T) {
	tests := []struct {
		name     string
		page     Page[string]
		expected bool
	}{
		{
			"HasNextPage",
			Page[string]{Page: 1, Size: 10, Total: 25},
			true,
		},
		{
			"LastPage",
			Page[string]{Page: 3, Size: 10, Total: 25},
			false,
		},
		{
			"SinglePage",
			Page[string]{Page: 1, Size: 10, Total: 5},
			false,
		},
		{
			"EmptyResult",
			Page[string]{Page: 1, Size: 10, Total: 0},
			false,
		},
		{
			"MiddlePage",
			Page[string]{Page: 2, Size: 10, Total: 30},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasNext := tt.page.HasNext()
			assert.Equal(t, tt.expected, hasNext, "HasNext should return correct value")
			t.Logf("✓ HasNext - Page: %d/%d, HasNext: %t",
				tt.page.Page, tt.page.TotalPages(), hasNext)
		})
	}
}

// TestPageHasPrevious tests page has previous functionality.
func TestPageHasPrevious(t *testing.T) {
	tests := []struct {
		name     string
		page     Page[string]
		expected bool
	}{
		{
			"FirstPage",
			Page[string]{Page: 1, Size: 10, Total: 25},
			false,
		},
		{
			"SecondPage",
			Page[string]{Page: 2, Size: 10, Total: 25},
			true,
		},
		{
			"LastPage",
			Page[string]{Page: 3, Size: 10, Total: 25},
			true,
		},
		{
			"MiddlePage",
			Page[string]{Page: 5, Size: 10, Total: 100},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPrevious := tt.page.HasPrevious()
			assert.Equal(t, tt.expected, hasPrevious, "HasPrevious should return correct value")
			t.Logf("✓ HasPrevious - Page: %d, HasPrevious: %t", tt.page.Page, hasPrevious)
		})
	}
}

// TestPageableJSONMarshaling tests pageable JSON marshaling functionality.
func TestPageableJSONMarshaling(t *testing.T) {
	pageable := Pageable{
		Page: 2,
		Size: 15,
	}

	// Marshal
	data, err := json.Marshal(pageable)
	require.NoError(t, err, "Pageable should marshal to JSON successfully")

	// Unmarshal
	var result Pageable

	err = json.Unmarshal(data, &result)
	require.NoError(t, err, "JSON should unmarshal to Pageable successfully")

	// Compare
	assert.Equal(t, pageable.Page, result.Page, "Page should match after marshaling")
	assert.Equal(t, pageable.Size, result.Size, "Size should match after marshaling")
	t.Logf("✓ JSON marshaling - Original: %+v, Result: %+v", pageable, result)
}

// TestPageJSONMarshaling tests page JSON marshaling functionality.
func TestPageJSONMarshaling(t *testing.T) {
	items := []string{"item1", "item2", "item3"}
	page := Page[string]{
		Page:  2,
		Size:  10,
		Total: 25,
		Items: items,
	}

	// Marshal
	data, err := json.Marshal(page)
	require.NoError(t, err, "Page should marshal to JSON successfully")

	// Unmarshal
	var result Page[string]

	err = json.Unmarshal(data, &result)
	require.NoError(t, err, "JSON should unmarshal to Page successfully")

	// Compare
	assert.Equal(t, page.Page, result.Page, "Page number should match after marshaling")
	assert.Equal(t, page.Size, result.Size, "Page size should match after marshaling")
	assert.Equal(t, page.Total, result.Total, "Total should match after marshaling")
	assert.Equal(t, len(page.Items), len(result.Items), "Items count should match after marshaling")

	for i, item := range page.Items {
		assert.Equal(t, item, result.Items[i], "Item[%d] should match after marshaling", i)
	}

	t.Logf("✓ Page JSON marshaling - Page: %d, Total: %d, Items: %d",
		result.Page, result.Total, len(result.Items))
}

// TestPageWithDifferentTypes tests page with different types functionality.
func TestPageWithDifferentTypes(t *testing.T) {
	t.Run("WithStructType", func(t *testing.T) {
		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		users := []User{
			{ID: 1, Name: "Alice"},
			{ID: 2, Name: "Bob"},
		}

		pageable := Pageable{Page: 1, Size: 10}
		userPage := New(pageable, 2, users)

		assert.Equal(t, "Alice", userPage.Items[0].Name, "First user name should be Alice")
		assert.Equal(t, 2, len(userPage.Items), "Should have 2 users")
		t.Logf("✓ Struct type - User count: %d, First user: %s", len(userPage.Items), userPage.Items[0].Name)
	})

	t.Run("WithIntType", func(t *testing.T) {
		numbers := []int{1, 2, 3, 4, 5}
		pageable := Pageable{Page: 1, Size: 10}
		numberPage := New(pageable, 5, numbers)

		assert.Len(t, numberPage.Items, 5, "Should have 5 numbers")
		assert.Equal(t, 1, numberPage.Items[0], "First number should be 1")
		t.Logf("✓ Int type - Number count: %d, First number: %d", len(numberPage.Items), numberPage.Items[0])
	})
}

// TestPaginationScenarios tests pagination scenarios functionality.
func TestPaginationScenarios(t *testing.T) {
	t.Run("ApiPaginationWorkflow", func(t *testing.T) {
		// Client sends request for page 2, size 10
		pageable := Pageable{Page: 2, Size: 10}
		pageable.Normalize()

		// Database query would use offset
		offset := pageable.Offset()
		assert.Equal(t, 10, offset, "Offset should be calculated correctly for page 2")

		// Mock data from database
		items := []string{"item11", "item12", "item13", "item14", "item15"}
		total := int64(45)

		// Create response page
		page := New(pageable, total, items)

		// Verify page metadata
		assert.True(t, page.HasPrevious(), "Page 2 should have previous page")
		assert.True(t, page.HasNext(), "Page 2 should have next page")
		assert.Equal(t, 5, page.TotalPages(), "Should have 5 total pages")

		t.Logf("✓ API pagination - Page: %d/%d, Items: %d, HasPrev: %t, HasNext: %t",
			page.Page, page.TotalPages(), len(page.Items), page.HasPrevious(), page.HasNext())
	})

	t.Run("EdgeCases", func(t *testing.T) {
		t.Run("EmptyResultSet", func(t *testing.T) {
			emptyPageable := Pageable{Page: 1, Size: 10}
			emptyPage := New[string](emptyPageable, 0, nil)

			assert.False(t, emptyPage.HasNext(), "Empty page should not have next")
			assert.False(t, emptyPage.HasPrevious(), "Empty page should not have previous")
			assert.Equal(t, 0, emptyPage.TotalPages(), "Empty page should have 0 total pages")
			t.Logf("✓ Empty result - Total: %d, Pages: %d", emptyPage.Total, emptyPage.TotalPages())
		})

		t.Run("SingleItemResult", func(t *testing.T) {
			singlePageable := Pageable{Page: 1, Size: 10}
			singlePage := New(singlePageable, 1, []string{"only item"})

			assert.False(t, singlePage.HasNext(), "Single item page should not have next")
			assert.Equal(t, 1, singlePage.TotalPages(), "Single item page should have 1 total page")
			t.Logf("✓ Single item - Total: %d, Pages: %d", singlePage.Total, singlePage.TotalPages())
		})
	})
}
