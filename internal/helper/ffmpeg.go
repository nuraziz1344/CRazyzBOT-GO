package helper

import (
	"encoding/json"
	"os/exec"
	"strconv"
)

func GenerateFfmpegArgs(input, output string, isAnimated bool) []string {
	if isAnimated {
		return []string{
			"-y", "-i", input,
			"-c:v", "libwebp",
			"-vf",
			"fps=15,scale=512:512:force_original_aspect_ratio=decrease:flags=lanczos,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=black@0.0,trim=start=0:end=10",
			"-quality", "65",
			"-compression_level", "6",
			"-loop", "0",
			"-an",
			"-v", "error",
			output,
		}
	} else {
		return []string{
			"-y", "-i", input,
			"-c:v", "libwebp",
			"-vf",
			"scale=512:512:force_original_aspect_ratio=decrease:flags=lanczos,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=black@0.0",
			"-quality", "75",
			"-compression_level", "6",
			"-v", "error",
			output,
		}
	}
}

// GetVideoDuration returns the duration of a video file in seconds
func GetVideoDuration(input string) (float64, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		input,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// GenerateFfmpegArgsWithBitrate generates FFmpeg args with dynamic bitrate based on duration
// Target: keep file under 1MB (using 700KB as safety margin to account for encoding overhead)
func GenerateFfmpegArgsWithBitrate(input, output string, isAnimated bool, duration float64) []string {
	if !isAnimated {
		return []string{
			"-y", "-i", input,
			"-c:v", "libwebp",
			"-vf",
			"scale=512:512:force_original_aspect_ratio=decrease:flags=lanczos,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=black@0.0",
			"-quality", "70",
			"-compression_level", "6",
			"-v", "error",
			output,
		}
	}

	// Target size: 700KB (safety margin under 1MB to account for WebP overhead and metadata)
	// bitrate = (target_size * 8) / duration_in_seconds
	targetSizeKB := 700.0
	bitrate := int((targetSizeKB * 8 * 1024) / duration)

	// Minimum bitrate to maintain some quality (100k)
	if bitrate < 100000 {
		bitrate = 100000
	}
	// Maximum bitrate cap (2M)
	if bitrate > 2000000 {
		bitrate = 2000000
	}

	return []string{
		"-y", "-i", input,
		"-c:v", "libwebp",
		"-b:v", strconv.Itoa(bitrate),
		"-vf",
		"fps=10,scale=512:512:force_original_aspect_ratio=decrease:flags=lanczos,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=black@0.0,trim=start=0:end=10",
		"-quality", "60",
		"-compression_level", "6",
		"-loop", "0",
		"-an",
		"-v", "error",
		output,
	}
}
