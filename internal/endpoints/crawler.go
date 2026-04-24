package endpoints

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/internal/utils"
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
	userAgents  []string // stealth: rotate per request
	timeout     time.Duration
	concurrency int
	rateLimiter models.RateLimiterIface
	stealthMode bool
	reportCode  func(int)
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

	// Use shared rate limiter if provided (e.g. autotune), otherwise create one.
	var rl models.RateLimiterIface
	if options.RateLimiter != nil {
		rl = options.RateLimiter
	} else {
		rl = utils.NewRateLimiter(options.Rate)
	}

	return &Crawler{
		client:      client,
		maxDepth:    options.Depth,
		visited:     make(map[string]bool),
		endpoints:   make(map[string]models.Endpoint),
		userAgent:   options.UserAgent,
		userAgents:  options.UserAgents,
		timeout:     time.Duration(options.Timeout) * time.Second,
		concurrency: options.Threads,
		rateLimiter: rl,
		stealthMode: options.Mode == "stealth",
		reportCode:  options.ReportCode,
	}
}

// pickUserAgent returns the UA to use for the next request.
func (c *Crawler) pickUserAgent() string {
	if len(c.userAgents) > 0 {
		return c.userAgents[rand.IntN(len(c.userAgents))]
	}
	return c.userAgent
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

	queue := []urlVisit{{URL: parsedBase, Depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		select {
		case <-ctx.Done():
			return c.getEndpoints(), ctx.Err()
		default:
		}

		if current.Depth > c.maxDepth {
			continue
		}

		urlStr := current.URL.String()
		if c.visited[urlStr] {
			continue
		}
		c.visited[urlStr] = true

		endpoints, links, err := c.fetchAndParse(ctx, current.URL)
		if err != nil {
			continue
		}

		for _, endpoint := range endpoints {
			if _, exists := c.endpoints[endpoint.URL]; !exists {
				c.endpoints[endpoint.URL] = endpoint
			}
		}

		if current.Depth < c.maxDepth {
			for _, link := range links {
				parsedLink, err := url.Parse(link)
				if err != nil {
					continue
				}
				resolvedURL := current.URL.ResolveReference(parsedLink)
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
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, nil, err
		}
	}

	// Stealth: random 1–4 s delay between requests.
	if c.stealthMode {
		delay := time.Duration(1+rand.IntN(3)) * time.Second
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("User-Agent", c.pickUserAgent())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if c.reportCode != nil {
		c.reportCode(resp.StatusCode)
	}

	endpoint := models.Endpoint{
		URL:        u.String(),
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "crawler",
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	endpoint.Headers = headers

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []models.Endpoint{endpoint}, nil, nil
	}

	links := c.extractLinks(body, u)
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
			}

			if attrName != "" {
				for _, attr := range n.Attr {
					if attr.Key == attrName {
						attrValue = attr.Val
						break
					}
				}

				if attrValue != "" {
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
