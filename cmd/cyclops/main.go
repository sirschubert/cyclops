package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/internal/endpoints"
	"github.com/sirschubert/cyclops/internal/hosts"
	outputfmt "github.com/sirschubert/cyclops/internal/output"
	"github.com/sirschubert/cyclops/internal/subdomains"
	"github.com/sirschubert/cyclops/pkg/models"
)

var (
	domain       = flag.String("d", "", "Target domain")
	wordlistPath = flag.String("w", "", "Subdomain wordlist path")
	threads      = flag.Int("t", 50, "Concurrent workers")
	rate         = flag.Int("r", 500, "Max requests per second")
	output       = flag.String("o", "", "Output file")
	format       = flag.String("format", "json", "Output format: json, html")
	depth        = flag.Int("depth", 2, "Crawl depth for endpoint discovery")
	passiveOnly  = flag.Bool("passive-only", false, "Only use passive sources (no DNS brute force)")
	proxy        = flag.String("proxy", "", "HTTP proxy URL")
	verbose      = flag.Bool("v", false, "Verbose output")
	timeout      = flag.Int("timeout", 10, "Per-request timeout in seconds")
	scanTimeout  = flag.Int("scan-timeout", 30, "Total scan timeout in minutes (0 = no limit)")
	userAgent    = flag.String("user-agent", "Cyclops/1.0", "User-Agent header")
	resolvers    = flag.String("resolvers", "", "Comma-separated list of DNS resolvers (e.g. 8.8.8.8,1.1.1.1)")
)

func main() {
	flag.Parse()

	if *domain == "" {
		fmt.Fprintln(os.Stderr, "Error: target domain is required (-d example.com)")
		flag.Usage()
		os.Exit(1)
	}

	// Configure structured logging: warnings/errors go to stderr.
	logLevel := slog.LevelWarn
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	// Parse DNS resolvers.
	var resolverList []string
	if *resolvers != "" {
		for _, r := range strings.Split(*resolvers, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				// Append default port if missing.
				if !strings.Contains(r, ":") {
					r += ":53"
				}
				resolverList = append(resolverList, r)
			}
		}
	}

	// Load subdomain wordlist from disk if specified.
	wordlist := ""
	if *wordlistPath != "" {
		data, err := os.ReadFile(*wordlistPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not read wordlist %q: %v\n", *wordlistPath, err)
			os.Exit(1)
		}
		wordlist = string(data)
	}

	options := models.ScanOptions{
		Domain:      *domain,
		Wordlist:    wordlist,
		Threads:     *threads,
		Rate:        *rate,
		Output:      *output,
		Format:      *format,
		Depth:       *depth,
		PassiveOnly: *passiveOnly,
		Proxy:       *proxy,
		Verbose:     *verbose,
		Timeout:     *timeout,
		UserAgent:   *userAgent,
		Resolvers:   resolverList,
	}

	result := models.Result{
		Domain:   *domain,
		ScanTime: time.Now().UTC(),
	}

	// Build root context with optional total scan timeout.
	ctx := context.Background()
	if *scanTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*scanTimeout)*time.Minute)
		defer cancel()
	}

	// Phase 1: Subdomain enumeration
	fmt.Println("[*] Starting subdomain enumeration...")
	subdomainsFound, err := runSubdomainEnumeration(ctx, options)
	if err != nil {
		log.Printf("Error during subdomain enumeration: %v", err)
	} else {
		result.Subdomains = subdomainsFound
		fmt.Printf("[+] Found %d subdomains\n", len(subdomainsFound))
	}

	// Phase 2: Host discovery
	fmt.Println("[*] Starting host discovery...")
	hostsFound, err := runHostDiscovery(ctx, options, result.Subdomains)
	if err != nil {
		log.Printf("Error during host discovery: %v", err)
	} else {
		for i, sub := range result.Subdomains {
			for _, host := range hostsFound {
				hostname := strings.TrimPrefix(host.URL, "http://")
				hostname = strings.TrimPrefix(hostname, "https://")
				if strings.HasPrefix(hostname, sub.Name) {
					result.Subdomains[i].Hosts = append(result.Subdomains[i].Hosts, host)
				}
			}
		}
		fmt.Printf("[+] Found %d live hosts\n", len(hostsFound))
	}

	// Phase 3: Endpoint discovery
	fmt.Println("[*] Starting endpoint discovery...")
	endpointsFound, err := runEndpointDiscovery(ctx, options, hostsFound)
	if err != nil {
		log.Printf("Error during endpoint discovery: %v", err)
	} else {
		for i, sub := range result.Subdomains {
			for j, host := range sub.Hosts {
				for _, ep := range endpointsFound {
					if strings.HasPrefix(ep.URL, host.URL) {
						result.Subdomains[i].Hosts[j].Endpoints = append(
							result.Subdomains[i].Hosts[j].Endpoints, ep,
						)
					}
				}
			}
		}
		fmt.Printf("[+] Found %d endpoints\n", len(endpointsFound))
	}

	// Output results
	fmt.Println("[*] Generating output...")
	if err := outputResults(result, options); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[*] Scan completed!")
}

