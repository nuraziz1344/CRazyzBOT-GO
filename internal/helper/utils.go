package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
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
		log.Println("PrettyPrint Error:", err.Error())
		return
	}
	log.Println(strings.Trim(o, "\n"))
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

func Temp(ext string) string {
	timestamp := time.Now().UnixNano()
	return "data/tmp/" + fmt.Sprint(timestamp) + ext
}

func TrimString(s string) string {
	return strings.Trim(strings.Trim(s, "\n"), " ")
}
