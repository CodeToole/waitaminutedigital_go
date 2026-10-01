package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPhase4PagesAndActiveNavigation(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		want       []string
		activeLink string
	}{
		{
			name:       "game room",
			path:       "/game-room",
			want:       []string{"Game Room · Waitaminute Digital", "First build loading…", "My first Godot game is in development", "First Godot Game", "Godot", "In development", "role=\"progressbar\""},
			activeLink: `href="/game-room" aria-current="page">Game Room</a>`,
		},
		{
			name:       "projects",
			path:       "/projects",
			want:       []string{"Projects · Waitaminute Digital", "Bible Study App", "Flutter", "Waitaminute Digital", "Go · Echo · templ · HTMX", "https://github.com/CodeToole/waitaminutedigital_go"},
			activeLink: `href="/projects" aria-current="page">Projects</a>`,
		},
		{
			name:       "about",
			path:       "/about",
			want:       []string{"About · Waitaminute Digital", "Neil", "Mobile, Alabama", "Buffalo, New York", "learning Go", "customer’s problem"},
			activeLink: `href="/about" aria-current="page">About</a>`,
		},
		{
			name:       "contact",
			path:       "/contact",
			want:       []string{"Contact · Waitaminute Digital", "name=\"name\"", "name=\"email\"", "name=\"subject\"", "Game Dev", "Custom Software", "Automation", "Other", "name=\"message\"", "name=\"website\""},
			activeLink: `href="/contact" aria-current="page">Contact</a>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := testServer(t)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
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
			if !strings.Contains(rec.Body.String(), tc.activeLink) {
				t.Errorf("active nav link %q not found", tc.activeLink)
			}
		})
	}
}

func TestContactValidationPreservesInput(t *testing.T) {
	tests := []struct {
		name       string
		form       url.Values
		wantError  string
		wantRetain string
	}{
		{
			name:       "missing name",
			form:       url.Values{"name": {""}, "email": {"neil@example.com"}, "subject": {"Other"}, "message": {"I need help with a tool."}},
			wantError:  "Please enter your name.",
			wantRetain: `value="neil@example.com"`,
		},
		{
			name:       "invalid email",
			form:       url.Values{"name": {"Neil"}, "email": {"bad-email"}, "subject": {"Other"}, "message": {"I need help with a tool."}},
			wantError:  "Enter a valid email address.",
			wantRetain: `value="Neil"`,
		},
		{
			name:       "unknown subject",
			form:       url.Values{"name": {"Neil"}, "email": {"neil@example.com"}, "subject": {"Something Else"}, "message": {"I need help with a tool."}},
			wantError:  "Choose one of the listed subjects.",
			wantRetain: "Something Else",
		},
		{
			name:       "short message",
			form:       url.Values{"name": {"Neil"}, "email": {"neil@example.com"}, "subject": {"Other"}, "message": {"Hi"}},
			wantError:  "Please include a little more detail",
			wantRetain: "Hi</textarea>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, database := testServer(t)
			response := postContact(t, server, tc.form, false)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), tc.wantError) {
				t.Errorf("body does not contain validation message %q", tc.wantError)
			}
			if !strings.Contains(response.Body.String(), tc.wantRetain) {
				t.Errorf("form input %q was not preserved", tc.wantRetain)
			}
			if count := inquiryCount(t, database); count != 0 {
				t.Errorf("validation failure saved %d inquiries, want 0", count)
			}
		})
	}
}

func TestContactValidSubmitSavesOneInquiry(t *testing.T) {
	server, database := testServer(t)
	response := postContact(t, server, validContactForm(), false)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Thanks for reaching out.") {
		t.Fatalf("success message missing: %s", response.Body.String())
	}
	if count := inquiryCount(t, database); count != 1 {
		t.Fatalf("inquiry count = %d, want 1", count)
	}
	var name, email, subject, message string
	if err := database.QueryRow(`SELECT name, email, subject, message FROM inquiry`).Scan(&name, &email, &subject, &message); err != nil {
		t.Fatalf("read saved inquiry: %v", err)
	}
	if name != "Neil" || email != "neil@example.com" || subject != "Custom Software" || message != "I need a small tool for a repetitive task." {
		t.Errorf("saved inquiry fields were not preserved: %q %q %q %q", name, email, subject, message)
	}
}

func TestContactHoneypotReturnsFakeSuccessWithoutSaving(t *testing.T) {
	server, database := testServer(t)
	form := validContactForm()
	form.Set("website", "https://spam.example")
	response := postContact(t, server, form, false)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "Thanks for reaching out.") {
		t.Fatalf("fake success message missing: %s", response.Body.String())
	}
	if count := inquiryCount(t, database); count != 0 {
		t.Fatalf("honeypot submission saved %d inquiries, want 0", count)
	}
}

func TestContactHXResponseIsFragment(t *testing.T) {
	server, database := testServer(t)
	response := postContact(t, server, validContactForm(), true)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `id="contact-form-container"`) || !strings.Contains(response.Body.String(), "Thanks for reaching out.") {
		t.Fatalf("contact success fragment missing: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "<html") || strings.Contains(response.Body.String(), "<head>") {
		t.Fatalf("HX response contained a full document: %s", response.Body.String())
	}
	if count := inquiryCount(t, database); count != 1 {
		t.Fatalf("inquiry count = %d, want 1", count)
	}
}

func validContactForm() url.Values {
	return url.Values{
		"name":    {"Neil"},
		"email":   {"neil@example.com"},
		"subject": {"Custom Software"},
		"message": {"I need a small tool for a repetitive task."},
	}
}

func postContact(t *testing.T, server *echo.Echo, form url.Values, hxRequest bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	if hxRequest {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func inquiryCount(t *testing.T, database *sql.DB) int {
	t.Helper()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM inquiry`).Scan(&count); err != nil {
		t.Fatalf("count inquiries: %v", err)
	}
	return count
}
