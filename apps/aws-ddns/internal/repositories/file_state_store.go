package repositories

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// FileStateStore keeps the last synchronized IPv4 in a small local file, so a
// restart or an unchanged address does not require a Route 53 query.
type FileStateStore struct {
	path string
}

func NewFileStateStore(path string) *FileStateStore {
	return &FileStateStore{path: path}
}

// LastIPv4 returns the stored address. A missing file — or unreadable
// content, which self-heals on the next save — reads as "nothing stored".
func (s *FileStateStore) LastIPv4() (string, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read state file %s: %w", s.path, err)
	}
	value := strings.TrimSpace(string(data))
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		return "", false, nil
	}
	return ip.To4().String(), true, nil
}

// SaveIPv4 writes the address atomically (temp file + rename), creating the
// state directory when absent.
func (s *FileStateStore) SaveIPv4(ipv4 string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(ipv4+"\n"), 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}
