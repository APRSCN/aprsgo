package model

import (
	"net/http"

	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

type response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// Resp is the basic resp method to return data
func Resp[T any](c fiber.Ctx, httpCode int, statusCode int, data T, msg string) error {

	return c.Status(httpCode).JSON(response[T]{statusCode, msg, data})
}

// --------------- 200 ---------------

func RespSuccess[T any](c fiber.Ctx, data T) error {
	return Resp(c, http.StatusOK, 0, data, "success")
}

// --------------- 400 ---------------

func RespNotFound(c fiber.Ctx) error {
	return Resp(c, http.StatusNotFound, 0, any(nil), "not found")
}

// --------------- 500 ---------------

func RespInternalServerError(c fiber.Ctx, err error) error {
	requestID := requestid.FromContext(c)
	logger.L.Error(
		"internal server error happened",
		zap.Error(err),
		zap.String("requestID", requestID),
	)
	return Resp(c, http.StatusInternalServerError, 0, any(nil), "internal server error")
}
