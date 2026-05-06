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
			"-filter_complex",
			"color=color=black@0.0,format=yuva420p,scale=512:512[bg];" +
				"[bg]drawbox=x=0:y=0:w=512:h=512:color=pink@0.5[out];" +
				"[out][0:v]overlay=x=100000:y=100000:shortest=1,fps=fps=15[base];" +
				"[0:v]scale=512:512:force_original_aspect_ratio=decrease,fps=fps=15[ov];" +
				"[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=512:h=512[out];" +
				"[out]trim=start=0:end=10",
			"-quality", "80",
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
			"-filter_complex",
			"color=color=black@0.0,format=yuva420p,scale=512:512[bg];" +
				"[bg]drawbox=x=0:y=0:w=512:h=512:color=pink@0.5[out];" +
				"[out][0:v]overlay=x=100000:y=100000:shortest=1[base];" +
				"[0:v]scale=512:512:force_original_aspect_ratio=decrease[ov];" +
				"[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=512:h=512",
			"-quality", "90",
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

	if isAnimated {
		return []string{
			"-y", "-i", input,
			"-c:v", "libwebp",
			"-b:v", strconv.Itoa(bitrate),
			"-filter_complex",
			"color=color=black@0.0,format=yuva420p,scale=512:512[bg];" +
				"[bg]drawbox=x=0:y=0:w=512:h=512:color=pink@0.5[out];" +
				"[out][0:v]overlay=x=100000:y=100000:shortest=1,fps=fps=10[base];" + // Lower FPS for smaller size
				"[0:v]scale=512:512:force_original_aspect_ratio=decrease,fps=fps=10[ov];" +
				"[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=512:h=512[out];" +
				"[out]trim=start=0:end=10",
			"-quality", "75", // Lower quality for smaller size
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
			"-b:v", strconv.Itoa(bitrate),
			"-filter_complex",
			"color=color=black@0.0,format=yuva420p,scale=512:512[bg];" +
				"[bg]drawbox=x=0:y=0:w=512:h=512:color=pink@0.5[out];" +
				"[out][0:v]overlay=x=100000:y=100000:shortest=1[base];" +
				"[0:v]scale=512:512:force_original_aspect_ratio=decrease[ov];" +
				"[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=512:h=512",
			"-quality", "85",
			"-compression_level", "6",
			"-v", "error",
			output,
		}
	}
}
