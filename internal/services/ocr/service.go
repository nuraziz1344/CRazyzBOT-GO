package ocr

import (
	"os/exec"
	"strings"
)

func Recognize(imagePath string) (string, error) {
	// tesseract <image> stdout
	cmd := exec.Command("tesseract", imagePath, "stdout")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
