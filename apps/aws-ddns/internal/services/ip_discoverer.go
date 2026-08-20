package services

import "context"

// IPDiscoverer returns the current public IPv4 address of the network this
// process runs in.
type IPDiscoverer interface {
	DiscoverIPv4(ctx context.Context) (string, error)
}
