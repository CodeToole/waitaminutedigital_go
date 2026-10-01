package handlers

import (
	"net/http"

	"github.com/CodeToole/waitaminutedigital_go/internal/models"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/labstack/echo/v4"
)

func GameRoom(site views.SiteConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       "Game Room",
			Description: "A first Godot game is in development at Waitaminute Digital.",
			Path:        "/game-room",
		})
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return views.GameRoomPage(meta).Render(c.Request().Context(), c.Response())
	}
}

func Projects(site views.SiteConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       "Projects",
			Description: "Software and tools built around real problems by Waitaminute Digital.",
			Path:        "/projects",
		})
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return views.ProjectsPage(meta).Render(c.Request().Context(), c.Response())
	}
}

func About(site views.SiteConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		meta := views.NewPageMeta(site, views.PageMeta{
			Title:       "About",
			Description: "Meet Neil, the founder of Waitaminute Digital in Mobile, Alabama.",
			Path:        "/about",
		})
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return views.AboutPage(meta).Render(c.Request().Context(), c.Response())
	}
}

func renderContact(c echo.Context, site views.SiteConfig, values models.ContactSubmission, fieldErrors map[string]string, success bool, status int) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	if c.Request().Header.Get("HX-Request") == "true" {
		if success {
			return views.ContactSuccess().Render(c.Request().Context(), c.Response())
		}
		return views.ContactForm(values, fieldErrors).Render(c.Request().Context(), c.Response())
	}
	meta := views.NewPageMeta(site, views.PageMeta{
		Title:       "Contact",
		Description: "Get in touch with Waitaminute Digital about games, software, and web development.",
		Path:        "/contact",
	})
	return views.ContactPage(meta, values, fieldErrors, success).Render(c.Request().Context(), c.Response())
}

func Contact(site views.SiteConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		return renderContact(c, site, models.ContactSubmission{}, nil, false, http.StatusOK)
	}
}
