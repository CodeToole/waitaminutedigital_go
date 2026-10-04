package main

import (
	"compress/gzip"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	appdb "github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/labstack/echo/v4"
)

func TestCanonicalHostRedirect(t *testing.T) {
	tests := []struct {
		name         string
		production   bool
		path         string
		host         string
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "production redirects and preserves path and query",
			production:   true,
			path:         "/dispatches/a%2Fb?category=devlog&page=2",
			host:         "www.waitaminutedigital.com",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "https://waitaminutedigital.com/dispatches/a%2Fb?category=devlog&page=2",
		},
		{
			name:       "health check is exempt",
			production: true,
			path:       "/health",
			host:       "www.waitaminutedigital.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "development does not redirect",
			path:       "/projects",
			host:       "www.waitaminutedigital.com",
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := testServer(t, serverOptions{Production: tc.production})
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get(echo.HeaderLocation); got != tc.wantLocation {
				t.Errorf("Location = %q, want %q", got, tc.wantLocation)
			}
		})
	}
}

func TestHTTPServerTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, 10*time.Second)
	}
	if server.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", server.ReadTimeout, 30*time.Second)
	}
	if server.WriteTimeout != 5*time.Minute {
		t.Errorf("WriteTimeout = %v, want %v", server.WriteTimeout, 5*time.Minute)
	}
	if server.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", server.IdleTimeout, 120*time.Second)
	}
}

func TestSecurityHeaders(t *testing.T) {
	server, _ := testServer(t)
	paths := []string{"/", "/admin/login", "/static/games/asteroid-attack/index.js"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			header := rec.Header()
			if got := header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := header.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
				t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
			}
			if got := header.Get("Permissions-Policy"); got != "camera=(), microphone=(), geolocation=()" {
				t.Errorf("Permissions-Policy = %q", got)
			}
			if got := header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
				t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", got)
			}
			if got := header.Get("Content-Security-Policy"); got != "" {
				t.Errorf("enforcing CSP header = %q, want empty by default", got)
			}
			csp := header.Get("Content-Security-Policy-Report-Only")
			for _, want := range []string{
				"script-src 'self' 'unsafe-inline' https://*.clarity.ms",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data: blob: https://*.clarity.ms",
				"font-src 'self'",
				"connect-src 'self' https://*.clarity.ms",
				"frame-src 'self'",
				"frame-ancestors 'self'",
			} {
				if !strings.Contains(csp, want) {
					t.Errorf("CSP %q does not contain %q", csp, want)
				}
			}
			hasWasmEval := strings.Contains(csp, "'wasm-unsafe-eval'")
			if strings.HasPrefix(path, "/static/games/") && !hasWasmEval {
				t.Errorf("game CSP %q does not contain 'wasm-unsafe-eval'", csp)
			}
			for _, directive := range []string{"worker-src 'self' blob:", "media-src 'self' blob:"} {
				hasDirective := strings.Contains(csp, directive)
				if strings.HasPrefix(path, "/static/games/") && !hasDirective {
					t.Errorf("game CSP %q does not contain %q", csp, directive)
				}
				if !strings.HasPrefix(path, "/static/games/") && hasDirective {
					t.Errorf("non-game CSP unexpectedly contains %q: %q", directive, csp)
				}
			}
			if !strings.HasPrefix(path, "/static/games/") && hasWasmEval {
				t.Errorf("non-game CSP unexpectedly contains 'wasm-unsafe-eval': %q", csp)
			}
		})
	}
}

func TestCSPEnforceUsesEnforcingHeader(t *testing.T) {
	server, _ := testServer(t, serverOptions{CSPEnforce: true})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("enforcing CSP header is empty")
	}
	if got := rec.Header().Get("Content-Security-Policy-Report-Only"); got != "" {
		t.Errorf("report-only CSP header = %q, want empty when CSP_ENFORCE is true", got)
	}
}

func TestStaticAssetsAndHomeLayout(t *testing.T) {
	server, _ := testServer(t)
	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/static/css/site.css", wantStatus: http.StatusOK},
		{path: "/static/img/logo-128.webp", wantStatus: http.StatusOK},
		{path: "/", wantStatus: http.StatusOK, wantBody: "<title>Waitaminute Digital</title>"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body does not contain %q", tc.wantBody)
			}
		})
	}
}

