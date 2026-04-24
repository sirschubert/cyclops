package endpoints

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
	"golang.org/x/net/html"
)

// Crawler performs breadth-first web crawling to discover endpoints
type Crawler struct {
	client      *http.Client
	maxDepth    int
	visited     map[string]bool
	endpoints   map[string]models.Endpoint
	userAgent   string
	timeout     time.Duration
	concurrency int
}

// NewCrawler creates a new web crawler
func NewCrawler(options models.ScanOptions) *Crawler {
 transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
	}

	if options.Proxy != "" {
		proxyURL, err := url.Parse(options.Proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &Crawler{
		client:      client,
		maxDepth:    options.Depth,
		visited:     make(map[string]bool),
		endpoints:   make(map[string]models.Endpoint),
		userAgent:   options.UserAgent,
		timeout:     time.Duration(options.Timeout) * time.Second,
		concurrency: options.Threads,
	}
}

// Crawl discovers endpoints from a given URL up to max depth.
func (c *Crawler) Crawl(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Start crawling from the base URL
	queue := []urlVisit{{URL: parsedBase, Depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check context cancellation
		select {
		case <-ctx.Done():
			return c.getEndpoints(), ctx.Err()
		default:
		}

		// Skip if we've reached max depth
		if current.Depth > c.maxDepth {
			continue
		}

		// Skip if already visited
		urlStr := current.URL.String()
		if c.visited[urlStr] {
			continue
		}
		c.visited[urlStr] = true

		// Fetch the page
		endpoints, links, err := c.fetchAndParse(ctx, current.URL)
		if err != nil {
			// Log error but continue crawling
			continue
		}

		// Add discovered endpoints
		for _, endpoint := range endpoints {
			if _, exists := c.endpoints[endpoint.URL]; !exists {
				c.endpoints[endpoint.URL] = endpoint
			}
		}

		// Add links to queue for further crawling (but not beyond max depth)
		if current.Depth < c.maxDepth {
			for _, link := range links {
				parsedLink, err := url.Parse(link)
				if err != nil {
					continue
				}

				// Resolve relative URLs
				resolvedURL := current.URL.ResolveReference(parsedLink)

				// Only crawl same domain
				if resolvedURL.Host == parsedBase.Host {
					queue = append(queue, urlVisit{URL: resolvedURL, Depth: current.Depth + 1})
				}
			}
		}
	}

	return c.getEndpoints(), nil
}

// urlVisit tracks a URL and its depth in the crawl queue
type urlVisit struct {
	URL   *url.URL
	Depth int
}

// fetchAndParse fetches a URL and extracts endpoints and links
func (c *Crawler) fetchAndParse(ctx context.Context, u *url.URL) ([]models.Endpoint, []string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	// Create endpoint for this URL
	endpoint := models.Endpoint{
		URL:        u.String(),
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "crawler",
	}

	// Read response headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	endpoint.Headers = headers

	// Read body for link extraction
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []models.Endpoint{endpoint}, nil, nil
	}

	// Extract links from HTML
	links := c.extractLinks(body, u)

	// Return the endpoint and discovered links
	return []models.Endpoint{endpoint}, links, nil
}

// extractLinks parses HTML and extracts links
func (c *Crawler) extractLinks(body []byte, baseURL *url.URL) []string {
	var links []string

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return links
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var attrName, attrValue string
			switch n.Data {
			case "a":
				attrName = "href"
			case "link":
				attrName = "href"
			case "script":
				attrName = "src"
			case "img":
				attrName = "src"
			case "form":
				attrName = "action"
			default:
				// Continue to children
			}

			if attrName != "" {
				for _, attr := range n.Attr {
					if attr.Key == attrName {
						attrValue = attr.Val
						break
					}
				}

				if attrValue != "" {
					// Resolve relative URLs
					if parsedURL, err := url.Parse(attrValue); err == nil {
						resolvedURL := baseURL.ResolveReference(parsedURL)
						links = append(links, resolvedURL.String())
					}
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child)
		}
	}

	f(doc)
	return links
}

// getEndpoints returns all discovered endpoints
func (c *Crawler) getEndpoints() []models.Endpoint {
	endpoints := make([]models.Endpoint, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}