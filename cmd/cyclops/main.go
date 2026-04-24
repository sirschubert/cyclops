package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/sirschubert/cyclops/internal/endpoints"
	"github.com/sirschubert/cyclops/internal/hosts"
	outputfmt "github.com/sirschubert/cyclops/internal/output"
	"github.com/sirschubert/cyclops/internal/subdomains"
	"github.com/sirschubert/cyclops/internal/utils"
	"github.com/sirschubert/cyclops/pkg/models"
)

// clearLineWriter prepends the ANSI "move to start of line + erase" sequence
// before every write so that slog output doesn't collide with the spinner.
type clearLineWriter struct{ w io.Writer }

func (c clearLineWriter) Write(p []byte) (int, error) {
	_, _ = c.w.Write([]byte("\r\033[K"))
	_, err := c.w.Write(p)
	return len(p), err
}

// ── Color helpers ────────────────────────────────────────────────────────────

var (
	cyan    = color.New(color.FgCyan)
	green   = color.New(color.FgGreen)
	yellow  = color.New(color.FgYellow)
	red     = color.New(color.FgRed)
	white   = color.New(color.FgWhite)
	dimItal = color.New(color.Faint, color.Italic)
)

// ── Quotes ───────────────────────────────────────────────────────────────────

const rareQuote = "You are the best hacker on the planet, I'm not even squiddin'."

var (
	normalQuotes     = []string{"Welcome aboard Captain, all systems online.", "Engine powering up...", "Ahead standard.", "All systems online.", "Cruising depth reached."}
	aggressiveQuotes = []string{"Ahead flank, emergency speed.", "Warning: engine overheat.", "Hull integrity compromised."}
	stealthQuotes    = []string{"Rig for silent running.", "Ahead slow."}
)

func pickQuote(mode string) string {
	if rand.Float64() < 0.01 {
		return rareQuote
	}
	switch mode {
	case "aggressive":
		return aggressiveQuotes[rand.IntN(len(aggressiveQuotes))]
	case "stealth":
		return stealthQuotes[rand.IntN(len(stealthQuotes))]
	default:
		return normalQuotes[rand.IntN(len(normalQuotes))]
	}
}

// ── Stealth User-Agents ───────────────────────────────────────────────────────

var stealthUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

// ── Flags ────────────────────────────────────────────────────────────────────

var (
	domain      = flag.String("d", "", "Target domain")
	wordlistPath = flag.String("w", "", "Subdomain wordlist path")
	threads     = flag.Int("t", 50, "Concurrent workers")
	rate        = flag.Int("r", 500, "Max requests per second")
	output      = flag.String("o", "", "Output file")
	format      = flag.String("format", "json", "Output format: json, html")
	depth       = flag.Int("depth", 2, "Crawl depth for endpoint discovery")
	passiveOnly = flag.Bool("passive-only", false, "Only use passive sources (no DNS brute force)")
	proxy       = flag.String("proxy", "", "HTTP proxy URL")
	verbose     = flag.Bool("v", false, "Verbose output")
	timeout     = flag.Int("timeout", 10, "Per-request timeout in seconds")
	scanTimeout = flag.Int("scan-timeout", 30, "Total scan timeout in minutes (0 = no limit)")
	userAgent   = flag.String("user-agent", "Cyclops/1.0", "User-Agent header")
	resolvers   = flag.String("resolvers", "", "Comma-separated list of DNS resolvers")
	mode        = flag.String("mode", "normal", "Scan mode: normal, stealth, aggressive")
	autotune    = flag.Bool("autotune", false, "Dynamically adjust request rate based on server responses")
	interactive = flag.Bool("interactive", false, "Pause between phases to review and select targets")
)

