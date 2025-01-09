package helper

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
)

func MarshalIndent(v any) (string, error) {
	d := &bytes.Buffer{}
	e := json.NewEncoder(d)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	err := e.Encode(v)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(strings.ReplaceAll(d.String(), `\n`, "\n"), `\"`, `"`), nil
}

func PrettyPrint(t interface{}) {
	o, err := MarshalIndent(t)
	if err != nil {
		log.Println("Error:", err.Error())
		return
	}
	log.Println(string(o))
}

func FindBetween(s string, start string, end string) string {
	a := 0
	if start != "" {
		a = strings.Index(s, start)
		if a == -1 {
			return ""
		} else {
			a += len(start)
		}
	}
	if end == "" {
		return s[a:]
	}
	b := strings.Index(s[a:], end)
	if b == -1 {
		return s[a:]
	}
	return s[a : a+b]
}

func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
