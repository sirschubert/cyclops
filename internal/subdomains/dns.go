package subdomains

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/sirschubert/cyclops/pkg/models"
)

// DNSResolver handles DNS lookups and subdomain enumeration.
type DNSResolver struct {
	resolver   *net.Resolver
	nameserver string
	timeout    time.Duration
}

// NewDNSResolver creates a new DNS resolver.
func NewDNSResolver(nameserver string) *DNSResolver {
	if nameserver == "" {
		nameserver = "8.8.8.8:53"
	}

	return &DNSResolver{
		resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Second * 5,
				}
				return d.DialContext(ctx, network, nameserver)
			},
		},
		nameserver: nameserver,
		timeout:    time.Second * 5,
	}
}

// Resolve performs a DNS lookup for the given domain and record type.
func (d *DNSResolver) Resolve(domain string, recordType uint16) ([]string, error) {
	var results []string

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), recordType)
	msg.RecursionDesired = true

	client := &dns.Client{Timeout: d.timeout}
	resp, _, err := client.Exchange(msg, d.nameserver)
	if err != nil {
		return nil, err
	}

	for _, answer := range resp.Answer {
		switch record := answer.(type) {
		case *dns.A:
			results = append(results, record.A.String())
		case *dns.AAAA:
			results = append(results, record.AAAA.String())
		case *dns.CNAME:
			results = append(results, strings.TrimSuffix(record.Target, "."))
		case *dns.TXT:
			for _, txt := range record.Txt {
				results = append(results, txt)
			}
		case *dns.MX:
			results = append(results, strings.TrimSuffix(record.Mx, "."))
		case *dns.NS:
			results = append(results, strings.TrimSuffix(record.Ns, "."))
		}
	}

	return results, nil
}