func runSubdomainEnumeration(ctx context.Context, options models.ScanOptions) ([]models.Subdomain, error) {
	var allSubdomains []string

	fmt.Println("  [*] Performing subdomain discovery...")

	// Certificate transparency lookup
	fmt.Println("    [*] Checking certificate transparency logs...")
	certSubdomains, err := subdomains.CertTransparency(ctx, options.Domain)
	if err != nil {
		slog.Warn("certificate transparency lookup failed", "err", err)
	} else {
		fmt.Printf("    [+] Found %d subdomains from certificates\n", len(certSubdomains))
		allSubdomains = append(allSubdomains, certSubdomains...)
	}

	// DNS brute force (if not passive-only)
	if !options.PassiveOnly {
		fmt.Println("    [*] Performing DNS brute force...")

		// Pick the first custom resolver if provided, else default.
		nameserver := "8.8.8.8:53"
		if len(options.Resolvers) > 0 {
			nameserver = options.Resolvers[0]
		}
		dnsResolver := subdomains.NewDNSResolver(nameserver)

		// Use the provided wordlist, or fall back to the built-in default.
		wordlistStr := options.Wordlist
		if wordlistStr == "" {
			wordlistStr = strings.Join(getDefaultWordlist(), "\n")
		}

		dnsSubdomains, err := dnsResolver.BruteForce(ctx, options.Domain, wordlistStr, options.Threads)
		if err != nil {
			slog.Warn("DNS brute force failed", "err", err)
		} else {
			fmt.Printf("    [+] Found %d subdomains from DNS brute force\n", len(dnsSubdomains))
			allSubdomains = append(allSubdomains, dnsSubdomains...)
		}
	}

	// Deduplicate
	seen := make(map[string]bool, len(allSubdomains))
	var subdomainObjects []models.Subdomain
	for _, sub := range allSubdomains {
		if !seen[sub] {
			seen[sub] = true
			subdomainObjects = append(subdomainObjects, models.Subdomain{
				Name:    sub,
				Sources: []string{"mixed"},
			})
		}
	}

	return subdomainObjects, nil
}

func runHostDiscovery(ctx context.Context, options models.ScanOptions, subds []models.Subdomain) ([]models.Host, error) {
	probe := hosts.NewHTTPProbe(options.Timeout, options.UserAgent)

	var urlsToProbe []string
	for _, sub := range subds {
		urlsToProbe = append(urlsToProbe, "http://"+sub.Name)
		urlsToProbe = append(urlsToProbe, "https://"+sub.Name)
	}

	results, err := probe.ProbeHosts(ctx, urlsToProbe, options.Threads)
	if err != nil {
		return nil, err
	}

	// Run technology fingerprinting on all live hosts.
	results = hosts.FingerprintHosts(results)

	if options.Verbose {
		for _, host := range results {
			fmt.Printf("  [+] %s [%d]\n", host.URL, host.StatusCode)
		}
	}

	return results, nil
}

