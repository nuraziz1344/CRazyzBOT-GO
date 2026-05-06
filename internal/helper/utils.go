package helper

import (
	"fmt"
	"strings"
	"time"
)

func Temp(ext string) string {
	timestamp := time.Now().UnixNano()
	return "data/tmp/" + fmt.Sprint(timestamp) + ext
}

func TrimString(s string) string {
	return strings.Trim(strings.Trim(s, "\n"), " ")
}
