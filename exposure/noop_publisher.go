package exposure

import "context"

// NoopPublisher is a Publisher that discards all events.
// Used as the default wiring until a real broker is configured. This is here as a placeholder to demonstrate possible event functionality.
type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _ string, _ any) error {
	return nil
}
