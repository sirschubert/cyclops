package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

// RobotsParser parses robots.txt and sitemap.xml files to discover endpoints
type RobotsParser struct {
	client    *http.Client
	userAgent string
	timeout   time.Duration
}

// NewRobotsParser creates a new robots.txt parser
func NewRobotsParser(options models.ScanOptions) *RobotsParser {
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

	return &RobotsParser{
		client:    client,
		userAgent: options.UserAgent,
		timeout:   time.Duration(options.Timeout) * time.Second,
	}
}

// ParseRobots fetches and parses robots.txt to discover disallowed paths
func (r *RobotsParser) ParseRobots(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedBase.Scheme, parsedBase.Host)
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, err
	}

	if r.userAgent != "" {
		req.Header.Set("User-Agent", r.userAgent)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("robots.txt returned status %d", resp.StatusCode)
	}

	// For now, we'll just create a placeholder endpoint for robots.txt
	// In a full implementation, we would parse the file and extract disallowed paths
	endpoint := models.Endpoint{
		URL:        robotsURL,
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "robots",
	}

	return []models.Endpoint{endpoint}, nil
}

// ParseSitemap fetches and parses sitemap.xml to discover URLs
func (r *RobotsParser) ParseSitemap(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", parsedBase.Scheme, parsedBase.Host)
	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	if r.userAgent != "" {
		req.Header.Set("User-Agent", r.userAgent)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sitemap.xml returned status %d", resp.StatusCode)
	}

	// For now, we'll just create a placeholder endpoint for sitemap.xml
	// In a full implementation, we would parse the XML and extract URLs
	endpoint := models.Endpoint{
		URL:        sitemapURL,
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "sitemap",
	}

	return []models.Endpoint{endpoint}, nil
}

// ParseAll parses both robots.txt and sitemap.xml
func (r *RobotsParser) ParseAll(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	var allEndpoints []models.Endpoint

	// Parse robots.txt
	robotsEndpoints, err := r.ParseRobots(ctx, baseURL)
	if err == nil {
		allEndpoints = append(allEndpoints, robotsEndpoints...)
	}

	// Parse sitemap.xml
	sitemapEndpoints, err := r.ParseSitemap(ctx, baseURL)
	if err == nil {
		allEndpoints = append(allEndpoints, sitemapEndpoints...)
	}

	return allEndpoints, nil
}