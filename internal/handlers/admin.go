package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/CodeToole/waitaminutedigital_go/internal/content"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/models"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

const maxCoverUpload = 5 << 20

var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

type Admin struct {
	siteURL   string
	database  *sql.DB
	uploadDir string
}

func NewAdmin(siteURL string, database *sql.DB, uploadDir string) *Admin {
	return &Admin{siteURL: siteURL, database: database, uploadDir: uploadDir}
}

func (admin *Admin) Dispatches(c echo.Context) error {
	ctx := c.Request().Context()
	articles, err := db.ListAdminArticles(ctx, admin.database)
	if err != nil {
		return adminError(c, err)
	}
	unread, err := db.CountUnreadInquiries(ctx, admin.database)
	if err != nil {
		return adminError(c, err)
	}
	return admin.render(c, "Dispatches", "/admin/dispatches", func(meta views.PageMeta, csrf string) error {
		return views.AdminDispatchesPage(meta, csrf, articles, unread).Render(ctx, c.Response())
	})
}

func (admin *Admin) NewArticle(c echo.Context) error {
	return admin.articleForm(c, models.Article{Category: "Devlog"}, true, "")
}

func (admin *Admin) EditArticle(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	article, err := db.GetArticle(c.Request().Context(), admin.database, id)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return adminError(c, err)
	}
	return admin.articleForm(c, article, false, "")
}

func (admin *Admin) CreateArticle(c echo.Context) error {
	article, err := bindArticle(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid dispatch form")
	}
	if err := validateArticle(article); err != nil {
		return admin.articleForm(c, article, true, err.Error())
	}
	exists, err := db.ArticleSlugExists(c.Request().Context(), admin.database, article.Slug, 0)
	if err != nil {
		return adminError(c, err)
	}
	if exists {
		return admin.articleForm(c, article, true, "That slug is already in use.")
	}
	image, err := admin.coverImage(c, "")
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return admin.articleForm(c, article, true, err.Error())
	}
	article.CoverImage = image
	if _, err := db.CreateArticle(c.Request().Context(), admin.database, article); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
}

func (admin *Admin) UpdateArticle(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	existing, err := db.GetArticle(c.Request().Context(), admin.database, id)
	if err != nil {
		return adminError(c, err)
	}
	article, err := bindArticle(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid dispatch form")
	}
	article.ID = id
	article.CoverImage = existing.CoverImage
	article.CreatedAt = existing.CreatedAt
	if err := validateArticle(article); err != nil {
		return admin.articleForm(c, article, false, err.Error())
	}
	exists, err := db.ArticleSlugExists(c.Request().Context(), admin.database, article.Slug, id)
	if err != nil {
		return adminError(c, err)
	}
	if exists {
		return admin.articleForm(c, article, false, "That slug is already in use.")
	}
	image, err := admin.coverImage(c, article.CoverImage)
	if err != nil {
		c.Response().WriteHeader(http.StatusBadRequest)
		return admin.articleForm(c, article, false, err.Error())
	}
	article.CoverImage = image
	if err := db.UpdateArticle(c.Request().Context(), admin.database, article); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
}

