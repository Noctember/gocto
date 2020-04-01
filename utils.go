package gocto

import (
	"regexp"
)

var escapeReg = regexp.MustCompile("@(everyone|here)")

func Escape(input string) string {
	return escapeReg.ReplaceAllString(input, "@\u200b$1")
}
