package main

import (
	"encoding/xml"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	for _, path := range []string{"/", "/dispatches", "/game-room", "/game-room/asteroid-attack", "/projects", "/about", "/contact"} {
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

func TestAdminPagesAreNoindexWithoutCanonicalOrOpenGraph(t *testing.T) {
	client, _ := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}

	for _, path := range []string{"/admin/login", "/admin/dispatches", "/admin/highlights", "/admin/inquiries"} {
		t.Run(path, func(t *testing.T) {
			response := client.get(path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			body := response.Body.String()
			if !strings.Contains(body, `<meta name="robots" content="noindex, nofollow">`) {
				t.Error("admin page missing noindex robots meta")
			}
			if strings.Contains(body, `rel="canonical"`) || strings.Contains(body, `property="og:`) {
				t.Error("admin page included canonical or Open Graph metadata")
			}
		})
	}
}

func TestRenderedStaticAssetURLsAreVersioned(t *testing.T) {
	server, database := testServer(t)
	seedHomeTestData(t, database)
	paths := []string{"/", "/game-room", "/about", "/dispatches/devlog-entry"}
	assetPattern := regexp.MustCompile(`(?:src|href|srcset)="([^"]*/static/[^"]+)"`)
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := request(t, server, path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			matches := assetPattern.FindAllStringSubmatch(response.Body.String(), -1)
			if len(matches) == 0 {
				t.Fatalf("page %q did not render static assets", path)
			}
			for _, match := range matches {
				assetURL, err := url.Parse(html.UnescapeString(match[1]))
				if err != nil {
					t.Errorf("parse static URL %q: %v", match[1], err)
					continue
				}
				if assetURL.Query().Get("v") == "" {
					t.Errorf("static asset URL is missing a content version: %q", match[1])
				}
			}
			if strings.Contains(response.Body.String(), `src="https://unpkg.com/htmx.org`) {
				t.Error("page loaded HTMX from the public CDN")
			}
		})
	}
}

func TestCacheControlForStaticUploadsAndAdmin(t *testing.T) {
	uploadDir := t.TempDir()
	staticDir := gameAssetStaticDir(t)
	if err := os.WriteFile(filepath.Join(uploadDir, "cache-test.txt"), []byte("upload"), 0600); err != nil {
		t.Fatalf("write upload fixture: %v", err)
	}
	server, _ := testServer(t, serverOptions{UploadDir: uploadDir, StaticDir: staticDir})
	tests := []struct {
		path      string
		want      string
		immutable bool
	}{
		{path: "/static/css/site.css", want: "public, max-age=31536000"},
		{path: "/static/css/site.css?v=abc123", want: "public, max-age=31536000, immutable", immutable: true},
		{path: "/static/games/asteroid-attack/index.wasm", want: "no-cache"},
		{path: "/static/games/asteroid-attack/index.pck", want: "no-cache"},
		{path: "/static/games/asteroid-attack/index.js", want: "no-cache"},
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
			if got := strings.Contains(rec.Header().Get("Cache-Control"), "immutable"); got != tc.immutable {
				t.Errorf("immutable = %v, want %v", got, tc.immutable)
			}
		})
	}
}

func TestVaryCookieIsNotDuplicated(t *testing.T) {
	server, _ := testServer(t)
	for _, path := range []string{"/", "/missing"} {
		record := request(t, server, path)
		cookieCount := 0
		for _, value := range record.Header().Values("Vary") {
			for _, token := range strings.Split(value, ",") {
				if strings.EqualFold(strings.TrimSpace(token), "Cookie") {
					cookieCount++
				}
			}
		}
		if cookieCount != 1 {
			t.Errorf("Vary Cookie count on %s = %d, want exactly 1; Vary=%q", path, cookieCount, record.Header().Values("Vary"))
		}
	}
}

func request(t *testing.T, server http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}
