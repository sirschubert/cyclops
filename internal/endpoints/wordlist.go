package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
	"github.com/sirschubert/cyclops/internal/utils"
)

// WordlistBruteforcer performs wordlist-based directory bruteforcing
type WordlistBruteforcer struct {
	client      *http.Client
	wordlist    []string
	extensions  []string
	userAgent   string
	timeout     time.Duration
	threads     int
	rateLimiter *utils.RateLimiter
}

// NewWordlistBruteforcer creates a new wordlist bruteforcer
func NewWordlistBruteforcer(options models.ScanOptions, wordlist []string) *WordlistBruteforcer {
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

	// Common extensions to try
	extensions := []string{"", ".html", ".php", ".asp", ".aspx", ".jsp", ".cgi", ".bak", ".old", ".txt", ".zip", ".tar.gz"}

	return &WordlistBruteforcer{
		client:      client,
		wordlist:    wordlist,
		extensions:  extensions,
		userAgent:   options.UserAgent,
		timeout:     time.Duration(options.Timeout) * time.Second,
		threads:     options.Threads,
		rateLimiter: utils.NewRateLimiter(options.Rate),
	}
}

// Bruteforce discovers endpoints using a wordlist
func (w *WordlistBruteforcer) Bruteforce(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	var endpoints []models.Endpoint
	var endpointsMutex sync.Mutex
	var wg sync.WaitGroup

	// Create worker pool
	jobs := make(chan string, w.threads)

	// Start workers
	for i := 0; i < w.threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for word := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Apply rate limiting
				if w.rateLimiter != nil {
					if err := w.rateLimiter.Wait(ctx); err != nil {
						return
					}
				}

				// Try each extension with the word
				for _, ext := range w.extensions {
					path := "/" + word + ext
					endpoint, err := w.testPath(ctx, parsedBase, path)
					if err == nil && endpoint != nil {
						endpointsMutex.Lock()
						endpoints = append(endpoints, *endpoint)
						endpointsMutex.Unlock()
					}
				}
			}
		}()
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for _, word := range w.wordlist {
			select {
			case <-ctx.Done():
				return
			case jobs <- word:
			}
		}
	}()

	// Wait for completion
	wg.Wait()

	return endpoints, ctx.Err()
}

// testPath tests a single path for existence
func (w *WordlistBruteforcer) testPath(ctx context.Context, baseURL *url.URL, path string) (*models.Endpoint, error) {
	fullURL := baseURL.ResolveReference(&url.URL{Path: path}).String()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if w.userAgent != "" {
		req.Header.Set("User-Agent", w.userAgent)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Only consider interesting status codes (not 404)
	if resp.StatusCode != 404 {
		endpoint := models.Endpoint{
			URL:        fullURL,
			Method:     "GET",
			StatusCode: resp.StatusCode,
			Source:     "wordlist",
		}

		// Read response headers
		headers := make(map[string]string)
		for key, values := range resp.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		endpoint.Headers = headers

		return &endpoint, nil
	}

	return nil, nil
}