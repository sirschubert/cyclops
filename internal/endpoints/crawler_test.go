package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirschubert/cyclops/pkg/models"
)

func newTestCrawler(depth int) *Crawler {
	return NewCrawler(models.ScanOptions{
		Depth:     depth,
		Timeout:   5,
		UserAgent: "TestCrawler/1.0",
		Threads:   5,
	})
}

func TestCrawler_DiscoversSameHostLinks(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `<html><body><a href="/page1">p1</a><a href="/page2">p2</a></body></html>`)
		case "/page1", "/page2":
			fmt.Fprintf(w, `<html><body>content</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	crawler := newTestCrawler(1)
	eps, err := crawler.Crawl(context.Background(), serverURL+"/")
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	found := make(map[string]bool)
	for _, ep := range eps {
		found[ep.URL] = true
	}

	for _, expected := range []string{serverURL + "/", serverURL + "/page1", serverURL + "/page2"} {
		if !found[expected] {
			t.Errorf("expected endpoint %q not found; got: %v", expected, eps)
		}
	}
}

func TestCrawler_RespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	crawler := newTestCrawler(2)
	_, err := crawler.Crawl(ctx, "http://example.com")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestCrawler_DoesNotFollowExternalLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><a href="https://other.com/path">external</a></body></html>`)
	}))
	defer server.Close()

	crawler := newTestCrawler(2)
	eps, err := crawler.Crawl(context.Background(), server.URL+"/")
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}
	for _, ep := range eps {
		if strings.HasPrefix(ep.URL, "https://other.com") {
			t.Errorf("crawler followed external link: %q", ep.URL)
		}
	}
}

func TestCrawler_RespectsMaxDepth(t *testing.T) {
	// Page at depth 0 links to depth1, which links to depth2.
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `<html><body><a href="/d1">d1</a></body></html>`)
		case "/d1":
			// serverURL is set after server starts; use relative link.
			fmt.Fprintf(w, `<html><body><a href="/d2">d2</a></body></html>`)
		case "/d2":
			fmt.Fprintf(w, `<html><body>deep</body></html>`)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	// Depth 1 should reach /d1 but not /d2.
	crawler := newTestCrawler(1)
	eps, err := crawler.Crawl(context.Background(), serverURL+"/")
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}
	for _, ep := range eps {
		if ep.URL == serverURL+"/d2" {
			t.Errorf("crawled beyond max depth: found %q", ep.URL)
		}
	}
}
