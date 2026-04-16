package middleware

import (
	"github.com/APRSCN/aprsgo/internal/meta"

	"github.com/gofiber/fiber/v3"
)

// CustomHeader sets custom header
func CustomHeader(c fiber.Ctx) error {
	c.Set("Server", meta.ServerText)

	return c.Next()
}
