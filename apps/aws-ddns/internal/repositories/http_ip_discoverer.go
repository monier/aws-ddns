package repositories

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of a discovery response is read — an IPv4
// dotted quad is at most 15 characters, so anything larger is garbage.
const maxResponseBytes = 256

// HTTPIPDiscoverer discovers the public IPv4 by querying external HTTPS
// endpoints in order, falling back to the next on any failure.
type HTTPIPDiscoverer struct {
	endpoints []string
	client    *http.Client
	logger    *slog.Logger
}

func NewHTTPIPDiscoverer(endpoints []string, timeout time.Duration, logger *slog.Logger) *HTTPIPDiscoverer {
	return &HTTPIPDiscoverer{
		endpoints: endpoints,
		client:    &http.Client{Timeout: timeout},
		logger:    logger,
	}
}

// DiscoverIPv4 returns the first valid IPv4 answered by the configured
// endpoints, or an error joining every endpoint failure when all of them fail.
func (d *HTTPIPDiscoverer) DiscoverIPv4(ctx context.Context) (string, error) {
	var failures []error
	for _, endpoint := range d.endpoints {
		d.logger.Debug("querying IP discovery endpoint", "endpoint", endpoint)
		start := time.Now()
		ip, err := d.query(ctx, endpoint)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			d.logger.Warn("IP discovery endpoint failed, trying next", "endpoint", endpoint, "error", err, "duration", time.Since(start).String())
			failures = append(failures, fmt.Errorf("%s: %w", endpoint, err))
			continue
		}
		d.logger.Debug("IP discovery endpoint answered", "endpoint", endpoint, "ip", ip, "duration", time.Since(start).String())
		return ip, nil
	}
	return "", fmt.Errorf("all IP discovery endpoints failed: %w", errors.Join(failures...))
}

func (d *HTTPIPDiscoverer) query(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	value := strings.TrimSpace(string(body))
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("response %q is not a valid IPv4 address", value)
	}
	return ip.To4().String(), nil
}
