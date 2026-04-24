package hosts

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirschubert/cyclops/internal/utils"
	"github.com/sirschubert/cyclops/pkg/models"
	"golang.org/x/net/html"
)

const bodyPreviewLimit = 512 * 1024 // 1MB read for fingerprinting
const bodyPreviewStoreLimit = 512   // bytes stored in model / serialized to JSON

// HTTPProbe performs HTTP/HTTPS probing of hosts.
type HTTPProbe struct {
	client    *http.Client
	timeout   time.Duration
	userAgent string
}

// NewHTTPProbe creates a new HTTP probe.
func NewHTTPProbe(timeout int, userAgent string) *HTTPProbe {
	if timeout <= 0 {
		timeout = 10
	}
	if userAgent == "" {
		userAgent = "Cyclops/1.0"
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Duration(timeout) * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: time.Duration(timeout) * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // scan tool must accept self-signed certs
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConnsPerHost: 10,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return &HTTPProbe{
		client:    client,
		timeout:   time.Duration(timeout) * time.Second,
		userAgent: userAgent,
	}
}

// ProbeHost probes a single URL and returns host metadata.
// It retries up to 2 times on transient network errors.
func (p *HTTPProbe) ProbeHost(ctx context.Context, url string) (*models.Host, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	var (
		resp *http.Response
		body []byte
	)

	err := utils.RetryWithBackoff(ctx, 2, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", p.userAgent)
		req.Header.Set("Accept", "*/*")

		r, err := p.client.Do(req)
		if err != nil {
			return err
		}
		defer r.Body.Close()

		b, err := io.ReadAll(io.LimitReader(r.Body, bodyPreviewLimit))
		if err != nil {
			b = []byte{}
		}
		resp = r
		body = b
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to probe host: %v", err)
	}

	title := extractTitle(string(body))
	server := resp.Header.Get("Server")
	if server == "" {
		server = resp.Header.Get("X-Powered-By")
	}

	headers := make(map[string]string, len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	scheme := "http"
	if strings.HasPrefix(url, "https://") {
		scheme = "https"
	}

	preview := string(body)
	if len(preview) > bodyPreviewStoreLimit {
		preview = preview[:bodyPreviewStoreLimit]
	}

	return &models.Host{
		URL:           url,
		Scheme:        scheme,
		StatusCode:    resp.StatusCode,
		Title:         title,
		Server:        server,
		ContentLength: len(body),
		Headers:       headers,
		BodyPreview:   preview,
	}, nil
}

// ProbeHosts probes multiple hosts concurrently using a bounded worker pool.
func (p *HTTPProbe) ProbeHosts(ctx context.Context, urls []string, threads int) ([]models.Host, error) {
	if threads <= 0 {
		threads = 10
	}

	jobs := make(chan string, len(urls))
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	var (
		mu      sync.Mutex
		results []models.Host
		wg      sync.WaitGroup
	)

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				host, err := p.ProbeHost(ctx, u)
				if err != nil {
					continue // unreachable host is expected; skip silently
				}
				mu.Lock()
				results = append(results, *host)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results, ctx.Err()
}

func extractTitle(htmlContent string) string {
	if !strings.Contains(strings.ToLower(htmlContent), "<title") {
		return ""
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return strings.TrimSpace(title)
}

// ProbeScheme tries both HTTPS and HTTP for a hostname.
func (p *HTTPProbe) ProbeScheme(ctx context.Context, host string) []models.Host {
	var results []models.Host

	if h, err := p.ProbeHost(ctx, "https://"+host); err == nil {
		results = append(results, *h)
	}
	if h, err := p.ProbeHost(ctx, "http://"+host); err == nil {
		results = append(results, *h)
	}

	return results
}
