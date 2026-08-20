package framework

import (
	"fmt"
	"os"
	"strings"
)

// The app data folder holds everything the app reads and writes locally:
// the INI configuration file and the last-known-IP state file. Because the
// configuration file lives inside it, its location can only come from the
// command line or the environment — never from the file itself.
const (
	defaultDataDir = "/var/lib/aws-ddns"
	stateFileName  = "last-ip.txt"
)

// ResolveDataDir picks the app data folder: the -data-dir flag, then the
// DATA_DIR environment variable, then the default. The second return value
// names the winning source, so startup logs show where the path came from.
func ResolveDataDir(flagValue string) (string, string) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, "flag"
	}
	if value := strings.TrimSpace(os.Getenv("DATA_DIR")); value != "" {
		return value, "environment"
	}
	return defaultDataDir, "default"
}

// EnsureDataDir creates the app data folder when absent and verifies the app
// can write to it, so a permission problem surfaces as one clear startup error
// instead of a degraded cache later.
func EnsureDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("data directory %s cannot be created: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".rw-probe-*")
	if err != nil {
		return fmt.Errorf("data directory %s is not writable: %w", dir, err)
	}
	probe.Close()
	if err := os.Remove(probe.Name()); err != nil {
		return fmt.Errorf("data directory %s is not writable: %w", dir, err)
	}
	return nil
}
