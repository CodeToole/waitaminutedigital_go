package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var csrfFieldPattern = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

type adminTestClient struct {
	server    *echo.Echo
	cookies   map[string]*http.Cookie
	csrf      string
	uploadDir string
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

func TestAdminFeaturedHighlightTracksDispatch(t *testing.T) {
	client, database := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}

	create := client.postForm("/admin/dispatches", url.Values{
		"title": {"Featured Dispatch"}, "slug": {"featured-dispatch"}, "category": {"Devlog"},
		"summary": {"Original summary"}, "body_md": {"Original body"}, "published": {"on"},
		"featured_highlight": {"on"},
	}, true, false)
	if create.Code != http.StatusSeeOther || create.Header().Get("Location") != "/admin/dispatches?notice=highlight-created" {
		t.Fatalf("create response = %d %q; want highlight-created notice", create.Code, create.Header().Get("Location"))
	}
	var articleID int64
	if err := database.QueryRow(`SELECT id FROM article WHERE slug = ?`, "featured-dispatch").Scan(&articleID); err != nil {
		t.Fatalf("find featured article: %v", err)
	}
	assertGeneratedHighlight(t, database, articleID, "Featured Dispatch", "Original summary", "/dispatches/featured-dispatch", true)

	edit := client.get(fmt.Sprintf("/admin/dispatches/%d/edit", articleID))
	if !strings.Contains(edit.Body.String(), `name="featured_highlight" value="on" checked`) {
		t.Fatalf("featured state was not checked in edit form: %s", edit.Body.String())
	}

	update := client.postForm(fmt.Sprintf("/admin/dispatches/%d", articleID), url.Values{
		"title": {"Updated Dispatch"}, "slug": {"updated-dispatch"}, "category": {"News"},
		"summary": {"Updated summary"}, "body_md": {"Updated body"}, "published": {"on"},
		"featured_highlight": {"on"},
	}, true, false)
	if update.Code != http.StatusSeeOther || update.Header().Get("Location") != "/admin/dispatches?notice=saved" {
		t.Fatalf("update response = %d %q; want saved notice", update.Code, update.Header().Get("Location"))
	}
	assertGeneratedHighlight(t, database, articleID, "Updated Dispatch", "Updated summary", "/dispatches/updated-dispatch", true)

	unfeature := client.postForm(fmt.Sprintf("/admin/dispatches/%d", articleID), url.Values{
		"title": {"Updated Dispatch"}, "slug": {"updated-dispatch"}, "category": {"News"},
		"summary": {"Updated summary"}, "body_md": {"Updated body"}, "published": {"on"},
	}, true, false)
	if unfeature.Code != http.StatusSeeOther {
		t.Fatalf("unfeature response = %d", unfeature.Code)
	}
	var highlightCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM highlight WHERE article_id = ?`, articleID).Scan(&highlightCount); err != nil {
		t.Fatalf("count generated highlights: %v", err)
	}
	if highlightCount != 0 {
		t.Fatalf("unchecking featured left %d generated highlight(s)", highlightCount)
	}
}

func TestAdminDeleteDispatchCleansLinkedHighlightAndUnusedCover(t *testing.T) {
	client, database := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}
	filename := "shared-cover.webp"
	coverPath := filepath.Join(client.uploadDir, filename)
	if err := os.WriteFile(coverPath, []byte("cover"), 0600); err != nil {
		t.Fatalf("create cover fixture: %v", err)
	}

	first := client.postForm("/admin/dispatches", url.Values{
		"title": {"Featured One"}, "slug": {"featured-one"}, "category": {"Devlog"},
		"summary": {"First"}, "body_md": {"First"}, "featured_highlight": {"on"},
	}, true, false)
	if first.Code != http.StatusSeeOther {
		t.Fatalf("create first dispatch status = %d", first.Code)
	}
	var firstID int64
	if err := database.QueryRow(`SELECT id FROM article WHERE slug = ?`, "featured-one").Scan(&firstID); err != nil {
		t.Fatalf("find first article: %v", err)
	}
	if _, err := database.Exec(`UPDATE article SET cover_image = ? WHERE id = ?`, "/uploads/"+filename, firstID); err != nil {
		t.Fatalf("set first cover: %v", err)
	}
	if _, err := database.Exec(`UPDATE highlight SET image = ? WHERE article_id = ?`, "/uploads/"+filename, firstID); err != nil {
		t.Fatalf("set highlight cover: %v", err)
	}
	second := client.postForm("/admin/dispatches", url.Values{
		"title": {"Second Dispatch"}, "slug": {"second-dispatch"}, "category": {"News"},
		"summary": {"Second"}, "body_md": {"Second"},
	}, true, false)
	if second.Code != http.StatusSeeOther {
		t.Fatalf("create second dispatch status = %d", second.Code)
	}
	var secondID int64
	if err := database.QueryRow(`SELECT id FROM article WHERE slug = ?`, "second-dispatch").Scan(&secondID); err != nil {
		t.Fatalf("find second article: %v", err)
	}
	if _, err := database.Exec(`UPDATE article SET cover_image = ? WHERE id = ?`, "/uploads/"+filename, secondID); err != nil {
		t.Fatalf("set second cover: %v", err)
	}

	deleted := client.postForm(fmt.Sprintf("/admin/dispatches/%d/delete", firstID), nil, true, false)
	if deleted.Code != http.StatusSeeOther || deleted.Header().Get("Location") != "/admin/dispatches?notice=deleted" {
		t.Fatalf("delete first response = %d %q", deleted.Code, deleted.Header().Get("Location"))
	}
	var articleCount, highlightCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM article WHERE id = ?`, firstID).Scan(&articleCount); err != nil {
		t.Fatalf("count deleted article: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM highlight WHERE article_id = ?`, firstID).Scan(&highlightCount); err != nil {
		t.Fatalf("count deleted highlight: %v", err)
	}
	if articleCount != 0 || highlightCount != 0 {
		t.Fatalf("delete left article/highlight counts %d/%d", articleCount, highlightCount)
	}
	if _, err := os.Stat(coverPath); err != nil {
		t.Fatalf("shared cover was removed too early: %v", err)
	}

	deleted = client.postForm(fmt.Sprintf("/admin/dispatches/%d/delete", secondID), nil, true, false)
	if deleted.Code != http.StatusSeeOther {
		t.Fatalf("delete second response = %d", deleted.Code)
	}
	if _, err := os.Stat(coverPath); !os.IsNotExist(err) {
		t.Fatalf("unused cover still exists or could not be checked: %v", err)
	}
}

