package storage

import "github.com/coldsmirk/vef-framework-go/event"

const (
	// EventTypeFilePromoted is published when a file is promoted from temp to permanent storage.
	// Deprecated: emitted by the legacy Promoter only; will be removed when
	// the Promoter is deleted.
	EventTypeFilePromoted = "vef.storage.file.promoted"
	// EventTypeFileDeleted is published when a file is deleted from storage.
	EventTypeFileDeleted = "vef.storage.file.deleted"
	// EventTypeDeleteDeadLetter is published when the delete worker has
	// exhausted retries for a pending-delete row. Operations should consume
	// this event to investigate; the row is parked, not removed.
	EventTypeDeleteDeadLetter = "vef.storage.delete.dead_letter"
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

// DeleteDeadLetterEvent reports a pending-delete row that the delete worker
// could not drain within its retry budget. The row is left in
// storage_pending_deletes (parked) for manual investigation.
type DeleteDeadLetterEvent struct {
	event.BaseEvent

	// PendingDeleteID is the primary key of the parked row.
	PendingDeleteID string `json:"pendingDeleteId"`
	// FileKey is the object key that failed to delete.
	FileKey string `json:"fileKey"`
	// Reason carries the original schedule reason.
	Reason DeleteReason `json:"reason"`
	// Attempts is the total number of failed attempts.
	Attempts int `json:"attempts"`
	// LastError captures the most recent error message for triage.
	LastError string `json:"lastError,omitempty"`
}

// NewDeleteDeadLetterEvent creates a new dead-letter event.
func NewDeleteDeadLetterEvent(id, key string, reason DeleteReason, attempts int, lastErr string) *DeleteDeadLetterEvent {
	return &DeleteDeadLetterEvent{
		BaseEvent:       event.NewBaseEvent(EventTypeDeleteDeadLetter),
		PendingDeleteID: id,
		FileKey:         key,
		Reason:          reason,
		Attempts:        attempts,
		LastError:       lastErr,
	}
}
