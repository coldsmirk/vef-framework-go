package storage

import "github.com/coldsmirk/vef-framework-go/event"

const (
	// EventTypeFilePromoted is published when a file is promoted from temp to permanent storage.
	EventTypeFilePromoted = "vef.storage.file.promoted"
	// EventTypeFileDeleted is published when a file is deleted from storage.
	EventTypeFileDeleted = "vef.storage.file.deleted"
)

// FileOperation represents the type of file operation.
type FileOperation string

const (
	// OperationPromote indicates a file promotion operation.
	OperationPromote FileOperation = "promote"
	// OperationDelete indicates a file deletion operation.
	OperationDelete FileOperation = "delete"
)

// FileEvent represents a file operation event in the storage system.
// Published when files are promoted or deleted during Promoter operations.
type FileEvent struct {
	event.BaseEvent

	// The operation type (promote/delete)
	Operation FileOperation `json:"operation"`
	// The meta type (uploaded_file/richtext/markdown)
	MetaType MetaType `json:"metaType"`
	// The file key (promoted key for promote, original key for delete)
	FileKey string `json:"fileKey"`
	// Parsed attributes from the meta tag
	Attrs map[string]string `json:"attrs,omitempty"`
}

// NewFilePromotedEvent creates a new file promoted event.
// FileKey is the NEW key after promotion.
func NewFilePromotedEvent(metaType MetaType, fileKey string, attrs map[string]string) *FileEvent {
	return &FileEvent{
		BaseEvent: event.NewBaseEvent(EventTypeFilePromoted),
		Operation: OperationPromote,
		MetaType:  metaType,
		FileKey:   fileKey,
		Attrs:     attrs,
	}
}

// NewFileDeletedEvent creates a new file deleted event.
func NewFileDeletedEvent(metaType MetaType, fileKey string, attrs map[string]string) *FileEvent {
	return &FileEvent{
		BaseEvent: event.NewBaseEvent(EventTypeFileDeleted),
		Operation: OperationDelete,
		MetaType:  metaType,
		FileKey:   fileKey,
		Attrs:     attrs,
	}
}
