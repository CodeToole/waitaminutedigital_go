package main

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSitemapXMLContainsStaticAndPublishedPagesOnly(t *testing.T) {
	server, database := testServer(t)
	seedHomeTestData(t, database)

	rec := request(t, server, "/sitemap.xml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/xml") {
		t.Errorf("Content-Type = %q, want application/xml", rec.Header().Get("Content-Type"))
	}

	var document struct {
		URLs []struct {
			Location string `xml:"loc"`
			LastMod  string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("invalid sitemap XML: %v", err)
	}
	locations := make(map[string]string, len(document.URLs))
	for _, entry := range document.URLs {
		locations[entry.Location] = entry.LastMod
		if strings.Contains(entry.Location, "/admin") {
			t.Errorf("sitemap contains admin URL %q", entry.Location)
		}
	}
	for _, path := range []string{"/", "/dispatches", "/game-room", "/projects", "/about", "/contact"} {
		if _, ok := locations["https://waitaminutedigital.com"+path]; !ok {
			t.Errorf("sitemap missing static path %q", path)
		}
	}
	articleURL := "https://waitaminutedigital.com/dispatches/devlog-entry"
	if lastmod := locations[articleURL]; lastmod == "" {
		t.Errorf("published article entry %q missing lastmod", articleURL)
	} else if _, err := time.Parse(time.RFC3339, lastmod); err != nil {
		t.Errorf("article lastmod %q is invalid: %v", lastmod, err)
	}
	if _, ok := locations["https://waitaminutedigital.com/dispatches/unpublished-entry"]; ok {
		t.Error("sitemap contains an unpublished article")
	}
}

func TestRobotsListsAdminDisallowAndSitemap(t *testing.T) {
	server, _ := testServer(t)
	rec := request(t, server, "/robots.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	for _, expected := range []string{"User-agent: *", "Disallow: /admin", "Sitemap: https://waitaminutedigital.com/sitemap.xml"} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Errorf("robots.txt missing %q: %s", expected, rec.Body.String())
		}
	}
}

func TestRSSFeedIsValidAndPublishedOnly(t *testing.T) {
	server, database := testServer(t)
	seedHomeTestData(t, database)

	rec := request(t, server, "/feed.xml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/rss+xml") {
		t.Errorf("Content-Type = %q, want application/rss+xml", rec.Header().Get("Content-Type"))
	}
	var feed struct {
		Version string `xml:"version,attr"`
		Channel struct {
			Items []struct {
				Title   string `xml:"title"`
				Link    string `xml:"link"`
				PubDate string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("invalid RSS XML: %v", err)
	}
	if feed.Version != "2.0" {
		t.Errorf("RSS version = %q, want 2.0", feed.Version)
	}
	got := make(map[string]struct{}, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		got[item.Title] = struct{}{}
		if !strings.HasPrefix(item.Link, "https://waitaminutedigital.com/dispatches/") {
			t.Errorf("RSS item link is not absolute: %q", item.Link)
		}
		if item.PubDate == "" {
			t.Errorf("RSS item %q is missing pubDate", item.Title)
		}
	}
	for _, title := range []string{"Devlog entry", "News entry", "Game Room entry"} {
		if _, ok := got[title]; !ok {
			t.Errorf("RSS feed missing published article %q", title)
		}
	}
	if _, ok := got["Unpublished entry"]; ok {
		t.Error("RSS feed contains an unpublished article")
	}
}

func TestCanonicalURLsAreAbsoluteAcrossRenderedPages(t *testing.T) {
	_, database := testServer(t)
	if _, err := database.Exec(
		`INSERT INTO article (title, slug, category, summary, body_md, published, cover_image) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"SEO Article", "seo-article", "Devlog", "SEO summary", "Article content", true, "/uploads/cover.webp",
	); err != nil {
		t.Fatalf("insert article fixture: %v", err)
	}
	server := newServer("https://seo.example", database)
	tests := []struct {
		path string
		want int
	}{
		{path: "/", want: http.StatusOK},
		{path: "/dispatches", want: http.StatusOK},
		{path: "/dispatches/seo-article", want: http.StatusOK},
		{path: "/game-room", want: http.StatusOK},
		{path: "/projects", want: http.StatusOK},
		{path: "/about", want: http.StatusOK},
		{path: "/contact", want: http.StatusOK},
		{path: "/missing-page", want: http.StatusNotFound},
		{path: "/admin/login", want: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			rec := request(t, server, tc.path)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			wantCanonical := `rel="canonical" href="https://seo.example` + tc.path + `"`
			if !strings.Contains(rec.Body.String(), wantCanonical) {
				t.Errorf("absolute canonical %q missing", wantCanonical)
			}
			if tc.path == "/dispatches/seo-article" {
				for _, expected := range []string{
					`property="og:type" content="article"`,
					`property="og:image" content="https://seo.example/uploads/cover.webp"`,
					`name="twitter:card" content="summary_large_image"`,
				} {
					if !strings.Contains(rec.Body.String(), expected) {
						t.Errorf("article metadata missing %q", expected)
					}
				}
			}
		})
	}
}

func TestCacheControlForStaticUploadsAndAdmin(t *testing.T) {
	uploadDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uploadDir, "cache-test.txt"), []byte("upload"), 0600); err != nil {
		t.Fatalf("write upload fixture: %v", err)
	}
	server, _ := testServer(t, serverOptions{UploadDir: uploadDir})
	tests := []struct {
		path string
		want string
	}{
		{path: "/static/css/site.css", want: "public, max-age=31536000"},
		{path: "/uploads/cache-test.txt", want: "public, max-age=31536000"},
		{path: "/admin/login", want: "no-store"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			rec := request(t, server, tc.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("Cache-Control = %q, want %q", got, tc.want)
			}
		})
	}
}

func request(t *testing.T, server http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}