func TestGameAssetMIMETypesAndGzip(t *testing.T) {
	staticDir := gameAssetStaticDir(t)
	server, _ := testServer(t, serverOptions{StaticDir: staticDir})
	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/static/games/asteroid-attack/index.wasm", contentType: "application/wasm"},
		{path: "/static/games/asteroid-attack/index.pck", contentType: "application/octet-stream"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			response := request(t, server, tc.path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); got != tc.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tc.contentType)
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/static/games/asteroid-attack/index.wasm", nil)
	req.Header.Set(echo.HeaderAcceptEncoding, "gzip")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("gzip request status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderContentEncoding); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	if got, want := string(body), "wasm test fixture"; got != want {
		t.Errorf("decompressed body = %q, want %q", got, want)
	}
}

func TestAsteroidAttackExportFilesAreServed(t *testing.T) {
	server, _ := testServer(t)
	page := request(t, server, "/static/games/asteroid-attack/index.html")
	if page.Code != http.StatusOK {
		t.Fatalf("export page status = %d, want %d; body: %s", page.Code, http.StatusOK, page.Body.String())
	}
	if !strings.Contains(page.Body.String(), `"executable":"index"`) {
		t.Error("export page does not contain Godot engine configuration")
	}

	files := []string{
		"index.js",
		"index.pck",
		"index.wasm",
		"index.side.wasm",
		"index.audio.worklet.js",
		"index.audio.position.worklet.js",
		"index.icon.png",
		"index.apple-touch-icon.png",
		"index.png",
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/static/games/asteroid-attack/"+name, nil)
			req.Header.Set("Range", "bytes=0-0")
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusPartialContent {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
			}
			if got := rec.Header().Get(echo.HeaderCacheControl); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
			switch filepath.Ext(name) {
			case ".wasm":
				if got := rec.Header().Get(echo.HeaderContentType); got != "application/wasm" {
					t.Errorf("Content-Type = %q, want application/wasm", got)
				}
			case ".pck":
				if got := rec.Header().Get(echo.HeaderContentType); got != "application/octet-stream" {
					t.Errorf("Content-Type = %q, want application/octet-stream", got)
				}
			}
		})
	}
}

func gameAssetStaticDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gameDir := filepath.Join(root, "games", "asteroid-attack")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatalf("create game asset fixture directory: %v", err)
	}
	for name, content := range map[string]string{
		"index.wasm": "wasm test fixture",
		"index.pck":  "pck test fixture",
		"index.js":   "console.log('game fixture')",
	} {
		if err := os.WriteFile(filepath.Join(gameDir, name), []byte(content), 0600); err != nil {
			t.Fatalf("write %s fixture: %v", name, err)
		}
	}
	cssDir := filepath.Join(root, "css")
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		t.Fatalf("create CSS fixture directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cssDir, "site.css"), []byte("test stylesheet"), 0600); err != nil {
		t.Fatalf("write stylesheet fixture: %v", err)
	}
	return root
}

