package bot

import (
	"strings"
)

func GetSenderNumber(sender string) string {
	if strings.Contains(sender, ":") {
		return strings.Split(sender, ":")[0]
	} else {
		return strings.Split(sender, "@")[0]
	}
}
