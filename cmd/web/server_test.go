package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticAssetsAndHomeLayout(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	server := newServer()
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
