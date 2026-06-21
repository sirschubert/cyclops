package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/sirschubert/cyclops/internal/checkpoint"
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
	logoOrange = color.RGB(244, 138, 22)
	logoGold   = color.RGB(245, 200, 80)

	logoBlue  = color.RGB(140, 195, 220)
	logoSteel = color.RGB(180, 218, 235)

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
	domain        = flag.String("d", "", "Target domain")
	wordlistPath  = flag.String("w", "", "Subdomain wordlist path")
	threads       = flag.Int("t", 50, "Concurrent workers")
	rate          = flag.Int("r", 500, "Max requests per second")
	output        = flag.String("o", "", "Output file")
	format        = flag.String("format", "text", "Output format: text, json, html, md")
	depth         = flag.Int("depth", 2, "Crawl depth for endpoint discovery")
	passiveOnly   = flag.Bool("passive-only", false, "Only use passive sources (no DNS brute force)")
	proxy         = flag.String("proxy", "", "HTTP proxy URL")
	verbose       = flag.Bool("v", false, "Verbose output")
	timeout       = flag.Int("timeout", 10, "Per-request timeout in seconds")
	scanTimeout   = flag.Int("scan-timeout", 30, "Total scan timeout in minutes (0 = no limit)")
	userAgent     = flag.String("user-agent", "Cyclops/1.1", "User-Agent header")
	resolvers     = flag.String("resolvers", "", "Comma-separated list of DNS resolvers")
	mode          = flag.String("mode", "normal", "Scan mode: normal, stealth, aggressive")
	autotune      = flag.Bool("autotune", false, "Dynamically adjust request rate based on server responses")
	interactive   = flag.Bool("interactive", false, "Pause between phases to review and select targets")
	extend        = flag.Bool("extend", false, "Enable directory bruteforcing after crawl")
	wordlistDir   = flag.String("wordlist-dir", "", "Custom directory wordlist path")
	insecure      = flag.Bool("insecure", false, "Disable TLS certificate verification")
	noColor       = flag.Bool("no-color", false, "Disable colored output")
	checkpointDir = flag.String("checkpoint-dir", "", "Directory for checkpoint files")
	resumeFrom    = flag.String("resume", "", "Resume scan from checkpoint file")
	blockMetadata = flag.Bool("block-metadata", false, "Block redirects to cloud metadata / link-local addresses (169.254.0.0/16)")
	silent        = flag.Bool("silent", false, "Suppress banner and progress output (only emit results)")
	showVersion   = flag.Bool("version", false, "Print version and exit")
)

// version is the current Cyclops release.
const version = "1.2.0"

// ── Active spinner tracking (for Ctrl+C handler) ─────────────────────────────

var (
	spinnerMu   sync.Mutex
	currentSpin *spinner.Spinner
)

func startSpinner(label string) *spinner.Spinner {
	s := newSpinner(label)
	spinnerMu.Lock()
	currentSpin = s
	spinnerMu.Unlock()
	s.Start()
	return s
}

func stopSpinner(s *spinner.Spinner) {
	s.Stop()
	spinnerMu.Lock()
	if currentSpin == s {
		currentSpin = nil
	}
	spinnerMu.Unlock()
}

