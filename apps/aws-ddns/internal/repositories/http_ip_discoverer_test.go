package repositories

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func TestDiscoverIPv4ReturnsFirstEndpointAnswer(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "203.0.113.7")
	})
	discoverer := NewHTTPIPDiscoverer([]string{server.URL}, time.Second, discardLogger())

	ip, err := discoverer.DiscoverIPv4(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "203.0.113.7", ip)
}

func TestDiscoverIPv4TrimsSurroundingWhitespace(t *testing.T) {
	// checkip.amazonaws.com answers with a trailing newline.
	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "203.0.113.7\n")
	})
	discoverer := NewHTTPIPDiscoverer([]string{server.URL}, time.Second, discardLogger())

	ip, err := discoverer.DiscoverIPv4(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "203.0.113.7", ip)
}

func TestDiscoverIPv4FallsBackWhenFirstEndpointFails(t *testing.T) {
	failing := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	working := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "198.51.100.42")
	})
	discoverer := NewHTTPIPDiscoverer([]string{failing.URL, working.URL}, time.Second, discardLogger())

	ip, err := discoverer.DiscoverIPv4(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "198.51.100.42", ip)
}

func TestDiscoverIPv4FallsBackOnInvalidAnswer(t *testing.T) {
	invalid := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "<html>not an ip</html>")
	})
	working := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "198.51.100.42")
	})
	discoverer := NewHTTPIPDiscoverer([]string{invalid.URL, working.URL}, time.Second, discardLogger())

	ip, err := discoverer.DiscoverIPv4(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "198.51.100.42", ip)
}

func TestDiscoverIPv4RejectsIPv6Answers(t *testing.T) {
	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "2001:db8::1")
	})
	discoverer := NewHTTPIPDiscoverer([]string{server.URL}, time.Second, discardLogger())

	_, err := discoverer.DiscoverIPv4(context.Background())

	assert.ErrorContains(t, err, "not a valid IPv4")
}

func TestDiscoverIPv4FailsWhenAllEndpointsFail(t *testing.T) {
	first := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	second := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	discoverer := NewHTTPIPDiscoverer([]string{first.URL, second.URL}, time.Second, discardLogger())

	_, err := discoverer.DiscoverIPv4(context.Background())

	assert.ErrorContains(t, err, "all IP discovery endpoints failed")
}

func TestDiscoverIPv4HonoursTimeout(t *testing.T) {
	slow := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		io.WriteString(w, "203.0.113.7")
	})
	discoverer := NewHTTPIPDiscoverer([]string{slow.URL}, 50*time.Millisecond, discardLogger())

	_, err := discoverer.DiscoverIPv4(context.Background())

	assert.Error(t, err)
}

func TestDiscoverIPv4StopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "203.0.113.7")
	})
	discoverer := NewHTTPIPDiscoverer([]string{server.URL, server.URL}, time.Second, discardLogger())

	_, err := discoverer.DiscoverIPv4(ctx)

	assert.ErrorIs(t, err, context.Canceled)
}
