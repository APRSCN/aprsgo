package handler

import (
	"fmt"
	"time"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/middleware"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/go-playground/validator/v10"
	fiberzap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/utils/v2"
	"go.gh.ink/json"
	"go.gh.ink/toolbox/fiber/v3/ip"
	"go.uber.org/zap"
)

// structValidator struct implementation
type structValidator struct {
	validate *validator.Validate
}

// Validate method implementation
func (v *structValidator) Validate(out any) error {
	return v.validate.Struct(out)
}

// fiberAPP provides a GoFiber app
func fiberAPP() *fiber.App {
	app := fiber.New(fiber.Config{
		JSONEncoder:     json.Marshal,
		JSONDecoder:     json.Unmarshal,
		ProxyHeader:     fiber.HeaderXForwardedFor,
		StructValidator: &structValidator{validate: validator.New()},
		ErrorHandler:    model.RespInternalServerError,
	})

	// Use recoverer
	app.Use(recoverer.New())

	// Use requestID middleware
	app.Use(requestid.New(requestid.Config{
		Next:      nil,
		Header:    fiber.HeaderXRequestID,
		Generator: utils.UUIDv4,
	}))

	// Use global logger
	app.Use(fiberzap.New(fiberzap.Config{
		Logger:   logger.L,
		SkipURIs: []string{"/ping"},
		Fields:   []string{"ip", "ips", "latency", "status", "method", "url", "requestId", "ua"},
		FieldsFunc: func(c fiber.Ctx) []zap.Field {
			return []zap.Field{
				zap.String("client", ip.GetIP(c)),
			}
		},
	}))

	// Use customer header middleware
	app.Use(middleware.CustomHeader)

	// Ping test router handler
	app.All("/ping", func(c fiber.Ctx) error {
		return model.RespSuccess(c, struct {
			Msg   string  `json:"msg"`
			Stamp float64 `json:"stamp"`
		}{
			Msg:   "pong",
			Stamp: float64(time.Now().UnixNano()) / 1e9,
		})
	})

	// Status info handler
	app.Get("/status", Status)

	// Stats info handler
	app.Get("/stats", Stats)

	// Favicon
	app.Get("/favicon.ico", favicon)

	// Logo
	app.Get("/logo.svg", logo)

	// Static service
	app.Get("/", index)

	// Not found router handler
	app.Use(func(c fiber.Ctx) error {
		return model.RespNotFound(c)
	})

	return app
}

// Run runs an HTTP server in a goroutine
func Run() *fiber.App {
	// Create Fiber app
	app := fiberAPP()

	addr := fmt.Sprintf("%s:%d", config.Get().Server.Status.Host, config.Get().Server.Status.Port)

	// Start HTTP server with GoFiber native listener in a goroutine
	go func() {
		if err := app.Listen(addr, fiber.ListenConfig{
			DisableStartupMessage: true,
		}); err != nil {
			logger.L.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	if config.Debug {
		host := config.Get().Server.Status.Host
		if host == "" {
			host = "[::]"
		}
		visit := host
		if host == "[::]" || host == "0.0.0.0" {
			visit = "localhost"
		}

		logger.L.Info(fmt.Sprintf("Server is running on %s:%d", host, config.Get().Server.Status.Port))
		logger.L.Debug(fmt.Sprintf("Visit by %s:%d", visit, config.Get().Server.Status.Port))
	}

	return app
}
