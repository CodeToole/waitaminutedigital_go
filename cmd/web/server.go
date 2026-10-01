package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/CodeToole/waitaminutedigital_go/internal/auth"
	"github.com/CodeToole/waitaminutedigital_go/internal/handlers"
	"github.com/CodeToole/waitaminutedigital_go/internal/notify"
	"github.com/CodeToole/waitaminutedigital_go/internal/views"
	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type serverOptions struct {
	AdminPasswordHash string
	Production        bool
	UploadDir         string
	ClarityID         string
	Notifier          notify.Notifier
}

func newServer(siteURL string, database *sql.DB, options ...serverOptions) *echo.Echo {
	settings := serverOptions{UploadDir: "./data/uploads"}
	if len(options) > 0 {
		settings = options[0]
		if settings.UploadDir == "" {
			settings.UploadDir = "./data/uploads"
		}
	}
	if settings.Notifier == nil {
		settings.Notifier = notify.NoopNotifier{}
	}
	site := views.SiteConfig{SiteURL: siteURL, Production: settings.Production, ClarityID: settings.ClarityID}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.HTTPErrorHandler = handlers.NewHTTPErrorHandler(site)
	sessions := scs.New()
	sessions.Lifetime = 12 * time.Hour
	sessions.Cookie.Name = "waitaminute_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = settings.Production
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	e.Use(echo.WrapMiddleware(sessions.LoadAndSave))

	e.Static("/static", "static")
	e.Static("/uploads", settings.UploadDir)
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/apple-touch-icon.png", "static/apple-touch-icon.png")
	e.GET("/", handlers.NewHome(site, database))
	e.HEAD("/", headOnly(handlers.NewHome(site, database)))
	e.GET("/dispatches", handlers.NewDispatches(site, database))
	e.HEAD("/dispatches", headOnly(handlers.NewDispatches(site, database)))
	e.GET("/dispatches/:slug", handlers.NewArticle(site, database))
	e.HEAD("/dispatches/:slug", headOnly(handlers.NewArticle(site, database)))
	e.GET("/game-room", handlers.GameRoom(site))
	e.HEAD("/game-room", headOnly(handlers.GameRoom(site)))
	e.GET("/projects", handlers.Projects(site))
	e.HEAD("/projects", headOnly(handlers.Projects(site)))
	e.GET("/about", handlers.About(site))
	e.HEAD("/about", headOnly(handlers.About(site)))
	e.GET("/contact", handlers.Contact(site))
	e.HEAD("/contact", headOnly(handlers.Contact(site)))
	e.POST("/contact", handlers.SubmitContact(site, database, settings.Notifier))
	e.GET("/health", handlers.Health)
	e.HEAD("/health", headOnly(handlers.Health))

	admin := e.Group("/admin", middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "form:_csrf",
		ContextKey:     "csrf",
		CookieName:     "waitaminute_csrf",
		CookieHTTPOnly: true,
		CookieSecure:   settings.Production,
		CookieSameSite: http.SameSiteLaxMode,
		ErrorHandler: func(err error, c echo.Context) error {
			return echo.NewHTTPError(http.StatusForbidden, "Invalid or missing CSRF token")
		},
	}))
	loginLimiter := auth.NewLoginLimiter(settings.Production)
	login := handlers.NewAdminLogin(site, settings.AdminPasswordHash, sessions, loginLimiter)
	adminHandlers := handlers.NewAdmin(site, database, settings.UploadDir)
	admin.GET("/login", login.Form)
	admin.HEAD("/login", headOnly(login.Form))
	admin.POST("/login", login.Submit)
	protected := admin.Group("", auth.RequireAdmin(sessions))
	protected.GET("", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/admin/dispatches")
	})
	protected.POST("/logout", handlers.AdminLogout(sessions))
	protected.GET("/dispatches", adminHandlers.Dispatches)
	protected.GET("/dispatches/new", adminHandlers.NewArticle)
	protected.GET("/dispatches/:id/edit", adminHandlers.EditArticle)
	protected.POST("/dispatches", adminHandlers.CreateArticle)
	protected.POST("/dispatches/:id", adminHandlers.UpdateArticle)
	protected.POST("/dispatches/:id/delete", adminHandlers.DeleteArticle)
	protected.POST("/dispatches/:id/publish", adminHandlers.ToggleArticle)
	protected.POST("/preview", adminHandlers.Preview)
	protected.GET("/highlights", adminHandlers.Highlights)
	protected.GET("/highlights/new", adminHandlers.NewHighlight)
	protected.GET("/highlights/:id/edit", adminHandlers.EditHighlight)
	protected.POST("/highlights", adminHandlers.CreateHighlight)
	protected.POST("/highlights/:id", adminHandlers.UpdateHighlight)
	protected.POST("/highlights/:id/delete", adminHandlers.DeleteHighlight)
	protected.POST("/highlights/:id/publish", adminHandlers.ToggleHighlight)
	protected.GET("/inquiries", adminHandlers.Inquiries)
	protected.POST("/inquiries/:id/read", adminHandlers.ToggleInquiryRead)
	protected.POST("/inquiries/:id/delete", adminHandlers.DeleteInquiry)

	return e
}

func headOnly(handler echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Writer = headResponseWriter{ResponseWriter: c.Response().Writer}
		return handler(c)
	}
}

type headResponseWriter struct {
	http.ResponseWriter
}

func (headResponseWriter) Write(body []byte) (int, error) {
	return len(body), nil
}

func (writer headResponseWriter) Flush() {
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (writer headResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}
