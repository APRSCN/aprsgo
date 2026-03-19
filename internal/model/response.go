package model

import (
	"net/http"

	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

// Resp is the basic resp method to return data
func Resp(c fiber.Ctx, httpCode int, statusCode int, data any, msg string) error {
	type response struct {
		Code int    `json:"code"`
		Data any    `json:"data"`
		Msg  string `json:"msg"`
	}
	return c.Status(httpCode).JSON(response{statusCode, data, msg})
}

// --------------- 200 ---------------

func RespSuccess(c fiber.Ctx, data any) error {
	return Resp(c, http.StatusOK, 0, data, "success")
}

// --------------- 400 ---------------

func RespNotFound(c fiber.Ctx) error {
	return Resp(c, http.StatusNotFound, 0, nil, "not found")
}

// --------------- 500 ---------------

func RespInternalServerError(c fiber.Ctx, err error) error {
	requestID := requestid.FromContext(c)
	logger.L.Error(
		"internal server error happened",
		zap.Error(err),
		zap.String("requestID", requestID),
	)
	return Resp(c, http.StatusInternalServerError, 0, nil, "internal server error")
}
