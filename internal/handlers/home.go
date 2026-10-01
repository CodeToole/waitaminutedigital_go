package handlers

import (
	"net/http"

	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

func Home(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(http.StatusOK)
	return views.Home().Render(c.Request().Context(), c.Response())
}
