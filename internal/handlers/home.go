package handlers

import (
	"database/sql"
	"net/http"
	"net/url"

	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/models"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

func NewHome(siteURL string, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		category := models.ResolveCategory(c.QueryParam("category"))
		articles, err := db.ListPublishedArticles(ctx, database, category, 6)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not load dispatches")
		}

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		if c.Request().Header.Get("HX-Request") == "true" {
			return views.LatestDispatches(category, articles, true).Render(ctx, c.Response())
		}

		highlights, err := db.ListPublishedHighlights(ctx, database)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not load highlights")
		}

		path := c.Request().URL.Path
		canonicalPath := path
		if path == "/" && category != "" {
			canonicalPath += "?category=" + url.QueryEscape(models.CategorySlug(category))
		}
		meta := views.NewPageMeta(siteURL, views.PageMeta{
			Title:         "Waitaminute Digital",
			Description:   "Building games, tools, and software that solve real problems.",
			Path:          path,
			CanonicalPath: canonicalPath,
		})
		return views.Home(meta, highlights, articles, category).Render(ctx, c.Response())
	}
}
