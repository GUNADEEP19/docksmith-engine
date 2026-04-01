package cache

import (
	"os"
	"path/filepath"
	"strings"
)

// Store persists key → layer digest mappings under docksmithRoot/cache.
type Store struct {
	dir string
}

// New returns a cache store rooted at filepath.Join(docksmithRoot, "cache").
func New(docksmithRoot string) *Store {
	root := strings.TrimSpace(docksmithRoot)
	if root == "" {
		root = defaultDocksmithRoot()
	}
	return &Store{dir: filepath.Join(root, "cache")}
}

func defaultDocksmithRoot() string {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	return filepath.Join(h, ".docksmith")
}

// Check implements internal.Cache.
func (s *Store) Check(key string) (layerDigest string, hit bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	p := filepath.Join(s.dir, key+".digest")
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	d := strings.TrimSpace(string(data))
	if d == "" {
		return "", false
	}
	return d, true
}

// Store implements internal.Cache.
func (s *Store) Store(key string, layerDigest string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(s.dir, key+".digest")
	return os.WriteFile(p, []byte(strings.TrimSpace(layerDigest)), 0o644)
}
