package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/CodeToole/waitaminutedigital_go/internal/notify"
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
			want:       []string{"Game Room · Waitaminute Digital", "Playable Games", "Asteroid Attack", "Godot 4.7", "Playable", `href="/game-room/asteroid-attack"`},
			activeLink: `href="/game-room" aria-current="page">Game Room</a>`,
		},
		{
			name: "Asteroid Attack",
			path: "/game-room/asteroid-attack",
			want: []string{
				"Asteroid Attack · Waitaminute Digital",
				"Pilot through an asteroid field",
				`<h2 id="game-controls-title">Controls</h2>`,
				`src="/static/games/asteroid-attack/index.html"`,
				`href="/static/games/asteroid-attack/index.html" target="_blank"`,
				`class="game-frame"`,
				`data-game-fire`,
				`src="/static/js/game-controls.js?v=`,
			},
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
			want:       []string{"Contact · Waitaminute Digital", "name=\"name\"", "name=\"email\"", "name=\"subject\"", "Game Development", "Web Development", "Custom Software", "Other", "name=\"message\"", "name=\"website\""},
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

func TestClarityScriptIsEnvGated(t *testing.T) {
	const clarityID = "wp42rt08kj"
	clarityMarker := `"clarity", "script", "` + clarityID + `"`

	tests := []struct {
		name        string
		options     serverOptions
		path        string
		wantPresent bool
	}{
		{
			name:        "absent in development",
			options:     serverOptions{Production: false, ClarityID: clarityID},
			path:        "/",
			wantPresent: false,
		},
		{
			name:        "absent on /admin in production",
			options:     serverOptions{Production: true, ClarityID: clarityID},
			path:        "/admin/login",
			wantPresent: false,
		},
		{
			name:        "present on / in production",
			options:     serverOptions{Production: true, ClarityID: clarityID},
			path:        "/",
			wantPresent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := testServer(t, tc.options)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			body := rec.Body.String()
			present := strings.Contains(body, clarityMarker)
			if present != tc.wantPresent {
				t.Errorf("clarity snippet present = %v, want %v", present, tc.wantPresent)
			}
		})
	}
}

func TestHEADPublicRoutesReturnStatusWithoutBody(t *testing.T) {
	tests := []struct {
		path       string
		wantStatus int
	}{
		{path: "/", wantStatus: http.StatusOK},
		{path: "/dispatches/missing", wantStatus: http.StatusNotFound},
		{path: "/dispatches", wantStatus: http.StatusOK},
		{path: "/game-room", wantStatus: http.StatusOK},
		{path: "/projects", wantStatus: http.StatusOK},
		{path: "/about", wantStatus: http.StatusOK},
		{path: "/contact", wantStatus: http.StatusOK},
		{path: "/health", wantStatus: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			server, _ := testServer(t)
			req := httptest.NewRequest(http.MethodHead, tc.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("HEAD body length = %d, want 0", rec.Body.Len())
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

// fakeNotifier is a test double for notify.Notifier: it records every call
// on a channel instead of making a real network request, which is what
// lets the tests below assert on the async goroutine without sleeping.
type fakeNotifier struct {
	calls chan notify.InquiryNotification
	err   error
}

func newFakeNotifier(err error) *fakeNotifier {
	return &fakeNotifier{calls: make(chan notify.InquiryNotification, 10), err: err}
}

var errFakeNotifyFailed = errors.New("fake notifier failure")

func (f *fakeNotifier) Notify(_ context.Context, notification notify.InquiryNotification) error {
	f.calls <- notification
	return f.err
}

func (f *fakeNotifier) expectCall(t *testing.T) notify.InquiryNotification {
	t.Helper()
	select {
	case notification := <-f.calls:
		return notification
	case <-time.After(2 * time.Second):
		t.Fatal("notifier was not called")
		return notify.InquiryNotification{}
	}
}

func (f *fakeNotifier) expectNoCall(t *testing.T) {
	t.Helper()
	select {
	case notification := <-f.calls:
		t.Fatalf("notifier should not have been called, got %+v", notification)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestContactValidSubmitNotifiesExactlyOnce(t *testing.T) {
	notifier := newFakeNotifier(nil)
	server, database := testServer(t, serverOptions{Notifier: notifier})

	response := postContact(t, server, validContactForm(), false)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if count := inquiryCount(t, database); count != 1 {
		t.Fatalf("inquiry count = %d, want 1", count)
	}

	notification := notifier.expectCall(t)
	if notification.Name != "Neil" || notification.Email != "neil@example.com" || notification.Subject != "Custom Software" {
		t.Errorf("notification fields were not preserved: %+v", notification)
	}
	if !strings.HasSuffix(notification.AdminURL, "/admin/inquiries") {
		t.Errorf("AdminURL = %q, want suffix /admin/inquiries", notification.AdminURL)
	}
	notifier.expectNoCall(t)
}

func TestContactHoneypotDoesNotNotify(t *testing.T) {
	notifier := newFakeNotifier(nil)
	server, _ := testServer(t, serverOptions{Notifier: notifier})

	form := validContactForm()
	form.Set("website", "https://spam.example")
	response := postContact(t, server, form, false)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	notifier.expectNoCall(t)
}

func TestContactNotifierErrorStillReturnsSuccess(t *testing.T) {
	notifier := newFakeNotifier(errFakeNotifyFailed)
	server, database := testServer(t, serverOptions{Notifier: notifier})

	response := postContact(t, server, validContactForm(), false)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Thanks for reaching out.") {
		t.Fatalf("success message missing despite notifier error: %s", response.Body.String())
	}
	if count := inquiryCount(t, database); count != 1 {
		t.Fatalf("inquiry count = %d, want 1", count)
	}
	notifier.expectCall(t)
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
