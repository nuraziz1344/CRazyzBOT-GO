package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"
)

// Proxy represents a single proxy entry
type Proxy struct {
	Address string // host:port
	Country string // ISO country code (might be empty)
	Source  string // where it was fetched from
}

// Manager manages a pool of free proxies with periodic refresh
type Manager struct {
	mu          sync.RWMutex
	pool        []Proxy
	lastRefresh time.Time

	refreshInterval  time.Duration
	testTimeout      time.Duration
	sources          []Source
	preferredCountry string // ISO code to prefer, e.g. "ID" for Indonesia
	store            storage.ProxyStore
}

// Source defines a proxy list source
type Source struct {
	Name    string
	URL     string
	Country string                     // ISO country code when the source is country-filtered
	Parser  func(body string) []string // returns []"host:port"
}

// NewManager creates a new proxy manager
func NewManager(preferredCountry string, refreshInterval time.Duration) *Manager {
	if refreshInterval <= 0 {
		refreshInterval = 30 * time.Minute
	}

	m := &Manager{
		refreshInterval:  refreshInterval,
		testTimeout:      5 * time.Second,
		preferredCountry: strings.ToUpper(preferredCountry),
	}

	m.sources = []Source{
		{
			Name:    "ditatompel-indonesia",
			URL:     "https://www.ditatompel.com/proxy/country/id",
			Country: "ID",
			Parser:  parseDitatompelHTML,
		},
	}

	return m
}

// SetStore enables persistence for the proxy pool. Call it before Start.
func (m *Manager) SetStore(store storage.ProxyStore) {
	m.store = store
}

// Start begins periodic proxy refresh in a background goroutine
func (m *Manager) Start(ctx context.Context) {
	if m.store != nil {
		proxies, err := m.store.LoadProxyPool(ctx)
		if err != nil {
			logutil.Warn(ctx, "Proxy: failed to load persisted pool", "error", err)
		} else if len(proxies) > 0 {
			m.mu.Lock()
			m.pool = make([]Proxy, 0, len(proxies))
			for _, p := range proxies {
				m.pool = append(m.pool, Proxy{Address: p.Address, Country: p.Country, Source: p.Source})
			}
			m.lastRefresh = proxies[0].LastSeen
			m.mu.Unlock()
			logutil.Info(ctx, "Proxy: loaded persisted pool", "count", len(proxies))
		}
	}

	// Do an immediate first refresh
	m.Refresh(ctx)

	go func() {
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.Refresh(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Refresh fetches proxy lists from all sources and updates the pool
func (m *Manager) Refresh(ctx context.Context) {
	logutil.Info(ctx, "Proxy: refreshing proxy list from sources", "sources", len(m.sources))

	var allProxies []Proxy
	for _, src := range m.sources {
		proxies, err := m.fetchSource(ctx, src)
		if err != nil {
			logutil.Warn(ctx, "Proxy: source fetch failed", "source", src.Name, "error", err)
			continue
		}
		for _, addr := range proxies {
			allProxies = append(allProxies, Proxy{
				Address: addr,
				Source:  src.Name,
				Country: src.Country,
			})
		}
	}

	if len(allProxies) == 0 {
		logutil.Warn(ctx, "Proxy: no proxies fetched from any source, keeping old pool")
		return
	}

	logutil.Info(ctx, "Proxy: fetched total candidates", "count", len(allProxies))

	// Deduplicate by address
	seen := make(map[string]bool)
	unique := make([]Proxy, 0, len(allProxies))
	for _, p := range allProxies {
		if !seen[p.Address] {
			seen[p.Address] = true
			unique = append(unique, p)
		}
	}

	m.mu.Lock()
	m.pool = unique
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	if m.store != nil {
		records := make([]storage.ProxyRecord, 0, len(unique))
		for _, p := range unique {
			records = append(records, storage.ProxyRecord{
				Address: p.Address,
				Country: p.Country,
				Source:  p.Source,
			})
		}
		if err := m.store.ReplaceProxyPool(ctx, records); err != nil {
			logutil.Warn(ctx, "Proxy: failed to persist pool", "error", err)
		}
	}

	logutil.Info(ctx, "Proxy: pool updated", "unique", len(unique), "sources_ok", len(allProxies) > 0)
}

// GetRandom returns a random proxy from the pool
func (m *Manager) GetRandom() *Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.pool) == 0 {
		return nil
	}

	return &m.pool[rand.Intn(len(m.pool))]
}

// GetRandomCountry returns a random proxy, optionally preferring the configured country
// If preferCountry is true and we have proxies from that country, returns one of those
func (m *Manager) GetRandomCountry(preferCountry bool) *Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.pool) == 0 {
		return nil
	}

	if preferCountry && m.preferredCountry != "" {
		// Try to find proxies from preferred country
		var preferred []Proxy
		// Prefer source metadata, then fall back to the address-level heuristic
		for _, p := range m.pool {
			if p.Country == m.preferredCountry || (p.Country == "" && isIndonesianIP(p.Address)) {
				preferred = append(preferred, p)
			}
		}
		if len(preferred) > 0 {
			return &preferred[rand.Intn(len(preferred))]
		}
	}

	return &m.pool[rand.Intn(len(m.pool))]
}