// BruteForce performs DNS brute-force enumeration.
// It respects ctx cancellation between worker iterations.
func (d *DNSResolver) BruteForce(ctx context.Context, domain, wordlist string, workers int) ([]string, error) {
	words := strings.Split(wordlist, "\n")
	var subdomains []string
	var mu sync.Mutex

	pool := &sync.WaitGroup{}
	jobs := make(chan string, workers)

	for i := 0; i < workers; i++ {
		pool.Add(1)
		go func() {
			defer pool.Done()
			for word := range jobs {
				if word == "" {
					continue
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				subdomain := fmt.Sprintf("%s.%s", strings.TrimSpace(word), domain)
				ips, err := d.resolver.LookupIPAddr(ctx, subdomain)
				if err == nil && len(ips) > 0 {
					mu.Lock()
					subdomains = append(subdomains, subdomain)
					mu.Unlock()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, word := range words {
			select {
			case <-ctx.Done():
				return
			case jobs <- word:
			}
		}
	}()

	pool.Wait()

	if err := ctx.Err(); err != nil {
		return subdomains, err
	}
	return subdomains, nil
}

// DetectWildcard checks for wildcard DNS records.
func (d *DNSResolver) DetectWildcard(ctx context.Context, domain string) (bool, string, error) {
	randomSub := fmt.Sprintf("cyclops-wildcard-test-%d.%s", time.Now().UnixNano(), domain)

	ips, err := d.resolver.LookupIPAddr(ctx, randomSub)
	if err != nil {
		return false, "", nil
	}

	if len(ips) > 0 {
		return true, ips[0].IP.String(), nil
	}

	return false, "", nil
}

// ZoneTransfer attempts zone transfer for all nameservers.
func (d *DNSResolver) ZoneTransfer(ctx context.Context, domain string) ([]string, error) {
	nsRecords, err := d.Resolve(domain, dns.TypeNS)
	if err != nil {
		return nil, err
	}

	var subdomains []string

	for _, ns := range nsRecords {
		select {
		case <-ctx.Done():
			return subdomains, ctx.Err()
		default:
		}

		nsIPs, err := d.resolver.LookupIPAddr(ctx, ns)
		if err != nil || len(nsIPs) == 0 {
			continue
		}

		nsIP := nsIPs[0].IP.String()
		nameserver := fmt.Sprintf("%s:53", nsIP)

		transfer := &dns.Transfer{}
		msg := &dns.Msg{}
		msg.SetAxfr(dns.Fqdn(domain))

		env, err := transfer.In(msg, nameserver)
		if err != nil {
			continue
		}

		for envelope := range env {
			if envelope.Error != nil {
				break
			}

			for _, rr := range envelope.RR {
				switch record := rr.(type) {
				case *dns.A:
					name := strings.TrimSuffix(record.Header().Name, "."+domain+".")
					if name != domain && !strings.Contains(name, "*") {
						subdomains = append(subdomains, fmt.Sprintf("%s.%s", name, domain))
					}
				case *dns.CNAME:
					name := strings.TrimSuffix(record.Header().Name, "."+domain+".")
					if name != domain && !strings.Contains(name, "*") {
						subdomains = append(subdomains, fmt.Sprintf("%s.%s", name, domain))
					}
				}
			}
		}
	}

	return subdomains, nil
}

// PTRLookup performs reverse DNS lookup.
func (d *DNSResolver) PTRLookup(ctx context.Context, ip string) ([]string, error) {
	return d.resolver.LookupAddr(ctx, ip)
}

// Enumerate performs comprehensive subdomain enumeration.
func (d *DNSResolver) Enumerate(ctx context.Context, domain string, options models.ScanOptions) ([]string, error) {
	var allSubdomains []string
	var mu sync.Mutex

	wildcard, _, err := d.DetectWildcard(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to detect wildcard: %v", err)
	}

	if wildcard && !options.PassiveOnly {
		return nil, fmt.Errorf("wildcard DNS detected, brute force enumeration would be ineffective")
	}

	recordTypes := []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeMX, dns.TypeNS, dns.TypeTXT}

	var wg sync.WaitGroup
	resultChan := make(chan []string, len(recordTypes))

	for _, recordType := range recordTypes {
		wg.Add(1)
		go func(rt uint16) {
			defer wg.Done()
			results, _ := d.Resolve(domain, rt)
			resultChan <- results
		}(recordType)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for results := range resultChan {
		mu.Lock()
		allSubdomains = append(allSubdomains, results...)
		mu.Unlock()
	}

	if !options.PassiveOnly {
		zoneSubs, _ := d.ZoneTransfer(ctx, domain)
		mu.Lock()
		allSubdomains = append(allSubdomains, zoneSubs...)
		mu.Unlock()
	}

	return allSubdomains, nil
}

// MultiResolver performs DNS brute-force using multiple nameservers in round-robin.
type MultiResolver struct {
	resolvers []*DNSResolver
}

// NewMultiResolver creates a MultiResolver from a list of nameserver addresses.
// Each address should be in "ip:port" format (port defaults to 53 if absent).
// Falls back to a single 8.8.8.8:53 resolver if the list is empty.
func NewMultiResolver(nameservers []string) *MultiResolver {
	if len(nameservers) == 0 {
		nameservers = []string{"8.8.8.8:53"}
	}
	resolvers := make([]*DNSResolver, 0, len(nameservers))
	for _, ns := range nameservers {
		if !strings.Contains(ns, ":") {
			ns += ":53"
		}
		resolvers = append(resolvers, NewDNSResolver(ns))
	}
	return &MultiResolver{resolvers: resolvers}
}

// BruteForce performs DNS brute-force enumeration distributing work across all resolvers.
// Workers are assigned resolvers in round-robin order so the load is spread evenly.
func (m *MultiResolver) BruteForce(ctx context.Context, domain, wordlist string, workers int) ([]string, error) {
	words := strings.Split(wordlist, "\n")
	var subdomains []string
	var mu sync.Mutex

	pool := &sync.WaitGroup{}
	jobs := make(chan string, workers)

	for i := 0; i < workers; i++ {
		resolver := m.resolvers[i%len(m.resolvers)] // round-robin
		pool.Add(1)
		go func(r *DNSResolver) {
			defer pool.Done()
			for word := range jobs {
				if word == "" {
					continue
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
				subdomain := fmt.Sprintf("%s.%s", strings.TrimSpace(word), domain)
				ips, err := r.resolver.LookupIPAddr(ctx, subdomain)
				if err == nil && len(ips) > 0 {
					mu.Lock()
					subdomains = append(subdomains, subdomain)
					mu.Unlock()
				}
			}
		}(resolver)
	}

	go func() {
		defer close(jobs)
		for _, word := range words {
			select {
			case <-ctx.Done():
				return
			case jobs <- word:
			}
		}
	}()

	pool.Wait()

	if err := ctx.Err(); err != nil {
		return subdomains, err
	}
	return subdomains, nil
}
