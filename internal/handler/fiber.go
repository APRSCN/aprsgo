package handler

import (
	"fmt"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/middleware"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/ghinknet/json"
	"github.com/ghinknet/toolbox/expr"
	"github.com/go-playground/validator/v10"
	fiberzap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/utils/v2"
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

// fiberAPP provides a fiber app
func fiberAPP() *fiber.App {
	app := fiber.New(fiber.Config{
		JSONEncoder:     json.Marshal,
		JSONDecoder:     json.Unmarshal,
		ProxyHeader:     fiber.HeaderXForwardedFor,
		StructValidator: &structValidator{validate: validator.New()},
		ErrorHandler:    model.RespInternalServerError,
	})

	// Use requestID middleware
	app.Use(requestid.New(requestid.Config{
		Next:      nil,
		Header:    fiber.HeaderXRequestID,
		Generator: utils.UUIDv4,
	}))

	app.Use(recoverer.New())

	// Use customer header middleware
	app.Use(middleware.CustomHeader)

	// Status info handler
	app.Get("/status", Status)

	// Stats info handler
	app.Get("/stats", Stats)

	// Favicon
	app.Get("/favicon.ico", favicon)

	// Logo
	app.Get("/logo.svg", logo)

	// Use global logger
	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger.L,
		Fields: []string{"ip", "ips", "latency", "status", "method", "url", "requestId", "ua"},
		FieldsFunc: func(c fiber.Ctx) []zap.Field {
			return []zap.Field{
				zap.String("client", expr.Ternary(len(c.IPs()) > 0, c.IPs(), []string{c.IP()})[0]),
			}
		},
	}))

	// Static service
	app.Get("/", index)

	// Not found router handler
	app.Use(func(c fiber.Ctx) error {
		return model.RespNotFound(c)
	})

	return app
}

// RunHTTPServer runs a HTTP server
func RunHTTPServer() {
	// Create Fiber app
	app := fiberAPP()

	addr := fmt.Sprintf("%s:%d", config.Get().Server.Status.Host, config.Get().Server.Status.Port)

	// Start HTTP server with Fiber native listener
	go func() {
		logger.L.Fatal("Failed to start main http service", zap.Error(app.Listen(addr)))
	}()

	if config.Debug {
		host := config.Get().Server.Status.Host
		if host == "" {
			host = "[::]"
		}
		visit := host
		if host == "0.0.0.0" || host == "[::]" {
			visit = "localhost"
		}

		logger.L.Info(fmt.Sprintf("HTTP Server is running on %s:%d", host, config.Get().Server.Status.Port))
		logger.L.Debug(fmt.Sprintf("Visit status by %s:%d", visit, config.Get().Server.Status.Port))
	}
}