// HTTPTransport creates an http.RoundTripper that routes through a random proxy
// Falls back to direct connection if no proxies available
func (m *Manager) HTTPTransport() http.RoundTripper {
	proxy := m.GetRandomCountry(m.preferredCountry != "")
	if proxy == nil {
		return &http.Transport{}
	}

	proxyURL, err := url.Parse("http://" + proxy.Address)
	if err != nil {
		return &http.Transport{}
	}

	return &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
}

// NewHTTPClient creates a net/http.Client with proxy transport, falling back to direct if pool empty
func (m *Manager) NewHTTPClient(timeout time.Duration) *http.Client {
	proxy := m.GetRandomCountry(m.preferredCountry != "")
	if proxy == nil {
		return &http.Client{Timeout: timeout}
	}

	proxyURL, err := url.Parse("http://" + proxy.Address)
	if err != nil {
		return &http.Client{Timeout: timeout}
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
}

// NewHTTPTransport creates an http.RoundTripper proxied via a random proxy from the pool.
func (m *Manager) NewHTTPTransport() http.RoundTripper {
	return m.HTTPTransport()
}

// TestProxy checks if a proxy is working by making a request to a known endpoint
func (m *Manager) TestProxy(addr string) bool {
	proxyURL, err := url.Parse("http://" + addr)
	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: m.testTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	req, err := http.NewRequest("GET", "http://httpbin.org/ip", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// PreferredCountry returns the configured preferred country code
func (m *Manager) PreferredCountry() string {
	return m.preferredCountry
}

// PoolSize returns the current number of proxies in the pool
func (m *Manager) PoolSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pool)
}

// PoolStats returns a summary of the proxy pool
func (m *Manager) PoolStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sourceCount := make(map[string]int)
	for _, p := range m.pool {
		sourceCount[p.Source]++
	}

	return map[string]interface{}{
		"total":        len(m.pool),
		"by_source":    sourceCount,
		"last_refresh": m.lastRefresh.Format(time.RFC3339),
	}
}

func (m *Manager) fetchSource(ctx context.Context, src Source) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", src.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, src.Name)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 2<<20))
	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return src.Parser(body.String()), nil
}

var hostPortPattern = regexp.MustCompile(`(?m)(?:^|[^0-9])((?:[0-9]{1,3}\.){3}[0-9]{1,3}):([0-9]{1,5})(?:[^0-9]|$)`)

// parseDitatompelHTML extracts host:port values from the public Indonesia
// table. The page renders each record over several lines, so parsing the
// address pattern is more stable than depending on incidental HTML tags.
func parseDitatompelHTML(body string) []string {
	matches := hostPortPattern.FindAllStringSubmatchIndex(body, -1)
	proxies := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for i, match := range matches {
		// The manager supports HTTP proxies. Ditatompel mixes SOCKS records
		// into the same country table, so discard those records here.
		recordEnd := len(body)
		if i+1 < len(matches) {
			recordEnd = matches[i+1][0]
		}
		if strings.Contains(strings.ToUpper(body[match[1+1]:recordEnd]), "SOCKS") {
			continue
		}

		ip := net.ParseIP(body[match[2]:match[3]])
		port, err := strconv.Atoi(body[match[4]:match[5]])
		if ip == nil || err != nil || port < 1 || port > 65535 {
			continue
		}
		address := net.JoinHostPort(ip.String(), strconv.Itoa(port))
		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}
		proxies = append(proxies, address)
	}
	return proxies
}

// isIndonesianIP performs a rough check if an IP might be Indonesian
// based on known Indonesian IP ranges
func isIndonesianIP(addr string) bool {
	host := addr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	// Known Indonesian IP blocks (major ranges assigned to Indonesia)
	indonesianRanges := []struct {
		start int
		end   int
	}{
		{36, 36},
		{103, 103},
		{110, 111},
		{112, 119},
		{124, 125},
		{180, 181},
		{182, 185},
		{202, 203},
		{210, 211},
		{222, 223},
	}

	firstOctet := 0
	if _, err := fmt.Sscanf(host, "%d", &firstOctet); err != nil {
		return false
	}

	for _, r := range indonesianRanges {
		if firstOctet >= r.start && firstOctet <= r.end {
			return true
		}
	}
	return false
}
