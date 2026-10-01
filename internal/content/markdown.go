package content

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/a-h/templ"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

func RenderMarkdown(source string) (templ.Component, error) {
	var rendered bytes.Buffer
	parser := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	if err := parser.Convert([]byte(source), &rendered); err != nil {
		return nil, fmt.Errorf("convert Markdown: %w", err)
	}

	safeHTML := bluemonday.UGCPolicy().SanitizeBytes(rendered.Bytes())
	return templ.Raw(string(safeHTML)), nil
}

func ReadTime(source string) int {
	words := len(strings.Fields(source))
	minutes := (words + 199) / 200
	if minutes < 1 {
		return 1
	}
	return minutes
}
