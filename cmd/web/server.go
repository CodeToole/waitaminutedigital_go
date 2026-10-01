package main

import (
	"database/sql"

	"github.com/CodeToole/waitaminutedigital_go/internal/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func newServer(siteURL string, database *sql.DB) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.HTTPErrorHandler = handlers.NewHTTPErrorHandler(siteURL)

	e.Static("/static", "static")
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/apple-touch-icon.png", "static/apple-touch-icon.png")
	e.GET("/", handlers.NewHome(siteURL, database))
	e.GET("/dispatches", handlers.NewDispatches(siteURL, database))
	e.GET("/dispatches/:slug", handlers.NewArticle(siteURL, database))
	e.GET("/health", handlers.Health)

	return e
}
