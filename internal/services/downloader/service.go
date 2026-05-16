package downloader

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"crazyzbot-go/internal/services"
)

const (
	maxDownloadSize = 50 * 1024 * 1024
	userAgent       = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36"
)

type Metadata struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	Uploader string  `json:"uploader"`
	Webpage  string  `json:"webpage_url"`
	Filename string  `json:"_filename"`
}

type Service struct {
	httpClient *http.Client
}

func NewService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Search uses yt-dlp to search for videos
func (s *Service) Search(ctx context.Context, query string, limit int) ([]Metadata, error) {
	// yt-dlp "ytsearchN:query" --dump-json --flat-playlist
	cmd := exec.CommandContext(ctx, "yt-dlp", fmt.Sprintf("ytsearch%d:%s", limit, query), "--dump-json", "--flat-playlist")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var results []Metadata
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var meta Metadata
		if err := json.Unmarshal(scanner.Bytes(), &meta); err == nil {
			// Construct webpage URL if missing (flat-playlist sometimes omits it or it's just ID)
			if meta.Webpage == "" && meta.ID != "" {
				meta.Webpage = "https://youtu.be/" + meta.ID
			}
			results = append(results, meta)
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *Service) Resolve(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	provider, err := detectProvider(rawURL)
	if err != nil {
		return nil, err
	}

	switch provider {
	case "tiktok":
		return s.downloadTikTok(ctx, rawURL)
	case "instagram":
		return s.downloadInstagram(ctx, rawURL)
	case "facebook":
		return s.downloadFacebook(ctx, rawURL)
	case "twitter":
		return s.downloadTwitter(ctx, rawURL)
	case "youtube":
		return s.downloadYouTubeV2(ctx, rawURL)
	default:
		return nil, fmt.Errorf("unsupported downloader URL")
	}
}

func (s *Service) Download(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	return s.Resolve(ctx, rawURL)
}

func detectProvider(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid URL")
	}

	host := strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
	switch {
	case strings.Contains(host, "tiktok.com"):
		return "tiktok", nil
	case strings.Contains(host, "instagram.com"):
		return "instagram", nil
	case strings.Contains(host, "facebook.com") || strings.Contains(host, "fb.watch") || strings.Contains(host, "m.facebook.com"):
		return "facebook", nil
	case strings.Contains(host, "twitter.com") || strings.Contains(host, "x.com") || strings.Contains(host, "t.co"):
		return "twitter", nil
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return "youtube", nil
	default:
		return "", fmt.Errorf("unsupported downloader URL")
	}
}

func (s *Service) downloadTikTok(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	body, headers, err := s.doRequest(ctx, http.MethodGet, "https://musicaldown.com", nil, map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to open MusicalDown: %w", err)
	}

	fields := extractInputs(string(body))
	if len(fields) == 0 {
		return nil, errors.New("failed to prepare TikTok request")
	}
	first := true
	for name := range fields {
		if first {
			fields[name] = rawURL
			first = false
		}
	}

	cookie := firstCookie(headers.Get("set-cookie"))
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	postBody := strings.NewReader(values.Encode())
	resultBody, _, err := s.doRequest(ctx, http.MethodPost, "https://musicaldown.com/download", postBody, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Cookie":       cookie,
		"Origin":       "https://musidown.com/download",
		"Referer":      "https://musidown.com/download/en",
	})
	if err != nil {
		return nil, fmt.Errorf("TikTok scraper failed: %w", err)
	}

	content := string(resultBody)
	images := extractTikTokSlideImages(content)
	if len(images) > 0 {
		return s.resultsFromURLs(ctx, "tiktok", "image", images, "TikTok slideshow", "")
	}

	links := extractLinks(content)
	videoSD := ""
	videoHD := ""
	musicURL := ""
	for _, link := range links {
		if link.Href == "" || link.Href == "#modal2" {
			continue
		}
		dataEvent := strings.ToLower(attrValue(link.Attrs, "data-event"))
		switch {
		case videoHD == "" && strings.Contains(dataEvent, "hd"):
			videoHD = link.Href
		case videoSD == "" && strings.Contains(dataEvent, "mp4"):
			videoSD = link.Href
		case musicURL == "" && strings.Contains(link.Href, "type=mp3"):
			musicURL = link.Href
		}
	}
	videoURL := videoSD
	if videoURL == "" {
		videoURL = videoHD
	}

	title := textBetween(content, `<p class="video-desc">`, `</p>`)
	if title == "" {
		title = "TikTok video"
	}
	if videoURL != "" {
		return s.resultsFromURLs(ctx, "tiktok", "video", []string{html.UnescapeString(videoURL)}, title, title)
	}
	if musicURL != "" {
		return s.resultsFromURLs(ctx, "tiktok", "audio", []string{html.UnescapeString(musicURL)}, title, sanitizeFilename(title)+".mp3")
	}

	return nil, errors.New("no TikTok media found")
}

