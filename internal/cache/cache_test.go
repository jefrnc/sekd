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

func TestBypass_GetAlwaysMisses(t *testing.T) {
	c := testCache(t)
	url := "https://example.com/bypass"
	if err := c.Set(url, []byte("cached data")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Sanity check: without bypass the entry should be returned.
	if _, ok := c.Get(url, time.Hour); !ok {
		t.Fatal("precondition failed: cached entry should be returned when bypass is off")
	}

	c.SetBypass(true)
	if !c.Bypass() {
		t.Error("Bypass() should report true after SetBypass(true)")
	}

	if _, ok := c.Get(url, time.Hour); ok {
		t.Error("Get should miss when bypass is enabled even if entry exists")
	}
}

func TestBypass_SetStillWrites(t *testing.T) {
	// Writes must still happen during bypass so the cache warms up for the
	// next non-bypass run. This is what keeps --no-cache from invalidating
	// subsequent cached reads.
	c := testCache(t)
	c.SetBypass(true)

	url := "https://example.com/warming"
	if err := c.Set(url, []byte("warm data")); err != nil {
		t.Fatalf("Set failed under bypass: %v", err)
	}

	// Turn bypass off and confirm the entry we wrote is retrievable.
	c.SetBypass(false)
	got, ok := c.Get(url, time.Hour)
	if !ok {
		t.Fatal("entry written during bypass was not retrievable after disabling bypass")
	}
	if string(got) != "warm data" {
		t.Errorf("Get = %q, want %q", got, "warm data")
	}
}

func TestBypass_DefaultOff(t *testing.T) {
	c := testCache(t)
	if c.Bypass() {
		t.Error("new cache should default to bypass disabled")
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