func TestHeroKicker(t *testing.T) {
	server, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<p class="kicker">GAME DEVELOPMENT · SOFTWARE DEVELOPMENT</p>`) {
		t.Errorf("hero kicker was not updated: %s", body)
	}
	if strings.Contains(body, "INDIE GAME DEV") {
		t.Errorf("old hero kicker text is still present")
	}
}

func TestHomeResponses(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		hxRequest  bool
		seed       bool
		want       []string
		wantAbsent []string
	}{
		{
			name: "empty states render",
			path: "/",
			want: []string{
				"Building games, tools, and software that solve real problems.",
				"No highlights yet. The first featured build is still loading.",
				"No dispatches yet. The first note ships with the next build.",
			},
			wantAbsent: []string{"Devlog entry", "Featured highlight"},
		},
		{
			name:       "only published items are visible",
			path:       "/",
			seed:       true,
			want:       []string{"Featured highlight", "Devlog entry", "News entry", "Game Room entry"},
			wantAbsent: []string{"Draft highlight", "Unpublished entry"},
		},
		{
			name:      "HTMX category response is only the active list fragment",
			path:      "/?category=devlog",
			hxRequest: true,
			seed:      true,
			want: []string{
				`id="latest-dispatches"`,
				`class="chip is-active"`,
				`aria-current="true"`,
				`hx-push-url="true"`,
				"Devlog entry",
			},
			wantAbsent: []string{"<head>", "<html", "News entry", "Unpublished entry"},
		},
		{
			name: "category query works without HTMX",
			path: "/?category=game-room",
			seed: true,
			want: []string{
				"<head>",
				`property="og:title"`,
				"Game Room entry",
				`href="/?category=game-room"`,
			},
			wantAbsent: []string{"Devlog entry", "News entry", "Unpublished entry"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, database := testServer(t)
			if tc.seed {
				seedHomeTestData(t, database)
			}

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.hxRequest {
				req.Header.Set("HX-Request", "true")
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}

			body := rec.Body.String()
			for _, expected := range tc.want {
				if !strings.Contains(body, expected) {
					t.Errorf("response does not contain %q", expected)
				}
			}
			for _, unexpected := range tc.wantAbsent {
				if strings.Contains(body, unexpected) {
					t.Errorf("response unexpectedly contains %q", unexpected)
				}
			}
		})
	}
}

func testServer(t *testing.T, options ...serverOptions) (*echo.Echo, *sql.DB) {
	t.Helper()
	t.Chdir(filepath.Join("..", ".."))
	database, err := appdb.Open(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	server := newServer("https://waitaminutedigital.com", database, options...)
	return server, database
}

func seedHomeTestData(t *testing.T, database *sql.DB) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{
			query: `INSERT INTO highlight (title, kicker, summary, href, image, sort_order, published) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []any{"Featured highlight", "Coming Soon", "A published highlight", "/game-room", "/static/img/logo-128.webp", 1, true},
		},
		{
			query: `INSERT INTO highlight (title, published) VALUES (?, ?)`,
			args:  []any{"Draft highlight", false},
		},
		{
			query: `INSERT INTO article (title, slug, category, summary, body_md, published, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []any{"Devlog entry", "devlog-entry", "Devlog", "A published devlog", "", true, "2026-09-29T10:00:00Z"},
		},
		{
			query: `INSERT INTO article (title, slug, category, summary, body_md, published, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []any{"Unpublished entry", "unpublished-entry", "Devlog", "A draft devlog", "", false, "2026-09-30T10:00:00Z"},
		},
		{
			query: `INSERT INTO article (title, slug, category, summary, body_md, published, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []any{"News entry", "news-entry", "News", "A published news post", "", true, "2026-09-28T10:00:00Z"},
		},
		{
			query: `INSERT INTO article (title, slug, category, summary, body_md, published, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			args:  []any{"Game Room entry", "game-room-entry", "Game Room", "A published game post", "", true, "2026-09-27T10:00:00Z"},
		},
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement.query, statement.args...); err != nil {
			t.Fatalf("seed test database: %v", err)
		}
	}
}

func TestLayoutContract(t *testing.T) {
	server, database := testServer(t)
	if _, err := database.Exec(
		`INSERT INTO article (title, slug, category, summary, body_md, published) VALUES (?, ?, ?, ?, ?, ?)`,
		"Some dispatch", "some-slug", "Devlog", "A test dispatch", "## Test body", true,
	); err != nil {
		t.Fatalf("insert layout fixture: %v", err)
	}
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "home has shared chrome and SEO metadata",
			path: "/",
			want: []string{
				">EDITORIAL<",
				`href="/dispatches"`, `href="/game-room"`, `href="/projects"`, `href="/about"`, `href="/contact"`,
				`aria-label="Toggle menu"`,
				`href="https://github.com/CodeToole" target="_blank" rel="noopener noreferrer"`,
				`href="https://www.linkedin.com/in/corneliustoole/" target="_blank" rel="noopener noreferrer"`,
				"© " + strconv.Itoa(time.Now().UTC().Year()) + " Waitaminute Digital",
				`property="og:title" content="Waitaminute Digital"`,
				`property="og:image" content="https://waitaminutedigital.com/static/img/mascot_head.webp?v=`,
				`name="twitter:card" content="summary_large_image"`,
			},
		},
		{
			name: "dispatch detail keeps Dispatches active",
			path: "/dispatches/some-slug",
			want: []string{`href="/dispatches" aria-current="page">Dispatches</a>`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			body := rec.Body.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if count := strings.Count(body, `src="/static/js/htmx.min.js?v=`); count != 1 {
				t.Errorf("self-hosted HTMX script count = %d, want 1", count)
			}
			assertOGTitleIsInHead(t, body)
		})
	}
}

func assertOGTitleIsInHead(t *testing.T, body string) {
	t.Helper()
	headEnd := strings.Index(body, "</head>")
	ogTitle := strings.Index(body, `property="og:title"`)
	if ogTitle < 0 || ogTitle > headEnd {
		t.Error("Open Graph title is not in <head>")
	}
}
