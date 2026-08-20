package framework

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Setenv("HOSTED_ZONE_ID", "Z0123456789ABCDEFGHIJ")
	t.Setenv("RECORD_NAME", "ddns-test.example.com")
}

func TestLoadConfigAppliesDefaultsWhenOptionalValuesAreAbsent(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("AWS_REGION", "")
	t.Setenv("INTERVAL", "")
	t.Setenv("TTL", "")
	t.Setenv("LOG_LEVEL", "")
	dataDir := t.TempDir()

	cfg, err := LoadConfig(dataDir)

	require.NoError(t, err)
	assert.Equal(t, dataDir, cfg.DataDir)
	assert.False(t, cfg.INIFound)
	assert.Equal(t, "us-east-1", cfg.AWSRegion)
	assert.Equal(t, 5*time.Minute, cfg.Interval)
	assert.Equal(t, int64(60), cfg.TTL)
	assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
	assert.Equal(t, filepath.Join(dataDir, "last-ip.txt"), cfg.StateFile)
	assert.Equal(t, []string{"https://api.ipify.org", "https://checkip.amazonaws.com"}, cfg.DiscoveryEndpoints)
	assert.Equal(t, 10*time.Second, cfg.HTTPTimeout)
}

func TestLoadConfigReadsExplicitEnvValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("AWS_REGION", "eu-west-3")
	t.Setenv("INTERVAL", "90s")
	t.Setenv("TTL", "300")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadConfig(t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, "eu-west-3", cfg.AWSRegion)
	assert.Equal(t, 90*time.Second, cfg.Interval)
	assert.Equal(t, int64(300), cfg.TTL)
	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
	assert.Equal(t, "Z0123456789ABCDEFGHIJ", cfg.HostedZoneID)
	assert.Equal(t, "ddns-test.example.com", cfg.RecordName)
}

func TestLoadConfigReadsTheINIFile(t *testing.T) {
	t.Setenv("HOSTED_ZONE_ID", "")
	t.Setenv("RECORD_NAME", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("INTERVAL", "")
	dataDir := writeConfigFile(t, `
AWS_ACCESS_KEY_ID = AKIAEXAMPLE
AWS_SECRET_ACCESS_KEY = secret-example
HOSTED_ZONE_ID = Z0123456789ABCDEFGHIJ
RECORD_NAME = home.example.com
INTERVAL = 15m
`)

	cfg, err := LoadConfig(dataDir)

	require.NoError(t, err)
	assert.True(t, cfg.INIFound)
	assert.Equal(t, filepath.Join(dataDir, "aws-ddns.ini"), cfg.INIPath)
	assert.Equal(t, "AKIAEXAMPLE", cfg.AccessKeyID)
	assert.Equal(t, "secret-example", cfg.SecretAccessKey)
	assert.Equal(t, "Z0123456789ABCDEFGHIJ", cfg.HostedZoneID)
	assert.Equal(t, "home.example.com", cfg.RecordName)
	assert.Equal(t, 15*time.Minute, cfg.Interval)
	assert.Equal(t, filepath.Join(dataDir, "last-ip.txt"), cfg.StateFile)
}

func TestLoadConfigEnvironmentOverridesTheINIFile(t *testing.T) {
	t.Setenv("HOSTED_ZONE_ID", "")
	t.Setenv("RECORD_NAME", "env-wins.example.com")
	t.Setenv("INTERVAL", "1m")
	dataDir := writeConfigFile(t, `
HOSTED_ZONE_ID = Z0123456789ABCDEFGHIJ
RECORD_NAME = file.example.com
INTERVAL = 15m
`)

	cfg, err := LoadConfig(dataDir)

	require.NoError(t, err)
	assert.Equal(t, "env-wins.example.com", cfg.RecordName)
	assert.Equal(t, time.Minute, cfg.Interval)
	assert.Equal(t, "Z0123456789ABCDEFGHIJ", cfg.HostedZoneID)
}

func TestLoadConfigFailsWhenHostedZoneIDIsMissing(t *testing.T) {
	t.Setenv("HOSTED_ZONE_ID", "")
	t.Setenv("RECORD_NAME", "ddns-test.example.com")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "HOSTED_ZONE_ID")
}

func TestLoadConfigFailsWhenRecordNameIsMissing(t *testing.T) {
	t.Setenv("HOSTED_ZONE_ID", "Z0123456789ABCDEFGHIJ")
	t.Setenv("RECORD_NAME", "   ")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "RECORD_NAME")
}

func TestLoadConfigFailsOnInvalidInterval(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("INTERVAL", "five minutes")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "INTERVAL")
}

func TestLoadConfigFailsOnNonPositiveInterval(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("INTERVAL", "-1m")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "INTERVAL")
}

func TestLoadConfigFailsOnInvalidTTL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("TTL", "0")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "TTL")
}

func TestLoadConfigFailsOnUnknownLogLevel(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LOG_LEVEL", "loud")

	_, err := LoadConfig(t.TempDir())

	assert.ErrorContains(t, err, "LOG_LEVEL")
}
