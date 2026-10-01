package content

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestRenderMarkdownProducesSafeHTML(t *testing.T) {
	component, err := RenderMarkdown("## Rendered heading\n\n<script>alert('unsafe')</script>Safe paragraph.")
	if err != nil {
		t.Fatalf("RenderMarkdown() returned error: %v", err)
	}

	var output bytes.Buffer
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatalf("render templ component: %v", err)
	}
	if !strings.Contains(output.String(), "<h2>Rendered heading</h2>") {
		t.Fatalf("Markdown was escaped instead of rendered: %s", output.String())
	}
	if strings.Contains(strings.ToLower(output.String()), "<script") {
		t.Fatalf("script element survived sanitization: %s", output.String())
	}
}

func TestReadTimeUsesTwoHundredWordsPerMinute(t *testing.T) {
	wordCounts := []struct {
		words int
		want  int
	}{
		{words: 0, want: 1},
		{words: 200, want: 1},
		{words: 201, want: 2},
	}
	for _, tc := range wordCounts {
		t.Run(strconv.Itoa(tc.words)+"_words", func(t *testing.T) {
			body := strings.Repeat("word ", tc.words)
			if got := ReadTime(body); got != tc.want {
				t.Errorf("ReadTime() = %d, want %d", got, tc.want)
			}
		})
	}
}
