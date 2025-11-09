package model

import (
	"net/http"

	"github.com/APRSCN/aprsgo/internal/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

func Resp(c fiber.Ctx, code int, data any, msg string) error {
	type response struct {
		Code int    `json:"code"`
		Data any    `json:"data"`
		Msg  string `json:"msg"`
	}
	return c.Status(http.StatusOK).JSON(response{code, data, msg})
}

// --------------- 200 ---------------

func RespSuccess(c fiber.Ctx, data any) error {
	return Resp(c, http.StatusOK, data, "success")
}

// --------------- 400 ---------------

func RespBadRequest(c fiber.Ctx) error {
	return Resp(c, http.StatusBadRequest, nil, "bad request")
}

func RespNotFound(c fiber.Ctx) error {
	return Resp(c, http.StatusNotFound, nil, "not found")
}

func RespMethodNotAllowed(c fiber.Ctx) error {
	return Resp(c, http.StatusMethodNotAllowed, nil, "method not allowed")
}

func RespTeaPot(c fiber.Ctx, data any) error {
	return Resp(c, http.StatusTeapot, data, "I'm a tea pot")
}

func RespTooManyRequests(c fiber.Ctx) error {
	return Resp(c, http.StatusTooManyRequests, nil, "too many requests")
}

// --------------- 500 ---------------

func RespInternalServerError(c fiber.Ctx, err error) error {
	requestID := requestid.FromContext(c)
	logger.L.Error(
		err.Error(),
		zap.String("requestID", requestID),
	)
	return Resp(c, http.StatusInternalServerError, nil, "internal server error")
}
