package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var csrfFieldPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

type adminTestClient struct {
	server  *echo.Echo
	cookies map[string]*http.Cookie
	csrf    string
}

func TestAdminUnauthenticatedRedirectsToLogin(t *testing.T) {
	server, _ := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("got status %d and Location %q; want redirect to login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdminWrongPasswordAndRateLimit(t *testing.T) {
	client, _ := newAdminTestClient(t)
	client.get("/admin/login")
	for attempt := 1; attempt <= 5; attempt++ {
		response := client.postForm("/admin/login", url.Values{"password": {"wrong"}}, true, false)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
		}
		if !strings.Contains(response.Body.String(), "Password is incorrect") {
			t.Fatalf("failed login did not show generic error: %s", response.Body.String())
		}
	}
	response := client.postForm("/admin/login", url.Values{"password": {"wrong"}}, true, false)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth failed login status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}

func TestAdminPostWithoutCSRFTokenIsForbidden(t *testing.T) {
	client, _ := newAdminTestClient(t)
	client.get("/admin/login")
	response := client.postForm("/admin/login", url.Values{"password": {"wrong"}}, false, false)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAdminCreatePublishAndPublicDispatch(t *testing.T) {
	client, database := newAdminTestClient(t)
	client.get("/admin/login")
	login := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false)
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/admin/dispatches" {
		t.Fatalf("login = %d %q, want redirect to admin dispatches", login.Code, login.Header().Get("Location"))
	}

	formPage := client.get("/admin/dispatches/new")
	if formPage.Code != http.StatusOK {
		t.Fatalf("new dispatch page status = %d", formPage.Code)
	}
	create := client.postForm("/admin/dispatches", url.Values{
		"title":    {"Created from admin"},
		"slug":     {"created-from-admin"},
		"category": {"Devlog"},
		"summary":  {"A draft created through the admin form."},
		"body_md":  {"## Admin-created body"},
	}, true, false)
	if create.Code != http.StatusSeeOther {
		t.Fatalf("create status = %d, body: %s", create.Code, create.Body.String())
	}
	var articleID int64
	if err := database.QueryRow(`SELECT id FROM article WHERE slug = ?`, "created-from-admin").Scan(&articleID); err != nil {
		t.Fatalf("created draft was not saved: %v", err)
	}

	client.get("/admin/dispatches")
	publish := client.postForm(fmt.Sprintf("/admin/dispatches/%d/publish", articleID), url.Values{}, true, true)
	if publish.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body: %s", publish.Code, publish.Body.String())
	}
	if !strings.Contains(publish.Body.String(), "Published") {
		t.Fatalf("publish fragment did not show new state: %s", publish.Body.String())
	}

	public := client.get("/dispatches?category=devlog")
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), "Created from admin") {
		t.Fatalf("published article missing from public dispatches: status=%d", public.Code)
	}
}

func TestAdminSessionCookieSecureOnlyInProduction(t *testing.T) {
	for _, production := range []bool{false, true} {
		name := "development"
		if production {
			name = "production"
		}
		t.Run(name, func(t *testing.T) {
			client, _ := newAdminTestClientWithEnvironment(t, production)
			client.get("/admin/login")
			response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false)
			if response.Code != http.StatusSeeOther {
				t.Fatalf("login status = %d", response.Code)
			}
			sessionCookie := client.cookies["waitaminute_session"]
			if sessionCookie == nil {
				t.Fatal("login did not set an SCS session cookie")
			}
			if sessionCookie.Secure != production {
				t.Errorf("session Secure = %v, want %v", sessionCookie.Secure, production)
			}
		})
	}
}

func TestAdminRejectsDisguisedImageUpload(t *testing.T) {
	client, database := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}
	client.get("/admin/dispatches/new")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"_csrf":    client.csrf,
		"title":    "Fake image",
		"slug":     "fake-image",
		"category": "Devlog",
		"summary":  "Should not save",
		"body_md":  "No image",
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	file, err := writer.CreateFormFile("cover_image", "not-an-image.png")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	_, _ = file.Write([]byte("this is plain text, not a PNG"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/dispatches", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	client.addCookies(req)
	rec := httptest.NewRecorder()
	client.server.ServeHTTP(rec, req)
	client.capture(rec)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disguised image status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM article WHERE slug = ?`, "fake-image").Scan(&count); err != nil {
		t.Fatalf("query fake article: %v", err)
	}
	if count != 0 {
		t.Fatalf("disguised upload saved %d article(s), want 0", count)
	}
}

func TestAdminPreviewSanitizesMarkdown(t *testing.T) {
	client, _ := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}
	client.get("/admin/dispatches/new")
	response := client.postForm("/admin/preview", url.Values{"body_md": {"## Preview heading\n\n<script>alert('bad')</script>Visible text."}}, true, true)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "<h2>Preview heading</h2>") || !strings.Contains(response.Body.String(), "Visible text") {
		t.Fatalf("preview did not render markdown as HTML: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "<script") || strings.Contains(response.Body.String(), "alert('bad')") {
		t.Fatalf("preview retained unsafe script: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "<html") || strings.Contains(response.Body.String(), "<head>") {
		t.Fatalf("preview returned a full document: %s", response.Body.String())
	}
}

func newAdminTestClient(t *testing.T) (*adminTestClient, *sql.DB) {
	return newAdminTestClientWithEnvironment(t, false)
}

func newAdminTestClientWithEnvironment(t *testing.T, production bool) (*adminTestClient, *sql.DB) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	server, database := testServer(t, serverOptions{
		AdminPasswordHash: string(hash),
		Production:        production,
		UploadDir:         t.TempDir(),
	})
	return &adminTestClient{server: server, cookies: make(map[string]*http.Cookie)}, database
}

func (client *adminTestClient) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	client.addCookies(req)
	rec := httptest.NewRecorder()
	client.server.ServeHTTP(rec, req)
	client.capture(rec)
	return rec
}

func (client *adminTestClient) postForm(path string, values url.Values, includeCSRF bool, hx bool) *httptest.ResponseRecorder {
	if values == nil {
		values = make(url.Values)
	}
	if includeCSRF {
		values.Set("_csrf", client.csrf)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	client.addCookies(req)
	rec := httptest.NewRecorder()
	client.server.ServeHTTP(rec, req)
	client.capture(rec)
	return rec
}

func (client *adminTestClient) addCookies(req *http.Request) {
	for _, cookie := range client.cookies {
		req.AddCookie(cookie)
	}
}

func (client *adminTestClient) capture(rec *httptest.ResponseRecorder) {
	for _, cookie := range rec.Result().Cookies() {
		client.cookies[cookie.Name] = cookie
	}
	if match := csrfFieldPattern.FindStringSubmatch(rec.Body.String()); len(match) == 2 {
		client.csrf = match[1]
	}
}
