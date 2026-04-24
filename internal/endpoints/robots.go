package endpoints

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/pkg/models"
)

// RobotsParser parses robots.txt and sitemap.xml files to discover endpoints.
type RobotsParser struct {
	client    *http.Client
	userAgent string
	timeout   time.Duration
}

// NewRobotsParser creates a new robots.txt parser.
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

// ParseRobots fetches and parses robots.txt, returning each Disallow path as an endpoint.
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

	var endpoints []models.Endpoint

	// robots.txt itself is an endpoint.
	endpoints = append(endpoints, models.Endpoint{
		URL:        robotsURL,
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "robots",
	})

	// Parse each line for Disallow and Sitemap directives.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var directive, value string
		if idx := strings.Index(line, ":"); idx != -1 {
			directive = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+1:])
		}

		switch strings.ToLower(directive) {
		case "disallow":
			if value != "" && value != "/" {
				fullURL := fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, value)
				endpoints = append(endpoints, models.Endpoint{
					URL:    fullURL,
					Method: "GET",
					Source: "robots",
				})
			}
		case "allow":
			if value != "" && value != "/" {
				fullURL := fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, value)
				endpoints = append(endpoints, models.Endpoint{
					URL:    fullURL,
					Method: "GET",
					Source: "robots",
				})
			}
		case "sitemap":
			if value != "" {
				// Recursively parse linked sitemaps.
				sitemapEndpoints, err := r.parseSitemapURL(ctx, value)
				if err == nil {
					endpoints = append(endpoints, sitemapEndpoints...)
				}
			}
		}
	}

	return endpoints, nil
}

// sitemapURLSet is the XML structure for a sitemap.
type sitemapURLSet struct {
	XMLName xml.Name      `xml:"urlset"`
	URLs    []sitemapURL  `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

// sitemapIndex is the XML structure for a sitemap index.
type sitemapIndex struct {
	XMLName  xml.Name         `xml:"sitemapindex"`
	Sitemaps []sitemapEntry   `xml:"sitemap"`
}

type sitemapEntry struct {
	Loc string `xml:"loc"`
}

// ParseSitemap fetches and parses sitemap.xml, returning each URL as an endpoint.
func (r *RobotsParser) ParseSitemap(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", parsedBase.Scheme, parsedBase.Host)
	return r.parseSitemapURL(ctx, sitemapURL)
}

// parseSitemapURL fetches and parses a sitemap at an arbitrary URL.
func (r *RobotsParser) parseSitemapURL(ctx context.Context, sitemapURL string) ([]models.Endpoint, error) {
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
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var endpoints []models.Endpoint

	// Record the sitemap file itself.
	endpoints = append(endpoints, models.Endpoint{
		URL:        sitemapURL,
		Method:     "GET",
		StatusCode: resp.StatusCode,
		Source:     "sitemap",
	})

	// Try parsing as a sitemap index first.
	var index sitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		for _, entry := range index.Sitemaps {
			if entry.Loc != "" {
				childEndpoints, err := r.parseSitemapURL(ctx, entry.Loc)
				if err == nil {
					endpoints = append(endpoints, childEndpoints...)
				}
			}
		}
		return endpoints, nil
	}

	// Fall back to parsing as a regular urlset.
	var urlset sitemapURLSet
	if err := xml.Unmarshal(body, &urlset); err != nil {
		return endpoints, nil // return at least the sitemap URL itself
	}

	for _, u := range urlset.URLs {
		if u.Loc != "" {
			endpoints = append(endpoints, models.Endpoint{
				URL:    u.Loc,
				Method: "GET",
				Source: "sitemap",
			})
		}
	}

	return endpoints, nil
}

// ParseAll parses both robots.txt and sitemap.xml.
func (r *RobotsParser) ParseAll(ctx context.Context, baseURL string) ([]models.Endpoint, error) {
	var allEndpoints []models.Endpoint

	if robotsEndpoints, err := r.ParseRobots(ctx, baseURL); err == nil {
		allEndpoints = append(allEndpoints, robotsEndpoints...)
	}

	if sitemapEndpoints, err := r.ParseSitemap(ctx, baseURL); err == nil {
		allEndpoints = append(allEndpoints, sitemapEndpoints...)
	}

	return allEndpoints, nil
}
