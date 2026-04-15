// Package battery provides cross-cutting utilities for the agent harness stack.
package battery

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns the XDG data directory for appName.
// Uses $XDG_DATA_HOME/<appName> or falls back to ~/.local/share/<appName>.
// The directory is created with 0700 permissions if it does not exist.
func DataDir(appName string) (string, error) {
	return xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share"), appName)
}

// ConfigDir returns the XDG config directory for appName.
// Uses $XDG_CONFIG_HOME/<appName> or falls back to ~/.config/<appName>.
// The directory is created with 0700 permissions if it does not exist.
func ConfigDir(appName string) (string, error) {
	return xdgDir("XDG_CONFIG_HOME", ".config", appName)
}

// CacheDir returns the XDG cache directory for appName.
// Uses $XDG_CACHE_HOME/<appName> or falls back to ~/.cache/<appName>.
// The directory is created with 0700 permissions if it does not exist.
func CacheDir(appName string) (string, error) {
	return xdgDir("XDG_CACHE_HOME", ".cache", appName)
}

func xdgDir(envKey, fallbackSuffix, appName string) (string, error) {
	base := os.Getenv(envKey)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("battery: resolve %s: %w", envKey, err)
		}
		base = filepath.Join(home, fallbackSuffix)
	}
	dir := filepath.Join(base, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("battery: create %s: %w", dir, err)
	}
	return dir, nil
}
