package gocto

import (
	"fmt"
	"github.com/andersfylling/disgord"
)

// Embed ...
type Embed struct {
	*disgord.Embed
}

const (
	EmbedLimitTitle       = 256
	EmbedLimitDescription = 2048
	EmbedLimitFieldValue  = 1024
	EmbedLimitFieldName   = 256
	EmbedLimitField       = 25
	EmbedLimitFooter      = 2048
	EmbedLimit            = 4000
)

func NewEmbed() *Embed {
	return &Embed{&disgord.Embed{}}
}

func (e *Embed) Build() *disgord.Embed {
	return e.Embed
}

func (e *Embed) SetTitle(name string) *Embed {
	e.Title = name
	return e
}

func (e *Embed) SetDescription(description string) *Embed {
	if len(description) > 2048 {
		description = description[:2048]
	}
	e.Description = description
	return e
}

func (e *Embed) AddField(name, value string, args ...interface{}) *Embed {
	if len(value) > 1024 {
		value = value[:1024]
	}

	if len(name) > 1024 {
		name = name[:1024]
	}

	if len(args) > 0 {
		value = fmt.Sprintf(value, args...)
	}

	e.Fields = append(e.Fields, &disgord.EmbedField{
		Name:  name,
		Value: value,
	})

	return e
}

func (e *Embed) AddInlineField(name, value string, args ...interface{}) *Embed {
	if len(value) > 1024 {
		value = value[:1024]
	}

	if len(name) > 1024 {
		name = name[:1024]
	}

	if len(args) > 0 {
		value = fmt.Sprintf(value, args...)
	}

	e.Fields = append(e.Fields, &disgord.EmbedField{
		Name:   name,
		Value:  value,
		Inline: true,
	})

	return e
}

func (e *Embed) SetFooter(args ...string) *Embed {
	iconURL := ""
	text := ""
	proxyURL := ""

	switch {
	case len(args) > 2:
		proxyURL = args[2]
		fallthrough
	case len(args) > 1:
		iconURL = args[1]
		fallthrough
	case len(args) > 0:
		text = args[0]
	case len(args) == 0:
		return e
	}

	e.Footer = &disgord.EmbedFooter{
		IconURL:      iconURL,
		Text:         text,
		ProxyIconURL: proxyURL,
	}
	return e
}

func (e *Embed) SetImage(args ...string) *Embed {
	var URL string
	var proxyURL string

	if len(args) == 0 {
		return e
	}

	if len(args) > 0 {
		URL = args[0]
	}

	if len(args) > 1 {
		proxyURL = args[1]
	}

	e.Image = &disgord.EmbedImage{
		URL:      URL,
		ProxyURL: proxyURL,
	}
	return e
}

func (e *Embed) SetThumbnail(args ...string) *Embed {
	var URL string
	var proxyURL string

	if len(args) == 0 {
		return e
	}

	if len(args) > 0 {
		URL = args[0]
	}

	if len(args) > 1 {
		proxyURL = args[1]
	}

	e.Thumbnail = &disgord.EmbedThumbnail{
		URL:      URL,
		ProxyURL: proxyURL,
	}
	return e
}

func (e *Embed) SetAuthor(args ...string) *Embed {
	var (
		name     string
		iconURL  string
		URL      string
		proxyURL string
	)

	if len(args) == 0 {
		return e
	}

	if len(args) > 0 {
		name = args[0]
	}

	if len(args) > 1 {
		iconURL = args[1]
	}

	if len(args) > 2 {
		URL = args[2]
	}

	if len(args) > 3 {
		proxyURL = args[3]
	}

	e.Author = &disgord.EmbedAuthor{
		Name:         name,
		IconURL:      iconURL,
		URL:          URL,
		ProxyIconURL: proxyURL,
	}

	return e
}

func (e *Embed) SetURL(URL string) *Embed {
	e.URL = URL
	return e
}

func (e *Embed) SetColor(clr int) *Embed {
	e.Color = clr
	return e
}

func (e *Embed) InlineAllFields() *Embed {
	for _, v := range e.Fields {
		v.Inline = true
	}
	return e
}

func (e *Embed) Truncate() *Embed {
	e.TruncateDescription()
	e.TruncateFields()
	e.TruncateFooter()
	e.TruncateTitle()
	return e
}

func (e *Embed) TruncateFields() *Embed {
	if len(e.Fields) > 25 {
		e.Fields = e.Fields[:EmbedLimitField]
	}

	for _, v := range e.Fields {

		if len(v.Name) > EmbedLimitFieldName {
			v.Name = v.Name[:EmbedLimitFieldName]
		}

		if len(v.Value) > EmbedLimitFieldValue {
			v.Value = v.Value[:EmbedLimitFieldValue]
		}

	}
	return e
}

func (e *Embed) TruncateDescription() *Embed {
	if len(e.Description) > EmbedLimitDescription {
		e.Description = e.Description[:EmbedLimitDescription]
	}
	return e
}

func (e *Embed) TruncateTitle() *Embed {
	if len(e.Title) > EmbedLimitTitle {
		e.Title = e.Title[:EmbedLimitTitle]
	}
	return e
}

func (e *Embed) TruncateFooter() *Embed {
	if e.Footer != nil && len(e.Footer.Text) > EmbedLimitFooter {
		e.Footer.Text = e.Footer.Text[:EmbedLimitFooter]
	}
	return e
}
