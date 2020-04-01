package gocto

import (
	"errors"
	"regexp"
	"strings"
)

type UsageTag struct {
	Name     string
	Type     string
	Rest     bool
	Required bool
}

func ParseUsage(usage string) ([]*UsageTag, error) {
	tags := make([]*UsageTag, 0)
	current := &UsageTag{Required: false, Rest: false, Type: "", Name: ""}
	typeMode := false
	for _, c := range usage {
		if c == '<' {
			current.Required = true
		} else if c == '>' {
			if strings.HasPrefix(current.Name, "@@") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "@@")
				current.Type = "member"
			} else if strings.HasPrefix(current.Name, "@") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "@")
				current.Type = "user"
			} else if strings.HasPrefix(current.Name, "#") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "#")
				current.Type = "channel"
			} else if current.Type == "" {
				current.Type = "literal"
			}
			tags = append(tags, current)
			current = &UsageTag{Required: false, Rest: false, Type: "", Name: ""}
			typeMode = false
		} else if c == '[' {
			if current.Required {
				return tags, errors.New("Cannot open an optional tag after opening a required one.")
			}
		} else if c == ']' {
			if strings.HasPrefix(current.Name, "@@") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "@@")
				current.Type = "member"
			} else if strings.HasPrefix(current.Name, "@") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "@")
				current.Type = "user"
			} else if strings.HasPrefix(current.Name, "#") && current.Type == "" {
				current.Name = strings.TrimPrefix(current.Name, "#")
				current.Type = "channel"
			} else if current.Type == "" {
				current.Type = "literal"
			}
			tags = append(tags, current)
			current = &UsageTag{Required: false, Rest: false, Type: "", Name: ""}
			typeMode = false
		} else if c == ' ' {
			continue
		} else if c == ':' {
			typeMode = true
		} else {
			if typeMode {
				current.Type += string(c)
			} else {
				current.Name += string(c)
			}
		}
	}
	for i, tag := range tags {
		if strings.HasSuffix(tag.Type, "...") {
			if i != len(tags)-1 {
				return tags, errors.New("Rest parameters can only appear last.")
			}
			tag.Type = strings.TrimSuffix(tag.Type, "...")
			tag.Rest = true
		}
	}
	return tags, nil
}

var HumanizeUsageRegex = regexp.MustCompile("(<|\\[)(\\w+):[^.]+?(\\.\\.\\.)?(>|\\])")

func HumanizeUsage(usage string) string {
	return HumanizeUsageRegex.ReplaceAllString(usage, "$1$2$3$4")
}
