package middleware

import (
	"github.com/APRSCN/aprsgo/internal/env"

	"github.com/gofiber/fiber/v3"
)

// CustomHeader sets custom header
func CustomHeader(c fiber.Ctx) error {
	c.Set("Server", env.ServerText)

	return c.Next()
}
