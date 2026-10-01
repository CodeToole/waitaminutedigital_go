package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeToole/waitaminutedigital_go/internal/handlers"
	"github.com/labstack/echo/v4"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantOK     bool
	}{
		{
			name:       "health check returns 200 with ok: true",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
			wantOK:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := handlers.Health(c)
			if err != nil {
				t.Fatalf("Health handler returned unexpected error: %v", err)
			}

			if rec.Code != tc.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tc.wantStatus)
			}

			var resp handlers.HealthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if resp.OK != tc.wantOK {
				t.Errorf("got ok = %v, want %v", resp.OK, tc.wantOK)
			}
		})
	}
}