func (admin *Admin) DeleteArticle(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.DeleteArticle(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
}

func (admin *Admin) ToggleArticle(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.ToggleArticlePublished(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	if c.Request().Header.Get("HX-Request") == "true" {
		articles, err := db.ListAdminArticles(c.Request().Context(), admin.database)
		if err != nil {
			return adminError(c, err)
		}
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return views.AdminArticleList(articles, csrfValue(c)).Render(c.Request().Context(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
}

func (admin *Admin) Preview(c echo.Context) error {
	var form struct {
		Body string `form:"body_md"`
	}
	if err := c.Bind(&form); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid preview request")
	}
	preview, err := content.RenderMarkdown(form.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Could not render preview")
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return views.AdminMarkdownPreview(preview).Render(c.Request().Context(), c.Response())
}

func (admin *Admin) Highlights(c echo.Context) error {
	highlights, err := db.ListAdminHighlights(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	unread, err := db.CountUnreadInquiries(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	return admin.render(c, "Highlights", "/admin/highlights", func(meta views.PageMeta, csrf string) error {
		return views.AdminHighlightsPage(meta, csrf, highlights, models.Highlight{}, false, "", unread).Render(c.Request().Context(), c.Response())
	})
}

func (admin *Admin) NewHighlight(c echo.Context) error {
	highlights, err := db.ListAdminHighlights(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	unread, err := db.CountUnreadInquiries(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	return admin.render(c, "Highlights", "/admin/highlights", func(meta views.PageMeta, csrf string) error {
		return views.AdminHighlightsPage(meta, csrf, highlights, models.Highlight{}, true, "", unread).Render(c.Request().Context(), c.Response())
	})
}

func (admin *Admin) EditHighlight(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	highlight, err := db.GetHighlight(c.Request().Context(), admin.database, id)
	if err != nil {
		return adminError(c, err)
	}
	highlights, err := db.ListAdminHighlights(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	unread, err := db.CountUnreadInquiries(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	return admin.render(c, "Highlights", "/admin/highlights", func(meta views.PageMeta, csrf string) error {
		return views.AdminHighlightsPage(meta, csrf, highlights, highlight, false, "", unread).Render(c.Request().Context(), c.Response())
	})
}

func (admin *Admin) CreateHighlight(c echo.Context) error {
	highlight, err := bindHighlight(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if _, err := db.CreateHighlight(c.Request().Context(), admin.database, highlight); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/highlights")
}

func (admin *Admin) UpdateHighlight(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	highlight, err := bindHighlight(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	highlight.ID = id
	if err := db.UpdateHighlight(c.Request().Context(), admin.database, highlight); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/highlights")
}

func (admin *Admin) DeleteHighlight(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.DeleteHighlight(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/highlights")
}

func (admin *Admin) ToggleHighlight(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.ToggleHighlightPublished(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/highlights")
}

func (admin *Admin) Inquiries(c echo.Context) error {
	inquiries, err := db.ListInquiries(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	unread, err := db.CountUnreadInquiries(c.Request().Context(), admin.database)
	if err != nil {
		return adminError(c, err)
	}
	return admin.render(c, "Inquiries", "/admin/inquiries", func(meta views.PageMeta, csrf string) error {
		return views.AdminInquiriesPage(meta, csrf, inquiries, unread).Render(c.Request().Context(), c.Response())
	})
}

func (admin *Admin) ToggleInquiryRead(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.ToggleInquiryRead(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/inquiries")
}

func (admin *Admin) DeleteInquiry(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err := db.DeleteInquiry(c.Request().Context(), admin.database, id); err != nil {
		return adminError(c, err)
	}
	return c.Redirect(http.StatusSeeOther, "/admin/inquiries")
}

func (admin *Admin) articleForm(c echo.Context, article models.Article, isNew bool, errorMessage string) error {
	meta := views.NewPageMeta(admin.siteURL, views.PageMeta{Title: "Dispatch", Description: "Manage dispatches.", Path: "/admin/dispatches"})
	return views.AdminArticleFormPage(meta, csrfValue(c), article, isNew, errorMessage).Render(c.Request().Context(), c.Response())
}

func (admin *Admin) render(c echo.Context, title string, path string, render func(views.PageMeta, string) error) error {
	meta := views.NewPageMeta(admin.siteURL, views.PageMeta{Title: title, Description: "Manage Waitaminute Digital content.", Path: path})
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return render(meta, csrfValue(c))
}

func (admin *Admin) coverImage(c echo.Context, current string) (string, error) {
	file, err := c.FormFile("cover_image")
	if errors.Is(err, http.ErrMissingFile) || file == nil {
		return current, nil
	}
	if err != nil {
		return current, fmt.Errorf("could not read cover upload")
	}
	return saveCoverImage(file, admin.uploadDir)
}

func saveCoverImage(fileHeader *multipart.FileHeader, uploadDir string) (string, error) {
	extension := uploadExtension(fileHeader.Filename)
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
		return "", errors.New("Only PNG, JPG, and WebP uploads are allowed.")
	}
	if fileHeader.Size > maxCoverUpload {
		return "", errors.New("Uploaded files must be 5MB or smaller.")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("Could not read uploaded image.")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxCoverUpload+1))
	if err != nil {
		return "", errors.New("Could not read uploaded image.")
	}
	if int64(len(body)) > maxCoverUpload {
		return "", errors.New("Uploaded files must be 5MB or smaller.")
	}
	contentType := http.DetectContentType(body)
	validType := (contentType == "image/png" && extension == ".png") ||
		(contentType == "image/jpeg" && (extension == ".jpg" || extension == ".jpeg")) ||
		(contentType == "image/webp" && extension == ".webp")
	if !validType {
		return "", errors.New("The upload content must match a PNG, JPG, or WebP image.")
	}
	randomName, err := uploadName()
	if err != nil {
		return "", fmt.Errorf("generate upload filename: %w", err)
	}
	return saveImageBytes(uploadDir, randomName+extension, body)
}

func validateArticle(article models.Article) error {
	if article.Title == "" {
		return errors.New("Title is required.")
	}
	if article.Slug == "" {
		return errors.New("Slug is required.")
	}
	validCategory := false
	for _, category := range models.ArticleCategories {
		if article.Category == category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		return errors.New("Choose a valid category.")
	}
	return nil
}

func normalizeSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = slugSeparators.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func checkboxValue(value string) bool {
	return value == "on" || value == "true" || value == "1"
}

func bindArticle(c echo.Context) (models.Article, error) {
	var form struct {
		Title    string `form:"title"`
		Slug     string `form:"slug"`
		Category string `form:"category"`
		Summary  string `form:"summary"`
		BodyMD   string `form:"body_md"`
	}
	if err := c.Bind(&form); err != nil {
		return models.Article{}, err
	}
	article := models.Article{
		Title:     strings.TrimSpace(form.Title),
		Slug:      normalizeSlug(form.Slug),
		Category:  strings.TrimSpace(form.Category),
		Summary:   strings.TrimSpace(form.Summary),
		BodyMD:    form.BodyMD,
		Published: checkboxValue(c.FormValue("published")),
	}
	if article.Slug == "" {
		article.Slug = normalizeSlug(article.Title)
	}
	return article, nil
}

func pathID(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid ID")
	}
	return id, nil
}

func bindHighlight(c echo.Context) (models.Highlight, error) {
	var form struct {
		Title     string `form:"title"`
		Kicker    string `form:"kicker"`
		Summary   string `form:"summary"`
		Href      string `form:"href"`
		Image     string `form:"image"`
		SortOrder int    `form:"sort_order"`
	}
	if err := c.Bind(&form); err != nil {
		return models.Highlight{}, err
	}
	sortOrder, err := strconv.Atoi(c.FormValue("sort_order"))
	if err != nil || sortOrder < 0 {
		return models.Highlight{}, errors.New("Sort order must be zero or greater.")
	}
	highlight := models.Highlight{
		Title:     strings.TrimSpace(form.Title),
		Kicker:    strings.TrimSpace(form.Kicker),
		Summary:   strings.TrimSpace(form.Summary),
		Href:      strings.TrimSpace(form.Href),
		Image:     strings.TrimSpace(form.Image),
		SortOrder: sortOrder,
		Published: checkboxValue(c.FormValue("published")),
	}
	if highlight.Title == "" {
		return models.Highlight{}, errors.New("Title is required.")
	}
	return highlight, nil
}

func csrfValue(c echo.Context) string {
	token, _ := c.Get("csrf").(string)
	return token
}

func adminError(c echo.Context, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	c.Logger().Error(err)
	return echo.NewHTTPError(http.StatusInternalServerError, "Admin operation failed")
}

func uploadName() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

func ensureUploadDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	return nil
}

func uploadExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

func saveImageBytes(directory string, filename string, body []byte) (string, error) {
	if err := ensureUploadDirectory(directory); err != nil {
		return "", err
	}
	path := filepath.Join(directory, filename)
	if err := os.WriteFile(path, body, 0600); err != nil {
		return "", fmt.Errorf("write upload: %w", err)
	}
	return "/uploads/" + filename, nil
}
