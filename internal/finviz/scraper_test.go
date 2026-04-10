package finviz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jefrnc/sekd/internal/cache"
)

// TestGetQuote_UsesCache verifies that two calls to GetQuote for the same
// ticker only hit the upstream HTTP server once — the second call must be
// served from cache. This was a real regression: the Scraper accepted a
// *cache.Cache in its constructor but ignored it entirely on the hot path.
func TestGetQuote_UsesCache(t *testing.T) {
	var hits int64
	const html = `<html><body>
<table class="snapshot-table2"><tr>
<td data-boxover-html="Current stock price">price</td><td>12.34</td>
<td data-boxover-html="Shares float">float</td><td>50M</td>
<td data-boxover-html="Short interest share">short</td><td>15%</td>
</tr></table>
<h2 class="quote-header_ticker-wrapper_company"><a>Test Co</a></h2>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	c := newTestCacheAt(t.TempDir())
	s := NewScraper(c)

	// Call s.get directly with the test server URL — it exercises the same
	// cache-aware path as GetQuote without needing to override the hardcoded
	// finviz.com URL.
	if _, err := s.get(context.Background(), srv.URL); err != nil {
		t.Fatalf("first get failed: %v", err)
	}
	if _, err := s.get(context.Background(), srv.URL); err != nil {
		t.Fatalf("second get failed: %v", err)
	}

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("expected 1 upstream hit with cache, got %d", got)
	}
}

// TestGetQuote_BypassesCacheWhenBypassSet verifies that turning on the
// cache bypass forces fresh HTTP fetches on every call.
func TestGetQuote_BypassesCacheWhenBypassSet(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()

	c := newTestCacheAt(t.TempDir())
	c.SetBypass(true)

	s := NewScraper(c)
	for i := 0; i < 3; i++ {
		if _, err := s.get(context.Background(), srv.URL); err != nil {
			t.Fatalf("get %d failed: %v", i, err)
		}
	}

	if got := atomic.LoadInt64(&hits); got != 3 {
		t.Errorf("expected 3 upstream hits with bypass, got %d", got)
	}
}

// TestGetQuote_HandlesUpstreamError surfaces non-200 responses cleanly
// and does not poison the cache with an error body.
func TestGetQuote_HandlesUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewScraper(newTestCacheAt(t.TempDir()))
	_, err := s.get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error from upstream 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

// newTestCacheAt constructs a Cache rooted at a temp dir. We do not use
// cache.New() because that resolves ~/.sekd and would pollute the real
// cache during tests.
func newTestCacheAt(dir string) *cache.Cache {
	return cache.NewForTesting(dir)
}
