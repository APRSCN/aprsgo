package handler

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/middleware"
	"github.com/APRSCN/aprsgo/internal/model"
	"github.com/APRSCN/aprsgo/internal/upgrade"
	"github.com/go-playground/validator/v10"
	fiberzap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"
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

// fiberAPP builds the GoFiber application. webFS is the embedded Nuxt SSG
// bundle; it may be nil in tests, in which case no static UI is served.
func fiberAPP(webFS fs.FS) *fiber.App {
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
		SkipURIs: []string{"/api/ping", "/api/status", "/api/stats"},
		Fields:   []string{"ip", "ips", "latency", "status", "method", "url", "requestId", "ua"},
		FieldsFunc: func(c fiber.Ctx) []zap.Field {
			return []zap.Field{
				zap.String("client", ip.GetIP(c)),
			}
		},
	}))

	// Use customer header middleware
	app.Use(middleware.CustomHeader)

	registerAPI(app)
	registerSubmit(app)
	registerStatic(app, webFS)

	// Not found router handler
	app.Use(func(c fiber.Ctx) error {
		return model.RespNotFound(c)
	})

	return app
}

// registerAPI wires the JSON API under /api.
func registerAPI(app *fiber.App) {
	api := app.Group("/api")

	api.All("/ping", func(c fiber.Ctx) error {
		return model.RespSuccess(c, struct {
			Msg   string  `json:"msg"`
			Stamp float64 `json:"stamp"`
		}{
			Msg:   "pong",
			Stamp: float64(time.Now().UnixNano()) / 1e9,
		})
	})
	api.Get("/status", Status)
	api.Get("/stats", Stats)
}

// registerSubmit wires the HTTP packet submit endpoints.
// Clients historically POST to "/"; we also expose "/api/submit".
func registerSubmit(app *fiber.App) {
	app.Post("/", Submit)
	app.Post("/api/submit", Submit)
}

// registerStatic serves the embedded Nuxt SSG bundle. The SPA fallback returns
// index.html for unknown GET routes so client-side routing works.
func registerStatic(app *fiber.App, webFS fs.FS) {
	if webFS == nil {
		return
	}
	app.Use("/", static.New("", static.Config{
		FS:         webFS,
		Browse:     false,
		IndexNames: []string{"index.html"},
	}))
}

// Run starts the HTTP server in a goroutine. webFS is the embedded web bundle.
func Run(webFS fs.FS) *fiber.App {
	app := fiberAPP(webFS)

	addr := fmt.Sprintf("%s:%d", config.Get().Server.Status.Host, config.Get().Server.Status.Port)

	// Build the HTTP listener through the upgrade helper so the status port is
	// also handed off across a live upgrade (and adopted from a parent process
	// after one), then serve on it.
	ln, err := upgrade.ListenTCP(addr)
	if err != nil {
		logger.L.Error("Failed to bind HTTP listener", zap.Error(err))
		return app
	}

	go func() {
		if err = app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}); err != nil {
			logger.L.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	if config.Debug() {
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
