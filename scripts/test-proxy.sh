#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "============================================"
echo "  CRazyzBOT Proxy Crawler Test"
echo "============================================"
echo ""

# Load .env if available
if [ -f .env ]; then
	echo "📄 Loading .env..."
	export $(grep -v '^#' .env | xargs)
fi

COUNTRY="${PROXY_COUNTRY:-ID}"
echo "🌍 Country preference: $COUNTRY"
echo ""

echo "🏗️  Building proxytest..."
go build -o /tmp/proxytest ./cmd/proxytest/ 2>&1
echo "✅ Build OK"
echo ""

echo "🚀 Running proxy crawler..."
echo "   (This fetches the Indonesian source + tests a few proxies)"
echo "   (May take 30-60s depending on proxy responsiveness)"
echo ""
/tmp/proxytest

echo ""
echo "============================================"
echo "  Done! To enable proxy in the bot:"
echo "  Set PROXY_ENABLED=true in your .env"
echo "============================================"
