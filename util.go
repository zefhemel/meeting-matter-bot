package main

import (
	"regexp"
	"strings"
)

func cleanMessage(s string) string {
	re := regexp.MustCompile(`[@#][\S]+`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

func cleanMessageHashtags(s string) string {
	re := regexp.MustCompile(`#[\S]+`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}
