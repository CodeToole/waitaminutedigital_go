package handlers

import (
	"net/http"

	"github.com/CodeToole/waitaminutedigital_go/internal/auth"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
)

type AdminLogin struct {
	siteURL  string
	hash     string
	sessions *scs.SessionManager
	limiter  *auth.LoginLimiter
}

func NewAdminLogin(siteURL string, hash string, sessions *scs.SessionManager, limiter *auth.LoginLimiter) *AdminLogin {
	return &AdminLogin{siteURL: siteURL, hash: hash, sessions: sessions, limiter: limiter}
}

func (login *AdminLogin) Form(c echo.Context) error {
	return login.render(c, http.StatusOK, "")
}

func (login *AdminLogin) Submit(c echo.Context) error {
	ip := c.RealIP()
	if !login.limiter.Allowed(ip) {
		return login.render(c, http.StatusTooManyRequests, "Too many failed attempts. Wait 15 minutes before trying again.")
	}

	var form struct {
		Password string `form:"password"`
	}
	if err := c.Bind(&form); err != nil {
		return login.render(c, http.StatusBadRequest, "Enter your password and try again.")
	}
	if !auth.PasswordMatches(form.Password, login.hash) {
		login.limiter.RecordFailure(ip)
		return login.render(c, http.StatusUnauthorized, "Password is incorrect.")
	}

	ctx := c.Request().Context()
	if err := login.sessions.RenewToken(ctx); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Could not start admin session")
	}
	login.sessions.Put(ctx, "admin", true)
	login.limiter.Reset(ip)
	return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
}

func (login *AdminLogin) render(c echo.Context, status int, message string) error {
	meta := views.NewPageMeta(login.siteURL, views.PageMeta{
		Title:       "Admin Login",
		Description: "Sign in to manage Waitaminute Digital.",
		Path:        "/admin/login",
	})
	token, _ := c.Get("csrf").(string)
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return views.AdminLoginPage(meta, token, message).Render(c.Request().Context(), c.Response())
}

func AdminLogout(sessions *scs.SessionManager) echo.HandlerFunc {
	return func(c echo.Context) error {
		if err := sessions.Destroy(c.Request().Context()); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Could not end admin session")
		}
		return c.Redirect(http.StatusSeeOther, "/admin/login")
	}
}
