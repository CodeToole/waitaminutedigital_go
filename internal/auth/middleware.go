package auth

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
)

func RequireAdmin(sessions *scs.SessionManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !sessions.GetBool(c.Request().Context(), "admin") {
				return c.Redirect(http.StatusSeeOther, "/admin/login")
			}
			return next(c)
		}
	}
}
