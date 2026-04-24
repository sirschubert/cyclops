package models

import (
	"time"
)

// Endpoint represents a discovered HTTP endpoint
type Endpoint struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Source     string            `json:"source"` // crawler, wordlist, js-analysis, sitemap, robots
}

// Host represents a live host with its metadata
type Host struct {
	URL           string     `json:"url"`
	Scheme        string     `json:"scheme"`
	StatusCode    int        `json:"status_code"`
	Title         string     `json:"title"`
	Server        string     `json:"server,omitempty"`
	Tech          []string   `json:"tech,omitempty"`
	ContentLength int        `json:"content_length"`
	Headers       map[string]string `json:"headers,omitempty"`
	Endpoints     []Endpoint `json:"endpoints,omitempty"`
	BodyPreview   string     `json:"body_preview,omitempty"`
}

// Subdomain represents a discovered subdomain with its hosts
type Subdomain struct {
	Name      string     `json:"subdomain"`
	Sources   []string   `json:"sources"`
	IP        string     `json:"ip,omitempty"`
	Hosts     []Host     `json:"hosts,omitempty"`
}

// Result is the top-level scan result
type Result struct {
	Domain     string       `json:"domain"`
	ScanTime   time.Time    `json:"scan_time"`
	Subdomains []Subdomain  `json:"subdomains"`
}

// ScanOptions holds configuration for a scan
type ScanOptions struct {
	Domain       string
	Wordlist     string
	Threads      int
	Rate         int
	Output       string
	Format       string
	Depth        int
	PassiveOnly  bool
	Proxy        string
	Verbose      bool
	Timeout      int
	UserAgent    string
	Resolvers    []string
}
