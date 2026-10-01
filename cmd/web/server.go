package main

import (
	"github.com/CodeToole/waitaminutedigital_go/internal/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func newServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Static("/static", "static")
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/apple-touch-icon.png", "static/apple-touch-icon.png")
	e.GET("/", handlers.Home)
	e.GET("/health", handlers.Health)

	return e
}