func main() {
	flag.Parse()

	// ── ASCII Art (always shown first) ───────────────────────────────────────
	cyan.Println(`   ___           _                   `)
	cyan.Println(`  / __\   _  ___| | ___  _ __  ___  `)
	cyan.Println(` / /| | | | |/ __| |/ _ \| '_ \/ __| `)
	cyan.Println(`/ /_| |_| | | (__| | (_) | |_) \__ \ `)
	cyan.Println(`\____/\__, |\___|_|\___/| .__/|___/ `)
	cyan.Println(`      |___/             |_|          `)
	fmt.Println()

	// ── Subnautica quote ──────────────────────────────────────────────────────
	dimItal.Printf("  ~ \"%s\"\n", pickQuote(*mode))
	fmt.Println()

	// ── No-args: print usage and exit cleanly ─────────────────────────────────
	if *domain == "" {
		cyan.Println("Usage:")
		white.Println("  cyclops -d <domain> [options]")
		fmt.Println()
		cyan.Println("Examples:")
		white.Println("  cyclops -d example.com")
		white.Println("  cyclops -d example.com -mode stealth -format html -o report.html")
		white.Println("  cyclops -d example.com -mode aggressive -autotune -v")
		white.Println("  cyclops -d example.com -interactive")
		fmt.Println()
		cyan.Println("Options:")
		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()
		os.Exit(0)
	}

	// ── Track which flags were set explicitly so mode defaults don't override ─
	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	// ── Apply mode defaults (explicit flags take priority) ────────────────────
	switch *mode {
	case "stealth":
		if !explicit["t"] {
			*threads = 5
		}
		if !explicit["r"] {
			*rate = 10
		}
		if !explicit["passive-only"] {
			*passiveOnly = true
		}
	case "aggressive":
		if !explicit["t"] {
			*threads = 200
		}
		if !explicit["r"] {
			*rate = 2000
		}
		if !explicit["depth"] {
			*depth = 4
		}
	}

	// ── Logging ───────────────────────────────────────────────────────────────
	logLevel := slog.LevelWarn
	if *verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(
		clearLineWriter{os.Stderr},
		&slog.HandlerOptions{Level: logLevel},
	)))

	// ── Parse resolvers ───────────────────────────────────────────────────────
	var resolverList []string
	if *resolvers != "" {
		for _, r := range strings.Split(*resolvers, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				if !strings.Contains(r, ":") {
					r += ":53"
				}
				resolverList = append(resolverList, r)
			}
		}
	}

	// ── Load wordlist ─────────────────────────────────────────────────────────
	wordlist := ""
	if *wordlistPath != "" {
		data, err := os.ReadFile(*wordlistPath)
		if err != nil {
			red.Fprintf(os.Stderr, "Error: could not read wordlist %q: %v\n", *wordlistPath, err)
			os.Exit(1)
		}
		wordlist = string(data)
	}

	// ── Autotune: shared rate limiter ─────────────────────────────────────────
	var sharedRL *utils.RateLimiter
	var autotuner *utils.Autotuner
	var reportCodeFn func(int)

	if *autotune {
		startRate := *rate
		sharedRL = utils.NewRateLimiter(startRate)
		autotuner = utils.NewAutotuner(startRate, sharedRL, *verbose)
		reportCodeFn = autotuner.ReportCode
	}

	// ── Build options ─────────────────────────────────────────────────────────
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
		Mode:        *mode,
		Autotune:    *autotune,
		Interactive: *interactive,
		ReportCode:  reportCodeFn,
	}

	if *mode == "stealth" {
		options.UserAgents = stealthUserAgents
	}
	if sharedRL != nil {
		options.RateLimiter = sharedRL
	}

	result := models.Result{
		Domain:   *domain,
		ScanTime: time.Now().UTC(),
		ScanMode: *mode,
	}

	// ── Root context ──────────────────────────────────────────────────────────
	ctx := context.Background()
	if *scanTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*scanTimeout)*time.Minute)
		defer cancel()
	}

	// Start autotune background loop.
	if autotuner != nil {
		go autotuner.Run(ctx)
		defer autotuner.Stop()
	}

	// ── Phase 1: Subdomain enumeration ────────────────────────────────────────
	s := newSpinner("Enumerating subdomains...")
	s.Start()
	subdomainsFound, err := runSubdomainEnumeration(ctx, options)
	s.Stop()

	if err != nil {
		red.Fprintf(os.Stderr, "[!] Subdomain enumeration error: %v\n", err)
	} else {
		result.Subdomains = subdomainsFound
		green.Printf("[+] Found %d subdomains\n", len(subdomainsFound))
	}

	if *verbose {
		for _, sub := range result.Subdomains {
			white.Printf("    %s\n", sub.Name)
		}
	}

	// Interactive: choose which subdomains to continue with.
	if *interactive && len(result.Subdomains) > 0 {
		if !promptContinue("host discovery") {
			outputResults(result, options)
			return
		}
		result.Subdomains = interactiveSelectSubdomains(result.Subdomains)
	}

	// ── Phase 2: Host discovery ───────────────────────────────────────────────
	s = newSpinner("Probing hosts...")
	s.Start()
	hostsFound, err := runHostDiscovery(ctx, options, result.Subdomains)
	s.Stop()

	if err != nil {
		red.Fprintf(os.Stderr, "[!] Host discovery error: %v\n", err)
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
		green.Printf("[+] Found %d live hosts\n", len(hostsFound))
	}

	if *verbose {
		for _, h := range hostsFound {
			white.Printf("    %s [%d]\n", h.URL, h.StatusCode)
		}
	}

	// Interactive: choose which hosts to crawl.
	if *interactive && len(hostsFound) > 0 {
		if !promptContinue("endpoint crawling") {
			outputResults(result, options)
			return
		}
		hostsFound = interactiveSelectHosts(hostsFound)
	}

	// ── Phase 3: Endpoint discovery ───────────────────────────────────────────
	s = newSpinner("Crawling endpoints...")
	s.Start()
	endpointsFound, err := runEndpointDiscovery(ctx, options, hostsFound, s)
	s.Stop()

	if err != nil {
		red.Fprintf(os.Stderr, "[!] Endpoint discovery error: %v\n", err)
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
		green.Printf("[+] Found %d endpoints\n", len(endpointsFound))
	}

	// ── Output ────────────────────────────────────────────────────────────────
	cyan.Println("[*] Generating output...")
	if err := outputResults(result, options); err != nil {
		red.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}

	green.Println("[*] Scan complete.")
}

