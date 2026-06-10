package downloader

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"crazyzbot-go/internal/services"
)

const tool77API = "https://www.tool77.com/en/v/download/all/request"

type tool77Request struct {
	URL string `json:"url"`
}

type tool77Response struct {
	Code string         `json:"code"`
	Data *tool77Data    `json:"data"`
}

type tool77Data struct {
	Data *tool77Payload `json:"data"`
}

type tool77Payload struct {
	Title         string           `json:"title"`
	Thumbnail     string           `json:"thumbnail"`
	ChannelTitle  string           `json:"channelTitle"`
	Normals       []tool77Media    `json:"normals"`
	Videos        []tool77Media    `json:"videos"`
	Audios        []tool77Media    `json:"audios"`
}

type tool77Media struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	Extension     string `json:"extension"`
	ContentLength *int64 `json:"contentLength"`
}

// tool77Decrypt reverses and base64-decodes the obfuscated URL.
func tool77Decrypt(encrypted string) (string, error) {
	// Reverse the string
	runes := []rune(encrypted)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	reversed := string(runes)

	decoded, err := base64.StdEncoding.DecodeString(reversed)
	if err != nil {
		return "", fmt.Errorf("tool77 decrypt failed: %w", err)
	}
	return string(decoded), nil
}

// normalizeYouTubeURL converts short youtu.be links to full youtube.com format
// The Tool77 API doesn't support youtu.be short URLs.
func normalizeYouTubeURL(rawURL string) string {
	videoID := extractYouTubeVideoID(rawURL)
	if videoID == "" {
		return rawURL
	}
	if strings.Contains(rawURL, "youtu.be") {
		return "https://www.youtube.com/watch?v=" + videoID
	}
	return rawURL
}

// DownloadYouTubeViaTool77 downloads a YouTube video by scraping the Tool77 API.
func (s *Service) DownloadYouTubeViaTool77(ctx context.Context, rawURL string) (*services.DownloadResult, error) {
	videoID := extractYouTubeVideoID(rawURL)
	if videoID == "" {
		return nil, fmt.Errorf("invalid YouTube URL")
	}

	// Tool77 API doesn't support youtu.be short URLs
	apiURL := normalizeYouTubeURL(rawURL)

	body, _ := json.Marshal(tool77Request{URL: apiURL})
	respBody, _, err := s.doRequest(ctx, "POST", tool77API, bytes.NewReader(body), map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"Referer":      "https://www.tool77.com/en/v/downloader",
	})
	if err != nil {
		return nil, fmt.Errorf("tool77 request failed: %w", err)
	}

	var resp tool77Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("tool77 parse failed: %w", err)
	}

	if resp.Code != "success" || resp.Data == nil || resp.Data.Data == nil {
		return nil, fmt.Errorf("tool77 API returned error")
	}

	payload := resp.Data.Data

	// Pick the best video under 50MB (maxDownloadSize)
	chosen := pickBestTool77Video(payload)

	var downloadURL string
	var mimeType string
	var mediaType string
	var filename string

	if chosen != nil {
		decryptedURL, err := tool77Decrypt(chosen.URL)
		if err != nil {
			return nil, err
		}
		downloadURL = decryptedURL
		mediaType = "video"
		switch chosen.Extension {
		case "mp4":
			mimeType = "video/mp4"
		case "webm":
			mimeType = "video/webm"
		default:
			mimeType = "video/mp4"
		}
		filename = sanitizeFilename(payload.Title) + "." + chosen.Extension
	} else {
		// Fallback: pick the best audio under 50MB
		audio := pickBestTool77Audio(payload)
		if audio == nil {
			return nil, fmt.Errorf("no suitable media found")
		}
		decryptedURL, err := tool77Decrypt(audio.URL)
		if err != nil {
			return nil, err
		}
		downloadURL = decryptedURL
		mediaType = "audio"
		mimeType = "audio/mpeg"
		filename = sanitizeFilename(payload.Title) + ".mp3"
	}

	// Fetch the actual media bytes
	data, contentType, err := s.fetchBytes(ctx, downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if contentType != "" {
		mimeType = contentType
	}

	return &services.DownloadResult{
		Type:     mediaType,
		URL:      downloadURL,
		Title:    payload.Title,
		Caption:  payload.Title,
		MimeType: mimeType,
		Filename: filename,
		Provider: "youtube",
		Buffer:   data,
	}, nil
}

// pickBestTool77Video selects the best quality video under maxDownloadSize.
// Prefers mp4 over webm, highest resolution first.
func pickBestTool77Video(payload *tool77Payload) *tool77Media {
	candidates := make([]tool77Media, 0, len(payload.Videos)+len(payload.Normals))

	// Normals are combined streams (video+audio), prefer them
	candidates = append(candidates, payload.Normals...)
	candidates = append(candidates, payload.Videos...)

	if len(candidates) == 0 {
		return nil
	}

	// Sort: by size ascending (closest to but under limit), prefer mp4
	sort.Slice(candidates, func(i, j int) bool {
		ci := candidates[i].ContentLength
		cj := candidates[j].ContentLength
		if ci != nil && cj != nil {
			return *ci < *cj
		}
		// Items without size go last
		return ci != nil
	})

	limit := int64(maxDownloadSize)

	// Find the best: highest resolution within limit, prefer mp4
	// Strategy: pick highest quality MP4 under limit, or webm under limit
	var best *tool77Media
	var bestSize int64

	for idx := len(candidates) - 1; idx >= 0; idx-- {
		c := candidates[idx]
		if c.ContentLength == nil {
			// If no size info, try it as last resort
			if best == nil {
				best = &c
			}
			continue
		}
		size := *c.ContentLength
		if size <= limit && size > bestSize {
			best = &candidates[idx]
			bestSize = size
		}
	}

	if best == nil && len(candidates) > 0 {
		// All over limit, pick the smallest
		best = &candidates[0]
	}

	return best
}

// pickBestTool77Audio selects the best audio stream under maxDownloadSize.
func pickBestTool77Audio(payload *tool77Payload) *tool77Media {
	candidates := payload.Audios
	if len(candidates) == 0 {
		return nil
	}

	limit := int64(maxDownloadSize)
	var best *tool77Media
	var bestSize int64

	for i := range candidates {
		c := candidates[i]
		if c.ContentLength == nil {
			if best == nil {
				best = &c
			}
			continue
		}
		size := *c.ContentLength
		// Prefer larger audio (higher bitrate) under limit
		if size <= limit && size > bestSize {
			best = &candidates[i]
			bestSize = size
		}
	}

	if best == nil && len(candidates) > 0 {
		best = &candidates[len(candidates)-1]
	}

	return best
}