func (s *Service) downloadInstagram(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	body, err := s.postSnapSave(ctx, "https://snapsave.app/id/action.php?lang=id", "https://snapsave.app", "https://snapsave.app/id", rawURL)
	if err != nil {
		return nil, fmt.Errorf("Instagram scraper failed: %w", err)
	}
	media := parseInstagramMedia(body)
	if len(media) == 0 {
		return nil, errors.New("no Instagram media found")
	}
	return s.resultsFromMedia(ctx, "instagram", media, "Instagram media")
}

func (s *Service) downloadFacebook(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	body, err := s.postSnapSave(ctx, "https://snapsave.app/action.php?lang=en", "https://snapsave.app", "https://snapsave.app/id/facebook-reels-download", rawURL)
	if err != nil {
		return nil, fmt.Errorf("Facebook scraper failed: %w", err)
	}
	media := parseFacebookMedia(body)
	if len(media) == 0 {
		return nil, errors.New("no Facebook media found")
	}
	return s.resultsFromMedia(ctx, "facebook", media, "Facebook video")
}

func (s *Service) downloadTwitter(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	base := "https://twitterdownloader.snapsave.app"
	body, _, err := s.doRequest(ctx, http.MethodGet, base, nil, map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Twitter token: %w", err)
	}
	token := extractInputValue(string(body), "token")
	if token == "" {
		return nil, errors.New("failed to get Twitter token")
	}

	values := url.Values{"url": {rawURL}, "token": {token}}
	resBody, _, err := s.doRequest(ctx, http.MethodPost, base+"/action.php", strings.NewReader(values.Encode()), map[string]string{
		"Accept":           "*/*",
		"Content-Type":     "application/x-www-form-urlencoded",
		"Origin":           base,
		"Referer":          base + "/",
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return nil, fmt.Errorf("Twitter scraper failed: %w", err)
	}

	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(resBody, &payload); err != nil || payload.Data == "" {
		return nil, errors.New("invalid Twitter scraper response")
	}

	media := parseTwitterMedia(payload.Data)
	if len(media) == 0 {
		return nil, errors.New("no Twitter media found")
	}
	return s.resultsFromMedia(ctx, "twitter", media, "Twitter media")
}

func (s *Service) downloadYouTubeV2(ctx context.Context, rawURL string) ([]*services.DownloadResult, error) {
	videoID := extractYouTubeVideoID(rawURL)
	if videoID == "" {
		return nil, errors.New("invalid YouTube URL")
	}

	infoReq := map[string]string{"videoId": videoID}
	infoBody, _ := json.Marshal(infoReq)
	body, _, err := s.doRequest(ctx, http.MethodPost, "https://embed.dlsrv.online/api/info", bytes.NewReader(infoBody), map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"Referer":      "https://embed.dlsrv.online/v1/full?videoId=" + videoID,
	})
	if err != nil {
		return nil, fmt.Errorf("YouTube info failed: %w", err)
	}

	var info struct {
		Status string `json:"status"`
		Info   struct {
			Title   string `json:"title"`
			Formats []struct {
				Type     string      `json:"type"`
				Quality  string      `json:"quality"`
				Format   string      `json:"format"`
				FileSize interface{} `json:"fileSize"`
			} `json:"formats"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &info); err != nil || info.Status != "info" {
		return nil, errors.New("invalid YouTube info response")
	}

	quality := pickYouTubeQuality(info.Info.Formats)
	if quality == "" {
		quality = "360"
	}

	downloadReq := map[string]string{"videoId": videoID, "format": "mp4", "quality": quality}
	downloadBody, _ := json.Marshal(downloadReq)
	resolved, err := s.resolveDlsrv(ctx, "https://embed.dlsrv.online/api/download/mp4", downloadBody, videoID)
	mimeType := "video/mp4"
	mediaType := "video"
	if err != nil || resolved.URL == "" {
		audioReq := map[string]string{"videoId": videoID, "format": "mp3", "quality": "128"}
		audioBody, _ := json.Marshal(audioReq)
		resolved, err = s.resolveDlsrv(ctx, "https://embed.dlsrv.online/api/download/mp3", audioBody, videoID)
		mimeType = "audio/mpeg"
		mediaType = "audio"
	}
	if err != nil {
		return nil, fmt.Errorf("YouTube download link failed: %w", err)
	}

	data, contentType, err := s.fetchBytes(ctx, resolved.URL)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		mimeType = contentType
	}
	title := info.Info.Title
	if title == "" {
		title = "YouTube video"
	}
	filename := resolved.Filename
	if filename == "" {
		filename = sanitizeFilename(title)
		if mediaType == "audio" {
			filename += ".mp3"
		} else {
			filename += ".mp4"
		}
	}

	return []*services.DownloadResult{{
		Type:     mediaType,
		URL:      resolved.URL,
		Title:    title,
		Caption:  title,
		MimeType: mimeType,
		Filename: filename,
		Provider: "youtube",
		Buffer:   data,
	}}, nil
}

type dlsrvResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

func (s *Service) resolveDlsrv(ctx context.Context, endpoint string, body []byte, videoID string) (*dlsrvResponse, error) {
	resBody, _, err := s.doRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body), map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"Referer":      "https://embed.dlsrv.online/v1/full?videoId=" + videoID,
	})
	if err != nil {
		return nil, err
	}
	var resolved dlsrvResponse
	if err := json.Unmarshal(resBody, &resolved); err != nil {
		return nil, err
	}
	if resolved.URL == "" {
		return nil, fmt.Errorf("empty download URL")
	}
	return &resolved, nil
}

func (s *Service) postSnapSave(ctx context.Context, endpoint string, origin string, referer string, rawURL string) (string, error) {
	values := url.Values{"url": {rawURL}}
	body, _, err := s.doRequest(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()), map[string]string{
		"Accept":           "*/*",
		"Content-Type":     "application/x-www-form-urlencoded",
		"Origin":           origin,
		"Referer":          referer,
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return "", err
	}
	decoded := decryptSnapSave(string(body))
	if decoded == "" {
		return "", errors.New("failed to decode SnapSave response")
	}
	return decoded, nil
}

func (s *Service) resultsFromMedia(ctx context.Context, provider string, media []mediaItem, defaultTitle string) ([]*services.DownloadResult, error) {
	results := make([]*services.DownloadResult, 0, len(media))
	var lastErr error
	for _, item := range media {
		data, contentType, err := s.fetchBytes(ctx, item.URL)
		if err != nil {
			lastErr = err
			continue
		}
		if !matchesMediaType(item.Type, contentType) {
			lastErr = fmt.Errorf("download URL returned %s", contentType)
			continue
		}
		mimeType := item.MimeType
		if mimeType == "" {
			mimeType = contentType
		}
		if mimeType == "" {
			mimeType = mimeFromType(item.Type)
		}
		caption := ""
		if len(results) == 0 {
			caption = defaultTitle
		}
		results = append(results, &services.DownloadResult{
			Type:     item.Type,
			URL:      item.URL,
			Title:    defaultTitle,
			Caption:  caption,
			MimeType: mimeType,
			Filename: item.Filename,
			Provider: provider,
			Buffer:   data,
		})
	}
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return results, nil
}

func (s *Service) resultsFromURLs(ctx context.Context, provider string, mediaType string, urls []string, title string, filename string) ([]*services.DownloadResult, error) {
	media := make([]mediaItem, 0, len(urls))
	for _, itemURL := range urls {
		media = append(media, mediaItem{Type: mediaType, URL: absolutizeURL(itemURL, "https://musicaldown.com"), Filename: filename})
	}
	return s.resultsFromMedia(ctx, provider, media, title)
}

func (s *Service) fetchBytes(ctx context.Context, mediaURL string) ([]byte, string, error) {
	body, headers, err := s.doRequest(ctx, http.MethodGet, mediaURL, nil, map[string]string{})
	if err != nil {
		return nil, "", err
	}
	contentType := headers.Get("content-type")
	if contentType != "" {
		contentType, _, _ = mime.ParseMediaType(contentType)
	}
	return body, contentType, nil
}

func (s *Service) doRequest(ctx context.Context, method string, rawURL string, body io.Reader, headers map[string]string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.Header, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxDownloadSize {
		return nil, res.Header, fmt.Errorf("media too large to send via WhatsApp")
	}
	limited := io.LimitReader(res.Body, maxDownloadSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, res.Header, err
	}
	if len(data) > maxDownloadSize {
		return nil, res.Header, fmt.Errorf("media too large to send via WhatsApp")
	}
	return data, res.Header, nil
}

type mediaItem struct {
	Type     string
	URL      string
	MimeType string
	Filename string
}

type anchor struct {
	Href  string
	Text  string
	Attrs string
}

func parseInstagramMedia(content string) []mediaItem {
	if items := parseInstagramDownloadItems(content); len(items) > 0 {
		return items
	}
	if items := parseInstagramTable(content); len(items) > 0 {
		return items
	}
	if items := parseInstagramCards(content); len(items) > 0 {
		return items
	}
	return parseInstagramSingleItem(content)
}

func parseInstagramDownloadItems(content string) []mediaItem {
	itemRe := regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*download-items[^"']*["'][^>]*>(.*?)</div>\s*</div>`)
	items := []mediaItem{}
	for _, match := range itemRe.FindAllStringSubmatch(content, -1) {
		block := match[1]
		link := firstHref(block)
		thumb := firstImageSrc(block)
		spanText := strings.ToLower(stripTags(block))
		if strings.Contains(block, "<video") || strings.Contains(spanText, "video") || strings.Contains(strings.ToLower(link), ".mp4") {
			if link != "" {
				return []mediaItem{{Type: "video", URL: normalizeSnapSaveURL(link)}}
			}
			continue
		}
		imageURL := thumb
		if imageURL == "" {
			imageURL = link
		}
		if imageURL != "" && !strings.Contains(strings.ToLower(imageURL), ".mp4") {
			items = append(items, mediaItem{Type: "image", URL: normalizeSnapSaveURL(imageURL)})
		}
	}
	return dedupeMedia(items)
}

func parseInstagramTable(content string) []mediaItem {
	if !strings.Contains(content, "table") {
		return nil
	}
	if progressURL := firstProgressAPI(content); progressURL != "" {
		return []mediaItem{{Type: "video", URL: normalizeSnapSaveURL(progressURL)}}
	}
	for _, link := range extractLinks(content) {
		href := normalizeSnapSaveURL(link.Href)
		if href != "" && (strings.Contains(strings.ToLower(href), ".mp4") || strings.Contains(strings.ToLower(link.Text), "download")) {
			return []mediaItem{{Type: "video", URL: href}}
		}
	}
	return nil
}

func parseInstagramCards(content string) []mediaItem {
	cardRe := regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*card[^"']*["'][^>]*>(.*?)</div>\s*</div>`)
	for _, match := range cardRe.FindAllStringSubmatch(content, -1) {
		block := match[1]
		href := normalizeSnapSaveURL(firstHref(block))
		if href == "" {
			continue
		}
		if strings.Contains(strings.ToLower(stripTags(block)), "photo") || looksLikeImageURL(href) {
			return []mediaItem{{Type: "image", URL: href}}
		}
		return []mediaItem{{Type: "video", URL: href}}
	}
	return nil
}

func parseInstagramSingleItem(content string) []mediaItem {
	if progressURL := firstProgressAPI(content); progressURL != "" {
		return []mediaItem{{Type: "video", URL: normalizeSnapSaveURL(progressURL)}}
	}
	for _, link := range extractLinks(content) {
		href := normalizeSnapSaveURL(link.Href)
		if href == "" || strings.HasPrefix(href, "#") || !isLikelyMediaDownload(href, link.Text, link.Attrs) {
			continue
		}
		if strings.Contains(strings.ToLower(link.Text), "photo") || looksLikeImageURL(href) {
			return []mediaItem{{Type: "image", URL: href}}
		}
		return []mediaItem{{Type: "video", URL: href}}
	}
	return nil
}

func parseFacebookMedia(content string) []mediaItem {
	if items := parseFacebookDownloadItems(content); len(items) > 0 {
		return items
	}
	if items := parseFacebookTable(content); len(items) > 0 {
		return items
	}
	if items := parseFacebookCards(content); len(items) > 0 {
		return items
	}
	return parseFacebookSingleItem(content)
}

func parseFacebookDownloadItems(content string) []mediaItem {
	itemRe := regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*download-items[^"']*["'][^>]*>(.*?)</div>\s*</div>`)
	for _, match := range itemRe.FindAllStringSubmatch(content, -1) {
		href := normalizeSnapSaveURL(firstHref(match[1]))
		if href != "" {
			return []mediaItem{{Type: "video", URL: href}}
		}
	}
	return nil
}

func parseFacebookTable(content string) []mediaItem {
	if !strings.Contains(content, "table") {
		return nil
	}
	items := []mediaItem{}
	for _, progressURL := range allProgressAPI(content) {
		items = append(items, mediaItem{Type: "video", URL: normalizeSnapSaveURL(progressURL)})
	}
	for _, link := range extractLinks(content) {
		href := normalizeSnapSaveURL(link.Href)
		if href != "" && isLikelyMediaDownload(href, link.Text, link.Attrs) {
			items = append(items, mediaItem{Type: "video", URL: href})
		}
	}
	return dedupeMedia(items)
}

func parseFacebookCards(content string) []mediaItem {
	cardRe := regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*card[^"']*["'][^>]*>(.*?)</div>\s*</div>`)
	for _, match := range cardRe.FindAllStringSubmatch(content, -1) {
		href := normalizeSnapSaveURL(firstHref(match[1]))
		if href != "" {
			return []mediaItem{{Type: "video", URL: href}}
		}
	}
	return nil
}

func parseFacebookSingleItem(content string) []mediaItem {
	if progressURL := firstProgressAPI(content); progressURL != "" {
		return []mediaItem{{Type: "video", URL: normalizeSnapSaveURL(progressURL)}}
	}
	for _, link := range extractLinks(content) {
		href := normalizeSnapSaveURL(link.Href)
		if href == "" || strings.HasPrefix(href, "#") || !isLikelyMediaDownload(href, link.Text, link.Attrs) {
			continue
		}
		return []mediaItem{{Type: "video", URL: href}}
	}
	return nil
}

func parseTwitterMedia(content string) []mediaItem {
	links := extractLinks(content)
	for _, link := range links {
		href := normalizeSnapSaveURL(link.Href)
		if href == "" {
			continue
		}
		lower := strings.ToLower(link.Text + " " + link.Attrs + " " + href)
		if strings.Contains(lower, "photo") || looksLikeImageURL(href) {
			return []mediaItem{{Type: "image", URL: href}}
		}
		return []mediaItem{{Type: "video", URL: href}}
	}
	return nil
}

func attrValue(attrs string, name string) string {
	re := regexp.MustCompile(`(?is)` + regexp.QuoteMeta(name) + `=["']([^"']+)["']`)
	match := re.FindStringSubmatch(attrs)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

func firstHref(content string) string {
	re := regexp.MustCompile(`(?is)<a\b[^>]*href=["']([^"']+)["']`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

func firstImageSrc(content string) string {
	re := regexp.MustCompile(`(?is)<img\b[^>]*src=["']([^"']+)["']`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

func firstProgressAPI(content string) string {
	urls := allProgressAPI(content)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func allProgressAPI(content string) []string {
	re := regexp.MustCompile(`(?is)get_progressApi\(["']([^"']+)["']\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	urls := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		progressURL := "https://snapsave.app" + html.UnescapeString(match[1])
		if !seen[progressURL] {
			seen[progressURL] = true
			urls = append(urls, progressURL)
		}
	}
	return urls
}

func stripTags(content string) string {
	re := regexp.MustCompile(`(?is)<[^>]+>`)
	return strings.TrimSpace(html.UnescapeString(re.ReplaceAllString(content, " ")))
}

func isLikelyMediaDownload(href string, text string, attrs string) bool {
	lower := strings.ToLower(href + " " + text + " " + attrs)
	return strings.Contains(lower, ".mp4") || strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") || strings.Contains(lower, ".png") || strings.Contains(lower, ".webp") || strings.Contains(lower, "download") || strings.Contains(lower, "get_progressapi")
}

func extractLinks(content string) []anchor {
	re := regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	hrefRe := regexp.MustCompile(`(?is)href=["']([^"']+)["']`)
	onclickRe := regexp.MustCompile(`(?is)get_progressApi\(["']([^"']+)["']\)`)
	stripRe := regexp.MustCompile(`(?is)<[^>]+>`)
	matches := re.FindAllStringSubmatch(content, -1)
	links := make([]anchor, 0, len(matches))
	for _, match := range matches {
		attrs := match[1]
		href := ""
		if hrefMatch := hrefRe.FindStringSubmatch(attrs); len(hrefMatch) > 1 {
			href = html.UnescapeString(hrefMatch[1])
		}
		if href == "" {
			if onclickMatch := onclickRe.FindStringSubmatch(attrs); len(onclickMatch) > 1 {
				href = "https://snapsave.app" + onclickMatch[1]
			}
		}
		text := strings.TrimSpace(html.UnescapeString(stripRe.ReplaceAllString(match[2], " ")))
		links = append(links, anchor{Href: href, Text: text, Attrs: attrs})
	}
	return links
}

func decryptSnapSave(data string) string {
	params := getEncodedParams(data)
	if len(params) < 6 {
		return ""
	}
	decoded := decodeSnapApp(params)
	if decoded == "" {
		return ""
	}
	return getDecodedSnapSave(decoded)
}

func getEncodedParams(data string) []string {
	parts := strings.Split(data, "decodeURIComponent(escape(r))}(")
	if len(parts) < 2 {
		return nil
	}
	paramString := strings.Split(parts[1], "))")[0]
	params := strings.Split(paramString, ",")
	for i := range params {
		params[i] = strings.Trim(strings.ReplaceAll(params[i], "\"", ""), " '")
	}
	return params
}

func decodeSnapApp(args []string) string {
	t := args[0]
	o := args[2]
	b, _ := strconv.Atoi(args[3])
	z, _ := strconv.Atoi(args[4])
	if z <= 0 || z > len("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ+/ ") || len(o) <= z {
		return ""
	}
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ+/"
	tArr := strings.Split(alphabet[:z], "")
	var result strings.Builder
	for i := 0; i < len(t); {
		s := ""
		for i < len(t) && t[i] != o[z] {
			s += string(t[i])
			i++
		}
		i++
		for j := 0; j < len(o); j++ {
			s = strings.ReplaceAll(s, string(o[j]), strconv.Itoa(j))
		}
		decoded := decodeBase(s, z, tArr)
		charCode := decoded - b
		if charCode > 0 {
			result.WriteRune(rune(charCode))
		}
	}
	return result.String()
}

func decodeBase(input string, base int, alphabet []string) int {
	total := 0
	runes := []rune(input)
	for c := 0; c < len(runes); c++ {
		char := string(runes[len(runes)-1-c])
		idx := -1
		for i, item := range alphabet {
			if item == char {
				idx = i
				break
			}
		}
		if idx != -1 {
			total += idx * int(math.Pow(float64(base), float64(c)))
		}
	}
	return total
}

func getDecodedSnapSave(data string) string {
	parts := strings.Split(data, `getElementById("download-section").innerHTML = "`)
	if len(parts) < 2 {
		parts = strings.Split(data, `getElementById("download-section").innerHTML="`)
	}
	if len(parts) < 2 {
		return ""
	}
	htmlPart := strings.Split(parts[1], `"; document.getElementById("inputData").remove();`)[0]
	htmlPart = strings.ReplaceAll(htmlPart, `\`, ``)
	htmlPart = strings.ReplaceAll(htmlPart, `\/`, `/`)
	htmlPart = strings.ReplaceAll(htmlPart, `\"`, `"`)
	return html.UnescapeString(htmlPart)
}

func extractInputs(content string) map[string]string {
	inputRe := regexp.MustCompile(`(?is)<input\b([^>]*)>`)
	nameRe := regexp.MustCompile(`(?is)name=["']([^"']+)["']`)
	valueRe := regexp.MustCompile(`(?is)value=["']([^"']*)["']`)
	fields := map[string]string{}
	for _, match := range inputRe.FindAllStringSubmatch(content, -1) {
		attrs := match[1]
		nameMatch := nameRe.FindStringSubmatch(attrs)
		if len(nameMatch) < 2 {
			continue
		}
		value := ""
		if valueMatch := valueRe.FindStringSubmatch(attrs); len(valueMatch) > 1 {
			value = html.UnescapeString(valueMatch[1])
		}
		fields[nameMatch[1]] = value
	}
	return fields
}

func extractInputValue(content string, name string) string {
	inputRe := regexp.MustCompile(`(?is)<input\b([^>]*)>`)
	valueRe := regexp.MustCompile(`(?is)value=["']([^"']*)["']`)
	for _, match := range inputRe.FindAllStringSubmatch(content, -1) {
		attrs := match[1]
		if strings.Contains(attrs, `name="`+name+`"`) || strings.Contains(attrs, `name='`+name+`'`) {
			if valueMatch := valueRe.FindStringSubmatch(attrs); len(valueMatch) > 1 {
				return html.UnescapeString(valueMatch[1])
			}
		}
	}
	return ""
}

func extractYouTubeVideoID(rawURL string) string {
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`,
		`youtube\.com/embed/([a-zA-Z0-9_-]{11})`,
		`youtube\.com/v/([a-zA-Z0-9_-]{11})`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(rawURL); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func pickYouTubeQuality(formats []struct {
	Type     string      `json:"type"`
	Quality  string      `json:"quality"`
	Format   string      `json:"format"`
	FileSize interface{} `json:"fileSize"`
}) string {
	qualities := make([]int, 0)
	for _, format := range formats {
		if format.Type != "video" || strings.ToLower(format.Format) != "mp4" {
			continue
		}
		size := int64FromAny(format.FileSize)
		if size > maxDownloadSize {
			continue
		}
		quality := strings.TrimSuffix(format.Quality, "p")
		value, err := strconv.Atoi(quality)
		if err == nil {
			qualities = append(qualities, value)
		}
	}
	if len(qualities) == 0 {
		return ""
	}
	sort.Ints(qualities)
	preferred := []int{480, 360, 720}
	for _, target := range preferred {
		for _, quality := range qualities {
			if quality == target {
				return strconv.Itoa(quality)
			}
		}
	}
	return strconv.Itoa(qualities[0])
}

func int64FromAny(value interface{}) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	default:
		return 0
	}
}

func normalizeSnapSaveURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" || raw == "#" || strings.HasPrefix(raw, "javascript:") {
		return ""
	}
	return absolutizeURL(raw, "https://snapsave.app")
}

func absolutizeURL(raw string, base string) string {
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		return base + raw
	}
	return raw
}

func looksLikeImageURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg") || strings.Contains(lower, ".png") || strings.Contains(lower, ".webp")
}

func mimeFromType(mediaType string) string {
	switch mediaType {
	case "image":
		return "image/jpeg"
	case "audio":
		return "audio/mpeg"
	case "video":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

func matchesMediaType(mediaType string, contentType string) bool {
	if contentType == "" || contentType == "application/octet-stream" {
		return true
	}
	switch mediaType {
	case "image":
		return strings.HasPrefix(contentType, "image/")
	case "audio":
		return strings.HasPrefix(contentType, "audio/")
	case "video":
		return strings.HasPrefix(contentType, "video/")
	default:
		return true
	}
}

func dedupeMedia(items []mediaItem) []mediaItem {
	seen := map[string]bool{}
	result := make([]mediaItem, 0, len(items))
	for _, item := range items {
		if item.URL == "" || seen[item.URL] {
			continue
		}
		seen[item.URL] = true
		result = append(result, item)
	}
	return result
}

func extractTikTokSlideImages(content string) []string {
	containerRe := regexp.MustCompile(`(?is)<div[^>]+class=["']col\s+s12\s+m3["'][^>]*>(.*?)</div>`)
	imageRe := regexp.MustCompile(`(?is)<img[^>]+src=["']([^"']+)["']`)
	seen := map[string]bool{}
	images := []string{}
	for _, container := range containerRe.FindAllStringSubmatch(content, -1) {
		if len(container) < 2 {
			continue
		}
		match := imageRe.FindStringSubmatch(container[1])
		if len(match) < 2 {
			continue
		}
		imageURL := html.UnescapeString(match[1])
		if imageURL != "" && strings.HasPrefix(imageURL, "http") && !seen[imageURL] {
			seen[imageURL] = true
			images = append(images, imageURL)
		}
	}
	return images
}

func uniqueStrings(matches [][]string, group int) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, match := range matches {
		if len(match) <= group {
			continue
		}
		value := html.UnescapeString(match[group])
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func firstCookie(value string) string {
	if value == "" {
		return ""
	}
	return strings.Split(value, ";")[0]
}

func textBetween(content string, start string, end string) string {
	parts := strings.Split(content, start)
	if len(parts) < 2 {
		return ""
	}
	value := strings.Split(parts[1], end)[0]
	return strings.TrimSpace(html.UnescapeString(regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(value, "")))
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "download"
	}
	re := regexp.MustCompile(`[\\/:*?"<>|]+`)
	value = re.ReplaceAllString(value, "_")
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}
