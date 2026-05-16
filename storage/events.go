package storage

// Storage event topics. Subscribers should match on the constant rather
// than the literal string to stay forward-compatible.
const (
	// EventTypeFilePromoted is published when a previously-pending upload
	// claim has been adopted by a business transaction (Files.OnCreate or
	// the new-side of Files.OnUpdate). One event per consumed claim.
	EventTypeFilePromoted = "vef.storage.file.promoted"
	// EventTypeFileDeleted is published when the delete worker has
	// successfully removed an object from the backend. One event per
	// pending-delete row drained.
	EventTypeFileDeleted = "vef.storage.file.deleted"
	// EventTypeDeleteDeadLetter is published when the delete worker has
	// exhausted retries for a pending-delete row. Operations should consume
	// this event to investigate; the row is parked, not removed.
	EventTypeDeleteDeadLetter = "vef.storage.delete.dead_letter"
)

// FilePromotedEvent reports the successful adoption of an upload claim by
// a business transaction. Subscribers can use it for audit, analytics, or
// downstream side-effects (cache warm-up, indexing, notifications).
type FilePromotedEvent struct {
	// FileKey is the object key the business model now owns.
	FileKey string `json:"fileKey"`
}

// NewFilePromotedEvent creates a new file-promoted event.
func NewFilePromotedEvent(key string) *FilePromotedEvent {
	return &FilePromotedEvent{FileKey: key}
}

// EventType implements event.Event.
func (*FilePromotedEvent) EventType() string { return EventTypeFilePromoted }

// FileDeletedEvent reports the successful removal of an object from the
// backend by the asynchronous delete worker. Subscribers can use it for
// cache invalidation, audit, or downstream cleanup.
type FileDeletedEvent struct {
	// FileKey is the object key that was just deleted.
	FileKey string `json:"fileKey"`
	// Reason carries the original schedule reason for the deletion.
	Reason DeleteReason `json:"reason"`
}

// NewFileDeletedEvent creates a new file-deleted event.
func NewFileDeletedEvent(key string, reason DeleteReason) *FileDeletedEvent {
	return &FileDeletedEvent{FileKey: key, Reason: reason}
}

// EventType implements event.Event.
func (*FileDeletedEvent) EventType() string { return EventTypeFileDeleted }

// DeleteDeadLetterEvent reports a pending-delete row that the delete worker
// could not drain within its retry budget. The row is left in
// sys_storage_pending_delete (parked) for manual investigation.
type DeleteDeadLetterEvent struct {
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
		PendingDeleteID: id,
		FileKey:         key,
		Reason:          reason,
		Attempts:        attempts,
		LastError:       lastErr,
	}
}

// EventType implements event.Event.
func (*DeleteDeadLetterEvent) EventType() string { return EventTypeDeleteDeadLetter }
