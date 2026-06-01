package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore provides atomic JSON file read/write for the file-based backend.
// Each entity type gets its own JSON file in the data directory.
type FileStore struct {
	mu    sync.RWMutex
	dir   string
	cache map[string]any // filename → parsed data, for reads
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir, cache: make(map[string]any)}
}

// Load reads and JSON-decodes a file into v. Returns error if file doesn't exist.
func (fs *FileStore) Load(filename string, v any) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := filepath.Join(fs.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("read %s: %w", filename, err)
	}
	if len(data) == 0 {
		return nil // empty file is fine
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", filename, err)
	}
	return nil
}

// Save JSON-encodes v and writes it atomically (tmp + rename) to filename.
func (fs *FileStore) Save(filename string, v any) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := filepath.Join(fs.dir, filename)
	tmpPath := path + ".tmp"

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", filename, err)
	}

	return nil
}

// Exists checks if a file exists.
func (fs *FileStore) Exists(filename string) bool {
	_, err := os.Stat(filepath.Join(fs.dir, filename))
	return err == nil
}

// EnsureFile creates an empty JSON file if it doesn't exist.
func (fs *FileStore) EnsureFile(filename, emptyValue string) error {
	if fs.Exists(filename) {
		return nil
	}
	return os.WriteFile(filepath.Join(fs.dir, filename), []byte(emptyValue), 0644)
}
