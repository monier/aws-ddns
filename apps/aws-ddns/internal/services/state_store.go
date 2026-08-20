package services

// StateStore persists the last IPv4 the service synchronized, so Route 53 is
// only queried when the discovered address actually changed.
type StateStore interface {
	// LastIPv4 returns the stored IPv4 and whether one is stored.
	LastIPv4() (string, bool, error)
	// SaveIPv4 stores the IPv4 as the last synchronized address.
	SaveIPv4(ipv4 string) error
}
