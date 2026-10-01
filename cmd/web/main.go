package main

import (
	"log"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
	"github.com/CodeToole/waitaminutedigital_go/internal/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg := config.Load()

	// Initialize Echo instance
	e := echo.New()
	e.HideBanner = true

	// Standard Echo middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Static assets served from /static
	e.Static("/static", "static")
	e.File("/favicon.ico", "static/favicon.ico")
	e.File("/apple-touch-icon.png", "static/apple-touch-icon.png")

	// Routes
	e.GET("/health", handlers.Health)

	addr := ":" + cfg.Port
	log.Printf("Starting Waitaminute Digital server on %s", addr)
	if err := e.Start(addr); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
