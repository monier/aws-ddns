package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDiscoverer struct {
	ip  string
	err error
}

func (f *fakeDiscoverer) DiscoverIPv4(context.Context) (string, error) {
	return f.ip, f.err
}

type fakeDNSRepository struct {
	value      string
	exists     bool
	readErr    error
	upsertErr  error
	upsertedIP string
	upsertTTL  int64
	reads      int
	upserts    int
}

func (f *fakeDNSRepository) ReadARecord(context.Context, string) (string, bool, error) {
	f.reads++
	return f.value, f.exists, f.readErr
}

func (f *fakeDNSRepository) UpsertARecord(_ context.Context, _ string, ipv4 string, ttl int64) error {
	f.upserts++
	f.upsertedIP = ipv4
	f.upsertTTL = ttl
	return f.upsertErr
}

type fakeStateStore struct {
	value   string
	ok      bool
	readErr error
	saveErr error
	saved   []string
}

func (f *fakeStateStore) LastIPv4() (string, bool, error) {
	return f.value, f.ok, f.readErr
}

func (f *fakeStateStore) SaveIPv4(ipv4 string) error {
	f.saved = append(f.saved, ipv4)
	return f.saveErr
}

func newSyncService(discoverer IPDiscoverer, repository DNSRepository, state StateStore) *SyncService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewSyncService(discoverer, repository, state, "ddns-test.example.com", 60, logger)
}

func TestSyncSkipsRoute53WhenCachedIPMatches(t *testing.T) {
	repository := &fakeDNSRepository{}
	state := &fakeStateStore{value: "203.0.113.7", ok: true}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Zero(t, repository.reads)
	assert.Zero(t, repository.upserts)
}

func TestSyncChecksRoute53WhenCachedIPDiffers(t *testing.T) {
	repository := &fakeDNSRepository{value: "192.0.2.1", exists: true}
	state := &fakeStateStore{value: "192.0.2.1", ok: true}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.reads)
	assert.Equal(t, 1, repository.upserts)
	assert.Equal(t, "203.0.113.7", repository.upsertedIP)
	assert.Equal(t, []string{"203.0.113.7"}, state.saved)
}

func TestSyncChecksRoute53WhenNothingIsCached(t *testing.T) {
	repository := &fakeDNSRepository{value: "203.0.113.7", exists: true}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.reads)
	assert.Zero(t, repository.upserts)
	// The matching record warms the cache so the next cycle skips Route 53.
	assert.Equal(t, []string{"203.0.113.7"}, state.saved)
}

func TestSyncChecksRoute53WhenStateReadFails(t *testing.T) {
	repository := &fakeDNSRepository{value: "203.0.113.7", exists: true}
	state := &fakeStateStore{readErr: errors.New("permission denied")}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.reads)
}

func TestSyncSucceedsWhenStateSaveFails(t *testing.T) {
	repository := &fakeDNSRepository{exists: false}
	state := &fakeStateStore{saveErr: errors.New("read-only filesystem")}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.upserts)
}

func TestSyncUpsertsWhenRecordDiffers(t *testing.T) {
	repository := &fakeDNSRepository{value: "192.0.2.1", exists: true}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.upserts)
	assert.Equal(t, "203.0.113.7", repository.upsertedIP)
	assert.Equal(t, int64(60), repository.upsertTTL)
	assert.Equal(t, []string{"203.0.113.7"}, state.saved)
}

func TestSyncUpsertsWhenRecordDoesNotExist(t *testing.T) {
	repository := &fakeDNSRepository{exists: false}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, repository.upserts)
	assert.Equal(t, "203.0.113.7", repository.upsertedIP)
}

func TestSyncFailsWithoutUpsertingWhenDiscoveryFails(t *testing.T) {
	repository := &fakeDNSRepository{}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{err: errors.New("all endpoints down")}, repository, state)

	err := service.Sync(context.Background())

	assert.ErrorContains(t, err, "discover public IPv4")
	assert.Zero(t, repository.upserts)
	assert.Empty(t, state.saved)
}

func TestSyncFailsWithoutUpsertingWhenReadFails(t *testing.T) {
	repository := &fakeDNSRepository{readErr: errors.New("access denied")}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	assert.ErrorContains(t, err, "read DNS record")
	assert.Zero(t, repository.upserts)
	assert.Empty(t, state.saved)
}

func TestSyncPropagatesUpsertFailuresWithoutCaching(t *testing.T) {
	repository := &fakeDNSRepository{exists: false, upsertErr: errors.New("throttled")}
	state := &fakeStateStore{}
	service := newSyncService(&fakeDiscoverer{ip: "203.0.113.7"}, repository, state)

	err := service.Sync(context.Background())

	assert.ErrorContains(t, err, "upsert DNS record")
	// A failed upsert must not be cached, or the retry would be skipped.
	assert.Empty(t, state.saved)
}