func TestAdminDeleteHighlightAndConfirmationUI(t *testing.T) {
	client, database := newAdminTestClient(t)
	client.get("/admin/login")
	if response := client.postForm("/admin/login", url.Values{"password": {"correct horse battery staple"}}, true, false); response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d", response.Code)
	}
	created := client.postForm("/admin/highlights", url.Values{
		"title": {"Bible Study App"}, "kicker": {"Test"}, "summary": {"Delete me"},
		"href": {"/game-room"}, "sort_order": {"0"}, "published": {"on"},
	}, true, false)
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/admin/highlights?notice=highlight-created" {
		t.Fatalf("create highlight response = %d %q", created.Code, created.Header().Get("Location"))
	}
	var highlightID int64
	if err := database.QueryRow(`SELECT id FROM highlight WHERE title = ?`, "Bible Study App").Scan(&highlightID); err != nil {
		t.Fatalf("find created highlight: %v", err)
	}
	page := client.get("/admin/highlights?notice=highlight-created")
	body := page.Body.String()
	for _, expected := range []string{"Highlight Created", `class="admin-confirm"`, `value="cancel" autofocus>Cancel</button>`, `class="btn btn-signout"`, "btn-draft"} {
		if !strings.Contains(body, expected) {
			t.Errorf("highlight admin page missing %q", expected)
		}
	}
	if !strings.Contains(html.UnescapeString(body), "Delete the highlight 'Bible Study App'? This can't be undone.") {
		t.Errorf("confirmation does not name the highlight and explain deletion is permanent: %s", body)
	}
	response := client.postForm(fmt.Sprintf("/admin/highlights/%d/delete", highlightID), nil, true, false)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/admin/highlights?notice=deleted" {
		t.Fatalf("delete highlight response = %d %q", response.Code, response.Header().Get("Location"))
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM highlight WHERE id = ?`, highlightID).Scan(&count); err != nil {
		t.Fatalf("count deleted highlight: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted highlight still exists")
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

func assertGeneratedHighlight(t *testing.T, database *sql.DB, articleID int64, title string, summary string, href string, published bool) {
	t.Helper()
	var gotTitle, kicker, gotSummary, gotHref string
	var gotPublished bool
	var gotArticleID int64
	err := database.QueryRow(`SELECT title, kicker, summary, href, published, article_id FROM highlight WHERE article_id = ?`, articleID).
		Scan(&gotTitle, &kicker, &gotSummary, &gotHref, &gotPublished, &gotArticleID)
	if err != nil {
		t.Fatalf("load generated highlight: %v", err)
	}
	if gotTitle != title || kicker == "" || gotSummary != summary || gotHref != href || gotPublished != published || gotArticleID != articleID {
		t.Fatalf("generated highlight = title %q kicker %q summary %q href %q published %v article %d", gotTitle, kicker, gotSummary, gotHref, gotPublished, gotArticleID)
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
	uploadDir := t.TempDir()
	server, database := testServer(t, serverOptions{
		AdminPasswordHash: string(hash),
		Production:        production,
		UploadDir:         uploadDir,
	})
	return &adminTestClient{server: server, cookies: make(map[string]*http.Cookie), uploadDir: uploadDir}, database
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
