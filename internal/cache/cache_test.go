package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCache(t *testing.T) *Cache {
	t.Helper()
	dir := t.TempDir()
	return &Cache{dir: dir}
}

func TestSetAndGet(t *testing.T) {
	c := testCache(t)
	url := "https://example.com/test"
	data := []byte("hello world")

	if err := c.Set(url, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, ok := c.Get(url, time.Hour)
	if !ok {
		t.Fatal("Get returned false for cached item")
	}
	if string(got) != string(data) {
		t.Errorf("Get = %q, want %q", got, data)
	}
}

func TestGetMiss(t *testing.T) {
	c := testCache(t)
	_, ok := c.Get("https://example.com/missing", time.Hour)
	if ok {
		t.Error("Get returned true for non-existent item")
	}
}

func TestGetExpired(t *testing.T) {
	c := testCache(t)
	url := "https://example.com/expired"

	if err := c.Set(url, []byte("data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Backdate the file modification time
	path := filepath.Join(c.dir, c.key(url))
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(path, past, past)

	_, ok := c.Get(url, time.Hour)
	if ok {
		t.Error("Get returned true for expired item")
	}

	// File should be removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Expired file was not removed")
	}
}

func TestKeyDeterministic(t *testing.T) {
	c := testCache(t)
	k1 := c.key("https://example.com/a")
	k2 := c.key("https://example.com/a")
	k3 := c.key("https://example.com/b")

	if k1 != k2 {
		t.Error("Same URL produced different keys")
	}
	if k1 == k3 {
		t.Error("Different URLs produced same key")
	}
}
