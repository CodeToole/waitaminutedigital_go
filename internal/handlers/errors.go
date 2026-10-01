package handlers

import (
	"errors"
	"net/http"

	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

func NewHTTPErrorHandler(site views.SiteConfig) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		status := http.StatusInternalServerError
		var httpError *echo.HTTPError
		if errors.As(err, &httpError) && httpError.Code > 0 {
			status = httpError.Code
		}
		notFound := status == http.StatusNotFound
		title := "Something went wrong"
		description := "The page could not be loaded. Please try again."
		if notFound {
			title = "Page not found"
			description = "The page you requested could not be found."
		}
		if status >= http.StatusInternalServerError {
			c.Logger().Error(err)
		}

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		c.Response().Header().Add(echo.HeaderVary, "HX-Request")
		c.Response().WriteHeader(status)
		if c.Request().Header.Get("HX-Request") == "true" {
			if renderErr := views.ErrorFragment(notFound).Render(c.Request().Context(), c.Response()); renderErr != nil {
				c.Logger().Error(renderErr)
			}
			return
		}

		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       title,
			Description: description,
			Path:        c.Request().URL.Path,
		})
		if renderErr := views.ErrorPage(meta, notFound).Render(c.Request().Context(), c.Response()); renderErr != nil {
			c.Logger().Error(renderErr)
		}
	}
}
