package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDispatchArticleResponses(t *testing.T) {
	body := "## Rendered heading\n\nVisible paragraph.\n\n<script>alert('unsafe')</script>\n\n" + strings.Repeat("word ", 400)
	tests := []struct {
		name       string
		slug       string
		published  bool
		insert     bool
		wantStatus int
		want       []string
		wantAbsent []string
	}{
		{
			name:       "published article renders sanitized markdown and sharing metadata",
			slug:       "safe-article",
			published:  true,
			insert:     true,
			wantStatus: http.StatusOK,
			want: []string{
				"<h2>Rendered heading</h2>",
				"<p>Visible paragraph.</p>",
				"3 min read",
				`property="og:type" content="article"`,
				"twitter.com/intent/tweet",
				"facebook.com/sharer/sharer.php",
				"linkedin.com/sharing/share-offsite",
				`data-copy-link="https://waitaminutedigital.com/dispatches/safe-article"`,
				"Copy Link",
			},
			wantAbsent: []string{"<script>alert('unsafe')", "alert('unsafe')"},
		},
		{
			name:       "draft article is styled as not found",
			slug:       "draft-article",
			insert:     true,
			wantStatus: http.StatusNotFound,
			want:       []string{"Page not found", `src="/static/img/mascot.webp?v=`, `href="/"`},
			wantAbsent: []string{"application/json", "draft secret"},
		},
		{
			name:       "missing article is styled as not found",
			slug:       "missing-article",
			wantStatus: http.StatusNotFound,
			want:       []string{"Page not found", `src="/static/img/mascot.webp?v=`, `href="/"`},
			wantAbsent: []string{"application/json", "{"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, database := testServer(t)
			if tc.insert {
				articleBody := body
				if !tc.published {
					articleBody = "draft secret"
				}
				if _, err := database.Exec(
					`INSERT INTO article (title, slug, category, summary, body_md, published) VALUES (?, ?, ?, ?, ?, ?)`,
					"Safe article", tc.slug, "Devlog", "A summary callout", articleBody, tc.published,
				); err != nil {
					t.Fatalf("insert article fixture: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/dispatches/"+tc.slug, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			for _, expected := range tc.want {
				if !strings.Contains(rec.Body.String(), expected) {
					t.Errorf("body does not contain %q", expected)
				}
			}
			for _, unexpected := range tc.wantAbsent {
				if strings.Contains(rec.Body.String(), unexpected) {
					t.Errorf("body unexpectedly contains %q", unexpected)
				}
			}
			if tc.wantStatus == http.StatusNotFound && rec.Header().Get("Content-Type") != "text/html; charset=UTF-8" {
				t.Errorf("Content-Type = %q, want HTML", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestDispatchPaginationAndHTMX(t *testing.T) {
	server, database := testServer(t)
	for number := 1; number <= 21; number++ {
		_, err := database.Exec(
			`INSERT INTO article (title, slug, category, summary, body_md, published, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("Dispatch %02d", number),
			fmt.Sprintf("dispatch-%02d", number),
			"Devlog",
			"Pagination fixture",
			"",
			true,
			fmt.Sprintf("2026-01-%02dT12:00:00Z", number),
		)
		if err != nil {
			t.Fatalf("insert dispatch fixture: %v", err)
		}
	}
	if _, err := database.Exec(
		`INSERT INTO article (title, slug, category, published) VALUES (?, ?, ?, ?)`,
		"Hidden dispatch", "hidden-dispatch", "Devlog", false,
	); err != nil {
		t.Fatalf("insert unpublished fixture: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		hxRequest  bool
		want       []string
		wantAbsent []string
	}{
		{
			name:       "first page has ten results and links forward with category",
			path:       "/dispatches?category=devlog",
			want:       []string{"Dispatch 21", "Dispatch 12", `href="/dispatches?category=devlog&amp;page=2"`, `aria-current="page"`},
			wantAbsent: []string{"Dispatch 11", "Hidden dispatch"},
		},
		{
			name:       "second page preserves category in both directions",
			path:       "/dispatches?category=devlog&page=2",
			want:       []string{"Dispatch 11", "Dispatch 02", `href="/dispatches?category=devlog"`, `href="/dispatches?category=devlog&amp;page=3"`, "Page 2 of 3"},
			wantAbsent: []string{"Dispatch 21", "Hidden dispatch"},
		},
		{
			name:       "HTMX pagination returns a fragment",
			path:       "/dispatches?category=devlog&page=3",
			hxRequest:  true,
			want:       []string{`id="dispatches-content"`, "Dispatch 01", `href="/dispatches?category=devlog&amp;page=2"`},
			wantAbsent: []string{"<head>", "<html", "Dispatch 02", "Hidden dispatch"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.hxRequest {
				req.Header.Set("HX-Request", "true")
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			for _, expected := range tc.want {
				if !strings.Contains(rec.Body.String(), expected) {
					t.Errorf("body does not contain %q", expected)
				}
			}
			for _, unexpected := range tc.wantAbsent {
				if strings.Contains(rec.Body.String(), unexpected) {
					t.Errorf("body unexpectedly contains %q", unexpected)
				}
			}
		})
	}
}

func TestStyledHTMXAndServerErrors(t *testing.T) {
	server, _ := testServer(t)
	server.GET("/test-error", func(echo.Context) error { return errors.New("private diagnostic") })

	tests := []struct {
		name       string
		path       string
		hxRequest  bool
		wantStatus int
		want       string
		wantAbsent string
	}{
		{name: "HX 404 is a small fragment", path: "/missing", hxRequest: true, wantStatus: http.StatusNotFound, want: "Page not found", wantAbsent: "<html"},
		{name: "500 is a styled HTML page", path: "/test-error", wantStatus: http.StatusInternalServerError, want: "Something went wrong", wantAbsent: "private diagnostic"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.hxRequest {
				req.Header.Set("HX-Request", "true")
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tc.want) || strings.Contains(rec.Body.String(), tc.wantAbsent) {
				t.Errorf("unexpected error response body: %s", rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "text/html; charset=UTF-8" {
				t.Errorf("Content-Type = %q, want HTML", rec.Header().Get("Content-Type"))
			}
		})
	}
}
