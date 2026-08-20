package process

import "context"

// Synchronizer is the single business operation the daemon loop drives.
type Synchronizer interface {
	Sync(ctx context.Context) error
}
