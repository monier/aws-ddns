package framework

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDataDirPrefersTheFlag(t *testing.T) {
	t.Setenv("DATA_DIR", "/from/env")

	dir, source := ResolveDataDir("/from/flag")

	assert.Equal(t, "/from/flag", dir)
	assert.Equal(t, "flag", source)
}

func TestResolveDataDirFallsBackToTheEnvironment(t *testing.T) {
	t.Setenv("DATA_DIR", "/from/env")

	dir, source := ResolveDataDir("")

	assert.Equal(t, "/from/env", dir)
	assert.Equal(t, "environment", source)
}

func TestResolveDataDirDefaultsWhenNothingIsSet(t *testing.T) {
	t.Setenv("DATA_DIR", "")

	dir, source := ResolveDataDir("  ")

	assert.Equal(t, "/var/lib/aws-ddns", dir)
	assert.Equal(t, "default", source)
}

func TestEnsureDataDirCreatesTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")

	require.NoError(t, EnsureDataDir(dir))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureDataDirAcceptsAnExistingWritableFolder(t *testing.T) {
	assert.NoError(t, EnsureDataDir(t.TempDir()))
}

func TestEnsureDataDirFailsWhenTheFolderIsNotWritable(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := EnsureDataDir(dir)

	assert.ErrorContains(t, err, "not writable")
}

func TestEnsureDataDirLeavesNoProbeFileBehind(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, EnsureDataDir(dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
