package framework

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the whole application configuration, merged once at startup from
// (lowest to highest precedence): built-in defaults, the INI configuration
// file in the app data folder, environment variables.
type Config struct {
	DataDir            string
	INIPath            string
	INIFound           bool
	AWSRegion          string
	AccessKeyID        string
	SecretAccessKey    string
	HostedZoneID       string
	RecordName         string
	Interval           time.Duration
	TTL                int64
	LogLevel           slog.Level
	StateFile          string
	DiscoveryEndpoints []string
	HTTPTimeout        time.Duration
}

const (
	defaultAWSRegion   = "us-east-1"
	defaultInterval    = 5 * time.Minute
	defaultTTL         = int64(60)
	defaultHTTPTimeout = 10 * time.Second
)

// defaultDiscoveryEndpoints are tried in order; the first valid IPv4 answer wins.
var defaultDiscoveryEndpoints = []string{
	"https://api.ipify.org",
	"https://checkip.amazonaws.com",
}

// LoadConfig reads and validates the configuration. dataDir is the resolved
// app data folder (see data_dir.go); its aws-ddns.ini is read when present,
// and environment variables override file values.
func LoadConfig(dataDir string) (Config, error) {
	fileValues, iniFound, err := loadConfigFile(dataDir)
	if err != nil {
		return Config{}, err
	}

	get := func(key, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
		if value := fileValues[key]; value != "" {
			return value
		}
		return fallback
	}

	cfg := Config{
		DataDir:            dataDir,
		INIPath:            filepath.Join(dataDir, configFileName),
		INIFound:           iniFound,
		AWSRegion:          get("AWS_REGION", defaultAWSRegion),
		AccessKeyID:        get("AWS_ACCESS_KEY_ID", ""),
		SecretAccessKey:    get("AWS_SECRET_ACCESS_KEY", ""),
		HostedZoneID:       get("HOSTED_ZONE_ID", ""),
		RecordName:         get("RECORD_NAME", ""),
		Interval:           defaultInterval,
		TTL:                defaultTTL,
		StateFile:          filepath.Join(dataDir, stateFileName),
		DiscoveryEndpoints: defaultDiscoveryEndpoints,
		HTTPTimeout:        defaultHTTPTimeout,
	}

	if cfg.HostedZoneID == "" {
		return Config{}, fmt.Errorf("HOSTED_ZONE_ID is required")
	}
	if cfg.RecordName == "" {
		return Config{}, fmt.Errorf("RECORD_NAME is required")
	}

	if raw := get("INTERVAL", ""); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("INTERVAL %q is not a valid duration: %w", raw, err)
		}
		if interval <= 0 {
			return Config{}, fmt.Errorf("INTERVAL must be positive, got %q", raw)
		}
		cfg.Interval = interval
	}

	if raw := get("TTL", ""); raw != "" {
		ttl, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || ttl <= 0 {
			return Config{}, fmt.Errorf("TTL must be a positive integer, got %q", raw)
		}
		cfg.TTL = ttl
	}

	level, err := parseLogLevel(get("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	return cfg, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(raw) {
	case "verbose":
		return slog.LevelDebug - 4, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warning", "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q is not one of verbose|debug|info|warning|error", raw)
	}
}
