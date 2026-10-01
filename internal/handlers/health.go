package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// HealthResponse represents the payload returned by the health check endpoint.
type HealthResponse struct {
	OK bool `json:"ok"`
}

// Health handles GET /health and returns a JSON status response.
func Health(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{OK: true})
}
