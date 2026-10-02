package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	appdb "github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/labstack/echo/v4"
)

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
