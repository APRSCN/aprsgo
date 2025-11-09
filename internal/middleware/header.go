package middleware

import (
	"fmt"

	"github.com/APRSCN/aprsgo/internal/config"

	"github.com/gofiber/fiber/v3"
)

// CustomHeader sets custom header
func CustomHeader(c fiber.Ctx) error {
	c.Set("X-Powered-By", fmt.Sprintf("%s %s %s", config.ENName, config.Version, config.Nickname))

	return c.Next()
}
