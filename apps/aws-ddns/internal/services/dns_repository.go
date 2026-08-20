package services

import "context"

// DNSRepository reads and writes the managed DNS A record.
type DNSRepository interface {
	// ReadARecord returns the record's current IPv4 value and whether the record exists.
	ReadARecord(ctx context.Context, name string) (value string, exists bool, err error)
	// UpsertARecord creates or updates the A record with the given IPv4 and TTL.
	UpsertARecord(ctx context.Context, name string, ipv4 string, ttl int64) error
}
