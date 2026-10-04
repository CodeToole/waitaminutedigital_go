package main

import (
	"database/sql"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
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
	StaticDir         string
	ClarityID         string
	Notifier          notify.Notifier
	SessionSecret     string
	CSPEnforce        bool
}

func newServer(siteURL string, database *sql.DB, options ...serverOptions) *echo.Echo {
	settings := serverOptions{UploadDir: "./data/uploads", StaticDir: "static"}
	if len(options) > 0 {
		settings = options[0]
		if settings.UploadDir == "" {
			settings.UploadDir = "./data/uploads"
		}
		if settings.StaticDir == "" {
			settings.StaticDir = "static"
		}
	}
	if settings.Notifier == nil {
		settings.Notifier = notify.NoopNotifier{}
	}
	site := views.SiteConfig{SiteURL: siteURL, Production: settings.Production, ClarityID: settings.ClarityID}

	e := echo.New()
	e.HideBanner = true
	e.Use(securityHeaders(settings.CSPEnforce))
	e.Use(canonicalHostMiddleware(settings.Production))
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(cacheControlMiddleware)
	e.Use(gameAssetHeaders)
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
		Skipper: func(c echo.Context) bool {
			if !strings.HasPrefix(c.Request().URL.Path, "/static/games/") {
				return true
			}
			switch strings.ToLower(path.Ext(c.Request().URL.Path)) {
			case ".wasm", ".pck", ".js":
				return false
			default:
				return true
			}
		},
	}))
	e.HTTPErrorHandler = handlers.NewHTTPErrorHandler(site)
	sessions := scs.New()
	sessions.Lifetime = 12 * time.Hour
	sessions.Cookie.Name = "waitaminute_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = settings.Production
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	if settings.SessionSecret != "" {
		sessions.Store = auth.NewHMACSessionStore(sessions.Store, []byte(settings.SessionSecret))
	}
	e.Use(echo.WrapMiddleware(sessions.LoadAndSave))
	e.Use(deduplicateCookieVary)

	e.StaticFS("/static", os.DirFS(settings.StaticDir))
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
	e.GET("/game-room/asteroid-attack", handlers.AsteroidAttack(site))
	e.HEAD("/game-room/asteroid-attack", headOnly(handlers.AsteroidAttack(site)))
	e.GET("/projects", handlers.Projects(site))
	e.HEAD("/projects", headOnly(handlers.Projects(site)))
	e.GET("/about", handlers.About(site))
	e.HEAD("/about", headOnly(handlers.About(site)))
	e.GET("/contact", handlers.Contact(site))
	e.HEAD("/contact", headOnly(handlers.Contact(site)))
	e.POST("/contact", handlers.SubmitContact(site, database, settings.Notifier))
	e.GET("/health", handlers.Health)
	e.HEAD("/health", headOnly(handlers.Health))
	e.GET("/sitemap.xml", handlers.NewSitemap(site, database))
	e.HEAD("/sitemap.xml", headOnly(handlers.NewSitemap(site, database)))
	e.GET("/robots.txt", handlers.Robots(site))
	e.HEAD("/robots.txt", headOnly(handlers.Robots(site)))
	e.GET("/feed.xml", handlers.NewFeed(site, database))
	e.HEAD("/feed.xml", headOnly(handlers.NewFeed(site, database)))

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

func securityHeaders(enforceCSP bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Response().Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			header.Set("X-Frame-Options", "SAMEORIGIN")
			header.Set(cspHeaderName(enforceCSP), contentSecurityPolicy(c.Request().URL.Path))
			return next(c)
		}
	}
}

func cspHeaderName(enforce bool) string {
	if enforce {
		return "Content-Security-Policy"
	}
	return "Content-Security-Policy-Report-Only"
}

func contentSecurityPolicy(requestPath string) string {
	scriptSrc := "'self' 'unsafe-inline' https://*.clarity.ms"
	isGameAsset := strings.HasPrefix(requestPath, "/static/games/")
	if isGameAsset {
		scriptSrc += " 'wasm-unsafe-eval'"
	}
	policy := []string{
		"default-src 'self'",
		"script-src " + scriptSrc,
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: https://*.clarity.ms",
		"font-src 'self'",
		"connect-src 'self' https://*.clarity.ms",
		"frame-src 'self'",
		"frame-ancestors 'self'",
	}
	if isGameAsset {
		policy = append(policy, "worker-src 'self' blob:", "media-src 'self' blob:")
	}
	return strings.Join(policy, "; ")
}

func canonicalHostMiddleware(production bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			host := request.Host
			if hostName, _, err := net.SplitHostPort(host); err == nil {
				host = hostName
			}
			if production && request.URL.Path != "/health" && strings.EqualFold(host, "www.waitaminutedigital.com") {
				return c.Redirect(http.StatusMovedPermanently, "https://waitaminutedigital.com"+request.URL.RequestURI())
			}
			return next(c)
		}
	}
}

func deduplicateCookieVary(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Response().Header()
		values := header.Values(echo.HeaderVary)
		cookieSeen := false
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			tokens := strings.Split(value, ",")
			kept := make([]string, 0, len(tokens))
			for _, token := range tokens {
				token = strings.TrimSpace(token)
				if strings.EqualFold(token, "Cookie") {
					if cookieSeen {
						continue
					}
					cookieSeen = true
				}
				if token != "" {
					kept = append(kept, token)
				}
			}
			if len(kept) > 0 {
				normalized = append(normalized, strings.Join(kept, ", "))
			}
		}
		header.Del(echo.HeaderVary)
		for _, value := range normalized {
			header.Add(echo.HeaderVary, value)
		}
		return next(c)
	}
}

func cacheControlMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Request().URL.Path
		switch {
		case path == "/admin" || strings.HasPrefix(path, "/admin/"):
			c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		case strings.HasPrefix(path, "/static/games/"):
			c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		case path == "/static" || strings.HasPrefix(path, "/static/") || path == "/uploads" || strings.HasPrefix(path, "/uploads/"):
			cachePolicy := "public, max-age=31536000"
			if c.QueryParam("v") != "" {
				cachePolicy += ", immutable"
			}
			c.Response().Header().Set(echo.HeaderCacheControl, cachePolicy)
		}
		return next(c)
	}
}

func gameAssetHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !strings.HasPrefix(c.Request().URL.Path, "/static/games/") {
			return next(c)
		}
		switch strings.ToLower(path.Ext(c.Request().URL.Path)) {
		case ".wasm":
			c.Response().Header().Set(echo.HeaderContentType, "application/wasm")
		case ".pck":
			c.Response().Header().Set(echo.HeaderContentType, "application/octet-stream")
		}
		return next(c)
	}
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
