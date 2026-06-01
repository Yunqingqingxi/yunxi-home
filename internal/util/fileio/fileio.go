// Package fileio provides atomic, logged file I/O operations.
// Both MCP and Skills subsystems use this shared layer for configuration persistence.
package fileio

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path atomically: write to temp file, then os.Rename.
// This prevents corruption if the write is interrupted mid-stream.
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("fileio: mkdir %s: %w", dir, err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("fileio: write tmp %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("fileio: rename %s → %s: %w", tmpPath, path, err)
	}

	slog.Debug("fileio: atomic write", "path", path, "bytes", len(data))
	return nil
}

// ReadJSON reads a file and unmarshals JSON into target.
// Returns an error with the file path on failure.
func ReadJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("fileio: read failed", "path", path, "error", err)
		return fmt.Errorf("fileio: read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		slog.Warn("fileio: invalid JSON", "path", path, "error", err)
		return fmt.Errorf("fileio: parse %s: %w", path, err)
	}

	return nil
}

// WriteJSON marshals data as JSON and writes it atomically.
func WriteJSON(path string, data any) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("fileio: marshal: %w", err)
	}
	return AtomicWrite(path, raw)
}

// ReadOrCreate reads path into target. If the file does not exist, it creates
// the file from the defaults, writes it, and unmarshals the result into target.
func ReadOrCreate(path string, target any, defaults any) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if defaults != nil {
			raw, err := json.MarshalIndent(defaults, "", "  ")
			if err != nil {
				return fmt.Errorf("fileio: marshal defaults: %w", err)
			}
			if err := AtomicWrite(path, raw); err != nil {
				return err
			}
			slog.Info("fileio: created default config", "path", path)
			return json.Unmarshal(raw, target)
		}
		return fmt.Errorf("fileio: %s not found and no defaults provided", path)
	}
	if err != nil {
		slog.Warn("fileio: read failed", "path", path, "error", err)
		return fmt.Errorf("fileio: read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		slog.Warn("fileio: invalid JSON", "path", path, "error", err)
		return fmt.Errorf("fileio: parse %s: %w", path, err)
	}

	return nil
}
