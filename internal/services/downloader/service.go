package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"crazyzbot-go/internal/services"
)

type Metadata struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	Uploader string  `json:"uploader"`
	Webpage  string  `json:"webpage_url"`
	Filename string  `json:"_filename"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
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

// GetVideoMetadata fetches metadata for a single URL
func (s *Service) GetVideoMetadata(ctx context.Context, url string) (*Metadata, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp", url, "--dump-json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(out, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetStream returns the command to stream download.
// Caller must access StdoutPipe(), Start(), and Wait().
func (s *Service) GetStream(ctx context.Context, url string, isAudio bool) (*exec.Cmd, error) {
	args := []string{url, "-o", "-", "--quiet", "--no-warnings"}
	if isAudio {
		// Extract audio, prefer mp3
		args = append(args, "-x", "--audio-format", "mp3")
	} else {
		// Prefer mp4
		args = append(args, "-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best")
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	return cmd, nil
}

// Download performs a direct download and returns the buffer (use with caution for large files)
func (s *Service) Download(ctx context.Context, url string) ([]*services.DownloadResult, error) {
	// Fetch metadata first to get title
	meta, err := s.GetVideoMetadata(ctx, url)
	title := "Downloaded Video"
	if err == nil {
		title = meta.Title
	}

	// Download video
	// Using -S res:720 to avoid 4K downloads which might be too large
	cmd := exec.CommandContext(ctx, "yt-dlp", url, "-o", "-", "-f", "best[ext=mp4]/best", "-S", "res:720", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return []*services.DownloadResult{{
		Type:     "video",
		URL:      url,
		Title:    title,
		MimeType: "video/mp4",
		Buffer:   out,
	}}, nil
}
