package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"crazyzbot-go/internal/proxy"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	country := os.Getenv("PROXY_COUNTRY")
	if country == "" {
		country = "ID"
	}

	fmt.Printf("🚀 Starting proxy crawler (preferred country: %s)\n\n", country)

	pm := proxy.NewManager(country, 30*time.Minute)

	// Do an immediate refresh
	pm.Refresh(ctx)

	// Show pool stats
	stats := pm.PoolStats()
	fmt.Printf("📊 Pool stats:\n")
	fmt.Printf("   Total proxies: %d\n", stats["total"])
	fmt.Printf("   Last refresh:  %s\n", stats["last_refresh"])
	if bySource, ok := stats["by_source"].(map[string]int); ok {
		fmt.Printf("   By source:\n")
		for src, count := range bySource {
			fmt.Printf("     • %s: %d\n", src, count)
		}
	}
	fmt.Println()

	// Pick and show some Indonesian proxies
	fmt.Println("🔍 Sampling random proxies (Indonesian-first):")
	for i := 0; i < 10; i++ {
		p := pm.GetRandomCountry(true)
		if p == nil {
			break
		}
		fmt.Printf("   %d. %s (source: %s, ID: %v)\n", i+1, p.Address, p.Source, isIndonesianIP(p.Address))
	}

	// Test a few proxies
	fmt.Println("\n🧪 Testing proxies (3s timeout each)...")
	testCount := 0
	passCount := 0
	for i := 0; i < 10; i++ {
		p := pm.GetRandom()
		if p == nil {
			break
		}
		testCount++
		ok := pm.TestProxy(p.Address)
		if ok {
			passCount++
			fmt.Printf("   ✅ %s\n", p.Address)
		} else {
			fmt.Printf("   ❌ %s\n", p.Address)
		}
	}
	fmt.Printf("\n📈 Test results: %d/%d proxies working\n", passCount, testCount)
}

// isIndonesianIP duplicated here for standalone testing
func isIndonesianIP(addr string) bool {
	host := addr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host = addr[:i]
			break
		}
	}
	ranges := []struct{ start, end int }{
		{36, 36}, {103, 103}, {110, 111}, {112, 119},
		{124, 125}, {180, 181}, {182, 185}, {202, 203},
		{210, 211}, {222, 223},
	}
	firstOctet := 0
	if _, err := fmt.Sscanf(host, "%d", &firstOctet); err != nil {
		return false
	}
	for _, r := range ranges {
		if firstOctet >= r.start && firstOctet <= r.end {
			return true
		}
	}
	return false
}
