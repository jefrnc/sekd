package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Cache struct {
	dir string
}

func New() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".sekd", "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

func (c *Cache) key(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h)
}

func (c *Cache) Get(url string, ttl time.Duration) ([]byte, bool) {
	path := filepath.Join(c.dir, c.key(url))
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > ttl {
		os.Remove(path)
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *Cache) Set(url string, data []byte) error {
	path := filepath.Join(c.dir, c.key(url))
	return os.WriteFile(path, data, 0644)
}
