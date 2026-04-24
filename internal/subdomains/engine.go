package subdomains

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sirschubert/cyclops/pkg/models"
)

// DiscoveryEngine combines multiple subdomain discovery techniques.
type DiscoveryEngine struct {
	dnsResolver *DNSResolver
	options     models.ScanOptions
}

// NewDiscoveryEngine creates a new subdomain discovery engine.
func NewDiscoveryEngine(options models.ScanOptions) *DiscoveryEngine {
	nameserver := ""
	if len(options.Resolvers) > 0 {
		nameserver = options.Resolvers[0]
	}
	return &DiscoveryEngine{
		dnsResolver: NewDNSResolver(nameserver),
		options:     options,
	}
}

// Discover performs comprehensive subdomain discovery using all configured sources.
func (de *DiscoveryEngine) Discover(ctx context.Context, domain string) ([]string, error) {
	type sourceResult struct {
		subdomains []string
		source     string
	}

	results := make(chan sourceResult, 3)
	errors := make(chan error, 3)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		subs, err := de.dnsResolver.Enumerate(ctx, domain, de.options)
		if err != nil {
			errors <- fmt.Errorf("DNS enumeration failed: %v", err)
			return
		}
		results <- sourceResult{subdomains: subs, source: "dns"}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		subs, err := CertTransparency(ctx, domain)
		if err != nil {
			errors <- fmt.Errorf("certificate transparency failed: %v", err)
			return
		}
		results <- sourceResult{subdomains: subs, source: "cert"}
	}()

	if !de.options.PassiveOnly && de.options.Wordlist != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subs, err := de.dnsResolver.BruteForce(ctx, domain, de.options.Wordlist, de.options.Threads)
			if err != nil {
				errors <- fmt.Errorf("brute force failed: %v", err)
				return
			}
			results <- sourceResult{subdomains: subs, source: "bruteforce"}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	var allSubdomains []string
	var mu sync.Mutex

	for sr := range results {
		slog.Debug("subdomain source results", "source", sr.source, "count", len(sr.subdomains))
		mu.Lock()
		allSubdomains = append(allSubdomains, sr.subdomains...)
		mu.Unlock()
	}

	for err := range errors {
		slog.Debug("subdomain discovery partial failure", "err", err)
	}

	seen := make(map[string]bool, len(allSubdomains))
	unique := make([]string, 0, len(allSubdomains))
	for _, s := range allSubdomains {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}

	return unique, nil
}