func runEndpointDiscovery(ctx context.Context, options models.ScanOptions, liveHosts []models.Host) ([]models.Endpoint, error) {
	var allEndpoints []models.Endpoint

	for _, host := range liveHosts {
		fmt.Printf("  [*] Crawling %s...\n", host.URL)

		crawler := endpoints.NewCrawler(options)
		robotsParser := endpoints.NewRobotsParser(options)

		if crawledEndpoints, err := crawler.Crawl(ctx, host.URL); err == nil {
			allEndpoints = append(allEndpoints, crawledEndpoints...)
		}
		if robotsEndpoints, err := robotsParser.ParseAll(ctx, host.URL); err == nil {
			allEndpoints = append(allEndpoints, robotsEndpoints...)
		}
	}

	return allEndpoints, nil
}

func outputResults(result models.Result, options models.ScanOptions) error {
	var formatter interface {
		WriteToFile(models.Result, string) error
		WriteToStdout(models.Result) error
	}

	switch options.Format {
	case "html":
		formatter = outputfmt.NewHTMLFormatter()
	default:
		formatter = outputfmt.NewJSONFormatter()
	}

	if options.Output != "" {
		if err := formatter.WriteToFile(result, options.Output); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("[*] Results written to %s\n", options.Output)
	} else {
		if err := formatter.WriteToStdout(result); err != nil {
			return fmt.Errorf("failed to write output to stdout: %w", err)
		}
	}

	return nil
}

func getDefaultWordlist() []string {
	return []string{
		"www", "mail", "ftp", "localhost", "webmail", "smtp", "pop", "ns1", "ns2",
		"ns3", "ns4", "api", "admin", "dev", "test", "blog", "cms", "shop", "forum",
		"wiki", "docs", "login", "portal", "secure", "vpn", "mysql", "sql", "db",
		"database", "redis", "elastic", "cache", "cdn", "static", "assets", "img",
		"image", "video", "music", "files", "download", "upload", "media", "stage",
		"staging", "prod", "production", "demo", "monitor", "status", "stats",
		"analytics", "metrics", "search", "chat", "support", "help", "contact",
		"info", "about", "news", "press", "career", "jobs", "hr", "finance",
		"pay", "payment", "billing", "invoice", "auth", "oauth", "sso", "ldap",
		"idp", "saml", "openid", "profile", "account", "user", "users", "member",
		"membership", "community", "social", "api-docs", "swagger", "graphql",
		"rest", "soap", "ws", "websocket", "rpc", "grpc", "xmlrpc", "jsonrpc",
		"reports", "dashboards", "dash", "ui", "frontend", "backend", "app",
		"mobile", "ios", "android", "client", "server", "core", "main", "master",
		"slave", "worker", "node", "cluster", "grid", "queue", "mq", "rabbitmq",
		"kafka", "pubsub", "notification", "notify", "alert", "alarm", "event",
		"log", "logs", "logging", "audit", "security", "firewall", "fw", "ids",
		"ips", "siem", "soc", "ticket", "tickets", "issue", "issues", "bug",
		"bugs", "features", "roadmap", "planning", "project",
		"projects", "tasks", "todo", "kanban", "scrum", "agile", "waterfall",
		"ci", "cd", "jenkins", "travis", "circle", "gitlab", "github", "bitbucket",
		"repo", "repository", "svn", "mercurial", "docker", "kubernetes", "k8s",
		"openshift", "containers", "vm", "virtual", "cloud", "aws",
		"azure", "gcp", "google", "ibm", "oracle", "terraform", "ansible", "chef",
		"puppet", "salt", "configuration", "config", "conf", "settings", "env",
		"environment", "devops", "sre", "operations", "ops", "sysadmin", "admin",
		"root", "backup", "restore", "archive", "backup-server", "nas", "san",
		"storage", "s3", "bucket", "fileserver", "share", "shared", "collaboration",
		"intranet", "extranet", "dmz", "edge", "cdn-edge", "gateway", "proxy",
		"reverse-proxy", "load-balancer", "lb", "haproxy", "nginx", "apache",
		"iis", "tomcat", "jetty", "undertow", "nodejs", "express", "django",
		"flask", "rails", "spring", "laravel", "symfony", "zend", "cakephp",
		"codeigniter", "yii", "wordpress", "drupal", "joomla", "magento", "prestashop",
		"shopify", "bigcommerce", "woocommerce", "opencart", "oscommerce", "virtuemart",
	}
}
