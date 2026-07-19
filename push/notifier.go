package push

import "context"

// Notifier is the business-facing entry of the server push channel. Delivery
// is best-effort by contract: recipients that are offline, disconnected, or
// too slow to drain their queue miss the message. Reliable notification
// belongs in business storage (a notification table the client pulls), with
// the push acting as the real-time hint.
type Notifier interface {
	// Push delivers the message to every recipient selected by the targets
	// (their union, each connection at most once). A zero message ID or time
	// is filled in. It returns an error only for an invalid message or target
	// set, never for missed recipients.
	Push(ctx context.Context, message Message, targets ...Target) error
}