// ── Spinner helper ────────────────────────────────────────────────────────────

func newSpinner(label string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = "  " + label
	s.Color("cyan")
	return s
}

// ── Scan phases ───────────────────────────────────────────────────────────────

func runSubdomainEnumeration(ctx context.Context, options models.ScanOptions) ([]models.Subdomain, error) {
	if options.Wordlist == "" && !options.PassiveOnly {
		options.Wordlist = strings.Join(getDefaultWordlist(), "\n")
	}
	engine := subdomains.NewDiscoveryEngine(options)
	names, err := engine.Discover(ctx, options.Domain)
	if err != nil {
		return nil, err
	}
	result := make([]models.Subdomain, 0, len(names))
	for _, name := range names {
		result = append(result, models.Subdomain{
			Name:    name,
			Sources: []string{"mixed"},
		})
	}
	return result, nil
}

func runHostDiscovery(ctx context.Context, options models.ScanOptions, subds []models.Subdomain) ([]models.Host, error) {
	probe := hosts.NewHTTPProbeFromOptions(options)

	var urlsToProbe []string
	for _, sub := range subds {
		urlsToProbe = append(urlsToProbe, "http://"+sub.Name)
		urlsToProbe = append(urlsToProbe, "https://"+sub.Name)
	}

	results, err := probe.ProbeHosts(ctx, urlsToProbe, options.Threads)
	if err != nil {
		return nil, err
	}

	return hosts.FingerprintHosts(results), nil
}

func runEndpointDiscovery(ctx context.Context, options models.ScanOptions, liveHosts []models.Host, s *spinner.Spinner) ([]models.Endpoint, error) {
	var allEndpoints []models.Endpoint

	for _, host := range liveHosts {
		s.Suffix = fmt.Sprintf("  Crawling endpoints... (%s)", host.URL)

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

// ── Output ────────────────────────────────────────────────────────────────────

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
		white.Printf("[*] Results written to %s\n", options.Output)
	} else {
		if err := formatter.WriteToStdout(result); err != nil {
			return fmt.Errorf("failed to write output to stdout: %w", err)
		}
	}

	return nil
}

// ── Interactive mode ──────────────────────────────────────────────────────────

var stdinReader = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(line)
}

// promptContinue asks "Continue to [phase]? [Y/n]:" and returns true to continue.
func promptContinue(nextPhase string) bool {
	ans := readLine(fmt.Sprintf("Continue to %s? [Y/n]: ", nextPhase))
	ans = strings.ToLower(ans)
	return ans == "" || ans == "y" || ans == "yes"
}

// promptAllOrSelect shows the (a)/(s) sub-menu and returns the choice.
func promptAllOrSelect() byte {
	fmt.Println("  (a) Scan all")
	fmt.Println("  (s) Select from list")
	ans := strings.ToLower(readLine("> "))
	if strings.HasPrefix(ans, "s") {
		return 's'
	}
	return 'a'
}

// parseRangeSelection parses "1,3,5-9" into 0-indexed positions.
func parseRangeSelection(input string, max int) []int {
	seen := make(map[int]bool)
	var indices []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			halves := strings.SplitN(part, "-", 2)
			start, errA := strconv.Atoi(strings.TrimSpace(halves[0]))
			end, errB := strconv.Atoi(strings.TrimSpace(halves[1]))
			if errA != nil || errB != nil {
				continue
			}
			for i := start; i <= end; i++ {
				if i >= 1 && i <= max && !seen[i] {
					indices = append(indices, i-1)
					seen[i] = true
				}
			}
		} else {
			i, err := strconv.Atoi(part)
			if err == nil && i >= 1 && i <= max && !seen[i] {
				indices = append(indices, i-1)
				seen[i] = true
			}
		}
	}
	return indices
}

func interactiveSelectSubdomains(subs []models.Subdomain) []models.Subdomain {
	if promptAllOrSelect() == 'a' {
		return subs
	}
	for i, s := range subs {
		white.Printf("  %3d) %s\n", i+1, s.Name)
	}
	ans := readLine("Select (e.g. 1,3,5-9): ")
	indices := parseRangeSelection(ans, len(subs))
	if len(indices) == 0 {
		return subs
	}
	selected := make([]models.Subdomain, 0, len(indices))
	for _, idx := range indices {
		selected = append(selected, subs[idx])
	}
	return selected
}

func interactiveSelectHosts(liveHosts []models.Host) []models.Host {
	if promptAllOrSelect() == 'a' {
		return liveHosts
	}
	for i, h := range liveHosts {
		white.Printf("  %3d) %s [%d]\n", i+1, h.URL, h.StatusCode)
	}
	ans := readLine("Select (e.g. 1,3,5-9): ")
	indices := parseRangeSelection(ans, len(liveHosts))
	if len(indices) == 0 {
		return liveHosts
	}
	selected := make([]models.Host, 0, len(indices))
	for _, idx := range indices {
		selected = append(selected, liveHosts[idx])
	}
	return selected
}

// ── Default wordlist ──────────────────────────────────────────────────────────

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
