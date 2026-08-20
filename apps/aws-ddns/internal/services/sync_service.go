package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SyncService owns the synchronization decision: discover the public IPv4,
// compare it with the locally stored last-known address, and only when they
// differ (or nothing is stored) consult Route 53 and upsert as needed.
type SyncService struct {
	discoverer IPDiscoverer
	repository DNSRepository
	state      StateStore
	recordName string
	ttl        int64
	logger     *slog.Logger
}

func NewSyncService(discoverer IPDiscoverer, repository DNSRepository, state StateStore, recordName string, ttl int64, logger *slog.Logger) *SyncService {
	return &SyncService{
		discoverer: discoverer,
		repository: repository,
		state:      state,
		recordName: recordName,
		ttl:        ttl,
		logger:     logger,
	}
}

// Sync performs one cycle. State-store failures never fail the cycle — they
// only cost the Route 53 comparison the cache would have skipped.
func (s *SyncService) Sync(ctx context.Context) error {
	s.logger.Debug("discovering public IPv4")
	discoverStart := time.Now()
	ip, err := s.discoverer.DiscoverIPv4(ctx)
	if err != nil {
		return fmt.Errorf("discover public IPv4: %w", err)
	}
	s.logger.Debug("public IPv4 discovered", "ip", ip, "duration", time.Since(discoverStart).String())

	cached, ok := s.lastKnownIPv4()
	if ok && cached == ip {
		s.logger.Info("public IPv4 unchanged since last synchronization, skipping Route 53 check", "record", s.recordName, "ip", ip)
		return nil
	}
	if ok {
		s.logger.Debug("public IPv4 differs from last known, checking Route 53", "lastKnown", cached, "current", ip)
	} else {
		s.logger.Debug("no last known IP cached, checking Route 53")
	}

	s.logger.Debug("reading DNS record from Route 53", "record", s.recordName)
	readStart := time.Now()
	current, exists, err := s.repository.ReadARecord(ctx, s.recordName)
	if err != nil {
		return fmt.Errorf("read DNS record %q: %w", s.recordName, err)
	}
	s.logger.Debug("DNS record read", "record", s.recordName, "exists", exists, "value", current, "duration", time.Since(readStart).String())

	if exists && current == ip {
		s.logger.Info("record already up to date", "record", s.recordName, "ip", ip)
		s.rememberIPv4(ip)
		return nil
	}

	s.logger.Debug("upserting DNS record", "record", s.recordName, "ip", ip, "ttl", s.ttl)
	upsertStart := time.Now()
	if err := s.repository.UpsertARecord(ctx, s.recordName, ip, s.ttl); err != nil {
		return fmt.Errorf("upsert DNS record %q: %w", s.recordName, err)
	}
	s.logger.Debug("DNS record upserted", "record", s.recordName, "duration", time.Since(upsertStart).String())

	previous := current
	if !exists {
		previous = "(none)"
	}
	s.logger.Info("record synchronized", "record", s.recordName, "previous", previous, "current", ip, "ttl", s.ttl)
	s.rememberIPv4(ip)
	return nil
}

func (s *SyncService) lastKnownIPv4() (string, bool) {
	value, ok, err := s.state.LastIPv4()
	if err != nil {
		s.logger.Warn("could not read the local IP state, checking Route 53 instead", "error", err)
		return "", false
	}
	return value, ok
}

func (s *SyncService) rememberIPv4(ipv4 string) {
	if err := s.state.SaveIPv4(ipv4); err != nil {
		s.logger.Warn("could not persist the last known IP; Route 53 will be checked again next cycle", "error", err)
	}
}
