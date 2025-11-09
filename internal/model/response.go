package model

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// Resp is the basic resp method to return data
func Resp(c fiber.Ctx, data any) error {
	return c.Status(http.StatusOK).JSON(data)
}

func RespNotFound(c fiber.Ctx) error {
	return Resp(c, map[string]string{"msg": "not found"})
}
