package helper

func GenerateFfmpegArgs(input, output string, isAnimated bool) []string {
	if isAnimated {
		return []string{"-y", "-i", input, "-c:v", "libwebp", "-filter_complex", "color=color=black@0.0,format=yuva420p,scale=600:600[bg];[bg]drawbox=x=0:y=0:w=600:h=600:color=pink@0.5[out];[out][0:v]overlay=x=100000:y=100000:shortest=1,fps=fps=15[base];[0:v]scale=600:600:force_original_aspect_ratio=decrease,fps=fps=15[ov];[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=600:h=600[out];[out]trim=start=0:end=10", "-v", "error", output}
	} else {
		return []string{"-y", "-i", input, "-c:v", "libwebp", "-filter_complex", "color=color=black@0.0,format=yuva420p,scale=600:600[bg];[bg]drawbox=x=0:y=0:w=600:h=600:color=pink@0.5[out];[out][0:v]overlay=x=100000:y=100000:shortest=1[base];[0:v]scale=600:600:force_original_aspect_ratio=decrease[ov];[base][ov]overlay=(W-w)/2:(H-h)/2,crop=w=600:h=600", "-v", "error", output}
	}
}
