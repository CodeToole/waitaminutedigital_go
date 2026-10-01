package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/CodeToole/waitaminutedigital_go/internal/content"
	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/models"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

const dispatchPageSize = 10

func NewDispatches(site views.SiteConfig, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		category := models.ResolveCategory(c.QueryParam("category"))
		page, err := strconv.Atoi(c.QueryParam("page"))
		if err != nil || page < 1 {
			page = 1
		}

		articles, total, currentPage, err := db.ListPublishedArticlesPage(
			c.Request().Context(), database, category, page, dispatchPageSize,
		)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not load dispatches")
		}
		totalPages := (total + dispatchPageSize - 1) / dispatchPageSize
		if totalPages == 0 {
			totalPages = 1
		}

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		if c.Request().Header.Get("HX-Request") == "true" {
			return views.DispatchesContent(category, articles, currentPage, totalPages, total).Render(
				c.Request().Context(), c.Response(),
			)
		}

		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       "Dispatches",
			Description: "Notes from the game desk: experiments, launches, and the mechanics behind the build.",
			Path:        "/dispatches",
		})
		return views.DispatchesPage(meta, category, articles, currentPage, totalPages, total).Render(
			c.Request().Context(), c.Response(),
		)
	}
}

func NewArticle(site views.SiteConfig, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		article, err := db.GetPublishedArticle(c.Request().Context(), database, c.Param("slug"))
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Dispatch not found")
		}
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not load dispatch")
		}

		body, err := content.RenderMarkdown(article.BodyMD)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not render dispatch")
		}

		image := article.CoverImage
		if image == "" {
			image = "/static/img/mascot_head.webp"
		}
		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       article.Title,
			Description: article.Summary,
			Path:        c.Request().URL.Path,
			Image:       image,
			OGType:      "article",
		})
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return views.ArticlePage(meta, article, content.ReadTime(article.BodyMD), body).Render(
			c.Request().Context(), c.Response(),
		)
	}
}
