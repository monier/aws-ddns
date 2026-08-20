package repositories

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStateStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-ip.txt")
	store := NewFileStateStore(path)

	require.NoError(t, store.SaveIPv4("203.0.113.7"))
	value, ok, err := store.LastIPv4()

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "203.0.113.7", value)
}

func TestFileStateStoreReportsNothingStoredWhenFileIsMissing(t *testing.T) {
	store := NewFileStateStore(filepath.Join(t.TempDir(), "last-ip.txt"))

	_, ok, err := store.LastIPv4()

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestFileStateStoreCreatesTheStateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "last-ip.txt")
	store := NewFileStateStore(path)

	require.NoError(t, store.SaveIPv4("203.0.113.7"))

	_, ok, err := store.LastIPv4()
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestFileStateStoreTreatsCorruptContentAsNothingStored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-ip.txt")
	require.NoError(t, os.WriteFile(path, []byte("not an ip\n"), 0o600))
	store := NewFileStateStore(path)

	_, ok, err := store.LastIPv4()

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestFileStateStoreOverwritesThePreviousValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "last-ip.txt")
	store := NewFileStateStore(path)

	require.NoError(t, store.SaveIPv4("192.0.2.1"))
	require.NoError(t, store.SaveIPv4("203.0.113.7"))
	value, ok, err := store.LastIPv4()

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "203.0.113.7", value)
}
