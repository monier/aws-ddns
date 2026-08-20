package framework

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// configFileName is the INI file's fixed name inside the app data folder.
const configFileName = "aws-ddns.ini"

// knownConfigKeys are the accepted INI keys — the same names as the
// environment variables, so there is one vocabulary for both sources. The
// data folder itself (DATA_DIR) is deliberately not a key: the file lives
// inside that folder, so its location must come from the flag or environment.
var knownConfigKeys = map[string]bool{
	"AWS_ACCESS_KEY_ID":     true,
	"AWS_SECRET_ACCESS_KEY": true,
	"HOSTED_ZONE_ID":        true,
	"RECORD_NAME":           true,
	"AWS_REGION":            true,
	"INTERVAL":              true,
	"TTL":                   true,
	"LOG_LEVEL":             true,
}

// loadConfigFile reads <dataDir>/aws-ddns.ini. A missing file is fine — the
// environment then carries the configuration; the second return value reports
// whether the file was found. Unknown keys are an error so a typo cannot
// silently fall back to a default.
func loadConfigFile(dataDir string) (map[string]string, bool, error) {
	path := filepath.Join(dataDir, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, false, nil
		}
		return nil, false, fmt.Errorf("read config file %s: %w", path, err)
	}

	values, err := parseINI(string(data))
	if err != nil {
		return nil, true, fmt.Errorf("parse config file %s: %w", path, err)
	}
	for key := range values {
		if !knownConfigKeys[key] {
			return nil, true, fmt.Errorf("config file %s: unknown key %q", path, key)
		}
	}
	return values, true, nil
}

// parseINI parses a flat INI document: `key = value` lines, `;`/`#` comments,
// and cosmetic `[section]` headers (accepted and ignored — all keys are
// global). Values may be wrapped in double quotes.
func parseINI(content string) (map[string]string, error) {
	values := map[string]string{}
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d: expected key = value, got %q", i+1, line)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", i+1)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, nil
}
