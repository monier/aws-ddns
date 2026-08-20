package framework

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfigFile places an aws-ddns.ini with the given content in a fresh
// data folder and returns the folder.
func writeConfigFile(t *testing.T, content string) string {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFileName), []byte(content), 0o600))
	return dir
}

func TestLoadConfigFileParsesKeysCommentsAndSections(t *testing.T) {
	dir := writeConfigFile(t, `
; aws-ddns configuration
[aws]
HOSTED_ZONE_ID = Z0123456789ABCDEFGHIJ
RECORD_NAME = "ddns-test.example.com"
# tuning
INTERVAL = 10m
`)

	values, found, err := loadConfigFile(dir)

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "Z0123456789ABCDEFGHIJ", values["HOSTED_ZONE_ID"])
	assert.Equal(t, "ddns-test.example.com", values["RECORD_NAME"])
	assert.Equal(t, "10m", values["INTERVAL"])
}

func TestLoadConfigFileReturnsEmptyWhenTheFileIsAbsent(t *testing.T) {
	values, found, err := loadConfigFile(t.TempDir())

	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, values)
}

func TestLoadConfigFileRejectsUnknownKeys(t *testing.T) {
	dir := writeConfigFile(t, "RECORD_NAM = typo.example.com\n")

	_, _, err := loadConfigFile(dir)

	assert.ErrorContains(t, err, `unknown key "RECORD_NAM"`)
}

func TestLoadConfigFileRejectsDataDirAsAKey(t *testing.T) {
	// The file lives inside the data folder, so the folder's own location
	// cannot come from the file.
	dir := writeConfigFile(t, "DATA_DIR = /elsewhere\n")

	_, _, err := loadConfigFile(dir)

	assert.ErrorContains(t, err, `unknown key "DATA_DIR"`)
}

func TestLoadConfigFileRejectsMalformedLines(t *testing.T) {
	dir := writeConfigFile(t, "just some words\n")

	_, _, err := loadConfigFile(dir)

	assert.ErrorContains(t, err, "expected key = value")
}
