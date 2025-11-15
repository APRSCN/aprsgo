package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// index returns static index
func index(c fiber.Ctx) error {
	return c.Status(http.StatusOK).SendFile("static/index.html")
}

// favicon returns static favicon
func favicon(c fiber.Ctx) error {
	return c.Status(http.StatusOK).SendFile("static/favicon.ico")
}

// logo returns static logo
func logo(c fiber.Ctx) error {
	return c.Status(http.StatusOK).SendFile("static/logo.svg")
}