func main() {
	flag.Parse()

	// ── Version ────────────────────────────────────────────────────────────
	if *showVersion {
		fmt.Printf("cyclops v%s\n", version)
		os.Exit(0)
	}

	// ── No-color ───────────────────────────────────────────────────────────
	color.NoColor = *noColor

	// ── Domain validation ──────────────────────────────────────────────────
	if strings.Contains(*domain, "://") || strings.Contains(*domain, "/") {
		red.Println("[!] Invalid domain: must not contain protocol or path")
		yellow.Println("[-] Example: cyclops -d example.com")
		return
	}

	// ── ASCII Art + quote (suppressed in silent mode) ────────────────────────
	if !*silent {
		logoOrange.Println(` 	  	 ██████╗██╗   ██╗ ██████╗██╗      ██████╗ ██████╗ ███████╗	`)
		logoOrange.Println(`		██╔════╝╚██╗ ██╔╝██╔════╝██║     ██╔═══██╗██╔══██╗██╔════╝  `)
		logoGold.Println(`		██║      ╚████╔╝ ██║     ██║     ██║   ██║██████╔╝███████╗	`)
		logoBlue.Println(`		██║       ╚██╔╝  ██║     ██║     ██║   ██║██╔═══╝ ╚════██║	`)
		logoSteel.Println(`		╚██████╗   ██║   ╚██████╗███████╗╚██████╔╝██║     ███████║	`)
		logoSteel.Println(`		 ╚═════╝   ╚═╝    ╚═════╝╚══════╝ ╚═════╝ ╚═╝     ╚══════╝	`)
		fmt.Println()

		dimItal.Printf("  ~ \"%s\"\n", pickQuote(*mode))
		fmt.Println()
	}

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
		white.Println("  cyclops -d example.com -format md -o report.md")
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

	// ── Infer output format from -o extension when -format not set ────────────
	if !explicit["format"] && *output != "" {
		switch strings.ToLower(filepath.Ext(*output)) {
		case ".json":
			*format = "json"
		case ".html", ".htm":
			*format = "html"
		case ".md", ".markdown":
			*format = "md"
		case ".txt", ".text":
			*format = "text"
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
		Domain:             *domain,
		Wordlist:           wordlist,
		Threads:            *threads,
		Rate:               *rate,
		Output:             *output,
		Format:             strings.ToLower(strings.TrimSpace(*format)),
		Depth:              *depth,
		PassiveOnly:        *passiveOnly,
		Proxy:              *proxy,
		Verbose:            *verbose,
		Timeout:            *timeout,
		UserAgent:          *userAgent,
		Resolvers:          resolverList,
		Mode:               *mode,
		Autotune:           *autotune,
		Interactive:        *interactive,
		ReportCode:         reportCodeFn,
		Extend:             *extend,
		DirWordlistPath:    *wordlistDir,
		InsecureSkipVerify: *insecure,
		CheckpointDir:      *checkpointDir,
		ResumeFrom:         *resumeFrom,
		BlockMetadata:      *blockMetadata,
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

	// Wrap in a cancellable context for Ctrl+C handling.
	ctx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()

	// Start autotune background loop.
	if autotuner != nil {
		go autotuner.Run(ctx)
		defer autotuner.Stop()
	}

	// ── Ctrl+C handler ────────────────────────────────────────────────────────
	var (
		interrupted atomic.Bool
		wantSave    atomic.Bool
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		for range sigChan {
			// Stop active spinner before printing.
			spinnerMu.Lock()
			sp := currentSpin
			spinnerMu.Unlock()
			if sp != nil {
				sp.Stop()
			}
			fmt.Fprint(os.Stderr, "\r\033[K")

			yellow.Fprintln(os.Stderr, "[!] Interrupt received.")
			ans := readLine("Cancel scan? [y/N]: ")
			if !isYes(ans) {
				// User wants to continue — restart spinner if still active.
				spinnerMu.Lock()
				sp2 := currentSpin
				spinnerMu.Unlock()
				if sp2 != nil && sp2 == sp {
					sp2.Start()
				}
				continue
			}

			saveAns := readLine("Save partial results found so far? [Y/n]: ")
			wantSave.Store(saveAns == "" || isYes(saveAns))

			interrupted.Store(true)
			cancelScan()
			return
		}
	}()

	// ── Resume from checkpoint ────────────────────────────────────────────────
	resumedRank := 0
	if *resumeFrom != "" {
		cp, err := checkpoint.Load(*resumeFrom)
		if err != nil {
			red.Fprintf(os.Stderr, "[!] Could not load checkpoint: %v\n", err)
			os.Exit(1)
		}
		result = cp.Result
		result.Domain = *domain
		resumedRank = checkpoint.Rank(cp.Phase)
		if !*silent {
			cyan.Printf("[*] Resuming from checkpoint (completed phase: %s)\n", cp.Phase)
		}
	}

	// saveCheckpoint persists progress after a phase when -checkpoint-dir is set.
	saveCheckpoint := func(phase string) {
		if *checkpointDir == "" {
			return
		}
		path, err := checkpoint.Save(*checkpointDir, *domain, *mode, phase, result)
		if err != nil {
			slog.Warn("failed to write checkpoint", "err", err)
			return
		}
		if !*silent {
			dimItal.Fprintf(os.Stderr, "    checkpoint saved: %s\n", path)
		}
	}

	// ── Phase 1: Subdomain enumeration ────────────────────────────────────────
	if resumedRank < checkpoint.Rank(checkpoint.PhaseSubdomains) {
		s := startSpinner("Enumerating subdomains...")
		subdomainsFound, err := runSubdomainEnumeration(ctx, options)
		stopSpinner(s)

		if err != nil {
			red.Fprintf(os.Stderr, "[!] Subdomain enumeration error: %v\n", err)
			return // Exit on error
		}

		if !*silent {
			green.Printf("[+] Found %d subdomains\n", len(subdomainsFound))
		}

		// If zero, fall back to the base domain so host discovery still has a
		// target to probe and somewhere to attach the results.
		if len(subdomainsFound) == 0 {
			if !*silent {
				yellow.Println("[-] No subdomains found via enumeration — proceeding to host discovery with base domain")
			}
			subdomainsFound = []models.Subdomain{{Name: *domain, Sources: []string{"base"}}}
		}

		if *verbose {
			for _, sub := range subdomainsFound {
				white.Printf("    %s\n", sub.Name)
			}
		}

		// Interactive: choose which subdomains to continue with.
		if *interactive && len(subdomainsFound) > 0 {
			if !promptContinue("host discovery") {
				result.Subdomains = subdomainsFound
				handleOutput(result, options, interrupted.Load(), wantSave.Load())
				return
			}
			subdomainsFound = interactiveSelectSubdomains(subdomainsFound)
		}

		result.Subdomains = subdomainsFound
		saveCheckpoint(checkpoint.PhaseSubdomains)
	} else if !*silent {
		green.Printf("[+] Loaded %d subdomains from checkpoint\n", len(result.Subdomains))
	}

	// ── Phase 2: Host discovery ───────────────────────────────────────────────
	var hostsFound []models.Host
	if resumedRank < checkpoint.Rank(checkpoint.PhaseHosts) {
		s := startSpinner("Probing hosts...")
		var err error
		hostsFound, err = runHostDiscovery(ctx, options, result.Subdomains)
		stopSpinner(s)

		if err != nil {
			red.Fprintf(os.Stderr, "[!] Host discovery error: %v\n", err)
		} else {
			for i := range result.Subdomains {
				name := strings.ToLower(result.Subdomains[i].Name)
				result.Subdomains[i].Hosts = nil
				for _, host := range hostsFound {
					if urlHost(host.URL) == name {
						result.Subdomains[i].Hosts = append(result.Subdomains[i].Hosts, host)
					}
				}
			}
			if !*silent {
				green.Printf("[+] Found %d live hosts\n", len(hostsFound))
			}
		}

		if *verbose {
			for _, h := range hostsFound {
				white.Printf("    %s [%d]\n", h.URL, h.StatusCode)
			}
		}

		// Interactive: choose which hosts to crawl.
		if *interactive && len(hostsFound) > 0 {
			if !promptContinue("endpoint crawling") {
				saveCheckpoint(checkpoint.PhaseHosts)
				handleOutput(result, options, interrupted.Load(), wantSave.Load())
				return
			}
			hostsFound = interactiveSelectHosts(hostsFound)
		}

		saveCheckpoint(checkpoint.PhaseHosts)
	} else {
		// Reconstruct the live-host list from the restored result.
		for _, sub := range result.Subdomains {
			hostsFound = append(hostsFound, sub.Hosts...)
		}
		if !*silent {
			green.Printf("[+] Loaded %d live hosts from checkpoint\n", len(hostsFound))
		}
	}

	// ── Phase 3: Endpoint discovery ───────────────────────────────────────────
	if resumedRank < checkpoint.Rank(checkpoint.PhaseEndpoints) {
		s := startSpinner("Crawling endpoints...")
		endpointsFound, err := runEndpointDiscovery(ctx, options, hostsFound, s)
		stopSpinner(s)

		if err != nil {
			red.Fprintf(os.Stderr, "[!] Endpoint discovery error: %v\n", err)
		} else {
			for i := range result.Subdomains {
				for j := range result.Subdomains[i].Hosts {
					hostURL := result.Subdomains[i].Hosts[j].URL
					result.Subdomains[i].Hosts[j].Endpoints = nil
					for _, ep := range endpointsFound {
						if sameOrigin(ep.URL, hostURL) {
							result.Subdomains[i].Hosts[j].Endpoints = append(
								result.Subdomains[i].Hosts[j].Endpoints, ep,
							)
						}
					}
				}
			}
			if !*silent {
				green.Printf("[+] Found %d endpoints\n", len(endpointsFound))
			}
		}

		saveCheckpoint(checkpoint.PhaseEndpoints)
	} else if !*silent {
		cyan.Println("[*] Endpoints already complete (from checkpoint)")
	}

	// ── Output ────────────────────────────────────────────────────────────────
	handleOutput(result, options, interrupted.Load(), wantSave.Load())
}

// handleOutput writes results or exits cleanly on cancel-without-save.
func handleOutput(result models.Result, options models.ScanOptions, interrupted, wantSave bool) {
	if interrupted && !wantSave {
		if !*silent {
			yellow.Println("[!] Scan cancelled. No results saved.")
		}
		os.Exit(0)
	}

	if !*silent {
		if interrupted {
			cyan.Println("[*] Saving partial results...")
		} else {
			cyan.Println("[*] Generating output...")
		}
	}

	if err := outputResults(result, options); err != nil {
		red.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}

	if !*silent {
		if interrupted {
			green.Println("[*] Partial results saved.")
		} else {
			green.Println("[*] Scan complete.")
		}
	}
}

// ── Spinner helper ────────────────────────────────────────────────────────────

func newSpinner(label string) *spinner.Spinner {
	w := io.Writer(os.Stderr)
	if *silent {
		w = io.Discard
	}
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(w))
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
	if len(urlsToProbe) == 0 {
		// Fallback: probe the base domain itself
		urlsToProbe = []string{"http://" + options.Domain, "https://" + options.Domain}
	}

	results, err := probe.ProbeHosts(ctx, urlsToProbe, options.Threads)
	if err != nil {
		return nil, err
	}

	return hosts.FingerprintHosts(results), nil
}

func runEndpointDiscovery(ctx context.Context, options models.ScanOptions, liveHosts []models.Host, s *spinner.Spinner) ([]models.Endpoint, error) {
	if len(liveHosts) == 0 {
		return nil, nil
	}

	// Share a single rate limiter across every host so that crawling hosts in
	// parallel doesn't multiply the effective request rate past -r. When
	// autotune is on, options.RateLimiter is already set and we reuse it.
	if options.RateLimiter == nil {
		options.RateLimiter = utils.NewRateLimiter(options.Rate)
	}

	// Load the directory wordlist once, up front, rather than per host.
	var dirWordlist []string
	if options.Extend && options.DirWordlistPath != "" {
		if data, err := os.ReadFile(options.DirWordlistPath); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					dirWordlist = append(dirWordlist, line)
				}
			}
		} else {
			slog.Warn("could not read directory wordlist", "path", options.DirWordlistPath, "err", err)
		}
	}

	// Crawl hosts concurrently. Stealth mode stays single-host to keep the scan
	// quiet; otherwise fan out up to -t hosts at a time.
	hostConcurrency := options.Threads
	if options.Mode == "stealth" || hostConcurrency < 1 {
		hostConcurrency = 1
	}
	if hostConcurrency > len(liveHosts) {
		hostConcurrency = len(liveHosts)
	}

	var (
		mu           sync.Mutex
		seen         = make(map[string]bool)
		allEndpoints []models.Endpoint
		completed    int
	)

	add := func(eps []models.Endpoint) {
		mu.Lock()
		for _, ep := range eps {
			if !seen[ep.URL] {
				seen[ep.URL] = true
				allEndpoints = append(allEndpoints, ep)
			}
		}
		mu.Unlock()
	}

	jobs := make(chan models.Host)
	var wg sync.WaitGroup
	for i := 0; i < hostConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for host := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				crawler := endpoints.NewCrawler(options)
				robotsParser := endpoints.NewRobotsParser(options)

				if eps, err := crawler.Crawl(ctx, host.URL); err == nil {
					add(eps)
				}
				if eps, err := robotsParser.ParseAll(ctx, host.URL); err == nil {
					add(eps)
				}
				if options.Extend {
					wb := endpoints.NewWordlistBruteforcer(options, dirWordlist)
					if eps, err := wb.Bruteforce(ctx, host.URL); err == nil {
						add(eps)
					}
				}

				mu.Lock()
				completed++
				done := completed
				mu.Unlock()
				spinnerMu.Lock()
				if currentSpin == s {
					s.Suffix = fmt.Sprintf("  Crawling endpoints... (%d/%d hosts)", done, len(liveHosts))
				}
				spinnerMu.Unlock()
			}
		}()
	}

	for _, h := range liveHosts {
		select {
		case jobs <- h:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return allEndpoints, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()

	return allEndpoints, nil
}

// ── Output ────────────────────────────────────────────────────────────────────

func outputResults(result models.Result, options models.ScanOptions) error {
	type formatter interface {
		WriteToFile(models.Result, string) error
		WriteToStdout(models.Result) error
	}

	var f formatter
	switch options.Format {
	case "html":
		f = outputfmt.NewHTMLFormatter()
	case "json":
		f = outputfmt.NewJSONFormatter()
	case "md", "markdown":
		f = outputfmt.NewMarkdownFormatter()
	case "text", "txt", "":
		f = outputfmt.NewTextFormatter()
	default:
		yellow.Fprintf(os.Stderr, "[!] Unknown format %q — falling back to text\n", options.Format)
		f = outputfmt.NewTextFormatter()
	}

	if options.Output != "" {
		if err := f.WriteToFile(result, options.Output); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		absPath, _ := os.Getwd()
		outPath := options.Output
		if !filepath.IsAbs(outPath) && !strings.Contains(outPath, string(filepath.Separator)) {
			outPath = filepath.Join(absPath, options.Output)
		}
		if !*silent {
			white.Printf("[*] Results written to %s\n", outPath)
		}
	} else {
		if err := f.WriteToStdout(result); err != nil {
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

func isYes(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
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

// ── URL matching helpers ──────────────────────────────────────────────────────

// urlHost returns the lowercased hostname (without port) of a URL string.
func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// sameOrigin reports whether two URLs share the same scheme and host:port.
func sameOrigin(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host)
}

// ── Default wordlist ──────────────────────────────────────────────────────────

func getDefaultWordlist() []string {
	words := []string{
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

	// Dedupe while preserving order — the literal list above has a few repeats
	// (e.g. "admin") that would otherwise waste brute-force requests.
	seen := make(map[string]bool, len(words))
	unique := words[:0]
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			unique = append(unique, w)
		}
	}
	return unique
}
