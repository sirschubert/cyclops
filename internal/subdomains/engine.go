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
	return &DiscoveryEngine{
		dnsResolver: NewDNSResolver(""),
		options:     options,
	}
}

// Discover performs comprehensive subdomain discovery using all configured sources.
func (de *DiscoveryEngine) Discover(ctx context.Context, domain string) ([]string, error) {
	var allSubdomains []string
	var mu sync.Mutex

	results := make(chan []string, 3)
	errors := make(chan error, 3)

	// DNS Enumeration
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		subs, err := de.dnsResolver.Enumerate(ctx, domain, de.options)
		if err != nil {
			errors <- fmt.Errorf("DNS enumeration failed: %v", err)
			return
		}
		results <- subs
	}()

	// Certificate Transparency
	wg.Add(1)
	go func() {
		defer wg.Done()
		subs, err := CertTransparency(ctx, domain)
		if err != nil {
			errors <- fmt.Errorf("certificate transparency failed: %v", err)
			return
		}
		results <- subs
	}()

	// Brute Force (only if wordlist provided and not passive mode)
	if !de.options.PassiveOnly && de.options.Wordlist != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subs, err := de.dnsResolver.BruteForce(ctx, domain, de.options.Wordlist, de.options.Threads)
			if err != nil {
				errors <- fmt.Errorf("brute force failed: %v", err)
				return
			}
			results <- subs
		}()
	}

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	for subs := range results {
		mu.Lock()
		allSubdomains = append(allSubdomains, subs...)
		mu.Unlock()
	}

	// Drain any errors and log them as warnings.
	for err := range errors {
		slog.Warn("subdomain discovery partial failure", "err", err)
	}

	// Deduplicate
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
